package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"worktree-ui/internal/git"
	"worktree-ui/internal/model"
	"worktree-ui/internal/sidebar"
)

// GitDataMsg is sent when git data has been fetched.
type GitDataMsg struct {
	Groups []model.RepoGroup
}

// GitDataErrMsg is sent when git data fetching fails.
type GitDataErrMsg struct {
	Err error
}

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
}

// NewModel creates a new TUI model.
func NewModel(cfg model.Config, runner git.CommandRunner) Model {
	return Model{
		sidebarWidth: cfg.SidebarWidth,
		config:       cfg,
		runner:       runner,
		loading:      true,
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
	switch msg := msg.(type) {

	case GitDataMsg:
		m.groups = msg.Groups
		m.items = sidebar.BuildItems(msg.Groups)
		m.cursor = FirstSelectable(m.items)
		m.loading = false
		return m, nil

	case GitDataErrMsg:
		m.err = msg.Err
		m.loading = false
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {

		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit

		case "up", "k":
			m.cursor = PrevSelectable(m.items, m.cursor)

		case "down", "j":
			m.cursor = NextSelectable(m.items, m.cursor)

		case "enter":
			if m.cursor < len(m.items) && m.items[m.cursor].Kind == model.ItemKindWorktree {
				m.selected = m.items[m.cursor].WorktreePath
				return m, tea.Quit
			}
		}
	}

	return m, nil
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
				status, _ := git.GetStatus(runner, worktrees[i].Path)
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
