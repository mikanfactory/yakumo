package tmux

import (
	"fmt"
	"path/filepath"
	"strings"
)

// PaneArea identifies a logical area in the layout.
type PaneArea int

const (
	PaneAreaCenter      PaneArea = iota
	PaneAreaTopRight
	PaneAreaBottomRight
)

const (
	mainWindowName       = "main-window"
	backgroundWindowName = "background-window"
)

// Pane represents a single tmux pane with its area, index, and tmux pane ID.
type Pane struct {
	Area   PaneArea
	Index  int    // 1-based: center-1, center-2, etc.
	PaneID string // e.g., "%0", "%1"
}

// SessionLayout holds all pane references for a worktree session.
type SessionLayout struct {
	SessionName  string
	Center1      Pane
	TopRight1    Pane
	BottomRight1 Pane
	Center2      Pane
	Center3      Pane
	BottomRight2 Pane
	BottomRight3 Pane
}

// parsePaneIDs parses the output of `tmux list-panes -F '#{pane_id}'` into a slice of pane ID strings.
func parsePaneIDs(output string) []string {
	var ids []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			ids = append(ids, line)
		}
	}
	return ids
}

// buildSessionLayout constructs a SessionLayout from captured pane IDs.
// mainPaneIDs must have exactly 3 elements (center-1, tr-1, br-1).
// bgPaneIDs must have exactly 4 elements (center-2, center-3, br-2, br-3).
func buildSessionLayout(sessionName string, mainPaneIDs []string, bgPaneIDs []string) (SessionLayout, error) {
	if len(mainPaneIDs) != 3 {
		return SessionLayout{}, fmt.Errorf("expected 3 main-window panes, got %d", len(mainPaneIDs))
	}
	if len(bgPaneIDs) != 4 {
		return SessionLayout{}, fmt.Errorf("expected 4 background-window panes, got %d", len(bgPaneIDs))
	}

	return SessionLayout{
		SessionName:  sessionName,
		Center1:      Pane{Area: PaneAreaCenter, Index: 1, PaneID: mainPaneIDs[0]},
		TopRight1:    Pane{Area: PaneAreaTopRight, Index: 1, PaneID: mainPaneIDs[1]},
		BottomRight1: Pane{Area: PaneAreaBottomRight, Index: 1, PaneID: mainPaneIDs[2]},
		Center2:      Pane{Area: PaneAreaCenter, Index: 2, PaneID: bgPaneIDs[0]},
		Center3:      Pane{Area: PaneAreaCenter, Index: 3, PaneID: bgPaneIDs[1]},
		BottomRight2: Pane{Area: PaneAreaBottomRight, Index: 2, PaneID: bgPaneIDs[2]},
		BottomRight3: Pane{Area: PaneAreaBottomRight, Index: 3, PaneID: bgPaneIDs[3]},
	}, nil
}

// HasSession checks if a tmux session with the given name exists.
func HasSession(runner Runner, sessionName string) (bool, error) {
	_, err := runner.Run("has-session", "-t", sessionName)
	if err != nil {
		return false, nil
	}
	return true, nil
}

// KillSession terminates a tmux session with the given name.
func KillSession(runner Runner, sessionName string) error {
	_, err := runner.Run("kill-session", "-t", sessionName)
	return err
}

// RenameSession renames a tmux session.
func RenameSession(runner Runner, oldName, newName string) error {
	_, err := runner.Run("rename-session", "-t", oldName, newName)
	return err
}

// BranchGetter returns the current git branch for a worktree path.
type BranchGetter func(worktreePath string) (string, error)

// ResolveSessionName determines the tmux session name for a worktree.
// It first checks for a session matching filepath.Base(worktreePath),
// then checks for a session matching the branch slug (e.g. "fix-login" from "shoji/fix-login").
func ResolveSessionName(runner Runner, worktreePath string, getBranch BranchGetter) string {
	defaultName := filepath.Base(worktreePath)
	if exists, _ := HasSession(runner, defaultName); exists {
		return defaultName
	}
	if getBranch == nil {
		return defaultName
	}
	branch, err := getBranch(worktreePath)
	if err != nil || branch == "" {
		return defaultName
	}
	slug := branch
	if parts := strings.SplitN(branch, "/", 2); len(parts) == 2 {
		slug = parts[1]
	}
	if exists, _ := HasSession(runner, slug); exists {
		return slug
	}
	return defaultName
}

// SwitchToSession switches the client to an existing session and selects the main-window.
func SwitchToSession(runner Runner, sessionName string) error {
	if _, err := runner.Run("switch-client", "-t", sessionName); err != nil {
		return fmt.Errorf("switching to session %s: %w", sessionName, err)
	}
	if _, err := runner.Run("select-window", "-t", sessionName+":"+mainWindowName); err != nil {
		return fmt.Errorf("selecting main-window in session %s: %w", sessionName, err)
	}
	return nil
}

