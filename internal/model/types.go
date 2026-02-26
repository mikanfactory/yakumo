package model

// Config represents the application configuration loaded from YAML.
type Config struct {
	SidebarWidth     int             `yaml:"sidebar_width"`
	DefaultBaseRef   string          `yaml:"default_base_ref"`
	Repositories     []RepositoryDef `yaml:"repositories"`
	WorktreeBasePath string          `yaml:"worktree_base_path"`
}

// RepositoryDef represents a repository entry from config.
type RepositoryDef struct {
	Name           string   `yaml:"name"`
	Path           string   `yaml:"path"`
	StartupCommand string   `yaml:"startup_command,omitempty"`
	RbCommands     []string `yaml:"rb_commands,omitempty"`
}

// RepoGroup represents a repository and all its discovered worktrees.
type RepoGroup struct {
	Name      string
	RootPath  string
	Worktrees []WorktreeInfo
}

// WorktreeInfo represents a single git worktree with its status.
type WorktreeInfo struct {
	Path   string
	Branch string
	Status StatusInfo
	IsBare bool
}

// StatusInfo holds the aggregated line change counts for a worktree.
type StatusInfo struct {
	Insertions int
	Deletions  int
}

// AgentState represents the current state of a Claude Code agent in a tmux pane.
type AgentState int

const (
	AgentStateNone    AgentState = iota // No Claude Code detected
	AgentStateIdle                      // Idle (prompt visible, ready for input)
	AgentStateRunning                   // Actively executing (spinner visible)
	AgentStateWaiting                   // Waiting for user permission/confirmation
)

// AgentInfo holds the detected status of a Claude Code instance in a single pane.
type AgentInfo struct {
	PaneID  string
	State   AgentState
	Elapsed string // e.g. "2m 30s", populated only when Running
}

// ItemKind identifies what type of navigation item this is.
type ItemKind int

const (
	ItemKindGroupHeader ItemKind = iota
	ItemKindWorktree
	ItemKindAddWorktree
	ItemKindAddRepo
	ItemKindSettings
)

// RenameStatus tracks the branch rename lifecycle.
type RenameStatus int

const (
	RenameStatusPending   RenameStatus = iota // Waiting for first prompt
	RenameStatusDetected                      // Prompt found, LLM call in flight
	RenameStatusCompleted                     // Branch has been renamed
	RenameStatusFailed                        // LLM or rename failed, keeping country name
	RenameStatusSkipped                       // No prompt detected within timeout
)

// BranchRenameInfo tracks the state of an LLM-based branch rename for one worktree.
type BranchRenameInfo struct {
	Status         RenameStatus
	OriginalBranch string
	NewBranch      string
	WorktreePath   string
	CreatedAt      int64  // Unix milliseconds when worktree was created
	SessionID      string // Claude session ID once detected
	FirstPrompt    string // The user's first prompt text
}

// NavigableItem is a flattened item in the sidebar list used for navigation.
type NavigableItem struct {
	Kind         ItemKind
	Label        string
	Selectable   bool
	WorktreePath string
	RepoRootPath string
	Status       StatusInfo
	AgentStatus  []AgentInfo
	IsBare       bool
}
