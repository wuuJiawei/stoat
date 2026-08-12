package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/wuuJiawei/stoat/internal/model"
)

var (
	titleStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E8E8E8"))
	mutedStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#777777"))
	selectedStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F2D675"))
	highStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF6B6B"))
	attentionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#F2B84B"))
	trustedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#7BC99B"))
)

func (m Model) listView() string {
	var output strings.Builder
	selected := sectionDefinitions[m.menuCursor]
	output.WriteString(titleStyle.Render("Stoat · " + selected.title))
	output.WriteString("\n")
	output.WriteString(mutedStyle.Render(fmt.Sprintf("%d item(s) · %s", len(m.items), selected.description)))
	output.WriteString("\n\n")
	if len(m.items) == 0 {
		output.WriteString("No items found in this category.\n")
	}
	visible := m.height - 7
	if visible < 5 {
		visible = 10
	}
	start := 0
	if m.cursor >= visible {
		start = m.cursor - visible + 1
	}
	end := start + visible
	if end > len(m.items) {
		end = len(m.items)
	}
	for index := start; index < end; index++ {
		item := m.items[index]
		marker := "  "
		style := lipgloss.NewStyle()
		if index == m.cursor {
			marker = "› "
			style = selectedStyle
		}
		line := fmt.Sprintf("%-36s %-16s %3d %s", truncate(item.Label, 34), string(item.Type), item.RiskScore, strings.ToUpper(string(item.RiskLevel)))
		output.WriteString(marker + style.Render(line) + "\n")
	}
	if len(m.warnings) > 0 {
		output.WriteString("\n" + attentionStyle.Render(fmt.Sprintf("%d partial scan warning(s)", len(m.warnings))))
	}
	output.WriteString("\n" + mutedStyle.Render("↑↓/jk navigate  Enter detail  Esc categories  q quit"))
	return output.String()
}

func (m Model) menuView() string {
	var output strings.Builder
	output.WriteString(titleStyle.Render("Stoat · macOS Persistence Inspector"))
	output.WriteString("\n")
	output.WriteString(mutedStyle.Render("Choose what you want to inspect"))
	output.WriteString("\n\n")
	for index, section := range sectionDefinitions {
		marker := "  "
		style := lipgloss.NewStyle()
		if index == m.menuCursor {
			marker = "› "
			style = selectedStyle
		}
		count := ""
		if m.loaded {
			count = fmt.Sprintf("%3d", len(filterSection(m.allItems, section.id)))
		}
		line := fmt.Sprintf("%d  %-20s %s", index+1, section.title, count)
		output.WriteString(marker + style.Render(line) + "\n")
		output.WriteString("   " + mutedStyle.Render(section.description) + "\n")
	}
	if m.loaded && len(m.warnings) > 0 {
		output.WriteString("\n" + attentionStyle.Render(fmt.Sprintf("%d partial scan warning(s)", len(m.warnings))))
	}
	output.WriteString("\n" + mutedStyle.Render("↑↓/jk navigate  1–5/Enter open  q quit"))
	return output.String()
}

func (m Model) loadingView() string {
	selected := sectionDefinitions[m.menuCursor]
	return titleStyle.Render("Stoat · "+selected.title) + "\n\n" +
		"Scanning macOS persistence items…\n\n" +
		mutedStyle.Render("q quit")
}

func (m Model) detailView(item model.PersistenceItem) string {
	var output strings.Builder
	output.WriteString(titleStyle.Render(item.Label))
	output.WriteString("\n\n")
	output.WriteString(field("Type", string(item.Type)) + "\n")
	output.WriteString(field("Scope", string(item.Scope)) + "\n")
	output.WriteString(field("Categories", categories(item)) + "\n")
	output.WriteString(field("Schedule", schedule(item)) + "\n")
	output.WriteString(field("Executable", emptyDash(item.Program)) + "\n")
	output.WriteString(field("Application", attribution(item)) + "\n")
	output.WriteString(field("Runtime", runtimeStatus(item)) + "\n")
	output.WriteString(field("Source", emptyDash(item.ConfigPath)) + "\n")
	output.WriteString(field("Signature", signature(item)) + "\n\n")
	riskLabel := fmt.Sprintf("%d · %s", item.RiskScore, strings.ToUpper(string(item.RiskLevel)))
	switch item.RiskLevel {
	case model.RiskHigh:
		riskLabel = highStyle.Render(riskLabel)
	case model.RiskAttention:
		riskLabel = attentionStyle.Render(riskLabel)
	case model.RiskTrusted:
		riskLabel = trustedStyle.Render(riskLabel)
	}
	output.WriteString(field("Risk", riskLabel) + "\n")
	for _, finding := range item.RiskFindings {
		if finding.Suppressed {
			output.WriteString(mutedStyle.Render("  ○ "+finding.Reason+" [suppressed: "+finding.SuppressionReason+"]") + "\n")
			continue
		}
		output.WriteString("  • " + finding.Reason + " [" + finding.RuleID + "]\n")
		for _, evidence := range finding.Evidence {
			output.WriteString(mutedStyle.Render("      "+evidence) + "\n")
		}
	}
	output.WriteString("\n" + mutedStyle.Render("Esc/h back to "+sectionDefinitions[m.menuCursor].title+"  q quit"))
	return output.String()
}

func truncate(value string, maximum int) string {
	characters := []rune(value)
	if len(characters) <= maximum {
		return value
	}
	return string(characters[:maximum-1]) + "…"
}

func emptyDash(value string) string {
	if value == "" {
		return "—"
	}
	return value
}
