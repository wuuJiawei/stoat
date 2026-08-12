package main

import (
	"bytes"
	"testing"
	"time"

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

func TestParseMutationOptions(t *testing.T) {
	options, err := parseOptions("disable", []string{"--confirm", "token", "item"}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if options.query != "item" || options.confirm != "token" {
		t.Fatalf("unexpected mutation options: %#v", options)
	}
}

func TestParseDeleteAliasOptions(t *testing.T) {
	options, err := parseOptions("delete", []string{"item", "--confirm", "token"}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if options.query != "item" || options.confirm != "token" || !knownCommand("delete") {
		t.Fatalf("unexpected delete options: %#v", options)
	}
}

func TestParseOptionsAfterIdentifier(t *testing.T) {
	options, err := parseOptions("diagnose", []string{"item", "--json", "--last", "2m", "--limit", "5"}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if options.query != "item" || !options.jsonOutput || options.logPeriod != 2*time.Minute || options.logLimit != 5 {
		t.Fatalf("unexpected interspersed options: %#v", options)
	}
}

func TestFindItemRejectsAmbiguousLabel(t *testing.T) {
	items := []model.PersistenceItem{{ID: "one", Label: "same"}, {ID: "two", Label: "same"}}
	if _, err := findItem(items, "same"); err == nil {
		t.Fatal("expected ambiguous label error")
	}
	item, err := findItem(items, "two")
	if err != nil || item.ID != "two" {
		t.Fatalf("id lookup failed: %#v %v", item, err)
	}
}

func TestParseWatchAndChangesOptions(t *testing.T) {
	watch, err := parseOptions("watch", []string{"--once", "--interval", "10s"}, &bytes.Buffer{})
	if err != nil || !watch.once || watch.interval != 10*time.Second {
		t.Fatalf("unexpected watch options: %#v %v", watch, err)
	}
	changes, err := parseOptions("changes", []string{"--limit", "12", "--json"}, &bytes.Buffer{})
	if err != nil || changes.eventLimit != 12 || !changes.jsonOutput {
		t.Fatalf("unexpected changes options: %#v %v", changes, err)
	}
}

func TestTerminalTextRemovesControlCharacters(t *testing.T) {
	if value := terminalText("ok\n\x1b[31m"); value != "ok  [31m" {
		t.Fatalf("unexpected terminal-safe text: %q", value)
	}
}
