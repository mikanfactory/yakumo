package diffui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	tabBar := m.renderTabBar()

	viewportHeight := m.height - 4 // tab bar + help line + margins

	var content string
	switch m.activeTab {
	case TabChanges:
		content = m.changes.view(m.width, viewportHeight)
	case TabChecks:
		content = m.checks.view(m.width, viewportHeight)
	}

	help := helpStyle.Render("  tab: switch pane  j/k: navigate  enter: open in vim  q: quit")

	return lipgloss.JoinVertical(lipgloss.Left, tabBar, content, help)
}

// === Tab Bar ===

func (m Model) renderTabBar() string {
	tabs := []struct {
		label string
		tab   Tab
	}{
		{fmt.Sprintf("Changes %d", len(m.changes.files)), TabChanges},
		{"Checks", TabChecks},
	}

	var rendered []string
	for _, t := range tabs {
		if t.tab == m.activeTab {
			rendered = append(rendered, activeTabStyle.Render(t.label))
		} else {
			rendered = append(rendered, inactiveTabStyle.Render(t.label))
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
}

// === Scroll Helper ===

func adjustScroll(cursor, scrollOff, viewportHeight, totalItems int) int {
	if totalItems <= viewportHeight {
		return 0
	}
	if cursor < scrollOff {
		return cursor
	}
	if cursor >= scrollOff+viewportHeight {
		return cursor - viewportHeight + 1
	}
	return scrollOff
}

// === ChangesModel View ===

func (m ChangesModel) view(width, height int) string {
	if m.loading {
		return filePathDimStyle.Render("  Loading changes...")
	}
	if m.err != nil {
		return filePathDimStyle.Render(fmt.Sprintf("  Error: %s", m.err.Error()))
	}
	if len(m.files) == 0 {
		return filePathDimStyle.Render("  No changes")
	}

	m.scrollOff = adjustScroll(m.cursor, m.scrollOff, height, len(m.files))

	var lines []string
	end := m.scrollOff + height
	if end > len(m.files) {
		end = len(m.files)
	}

	for i := m.scrollOff; i < end; i++ {
		f := m.files[i]

		dir := filepath.Dir(f.Path)
		name := filepath.Base(f.Path)

		var pathStr string
		if dir != "." {
			pathStr = filePathDimStyle.Render(dir+"/") + fileNameBoldStyle.Render(name)
		} else {
			pathStr = fileNameBoldStyle.Render(name)
		}

		var statsStr string
		if f.Additions > 0 {
			statsStr += additionStyle.Render(fmt.Sprintf("+%d", f.Additions))
		}
		if f.Deletions > 0 {
			if statsStr != "" {
				statsStr += " "
			}
			statsStr += deletionStyle.Render(fmt.Sprintf("-%d", f.Deletions))
		}

		// Calculate padding for right alignment
		pathWidth := lipgloss.Width(pathStr)
		statsWidth := lipgloss.Width(statsStr)
		padding := width - pathWidth - statsWidth - 4 // 4 for margins
		if padding < 1 {
			padding = 1
		}

		line := fmt.Sprintf("  %s%s%s", pathStr, strings.Repeat(" ", padding), statsStr)

		if i == m.cursor {
			line = selectedStyle.Render(line)
		}

		lines = append(lines, line)
	}

	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

// === ChecksModel View ===

func (m ChecksModel) view(width, height int) string {
	if m.loading {
		return filePathDimStyle.Render("  Loading PR data...")
	}
	if m.err != nil {
		return filePathDimStyle.Render(fmt.Sprintf("  Error: %s", m.err.Error()))
	}

	var allLines []string

	// PR Title
	allLines = append(allLines, prTitleStyle.Render(m.prTitle))
	allLines = append(allLines, "")

	// PR Description
	descLines := strings.Split(m.prDescription, "\n")
	for _, line := range descLines {
		if strings.HasPrefix(line, "## ") {
			allLines = append(allLines, sectionHeaderStyle.Render(line))
		} else {
			allLines = append(allLines, fileStyle.Render(line))
		}
	}
	allLines = append(allLines, "")

	// Git status
	allLines = append(allLines, sectionHeaderStyle.Render("Git status"))
	allLines = append(allLines, "")

	statusIcon := passedStyle.Render("○")
	allLines = append(allLines, fmt.Sprintf("%s %s", statusIcon, m.gitStatus))
	if m.commitsBehind > 0 {
		allLines = append(allLines, fmt.Sprintf("%s %d commits behind main",
			yellowStyle.Render("○"),
			m.commitsBehind))
	}
	allLines = append(allLines, "")

	// Checks
	allLines = append(allLines, sectionHeaderStyle.Render("Checks"))
	allLines = append(allLines, "")
	for _, check := range m.checks {
		var icon string
		if check.Passed {
			icon = passedStyle.Render("✓")
		} else {
			icon = failedStyle.Render("✗")
		}
		allLines = append(allLines, fmt.Sprintf("  %s %s  %s  %s",
			icon,
			checkIconStyle.Render("⊙"),
			fileStyle.Render(check.Name),
			filePathDimStyle.Render(check.Duration)))
	}
	allLines = append(allLines, "")

	// Comments
	allLines = append(allLines, sectionHeaderStyle.Render("Comments"))
	allLines = append(allLines, "")
	if len(m.comments) == 0 {
		allLines = append(allLines, filePathDimStyle.Render("  No comments yet"))
	}
	for _, c := range m.comments {
		allLines = append(allLines, fmt.Sprintf("  %s  %s  %s",
			checkIconStyle.Render("○"),
			commentAuthorStyle.Render(c.Author),
			filePathDimStyle.Render(c.Preview)))
	}
	allLines = append(allLines, "")

	// Your todos
	allLines = append(allLines, sectionHeaderStyle.Render("Your todos"))
	allLines = append(allLines, "")
	if len(m.todos) == 0 {
		allLines = append(allLines, filePathDimStyle.Render("  No todos yet"))
	}
	for _, todo := range m.todos {
		allLines = append(allLines, fmt.Sprintf("  [ ] %s", fileStyle.Render(todo)))
	}

	// Clamp scroll offset
	maxScroll := len(allLines) - height
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.scrollOff > maxScroll {
		m.scrollOff = maxScroll
	}

	// Apply scroll and viewport
	start := m.scrollOff
	end := start + height
	if end > len(allLines) {
		end = len(allLines)
	}

	visible := allLines[start:end]

	for len(visible) < height {
		visible = append(visible, "")
	}

	return strings.Join(visible, "\n")
}
