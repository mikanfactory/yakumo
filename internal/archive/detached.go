package archive

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// Subcommand is the hidden yakumo subcommand that runs a single archive.
const Subcommand = "archive-worktree"

// Detached returns an archive function that runs `exe archive-worktree` in a
// new session, so the archive keeps running after the caller exits or its
// terminal (e.g. a tmux popup) closes.
func Detached(exe string) func(repoRootPath, worktreePath string) error {
	return func(repoRootPath, worktreePath string) error {
		// A pipe would deliver SIGPIPE to the child once the caller exits, so
		// stderr goes to a file instead.
		errFile, err := os.CreateTemp("", "yakumo-archive-*.log")
		if err != nil {
			return err
		}
		defer os.Remove(errFile.Name())
		defer errFile.Close()

		cmd := exec.Command(exe, Subcommand, "--repo", repoRootPath, "--path", worktreePath)
		cmd.Stderr = errFile
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err := cmd.Run(); err != nil {
			stderr, _ := os.ReadFile(errFile.Name())
			if msg := strings.TrimSpace(string(stderr)); msg != "" {
				return fmt.Errorf("%s", msg)
			}
			return err
		}
		return nil
	}
}
