package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"worktree-ui/internal/git"
	"worktree-ui/internal/github"
)

// === Tab ===

type Tab int

const (
	TabChanges Tab = iota
	TabChecks
)

const pollInterval = 5 * time.Second

// === Data Types ===

type ChangedFile struct {
	Path      string
	Additions int
	Deletions int
}

type CheckResult struct {
	Name     string
	Passed   bool
	Duration string
}

type PRComment struct {
	Author  string
	Preview string
}

// === Messages ===

type ChangesDataMsg struct {
	Files []ChangedFile
}

type ChangesDataErrMsg struct {
	Err error
}

type ChecksDataMsg struct {
	Checks ChecksModel
}

type ChecksDataErrMsg struct {
	Err error
}

type VimFinishedMsg struct {
	Err error
}

type TickMsg time.Time

// === Sub-Models ===

type ChangesModel struct {
	files     []ChangedFile
	cursor    int
	scrollOff int
	loading   bool
	err       error
}

type ChecksModel struct {
	prTitle       string
	prDescription string
	gitStatus     string
	commitsBehind int
	checks        []CheckResult
	comments      []PRComment
	todos         []string
	scrollOff     int
	loading       bool
	err           error
}

// === Main Model ===

type Model struct {
	activeTab Tab
	width     int
	height    int
	quitting  bool

	repoDir   string
	gitRunner git.CommandRunner
	ghRunner  github.Runner

	changes ChangesModel
	checks   ChecksModel
}

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

// === Data Fetching Commands ===

func fetchChangesCmd(runner git.CommandRunner, dir string) tea.Cmd {
	return func() tea.Msg {
		entries, err := git.GetDiffNumstat(runner, dir, "origin/main")
		if err != nil {
			return ChangesDataErrMsg{Err: err}
		}
		files := make([]ChangedFile, len(entries))
		for i, e := range entries {
			files[i] = ChangedFile{
				Path:      e.Path,
				Additions: e.Additions,
				Deletions: e.Deletions,
			}
		}
		return ChangesDataMsg{Files: files}
	}
}

func fetchChecksCmd(ghRunner github.Runner, gitRunner git.CommandRunner, dir string) tea.Cmd {
	return func() tea.Msg {
		pr, err := github.FetchPR(ghRunner, dir)
		if err != nil {
			return ChecksDataErrMsg{Err: err}
		}

		commitsBehind, _ := git.GetCommitsBehind(gitRunner, dir, "origin/main")

		checks := make([]CheckResult, len(pr.StatusCheckRollup))
		for i, sc := range pr.StatusCheckRollup {
			checks[i] = CheckResult{
				Name:     sc.CheckName(),
				Passed:   sc.Passed(),
				Duration: sc.DurationString(),
			}
		}

		comments := make([]PRComment, len(pr.Comments))
		for i, c := range pr.Comments {
			comments[i] = PRComment{
				Author:  c.Author.Login,
				Preview: c.Preview(80),
			}
		}

		gitStatus := github.MapMergeStateStatus(pr.MergeStateStatus, pr.ReviewDecision)

		return ChecksDataMsg{
			Checks: ChecksModel{
				prTitle:       pr.Title,
				prDescription: pr.Body,
				gitStatus:     gitStatus,
				commitsBehind: commitsBehind,
				checks:        checks,
				comments:      comments,
				todos:         []string{},
			},
		}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(pollInterval, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
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

// === ChangesModel Methods ===

func (m ChangesModel) update(msg tea.KeyMsg) ChangesModel {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.files)-1 {
			m.cursor++
		}
	case "g":
		m.cursor = 0
	case "G":
		if len(m.files) > 0 {
			m.cursor = len(m.files) - 1
		}
	}
	return m
}

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

// === ChecksModel Methods ===

func (m ChecksModel) update(msg tea.KeyMsg) ChecksModel {
	switch msg.String() {
	case "up", "k":
		if m.scrollOff > 0 {
			m.scrollOff--
		}
	case "down", "j":
		m.scrollOff++
	case "g":
		m.scrollOff = 0
	case "G":
		// Let the view clamp this
		m.scrollOff = 999
	}
	return m
}

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

// === Init / Update / View ===

func initialModel(repoDir string, gitRunner git.CommandRunner, ghRunner github.Runner) Model {
	return Model{
		activeTab: TabChanges,
		width:     80,
		height:    24,
		repoDir:   repoDir,
		gitRunner: gitRunner,
		ghRunner:  ghRunner,
		changes: ChangesModel{
			loading: true,
		},
		checks: ChecksModel{
			loading: true,
		},
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		fetchChangesCmd(m.gitRunner, m.repoDir),
		fetchChecksCmd(m.ghRunner, m.gitRunner, m.repoDir),
		tickCmd(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case ChangesDataMsg:
		m.changes = ChangesModel{
			files:     msg.Files,
			cursor:    m.changes.cursor,
			scrollOff: m.changes.scrollOff,
		}
		return m, nil

	case ChangesDataErrMsg:
		m.changes.loading = false
		m.changes.err = msg.Err
		return m, nil

	case ChecksDataMsg:
		msg.Checks.scrollOff = m.checks.scrollOff
		m.checks = msg.Checks
		return m, nil

	case ChecksDataErrMsg:
		m.checks.loading = false
		m.checks.err = msg.Err
		return m, nil

	case VimFinishedMsg:
		return m, tea.Batch(
			fetchChangesCmd(m.gitRunner, m.repoDir),
			fetchChecksCmd(m.ghRunner, m.gitRunner, m.repoDir),
		)

	case TickMsg:
		return m, tea.Batch(
			fetchChangesCmd(m.gitRunner, m.repoDir),
			fetchChecksCmd(m.ghRunner, m.gitRunner, m.repoDir),
			tickCmd(),
		)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit

		case "tab":
			m.activeTab = (m.activeTab + 1) % 2
			return m, tea.Batch(
				fetchChangesCmd(m.gitRunner, m.repoDir),
				fetchChecksCmd(m.ghRunner, m.gitRunner, m.repoDir),
			)

		case "shift+tab":
			m.activeTab = (m.activeTab + 1) % 2
			return m, tea.Batch(
				fetchChangesCmd(m.gitRunner, m.repoDir),
				fetchChecksCmd(m.ghRunner, m.gitRunner, m.repoDir),
			)

		case "1":
			m.activeTab = TabChanges
			return m, nil

		case "2":
			m.activeTab = TabChecks
			return m, nil

		case "enter":
			if m.activeTab == TabChanges && len(m.changes.files) > 0 {
				file := m.changes.files[m.changes.cursor]
				fullPath := filepath.Join(m.repoDir, file.Path)
				c := exec.Command("vim", fullPath)
				return m, tea.ExecProcess(c, func(err error) tea.Msg {
					return VimFinishedMsg{Err: err}
				})
			}
			return m, nil

		default:
			switch m.activeTab {
			case TabChanges:
				m.changes = m.changes.update(msg)
			case TabChecks:
				m.checks = m.checks.update(msg)
			}
		}
	}

	return m, nil
}

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

// === Main ===

func main() {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}

	gitRunner := git.OSCommandRunner{}
	ghRunner := github.OSRunner{}

	p := tea.NewProgram(
		initialModel(dir, gitRunner, ghRunner),
		tea.WithAltScreen(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}
