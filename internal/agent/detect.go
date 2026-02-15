package agent

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"worktree-ui/internal/model"
	"worktree-ui/internal/tmux"
)

// PaneInfo holds raw tmux data for a single pane.
type PaneInfo struct {
	PaneID         string
	PaneTitle      string
	CurrentCommand string
}

var (
	// Version pattern: e.g. "2.1.34", "10.0.1"
	versionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

	// Running patterns (tcmux-compatible)
	runningPattern          = regexp.MustCompile(`(?m)^[✢✽✶✻·]\s+.+?…?\s*\([^)]*·\s*((?:\d+[smh]\s*)+)`)
	runningPatternTimeFirst = regexp.MustCompile(`(?m)^[✢✽✶✻·]\s+.+?…?\s*\(((?:\d+[smh]\s*)+)\s*·`)
	runningFallbackPattern  = regexp.MustCompile(`(?m)^[✢✽✶✻·]\s+.+?…?\s*\((esc|ctrl\+c) to interrupt`)

	// Waiting patterns
	waitingPatterns = []string{
		"Yes, allow once",
		"Yes, allow always",
		"Yes, don't ask again",
		"Do you trust",
		"Run this command?",
		"Continue?",
		"(Y/n)",
		"(y/N)",
		"[Y/n]",
		"[y/N]",
		"(yes/no)",
	}

	// Idle pattern
	idlePattern = regexp.MustCompile(`(?m)^\s*❯`)
)

// isClaudeProcess returns true if the pane_current_command indicates Claude Code.
func isClaudeProcess(command string) bool {
	cmd := strings.ToLower(command)
	if cmd == "node" || cmd == "claude" {
		return true
	}
	return versionPattern.MatchString(command)
}

// isClaudeTitle returns true if the pane title starts with Claude Code indicators.
func isClaudeTitle(title string) bool {
	if title == "" {
		return false
	}
	r, _ := utf8.DecodeRuneInString(title)
	// ✳ U+2733 sparkle (idle indicator)
	if r == '\u2733' {
		return true
	}
	// Braille pattern range U+2800-U+28FF (spinner)
	if r >= '\u2800' && r <= '\u28FF' {
		return true
	}
	return false
}

// isClaude returns true if either title or process detection indicates Claude Code.
func isClaude(info PaneInfo) bool {
	return isClaudeTitle(info.PaneTitle) || isClaudeProcess(info.CurrentCommand)
}

// parseAllPanes parses the output of list-panes with tab-separated format.
func parseAllPanes(output string) []PaneInfo {
	var panes []PaneInfo
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		panes = append(panes, PaneInfo{
			PaneID:         parts[0],
			PaneTitle:      parts[1],
			CurrentCommand: parts[2],
		})
	}
	return panes
}

// lastNonEmptyLines returns the last n non-empty, non-separator lines from a slice.
func lastNonEmptyLines(lines []string, n int) []string {
	var result []string
	for i := len(lines) - 1; i >= 0 && len(result) < n; i-- {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		result = append(result, line)
	}
	// Reverse to maintain order
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result
}

// DetectState reads pane content via capture-pane and determines agent state.
func DetectState(runner tmux.Runner, paneID string) (model.AgentState, string, error) {
	out, err := runner.Run("capture-pane", "-p", "-t", paneID)
	if err != nil {
		return model.AgentStateNone, "", err
	}

	lines := strings.Split(out, "\n")
	meaningful := lastNonEmptyLines(lines, 30)
	content := strings.Join(meaningful, "\n")

	// Check running patterns (highest priority after modes)
	if matches := runningPattern.FindStringSubmatch(content); len(matches) > 1 {
		return model.AgentStateRunning, strings.TrimSpace(matches[1]), nil
	}

	if matches := runningPatternTimeFirst.FindStringSubmatch(content); len(matches) > 1 {
		return model.AgentStateRunning, strings.TrimSpace(matches[1]), nil
	}

	if runningFallbackPattern.MatchString(content) {
		return model.AgentStateRunning, "", nil
	}

	// Check waiting patterns
	for _, pattern := range waitingPatterns {
		if strings.Contains(content, pattern) {
			return model.AgentStateWaiting, "", nil
		}
	}

	// Check idle pattern
	if idlePattern.MatchString(content) {
		return model.AgentStateIdle, "", nil
	}

	return model.AgentStateNone, "", nil
}

// DetectSessionAgents checks all panes in a tmux session for Claude Code instances.
// Returns nil if the session does not exist.
func DetectSessionAgents(runner tmux.Runner, sessionName string) ([]model.AgentInfo, error) {
	exists, _ := tmux.HasSession(runner, sessionName)
	if !exists {
		return nil, nil
	}

	out, err := runner.Run("list-panes", "-s", "-t", sessionName, "-F", "#{pane_id}\t#{pane_title}\t#{pane_current_command}")
	if err != nil {
		return nil, err
	}

	panes := parseAllPanes(out)
	var agents []model.AgentInfo

	for _, pane := range panes {
		if !isClaude(pane) {
			continue
		}

		state, elapsed, err := DetectState(runner, pane.PaneID)
		if err != nil {
			continue
		}

		agents = append(agents, model.AgentInfo{
			PaneID:  pane.PaneID,
			State:   state,
			Elapsed: elapsed,
		})
	}

	return agents, nil
}
