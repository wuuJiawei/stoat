package action

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wuuJiawei/stoat/internal/executil"
	"github.com/wuuJiawei/stoat/internal/model"
)

type fakeRunner struct {
	disabled bool
	loaded   bool
	failVerb string
}

func (r *fakeRunner) Run(_ context.Context, command string, args ...string) (executil.Result, error) {
	if command != "launchctl" || len(args) == 0 {
		return executil.Result{}, errors.New("unexpected command")
	}
	if args[0] == r.failVerb {
		return executil.Result{}, errors.New("injected failure")
	}
	switch args[0] {
	case "bootout":
		r.loaded = false
	case "bootstrap":
		r.loaded = true
	case "disable":
		r.disabled = true
	case "enable":
		r.disabled = false
	case "print-disabled":
		return executil.Result{Output: []byte(`"com.example.job" => ` + map[bool]string{true: "true", false: "false"}[r.disabled] + `;`)}, nil
	case "print":
		if !r.loaded {
			return executil.Result{Output: []byte("Could not find service")}, errors.New("not loaded")
		}
	}
	return executil.Result{}, nil
}

func TestDisableRequiresMatchingTokenAndRestores(t *testing.T) {
	manager, item, runner := testManager(t)
	plan, err := manager.Plan(Disable, item)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Apply(context.Background(), plan, "wrong"); err == nil {
		t.Fatal("expected confirmation failure")
	}
	operation, err := manager.Apply(context.Background(), plan, plan.ConfirmationToken)
	if err != nil {
		t.Fatal(err)
	}
	if operation.Status != StatusSucceeded || !runner.disabled || runner.loaded {
		t.Fatalf("unexpected disabled state: %#v runner=%#v", operation, runner)
	}
	restorePlan, err := manager.PlanRestore(operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := manager.Restore(context.Background(), restorePlan, restorePlan.ConfirmationToken)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Status != StatusRestored || runner.disabled || !runner.loaded {
		t.Fatalf("unexpected restored state: %#v runner=%#v", restored, runner)
	}
}

func TestQuarantineMovesAndRestoresConfiguration(t *testing.T) {
	manager, item, _ := testManager(t)
	plan, err := manager.Plan(Quarantine, item)
	if err != nil {
		t.Fatal(err)
	}
	operation, err := manager.Apply(context.Background(), plan, plan.ConfirmationToken)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(item.ConfigPath); !os.IsNotExist(err) {
		t.Fatalf("configuration still exists: %v", err)
	}
	if _, err := os.Stat(operation.QuarantinePath); err != nil {
		t.Fatal(err)
	}
	restorePlan, err := manager.PlanRestore(operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Restore(context.Background(), restorePlan, restorePlan.ConfirmationToken); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(item.ConfigPath); err != nil || !strings.Contains(string(data), "com.example.job") {
		t.Fatalf("configuration not restored: %q %v", data, err)
	}
}

func TestFailureRollsBackOriginalState(t *testing.T) {
	manager, item, runner := testManager(t)
	plan, err := manager.Plan(Disable, item)
	if err != nil {
		t.Fatal(err)
	}
	runner.failVerb = "disable"
	operation, err := manager.Apply(context.Background(), plan, plan.ConfirmationToken)
	if err == nil {
		t.Fatal("expected injected failure")
	}
	if operation.Status != StatusRolledBack || runner.disabled || !runner.loaded {
		t.Fatalf("original state not restored: %#v runner=%#v", operation, runner)
	}
}

func TestPlanRejectsCronAndUnsafeMode(t *testing.T) {
	manager, item, _ := testManager(t)
	cron := item
	cron.Source = model.SourceCron
	cron.Type = model.TypeCron
	if _, err := manager.Plan(Disable, cron); err == nil {
		t.Fatal("expected cron rejection")
	}
	if err := os.Chmod(item.ConfigPath, 0o666); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Plan(Disable, item); err == nil {
		t.Fatal("expected unsafe mode rejection")
	}
}

func testManager(t *testing.T) (*Manager, model.PersistenceItem, *fakeRunner) {
	t.Helper()
	home := t.TempDir()
	launchAgents := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(launchAgents, 0o700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(launchAgents, "com.example.job.plist")
	if err := os.WriteFile(configPath, []byte(`<plist><string>com.example.job</string></plist>`), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{loaded: true}
	manager := NewManager(home, filepath.Join(home, "store"), runner, "501")
	manager.euid = func() int { return 501 }
	manager.now = func() time.Time { return time.Unix(100, 0).UTC() }
	manager.newID = func() (string, error) { return strings.Repeat("a", 32), nil }
	item := model.PersistenceItem{
		ID: "item", Label: "com.example.job", Type: model.TypeLaunchAgent, Scope: model.ScopeUser,
		Source: model.SourceLaunchd, ConfigPath: configPath, Runtime: model.RuntimeInfo{Loaded: true},
	}
	return manager, item, runner
}
