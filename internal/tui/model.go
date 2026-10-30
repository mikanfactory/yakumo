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

	"github.com/mikanfactory/yakumo/internal/agent"
	"github.com/mikanfactory/yakumo/internal/archive"
	"github.com/mikanfactory/yakumo/internal/branchname"
	"github.com/mikanfactory/yakumo/internal/config"
	"github.com/mikanfactory/yakumo/internal/git"
	"github.com/mikanfactory/yakumo/internal/github"
	"github.com/mikanfactory/yakumo/internal/model"
	"github.com/mikanfactory/yakumo/internal/pathcomplete"
	"github.com/mikanfactory/yakumo/internal/tmux"
)

// GitDataMsg is sent when git data has been fetched.
type GitDataMsg struct {
	Groups []model.RepoGroup
	Seq    int
}

// GitDataErrMsg is sent when git data fetching fails.
type GitDataErrMsg struct {
	Err error
}

// WorktreeAddedMsg is sent when a new worktree has been created.
type WorktreeAddedMsg struct {
	WorktreePath string
	Branch       string
}

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

// PathSuggestionsMsg delivers directory completion candidates for the add-repo text input.
type PathSuggestionsMsg struct {
	Suggestions []string
	ForDir      string
}

// RepoAddedMsg is sent when a repository has been added to config.
type RepoAddedMsg struct{}

// RepoAddErrMsg is sent when adding a repository to config fails.
type RepoAddErrMsg struct {
	Err error
}

// WorktreeArchivedMsg is sent when a worktree has been successfully archived.
type WorktreeArchivedMsg struct {
	WorktreePath string
}

// WorktreeArchiveErrMsg is sent when worktree archiving fails.
type WorktreeArchiveErrMsg struct {
	WorktreePath string
	Err          error
}

// SessionReadyMsg is sent when the tmux session for the selected worktree is ready.
type SessionReadyMsg struct {
	WorktreePath string
	RepoRootPath string
	SessionName  string
}

// SessionSetupErrMsg is sent when tmux session setup fails.
type SessionSetupErrMsg struct {
	WorktreePath string
	Err          error
}

// SessionSwitchedMsg is sent when the client has switched to the session of the
// last selected worktree.
type SessionSwitchedMsg struct {
	WorktreePath string
	RepoRootPath string
}

// SessionSwitchErrMsg is sent when switching to the prepared session fails.
type SessionSwitchErrMsg struct {
	WorktreePath string
	Err          error
}

// SessionSetupFunc prepares the tmux session for a worktree without switching
// to it and returns the session name.
type SessionSetupFunc func(worktreePath, repoRootPath string) (string, error)

// SessionSwitchFunc switches the tmux client to the named session.
type SessionSwitchFunc func(sessionName string) error

// WorktreeArchiveFunc removes a worktree and its tmux session.
type WorktreeArchiveFunc func(repoRootPath, worktreePath string) error

type pendingOp int

const (
	opSetup pendingOp = iota + 1
	opArchive
)

// agentPollInterval is how often we poll tmux for Claude Code agent status.
const agentPollInterval = 500 * time.Millisecond

// Model is the BubbleTea model for the sidebar.
type Model struct {
	items                  []model.NavigableItem
	groups                 []model.RepoGroup
	cursor                 int
	sidebarWidth           int
	height                 int
	scrollOff              int
	selected               string
	selectedRepoPath       string
	quitting               bool
	err                    error
	config                 model.Config
	runner                 git.CommandRunner
	loading                bool
	addingRepo             bool
	addingWorktree         bool
	addingWorktreeRepoPath string
	textInput              textinput.Model
	configPath             string
	tmuxRunner             tmux.Runner
	ghRunner               github.Runner
	agentStatus            map[string][]model.AgentInfo
	lastSuggestionDir      string
	confirmingArchive      bool
	archiveTarget          model.NavigableItem
	agentTickRunning       bool
	sessionSetup           SessionSetupFunc
	sessionSwitch          SessionSwitchFunc
	archive                WorktreeArchiveFunc
	archivesDetached       bool
	setupTarget            model.NavigableItem
	readySessions          map[string]string
	pendingOps             map[string]pendingOp
	quitWhenIdle           bool
	refreshSeq             int
}

