package tmux

import (
	"fmt"
	"os"
	"strings"
)

// CurrentSessionName retrieves the current tmux session name.
// When $TMUX_PANE is set, it is used as an explicit target to ensure the
// correct session is resolved even when multiple tmux sessions exist.
func CurrentSessionName(runner Runner) (string, error) {
	args := []string{"display-message", "-p"}
	if pane := os.Getenv("TMUX_PANE"); pane != "" {
		args = append(args, "-t", pane)
	}
	args = append(args, "#{session_name}")
	out, err := runner.Run(args...)
	if err != nil {
		return "", fmt.Errorf("getting session name: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// SwapCenter swaps center panes between main-window and background-window.
// Replicates the logic from scripts/swap-center.sh.
func SwapCenter(runner Runner) error {
	session, err := CurrentSessionName(runner)
	if err != nil {
		return err
	}

	src1 := session + ":main-window.0"
	dst1 := session + ":background-window.0"
	if _, err := runner.Run("swap-pane", "-d", "-s", src1, "-t", dst1); err != nil {
		return fmt.Errorf("swap center step 1: %w", err)
	}

	src2 := session + ":background-window.0"
	dst2 := session + ":background-window.1"
	if _, err := runner.Run("swap-pane", "-d", "-s", src2, "-t", dst2); err != nil {
		return fmt.Errorf("swap center step 2: %w", err)
	}

	return nil
}

// SwapRightBelow swaps right-below panes between main-window and background-window.
// Replicates the logic from scripts/swap-rb.sh.
func SwapRightBelow(runner Runner) error {
	session, err := CurrentSessionName(runner)
	if err != nil {
		return err
	}

	src1 := session + ":main-window.2"
	dst1 := session + ":background-window.2"
	if _, err := runner.Run("swap-pane", "-d", "-s", src1, "-t", dst1); err != nil {
		return fmt.Errorf("swap right-below step 1: %w", err)
	}

	src2 := session + ":background-window.2"
	dst2 := session + ":background-window.3"
	if _, err := runner.Run("swap-pane", "-d", "-s", src2, "-t", dst2); err != nil {
		return fmt.Errorf("swap right-below step 2: %w", err)
	}

	return nil
}
