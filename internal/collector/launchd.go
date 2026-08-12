package collector

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wuuJiawei/stoat/internal/executil"
	"github.com/wuuJiawei/stoat/internal/model"
	"github.com/wuuJiawei/stoat/internal/parser"
)

const maxPlistSize = 2 << 20

type LaunchdPath struct {
	Path  string
	Type  model.ItemType
	Scope model.Scope
}

type Launchd struct {
	runner executil.Runner
	paths  []LaunchdPath
}

func NewLaunchd(runner executil.Runner, home string, includeAppleSystem bool) *Launchd {
	paths := []LaunchdPath{
		{Path: filepath.Join(home, "Library", "LaunchAgents"), Type: model.TypeLaunchAgent, Scope: model.ScopeUser},
		{Path: "/Library/LaunchAgents", Type: model.TypeLaunchAgent, Scope: model.ScopeSystem},
		{Path: "/Library/LaunchDaemons", Type: model.TypeLaunchDaemon, Scope: model.ScopeSystem},
	}
	if includeAppleSystem {
		paths = append(paths,
			LaunchdPath{Path: "/System/Library/LaunchAgents", Type: model.TypeLaunchAgent, Scope: model.ScopeSystem},
			LaunchdPath{Path: "/System/Library/LaunchDaemons", Type: model.TypeLaunchDaemon, Scope: model.ScopeSystem},
		)
	}
	return &Launchd{runner: runner, paths: paths}
}

func (c *Launchd) Name() string { return "launchd" }

func (c *Launchd) Collect(ctx context.Context) Result {
	var result Result
	for _, root := range c.paths {
		entries, err := os.ReadDir(root.Path)
		if err != nil {
			if !os.IsNotExist(err) && !os.IsPermission(err) {
				result.Warnings = append(result.Warnings, NewWarning(c.Name(), root.Path, err))
			}
			continue
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, entry := range entries {
			if ctx.Err() != nil {
				result.Warnings = append(result.Warnings, NewWarning(c.Name(), root.Path, ctx.Err()))
				return result
			}
			if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".plist") {
				continue
			}
			path := filepath.Join(root.Path, entry.Name())
			info, err := os.Lstat(path)
			if err != nil {
				result.Warnings = append(result.Warnings, NewWarning(c.Name(), path, err))
				continue
			}
			if !info.Mode().IsRegular() {
				result.Warnings = append(result.Warnings, NewWarning(c.Name(), path, fmt.Errorf("skipped non-regular plist")))
				continue
			}
			if info.Size() > maxPlistSize {
				result.Warnings = append(result.Warnings, NewWarning(c.Name(), path, fmt.Errorf("plist exceeds %d bytes", maxPlistSize)))
				continue
			}
			converted, err := c.runner.Run(ctx, "plutil", "-convert", "json", "-o", "-", "--", path)
			if err != nil {
				result.Warnings = append(result.Warnings, NewWarning(c.Name(), path, err))
				continue
			}
			item, err := parser.ParseLaunchdJSON(converted.Output, path, root.Type, root.Scope)
			if err != nil {
				result.Warnings = append(result.Warnings, NewWarning(c.Name(), path, err))
				continue
			}
			result.Items = append(result.Items, item)
		}
	}
	return result
}