// NewModel creates a new TUI model.
// tmuxRunner may be nil when running outside tmux (agent polling is skipped).
// ghRunner may be nil when gh CLI is not available (PR URL cloning is skipped).
func NewModel(cfg model.Config, runner git.CommandRunner, configPath string, tmuxRunner tmux.Runner, ghRunner github.Runner) Model {
	ti := textinput.New()
	ti.Placeholder = "/path/to/repository"
	ti.CharLimit = 256
	ti.Width = 50
	ti.ShowSuggestions = true

	return Model{
		sidebarWidth: cfg.SidebarWidth,
		height:       24,
		config:       cfg,
		runner:       runner,
		loading:      true,
		configPath:   configPath,
		textInput:    ti,
		tmuxRunner:   tmuxRunner,
		ghRunner:     ghRunner,
	}
}

// WithSessionSetup makes worktree selection run setup in the background.
// Several setups may run in parallel; once all of them finish, the client is
// switched to the last selected worktree and the program quits.
func (m Model) WithSessionSetup(setup SessionSetupFunc, switchFn SessionSwitchFunc) Model {
	m.sessionSetup = setup
	m.sessionSwitch = switchFn
	return m
}

// WithDetachedArchive runs archives through archiveFn, which must keep working
// after this program exits, so that quitting never waits for them.
func (m Model) WithDetachedArchive(archiveFn WorktreeArchiveFunc) Model {
	m.archive = archiveFn
	m.archivesDetached = true
	return m
}

// Selected returns the selected worktree path, if any.
func (m Model) Selected() string {
	return m.selected
}

// SelectedRepoPath returns the repository root path for the selected worktree.
func (m Model) SelectedRepoPath() string {
	return m.selectedRepoPath
}

