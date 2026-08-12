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
	"strconv"
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
	Enable     Kind = "enable"
	Quarantine Kind = "quarantine"
	Remove     Kind = "remove"
	Uninstall  Kind = "uninstall"
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
	AppPath           string         `json:"app_path,omitempty"`
	AppInfoSHA256     string         `json:"app_info_sha256,omitempty"`
	WasLoaded         bool           `json:"was_loaded"`
	WasDisabled       bool           `json:"was_disabled"`
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
	AppTrashPath   string     `json:"app_trash_path,omitempty"`
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
	if kind != Disable && kind != Enable && kind != Quarantine && kind != Remove && kind != Uninstall {
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
		WasDisabled:  item.Runtime.Disabled,
		RequiresRoot: item.Scope == model.ScopeSystem,
	}
	if info.Mode().Perm()&0o022 != 0 {
		return Plan{}, errors.New("refusing writable-by-group-or-others launchd configuration")
	}
	if kind == Uninstall {
		appPath := item.Attribution.AppPath
		if appPath == "" {
			appPath = item.AppPath
		}
		plan.AppPath, plan.AppInfoSHA256, err = m.validateAppPath(appPath)
		if err != nil {
			return Plan{}, err
		}
	}
	plan.ConfirmationToken = confirmationToken(
		string(kind), plan.ItemID, plan.ConfigSHA256, plan.Domain, plan.Label,
		strconv.FormatBool(plan.WasLoaded), strconv.FormatBool(plan.WasDisabled), plan.AppPath, plan.AppInfoSHA256,
	)
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
		Runtime:     model.RuntimeInfo{Checked: true, Loaded: expected.WasLoaded, Disabled: expected.WasDisabled},
		Attribution: model.AttributionInfo{Checked: expected.AppPath != "", AppPath: expected.AppPath},
	}
	current, err := m.Plan(expected.Action, currentItem)
	if err != nil {
		return Operation{}, err
	}
	if current.ConfirmationToken != token {
		return Operation{}, errors.New("launchd configuration changed after planning; generate a new plan")
	}
	disabled, err := m.disabledState(ctx, current)
	if err != nil {
		return Operation{}, fmt.Errorf("verify current launchd state: %w", err)
	}
	if disabled != expected.WasDisabled {
		return Operation{}, errors.New("launchd state changed after planning; generate a new plan")
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
	recordStep := func(target *Operation, name string, stepErr error) error {
		step := Step{Name: name, Succeeded: stepErr == nil, At: m.now()}
		if stepErr != nil {
			step.Error = stepErr.Error()
		}
		target.Steps = append(target.Steps, step)
		if persistErr := m.store.Update(*target); persistErr != nil {
			return fmt.Errorf("persist %s step: %w", name, persistErr)
		}
		return nil
	}
	if current.Action == Enable {
		return m.applyEnable(ctx, operation, recordStep)
	}

	if current.WasLoaded {
		stepErr := m.bootout(ctx, current)
		if persistErr := recordStep(&operation, "bootout", stepErr); persistErr != nil {
			return m.failAndRollback(ctx, operation, persistErr, stepErr == nil, false, false)
		}
		if stepErr != nil {
			return m.failAndRollback(ctx, operation, stepErr, false, false, false)
		}
	}
	stepErr := m.setDisabled(ctx, current, true)
	if persistErr := recordStep(&operation, "disable", stepErr); persistErr != nil {
		return m.failAndRollback(ctx, operation, persistErr, current.WasLoaded, false, false)
	}
	if stepErr != nil {
		return m.failAndRollback(ctx, operation, stepErr, current.WasLoaded, false, false)
	}

	quarantined := false
	removed := false
	if current.Action == Quarantine {
		operation.QuarantinePath = current.ConfigPath + ".stoat-quarantined-" + operation.ID
		if err := m.store.Update(operation); err != nil {
			return m.failAndRollback(ctx, operation, fmt.Errorf("persist quarantine destination: %w", err), current.WasLoaded, false, false)
		}
		stepErr = moveNoReplace(current.ConfigPath, operation.QuarantinePath)
		quarantined = stepErr == nil
		if persistErr := recordStep(&operation, "quarantine", stepErr); persistErr != nil {
			return m.failAndRollback(ctx, operation, persistErr, current.WasLoaded, quarantined, false)
		}
		if stepErr != nil {
			return m.failAndRollback(ctx, operation, stepErr, current.WasLoaded, false, false)
		}
	}
	if current.Action == Remove || current.Action == Uninstall {
		stepErr = os.Remove(current.ConfigPath)
		removed = stepErr == nil
		if persistErr := recordStep(&operation, "remove-configuration", stepErr); persistErr != nil {
			return m.failAndRollback(ctx, operation, persistErr, current.WasLoaded, removed, false)
		}
		if stepErr != nil {
			return m.failAndRollback(ctx, operation, stepErr, current.WasLoaded, false, false)
		}
	}
	appMoved := false
	if current.Action == Uninstall {
		operation.AppTrashPath, stepErr = m.prepareTrashDestination(current.AppPath, operation.ID)
		if stepErr == nil {
			stepErr = m.store.Update(operation)
		}
		if stepErr == nil {
			stepErr = moveDirectoryNoReplace(current.AppPath, operation.AppTrashPath)
		}
		appMoved = stepErr == nil
		if persistErr := recordStep(&operation, "move-application-to-trash", stepErr); persistErr != nil {
			return m.failAndRollback(ctx, operation, persistErr, current.WasLoaded, removed, appMoved)
		}
		if stepErr != nil {
			return m.failAndRollback(ctx, operation, stepErr, current.WasLoaded, removed, false)
		}
	}
	stepErr = m.verifyDisabled(ctx, current)
	if persistErr := recordStep(&operation, "verify", stepErr); persistErr != nil {
		return m.failAndRollback(ctx, operation, persistErr, current.WasLoaded, quarantined || removed, appMoved)
	}
	if stepErr != nil {
		return m.failAndRollback(ctx, operation, stepErr, current.WasLoaded, quarantined || removed, appMoved)
	}
	now := m.now()
	operation.Status = StatusSucceeded
	operation.CompletedAt = &now
	if err := m.store.Update(operation); err != nil {
		return m.failAndRollback(ctx, operation, fmt.Errorf("persist completed action: %w", err), current.WasLoaded, quarantined || removed, appMoved)
	}
	return operation, nil
}

