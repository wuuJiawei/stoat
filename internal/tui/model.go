package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wuuJiawei/stoat/internal/collector"
	"github.com/wuuJiawei/stoat/internal/model"
)

type Model struct {
	allItems       []model.PersistenceItem
	items          []model.PersistenceItem
	warnings       []collector.Warning
	loader         Loader
	action         ActionHandler
	menuCursor     int
	cursor         int
	actionCursor   int
	actions        []actionDefinition
	pending        actionDefinition
	confirmText    string
	resultMessage  string
	resultError    string
	currentVersion string
	latestVersion  string
	updateChecker  UpdateChecker
	screen         screen
	loaded         bool
	width          int
	height         int
}

type Loader func() ([]model.PersistenceItem, []collector.Warning)

type ActionKind string

const (
	ActionDisable    ActionKind = "disable"
	ActionEnable     ActionKind = "enable"
	ActionQuarantine ActionKind = "quarantine"
	ActionRemove     ActionKind = "remove"
	ActionUninstall  ActionKind = "uninstall"
)

type ActionHandler func(ActionKind, model.PersistenceItem) (string, error)

type UpdateChecker func(context.Context, string) (string, error)

type Option func(*Model)

func WithVersion(version string) Option {
	return func(model *Model) {
		model.currentVersion = version
	}
}

func WithUpdateChecker(checker UpdateChecker) Option {
	return func(model *Model) {
		model.updateChecker = checker
	}
}

type screen uint8

const (
	screenMenu screen = iota
	screenLoading
	screenList
	screenDetail
	screenActions
	screenConfirm
	screenApplying
	screenResult
)

type section uint8

const (
	sectionStartup section = iota
	sectionScheduled
	sectionBackground
	sectionSuspicious
	sectionAll
)

type sectionDefinition struct {
	id          section
	title       string
	description string
}

var sectionDefinitions = []sectionDefinition{
	{id: sectionStartup, title: "Startup Items", description: "Runs at login or system startup"},
	{id: sectionScheduled, title: "Scheduled Tasks", description: "Runs on a timer or calendar schedule"},
	{id: sectionBackground, title: "Background Items", description: "Runs or remains active in the background"},
	{id: sectionSuspicious, title: "Suspicious Items", description: "Requires attention or has high risk"},
	{id: sectionAll, title: "All Items", description: "Every detected persistence item"},
}

type loadedMsg struct {
	items    []model.PersistenceItem
	warnings []collector.Warning
}

type actionDefinition struct {
	kind        ActionKind
	title       string
	description string
	confirmWord string
}

type actionCompletedMsg struct {
	message  string
	err      error
	items    []model.PersistenceItem
	warnings []collector.Warning
}

type updateCheckedMsg struct {
	latest string
}

func New(items []model.PersistenceItem, warnings []collector.Warning, options ...Option) Model {
	result := Model{
		allItems: append([]model.PersistenceItem(nil), items...),
		warnings: append([]collector.Warning(nil), warnings...),
		loaded:   true,
		screen:   screenMenu,
	}
	applyOptions(&result, options)
	return result
}

func NewWithLoader(loader Loader, options ...Option) Model {
	result := Model{loader: loader, screen: screenMenu}
	applyOptions(&result, options)
	return result
}

func NewWithActions(loader Loader, handler ActionHandler, options ...Option) Model {
	result := Model{loader: loader, action: handler, screen: screenMenu}
	applyOptions(&result, options)
	return result
}

func applyOptions(model *Model, options []Option) {
	for _, option := range options {
		if option != nil {
			option(model)
		}
	}
}

func (m Model) Init() tea.Cmd {
	if m.updateChecker == nil || m.currentVersion == "" || m.currentVersion == "dev" {
		return nil
	}
	checker := m.updateChecker
	current := m.currentVersion
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		latest, err := checker(ctx, current)
		if err != nil {
			return updateCheckedMsg{}
		}
		return updateCheckedMsg{latest: latest}
	}
}

