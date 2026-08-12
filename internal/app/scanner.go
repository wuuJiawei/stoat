package app

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/wuuJiawei/stoat/internal/collector"
	"github.com/wuuJiawei/stoat/internal/model"
	"github.com/wuuJiawei/stoat/internal/risk"
	"github.com/wuuJiawei/stoat/internal/signing"
)

type Report struct {
	SchemaVersion int                     `json:"schema_version"`
	GeneratedAt   time.Time               `json:"generated_at"`
	ToolVersion   string                  `json:"tool_version,omitempty"`
	Items         []model.PersistenceItem `json:"items"`
	Warnings      []collector.Warning     `json:"warnings,omitempty"`
}

type Enricher interface {
	Name() string
	Enrich(context.Context, *model.PersistenceItem) error
}

type preparer interface {
	Prepare(context.Context) error
}

type Scanner struct {
	collectors []collector.Collector
	enrichers  []Enricher
	risk       *risk.Engine
	workers    int
}

func NewScanner(collectors []collector.Collector, inspector *signing.Inspector, riskEngine *risk.Engine) *Scanner {
	var enrichers []Enricher
	if inspector != nil {
		enrichers = append(enrichers, inspector)
	}
	return NewScannerWithEnrichers(collectors, enrichers, riskEngine)
}

func NewScannerWithEnrichers(collectors []collector.Collector, enrichers []Enricher, riskEngine *risk.Engine) *Scanner {
	return &Scanner{
		collectors: append([]collector.Collector(nil), collectors...),
		enrichers:  append([]Enricher(nil), enrichers...),
		risk:       riskEngine,
		workers:    4,
	}
}

func (s *Scanner) Scan(ctx context.Context) Report {
	report := Report{SchemaVersion: 1, GeneratedAt: time.Now().UTC()}
	for _, source := range s.collectors {
		if ctx.Err() != nil {
			break
		}
		result := source.Collect(ctx)
		report.Items = append(report.Items, result.Items...)
		report.Warnings = append(report.Warnings, result.Warnings...)
	}
	report.Items = deduplicate(report.Items)
	for _, enricher := range s.enrichers {
		if setup, ok := enricher.(preparer); ok {
			if err := setup.Prepare(ctx); err != nil {
				report.Warnings = append(report.Warnings, collector.NewWarning(enricher.Name(), "", err))
			}
		}
		report.Warnings = append(report.Warnings, s.enrich(ctx, enricher, report.Items)...)
	}
	for index := range report.Items {
		s.risk.Evaluate(&report.Items[index])
	}
	sort.SliceStable(report.Items, func(i, j int) bool {
		if report.Items[i].RiskScore == report.Items[j].RiskScore {
			return report.Items[i].Label < report.Items[j].Label
		}
		return report.Items[i].RiskScore > report.Items[j].RiskScore
	})
	return report
}

func (s *Scanner) enrich(ctx context.Context, enricher Enricher, items []model.PersistenceItem) []collector.Warning {
	if enricher == nil || len(items) == 0 {
		return nil
	}
	workers := s.workers
	if workers > len(items) {
		workers = len(items)
	}
	jobs := make(chan int)
	warnings := make(chan collector.Warning, len(items))
	var group sync.WaitGroup
	group.Add(workers)
	for range workers {
		go func() {
			defer group.Done()
			for index := range jobs {
				if ctx.Err() != nil {
					return
				}
				if err := enricher.Enrich(ctx, &items[index]); err != nil {
					warnings <- collector.NewWarning(enricher.Name(), items[index].Program, err)
				}
			}
		}()
	}
sendLoop:
	for index := range items {
		select {
		case jobs <- index:
		case <-ctx.Done():
			break sendLoop
		}
	}
	close(jobs)
	group.Wait()
	close(warnings)
	result := make([]collector.Warning, 0, len(warnings))
	for warning := range warnings {
		result = append(result, warning)
	}
	return result
}

func deduplicate(items []model.PersistenceItem) []model.PersistenceItem {
	byID := make(map[string]int, len(items))
	result := make([]model.PersistenceItem, 0, len(items))
	for _, item := range items {
		if index, exists := byID[item.ID]; exists {
			result[index].Categories = mergeCategories(result[index].Categories, item.Categories)
			continue
		}
		byID[item.ID] = len(result)
		result = append(result, item)
	}
	return result
}

func mergeCategories(left, right []model.Category) []model.Category {
	result := append([]model.Category(nil), left...)
	for _, category := range right {
		result = model.AddCategory(result, category)
	}
	return result
}
