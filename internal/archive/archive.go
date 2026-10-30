package archive

import (
	"log"
	"os"
	"strings"

	"github.com/mikanfactory/yakumo/internal/git"
	"github.com/mikanfactory/yakumo/internal/tmux"
)

// Options configures the removal of a worktree and its tmux session.
type Options struct {
	// TmuxRunner is used to kill the worktree's session. Nil skips tmux cleanup.
	TmuxRunner   tmux.Runner
	GitRunner    git.CommandRunner
	RepoRootPath string
	WorktreePath string
}

// Run kills the worktree's tmux session and removes the worktree.
func Run(opts Options) error {
	if opts.TmuxRunner != nil {
		// Processes inside the worktree would block git worktree remove.
		sessionName := tmux.ResolveSessionName(opts.TmuxRunner, opts.WorktreePath, branchGetter(opts.GitRunner))

		if tmux.IsCurrentSession(opts.TmuxRunner, sessionName) {
			if err := tmux.SwitchToMainSession(opts.TmuxRunner); err != nil {
				log.Printf("[archive] switch to main session failed (non-fatal): %v", err)
			}
		}

		tmux.KillSession(opts.TmuxRunner, sessionName) // ignore error (session may not exist)
	}

	if err := git.RemoveWorktree(opts.GitRunner, opts.RepoRootPath, opts.WorktreePath); err != nil {
		return err
	}

	if _, err := os.Stat(opts.WorktreePath); err == nil {
		os.RemoveAll(opts.WorktreePath)
	}
	return nil
}

func branchGetter(runner git.CommandRunner) tmux.BranchGetter {
	if runner == nil {
		return nil
	}
	return func(worktreePath string) (string, error) {
		out, err := runner.Run(worktreePath, "symbolic-ref", "--short", "HEAD")
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(out), nil
	}
}
