package tui

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/mikanfactory/yakumo/internal/agent"
	"github.com/mikanfactory/yakumo/internal/branchname"
	"github.com/mikanfactory/yakumo/internal/claude"
	"github.com/mikanfactory/yakumo/internal/config"
	"github.com/mikanfactory/yakumo/internal/git"
	"github.com/mikanfactory/yakumo/internal/github"
	"github.com/mikanfactory/yakumo/internal/model"
	"github.com/mikanfactory/yakumo/internal/sidebar"
	"github.com/mikanfactory/yakumo/internal/tmux"
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
type WorktreeAddedMsg struct {
	WorktreePath string
	Branch       string
	CreatedAt    int64 // Unix milliseconds
}

// BranchRenameStartMsg indicates a first prompt was detected for a worktree.
type BranchRenameStartMsg struct {
	WorktreePath string
	Prompt       string
	SessionID    string
}

// BranchRenameResultMsg carries the result of the LLM + git branch rename.
type BranchRenameResultMsg struct {
	WorktreePath string
	NewBranch    string
	Err          error
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

// renameTimeoutMs is how long to wait for a prompt before giving up (10 minutes).
const renameTimeoutMs = 10 * 60 * 1000

// Model is the BubbleTea model for the sidebar.
type Model struct {
	items                    []model.NavigableItem
	groups                   []model.RepoGroup
	cursor                   int
	sidebarWidth             int
	selected                 string
	selectedRepoPath         string
	quitting                 bool
	err                      error
	config                   model.Config
	runner                   git.CommandRunner
	loading                  bool
	addingRepo               bool
	addingWorktree           bool
	addingWorktreeRepoPath   string
	textInput                textinput.Model
	configPath               string
	tmuxRunner               tmux.Runner
	ghRunner                 github.Runner
	agentStatus              map[string][]model.AgentInfo
	branchRenames            map[string]model.BranchRenameInfo
	claudeReader             claude.Reader
	branchNameGen            branchname.Generator
	confirmingArchive        bool
	archiveTarget            int
}

// NewModel creates a new TUI model.
// tmuxRunner may be nil when running outside tmux (agent polling is skipped).
// ghRunner may be nil when gh CLI is not available (PR URL cloning is skipped).
// claudeReader and branchNameGen may be nil to disable LLM branch naming.
func NewModel(cfg model.Config, runner git.CommandRunner, configPath string, tmuxRunner tmux.Runner, ghRunner github.Runner, claudeReader claude.Reader, branchNameGen branchname.Generator) Model {
	ti := textinput.New()
	ti.Placeholder = "/path/to/repository"
	ti.CharLimit = 256
	ti.Width = 50

	var renames map[string]model.BranchRenameInfo
	if claudeReader != nil && branchNameGen != nil {
		renames = make(map[string]model.BranchRenameInfo)
	}

	return Model{
		sidebarWidth:  cfg.SidebarWidth,
		config:        cfg,
		runner:        runner,
		loading:       true,
		configPath:    configPath,
		textInput:     ti,
		tmuxRunner:    tmuxRunner,
		ghRunner:      ghRunner,
		branchRenames: renames,
		claudeReader:  claudeReader,
		branchNameGen: branchNameGen,
	}
}

// Selected returns the selected worktree path, if any.
func (m Model) Selected() string {
	return m.selected
}

// SelectedRepoPath returns the repository root path for the selected worktree.
func (m Model) SelectedRepoPath() string {
	return m.selectedRepoPath
}

// PendingRename returns the BranchRenameInfo for the given worktree path
// if it is in pending status. Returns nil otherwise.
func (m Model) PendingRename(worktreePath string) *model.BranchRenameInfo {
	if m.branchRenames == nil {
		return nil
	}
	for path, info := range m.branchRenames {
		if path == worktreePath && info.Status == model.RenameStatusPending {
			return &info
		}
	}
	return nil
}

func (m Model) Init() tea.Cmd {
	return fetchGitDataCmd(m.config, m.runner)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

		var cmds []tea.Cmd
		cmds = append(cmds, agentTickCmd())

		now := time.Now().UnixMilli()
		for path, info := range m.branchRenames {
			if info.Status != model.RenameStatusPending {
				continue
			}
			if now-info.CreatedAt > renameTimeoutMs {
				log.Printf("[branch-rename] timeout: path=%q elapsed=%dms", path, now-info.CreatedAt)
				info.Status = model.RenameStatusSkipped
				m.branchRenames[path] = info
				continue
			}
			log.Printf("[branch-rename] polling: path=%q elapsed=%dms", path, now-info.CreatedAt)
			cmds = append(cmds, checkPromptCmd(m.claudeReader, path, info.CreatedAt))
		}

		return m, tea.Batch(cmds...)

	case GitDataErrMsg:
		m.err = msg.Err
		m.loading = false
		return m, nil

	case WorktreeAddedMsg:
		m.loading = true
		if m.branchRenames != nil && msg.WorktreePath != "" {
			log.Printf("[branch-rename] WorktreeAdded: path=%q branch=%q createdAt=%d", msg.WorktreePath, msg.Branch, msg.CreatedAt)
			m.branchRenames[msg.WorktreePath] = model.BranchRenameInfo{
				Status:         model.RenameStatusPending,
				OriginalBranch: msg.Branch,
				WorktreePath:   msg.WorktreePath,
				CreatedAt:      msg.CreatedAt,
			}
		} else if m.branchRenames == nil {
			log.Printf("[branch-rename] WorktreeAdded: feature disabled (branchRenames=nil)")
		}
		return m, fetchGitDataCmd(m.config, m.runner)

	case BranchRenameStartMsg:
		if info, ok := m.branchRenames[msg.WorktreePath]; ok && info.Status == model.RenameStatusPending {
			info.Status = model.RenameStatusDetected
			info.FirstPrompt = msg.Prompt
			info.SessionID = msg.SessionID
			m.branchRenames[msg.WorktreePath] = info
			return m, renameBranchCmd(m.branchNameGen, m.runner, msg.WorktreePath, info.OriginalBranch, msg.Prompt)
		}
		return m, nil

	case BranchRenameResultMsg:
		if info, ok := m.branchRenames[msg.WorktreePath]; ok {
			if msg.Err != nil {
				info.Status = model.RenameStatusFailed
			} else {
				info.Status = model.RenameStatusCompleted
				info.NewBranch = msg.NewBranch
			}
			m.branchRenames[msg.WorktreePath] = info
		}
		if msg.Err == nil {
			m.loading = true
			return m, fetchGitDataCmd(m.config, m.runner)
		}
		return m, nil

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
						m.selectedRepoPath = item.RepoRootPath
						return m, tea.Quit
					}
					if item.Kind == model.ItemKindAddWorktree {
						m.addingWorktree = true
						m.addingWorktreeRepoPath = item.RepoRootPath
						m.err = nil
						m.textInput.Placeholder = "URL to clone or Enter for new branch"
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
					m.selectedRepoPath = item.RepoRootPath
					return m, tea.Quit
				}
				if item.Kind == model.ItemKindAddWorktree {
					m.addingWorktree = true
					m.addingWorktreeRepoPath = item.RepoRootPath
					m.err = nil
					m.textInput.Placeholder = "URL to clone or Enter for new branch"
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
				// Empty input: create worktree with random branch name
				return m, addWorktreeCmd(m.runner, m.addingWorktreeRepoPath, m.config.WorktreeBasePath, repoName)
			}
			// URL input: clone from URL
			return m, addWorktreeFromURLCmd(m.runner, m.ghRunner, m.addingWorktreeRepoPath, m.config.WorktreeBasePath, repoName, input)
		case tea.KeyCtrlC:
			m.quitting = true
			return m, tea.Quit
		}

	case WorktreeAddedMsg:
		m.loading = true
		m.addingWorktree = false
		if m.branchRenames != nil && msg.WorktreePath != "" {
			m.branchRenames[msg.WorktreePath] = model.BranchRenameInfo{
				Status:         model.RenameStatusPending,
				OriginalBranch: msg.Branch,
				WorktreePath:   msg.WorktreePath,
				CreatedAt:      msg.CreatedAt,
			}
		}
		return m, fetchGitDataCmd(m.config, m.runner)

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
			item := m.items[m.archiveTarget]
			m.loading = true
			m.err = nil
			return m, archiveWorktreeCmd(m.runner, m.tmuxRunner, item.RepoRootPath, item.WorktreePath)
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

