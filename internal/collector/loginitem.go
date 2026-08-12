package collector

import (
	"context"

	"github.com/wuuJiawei/stoat/internal/executil"
	"github.com/wuuJiawei/stoat/internal/parser"
)

type LoginItems struct {
	runner executil.Runner
}

func NewLoginItems(runner executil.Runner) *LoginItems { return &LoginItems{runner: runner} }

func (c *LoginItems) Name() string { return "login-items" }

func (c *LoginItems) Collect(ctx context.Context) Result {
	dump, err := c.runner.Run(ctx, "sfltool", "dumpbtm")
	if err != nil {
		return Result{Warnings: []Warning{NewWarning(c.Name(), "", err)}}
	}
	return Result{Items: parser.ParseBTMDump(dump.Output)}
}
