package runtimeinfo

import (
	"testing"

	"github.com/wuuJiawei/stoat/internal/model"
)

func TestParsePrint(t *testing.T) {
	runtime := ParsePrint([]byte(`com.example.worker = {
	state = running
	pid = 431
	last exit code = 7
}`))
	if !runtime.Loaded || !runtime.Running || runtime.PID != 431 {
		t.Fatalf("unexpected runtime: %#v", runtime)
	}
	if runtime.LastExitCode == nil || *runtime.LastExitCode != 7 {
		t.Fatalf("unexpected exit code: %#v", runtime.LastExitCode)
	}
}

func TestParseDisabled(t *testing.T) {
	values := ParseDisabled([]byte(`disabled services = {
	"com.example.enabled" => false
	"com.example.disabled" => true
}`))
	if values["com.example.enabled"] || !values["com.example.disabled"] {
		t.Fatalf("unexpected disabled map: %#v", values)
	}
}

func TestDomain(t *testing.T) {
	daemon := &model.PersistenceItem{Type: model.TypeLaunchDaemon}
	agent := &model.PersistenceItem{Type: model.TypeLaunchAgent}
	if Domain(daemon, "501") != "system" || Domain(agent, "501") != "gui/501" {
		t.Fatal("unexpected launchctl domain")
	}
}
