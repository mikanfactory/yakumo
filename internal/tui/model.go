package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"worktree-ui/internal/agent"
	"worktree-ui/internal/config"
	"worktree-ui/internal/git"
	"worktree-ui/internal/model"
	"worktree-ui/internal/sidebar"
	"worktree-ui/internal/tmux"
)

// GitDataMsg is sent when git data has been fetched.
type GitDataMsg struct {
	Groups []model.RepoGroup
}

// GitDataErrMsg is sent when git data fetching fails.
type GitDataErrMsg struct {
	Err error
}

// WorktreeAddedMsg is sent when a new worktree has been created.
type WorktreeAddedMsg struct{}

// WorktreeAddErrMsg is sent when worktree creation fails.
type WorktreeAddErrMsg struct {
	Err error
}

// RepoValidatedMsg is sent when a repository path has been validated.
type RepoValidatedMsg struct {
	Name string
	Path string
}

// RepoValidationErrMsg is sent when repository validation fails.
type RepoValidationErrMsg struct {
	Err error
}

// AgentTickMsg triggers periodic agent status refresh.
type AgentTickMsg time.Time

// AgentStatusMsg delivers fetched agent status for all worktrees.
type AgentStatusMsg struct {
	Statuses map[string][]model.AgentInfo
}

// RepoAddedMsg is sent when a repository has been added to config.
type RepoAddedMsg struct{}

// RepoAddErrMsg is sent when adding a repository to config fails.
type RepoAddErrMsg struct {
	Err error
}

// WorktreeArchivedMsg is sent when a worktree has been successfully archived.
type WorktreeArchivedMsg struct{}

// WorktreeArchiveErrMsg is sent when worktree archiving fails.
type WorktreeArchiveErrMsg struct {
	Err error
}

// agentPollInterval is how often we poll tmux for Claude Code agent status.
const agentPollInterval = 2 * time.Second

// Model is the BubbleTea model for the sidebar.
type Model struct {
	items        []model.NavigableItem
	groups       []model.RepoGroup
	cursor       int
	sidebarWidth int
	selected     string
	quitting     bool
	err          error
	config       model.Config
	runner       git.CommandRunner
	loading      bool
	addingRepo   bool
	textInput    textinput.Model
	configPath   string
	tmuxRunner        tmux.Runner
	agentStatus       map[string][]model.AgentInfo
	confirmingArchive bool
	archiveTarget     int
}

// NewModel creates a new TUI model.
// tmuxRunner may be nil when running outside tmux (agent polling is skipped).
func NewModel(cfg model.Config, runner git.CommandRunner, configPath string, tmuxRunner tmux.Runner) Model {
	ti := textinput.New()
	ti.Placeholder = "/path/to/repository"
	ti.CharLimit = 256
	ti.Width = 50

	return Model{
		sidebarWidth: cfg.SidebarWidth,
		config:       cfg,
		runner:       runner,
		loading:      true,
		configPath:   configPath,
		textInput:    ti,
		tmuxRunner:   tmuxRunner,
	}
}

// Selected returns the selected worktree path, if any.
func (m Model) Selected() string {
	return m.selected
}

