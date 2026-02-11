package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"worktree-ui/internal/model"
)

var (
	colorFg         = lipgloss.Color("#cdd6f4")
	colorFgDim      = lipgloss.Color("#6c7086")
	colorAccent     = lipgloss.Color("#89b4fa")
	colorGreen      = lipgloss.Color("#a6e3a1")
	colorRed        = lipgloss.Color("#f38ba8")
	colorYellow     = lipgloss.Color("#f9e2af")
	colorActionItem = lipgloss.Color("#89dceb")

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorFg).
			PaddingLeft(1).
			PaddingBottom(1)

	groupHeaderStyle = lipgloss.NewStyle().
				Foreground(colorFgDim).
				Bold(true).
				PaddingLeft(1)

	worktreeStyle = lipgloss.NewStyle().
			Foreground(colorFg).
			PaddingLeft(3)

	worktreeSelectedStyle = lipgloss.NewStyle().
				Foreground(colorAccent).
				Bold(true).
				PaddingLeft(1)

	actionStyle = lipgloss.NewStyle().
			Foreground(colorActionItem).
			PaddingLeft(1).
			PaddingTop(1)

	actionSelectedStyle = lipgloss.NewStyle().
				Foreground(colorAccent).
				Bold(true).
				PaddingLeft(1).
				PaddingTop(1)

	helpStyle = lipgloss.NewStyle().
			Foreground(colorFgDim).
			PaddingLeft(1).
			PaddingTop(1)
)

// FormatStatus formats a StatusInfo as a colored badge string.
func FormatStatus(s model.StatusInfo) string {
	var parts []string

	modStyle := lipgloss.NewStyle().Foreground(colorYellow)
	addStyle := lipgloss.NewStyle().Foreground(colorGreen)
	delStyle := lipgloss.NewStyle().Foreground(colorRed)
	dimStyle := lipgloss.NewStyle().Foreground(colorFgDim)

	if s.Modified > 0 {
		parts = append(parts, modStyle.Render(fmt.Sprintf("M%d", s.Modified)))
	}
	if s.Added > 0 {
		parts = append(parts, addStyle.Render(fmt.Sprintf("+%d", s.Added)))
	}
	if s.Deleted > 0 {
		parts = append(parts, delStyle.Render(fmt.Sprintf("-%d", s.Deleted)))
	}
	if s.Untracked > 0 {
		parts = append(parts, dimStyle.Render(fmt.Sprintf("?%d", s.Untracked)))
	}

	if len(parts) == 0 {
		return ""
	}

	return " " + dimStyle.Render("[") + strings.Join(parts, " ") + dimStyle.Render("]")
}
