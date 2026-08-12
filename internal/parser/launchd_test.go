package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wuuJiawei/stoat/internal/model"
)

func TestParseLaunchdJSON(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "launchd", "com.example.worker.json"))
	if err != nil {
		t.Fatal(err)
	}
	item, err := ParseLaunchdJSON(data, "/Library/LaunchAgents/com.example.worker.plist", model.TypeLaunchAgent, model.ScopeSystem)
	if err != nil {
		t.Fatal(err)
	}
	if item.Label != "com.example.worker" || item.Program != "/usr/local/bin/worker" {
		t.Fatalf("unexpected identity: %#v", item)
	}
	for _, category := range []model.Category{model.CategoryStartup, model.CategoryScheduled, model.CategoryBackground} {
		if !item.HasCategory(category) {
			t.Errorf("missing category %s", category)
		}
	}
	if len(item.Schedules) != 3 {
		t.Fatalf("expected 3 schedules, got %d", len(item.Schedules))
	}
	if !item.KeepAlive {
		t.Error("KeepAlive dictionary should enable background classification")
	}
}

func BenchmarkParseLaunchdJSON(b *testing.B) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "launchd", "com.example.worker.json"))
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for range b.N {
		if _, err := ParseLaunchdJSON(data, "/Library/LaunchAgents/com.example.worker.plist", model.TypeLaunchAgent, model.ScopeSystem); err != nil {
			b.Fatal(err)
		}
	}
}

func TestParseLaunchdCalendarDictionary(t *testing.T) {
	data := []byte(`{"Label":"test","StartCalendarInterval":{"Hour":3,"Minute":0}}`)
	item, err := ParseLaunchdJSON(data, "/tmp/test.plist", model.TypeLaunchAgent, model.ScopeUser)
	if err != nil {
		t.Fatal(err)
	}
	if len(item.Schedules) != 1 || item.Schedules[0].Description != "hour=03 minute=00" {
		t.Fatalf("unexpected schedule: %#v", item.Schedules)
	}
}
