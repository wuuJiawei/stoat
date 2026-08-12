package main

import (
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

func TestFilterItemsUsesMinimumRiskForSuspicious(t *testing.T) {
	items := []model.PersistenceItem{{RiskLevel: model.RiskNormal}, {RiskLevel: model.RiskAttention}, {RiskLevel: model.RiskHigh}}
	filtered := filterItems(items, "", model.RiskAttention, true)
	if len(filtered) != 2 {
		t.Fatalf("expected attention and high: %#v", filtered)
	}
}
