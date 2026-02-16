package model

// Config represents the application configuration loaded from YAML.
type Config struct {
	SidebarWidth    int             `yaml:"sidebar_width"`
	Repositories    []RepositoryDef `yaml:"repositories"`
	WorktreeBasePath string         `yaml:"worktree_base_path"`
}

// RepositoryDef represents a repository entry from config.
type RepositoryDef struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
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
