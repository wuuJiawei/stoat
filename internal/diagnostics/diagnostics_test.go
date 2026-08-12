package diagnostics

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/wuuJiawei/stoat/internal/executil"
	"github.com/wuuJiawei/stoat/internal/model"
)

type captureRunner struct{ args []string }

func (r *captureRunner) Run(_ context.Context, _ string, args ...string) (executil.Result, error) {
	r.args = append([]string(nil), args...)
	return executil.Result{Output: []byte(`{"timestamp":"2026-01-01","process":"worker","eventMessage":"started"}` + "\n")}, nil
}

func TestInspectorEscapesPredicateAndFindsIssue(t *testing.T) {
	runner := &captureRunner{}
	inspector := NewInspector(runner)
	exitCode := 7
	report := inspector.Inspect(context.Background(), model.PersistenceItem{
		ID: "one", Label: "job", Program: `/tmp/weird"name`, Exists: true,
		Runtime: model.RuntimeInfo{Checked: true, Loaded: true, LastExitCode: &exitCode},
	}, time.Hour, 10)
	if len(report.RecentLogs) != 1 || len(report.Issues) != 1 {
		t.Fatalf("unexpected report: %#v", report)
	}
	predicate := runner.args[len(runner.args)-1]
	if !strings.Contains(predicate, `weird\"name`) {
		t.Fatalf("predicate was not escaped: %q", predicate)
	}
}

func TestParseNDJSONKeepsValidEntries(t *testing.T) {
	entries, err := ParseNDJSON([]byte("not-json\n{\"process\":\"ok\",\"eventMessage\":\"message\"}\n"), 10)
	if len(entries) != 1 || err == nil {
		t.Fatalf("unexpected parse result: %#v %v", entries, err)
	}
}