func (m Model) Init() tea.Cmd {
	return fetchGitDataCmd(m.config, m.runner)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Capture terminal size for cursor-following scroll. Must run before
	// modal-mode dispatch so resize events are honored even during modals.
	if sizeMsg, ok := msg.(tea.WindowSizeMsg); ok {
		m.height = sizeMsg.Height
		m = recomputeScroll(m)
		return m, nil
	}

	if next, cmd, handled := m.updateBackground(msg); handled {
		return next, cmd
	}

	// Handle add-repo input mode
	if m.addingRepo {
		return m.updateAddRepoMode(msg)
	}

	// Handle add-worktree input mode
	if m.addingWorktree {
		return m.updateAddWorktreeMode(msg)
	}

	// Handle archive confirmation mode
	if m.confirmingArchive {
		return m.updateConfirmArchiveMode(msg)
	}

	switch msg := msg.(type) {

	case WorktreeAddedMsg:
		m.loading = true
		return m.refresh()

	case WorktreeAddErrMsg:
		m.err = msg.Err
		m.loading = false
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
		return m.refresh()

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
					m = recomputeScroll(m)
					if m.quitWhenIdle {
						return m, nil
					}
					if item.Kind == model.ItemKindWorktree {
						return m.selectWorktree(item)
					}
					if item.Kind == model.ItemKindAddWorktree {
						m.addingWorktree = true
						m.addingWorktreeRepoPath = item.RepoRootPath
						m.err = nil
						m.textInput.Placeholder = "URL, branch name, or Enter for new branch"
						cmd := m.textInput.Focus()
						return m, cmd
					}
					if item.Kind == model.ItemKindAddRepo {
						m.addingRepo = true
						m.err = nil
						m.textInput.Placeholder = "/path/to/repository"
						cmd := m.textInput.Focus()
						return m, cmd
					}
					return m, nil
				}
			}
		}

	case tea.KeyMsg:
		m.err = nil
		switch msg.String() {

		case "ctrl+c":
			return m.requestQuit(true)

		case "q":
			return m.requestQuit(false)

		case "up", "k":
			m.cursor = PrevSelectable(m.items, m.cursor)
			m = recomputeScroll(m)

		case "down", "j":
			m.cursor = NextSelectable(m.items, m.cursor)
			m = recomputeScroll(m)

		case "d":
			if m.quitWhenIdle {
				return m, nil
			}
			if m.cursor < len(m.items) {
				item := m.items[m.cursor]
				if item.Kind == model.ItemKindWorktree && !item.IsBare && !m.isPending(item.WorktreePath) {
					m.confirmingArchive = true
					m.archiveTarget = item
					return m, nil
				}
			}

		case "enter":
			if m.quitWhenIdle {
				return m, nil
			}
			if m.cursor < len(m.items) {
				item := m.items[m.cursor]
				if item.Kind == model.ItemKindWorktree {
					return m.selectWorktree(item)
				}
				if item.Kind == model.ItemKindAddWorktree {
					m.addingWorktree = true
					m.addingWorktreeRepoPath = item.RepoRootPath
					m.err = nil
					m.textInput.Placeholder = "URL, branch name, or Enter for new branch"
					cmd := m.textInput.Focus()
					return m, cmd
				}
				if item.Kind == model.ItemKindAddRepo {
					m.addingRepo = true
					m.err = nil
					m.textInput.Placeholder = "/path/to/repository"
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
			m.textInput.SetSuggestions(nil)
			m.lastSuggestionDir = ""
			m.err = nil
			return m, nil
		case tea.KeyEnter:
			path := strings.TrimSpace(m.textInput.Value())
			if path == "" {
				m.err = fmt.Errorf("path cannot be empty")
				return m, nil
			}
			m.textInput.SetSuggestions(nil)
			m.lastSuggestionDir = ""
			m.loading = true
			m.err = nil
			return m, validateRepoCmd(m.runner, path)
		case tea.KeyCtrlC:
			return m.requestQuit(true)
		}

	case PathSuggestionsMsg:
		homeDir, _ := os.UserHomeDir()
		currentDir := pathcomplete.ExtractDir(m.textInput.Value(), homeDir)
		if msg.ForDir == currentDir {
			m.textInput.SetSuggestions(msg.Suggestions)
		}
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
		m.textInput.SetSuggestions(nil)
		m.lastSuggestionDir = ""
		m.loading = true
		return m.refresh()

	case RepoAddErrMsg:
		m.err = msg.Err
		m.loading = false
		m.addingRepo = false
		return m, nil
	}

	// Delegate to textinput
	prevValue := m.textInput.Value()
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	newValue := m.textInput.Value()

	// Check if the directory portion changed; if so, fetch new suggestions.
	if newValue != prevValue {
		homeDir, _ := os.UserHomeDir()
		newDir := pathcomplete.ExtractDir(newValue, homeDir)
		if newDir != m.lastSuggestionDir {
			m.lastSuggestionDir = newDir
			return m, tea.Batch(cmd, fetchPathSuggestionsCmd(newValue))
		}
	}

	return m, cmd
}

const maxPathSuggestions = 50

func fetchPathSuggestionsCmd(input string) tea.Cmd {
	return func() tea.Msg {
		homeDir, _ := os.UserHomeDir()
		dir := pathcomplete.ExtractDir(input, homeDir)
		suggestions := pathcomplete.ListDirSuggestions(input, homeDir, pathcomplete.DefaultDirLister, maxPathSuggestions)
		return PathSuggestionsMsg{
			Suggestions: suggestions,
			ForDir:      dir,
		}
	}
}

