package sessionsetup

import (
	"fmt"
	"log"
	"strings"

	"github.com/mikanfactory/yakumo/internal/git"
	"github.com/mikanfactory/yakumo/internal/tmux"
)

// Options configures the tmux session setup for a selected worktree.
type Options struct {
	TmuxRunner     tmux.Runner
	GitRunner      git.CommandRunner
	WorktreePath   string
	StartupCommand string
	// DiffUICommand is sent to the top-right pane of a new session. Empty skips it.
	DiffUICommand string
	// ClaudeCommand is sent to the center pane of a new session. Empty skips it.
	ClaudeCommand string
	// TrustDirectory marks the worktree as trusted for Claude before launch. Nil skips it.
	TrustDirectory func(worktreePath string) error
}

// Setup finds or creates the tmux session for the worktree and returns its name
// without switching the client. For newly created sessions it also launches
// diff-ui and Claude and focuses the center pane.
func Setup(opts Options) (string, error) {
	var getBranch tmux.BranchGetter
	if opts.GitRunner != nil {
		getBranch = func(worktreePath string) (string, error) {
			out, err := opts.GitRunner.Run(worktreePath, "symbolic-ref", "--short", "HEAD")
			if err != nil {
				return "", err
			}
			return strings.TrimSpace(out), nil
		}
	}

	layout, err := tmux.EnsureWorktreeSession(opts.TmuxRunner, opts.WorktreePath, opts.StartupCommand, getBranch)
	if err != nil {
		return "", fmt.Errorf("tmux error: %w", err)
	}

	if layout.BottomRight1.PaneID == "" {
		return layout.SessionName, nil
	}

	if opts.DiffUICommand != "" {
		if err := tmux.SendKeys(opts.TmuxRunner, layout.TopRight1.PaneID, opts.DiffUICommand); err != nil {
			log.Printf("[setup] diff-ui launch error: %v", err)
		}
	}

	if opts.ClaudeCommand != "" {
		if opts.TrustDirectory != nil {
			if err := opts.TrustDirectory(opts.WorktreePath); err != nil {
				log.Printf("[setup] claude trust warning: %v", err)
			}
		}
		if err := tmux.SendKeys(opts.TmuxRunner, layout.Center1.PaneID, opts.ClaudeCommand); err != nil {
			log.Printf("[setup] claude launch error: %v", err)
		}
	}

	if err := tmux.SelectPane(opts.TmuxRunner, layout.Center1.PaneID); err != nil {
		log.Printf("[setup] select pane error: %v", err)
	}

	return layout.SessionName, nil
}
