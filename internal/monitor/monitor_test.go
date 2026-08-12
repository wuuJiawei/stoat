package monitor

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/wuuJiawei/stoat/internal/app"
	"github.com/wuuJiawei/stoat/internal/model"
)

func TestObserveInitializesThenReportsChange(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state", "snapshot.json")
	first := app.Report{GeneratedAt: time.Unix(1, 0), Items: []model.PersistenceItem{{ID: "one", Label: "old"}}}
	observation, err := Observe(statePath, first)
	if err != nil || !observation.Initialized {
		t.Fatalf("initialize: %#v %v", observation, err)
	}
	second := app.Report{GeneratedAt: time.Unix(2, 0), Items: []model.PersistenceItem{{ID: "one", Label: "new"}}}
	observation, err = Observe(statePath, second)
	if err != nil {
		t.Fatal(err)
	}
	if observation.Initialized || len(observation.Diff.Changes) != 1 || observation.Diff.Changes[0].Fields[0] != "label" {
		t.Fatalf("unexpected observation: %#v", observation)
	}
	events, err := List(statePath, 10)
	if err != nil || len(events) != 1 || events[0].ID == "" {
		t.Fatalf("unexpected events: %#v %v", events, err)
	}
}

func TestObserveRequiresAbsolutePath(t *testing.T) {
	if _, err := Observe("relative.json", app.Report{}); err == nil {
		t.Fatal("expected absolute path rejection")
	}
}
