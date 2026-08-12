package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wuuJiawei/stoat/internal/collector"
	"github.com/wuuJiawei/stoat/internal/model"
)

type Model struct {
	allItems   []model.PersistenceItem
	items      []model.PersistenceItem
	warnings   []collector.Warning
	loader     Loader
	menuCursor int
	cursor     int
	screen     screen
	loaded     bool
	width      int
	height     int
}

type Loader func() ([]model.PersistenceItem, []collector.Warning)

type screen uint8

const (
	screenMenu screen = iota
	screenLoading
	screenList
	screenDetail
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

func New(items []model.PersistenceItem, warnings []collector.Warning) Model {
	return Model{
		allItems: append([]model.PersistenceItem(nil), items...),
		warnings: append([]collector.Warning(nil), warnings...),
		loaded:   true,
		screen:   screenMenu,
	}
}

func NewWithLoader(loader Loader) Model {
	return Model{loader: loader, screen: screenMenu}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch value := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = value.Width, value.Height
	case loadedMsg:
		m.allItems = append([]model.PersistenceItem(nil), value.items...)
		m.warnings = append([]collector.Warning(nil), value.warnings...)
		m.loaded = true
		m.showSelectedSection()
	case tea.KeyMsg:
		key := value.String()
		if key == "q" || key == "ctrl+c" {
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
			if key == "esc" || key == "backspace" || key == "left" || key == "h" {
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
	default:
		return m.menuView()
	}
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
