package app

import (
	"context"
	"testing"

	"github.com/wuuJiawei/stoat/internal/collector"
	"github.com/wuuJiawei/stoat/internal/model"
	"github.com/wuuJiawei/stoat/internal/risk"
)

type staticCollector struct {
	items []model.PersistenceItem
}

func (c staticCollector) Name() string { return "static" }

func (c staticCollector) Collect(context.Context) collector.Result {
	return collector.Result{Items: append([]model.PersistenceItem(nil), c.items...)}
}

func TestScannerDeduplicatesAndMergesCategories(t *testing.T) {
	items := []model.PersistenceItem{
		{ID: "same", Label: "job", Categories: []model.Category{model.CategoryStartup}},
		{ID: "same", Label: "job", Categories: []model.Category{model.CategoryBackground}},
	}
	report := NewScanner([]collector.Collector{staticCollector{items: items}}, nil, risk.NewEngine()).Scan(context.Background())
	if len(report.Items) != 1 {
		t.Fatalf("expected one item, got %d", len(report.Items))
	}
	if !report.Items[0].HasCategory(model.CategoryStartup) || !report.Items[0].HasCategory(model.CategoryBackground) {
		t.Fatalf("categories were not merged: %#v", report.Items[0].Categories)
	}
}
