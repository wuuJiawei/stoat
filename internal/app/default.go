package app

import (
	"os"
	"strconv"
	"time"

	"github.com/wuuJiawei/stoat/internal/attribution"
	"github.com/wuuJiawei/stoat/internal/collector"
	"github.com/wuuJiawei/stoat/internal/executil"
	"github.com/wuuJiawei/stoat/internal/risk"
	"github.com/wuuJiawei/stoat/internal/runtimeinfo"
	"github.com/wuuJiawei/stoat/internal/signing"
)

func NewDefaultScanner(home string, includeAppleSystem bool) *Scanner {
	runner := executil.NewExecRunner(3*time.Second, 8<<20)
	collectors := []collector.Collector{
		collector.NewLoginItems(runner),
		collector.NewLaunchd(runner, home, includeAppleSystem),
		collector.NewCron(runner),
	}
	enrichers := []Enricher{
		runtimeinfo.NewInspector(runner, strconv.Itoa(os.Getuid())),
		attribution.NewInspector(runner),
		signing.NewInspector(runner),
	}
	return NewScannerWithEnrichers(collectors, enrichers, risk.NewEngine())
}