func (m *Manager) applyEnable(ctx context.Context, operation Operation, recordStep func(*Operation, string, error) error) (Operation, error) {
	plan := operation.Plan
	stepErr := m.setDisabled(ctx, plan, false)
	if persistErr := recordStep(&operation, "enable", stepErr); persistErr != nil {
		return m.failEnable(ctx, operation, persistErr, false)
	}
	if stepErr != nil {
		return m.failEnable(ctx, operation, stepErr, false)
	}
	bootstrapped := false
	if !plan.WasLoaded {
		stepErr = m.bootstrap(ctx, plan)
		bootstrapped = stepErr == nil
		if persistErr := recordStep(&operation, "bootstrap", stepErr); persistErr != nil {
			return m.failEnable(ctx, operation, persistErr, bootstrapped)
		}
		if stepErr != nil {
			return m.failEnable(ctx, operation, stepErr, false)
		}
	}
	stepErr = m.verifyEnabled(ctx, plan, true)
	if persistErr := recordStep(&operation, "verify", stepErr); persistErr != nil {
		return m.failEnable(ctx, operation, persistErr, bootstrapped)
	}
	if stepErr != nil {
		return m.failEnable(ctx, operation, stepErr, bootstrapped)
	}
	now := m.now()
	operation.Status = StatusSucceeded
	operation.CompletedAt = &now
	if err := m.store.Update(operation); err != nil {
		return m.failEnable(ctx, operation, fmt.Errorf("persist completed action: %w", err), bootstrapped)
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
	if operation.Plan.Action == Enable {
		return RestorePlan{}, errors.New("enable operations are reversed by disabling the item, not by restore")
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
	appRestored := false
	if operation.Plan.Action == Uninstall {
		if _, err := os.Lstat(operation.Plan.AppPath); !os.IsNotExist(err) {
			return Operation{}, errors.New("refusing to overwrite an existing application during restore")
		}
		if err := m.validateStoredTrashPath(operation); err != nil {
			return Operation{}, err
		}
		if err := moveDirectoryNoReplace(operation.AppTrashPath, operation.Plan.AppPath); err != nil {
			return Operation{}, fmt.Errorf("restore application from Trash: %w", err)
		}
		appRestored = true
		if err := m.recordRestoreStep(&operation, "restore-application"); err != nil {
			return m.restoreFailed(ctx, operation, err, appRestored)
		}
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
			if appRestored {
				return m.restoreFailed(ctx, operation, fmt.Errorf("restore quarantined configuration: %w", err), appRestored)
			}
			return Operation{}, fmt.Errorf("restore quarantined configuration: %w", err)
		}
		if err := m.recordRestoreStep(&operation, "restore-file"); err != nil {
			return m.restoreFailed(ctx, operation, err, appRestored)
		}
	} else {
		data, _, readErr := readProtectedFile(operation.Plan.ConfigPath)
		if os.IsNotExist(readErr) {
			backup, _, backupErr := readProtectedFile(operation.BackupPath)
			if backupErr != nil {
				if appRestored {
					return m.restoreFailed(ctx, operation, fmt.Errorf("read operation backup: %w", backupErr), appRestored)
				}
				return Operation{}, fmt.Errorf("read operation backup: %w", backupErr)
			}
			if err := writeAtomic(operation.Plan.ConfigPath, os.FileMode(operation.OriginalMode), func(writer io.Writer) error {
				_, err := writer.Write(backup)
				return err
			}); err != nil {
				if appRestored {
					return m.restoreFailed(ctx, operation, fmt.Errorf("restore missing launchd configuration: %w", err), appRestored)
				}
				return Operation{}, fmt.Errorf("restore missing launchd configuration: %w", err)
			}
			if err := m.recordRestoreStep(&operation, "restore-file-from-backup"); err != nil {
				return m.restoreFailed(ctx, operation, err, appRestored)
			}
		} else if readErr != nil {
			if appRestored {
				return m.restoreFailed(ctx, operation, fmt.Errorf("inspect launchd configuration before restore: %w", readErr), appRestored)
			}
			return Operation{}, fmt.Errorf("inspect launchd configuration before restore: %w", readErr)
		} else {
			hash := sha256.Sum256(data)
			if hex.EncodeToString(hash[:]) != operation.Plan.ConfigSHA256 {
				if appRestored {
					return m.restoreFailed(ctx, operation, errors.New("launchd configuration changed after disable; refusing restore"), appRestored)
				}
				return Operation{}, errors.New("launchd configuration changed after disable; refusing restore")
			}
		}
	}
	if err := m.setDisabled(ctx, operation.Plan, operation.Plan.WasDisabled); err != nil {
		return m.restoreFailed(ctx, operation, err, appRestored)
	}
	if err := m.recordRestoreStep(&operation, "restore-disabled-state"); err != nil {
		return m.restoreFailed(ctx, operation, err, appRestored)
	}
	if !operation.Plan.WasDisabled && operation.Plan.WasLoaded {
		if err := m.bootstrap(ctx, operation.Plan); err != nil {
			return m.restoreFailed(ctx, operation, err, appRestored)
		}
		if err := m.recordRestoreStep(&operation, "bootstrap"); err != nil {
			return m.restoreFailed(ctx, operation, err, appRestored)
		}
	}
	if operation.Plan.WasDisabled {
		disabled, err := m.disabledState(ctx, operation.Plan)
		if err != nil || !disabled {
			if err == nil {
				err = errors.New("launchctl did not restore the original disabled state")
			}
			return m.restoreFailed(ctx, operation, err, appRestored)
		}
	} else if err := m.verifyEnabled(ctx, operation.Plan, operation.Plan.WasLoaded); err != nil {
		return m.restoreFailed(ctx, operation, err, appRestored)
	}
	if err := m.recordRestoreStep(&operation, "verify-restore"); err != nil {
		return m.restoreFailed(ctx, operation, err, appRestored)
	}
	now := m.now()
	operation.Status = StatusRestored
	operation.CompletedAt = &now
	operation.Error = ""
	if err := m.store.Update(operation); err != nil {
		return m.restoreFailed(ctx, operation, fmt.Errorf("persist completed restore: %w", err), appRestored)
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
	if operation.Plan.Action != Disable && operation.Plan.Action != Enable && operation.Plan.Action != Quarantine && operation.Plan.Action != Remove && operation.Plan.Action != Uninstall {
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
	if operation.Plan.Action == Uninstall {
		if err := m.validateStoredTrashPath(operation); err != nil {
			return err
		}
		appPath, digest, err := m.validateAppPath(operation.Plan.AppPath)
		if os.IsNotExist(err) && operation.Status == StatusSucceeded {
			appPath = filepath.Clean(operation.Plan.AppPath)
			digest, err = appBundleDigest(operation.AppTrashPath)
		}
		if err != nil || appPath != operation.Plan.AppPath || digest != operation.Plan.AppInfoSHA256 {
			return errors.New("operation manifest application identity check failed")
		}
	}
	return nil
}

func (m *Manager) validateAppPath(path string) (string, string, error) {
	if path == "" || !filepath.IsAbs(path) || !strings.HasSuffix(strings.ToLower(path), ".app") {
		return "", "", errors.New("uninstall requires an attributed .app bundle")
	}
	path = filepath.Clean(path)
	allowedRoots := []string{filepath.Join(m.home, "Applications"), "/Applications"}
	allowed := false
	for _, root := range allowedRoots {
		if filepath.Dir(path) == filepath.Clean(root) {
			allowed = true
			break
		}
	}
	if !allowed || strings.HasPrefix(path, "/System/") {
		return "", "", errors.New("application must be directly inside ~/Applications or /Applications")
	}
	digest, err := appBundleDigest(path)
	if err != nil {
		return "", "", err
	}
	return path, digest, nil
}

func appBundleDigest(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("refusing a non-directory or symbolic-link application")
	}
	infoPath := filepath.Join(path, "Contents", "Info.plist")
	data, _, err := readProtectedFile(infoPath)
	if err != nil {
		return "", fmt.Errorf("inspect application Info.plist: %w", err)
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func (m *Manager) prepareTrashDestination(appPath, operationID string) (string, error) {
	trash := filepath.Join(m.home, ".Trash")
	if err := secureMkdirAll(trash); err != nil {
		return "", fmt.Errorf("prepare Trash: %w", err)
	}
	destination := filepath.Join(trash, filepath.Base(appPath)+".stoat-"+operationID)
	return destination, nil
}

func (m *Manager) validateStoredTrashPath(operation Operation) error {
	expected := filepath.Join(m.home, ".Trash", filepath.Base(operation.Plan.AppPath)+".stoat-"+operation.ID)
	if filepath.Clean(operation.AppTrashPath) != filepath.Clean(expected) {
		return errors.New("operation manifest Trash path check failed")
	}
	return nil
}

func (m *Manager) restoreConfigFromBackup(operation Operation) error {
	backup, _, err := readProtectedFile(operation.BackupPath)
	if err != nil {
		return err
	}
	return writeAtomic(operation.Plan.ConfigPath, os.FileMode(operation.OriginalMode), func(writer io.Writer) error {
		_, err := writer.Write(backup)
		return err
	})
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
	if plan.Action == Quarantine || plan.Action == Remove || plan.Action == Uninstall {
		if _, err := os.Lstat(plan.ConfigPath); !os.IsNotExist(err) {
			return errors.New("launchd configuration remains at its original path")
		}
	}
	return nil
}

func (m *Manager) verifyEnabled(ctx context.Context, plan Plan, requireLoaded bool) error {
	disabled, err := m.disabledState(ctx, plan)
	if err != nil {
		return err
	}
	if disabled {
		return errors.New("launchctl still reports the job as disabled")
	}
	if requireLoaded {
		if _, err := m.runner.Run(ctx, "launchctl", "print", plan.Domain+"/"+plan.Label); err != nil {
			return fmt.Errorf("restored job is not loaded: %w", err)
		}
	}
	return nil
}

func (m *Manager) failAndRollback(ctx context.Context, operation Operation, cause error, reload, configMoved, appMoved bool) (Operation, error) {
	rollbackErr := m.rollback(ctx, operation, reload, configMoved, appMoved)
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

func (m *Manager) failEnable(ctx context.Context, operation Operation, cause error, bootstrapped bool) (Operation, error) {
	var failures []error
	if bootstrapped {
		if err := m.bootout(ctx, operation.Plan); err != nil {
			failures = append(failures, fmt.Errorf("bootout: %w", err))
		}
	}
	if err := m.setDisabled(ctx, operation.Plan, operation.Plan.WasDisabled); err != nil {
		failures = append(failures, fmt.Errorf("restore disabled state: %w", err))
	}
	now := m.now()
	operation.CompletedAt = &now
	operation.Error = cause.Error()
	operation.Status = StatusRolledBack
	if rollbackErr := errors.Join(failures...); rollbackErr != nil {
		operation.Status = StatusFailed
		operation.Error += "; rollback: " + rollbackErr.Error()
	}
	if persistErr := m.store.Update(operation); persistErr != nil {
		return operation, fmt.Errorf("action failed: %w; persist rollback status: %v", cause, persistErr)
	}
	return operation, fmt.Errorf("action failed: %w", cause)
}

func (m *Manager) rollback(ctx context.Context, operation Operation, reload, configMoved, appMoved bool) error {
	var failures []error
	if appMoved {
		if err := moveDirectoryNoReplace(operation.AppTrashPath, operation.Plan.AppPath); err != nil {
			failures = append(failures, fmt.Errorf("restore application: %w", err))
		}
	}
	if configMoved {
		if operation.Plan.Action == Quarantine {
			if err := moveNoReplace(operation.QuarantinePath, operation.Plan.ConfigPath); err != nil {
				failures = append(failures, fmt.Errorf("restore file: %w", err))
			}
		} else if err := m.restoreConfigFromBackup(operation); err != nil {
			failures = append(failures, fmt.Errorf("restore file: %w", err))
		}
	}
	if err := m.setDisabled(ctx, operation.Plan, operation.Plan.WasDisabled); err != nil {
		failures = append(failures, fmt.Errorf("restore disabled state: %w", err))
	}
	if reload {
		if err := m.bootstrap(ctx, operation.Plan); err != nil {
			failures = append(failures, fmt.Errorf("bootstrap: %w", err))
		}
	}
	return errors.Join(failures...)
}

func (m *Manager) restoreFailed(ctx context.Context, operation Operation, cause error, appRestored bool) (Operation, error) {
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
	if operation.Plan.Action == Remove || operation.Plan.Action == Uninstall {
		if err := os.Remove(operation.Plan.ConfigPath); err != nil && !os.IsNotExist(err) {
			rollbackErr = errors.Join(rollbackErr, err)
		}
	}
	if appRestored {
		if err := moveDirectoryNoReplace(operation.Plan.AppPath, operation.AppTrashPath); err != nil {
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

func moveDirectoryNoReplace(source, destination string) error {
	if _, err := os.Lstat(destination); !os.IsNotExist(err) {
		if err == nil {
			return errors.New("destination already exists")
		}
		return err
	}
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("source is not a regular application directory")
	}
	return os.Rename(source, destination)
}
