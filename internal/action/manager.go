package action

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/wuuJiawei/stoat/internal/executil"
	"github.com/wuuJiawei/stoat/internal/model"
	"github.com/wuuJiawei/stoat/internal/runtimeinfo"
	"golang.org/x/sys/unix"
)

type Kind string

const (
	Disable    Kind = "disable"
	Quarantine Kind = "quarantine"
)

type Status string

const (
	StatusPlanned    Status = "planned"
	StatusSucceeded  Status = "succeeded"
	StatusRestored   Status = "restored"
	StatusRolledBack Status = "rolled_back"
	StatusFailed     Status = "failed"
)

const schemaVersion = 1

var operationIDPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

type Plan struct {
	SchemaVersion     int            `json:"schema_version"`
	Action            Kind           `json:"action"`
	ItemID            string         `json:"item_id"`
	Label             string         `json:"label"`
	Type              model.ItemType `json:"type"`
	Scope             model.Scope    `json:"scope"`
	Domain            string         `json:"domain"`
	ConfigPath        string         `json:"config_path"`
	ConfigSHA256      string         `json:"config_sha256"`
	WasLoaded         bool           `json:"was_loaded"`
	RequiresRoot      bool           `json:"requires_root"`
	ConfirmationToken string         `json:"confirmation_token"`
}

type Step struct {
	Name      string    `json:"name"`
	Succeeded bool      `json:"succeeded"`
	At        time.Time `json:"at"`
	Error     string    `json:"error,omitempty"`
}

