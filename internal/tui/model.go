package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wuuJiawei/stoat/internal/collector"
	"github.com/wuuJiawei/stoat/internal/model"
)

type Model struct {
	items    []model.PersistenceItem
	warnings []collector.Warning
	cursor   int
	detail   bool
	width    int
	height   int
}

func New(items []model.PersistenceItem, warnings []collector.Warning) Model {
	return Model{items: append([]model.PersistenceItem(nil), items...), warnings: append([]collector.Warning(nil), warnings...)}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch value := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = value.Width, value.Height
	case tea.KeyMsg:
		switch value.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc", "backspace":
			m.detail = false
		case "up", "k":
			if !m.detail && m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if !m.detail && m.cursor+1 < len(m.items) {
				m.cursor++
			}
		case "enter", "right", "l":
			if len(m.items) > 0 {
				m.detail = true
			}
		case "left", "h":
			m.detail = false
		}
	}
	return m, nil
}

func (m Model) View() string {
	if m.detail && len(m.items) > 0 {
		return m.detailView(m.items[m.cursor])
	}
	return m.listView()
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

func field(label, value string) string {
	return fmt.Sprintf("%-13s %s", label, value)
}
