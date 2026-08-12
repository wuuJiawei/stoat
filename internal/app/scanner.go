package app

import (
	"context"
	"sort"
	"sync"

	"github.com/wuuJiawei/stoat/internal/collector"
	"github.com/wuuJiawei/stoat/internal/model"
	"github.com/wuuJiawei/stoat/internal/risk"
	"github.com/wuuJiawei/stoat/internal/signing"
)

type Report struct {
	Items    []model.PersistenceItem `json:"items"`
	Warnings []collector.Warning     `json:"warnings,omitempty"`
}

type Scanner struct {
	collectors []collector.Collector
	inspector  *signing.Inspector
	risk       *risk.Engine
	workers    int
}

func NewScanner(collectors []collector.Collector, inspector *signing.Inspector, riskEngine *risk.Engine) *Scanner {
	return &Scanner{collectors: append([]collector.Collector(nil), collectors...), inspector: inspector, risk: riskEngine, workers: 4}
}

func (s *Scanner) Scan(ctx context.Context) Report {
	report := Report{}
	for _, source := range s.collectors {
		if ctx.Err() != nil {
			break
		}
		result := source.Collect(ctx)
		report.Items = append(report.Items, result.Items...)
		report.Warnings = append(report.Warnings, result.Warnings...)
	}
	report.Items = deduplicate(report.Items)
	report.Warnings = append(report.Warnings, s.enrich(ctx, report.Items)...)
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

func (s *Scanner) enrich(ctx context.Context, items []model.PersistenceItem) []collector.Warning {
	if s.inspector == nil || len(items) == 0 {
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
				if err := s.inspector.Enrich(ctx, &items[index]); err != nil {
					warnings <- collector.NewWarning("file-inspector", items[index].Program, err)
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
