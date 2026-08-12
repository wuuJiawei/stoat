package risk

import (
	"testing"

	"github.com/wuuJiawei/stoat/internal/model"
)

func TestHighRiskRootDaemonInTemporaryDirectory(t *testing.T) {
	item := model.PersistenceItem{
		Type:      model.TypeLaunchDaemon,
		User:      "root",
		Program:   "/private/tmp/.update",
		Exists:    true,
		Signature: model.SignatureInfo{Checked: true, Signed: false},
		Schedules: []model.Schedule{{Expression: "* * * * *"}},
	}
	NewEngine().Evaluate(&item)
	if item.RiskLevel != model.RiskHigh || item.RiskScore != 100 {
		t.Fatalf("expected high risk 100, got %s %d", item.RiskLevel, item.RiskScore)
	}
	if len(item.RiskReasons) < 3 {
		t.Fatalf("expected reasons, got %#v", item.RiskReasons)
	}
}

func TestAppleSystemItemIsTrusted(t *testing.T) {
	item := model.PersistenceItem{
		Program:    "/System/Library/CoreServices/example",
		ConfigPath: "/System/Library/LaunchAgents/com.apple.example.plist",
		Exists:     true,
		Signature:  model.SignatureInfo{Checked: true, Signed: true, AppleSigned: true},
	}
	NewEngine().Evaluate(&item)
	if item.RiskLevel != model.RiskTrusted || item.RiskScore != 0 {
		t.Fatalf("expected trusted 0, got %s %d", item.RiskLevel, item.RiskScore)
	}
}
