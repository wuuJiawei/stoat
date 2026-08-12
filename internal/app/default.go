package app

import (
	"time"

	"github.com/wuuJiawei/stoat/internal/collector"
	"github.com/wuuJiawei/stoat/internal/executil"
	"github.com/wuuJiawei/stoat/internal/risk"
	"github.com/wuuJiawei/stoat/internal/signing"
)

func NewDefaultScanner(home string, includeAppleSystem bool) *Scanner {
	runner := executil.NewExecRunner(3*time.Second, 8<<20)
	collectors := []collector.Collector{
		collector.NewLoginItems(runner),
		collector.NewLaunchd(runner, home, includeAppleSystem),
		collector.NewCron(runner),
	}
	return NewScanner(collectors, signing.NewInspector(runner), risk.NewEngine())
}