func (m Model) Init() tea.Cmd {
	return fetchGitDataCmd(m.config, m.runner)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle add-repo input mode
	if m.addingRepo {
		return m.updateAddRepoMode(msg)
	}

	// Handle archive confirmation mode
	if m.confirmingArchive {
		return m.updateConfirmArchiveMode(msg)
	}

	switch msg := msg.(type) {

	case GitDataMsg:
		m.groups = msg.Groups
		m.items = sidebar.BuildItems(msg.Groups)
		m.cursor = FirstSelectable(m.items)
		m.loading = false
		return m, agentTickCmd()

	case AgentTickMsg:
		if len(m.groups) > 0 && m.tmuxRunner != nil {
			return m, fetchAgentStatusCmd(m.tmuxRunner, m.groups)
		}
		return m, agentTickCmd()

	case AgentStatusMsg:
		m.agentStatus = msg.Statuses
		for i := range m.items {
			if m.items[i].Kind == model.ItemKindWorktree {
				sessionName := filepath.Base(m.items[i].WorktreePath)
				m.items[i].AgentStatus = m.agentStatus[sessionName]
			}
		}
		return m, agentTickCmd()

	case GitDataErrMsg:
		m.err = msg.Err
		m.loading = false
		return m, nil

	case WorktreeAddedMsg:
		m.loading = true
		return m, fetchGitDataCmd(m.config, m.runner)

	case WorktreeAddErrMsg:
		m.err = msg.Err
		m.loading = false
		return m, nil

	case WorktreeArchivedMsg:
		m.loading = true
		m.confirmingArchive = false
		return m, fetchGitDataCmd(m.config, m.runner)

	case WorktreeArchiveErrMsg:
		m.err = msg.Err
		m.loading = false
		m.confirmingArchive = false
		return m, nil

	case RepoValidatedMsg:
		m.loading = true
		return m, addRepoToConfigCmd(m.configPath, msg.Name, msg.Path)

	case RepoValidationErrMsg:
		m.err = msg.Err
		m.loading = false
		return m, nil

	case RepoAddedMsg:
		cfg, err := config.LoadFromFile(m.configPath)
		if err != nil {
			m.err = err
			m.loading = false
			m.addingRepo = false
			return m, nil
		}
		m.config = cfg
		m.addingRepo = false
		m.textInput.SetValue("")
		m.loading = true
		return m, fetchGitDataCmd(m.config, m.runner)

	case RepoAddErrMsg:
		m.err = msg.Err
		m.loading = false
		m.addingRepo = false
		return m, nil

	case tea.MouseMsg:
		if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
			for i, item := range m.items {
				if !item.Selectable {
					continue
				}
				if zone.Get(ZoneID(i)).InBounds(msg) {
					m.cursor = i
					if item.Kind == model.ItemKindWorktree {
						m.selected = item.WorktreePath
						return m, tea.Quit
					}
					if item.Kind == model.ItemKindAddWorktree {
						m.loading = true
						return m, addWorktreeCmd(m.runner, item.RepoRootPath, m.config.WorktreeBasePath)
					}
					if item.Kind == model.ItemKindAddRepo {
						m.addingRepo = true
						m.err = nil
						cmd := m.textInput.Focus()
						return m, cmd
					}
					return m, nil
				}
			}
		}

	case tea.KeyMsg:
		switch msg.String() {

		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit

		case "up", "k":
			m.cursor = PrevSelectable(m.items, m.cursor)

		case "down", "j":
			m.cursor = NextSelectable(m.items, m.cursor)

		case "d":
			if m.cursor < len(m.items) {
				item := m.items[m.cursor]
				if item.Kind == model.ItemKindWorktree && !item.IsBare {
					m.confirmingArchive = true
					m.archiveTarget = m.cursor
					m.err = nil
					return m, nil
				}
			}

		case "enter":
			if m.cursor < len(m.items) {
				item := m.items[m.cursor]
				if item.Kind == model.ItemKindWorktree {
					m.selected = item.WorktreePath
					return m, tea.Quit
				}
				if item.Kind == model.ItemKindAddWorktree {
					m.loading = true
					return m, addWorktreeCmd(m.runner, item.RepoRootPath, m.config.WorktreeBasePath)
				}
				if item.Kind == model.ItemKindAddRepo {
					m.addingRepo = true
					m.err = nil
					cmd := m.textInput.Focus()
					return m, cmd
				}
			}
		}
	}

	return m, nil
}

func (m Model) updateAddRepoMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEscape:
			m.addingRepo = false
			m.textInput.SetValue("")
			m.err = nil
			return m, nil
		case tea.KeyEnter:
			path := strings.TrimSpace(m.textInput.Value())
			if path == "" {
				m.err = fmt.Errorf("path cannot be empty")
				return m, nil
			}
			m.loading = true
			m.err = nil
			return m, validateRepoCmd(m.runner, path)
		case tea.KeyCtrlC:
			m.quitting = true
			return m, tea.Quit
		}

	case RepoValidatedMsg:
		m.loading = true
		return m, addRepoToConfigCmd(m.configPath, msg.Name, msg.Path)

	case RepoValidationErrMsg:
		m.err = msg.Err
		m.loading = false
		return m, nil

	case RepoAddedMsg:
		cfg, err := config.LoadFromFile(m.configPath)
		if err != nil {
			m.err = err
			m.loading = false
			m.addingRepo = false
			return m, nil
		}
		m.config = cfg
		m.addingRepo = false
		m.textInput.SetValue("")
		m.loading = true
		return m, fetchGitDataCmd(m.config, m.runner)

	case RepoAddErrMsg:
		m.err = msg.Err
		m.loading = false
		m.addingRepo = false
		return m, nil
	}

	// Delegate to textinput
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

// ZoneID returns the bubblezone ID for an item at the given index.
func ZoneID(index int) string {
	return fmt.Sprintf("item-%d", index)
}

