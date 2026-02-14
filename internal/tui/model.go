package tui

import (
	"fmt"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

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

// WorktreeAddedMsg is sent when a new worktree has been created.
type WorktreeAddedMsg struct{}

// WorktreeAddErrMsg is sent when worktree creation fails.
type WorktreeAddErrMsg struct {
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

	case WorktreeAddedMsg:
		m.loading = true
		return m, fetchGitDataCmd(m.config, m.runner)

	case WorktreeAddErrMsg:
		m.err = msg.Err
		m.loading = false
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
			}
		}
	}

	return m, nil
}

// ZoneID returns the bubblezone ID for an item at the given index.
func ZoneID(index int) string {
	return fmt.Sprintf("item-%d", index)
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