type Operation struct {
	SchemaVersion  int        `json:"schema_version"`
	ID             string     `json:"id"`
	Status         Status     `json:"status"`
	Plan           Plan       `json:"plan"`
	CreatedAt      time.Time  `json:"created_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	OriginalMode   uint32     `json:"original_mode"`
	BackupPath     string     `json:"backup_path"`
	QuarantinePath string     `json:"quarantine_path,omitempty"`
	Steps          []Step     `json:"steps"`
	Error          string     `json:"error,omitempty"`
}

type RestorePlan struct {
	SchemaVersion     int    `json:"schema_version"`
	OperationID       string `json:"operation_id"`
	Label             string `json:"label"`
	Action            Kind   `json:"original_action"`
	ConfigPath        string `json:"config_path"`
	ConfirmationToken string `json:"confirmation_token"`
}

type Manager struct {
	home   string
	store  *Store
	runner executil.Runner
	uid    string
	euid   func() int
	now    func() time.Time
	newID  func() (string, error)
}

func NewManager(home, dataDir string, runner executil.Runner, uid string) *Manager {
	return &Manager{
		home: home, store: NewStore(dataDir), runner: runner, uid: uid,
		euid: os.Geteuid, now: func() time.Time { return time.Now().UTC() }, newID: randomID,
	}
}

func (m *Manager) Plan(kind Kind, item model.PersistenceItem) (Plan, error) {
	if kind != Disable && kind != Quarantine {
		return Plan{}, fmt.Errorf("unsupported action %q", kind)
	}
	if item.Source != model.SourceLaunchd || (item.Type != model.TypeLaunchAgent && item.Type != model.TypeLaunchDaemon) {
		return Plan{}, errors.New("only launchd agents and daemons can be changed")
	}
	path, err := m.validateConfigPath(item)
	if err != nil {
		return Plan{}, err
	}
	data, info, err := readProtectedFile(path)
	if err != nil {
		return Plan{}, fmt.Errorf("inspect launchd configuration: %w", err)
	}
	hash := sha256.Sum256(data)
	plan := Plan{
		SchemaVersion: schemaVersion, Action: kind, ItemID: item.ID, Label: item.Label,
		Type: item.Type, Scope: item.Scope, Domain: runtimeinfo.Domain(&item, m.uid),
		ConfigPath: path, ConfigSHA256: hex.EncodeToString(hash[:]), WasLoaded: item.Runtime.Loaded,
		RequiresRoot: item.Scope == model.ScopeSystem,
	}
	if info.Mode().Perm()&0o022 != 0 {
		return Plan{}, errors.New("refusing writable-by-group-or-others launchd configuration")
	}
	plan.ConfirmationToken = confirmationToken(string(kind), plan.ItemID, plan.ConfigSHA256, plan.Domain, plan.Label)
	return plan, nil
}

func (m *Manager) Apply(ctx context.Context, expected Plan, token string) (Operation, error) {
	if token == "" || token != expected.ConfirmationToken {
		return Operation{}, errors.New("confirmation token does not match the action plan")
	}
	if expected.RequiresRoot && m.euid() != 0 {
		return Operation{}, errors.New("system launchd items require running Stoat as root; Stoat never invokes sudo")
	}
	currentItem := model.PersistenceItem{
		ID: expected.ItemID, Label: expected.Label, Type: expected.Type, Scope: expected.Scope,
		Source: model.SourceLaunchd, ConfigPath: expected.ConfigPath,
		Runtime: model.RuntimeInfo{Loaded: expected.WasLoaded},
	}
	current, err := m.Plan(expected.Action, currentItem)
	if err != nil {
		return Operation{}, err
	}
	if current.ConfirmationToken != token {
		return Operation{}, errors.New("launchd configuration changed after planning; generate a new plan")
	}
	data, info, err := readProtectedFile(current.ConfigPath)
	if err != nil {
		return Operation{}, err
	}
	id, err := m.newID()
	if err != nil {
		return Operation{}, fmt.Errorf("create operation id: %w", err)
	}
	operation := Operation{
		SchemaVersion: schemaVersion, ID: id, Status: StatusPlanned, Plan: current,
		CreatedAt: m.now(), OriginalMode: uint32(info.Mode().Perm()),
	}
	operation.BackupPath, err = m.store.Create(operation, data)
	if err != nil {
		return Operation{}, err
	}
	recordStep := func(name string, stepErr error) error {
		step := Step{Name: name, Succeeded: stepErr == nil, At: m.now()}
		if stepErr != nil {
			step.Error = stepErr.Error()
		}
		operation.Steps = append(operation.Steps, step)
		if persistErr := m.store.Update(operation); persistErr != nil {
			return fmt.Errorf("persist %s step: %w", name, persistErr)
		}
		return nil
	}

	if current.WasLoaded {
		stepErr := m.bootout(ctx, current)
		if persistErr := recordStep("bootout", stepErr); persistErr != nil {
			return m.failAndRollback(ctx, operation, persistErr, stepErr == nil, false)
		}
		if stepErr != nil {
			return m.failAndRollback(ctx, operation, stepErr, false, false)
		}
	}
	stepErr := m.setDisabled(ctx, current, true)
	if persistErr := recordStep("disable", stepErr); persistErr != nil {
		return m.failAndRollback(ctx, operation, persistErr, current.WasLoaded, false)
	}
	if stepErr != nil {
		return m.failAndRollback(ctx, operation, stepErr, current.WasLoaded, false)
	}

	quarantined := false
	if current.Action == Quarantine {
		operation.QuarantinePath = current.ConfigPath + ".stoat-quarantined-" + operation.ID
		stepErr = moveNoReplace(current.ConfigPath, operation.QuarantinePath)
		quarantined = stepErr == nil
		if persistErr := recordStep("quarantine", stepErr); persistErr != nil {
			return m.failAndRollback(ctx, operation, persistErr, current.WasLoaded, quarantined)
		}
		if stepErr != nil {
			return m.failAndRollback(ctx, operation, stepErr, current.WasLoaded, false)
		}
	}
	stepErr = m.verifyDisabled(ctx, current)
	if persistErr := recordStep("verify", stepErr); persistErr != nil {
		return m.failAndRollback(ctx, operation, persistErr, current.WasLoaded, quarantined)
	}
	if stepErr != nil {
		return m.failAndRollback(ctx, operation, stepErr, current.WasLoaded, quarantined)
	}
	now := m.now()
	operation.Status = StatusSucceeded
	operation.CompletedAt = &now
	if err := m.store.Update(operation); err != nil {
		return m.failAndRollback(ctx, operation, fmt.Errorf("persist completed action: %w", err), current.WasLoaded, quarantined)
	}
	return operation, nil
}

func (m *Manager) PlanRestore(operationID string) (RestorePlan, error) {
	operation, err := m.store.Read(operationID)
	if err != nil {
		return RestorePlan{}, err
	}
	if operation.Status != StatusSucceeded {
		return RestorePlan{}, fmt.Errorf("operation %s is not restorable from status %s", operationID, operation.Status)
	}
	if err := m.validateStoredOperation(operation); err != nil {
		return RestorePlan{}, err
	}
	if err := m.store.VerifyBackup(operation); err != nil {
		return RestorePlan{}, err
	}
	return RestorePlan{
		SchemaVersion: schemaVersion, OperationID: operation.ID, Label: operation.Plan.Label,
		Action: operation.Plan.Action, ConfigPath: operation.Plan.ConfigPath,
		ConfirmationToken: confirmationToken("restore", operation.ID, operation.Plan.ConfigSHA256, operation.Plan.ConfigPath),
	}, nil
}

func (m *Manager) Restore(ctx context.Context, plan RestorePlan, token string) (Operation, error) {
	if token == "" || token != plan.ConfirmationToken {
		return Operation{}, errors.New("confirmation token does not match the restore plan")
	}
	current, err := m.PlanRestore(plan.OperationID)
	if err != nil {
		return Operation{}, err
	}
	if current.ConfirmationToken != token {
		return Operation{}, errors.New("operation changed after planning; generate a new restore plan")
	}
	operation, err := m.store.Read(plan.OperationID)
	if err != nil {
		return Operation{}, err
	}
	if operation.Plan.RequiresRoot && m.euid() != 0 {
		return Operation{}, errors.New("system launchd items require running Stoat as root; Stoat never invokes sudo")
	}
	if operation.Plan.Action == Quarantine {
		if _, err := os.Lstat(operation.Plan.ConfigPath); !os.IsNotExist(err) {
			return Operation{}, errors.New("refusing to overwrite existing launchd configuration during restore")
		}
		data, _, err := readProtectedFile(operation.QuarantinePath)
		if err != nil {
			return Operation{}, fmt.Errorf("inspect quarantined configuration: %w", err)
		}
		hash := sha256.Sum256(data)
		if hex.EncodeToString(hash[:]) != operation.Plan.ConfigSHA256 {
			return Operation{}, errors.New("quarantined configuration integrity check failed")
		}
		if err := moveNoReplace(operation.QuarantinePath, operation.Plan.ConfigPath); err != nil {
			return Operation{}, fmt.Errorf("restore quarantined configuration: %w", err)
		}
		if err := m.recordRestoreStep(&operation, "restore-file"); err != nil {
			return m.restoreFailed(ctx, operation, err)
		}
	} else {
		data, _, readErr := readProtectedFile(operation.Plan.ConfigPath)
		if os.IsNotExist(readErr) {
			backup, _, backupErr := readProtectedFile(operation.BackupPath)
			if backupErr != nil {
				return Operation{}, fmt.Errorf("read operation backup: %w", backupErr)
			}
			if err := writeAtomic(operation.Plan.ConfigPath, os.FileMode(operation.OriginalMode), func(writer io.Writer) error {
				_, err := writer.Write(backup)
				return err
			}); err != nil {
				return Operation{}, fmt.Errorf("restore missing launchd configuration: %w", err)
			}
			if err := m.recordRestoreStep(&operation, "restore-file-from-backup"); err != nil {
				return m.restoreFailed(ctx, operation, err)
			}
		} else if readErr != nil {
			return Operation{}, fmt.Errorf("inspect launchd configuration before restore: %w", readErr)
		} else {
			hash := sha256.Sum256(data)
			if hex.EncodeToString(hash[:]) != operation.Plan.ConfigSHA256 {
				return Operation{}, errors.New("launchd configuration changed after disable; refusing restore")
			}
		}
	}
	if err := m.setDisabled(ctx, operation.Plan, false); err != nil {
		return m.restoreFailed(ctx, operation, err)
	}
	if err := m.recordRestoreStep(&operation, "enable"); err != nil {
		return m.restoreFailed(ctx, operation, err)
	}
	if operation.Plan.WasLoaded {
		if err := m.bootstrap(ctx, operation.Plan); err != nil {
			return m.restoreFailed(ctx, operation, err)
		}
		if err := m.recordRestoreStep(&operation, "bootstrap"); err != nil {
			return m.restoreFailed(ctx, operation, err)
		}
	}
	if err := m.verifyEnabled(ctx, operation.Plan); err != nil {
		return m.restoreFailed(ctx, operation, err)
	}
	if err := m.recordRestoreStep(&operation, "verify-restore"); err != nil {
		return m.restoreFailed(ctx, operation, err)
	}
	now := m.now()
	operation.Status = StatusRestored
	operation.CompletedAt = &now
	operation.Error = ""
	if err := m.store.Update(operation); err != nil {
		return m.restoreFailed(ctx, operation, fmt.Errorf("persist completed restore: %w", err))
	}
	return operation, nil
}

func (m *Manager) List() ([]Operation, error)                 { return m.store.List() }
func (m *Manager) Read(operationID string) (Operation, error) { return m.store.Read(operationID) }

func (m *Manager) validateConfigPath(item model.PersistenceItem) (string, error) {
	if item.ConfigPath == "" || !filepath.IsAbs(item.ConfigPath) {
		return "", errors.New("launchd configuration path must be absolute")
	}
	path := filepath.Clean(item.ConfigPath)
	var root string
	switch {
	case item.Type == model.TypeLaunchAgent && item.Scope == model.ScopeUser:
		root = filepath.Join(m.home, "Library", "LaunchAgents")
	case item.Type == model.TypeLaunchAgent && item.Scope == model.ScopeSystem:
		root = "/Library/LaunchAgents"
	case item.Type == model.TypeLaunchDaemon && item.Scope == model.ScopeSystem:
		root = "/Library/LaunchDaemons"
	default:
		return "", errors.New("unsupported launchd type and scope")
	}
	if !withinRoot(path, root) || strings.HasPrefix(path, "/System/") {
		return "", fmt.Errorf("refusing launchd configuration outside protected root %s", root)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve launchd root: %w", err)
	}
	resolvedParent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return "", fmt.Errorf("resolve launchd configuration directory: %w", err)
	}
	if filepath.Clean(resolvedParent) != filepath.Clean(resolvedRoot) {
		return "", errors.New("refusing launchd configuration through a symbolic-link directory")
	}
	return path, nil
}

func (m *Manager) validateStoredOperation(operation Operation) error {
	if operation.Plan.Action != Disable && operation.Plan.Action != Quarantine {
		return errors.New("operation manifest contains an unsupported action")
	}
	item := model.PersistenceItem{
		Type: operation.Plan.Type, Scope: operation.Plan.Scope, Source: model.SourceLaunchd,
		ConfigPath: operation.Plan.ConfigPath,
	}
	path, err := m.validateConfigPath(item)
	if err != nil {
		return fmt.Errorf("validate operation path: %w", err)
	}
	if path != operation.Plan.ConfigPath || operation.Plan.RequiresRoot != (operation.Plan.Scope == model.ScopeSystem) {
		return errors.New("operation manifest scope check failed")
	}
	expectedDomain := runtimeinfo.Domain(&item, m.uid)
	if operation.Plan.Domain != expectedDomain || operation.Plan.Label == "" || operation.Plan.ItemID == "" {
		return errors.New("operation manifest launchd identity check failed")
	}
	if len(operation.Plan.ConfigSHA256) != sha256.Size*2 {
		return errors.New("operation manifest configuration digest is invalid")
	}
	if operation.Plan.Action == Quarantine {
		expected := operation.Plan.ConfigPath + ".stoat-quarantined-" + operation.ID
		if operation.QuarantinePath != expected {
			return errors.New("operation manifest quarantine path check failed")
		}
	}
	return nil
}

func withinRoot(path, root string) bool {
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	return err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))
}

func readProtectedFile(path string) ([]byte, os.FileInfo, error) {
	descriptor, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, nil, err
	}
	file := os.NewFile(uintptr(descriptor), path)
	if file == nil {
		_ = unix.Close(descriptor)
		return nil, nil, errors.New("open protected file")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, nil, errors.New("refusing non-regular or symbolic-link configuration")
	}
	if info.Size() > 2<<20 {
		return nil, nil, errors.New("launchd configuration exceeds 2 MiB")
	}
	data, err := io.ReadAll(io.LimitReader(file, (2<<20)+1))
	if len(data) > 2<<20 {
		return nil, nil, errors.New("launchd configuration exceeds 2 MiB")
	}
	return data, info, err
}

func (m *Manager) bootout(ctx context.Context, plan Plan) error {
	result, err := m.runner.Run(ctx, "launchctl", "bootout", plan.Domain, plan.ConfigPath)
	if err != nil && runtimeinfo.IsNotLoaded(result.Output) {
		return nil
	}
	return err
}

func (m *Manager) bootstrap(ctx context.Context, plan Plan) error {
	_, err := m.runner.Run(ctx, "launchctl", "bootstrap", plan.Domain, plan.ConfigPath)
	return err
}

func (m *Manager) setDisabled(ctx context.Context, plan Plan, disabled bool) error {
	verb := "enable"
	if disabled {
		verb = "disable"
	}
	_, err := m.runner.Run(ctx, "launchctl", verb, plan.Domain+"/"+plan.Label)
	return err
}

func (m *Manager) disabledState(ctx context.Context, plan Plan) (bool, error) {
	result, err := m.runner.Run(ctx, "launchctl", "print-disabled", plan.Domain)
	if err != nil {
		return false, err
	}
	return runtimeinfo.ParseDisabled(result.Output)[plan.Label], nil
}

func (m *Manager) verifyDisabled(ctx context.Context, plan Plan) error {
	disabled, err := m.disabledState(ctx, plan)
	if err != nil {
		return err
	}
	if !disabled {
		return errors.New("launchctl did not report the job as disabled")
	}
	if plan.Action == Quarantine {
		if _, err := os.Lstat(plan.ConfigPath); !os.IsNotExist(err) {
			return errors.New("launchd configuration remains at its original path")
		}
	}
	return nil
}

func (m *Manager) verifyEnabled(ctx context.Context, plan Plan) error {
	disabled, err := m.disabledState(ctx, plan)
	if err != nil {
		return err
	}
	if disabled {
		return errors.New("launchctl still reports the job as disabled")
	}
	if plan.WasLoaded {
		if _, err := m.runner.Run(ctx, "launchctl", "print", plan.Domain+"/"+plan.Label); err != nil {
			return fmt.Errorf("restored job is not loaded: %w", err)
		}
	}
	return nil
}

func (m *Manager) failAndRollback(ctx context.Context, operation Operation, cause error, reload, quarantined bool) (Operation, error) {
	rollbackErr := m.rollback(ctx, operation, reload, quarantined)
	now := m.now()
	operation.CompletedAt = &now
	operation.Error = cause.Error()
	operation.Status = StatusRolledBack
	if rollbackErr != nil {
		operation.Status = StatusFailed
		operation.Error += "; rollback: " + rollbackErr.Error()
	}
	if persistErr := m.store.Update(operation); persistErr != nil {
		return operation, fmt.Errorf("action failed: %w; persist rollback status: %v", cause, persistErr)
	}
	return operation, fmt.Errorf("action failed: %w", cause)
}

func (m *Manager) rollback(ctx context.Context, operation Operation, reload, quarantined bool) error {
	var failures []error
	if quarantined {
		if err := moveNoReplace(operation.QuarantinePath, operation.Plan.ConfigPath); err != nil {
			failures = append(failures, fmt.Errorf("restore file: %w", err))
		}
	}
	if err := m.setDisabled(ctx, operation.Plan, false); err != nil {
		failures = append(failures, fmt.Errorf("enable: %w", err))
	}
	if reload {
		if err := m.bootstrap(ctx, operation.Plan); err != nil {
			failures = append(failures, fmt.Errorf("bootstrap: %w", err))
		}
	}
	return errors.Join(failures...)
}

func (m *Manager) restoreFailed(ctx context.Context, operation Operation, cause error) (Operation, error) {
	var rollbackErr error
	if operation.Plan.WasLoaded {
		if err := m.bootout(ctx, operation.Plan); err != nil {
			rollbackErr = errors.Join(rollbackErr, err)
		}
	}
	if operation.Plan.Action == Quarantine {
		if err := moveNoReplace(operation.Plan.ConfigPath, operation.QuarantinePath); err != nil {
			rollbackErr = errors.Join(rollbackErr, err)
		}
	}
	if err := m.setDisabled(ctx, operation.Plan, true); err != nil {
		rollbackErr = errors.Join(rollbackErr, err)
	}
	operation.Status = StatusSucceeded
	operation.Error = "restore failed and original disabled state was reinstated: " + cause.Error()
	if rollbackErr != nil {
		operation.Status = StatusFailed
		operation.Error += "; rollback: " + rollbackErr.Error()
	}
	if persistErr := m.store.Update(operation); persistErr != nil {
		return operation, fmt.Errorf("restore failed: %w; persist rollback status: %v", cause, persistErr)
	}
	return operation, fmt.Errorf("restore failed: %w", cause)
}

func (m *Manager) recordRestoreStep(operation *Operation, name string) error {
	operation.Steps = append(operation.Steps, Step{Name: name, Succeeded: true, At: m.now()})
	if err := m.store.Update(*operation); err != nil {
		return fmt.Errorf("persist %s step: %w", name, err)
	}
	return nil
}

func confirmationToken(parts ...string) string {
	hash := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(hash[:12])
}

func randomID() (string, error) {
	data := make([]byte, 16)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

func moveNoReplace(source, destination string) error {
	if err := os.Link(source, destination); err != nil {
		return err
	}
	if err := os.Remove(source); err != nil {
		_ = os.Remove(destination)
		return err
	}
	return nil
}
