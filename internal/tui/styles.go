package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/mikanfactory/yakumo/internal/model"
)

// Agent status icon (U+25CF Black Circle, colored per state)
const iconAgent = "●"

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

	errorStyle = lipgloss.NewStyle().
			Foreground(colorRed).
			PaddingLeft(1)

	// Agent status colors (Catppuccin-compatible)
	colorAgentIdle    = colorGreen      // #a6e3a1
	colorAgentRunning = colorYellow     // #f9e2af
	colorAgentWaiting = colorActionItem // #89dceb (cyan)
)

// FormatStatus formats a StatusInfo as colored line change counts (e.g. "+888 -89").
func FormatStatus(s model.StatusInfo) string {
	if s.Insertions == 0 && s.Deletions == 0 {
		return ""
	}

	addStyle := lipgloss.NewStyle().Foreground(colorGreen)
	delStyle := lipgloss.NewStyle().Foreground(colorRed)

	var parts []string
	if s.Insertions > 0 {
		parts = append(parts, addStyle.Render(fmt.Sprintf("+%d", s.Insertions)))
	}
	if s.Deletions > 0 {
		parts = append(parts, delStyle.Render(fmt.Sprintf("-%d", s.Deletions)))
	}
	return strings.Join(parts, " ")
}

// AgentIcon returns a colored ● icon representing the highest-priority
// agent state. Returns empty string when no agents are present.
func AgentIcon(agents []model.AgentInfo) string {
	if len(agents) == 0 {
		return ""
	}

	highestState := model.AgentStateIdle
	for _, a := range agents {
		if a.State > highestState {
			highestState = a.State
		}
	}

	var color lipgloss.Color
	var icon string
	switch highestState {
	case model.AgentStateRunning:
		color = colorAgentRunning
		icon = iconAgent
	case model.AgentStateWaiting:
		color = colorAgentWaiting
		icon = iconAgent
	default:
		color = colorAgentIdle
		icon = iconAgent
	}

	return lipgloss.NewStyle().Foreground(color).Render(icon) + " "
}
