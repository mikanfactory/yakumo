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

type OpenEditorResultMsg struct {
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

// CommandStarter starts an external command without blocking.
// Implementations should reap the child process to avoid zombies.
type CommandStarter func(name string, args ...string) error

func defaultCommandStarter(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

type Model struct {
	activeTab Tab
	width     int
	height    int
	quitting  bool

	repoDir   string
	gitRunner git.CommandRunner
	ghRunner  github.Runner
	baseRef   string

	editorStarter CommandStarter

	statusMsg string

	changes ChangesModel
	checks  ChecksModel
}

// NewModel creates a new diff UI model.
func NewModel(repoDir string, gitRunner git.CommandRunner, ghRunner github.Runner, baseRef string) Model {
	return Model{
		activeTab:     TabChanges,
		width:         80,
		height:        24,
		repoDir:       repoDir,
		gitRunner:     gitRunner,
		ghRunner:      ghRunner,
		baseRef:       baseRef,
		editorStarter: defaultCommandStarter,
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

	case OpenEditorResultMsg:
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
				return m, openZedCmd(m.editorStarter, fullPath)
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

// === Open File in Zed ===

func openZedCmd(starter CommandStarter, filePath string) tea.Cmd {
	return func() tea.Msg {
		if err := starter("zed", filePath); err != nil {
			return OpenEditorResultMsg{Err: fmt.Errorf("zedの起動に失敗: %w", err)}
		}
		return OpenEditorResultMsg{}
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
