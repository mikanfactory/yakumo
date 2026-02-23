package diffui

import (
	"time"

	"github.com/charmbracelet/lipgloss"
)

const pollInterval = 5 * time.Second

// === Color Palette ===

var (
	colorSecondary = lipgloss.Color("212")
	colorGreen     = lipgloss.Color("82")
	colorRed       = lipgloss.Color("196")
	colorDimmed    = lipgloss.Color("240")
	colorWhite     = lipgloss.Color("255")
	colorYellow    = lipgloss.Color("220")
)

// === Styles ===

var (
	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite).
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorDimmed)

	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(colorDimmed).
				Padding(0, 1)

	cursorStyle = lipgloss.NewStyle().
			Foreground(colorSecondary).
			Bold(true)

	fileStyle = lipgloss.NewStyle().
			Foreground(colorWhite)

	additionStyle = lipgloss.NewStyle().
			Foreground(colorGreen)

	deletionStyle = lipgloss.NewStyle().
			Foreground(colorRed)

	filePathDimStyle = lipgloss.NewStyle().
				Foreground(colorDimmed)

	fileNameBoldStyle = lipgloss.NewStyle().
				Foreground(colorWhite).
				Bold(true)

	prTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite)

	sectionHeaderStyle = lipgloss.NewStyle().
				Foreground(colorDimmed)

	passedStyle = lipgloss.NewStyle().
			Foreground(colorGreen)

	failedStyle = lipgloss.NewStyle().
			Foreground(colorRed)

	commentAuthorStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorWhite)

	helpStyle = lipgloss.NewStyle().
			Foreground(colorDimmed)

	checkIconStyle = lipgloss.NewStyle().
			Foreground(colorDimmed)

	yellowStyle = lipgloss.NewStyle().
			Foreground(colorYellow)

	selectedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236"))
)
