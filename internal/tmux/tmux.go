package tmux

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// IsInsideTmux checks whether the current process is running inside a tmux session.
var IsInsideTmux = func() bool {
	return os.Getenv("TMUX") != ""
}

// FindWindow looks for a tmux window whose name matches the given name.
// Returns the window index if found, or empty string if not.
func FindWindow(runner Runner, windowName string) (string, error) {
	out, err := runner.Run("list-windows", "-F", "#{window_name}\t#{window_index}")
	if err != nil {
		return "", err
	}
	return parseWindowList(out, windowName), nil
}

// SwitchToWindow switches to an existing tmux window by index.
func SwitchToWindow(runner Runner, windowIndex string) error {
	_, err := runner.Run("select-window", "-t", windowIndex)
	return err
}

// CreateWindow creates a new tmux window with the given name and starting directory.
func CreateWindow(runner Runner, windowName string, startDir string) error {
	_, err := runner.Run("new-window", "-n", windowName, "-c", startDir)
	return err
}

// SelectWorktreeWindow finds or creates a tmux window for the given worktree path,
// then switches to it.
func SelectWorktreeWindow(runner Runner, worktreePath string) error {
	windowName := filepath.Base(worktreePath)

	target, err := FindWindow(runner, windowName)
	if err != nil {
		return fmt.Errorf("listing tmux windows: %w", err)
	}

	if target != "" {
		return SwitchToWindow(runner, target)
	}

	return CreateWindow(runner, windowName, worktreePath)
}

// SendKeys sends a command string to the given pane target via tmux send-keys.
// The target should be a pane ID (e.g., "%2") or a session:window.pane reference.
func SendKeys(runner Runner, target string, command string) error {
	_, err := runner.Run("send-keys", "-t", target, command, "Enter")
	if err != nil {
		return fmt.Errorf("sending keys to %s: %w", target, err)
	}
	return nil
}

// SelectPane selects a specific tmux pane by its ID.
func SelectPane(runner Runner, paneID string) error {
	_, err := runner.Run("select-pane", "-t", paneID)
	if err != nil {
		return fmt.Errorf("selecting pane %s: %w", paneID, err)
	}
	return nil
}

// parseWindowList parses `tmux list-windows` output and returns the window index
// for the window matching the given name, or empty string if not found.
func parseWindowList(output string, windowName string) string {
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) == 2 && parts[0] == windowName {
			return parts[1]
		}
	}
	return ""
}
