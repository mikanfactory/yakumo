package archive

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mikanfactory/yakumo/internal/git"
	"github.com/mikanfactory/yakumo/internal/tmux"
)

func TestRun_KillsSessionAndRemovesWorktree(t *testing.T) {
	runner := git.FakeCommandRunner{
		Outputs: map[string]string{
			"/repo:[worktree remove /tmp/old-worktree]": "",
		},
	}
	tmuxRunner := &tmux.FakeRunner{
		Outputs: map[string]string{
			"[kill-session -t =old-worktree]": "",
		},
	}

	err := Run(Options{TmuxRunner: tmuxRunner, GitRunner: runner, RepoRootPath: "/repo", WorktreePath: "/tmp/old-worktree"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, call := range tmuxRunner.Calls {
		if call[0] == "kill-session" {
			found = true
		}
	}
	if !found {
		t.Error("expected kill-session to be called")
	}
}

func TestRun_GitError(t *testing.T) {
	runner := git.FakeCommandRunner{Outputs: map[string]string{}}

	err := Run(Options{TmuxRunner: &tmux.FakeRunner{}, GitRunner: runner, RepoRootPath: "/repo", WorktreePath: "/tmp/old-worktree"})
	if err == nil {
		t.Fatal("expected error when git worktree remove fails")
	}
}

func TestRun_NilTmuxRunner(t *testing.T) {
	runner := git.FakeCommandRunner{
		Outputs: map[string]string{
			"/repo:[worktree remove /tmp/old-worktree]": "",
		},
	}

	if err := Run(Options{GitRunner: runner, RepoRootPath: "/repo", WorktreePath: "/tmp/old-worktree"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_RemovesLeftoverDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	worktreePath := filepath.Join(tmpDir, "leftover-wt")
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	runner := git.FakeCommandRunner{
		Outputs: map[string]string{
			fmt.Sprintf("%s:[worktree remove %s]", tmpDir, worktreePath): "",
		},
	}

	if err := Run(Options{GitRunner: runner, RepoRootPath: tmpDir, WorktreePath: worktreePath}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Error("expected worktree directory to be removed")
	}
}

func TestRun_SwitchesToMainWhenInCurrentSession(t *testing.T) {
	t.Setenv("TMUX_PANE", "")
	runner := git.FakeCommandRunner{
		Outputs: map[string]string{
			"/repo:[worktree remove /tmp/south-korea]": "",
		},
	}
	tmuxRunner := &tmux.FakeRunner{
		Outputs: map[string]string{
			"[has-session -t =south-korea]":        "",
			"[display-message -p #{session_name}]": "south-korea\n",
			"[has-session -t =yakumo-main]":        "",
			"[switch-client -t yakumo-main]":       "",
			"[kill-session -t south-korea]":        "",
		},
	}

	if err := Run(Options{TmuxRunner: tmuxRunner, GitRunner: runner, RepoRootPath: "/repo", WorktreePath: "/tmp/south-korea"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	switchIdx, killIdx := -1, -1
	for i, call := range tmuxRunner.Calls {
		if len(call) >= 3 && call[0] == "switch-client" && call[2] == "yakumo-main" {
			switchIdx = i
		}
		if call[0] == "kill-session" {
			killIdx = i
		}
	}
	if switchIdx == -1 || killIdx == -1 || switchIdx >= killIdx {
		t.Errorf("expected switch-client (idx=%d) before kill-session (idx=%d)", switchIdx, killIdx)
	}
}
