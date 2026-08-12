package snapshot

import (
	"testing"
	"time"

	"github.com/wuuJiawei/stoat/internal/model"
)

func TestCompareIgnoresVolatileRuntimeState(t *testing.T) {
	beforeItem := model.PersistenceItem{ID: "one", Label: "job", Program: "/bin/job", Runtime: model.RuntimeInfo{Loaded: true, Running: true, PID: 10}}
	afterItem := beforeItem
	afterItem.Runtime = model.RuntimeInfo{Loaded: true, Running: true, PID: 99}
	diff := Compare(
		Snapshot{CreatedAt: time.Unix(1, 0), Items: []model.PersistenceItem{beforeItem}},
		Snapshot{CreatedAt: time.Unix(2, 0), Items: []model.PersistenceItem{afterItem}},
	)
	if len(diff.Changes) != 0 {
		t.Fatalf("volatile PID created a diff: %#v", diff.Changes)
	}
}

func TestCompareReportsMaterialChanges(t *testing.T) {
	before := Snapshot{Items: []model.PersistenceItem{{ID: "same", Label: "job", Program: "/bin/old"}, {ID: "removed", Label: "removed"}}}
	after := Snapshot{Items: []model.PersistenceItem{{ID: "same", Label: "job", Program: "/bin/new"}, {ID: "added", Label: "added"}}}
	diff := Compare(before, after)
	if len(diff.Changes) != 3 {
		t.Fatalf("expected 3 changes, got %#v", diff.Changes)
	}
	foundExecutable := false
	for _, change := range diff.Changes {
		if change.ID == "same" && len(change.Fields) == 1 && change.Fields[0] == "executable" {
			foundExecutable = true
		}
	}
	if !foundExecutable {
		t.Fatalf("executable change not reported: %#v", diff.Changes)
	}
}
