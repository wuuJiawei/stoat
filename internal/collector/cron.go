package collector

import (
	"context"
	"fmt"
	"os"

	"github.com/wuuJiawei/stoat/internal/executil"
	"github.com/wuuJiawei/stoat/internal/model"
	"github.com/wuuJiawei/stoat/internal/parser"
)

const maxCronSize = 1 << 20

type Cron struct {
	runner executil.Runner
}

func NewCron(runner executil.Runner) *Cron { return &Cron{runner: runner} }

func (c *Cron) Name() string { return "cron" }

func (c *Cron) Collect(ctx context.Context) Result {
	var result Result
	userCrontab, err := c.runner.Run(ctx, "crontab", "-l")
	if err == nil {
		items, parseErrors := parser.ParseCron(userCrontab.Output, model.ScopeUser, "user:crontab", false)
		result.Items = append(result.Items, items...)
		for _, parseErr := range parseErrors {
			result.Warnings = append(result.Warnings, NewWarning(c.Name(), "user:crontab", parseErr))
		}
	} else if len(userCrontab.Output) > 0 && !containsNoCrontab(userCrontab.Output) {
		result.Warnings = append(result.Warnings, NewWarning(c.Name(), "user:crontab", err))
	}

	const systemCrontab = "/etc/crontab"
	data, err := readBoundedRegularFile(systemCrontab, maxCronSize)
	if err == nil {
		items, parseErrors := parser.ParseCron(data, model.ScopeSystem, systemCrontab, true)
		result.Items = append(result.Items, items...)
		for _, parseErr := range parseErrors {
			result.Warnings = append(result.Warnings, NewWarning(c.Name(), systemCrontab, parseErr))
		}
	} else if !os.IsNotExist(err) && !os.IsPermission(err) {
		result.Warnings = append(result.Warnings, NewWarning(c.Name(), systemCrontab, err))
	}
	return result
}

func readBoundedRegularFile(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("refusing non-regular file")
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("file exceeds %d bytes", limit)
	}
	return os.ReadFile(path)
}

func containsNoCrontab(output []byte) bool {
	text := string(output)
	return containsFold(text, "no crontab for") || containsFold(text, "no crontab")
}

func containsFold(value, fragment string) bool {
	if len(fragment) > len(value) {
		return false
	}
	for index := 0; index+len(fragment) <= len(value); index++ {
		if equalFoldASCII(value[index:index+len(fragment)], fragment) {
			return true
		}
	}
	return false
}

func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for index := range a {
		left, right := a[index], b[index]
		if left >= 'A' && left <= 'Z' {
			left += 'a' - 'A'
		}
		if right >= 'A' && right <= 'Z' {
			right += 'a' - 'A'
		}
		if left != right {
			return false
		}
	}
	return true
}