func (m Model) updateAddWorktreeMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEscape:
			m.addingWorktree = false
			m.addingWorktreeRepoPath = ""
			m.textInput.SetValue("")
			m.err = nil
			return m, nil
		case tea.KeyEnter:
			input := strings.TrimSpace(m.textInput.Value())
			m.textInput.SetValue("")
			m.addingWorktree = false
			m.loading = true
			m.err = nil
			repoName := repoNameFromConfig(m.config, m.addingWorktreeRepoPath)
			if input == "" {
				return m, addWorktreeCmd(m.runner, m.addingWorktreeRepoPath, m.config.WorktreeBasePath, repoName, m.config.DefaultBaseRef)
			}
			if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
				return m, addWorktreeFromURLCmd(m.runner, m.ghRunner, m.addingWorktreeRepoPath, m.config.WorktreeBasePath, repoName, input)
			}
			return m, addWorktreeFromBranchNameCmd(m.runner, m.addingWorktreeRepoPath, m.config.WorktreeBasePath, repoName, input)
		case tea.KeyCtrlC:
			return m.requestQuit(true)
		}

	case WorktreeAddedMsg:
		m.loading = true
		m.addingWorktree = false
		return m.refresh()

	case WorktreeAddErrMsg:
		m.err = msg.Err
		m.loading = false
		m.addingWorktree = false
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
			item := m.archiveTarget
			m.confirmingArchive = false
			m.err = nil
			m = m.markPending(item.WorktreePath, opArchive)
			return m, archiveWorktreeCmd(m.archiveFunc(), item.RepoRootPath, item.WorktreePath)
		case tea.KeyCtrlC:
			return m.requestQuit(true)
		}
	}

	return m, nil
}

func archiveWorktreeCmd(archiveFn WorktreeArchiveFunc, repoRootPath, worktreePath string) tea.Cmd {
	return func() tea.Msg {
		if err := archiveFn(repoRootPath, worktreePath); err != nil {
			return WorktreeArchiveErrMsg{WorktreePath: worktreePath, Err: err}
		}
		return WorktreeArchivedMsg{WorktreePath: worktreePath}
	}
}

func inProcessArchive(runner git.CommandRunner, tmuxRunner tmux.Runner) WorktreeArchiveFunc {
	return func(repoRootPath, worktreePath string) error {
		return archive.Run(archive.Options{
			TmuxRunner:   tmuxRunner,
			GitRunner:    runner,
			RepoRootPath: repoRootPath,
			WorktreePath: worktreePath,
		})
	}
}

func (m Model) archiveFunc() WorktreeArchiveFunc {
	if m.archive != nil {
		return m.archive
	}
	return inProcessArchive(m.runner, m.tmuxRunner)
}

func repoNameFromConfig(cfg model.Config, repoPath string) string {
	for _, repo := range cfg.Repositories {
		if repo.Path == repoPath {
			return repo.Name
		}
	}
	return filepath.Base(repoPath)
}

func addWorktreeCmd(runner git.CommandRunner, repoPath, basePath, repoName, baseRef string) tea.Cmd {
	return func() tea.Msg {
		userName, err := git.GetUserName(runner, repoPath)
		if err != nil {
			return WorktreeAddErrMsg{Err: err}
		}

		country := git.RandomCountry()
		baseSlug := git.Slugify(country)
		userSlug := branchname.SanitizeBranchName(userName)
		if userSlug == "" {
			userSlug = "user"
		}

		if fetchBranch, ok := strings.CutPrefix(baseRef, "origin/"); ok {
			if err := git.FetchBranch(runner, repoPath, fetchBranch); err != nil {
				return WorktreeAddErrMsg{Err: fmt.Errorf("fetching %s: %w", baseRef, err)}
			}
		}

		if err := os.MkdirAll(filepath.Join(basePath, repoName), 0o755); err != nil {
			return WorktreeAddErrMsg{Err: fmt.Errorf("creating parent directory: %w", err)}
		}

		const maxRetries = 10
		for attempt := 1; attempt <= maxRetries; attempt++ {
			slug := baseSlug
			if attempt > 1 {
				slug = fmt.Sprintf("%s-%d", baseSlug, attempt)
			}
			branch := userSlug + "/" + slug
			newPath := filepath.Join(basePath, repoName, slug)

			if err := git.AddWorktree(runner, repoPath, newPath, branch, baseRef); err != nil {
				if git.IsBranchExistsError(err) {
					continue
				}
				return WorktreeAddErrMsg{Err: err}
			}

			return WorktreeAddedMsg{
				WorktreePath: newPath,
				Branch:       branch,
			}
		}

		return WorktreeAddErrMsg{
			Err: fmt.Errorf("could not create worktree for %q: branch already exists after %d attempts", baseSlug, maxRetries),
		}
	}
}

