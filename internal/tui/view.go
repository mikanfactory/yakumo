package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"worktree-ui/internal/model"
)

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	if m.selected != "" {
		return m.selected
	}

	if m.loading {
		return titleStyle.Render("Workspaces") + "\n\n  Loading..."
	}

	if m.err != nil {
		return titleStyle.Render("Workspaces") + "\n\n  Error: " + m.err.Error()
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render("Workspaces"))
	b.WriteString("\n")

	for i, item := range m.items {
		isSelected := i == m.cursor
		b.WriteString(renderItem(item, isSelected, m.sidebarWidth))
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("q: quit  ↑↓/jk: move  enter: select"))

	return b.String()
}

func renderItem(item model.NavigableItem, selected bool, width int) string {
	switch item.Kind {
	case model.ItemKindGroupHeader:
		return groupHeaderStyle.Render(item.Label)

	case model.ItemKindWorktree:
		return renderWorktree(item, selected, width)

	case model.ItemKindAddRepo, model.ItemKindSettings:
		return renderAction(item, selected)

	default:
		return item.Label
	}
}

func renderWorktree(item model.NavigableItem, selected bool, width int) string {
	statusBadge := FormatStatus(item.Status)
	branchName := item.Label

	if selected {
		cursor := "> "
		maxBranchLen := width - lipgloss.Width(cursor) - lipgloss.Width(statusBadge) - 1
		if maxBranchLen > 0 && lipgloss.Width(branchName) > maxBranchLen {
			branchName = truncate(branchName, maxBranchLen)
		}
		return worktreeSelectedStyle.Render(cursor+branchName) + statusBadge
	}

	indent := "   "
	maxBranchLen := width - lipgloss.Width(indent) - lipgloss.Width(statusBadge) - 1
	if maxBranchLen > 0 && lipgloss.Width(branchName) > maxBranchLen {
		branchName = truncate(branchName, maxBranchLen)
	}
	return worktreeStyle.Render(branchName) + statusBadge
}

func renderAction(item model.NavigableItem, selected bool) string {
	if selected {
		return actionSelectedStyle.Render(fmt.Sprintf("> %s", item.Label))
	}
	return actionStyle.Render(fmt.Sprintf("  %s", item.Label))
}

func truncate(s string, maxLen int) string {
	if maxLen <= 3 {
		return s[:maxLen]
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-1]) + "…"
}