func (m Model) updateConfirmArchiveMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEscape:
			m.confirmingArchive = false
			m.err = nil
			return m, nil
		case tea.KeyEnter:
			item := m.items[m.archiveTarget]
			m.loading = true
			m.err = nil
			return m, archiveWorktreeCmd(m.runner, item.RepoRootPath, item.WorktreePath)
		case tea.KeyCtrlC:
			m.quitting = true
			return m, tea.Quit
		}

	case WorktreeArchivedMsg:
		m.loading = true
		m.confirmingArchive = false
		return m, fetchGitDataCmd(m.config, m.runner)

	case WorktreeArchiveErrMsg:
		m.err = msg.Err
		m.loading = false
		m.confirmingArchive = false
		return m, nil
	}

	return m, nil
}

func archiveWorktreeCmd(runner git.CommandRunner, repoRootPath, worktreePath string) tea.Cmd {
	return func() tea.Msg {
		if err := git.RemoveWorktree(runner, repoRootPath, worktreePath); err != nil {
			return WorktreeArchiveErrMsg{Err: err}
		}
		return WorktreeArchivedMsg{}
	}
}

func addWorktreeCmd(runner git.CommandRunner, repoPath, basePath string) tea.Cmd {
	return func() tea.Msg {
		userName, err := git.GetUserName(runner, repoPath)
		if err != nil {
			return WorktreeAddErrMsg{Err: err}
		}

		country := git.RandomCountry()
		slug := git.Slugify(country)
		branch := userName + "/" + slug
		newPath := filepath.Join(basePath, slug)

		if err := git.AddWorktree(runner, repoPath, newPath, branch); err != nil {
			return WorktreeAddErrMsg{Err: err}
		}

		return WorktreeAddedMsg{}
	}
}

func validateRepoCmd(runner git.CommandRunner, inputPath string) tea.Cmd {
	return func() tea.Msg {
		p := inputPath
		// Expand ~ to home directory
		if strings.HasPrefix(p, "~/") {
			home, err := os.UserHomeDir()
			if err != nil {
				return RepoValidationErrMsg{Err: fmt.Errorf("expanding home directory: %w", err)}
			}
			p = filepath.Join(home, p[2:])
		}

		expanded, err := filepath.Abs(p)
		if err != nil {
			return RepoValidationErrMsg{Err: fmt.Errorf("invalid path: %w", err)}
		}

		if _, err := os.Stat(expanded); err != nil {
			return RepoValidationErrMsg{Err: fmt.Errorf("path does not exist: %s", expanded)}
		}

		root, err := runner.Run(expanded, "rev-parse", "--show-toplevel")
		if err != nil {
			return RepoValidationErrMsg{Err: fmt.Errorf("not a git repository: %s", expanded)}
		}

		root = strings.TrimSpace(root)
		name := filepath.Base(root)
		return RepoValidatedMsg{Name: name, Path: root}
	}
}

func addRepoToConfigCmd(configPath, name, repoPath string) tea.Cmd {
	return func() tea.Msg {
		if err := config.AppendRepository(configPath, name, repoPath); err != nil {
			return RepoAddErrMsg{Err: err}
		}
		return RepoAddedMsg{}
	}
}

func agentTickCmd() tea.Cmd {
	return tea.Tick(agentPollInterval, func(t time.Time) tea.Msg {
		return AgentTickMsg(t)
	})
}

func fetchAgentStatusCmd(tmuxRunner tmux.Runner, groups []model.RepoGroup) tea.Cmd {
	return func() tea.Msg {
		statuses := make(map[string][]model.AgentInfo)
		for _, group := range groups {
			for _, wt := range group.Worktrees {
				sessionName := filepath.Base(wt.Path)
				agents, err := agent.DetectSessionAgents(tmuxRunner, sessionName)
				if err != nil {
					continue
				}
				if len(agents) > 0 {
					statuses[sessionName] = agents
				}
			}
		}
		return AgentStatusMsg{Statuses: statuses}
	}
}

func fetchGitDataCmd(cfg model.Config, runner git.CommandRunner) tea.Cmd {
	return func() tea.Msg {
		var groups []model.RepoGroup

		for _, repoDef := range cfg.Repositories {
			entries, err := git.ListWorktrees(runner, repoDef.Path)
			if err != nil {
				return GitDataErrMsg{Err: err}
			}

			worktrees := git.ToWorktreeInfo(entries)
			for i := range worktrees {
				status, _ := git.GetBranchDiffStat(runner, worktrees[i].Path)
				worktrees[i].Status = status
			}

			groups = append(groups, model.RepoGroup{
				Name:      repoDef.Name,
				RootPath:  repoDef.Path,
				Worktrees: worktrees,
			})
		}

		return GitDataMsg{Groups: groups}
	}
}
