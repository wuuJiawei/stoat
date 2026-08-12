package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wuuJiawei/stoat/internal/model"
)

func TestParseCron(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "cron", "user.crontab"))
	if err != nil {
		t.Fatal(err)
	}
	items, parseErrors := ParseCron(data, model.ScopeUser, "user:crontab", false)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if len(parseErrors) != 1 {
		t.Fatalf("expected 1 parse error, got %d", len(parseErrors))
	}
	if items[0].Schedules[0].Description != "Every 5 minutes" {
		t.Fatalf("unexpected description: %s", items[0].Schedules[0].Description)
	}
	if !items[1].HasCategory(model.CategoryStartup) || items[1].HasCategory(model.CategoryScheduled) {
		t.Fatalf("@reboot classification is wrong: %#v", items[1].Categories)
	}
}

func TestParseSystemCronUser(t *testing.T) {
	items, parseErrors := ParseCron([]byte("0 3 * * * root /usr/local/bin/backup\n"), model.ScopeSystem, "/etc/crontab", true)
	if len(parseErrors) != 0 || len(items) != 1 {
		t.Fatalf("unexpected result: items=%d errors=%d", len(items), len(parseErrors))
	}
	if items[0].User != "root" || items[0].Command != "/usr/local/bin/backup" {
		t.Fatalf("unexpected system cron: %#v", items[0])
	}
}
