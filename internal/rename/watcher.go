package rename

import (
	"fmt"
	"strings"
	"time"

	"github.com/mikanfactory/yakumo/internal/branchname"
	"github.com/mikanfactory/yakumo/internal/claude"
	"github.com/mikanfactory/yakumo/internal/git"
)

// WatcherConfig holds the parameters for a rename watcher.
type WatcherConfig struct {
	WorktreePath string
	Branch       string
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
}

// NewWatcher creates a new rename watcher.
func NewWatcher(cfg WatcherConfig, reader claude.Reader, gen branchname.Generator, runner git.CommandRunner) *Watcher {
	return &Watcher{
		config:    cfg,
		reader:    reader,
		generator: gen,
		runner:    runner,
	}
}

// Run polls for a first prompt and renames the branch when found.
// Returns nil on success, or an error on timeout / rename failure.
func (w *Watcher) Run() error {
	deadline := time.Now().Add(w.config.Timeout)

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout: no prompt detected within %v", w.config.Timeout)
		}

		prompt, found := w.findPrompt()
		if found {
			return w.renameBranch(prompt)
		}

		time.Sleep(w.config.PollInterval)
	}
}

func (w *Watcher) findPrompt() (string, bool) {
	data, err := w.reader.ReadHistoryFile()
	if err != nil {
		return "", false
	}
	entries, err := claude.ParseHistory(data)
	if err != nil {
		return "", false
	}
	prompt, _, found := claude.FindFirstPrompt(entries, w.config.WorktreePath, w.config.CreatedAt)
	return prompt, found
}

func (w *Watcher) renameBranch(prompt string) error {
	name, err := w.generator.GenerateBranchName(prompt)
	if err != nil {
		return fmt.Errorf("generating branch name: %w", err)
	}

	sanitized := branchname.SanitizeBranchName(name)
	if sanitized == "" {
		return fmt.Errorf("generated branch name is empty")
	}

	// Preserve username prefix: "shoji/south-korea" -> "shoji/fix-login"
	newBranch := sanitized
	if parts := strings.SplitN(w.config.Branch, "/", 2); len(parts) == 2 {
		newBranch = parts[0] + "/" + sanitized
	}

	if err := git.RenameBranch(w.runner, w.config.WorktreePath, w.config.Branch, newBranch); err != nil {
		return fmt.Errorf("renaming branch: %w", err)
	}

	return nil
}