func archiveWorktreeCmd(runner git.CommandRunner, tmuxRunner tmux.Runner, repoRootPath, worktreePath string) tea.Cmd {
	return func() tea.Msg {
		// Kill tmux session first (processes inside worktree would block git worktree remove)
		sessionName := filepath.Base(worktreePath)
		if tmuxRunner != nil {
			tmux.KillSession(tmuxRunner, sessionName) // ignore error (session may not exist)
		}

		if err := git.RemoveWorktree(runner, repoRootPath, worktreePath); err != nil {
			return WorktreeArchiveErrMsg{Err: err}
		}

		// Clean up directory if it still remains
		if _, err := os.Stat(worktreePath); err == nil {
			os.RemoveAll(worktreePath)
		}

		return WorktreeArchivedMsg{}
	}
}

func repoNameFromConfig(cfg model.Config, repoPath string) string {
	for _, repo := range cfg.Repositories {
		if repo.Path == repoPath {
			return repo.Name
		}
	}
	return filepath.Base(repoPath)
}

func addWorktreeCmd(runner git.CommandRunner, repoPath, basePath, repoName string) tea.Cmd {
	return func() tea.Msg {
		userName, err := git.GetUserName(runner, repoPath)
		if err != nil {
			return WorktreeAddErrMsg{Err: err}
		}

		country := git.RandomCountry()
		slug := git.Slugify(country)
		branch := userName + "/" + slug
		newPath := filepath.Join(basePath, repoName, slug)
		createdAt := time.Now().UnixMilli()

		if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
			return WorktreeAddErrMsg{Err: fmt.Errorf("creating parent directory: %w", err)}
		}

		if err := git.AddWorktree(runner, repoPath, newPath, branch); err != nil {
			return WorktreeAddErrMsg{Err: err}
		}

		return WorktreeAddedMsg{
			WorktreePath: newPath,
			Branch:       branch,
			CreatedAt:    createdAt,
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
			CreatedAt:    time.Now().UnixMilli(),
		}
	}
}

