package rename

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/mikanfactory/yakumo/internal/branchname"
	"github.com/mikanfactory/yakumo/internal/claude"
	"github.com/mikanfactory/yakumo/internal/git"
	"github.com/mikanfactory/yakumo/internal/tmux"
)

// WatcherConfig holds the parameters for a rename watcher.
type WatcherConfig struct {
	WorktreePath string
	Branch       string
	SessionName  string
	CreatedAt    int64
	PollInterval time.Duration
	Timeout      time.Duration
}

// Watcher polls Claude history for a first prompt and renames the branch accordingly.
type Watcher struct {
	config    WatcherConfig
	reader    claude.Reader
	generator branchname.Generator
	runner    git.CommandRunner
	tmuxRunner tmux.Runner
	logger    *log.Logger
}

// NewWatcher creates a new rename watcher.
func NewWatcher(cfg WatcherConfig, reader claude.Reader, gen branchname.Generator, runner git.CommandRunner, tmuxRunner tmux.Runner) *Watcher {
	return &Watcher{
		config:     cfg,
		reader:     reader,
		generator:  gen,
		runner:     runner,
		tmuxRunner: tmuxRunner,
	}
}

// SetLogger sets a logger for the watcher. If nil, logging is disabled.
func (w *Watcher) SetLogger(l *log.Logger) {
	w.logger = l
}

func (w *Watcher) logf(format string, args ...interface{}) {
	if w.logger != nil {
		w.logger.Printf("[branch-rename] "+format, args...)
	}
}

// Run polls for a first prompt and renames the branch when found.
// Returns nil on success, or an error on timeout / rename failure.
func (w *Watcher) Run() error {
	w.logf("started: path=%q branch=%q createdAt=%d timeout=%v", w.config.WorktreePath, w.config.Branch, w.config.CreatedAt, w.config.Timeout)
	deadline := time.Now().Add(w.config.Timeout)

	for {
		if time.Now().After(deadline) {
			w.logf("timeout: no prompt detected within %v for path=%q", w.config.Timeout, w.config.WorktreePath)
			return fmt.Errorf("timeout: no prompt detected within %v", w.config.Timeout)
		}

		w.logf("polling: path=%q elapsed=%dms", w.config.WorktreePath, time.Now().UnixMilli()-w.config.CreatedAt)
		prompt, found := w.findPrompt()
		if found {
			w.logf("prompt detected: %q for path=%q", prompt, w.config.WorktreePath)
			return w.renameBranch(prompt)
		}

		time.Sleep(w.config.PollInterval)
	}
}

func (w *Watcher) findPrompt() (string, bool) {
	data, err := w.reader.ReadHistoryFile()
	if err != nil {
		w.logf("findPrompt: ReadHistoryFile error: %v", err)
		return "", false
	}
	entries, err := claude.ParseHistory(data)
	if err != nil {
		w.logf("findPrompt: ParseHistory error: %v", err)
		return "", false
	}
	prompt, _, found := claude.FindFirstPrompt(entries, w.config.WorktreePath, w.config.CreatedAt)
	if !found {
		w.logf("findPrompt: no prompt found for path=%q afterTimestamp=%d (entries=%d)", w.config.WorktreePath, w.config.CreatedAt, len(entries))
	}
	return prompt, found
}

func (w *Watcher) renameBranch(prompt string) error {
	w.logf("renameBranch: generating name for prompt=%q", prompt)
	name, err := w.generator.GenerateBranchName(prompt)
	if err != nil {
		w.logf("renameBranch: GenerateBranchName error: %v", err)
		return fmt.Errorf("generating branch name: %w", err)
	}

	sanitized := branchname.SanitizeBranchName(name)
	if sanitized == "" {
		w.logf("renameBranch: SanitizeBranchName returned empty for raw=%q", name)
		return fmt.Errorf("generated branch name is empty")
	}

	// Preserve username prefix: "shoji/south-korea" -> "shoji/fix-login"
	newBranch := sanitized
	if parts := strings.SplitN(w.config.Branch, "/", 2); len(parts) == 2 {
		newBranch = parts[0] + "/" + sanitized
	}

	w.logf("renameBranch: renaming %q -> %q in %q", w.config.Branch, newBranch, w.config.WorktreePath)
	if err := git.RenameBranch(w.runner, w.config.WorktreePath, w.config.Branch, newBranch); err != nil {
		w.logf("renameBranch: RenameBranch error: %v", err)
		return fmt.Errorf("renaming branch: %w", err)
	}

	w.logf("renameBranch: success %q -> %q", w.config.Branch, newBranch)

	// Rename tmux session to match the new branch slug (non-fatal)
	if w.tmuxRunner != nil && w.config.SessionName != "" {
		newSessionName := branchname.SlugFromBranch(newBranch)
		if newSessionName != w.config.SessionName {
			if err := tmux.RenameSession(w.tmuxRunner, w.config.SessionName, newSessionName); err != nil {
				w.logf("renameBranch: tmux rename-session failed (non-fatal): %v", err)
			} else {
				w.logf("renameBranch: tmux session renamed %q -> %q", w.config.SessionName, newSessionName)
			}
		}
	}

	return nil
}