func addWorktreeFromURLCmd(runner git.CommandRunner, ghRunner github.Runner, repoPath, basePath, repoName, rawURL string) tea.Cmd {
	return func() tea.Msg {
		urlInfo, err := github.ParseGitHubURL(rawURL)
		if err != nil {
			return WorktreeAddErrMsg{Err: fmt.Errorf("invalid URL: %w", err)}
		}

		var branch string
		switch urlInfo.Type {
		case github.URLTypeBranch:
			branch = urlInfo.Branch
		case github.URLTypePR:
			if ghRunner == nil {
				return WorktreeAddErrMsg{Err: fmt.Errorf("gh CLI is not available; cannot resolve PR URL")}
			}
			prBranch, err := github.FetchPRBranch(ghRunner, repoPath, rawURL)
			if err != nil {
				return WorktreeAddErrMsg{Err: fmt.Errorf("resolving PR branch: %w", err)}
			}
			branch = prBranch
		}

		return createWorktreeFromBranch(runner, repoPath, basePath, repoName, branch)
	}
}

func addWorktreeFromBranchNameCmd(runner git.CommandRunner, repoPath, basePath, repoName, branch string) tea.Cmd {
	return func() tea.Msg {
		return createWorktreeFromBranch(runner, repoPath, basePath, repoName, branch)
	}
}

func createWorktreeFromBranch(runner git.CommandRunner, repoPath, basePath, repoName, branch string) tea.Msg {
	if err := git.FetchBranch(runner, repoPath, branch); err != nil {
		return WorktreeAddErrMsg{Err: fmt.Errorf("fetching branch %q: %w", branch, err)}
	}

	slug := github.BranchSlug(branch)
	newPath := filepath.Join(basePath, repoName, slug)

	if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
		return WorktreeAddErrMsg{Err: fmt.Errorf("creating parent directory: %w", err)}
	}

	if err := git.AddWorktreeFromBranch(runner, repoPath, newPath, branch); err != nil {
		return WorktreeAddErrMsg{Err: fmt.Errorf("creating worktree: %w", err)}
	}

	return WorktreeAddedMsg{
		WorktreePath: newPath,
		Branch:       branch,
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

func fetchAgentStatusCmd(tmuxRunner tmux.Runner, gitRunner git.CommandRunner, groups []model.RepoGroup) tea.Cmd {
	return func() tea.Msg {
		var getBranch tmux.BranchGetter
		if gitRunner != nil {
			getBranch = func(worktreePath string) (string, error) {
				out, err := gitRunner.Run(worktreePath, "symbolic-ref", "--short", "HEAD")
				if err != nil {
					return "", err
				}
				return strings.TrimSpace(out), nil
			}
		}

		statuses := make(map[string][]model.AgentInfo)
		for _, group := range groups {
			for _, wt := range group.Worktrees {
				sessionName := tmux.ResolveSessionName(tmuxRunner, wt.Path, getBranch)
				agents, err := agent.DetectSessionAgents(tmuxRunner, sessionName)
				if err != nil {
					continue
				}
				if len(agents) > 0 {
					statuses[wt.Path] = agents
				}
			}
		}
		return AgentStatusMsg{Statuses: statuses}
	}
}

func fetchGitDataCmd(cfg model.Config, runner git.CommandRunner) tea.Cmd {
	return func() tea.Msg {
		var groups []model.RepoGroup

		baseRef := cfg.DefaultBaseRef
		if baseRef == "" {
			baseRef = config.DefaultBaseRef
		}

		for _, repoDef := range cfg.Repositories {
			entries, err := git.ListWorktrees(runner, repoDef.Path)
			if err != nil {
				return GitDataErrMsg{Err: err}
			}

			worktrees := git.ToWorktreeInfo(entries)
			for i := range worktrees {
				status, err := git.GetBranchDiffStat(runner, worktrees[i].Path, baseRef)
				if err != nil {
					return GitDataErrMsg{Err: err}
				}
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