func checkPromptCmd(reader claude.Reader, worktreePath string, createdAt int64) tea.Cmd {
	return func() tea.Msg {
		data, err := reader.ReadHistoryFile()
		if err != nil {
			log.Printf("[branch-rename] checkPrompt: ReadHistoryFile error: %v", err)
			return nil
		}
		entries, err := claude.ParseHistory(data)
		if err != nil {
			log.Printf("[branch-rename] checkPrompt: ParseHistory error: %v", err)
			return nil
		}
		prompt, sessionID, found := claude.FindFirstPrompt(entries, worktreePath, createdAt)
		if !found {
			log.Printf("[branch-rename] checkPrompt: no prompt found for path=%q afterTimestamp=%d (entries=%d)", worktreePath, createdAt, len(entries))
			return nil
		}
		log.Printf("[branch-rename] checkPrompt: found prompt=%q sessionID=%q for path=%q", prompt, sessionID, worktreePath)
		return BranchRenameStartMsg{
			WorktreePath: worktreePath,
			Prompt:       prompt,
			SessionID:    sessionID,
		}
	}
}

func renameBranchCmd(gen branchname.Generator, runner git.CommandRunner, worktreePath, originalBranch, prompt string) tea.Cmd {
	return func() tea.Msg {
		log.Printf("[branch-rename] renameBranch: generating name for prompt=%q", prompt)
		name, err := gen.GenerateBranchName(prompt)
		if err != nil {
			log.Printf("[branch-rename] renameBranch: GenerateBranchName error: %v", err)
			return BranchRenameResultMsg{WorktreePath: worktreePath, Err: err}
		}

		sanitized := branchname.SanitizeBranchName(name)
		if sanitized == "" {
			log.Printf("[branch-rename] renameBranch: SanitizeBranchName returned empty for raw=%q", name)
			return BranchRenameResultMsg{WorktreePath: worktreePath, Err: fmt.Errorf("generated branch name is empty")}
		}

		// Preserve username prefix: "shoji/south-korea" -> "shoji/fix-login"
		newBranch := sanitized
		if parts := strings.SplitN(originalBranch, "/", 2); len(parts) == 2 {
			newBranch = parts[0] + "/" + sanitized
		}

		log.Printf("[branch-rename] renameBranch: renaming %q -> %q in %q", originalBranch, newBranch, worktreePath)
		if err := git.RenameBranch(runner, worktreePath, originalBranch, newBranch); err != nil {
			log.Printf("[branch-rename] renameBranch: RenameBranch error: %v", err)
			return BranchRenameResultMsg{WorktreePath: worktreePath, Err: err}
		}

		log.Printf("[branch-rename] renameBranch: success %q -> %q", originalBranch, newBranch)
		return BranchRenameResultMsg{WorktreePath: worktreePath, NewBranch: newBranch}
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
