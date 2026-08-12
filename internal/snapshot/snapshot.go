package snapshot

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"time"

	"github.com/wuuJiawei/stoat/internal/app"
	"github.com/wuuJiawei/stoat/internal/collector"
	"github.com/wuuJiawei/stoat/internal/exporter"
	"github.com/wuuJiawei/stoat/internal/model"
)

const (
	SchemaVersion   = 1
	maxSnapshotSize = 32 << 20
)

type Snapshot struct {
	SchemaVersion int                     `json:"schema_version"`
	CreatedAt     time.Time               `json:"created_at"`
	ToolVersion   string                  `json:"tool_version,omitempty"`
	Items         []model.PersistenceItem `json:"items"`
	Warnings      []collector.Warning     `json:"warnings,omitempty"`
}

type ChangeType string

const (
	ChangeAdded    ChangeType = "added"
	ChangeRemoved  ChangeType = "removed"
	ChangeModified ChangeType = "modified"
)

type Change struct {
	Type   ChangeType             `json:"type"`
	ID     string                 `json:"id"`
	Label  string                 `json:"label"`
	Fields []string               `json:"fields,omitempty"`
	Before *model.PersistenceItem `json:"before,omitempty"`
	After  *model.PersistenceItem `json:"after,omitempty"`
}

type Diff struct {
	SchemaVersion int       `json:"schema_version"`
	Before        time.Time `json:"before"`
	After         time.Time `json:"after"`
	Changes       []Change  `json:"changes"`
}

func New(report app.Report) Snapshot {
	return Snapshot{
		SchemaVersion: SchemaVersion,
		CreatedAt:     report.GeneratedAt,
		ToolVersion:   report.ToolVersion,
		Items:         append([]model.PersistenceItem(nil), report.Items...),
		Warnings:      append([]collector.Warning(nil), report.Warnings...),
	}
}

func WriteFile(path string, force bool, value Snapshot) error {
	return exporter.WriteFile(path, force, func(writer io.Writer) error {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(value)
	})
}

func ReadFile(path string) (Snapshot, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return Snapshot{}, err
	}
	if !info.Mode().IsRegular() {
		return Snapshot{}, errors.New("refusing non-regular snapshot")
	}
	if info.Size() > maxSnapshotSize {
		return Snapshot{}, fmt.Errorf("snapshot exceeds %d bytes", maxSnapshotSize)
	}
	file, err := os.Open(path)
	if err != nil {
		return Snapshot{}, err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, maxSnapshotSize+1))
	decoder.DisallowUnknownFields()
	var value Snapshot
	if err := decoder.Decode(&value); err != nil {
		return Snapshot{}, fmt.Errorf("decode snapshot: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return Snapshot{}, errors.New("snapshot contains multiple JSON values")
		}
		return Snapshot{}, fmt.Errorf("decode trailing snapshot data: %w", err)
	}
	if value.SchemaVersion != SchemaVersion {
		return Snapshot{}, fmt.Errorf("unsupported snapshot schema version %d", value.SchemaVersion)
	}
	seen := make(map[string]struct{}, len(value.Items))
	for _, item := range value.Items {
		if item.ID == "" {
			return Snapshot{}, errors.New("snapshot contains item without id")
		}
		if _, exists := seen[item.ID]; exists {
			return Snapshot{}, fmt.Errorf("snapshot contains duplicate item id %s", item.ID)
		}
		seen[item.ID] = struct{}{}
	}
	return value, nil
}

func Compare(before, after Snapshot) Diff {
	result := Diff{SchemaVersion: SchemaVersion, Before: before.CreatedAt, After: after.CreatedAt}
	beforeByID := indexItems(before.Items)
	afterByID := indexItems(after.Items)
	ids := make([]string, 0, len(beforeByID)+len(afterByID))
	seen := make(map[string]struct{}, len(beforeByID)+len(afterByID))
	for id := range beforeByID {
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	for id := range afterByID {
		if _, exists := seen[id]; !exists {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		beforeItem, existedBefore := beforeByID[id]
		afterItem, existsAfter := afterByID[id]
		switch {
		case !existedBefore:
			copyAfter := afterItem
			result.Changes = append(result.Changes, Change{Type: ChangeAdded, ID: id, Label: afterItem.Label, After: &copyAfter})
		case !existsAfter:
			copyBefore := beforeItem
			result.Changes = append(result.Changes, Change{Type: ChangeRemoved, ID: id, Label: beforeItem.Label, Before: &copyBefore})
		default:
			fields := ChangedFields(beforeItem, afterItem)
			if len(fields) > 0 {
				copyBefore, copyAfter := beforeItem, afterItem
				result.Changes = append(result.Changes, Change{
					Type: ChangeModified, ID: id, Label: afterItem.Label, Fields: fields, Before: &copyBefore, After: &copyAfter,
				})
			}
		}
	}
	return result
}

func ChangedFields(before, after model.PersistenceItem) []string {
	fields := make([]string, 0, 12)
	compare := func(name string, left, right any) {
		if !reflect.DeepEqual(left, right) {
			fields = append(fields, name)
		}
	}
	compare("label", before.Label, after.Label)
	compare("categories", before.Categories, after.Categories)
	compare("executable", before.Program, after.Program)
	compare("arguments", before.Arguments, after.Arguments)
	compare("command", before.Command, after.Command)
	compare("working_directory", before.WorkingDir, after.WorkingDir)
	compare("triggers", triggerFingerprint(before), triggerFingerprint(after))
	compare("user", before.User, after.User)
	compare("disabled", before.Runtime.Disabled, after.Runtime.Disabled)
	compare("application", before.Attribution, after.Attribution)
	compare("signature", before.Signature, after.Signature)
	compare("file", fileFingerprint(before), fileFingerprint(after))
	return fields
}

func indexItems(items []model.PersistenceItem) map[string]model.PersistenceItem {
	result := make(map[string]model.PersistenceItem, len(items))
	for _, item := range items {
		result[item.ID] = item
	}
	return result
}

func triggerFingerprint(item model.PersistenceItem) [32]byte {
	value := struct {
		RunAtLoad bool
		KeepAlive bool
		Schedules []model.Schedule
		Watch     []string
		Queue     []string
	}{item.RunAtLoad, item.KeepAlive, item.Schedules, item.WatchPaths, item.QueueDirs}
	data, _ := json.Marshal(value)
	return sha256.Sum256(data)
}

func fileFingerprint(item model.PersistenceItem) [32]byte {
	value := struct {
		Exists   bool
		Owner    string
		Mode     string
		Writable bool
	}{item.Exists, item.Owner, item.Mode, item.WritableByOthers}
	data, _ := json.Marshal(value)
	return sha256.Sum256(data)
}
