package runtimeinfo

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/wuuJiawei/stoat/internal/executil"
	"github.com/wuuJiawei/stoat/internal/model"
)

var ErrNotLoaded = errors.New("launchd job is not loaded")

type Inspector struct {
	runner   executil.Runner
	uid      string
	mu       sync.RWMutex
	disabled map[string]map[string]bool
}

func NewInspector(runner executil.Runner, uid string) *Inspector {
	return &Inspector{runner: runner, uid: uid, disabled: make(map[string]map[string]bool)}
}

func (i *Inspector) Name() string { return "launchctl-runtime" }

// Prepare captures domain-wide disabled state once per scan. A failure is
// non-fatal because individual jobs can still be inspected.
func (i *Inspector) Prepare(ctx context.Context) error {
	var failures []error
	for _, domain := range []string{"gui/" + i.uid, "system"} {
		result, err := i.runner.Run(ctx, "launchctl", "print-disabled", domain)
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", domain, err))
			continue
		}
		i.mu.Lock()
		i.disabled[domain] = ParseDisabled(result.Output)
		i.mu.Unlock()
	}
	return errors.Join(failures...)
}

func (i *Inspector) Enrich(ctx context.Context, item *model.PersistenceItem) error {
	if item.Source != model.SourceLaunchd || item.Label == "" {
		return nil
	}
	domain := Domain(item, i.uid)
	item.Runtime.Checked = true
	item.Runtime.Domain = domain
	i.mu.RLock()
	item.Runtime.Disabled = i.disabled[domain][item.Label]
	i.mu.RUnlock()

	result, err := i.runner.Run(ctx, "launchctl", "print", domain+"/"+item.Label)
	if err != nil {
		if IsNotLoaded(result.Output) {
			return nil
		}
		return err
	}
	parsed := ParsePrint(result.Output)
	parsed.Checked = true
	parsed.Domain = domain
	parsed.Disabled = item.Runtime.Disabled || parsed.Disabled
	item.Runtime = parsed
	return nil
}

func Domain(item *model.PersistenceItem, uid string) string {
	if item.Type == model.TypeLaunchDaemon {
		return "system"
	}
	return "gui/" + uid
}

func ParsePrint(data []byte) model.RuntimeInfo {
	result := model.RuntimeInfo{Loaded: true}
	for _, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(rawLine)
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(strings.ToLower(key))
		value = strings.TrimSpace(value)
		switch key {
		case "state":
			result.State = value
		case "pid":
			result.PID, _ = strconv.Atoi(value)
		case "last exit code":
			if code, err := strconv.Atoi(value); err == nil {
				result.LastExitCode = &code
			}
		case "disabled":
			result.Disabled = strings.EqualFold(value, "true")
		}
	}
	result.Running = strings.EqualFold(result.State, "running") || result.PID > 0
	return result
}

func ParseDisabled(data []byte) map[string]bool {
	result := make(map[string]bool)
	for _, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(rawLine)
		key, value, ok := strings.Cut(line, "=>")
		if !ok {
			continue
		}
		key = strings.Trim(strings.TrimSpace(key), "\"")
		value = strings.TrimSuffix(strings.TrimSpace(value), ";")
		if key != "" {
			result[key] = strings.EqualFold(value, "true")
		}
	}
	return result
}

func IsNotLoaded(output []byte) bool {
	text := strings.ToLower(string(output))
	return strings.Contains(text, "could not find service") ||
		strings.Contains(text, "service not found") ||
		strings.Contains(text, "no such process")
}
