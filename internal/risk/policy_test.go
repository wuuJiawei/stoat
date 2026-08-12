package risk

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wuuJiawei/stoat/internal/model"
)

func TestPolicySuppressesOnlyNamedFinding(t *testing.T) {
	item := model.PersistenceItem{
		ID:      "0123456789abcdefabcd",
		Program: "/private/tmp/example.sh",
		Exists:  true,
	}
	policy := Policy{Exceptions: []Exception{{
		ItemID: item.ID, RuleIDs: []string{"script-execution"}, Reason: "reviewed internal script",
	}}}
	NewEngine().WithPolicy(policy).Evaluate(&item)
	if item.RiskScore != 60 {
		t.Fatalf("expected only script rule suppressed, got %d", item.RiskScore)
	}
	foundSuppressed := false
	for _, finding := range item.RiskFindings {
		if finding.RuleID == "script-execution" && finding.Suppressed {
			foundSuppressed = true
		}
	}
	if !foundSuppressed {
		t.Fatalf("suppressed finding not retained for audit: %#v", item.RiskFindings)
	}
}

func TestPublishedSchemaContainsEverySuppressibleRule(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "schemas", "risk-policy.v1.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	for ruleID := range SuppressibleRuleIDs() {
		if !bytes.Contains(data, []byte(`"`+ruleID+`"`)) {
			t.Errorf("published schema is missing rule %s", ruleID)
		}
	}
}

func TestLoadPolicyRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.json")
	data := []byte(`{"schema_version":1,"exceptions":[],"unsafe":true}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPolicy(path, time.Now()); err == nil {
		t.Fatal("expected strict schema rejection")
	}
}

func TestExpiredPolicyExceptionIsIgnored(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	policy, err := validatePolicy(Policy{SchemaVersion: 1, Exceptions: []Exception{{
		ItemID: "0123456789abcdefabcd", RuleIDs: []string{"every-minute"}, Reason: "temporary", ExpiresAt: &past,
	}}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(policy.Exceptions) != 0 || policy.ExpiredExceptions != 1 {
		t.Fatalf("expired exception remained active: %#v", policy)
	}
}
