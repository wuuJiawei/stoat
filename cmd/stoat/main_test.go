package main

import (
	"bytes"
	"testing"

	"github.com/wuuJiawei/stoat/internal/model"
)

func TestFilterItemsUsesExactRiskForFlag(t *testing.T) {
	items := []model.PersistenceItem{{RiskLevel: model.RiskAttention}, {RiskLevel: model.RiskHigh}}
	filtered := filterItems(items, "", model.RiskAttention, false)
	if len(filtered) != 1 || filtered[0].RiskLevel != model.RiskAttention {
		t.Fatalf("expected exact attention match: %#v", filtered)
	}
}

func TestParseDiffOptions(t *testing.T) {
	options, err := parseOptions("diff", []string{"--json", "before.json", "after.json"}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if !options.jsonOutput || options.beforePath != "before.json" || options.afterPath != "after.json" {
		t.Fatalf("unexpected diff options: %#v", options)
	}
}

func TestUnknownCommandFailsBeforePlatformCheck(t *testing.T) {
	err := run([]string{"unknown"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || err.Error() != `unknown command "unknown"` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFilterItemsUsesMinimumRiskForSuspicious(t *testing.T) {
	items := []model.PersistenceItem{{RiskLevel: model.RiskNormal}, {RiskLevel: model.RiskAttention}, {RiskLevel: model.RiskHigh}}
	filtered := filterItems(items, "", model.RiskAttention, true)
	if len(filtered) != 2 {
		t.Fatalf("expected attention and high: %#v", filtered)
	}
}