func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch value := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = value.Width, value.Height
	case loadedMsg:
		m.allItems = append([]model.PersistenceItem(nil), value.items...)
		m.warnings = append([]collector.Warning(nil), value.warnings...)
		m.loaded = true
		m.showSelectedSection()
	case actionCompletedMsg:
		m.resultMessage = value.message
		m.resultError = ""
		if value.err != nil {
			m.resultError = value.err.Error()
		} else {
			m.allItems = append([]model.PersistenceItem(nil), value.items...)
			m.warnings = append([]collector.Warning(nil), value.warnings...)
			m.items = filterSection(m.allItems, sectionDefinitions[m.menuCursor].id)
			if m.cursor >= len(m.items) && m.cursor > 0 {
				m.cursor--
			}
		}
		m.screen = screenResult
	case updateCheckedMsg:
		m.latestVersion = value.latest
	case tea.KeyMsg:
		key := value.String()
		if key == "ctrl+c" || (key == "q" && m.screen != screenApplying) {
			return m, tea.Quit
		}
		switch m.screen {
		case screenMenu:
			switch key {
			case "up", "k":
				if m.menuCursor > 0 {
					m.menuCursor--
				}
			case "down", "j":
				if m.menuCursor+1 < len(sectionDefinitions) {
					m.menuCursor++
				}
			case "1", "2", "3", "4", "5":
				m.menuCursor = int(key[0] - '1')
				return m.openSelectedSection()
			case "enter", "right", "l":
				return m.openSelectedSection()
			}
		case screenList:
			switch key {
			case "esc", "backspace", "left", "h":
				m.screen = screenMenu
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if m.cursor+1 < len(m.items) {
					m.cursor++
				}
			case "enter", "right", "l":
				if len(m.items) > 0 {
					m.screen = screenDetail
				}
			}
		case screenDetail:
			switch key {
			case "esc", "backspace", "left", "h":
				m.screen = screenList
			case "a":
				if m.action != nil && len(m.items) > 0 {
					m.actions = availableActions(m.items[m.cursor])
					m.actionCursor = 0
					m.screen = screenActions
				}
			}
		case screenActions:
			switch key {
			case "esc", "backspace", "left", "h":
				m.screen = screenDetail
			case "up", "k":
				if m.actionCursor > 0 {
					m.actionCursor--
				}
			case "down", "j":
				if m.actionCursor+1 < len(m.actions) {
					m.actionCursor++
				}
			case "enter", "right", "l":
				if len(m.actions) > 0 {
					m.pending = m.actions[m.actionCursor]
					m.confirmText = ""
					m.screen = screenConfirm
				}
			}
		case screenConfirm:
			switch key {
			case "esc":
				m.screen = screenActions
			case "backspace":
				characters := []rune(m.confirmText)
				if len(characters) > 0 {
					m.confirmText = string(characters[:len(characters)-1])
				}
			case "y":
				if m.pending.confirmWord == "" {
					return m.startAction()
				}
			case "enter":
				if m.pending.confirmWord != "" && m.confirmText == m.pending.confirmWord {
					return m.startAction()
				}
			default:
				if m.pending.confirmWord != "" && len(value.Runes) > 0 {
					candidate := strings.ToUpper(m.confirmText + string(value.Runes))
					if strings.HasPrefix(m.pending.confirmWord, candidate) {
						m.confirmText = candidate
					}
				}
			}
		case screenResult:
			if key == "enter" || key == "esc" || key == "backspace" || key == "left" || key == "h" {
				m.screen = screenList
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	switch m.screen {
	case screenLoading:
		return m.loadingView()
	case screenList:
		return m.listView()
	case screenDetail:
		if len(m.items) == 0 {
			return m.listView()
		}
		return m.detailView(m.items[m.cursor])
	case screenActions:
		return m.actionsView()
	case screenConfirm:
		return m.confirmView()
	case screenApplying:
		return m.applyingView()
	case screenResult:
		return m.resultView()
	default:
		return m.menuView()
	}
}

func (m Model) startAction() (tea.Model, tea.Cmd) {
	if m.action == nil || len(m.items) == 0 {
		return m, nil
	}
	item := m.items[m.cursor]
	handler := m.action
	loader := m.loader
	kind := m.pending.kind
	m.screen = screenApplying
	return m, func() tea.Msg {
		message, err := handler(kind, item)
		if err != nil {
			return actionCompletedMsg{message: message, err: err}
		}
		var items []model.PersistenceItem
		var warnings []collector.Warning
		if loader != nil {
			items, warnings = loader()
		}
		return actionCompletedMsg{message: message, items: items, warnings: warnings}
	}
}

func availableActions(item model.PersistenceItem) []actionDefinition {
	if item.Source != model.SourceLaunchd || (item.Type != model.TypeLaunchAgent && item.Type != model.TypeLaunchDaemon) {
		return nil
	}
	state := actionDefinition{kind: ActionDisable, title: "Disable", description: "Stop the job and prevent it from loading"}
	if item.Runtime.Disabled {
		state = actionDefinition{kind: ActionEnable, title: "Enable", description: "Enable and load the job"}
	}
	actions := []actionDefinition{
		state,
		{kind: ActionQuarantine, title: "Quarantine", description: "Disable and move the configuration aside"},
		{kind: ActionRemove, title: "Remove startup item", description: "Delete the configuration; Stoat keeps a restorable backup", confirmWord: "REMOVE"},
	}
	appPath := item.Attribution.AppPath
	if appPath == "" {
		appPath = item.AppPath
	}
	if strings.HasSuffix(strings.ToLower(appPath), ".app") {
		actions = append(actions, actionDefinition{
			kind: ActionUninstall, title: "Uninstall application", description: "Remove the startup item and move its attributed app to Trash", confirmWord: "UNINSTALL",
		})
	}
	return actions
}

func (m Model) openSelectedSection() (tea.Model, tea.Cmd) {
	if m.loaded {
		m.showSelectedSection()
		return m, nil
	}
	m.screen = screenLoading
	loader := m.loader
	return m, func() tea.Msg {
		if loader == nil {
			return loadedMsg{}
		}
		items, warnings := loader()
		return loadedMsg{items: items, warnings: warnings}
	}
}

func (m *Model) showSelectedSection() {
	m.items = filterSection(m.allItems, sectionDefinitions[m.menuCursor].id)
	m.cursor = 0
	m.screen = screenList
}

func filterSection(items []model.PersistenceItem, selected section) []model.PersistenceItem {
	filtered := make([]model.PersistenceItem, 0, len(items))
	for _, item := range items {
		if matchesSection(item, selected) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func matchesSection(item model.PersistenceItem, selected section) bool {
	switch selected {
	case sectionStartup:
		return item.HasCategory(model.CategoryStartup)
	case sectionScheduled:
		return item.HasCategory(model.CategoryScheduled)
	case sectionBackground:
		return item.HasCategory(model.CategoryBackground)
	case sectionSuspicious:
		return item.RiskLevel == model.RiskAttention || item.RiskLevel == model.RiskHigh
	case sectionAll:
		return true
	default:
		return false
	}
}

func categories(item model.PersistenceItem) string {
	values := make([]string, len(item.Categories))
	for index, category := range item.Categories {
		values[index] = string(category)
	}
	return strings.Join(values, ", ")
}

func schedule(item model.PersistenceItem) string {
	if len(item.Schedules) == 0 {
		return "—"
	}
	return item.Schedules[0].Description
}

func signature(item model.PersistenceItem) string {
	if !item.Signature.Checked {
		return "Not checked"
	}
	if !item.Signature.Signed {
		return "Unsigned"
	}
	if item.Signature.Signer != "" {
		return "Signed · " + item.Signature.Signer
	}
	return "Signed"
}

func runtimeStatus(item model.PersistenceItem) string {
	if !item.Runtime.Checked {
		return "Not checked"
	}
	if item.Runtime.Disabled {
		return "Disabled"
	}
	if item.Runtime.Running {
		if item.Runtime.PID > 0 {
			return fmt.Sprintf("Running · PID %d", item.Runtime.PID)
		}
		return "Running"
	}
	if item.Runtime.Loaded {
		return "Loaded · " + emptyDash(item.Runtime.State)
	}
	return "Not loaded"
}

func attribution(item model.PersistenceItem) string {
	if !item.Attribution.Checked {
		return "Not checked"
	}
	if item.Attribution.Name != "" {
		return item.Attribution.Name + " · " + emptyDash(item.Attribution.BundleID)
	}
	if item.Attribution.BundleID != "" {
		return item.Attribution.BundleID
	}
	return "Unattributed"
}

func field(label, value string) string {
	return fmt.Sprintf("%-13s %s", label, value)
}
