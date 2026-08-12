package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wuuJiawei/stoat/internal/collector"
	"github.com/wuuJiawei/stoat/internal/model"
)

func TestLazyModelShowsCategoryMenuBeforeScanning(t *testing.T) {
	loads := 0
	m := NewWithLoader(func() ([]model.PersistenceItem, []collector.Warning) {
		loads++
		return testItems(), nil
	})

	view := m.View()
	if loads != 0 {
		t.Fatal("loader ran before a category was selected")
	}
	if !strings.Contains(view, "Choose what you want to inspect") || !strings.Contains(view, "Startup Items") {
		t.Fatalf("category menu missing from initial view: %q", view)
	}
	if strings.Contains(view, "Login Helper") {
		t.Fatalf("item leaked into category menu: %q", view)
	}
}

func TestSelectingCategoryLoadsOnceAndFiltersItems(t *testing.T) {
	loads := 0
	m := NewWithLoader(func() ([]model.PersistenceItem, []collector.Warning) {
		loads++
		return testItems(), nil
	})

	loading, command := updateModel(t, m, key(tea.KeyEnter))
	if loading.screen != screenLoading || command == nil {
		t.Fatalf("expected loading screen and command: %#v", loading)
	}
	loaded, _ := updateModel(t, loading, command())
	if loads != 1 || loaded.screen != screenList {
		t.Fatalf("expected one completed load: loads=%d screen=%d", loads, loaded.screen)
	}
	if len(loaded.items) != 1 || loaded.items[0].Label != "Login Helper" {
		t.Fatalf("startup category was not filtered: %#v", loaded.items)
	}

	menu, _ := updateModel(t, loaded, key(tea.KeyEsc))
	scheduled, command := updateModel(t, menu, runeKey('2'))
	if command != nil || loads != 1 {
		t.Fatalf("cached scan should be reused: loads=%d", loads)
	}
	if len(scheduled.items) != 1 || scheduled.items[0].Label != "Nightly Task" {
		t.Fatalf("scheduled category was not filtered: %#v", scheduled.items)
	}
}

func TestNavigationReturnsFromDetailToListThenMenu(t *testing.T) {
	m := New(testItems(), nil)
	list, _ := updateModel(t, m, key(tea.KeyEnter))
	detail, _ := updateModel(t, list, key(tea.KeyEnter))
	if detail.screen != screenDetail {
		t.Fatalf("expected detail screen, got %d", detail.screen)
	}
	list, _ = updateModel(t, detail, key(tea.KeyEsc))
	if list.screen != screenList {
		t.Fatalf("expected list screen, got %d", list.screen)
	}
	menu, _ := updateModel(t, list, key(tea.KeyEsc))
	if menu.screen != screenMenu {
		t.Fatalf("expected category menu, got %d", menu.screen)
	}
}

func testItems() []model.PersistenceItem {
	return []model.PersistenceItem{
		{ID: "startup", Label: "Login Helper", Categories: []model.Category{model.CategoryStartup}, RiskLevel: model.RiskNormal},
		{ID: "scheduled", Label: "Nightly Task", Categories: []model.Category{model.CategoryScheduled}, RiskLevel: model.RiskNormal},
		{ID: "background", Label: "Unknown Agent", Categories: []model.Category{model.CategoryBackground}, RiskLevel: model.RiskHigh},
	}
}

func updateModel(t *testing.T, current Model, message tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	updated, command := current.Update(message)
	result, ok := updated.(Model)
	if !ok {
		t.Fatalf("unexpected model type %T", updated)
	}
	return result, command
}

func key(keyType tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: keyType}
}

func runeKey(value rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{value}}
}