// listPaneIDs fetches pane IDs for a specific window in a session.
func listPaneIDs(runner Runner, sessionName string, windowName string) ([]string, error) {
	target := sessionName + ":" + windowName
	out, err := runner.Run("list-panes", "-t", target, "-F", "#{pane_id}")
	if err != nil {
		return nil, fmt.Errorf("listing panes for %s: %w", target, err)
	}
	return parsePaneIDs(out), nil
}

// createMainWindow sets up the 3-pane layout in the initial window.
// Layout:
//
//	+------------------+----------+
//	|                  | tr-1     |
//	|  center-1        +----------+
//	|                  | br-1     |
//	+------------------+----------+
func createMainWindow(runner Runner, sessionName string, startDir string) error {
	sessionTarget := sessionName + ":0"

	if _, err := runner.Run("rename-window", "-t", sessionTarget, mainWindowName); err != nil {
		return fmt.Errorf("renaming window to %s: %w", mainWindowName, err)
	}

	mainTarget := sessionName + ":" + mainWindowName

	if _, err := runner.Run("split-window", "-h", "-t", mainTarget, "-c", startDir, "-p", "25"); err != nil {
		return fmt.Errorf("creating right column split: %w", err)
	}

	if _, err := runner.Run("split-window", "-v", "-t", mainTarget+".1", "-c", startDir); err != nil {
		return fmt.Errorf("creating bottom-right split: %w", err)
	}

	return nil
}

// createBackgroundWindow creates the background window with 4 panes.
func createBackgroundWindow(runner Runner, sessionName string, startDir string) error {
	if _, err := runner.Run("new-window", "-t", sessionName, "-n", backgroundWindowName, "-c", startDir); err != nil {
		return fmt.Errorf("creating background window: %w", err)
	}

	bgTarget := sessionName + ":" + backgroundWindowName

	for i := 0; i < 3; i++ {
		if _, err := runner.Run("split-window", "-v", "-t", bgTarget, "-c", startDir); err != nil {
			return fmt.Errorf("creating background pane %d: %w", i+2, err)
		}
	}

	return nil
}

// CreateSessionLayout creates a full session with main-window (3 panes) and
// background-window (5 panes), returning a SessionLayout with all pane IDs.
// If startupCommand is non-empty, it is sent to the initial pane before splitting.
func CreateSessionLayout(runner Runner, sessionName string, startDir string, startupCommand string) (SessionLayout, error) {
	if _, err := runner.Run("new-session", "-d", "-s", sessionName, "-c", startDir); err != nil {
		return SessionLayout{}, fmt.Errorf("creating session %s: %w", sessionName, err)
	}

	if startupCommand != "" {
		if _, err := runner.Run("run-shell", "-c", startDir, startupCommand); err != nil {
			// Non-fatal: startup command failure should not block session creation
		}
	}

	if err := createMainWindow(runner, sessionName, startDir); err != nil {
		return SessionLayout{}, err
	}

	mainPaneIDs, err := listPaneIDs(runner, sessionName, mainWindowName)
	if err != nil {
		return SessionLayout{}, err
	}

	if err := createBackgroundWindow(runner, sessionName, startDir); err != nil {
		return SessionLayout{}, err
	}

	bgPaneIDs, err := listPaneIDs(runner, sessionName, backgroundWindowName)
	if err != nil {
		return SessionLayout{}, err
	}

	return buildSessionLayout(sessionName, mainPaneIDs, bgPaneIDs)
}

// SelectWorktreeSession finds or creates a tmux session for the given worktree path.
// If the session already exists, it switches to it.
// If not, it creates the full layout and switches to the new session.
// startupCommand is sent to the initial pane before splitting (only for new sessions).
// getBranch is optional; when provided, it is used to resolve renamed sessions.
func SelectWorktreeSession(runner Runner, worktreePath string, startupCommand string, getBranch BranchGetter) (SessionLayout, error) {
	sessionName := ResolveSessionName(runner, worktreePath, getBranch)

	exists, _ := HasSession(runner, sessionName)

	if exists {
		if err := SwitchToSession(runner, sessionName); err != nil {
			return SessionLayout{}, err
		}
		return SessionLayout{SessionName: sessionName}, nil
	}

	// For new sessions, use the default name (filepath.Base)
	newSessionName := filepath.Base(worktreePath)
	layout, err := CreateSessionLayout(runner, newSessionName, worktreePath, startupCommand)
	if err != nil {
		return SessionLayout{}, fmt.Errorf("creating session layout: %w", err)
	}

	if err := SwitchToSession(runner, newSessionName); err != nil {
		return layout, fmt.Errorf("switching to new session: %w", err)
	}

	return layout, nil
}
