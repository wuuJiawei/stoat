package tui

import (
	"context"
	"errors"
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

func TestMenuShowsStoatBrandAddressTaglineAndVersion(t *testing.T) {
	m := NewWithLoader(nil, WithVersion("v1.2.0"))
	view := m.View()
	for _, expected := range []string{
		"____", "https://github.com/wuuJiawei/stoat",
		"Inspect and manage what runs on your Mac.", "Version v1.2.0",
	} {
		if !strings.Contains(view, expected) {
			t.Fatalf("brand view missing %q: %q", expected, view)
		}
	}
}

func TestAsyncUpdateCheckShowsNewVersionWithoutBlockingInitialView(t *testing.T) {
	calls := 0
	m := NewWithLoader(nil,
		WithVersion("v1.2.0"),
		WithUpdateChecker(func(ctx context.Context, current string) (string, error) {
			calls++
			if current != "v1.2.0" {
				t.Fatalf("unexpected current version: %q", current)
			}
			return "v1.3.0", nil
		}),
	)
	if strings.Contains(m.View(), "Update v1.3.0") || calls != 0 {
		t.Fatal("update check blocked the initial view")
	}
	command := m.Init()
	if command == nil {
		t.Fatal("expected asynchronous update command")
	}
	updated, _ := updateModel(t, m, command())
	if calls != 1 || !strings.Contains(updated.View(), "Update v1.3.0 available") ||
		!strings.Contains(updated.View(), "run the install command again") {
		t.Fatalf("update notice missing: calls=%d view=%q", calls, updated.View())
	}
}

func TestFailedUpdateCheckStaysSilent(t *testing.T) {
	m := NewWithLoader(nil,
		WithVersion("v1.2.0"),
		WithUpdateChecker(func(context.Context, string) (string, error) {
			return "", errors.New("offline")
		}),
	)
	updated, _ := updateModel(t, m, m.Init()())
	if strings.Contains(updated.View(), "Update ") {
		t.Fatalf("failed check leaked into UI: %q", updated.View())
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

func TestDetailActionMenuConfirmsRemoveBeforeRunning(t *testing.T) {
	item := model.PersistenceItem{
		ID: "agent", Label: "Example Agent", Source: model.SourceLaunchd,
		Type: model.TypeLaunchAgent, ConfigPath: "/tmp/example.plist",
		Categories: []model.Category{model.CategoryStartup},
	}
	runs := 0
	m := NewWithActions(func() ([]model.PersistenceItem, []collector.Warning) {
		return []model.PersistenceItem{item}, nil
	}, func(kind ActionKind, selected model.PersistenceItem) (string, error) {
		runs++
		if kind != ActionRemove || selected.ID != item.ID {
			t.Fatalf("unexpected action: %s %#v", kind, selected)
		}
		return "removed", nil
	})
	loading, command := updateModel(t, m, key(tea.KeyEnter))
	list, _ := updateModel(t, loading, command())
	detail, _ := updateModel(t, list, key(tea.KeyEnter))
	actions, _ := updateModel(t, detail, runeKey('a'))
	if actions.screen != screenActions || len(actions.actions) != 3 {
		t.Fatalf("expected launchd actions: %#v", actions.actions)
	}
	actions, _ = updateModel(t, actions, key(tea.KeyDown))
	actions, _ = updateModel(t, actions, key(tea.KeyDown))
	confirm, _ := updateModel(t, actions, key(tea.KeyEnter))
	if confirm.screen != screenConfirm || runs != 0 {
		t.Fatalf("remove ran before confirmation: screen=%d runs=%d", confirm.screen, runs)
	}
	for _, character := range "REMOVE" {
		confirm, _ = updateModel(t, confirm, runeKey(character))
	}
	applying, command := updateModel(t, confirm, key(tea.KeyEnter))
	if applying.screen != screenApplying || command == nil || runs != 0 {
		t.Fatalf("expected deferred action: screen=%d runs=%d", applying.screen, runs)
	}
	result, _ := updateModel(t, applying, command())
	if result.screen != screenResult || result.resultError != "" || runs != 1 {
		t.Fatalf("unexpected action result: %#v runs=%d", result, runs)
	}
}

func TestAvailableActionsOffersEnableForDisabledJobAndUninstallForAttributedApp(t *testing.T) {
	item := model.PersistenceItem{
		Source: model.SourceLaunchd, Type: model.TypeLaunchAgent,
		Runtime:     model.RuntimeInfo{Disabled: true},
		Attribution: model.AttributionInfo{AppPath: "/Applications/Example.app"},
	}
	actions := availableActions(item)
	if len(actions) != 4 || actions[0].kind != ActionEnable || actions[3].kind != ActionUninstall {
		t.Fatalf("unexpected actions: %#v", actions)
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
