package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/mikanfactory/yakumo/internal/model"
)

const (
	workspacesTitle = "Workspaces"
	workspacesHelp  = "q: quit  ↑↓/jk: move  enter/click: select  d: archive"
)

// reservedRows is the chrome height (title + spacer + help). The title and
// help styles are static, so this is computed once at package init rather than
// re-rendered on every frame.
var reservedRows = lipgloss.Height(titleStyle.Render(workspacesTitle)) + 1 + lipgloss.Height(helpStyle.Render(workspacesHelp))

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	if m.selected != "" && !m.quitWhenIdle {
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
		return titleStyle.Render(workspacesTitle) + "\n\n  Loading..."
	}

	if m.err != nil && len(m.items) == 0 {
		return titleStyle.Render(workspacesTitle) + "\n\n  Error: " + m.err.Error()
	}

	title := titleStyle.Render(workspacesTitle)
	footer := renderFooter(m)

	vp := viewportHeight(m.height)

	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n")

	used := 0
	for i := m.scrollOff; i < len(m.items); i++ {
		item := m.items[i]
		isSelected := i == m.cursor
		line := renderItem(item, isSelected, m.sidebarWidth)
		if label := m.pendingLabel(item); label != "" {
			line = renderWorktreeWithBadge(item, isSelected, m.sidebarWidth, pendingStyle.Render(label))
		}
		h := lipgloss.Height(line)
		if vp > 0 && used+h > vp {
			break
		}
		if item.Selectable {
			line = zone.Mark(ZoneID(i), line)
		}
		b.WriteString(line)
		b.WriteString("\n")
		used += h
	}

	b.WriteString(footer)

	return zone.Scan(b.String())
}

// renderFooter replaces the help line with status text of the same height so
// that the viewport size used for scrolling stays unchanged.
func renderFooter(m Model) string {
	if m.quitWhenIdle && m.blockingOps() > 0 {
		return helpStyle.Render(m.pendingSummary())
	}
	if m.err != nil {
		msg := "Error: " + m.err.Error()
		if m.sidebarWidth > 0 {
			msg = truncate(msg, m.sidebarWidth)
		}
		return footerErrorStyle.Render(msg)
	}
	return helpStyle.Render(workspacesHelp)
}

// viewportHeight returns the rows available for the items section given the
// full terminal height. Returns 0 as a sentinel meaning "size unknown — render
// every item" so the first frames before WindowSizeMsg arrives still work.
func viewportHeight(termHeight int) int {
	if termHeight <= 0 {
		return 0
	}
	h := termHeight - reservedRows
	if h < 1 {
		return 1
	}
	return h
}

// itemHeights returns the rendered terminal-row count for each item. Heights
// vary by Kind (action rows have PaddingTop) so callers must use this rather
// than assuming 1 row per item.
func itemHeights(items []model.NavigableItem, cursor, sidebarWidth int) []int {
	heights := make([]int, len(items))
	for i, item := range items {
		line := renderItem(item, i == cursor, sidebarWidth)
		heights[i] = lipgloss.Height(line)
	}
	return heights
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
	return renderWorktreeWithBadge(item, selected, width, FormatStatus(item.Status))
}

func renderWorktreeWithBadge(item model.NavigableItem, selected bool, width int, statusBadge string) string {
	agentIcon := AgentIcon(item.AgentStatus)
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

	b.WriteString(fmt.Sprintf("  Remove worktree '%s'?\n", m.archiveTarget.Label))
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
	b.WriteString(helpStyle.Render("enter: confirm  tab: complete  esc: cancel"))

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

	b.WriteString("  Paste a GitHub URL, enter a branch name, or press Enter for a new branch:\n\n")
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
