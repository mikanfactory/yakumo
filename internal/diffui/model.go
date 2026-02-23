package diffui

import (
	"os/exec"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"worktree-ui/internal/git"
	"worktree-ui/internal/github"
)

// === Tab ===

type Tab int

const (
	TabChanges Tab = iota
	TabChecks
)

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
	checks  ChecksModel
}

// NewModel creates a new diff UI model.
func NewModel(repoDir string, gitRunner git.CommandRunner, ghRunner github.Runner) Model {
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

// === Sub-Model Update Methods ===

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
