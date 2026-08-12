package diagnostics

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/wuuJiawei/stoat/internal/executil"
	"github.com/wuuJiawei/stoat/internal/model"
)

const maxLogLine = 1 << 20

type Entry struct {
	Timestamp   string `json:"timestamp,omitempty"`
	Process     string `json:"process,omitempty"`
	Subsystem   string `json:"subsystem,omitempty"`
	Category    string `json:"category,omitempty"`
	MessageType string `json:"message_type,omitempty"`
	Message     string `json:"message,omitempty"`
}

type Report struct {
	SchemaVersion int               `json:"schema_version"`
	GeneratedAt   time.Time         `json:"generated_at"`
	ItemID        string            `json:"item_id"`
	Label         string            `json:"label"`
	Program       string            `json:"program,omitempty"`
	Runtime       model.RuntimeInfo `json:"runtime"`
	RiskLevel     model.RiskLevel   `json:"risk_level"`
	RiskScore     int               `json:"risk_score"`
	Issues        []string          `json:"issues,omitempty"`
	RecentLogs    []Entry           `json:"recent_logs,omitempty"`
	LogWarning    string            `json:"log_warning,omitempty"`
}

type Inspector struct {
	runner executil.Runner
	now    func() time.Time
}

func NewInspector(runner executil.Runner) *Inspector {
	return &Inspector{runner: runner, now: func() time.Time { return time.Now().UTC() }}
}

func (i *Inspector) Inspect(ctx context.Context, item model.PersistenceItem, period time.Duration, limit int) Report {
	if limit < 1 {
		limit = 1
	}
	if limit > 500 {
		limit = 500
	}
	report := Report{
		SchemaVersion: 1, GeneratedAt: i.now(), ItemID: item.ID, Label: item.Label,
		Program: item.Program, Runtime: item.Runtime, RiskLevel: item.RiskLevel, RiskScore: item.RiskScore,
	}
	if !item.Exists && filepath.IsAbs(item.Program) {
		report.Issues = append(report.Issues, "configured executable does not exist")
	}
	if item.Runtime.Disabled {
		report.Issues = append(report.Issues, "job is disabled in launchctl")
	}
	if item.Runtime.Checked && !item.Runtime.Loaded {
		report.Issues = append(report.Issues, "job is not currently loaded")
	}
	if item.Runtime.LastExitCode != nil && *item.Runtime.LastExitCode != 0 {
		report.Issues = append(report.Issues, fmt.Sprintf("last exit code is %d", *item.Runtime.LastExitCode))
	}
	process := filepath.Base(item.Program)
	if process == "." || process == string(filepath.Separator) || process == "" {
		report.LogWarning = "recent logs unavailable because the executable process name is unknown"
		return report
	}
	if period < time.Minute || period > 24*time.Hour {
		report.LogWarning = "recent logs unavailable because the requested period is outside 1m–24h"
		return report
	}
	predicate := `process == "` + escapePredicateString(process) + `"`
	result, err := i.runner.Run(ctx, "log", "show", "--style", "ndjson", "--last", fmt.Sprintf("%ds", int(period.Seconds())), "--predicate", predicate)
	if err != nil {
		report.LogWarning = "unified log query failed: " + err.Error()
		return report
	}
	entries, err := ParseNDJSON(result.Output, limit)
	if err != nil {
		report.LogWarning = "unified log output was partially invalid: " + err.Error()
	}
	report.RecentLogs = entries
	if result.Truncated {
		report.LogWarning = strings.TrimSpace(report.LogWarning + " output truncated at safety limit")
	}
	return report
}

func ParseNDJSON(data []byte, limit int) ([]Entry, error) {
	if limit < 1 {
		return nil, errors.New("log entry limit must be positive")
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	scanner.Buffer(make([]byte, 4096), maxLogLine)
	entries := make([]Entry, 0, min(limit, 32))
	var failures []error
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var raw struct {
			Timestamp    string `json:"timestamp"`
			Process      string `json:"process"`
			Subsystem    string `json:"subsystem"`
			Category     string `json:"category"`
			MessageType  string `json:"messageType"`
			EventMessage string `json:"eventMessage"`
		}
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			failures = append(failures, err)
			continue
		}
		entries = append(entries, Entry{
			Timestamp: raw.Timestamp, Process: raw.Process, Subsystem: raw.Subsystem,
			Category: raw.Category, MessageType: raw.MessageType, Message: raw.EventMessage,
		})
		if len(entries) >= limit {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		failures = append(failures, err)
	}
	return entries, errors.Join(failures...)
}

func escapePredicateString(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, `"`, `\"`)
}
