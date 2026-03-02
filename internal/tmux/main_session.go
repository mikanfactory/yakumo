package tmux

import (
	"fmt"
	"os"
)

// MainSessionName is the name of the yakumo main tmux session.
const MainSessionName = "yakumo-main"

const yakumoASCIIArt = ` ██╗   ██╗  █████╗  ██╗  ██╗ ██╗   ██╗ ███╗   ███╗  ██████╗
 ╚██╗ ██╔╝ ██╔══██╗ ██║ ██╔╝ ██║   ██║ ████╗ ████║ ██╔═══██╗
  ╚████╔╝  ███████║ █████╔╝  ██║   ██║ ██╔████╔██║ ██║   ██║
   ╚██╔╝   ██╔══██║ ██╔═██╗  ██║   ██║ ██║╚██╔╝██║ ██║   ██║
    ██║    ██║  ██║ ██║  ██╗ ╚██████╔╝ ██║ ╚═╝ ██║ ╚██████╔╝
    ╚═╝    ╚═╝  ╚═╝ ╚═╝  ╚═╝  ╚═════╝  ╚═╝     ╚═╝  ╚═════╝`

// EnsureMainSession checks if the yakumo main session exists, and creates it if not.
// When creating a new session, it sends the ASCII art banner via echo.
func EnsureMainSession(runner Runner) error {
	exists, err := HasSession(runner, MainSessionName)
	if err != nil {
		return fmt.Errorf("checking main session: %w", err)
	}
	if exists {
		return nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "/"
	}

	if _, err := runner.Run("new-session", "-d", "-s", MainSessionName, "-c", homeDir); err != nil {
		return fmt.Errorf("creating main session: %w", err)
	}

	// Display ASCII art banner (non-fatal)
	SendKeys(runner, MainSessionName, "echo '"+yakumoASCIIArt+"'")

	return nil
}

// IsCurrentSession returns true if the given session name matches the current tmux session.
func IsCurrentSession(runner Runner, sessionName string) bool {
	current, err := CurrentSessionName(runner)
	if err != nil {
		return false
	}
	return current == sessionName
}

// SwitchToMainSession ensures the main session exists and switches the client to it.
func SwitchToMainSession(runner Runner) error {
	if err := EnsureMainSession(runner); err != nil {
		return err
	}
	if _, err := runner.Run("switch-client", "-t", MainSessionName); err != nil {
		return fmt.Errorf("switching to main session: %w", err)
	}
	return nil
}
