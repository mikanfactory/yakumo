package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"worktree-ui/internal/model"
)

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	if m.selected != "" {
		return m.selected
	}

	if m.addingRepo {
		return renderAddRepoView(m)
	}

	if m.addingWorktree {
		return renderAddWorktreeView(m)
	}

	if m.confirmingArchive {
		return renderArchiveConfirmView(m)
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
		line := renderItem(item, isSelected, m.sidebarWidth)
		if item.Selectable {
			line = zone.Mark(ZoneID(i), line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("q: quit  ↑↓/jk: move  enter/click: select  d: archive"))

	return zone.Scan(b.String())
}

func renderItem(item model.NavigableItem, selected bool, width int) string {
	switch item.Kind {
	case model.ItemKindGroupHeader:
		return groupHeaderStyle.Render(item.Label)

	case model.ItemKindWorktree:
		return renderWorktree(item, selected, width)

	case model.ItemKindAddWorktree, model.ItemKindAddRepo, model.ItemKindSettings:
		return renderAction(item, selected)

	default:
		return item.Label
	}
}

func renderWorktree(item model.NavigableItem, selected bool, width int) string {
	agentIcon := AgentIcon(item.AgentStatus)
	statusBadge := FormatStatus(item.Status)
	branchName := item.Label

	// Use inline styles to avoid PaddingLeft double-application when
	// inserting agent icon between indent and branch name.
	selectedBranchStyle := lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	normalBranchStyle := lipgloss.NewStyle().Foreground(colorFg)

	var leftPart string
	if selected {
		prefix := " > " + agentIcon
		maxBranchLen := width - lipgloss.Width(prefix) - lipgloss.Width(statusBadge) - 1
		if maxBranchLen > 0 && lipgloss.Width(branchName) > maxBranchLen {
			branchName = truncate(branchName, maxBranchLen)
		}
		leftPart = selectedBranchStyle.Render(" > ") + agentIcon + selectedBranchStyle.Render(branchName)
	} else {
		prefix := "   " + agentIcon
		maxBranchLen := width - lipgloss.Width(prefix) - lipgloss.Width(statusBadge) - 1
		if maxBranchLen > 0 && lipgloss.Width(branchName) > maxBranchLen {
			branchName = truncate(branchName, maxBranchLen)
		}
		leftPart = "   " + agentIcon + normalBranchStyle.Render(branchName)
	}

	if statusBadge == "" {
		return leftPart
	}

	padding := width - lipgloss.Width(leftPart) - lipgloss.Width(statusBadge)
	if padding < 1 {
		padding = 1
	}
	return leftPart + strings.Repeat(" ", padding) + statusBadge
}

func renderAction(item model.NavigableItem, selected bool) string {
	if selected {
		return actionSelectedStyle.Render(fmt.Sprintf("> %s", item.Label))
	}
	return actionStyle.Render(fmt.Sprintf("  %s", item.Label))
}

func renderArchiveConfirmView(m Model) string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Archive Worktree"))
	b.WriteString("\n\n")

	if m.loading {
		b.WriteString("  Removing worktree...")
		return b.String()
	}

	item := m.items[m.archiveTarget]
	b.WriteString(fmt.Sprintf("  Remove worktree '%s'?\n", item.Label))
	b.WriteString("  The branch will be preserved.\n")

	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render(fmt.Sprintf("  Error: %s", m.err.Error())))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("enter: confirm  esc: cancel"))

	return b.String()
}

func renderAddRepoView(m Model) string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Add Repository"))
	b.WriteString("\n\n")

	if m.loading {
		b.WriteString("  Validating...")
		return b.String()
	}

	b.WriteString("  Enter the path to a git repository:\n\n")
	b.WriteString("  ")
	b.WriteString(m.textInput.View())
	b.WriteString("\n")

	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render(fmt.Sprintf("  Error: %s", m.err.Error())))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("enter: confirm  esc: cancel"))

	return b.String()
}

func renderAddWorktreeView(m Model) string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Add Worktree"))
	b.WriteString("\n\n")

	if m.loading {
		b.WriteString("  Creating worktree...")
		return b.String()
	}

	b.WriteString("  Paste a GitHub URL or press Enter for a new branch:\n\n")
	b.WriteString("  ")
	b.WriteString(m.textInput.View())
	b.WriteString("\n")

	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render(fmt.Sprintf("  Error: %s", m.err.Error())))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("enter: confirm  esc: cancel"))

	return b.String()
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
