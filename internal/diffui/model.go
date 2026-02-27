package diffui

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/mikanfactory/yakumo/internal/git"
	"github.com/mikanfactory/yakumo/internal/github"
	"github.com/mikanfactory/yakumo/internal/tmux"
)

// === Tab ===

type Tab int

const (
	TabChanges Tab = iota
	TabChecks
	tabCount
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

type OpenVimResultMsg struct {
	Err error
}

type OpenPRResultMsg struct {
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
	prURL         string
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

	repoDir     string
	gitRunner   git.CommandRunner
	ghRunner    github.Runner
	tmuxRunner  tmux.Runner // nil when not inside tmux
	sessionName string      // cached tmux session name (empty when not in tmux)
	baseRef     string

	statusMsg string

	changes ChangesModel
	checks  ChecksModel
}

// NewModel creates a new diff UI model.
// tmuxRunner may be nil when running outside tmux (vim opens in the current pane).
// sessionName is the cached tmux session name; pass "" if unknown.
func NewModel(repoDir string, gitRunner git.CommandRunner, ghRunner github.Runner, tmuxRunner tmux.Runner, sessionName string, baseRef string) Model {
	return Model{
		activeTab:   TabChanges,
		width:       80,
		height:      24,
		repoDir:     repoDir,
		gitRunner:   gitRunner,
		ghRunner:    ghRunner,
		tmuxRunner:  tmuxRunner,
		sessionName: sessionName,
		baseRef:     baseRef,
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
		fetchChangesCmd(m.gitRunner, m.repoDir, m.baseRef),
		fetchChecksCmd(m.ghRunner, m.gitRunner, m.repoDir, m.baseRef),
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
			fetchChangesCmd(m.gitRunner, m.repoDir, m.baseRef),
			fetchChecksCmd(m.ghRunner, m.gitRunner, m.repoDir, m.baseRef),
		)

	case OpenVimResultMsg:
		if msg.Err != nil {
			m.statusMsg = msg.Err.Error()
		}
		return m, nil

	case OpenPRResultMsg:
		if msg.Err != nil {
			m.statusMsg = msg.Err.Error()
		}
		return m, nil

	case tea.MouseMsg:
		if msg.Action == tea.MouseActionRelease && m.activeTab == TabChecks {
			if zone.Get("open-pr").InBounds(msg) && m.checks.prURL != "" {
				return m, openPRInBrowserCmd(m.checks.prURL)
			}
		}
		return m, nil

	case TickMsg:
		return m, tea.Batch(
			fetchChangesCmd(m.gitRunner, m.repoDir, m.baseRef),
			fetchChecksCmd(m.ghRunner, m.gitRunner, m.repoDir, m.baseRef),
			tickCmd(),
		)

	case tea.KeyMsg:
		m.statusMsg = ""

		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit

		case "tab":
			m.activeTab = (m.activeTab + 1) % tabCount
			return m, tea.Batch(
				fetchChangesCmd(m.gitRunner, m.repoDir, m.baseRef),
				fetchChecksCmd(m.ghRunner, m.gitRunner, m.repoDir, m.baseRef),
			)

		case "shift+tab":
			m.activeTab = (m.activeTab + tabCount - 1) % tabCount
			return m, tea.Batch(
				fetchChangesCmd(m.gitRunner, m.repoDir, m.baseRef),
				fetchChecksCmd(m.ghRunner, m.gitRunner, m.repoDir, m.baseRef),
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

				if m.tmuxRunner != nil {
					return m, openVimInIdleCenterPaneCmd(m.tmuxRunner, fullPath, m.sessionName)
				}

				// Fallback: open vim in the current pane (non-tmux environment)
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
				var cmd tea.Cmd
				m.checks, cmd = m.checks.update(msg)
				if cmd != nil {
					return m, cmd
				}
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

func (m ChecksModel) update(msg tea.KeyMsg) (ChecksModel, tea.Cmd) {
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
	case "o":
		if m.prURL != "" {
			return m, openPRInBrowserCmd(m.prURL)
		}
	}
	return m, nil
}

// === Vim in Center Pane ===

// centerPaneTargets returns the tmux targets for all center panes relative to a session.
func centerPaneTargets(session string) []string {
	return []string{
		session + ":main-window.0",
		session + ":background-window.0",
		session + ":background-window.1",
	}
}

// isShellCommand returns true if the command name indicates an idle shell prompt.
func isShellCommand(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	cmd = strings.TrimPrefix(cmd, "-") // login shell prefix (e.g., "-zsh")
	cmd = strings.ToLower(cmd)
	switch cmd {
	case "zsh", "bash", "fish", "sh", "dash", "ksh":
		return true
	}
	return false
}

// openVimInIdleCenterPaneCmd finds an idle center pane, sends vim there,
// swaps it to main-window if needed, and focuses it.
// When sessionName is non-empty it is used directly; otherwise CurrentSessionName is called as a fallback.
func openVimInIdleCenterPaneCmd(runner tmux.Runner, filePath string, sessionName string) tea.Cmd {
	return func() tea.Msg {
		session := sessionName
		if session == "" {
			var err error
			session, err = tmux.CurrentSessionName(runner)
			if err != nil {
				return OpenVimResultMsg{Err: fmt.Errorf("セッション名の取得に失敗: %w", err)}
			}
		}

		targets := centerPaneTargets(session)
		mainCenter := targets[0]

		idleTarget := ""
		for _, target := range targets {
			cmd, err := tmux.PaneCurrentCommand(runner, target)
			if err != nil {
				continue
			}
			if isShellCommand(cmd) {
				idleTarget = target
				break
			}
		}

		if idleTarget == "" {
			return OpenVimResultMsg{Err: fmt.Errorf("利用可能なcenter paneがありません")}
		}

		cmd := "vim " + shellEscape(filePath)
		if err := tmux.SendKeys(runner, idleTarget, cmd); err != nil {
			return OpenVimResultMsg{Err: fmt.Errorf("vimの起動に失敗: %w", err)}
		}

		// Swap the idle pane to main-window center if it's in the background
		if idleTarget != mainCenter {
			if _, err := runner.Run("swap-pane", "-d", "-s", idleTarget, "-t", mainCenter); err != nil {
				return OpenVimResultMsg{Err: fmt.Errorf("paneの入れ替えに失敗: %w", err)}
			}
		}

		if err := tmux.SelectPane(runner, mainCenter); err != nil {
			return OpenVimResultMsg{Err: fmt.Errorf("paneのフォーカスに失敗: %w", err)}
		}

		return OpenVimResultMsg{}
	}
}

// === Open PR in Browser ===

func openPRInBrowserCmd(url string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", url)
		case "windows":
			cmd = exec.Command("cmd", "/c", "start", url)
		default:
			cmd = exec.Command("xdg-open", url)
		}
		err := cmd.Start()
		return OpenPRResultMsg{Err: err}
	}
}

// shellEscape wraps a string in single quotes for safe shell usage.
func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// === Data Fetching Commands ===

func fetchChangesCmd(runner git.CommandRunner, dir, baseRef string) tea.Cmd {
	base := normalizeBaseRef(baseRef)
	return func() tea.Msg {
		entries, err := git.GetAllChanges(runner, dir, base)
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

func fetchChecksCmd(ghRunner github.Runner, gitRunner git.CommandRunner, dir, baseRef string) tea.Cmd {
	base := normalizeBaseRef(baseRef)
	return func() tea.Msg {
		pr, err := github.FetchPR(ghRunner, dir)
		if err != nil {
			return ChecksDataErrMsg{Err: err}
		}

		commitsBehind, _ := git.GetCommitsBehind(gitRunner, dir, base)

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
				prURL:         pr.URL,
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

func normalizeBaseRef(baseRef string) string {
	if strings.TrimSpace(baseRef) == "" {
		return "origin/main"
	}
	return baseRef
}
