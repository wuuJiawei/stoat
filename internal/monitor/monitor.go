package monitor

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/wuuJiawei/stoat/internal/app"
	"github.com/wuuJiawei/stoat/internal/exporter"
	"github.com/wuuJiawei/stoat/internal/snapshot"
)

type Observation struct {
	SchemaVersion int           `json:"schema_version"`
	ID            string        `json:"id,omitempty"`
	ObservedAt    time.Time     `json:"observed_at"`
	Initialized   bool          `json:"initialized"`
	Diff          snapshot.Diff `json:"diff"`
}

func Observe(statePath string, report app.Report) (Observation, error) {
	if statePath == "" || !filepath.IsAbs(statePath) {
		return Observation{}, errors.New("monitor state path must be absolute")
	}
	if err := secureStateDirectory(filepath.Dir(statePath)); err != nil {
		return Observation{}, err
	}
	current := snapshot.New(report)
	observation := Observation{SchemaVersion: 1, ObservedAt: report.GeneratedAt}
	previous, err := snapshot.ReadFile(statePath)
	if os.IsNotExist(err) {
		observation.Initialized = true
		if err := snapshot.WriteFile(statePath, false, current); err != nil {
			return Observation{}, fmt.Errorf("initialize monitor state: %w", err)
		}
		return observation, nil
	}
	if err != nil {
		return Observation{}, fmt.Errorf("read monitor state: %w", err)
	}
	observation.Diff = snapshot.Compare(previous, current)
	if len(observation.Diff.Changes) > 0 {
		if err := writeEvent(statePath, &observation); err != nil {
			return Observation{}, fmt.Errorf("write monitor event: %w", err)
		}
	}
	if err := snapshot.WriteFile(statePath, true, current); err != nil {
		return Observation{}, fmt.Errorf("update monitor state: %w", err)
	}
	return observation, nil
}

func List(statePath string, limit int) ([]Observation, error) {
	if statePath == "" || !filepath.IsAbs(statePath) {
		return nil, errors.New("monitor state path must be absolute")
	}
	if limit < 1 || limit > 1000 {
		return nil, errors.New("event limit must be between 1 and 1000")
	}
	directory := eventDirectory(statePath)
	info, err := os.Lstat(directory)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode().Perm()&0o077 != 0 {
		return nil, errors.New("monitor event path must be a private directory")
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() > entries[j].Name() })
	result := make([]Observation, 0, min(limit, len(entries)))
	for _, entry := range entries {
		if len(result) >= limit {
			break
		}
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		observation, err := readEvent(filepath.Join(directory, entry.Name()))
		if err != nil {
			return nil, err
		}
		result = append(result, observation)
	}
	return result, nil
}

func writeEvent(statePath string, observation *Observation) error {
	identity := struct {
		Before  time.Time         `json:"before"`
		Changes []snapshot.Change `json:"changes"`
	}{Before: observation.Diff.Before, Changes: observation.Diff.Changes}
	data, err := json.Marshal(identity)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(data)
	observation.ID = hex.EncodeToString(hash[:12])
	directory := eventDirectory(statePath)
	if err := secureStateDirectory(directory); err != nil {
		return err
	}
	name := fmt.Sprintf("%020d-%s.json", observation.Diff.Before.UnixNano(), observation.ID)
	path := filepath.Join(directory, name)
	err = exporter.WriteFile(path, false, func(writer io.Writer) error {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(observation)
	})
	if err != nil && !errors.Is(err, exporter.ErrPathExists) {
		return err
	}
	return retainNewest(directory, 1000)
}

func readEvent(path string) (Observation, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return Observation{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > 8<<20 {
		return Observation{}, errors.New("refusing unsafe monitor event")
	}
	file, err := os.Open(path)
	if err != nil {
		return Observation{}, err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, (8<<20)+1))
	decoder.DisallowUnknownFields()
	var observation Observation
	if err := decoder.Decode(&observation); err != nil {
		return Observation{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return Observation{}, errors.New("monitor event contains trailing data")
	}
	if observation.SchemaVersion != 1 || observation.ID == "" || len(observation.Diff.Changes) == 0 {
		return Observation{}, errors.New("monitor event validation failed")
	}
	return observation, nil
}

func retainNewest(directory string, limit int) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			files = append(files, entry.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	for _, name := range files[min(limit, len(files)):] {
		if err := os.Remove(filepath.Join(directory, name)); err != nil {
			return err
		}
	}
	return nil
}

func eventDirectory(statePath string) string { return filepath.Join(filepath.Dir(statePath), "events") }

func secureStateDirectory(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("create monitor state directory: %w", err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode().Perm()&0o077 != 0 {
		return errors.New("monitor state directory must be a private directory")
	}
	return nil
}
