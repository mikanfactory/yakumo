package agent

import (
	"fmt"
	"testing"

	"worktree-ui/internal/model"
	"worktree-ui/internal/tmux"
)

func TestIsClaudeProcess(t *testing.T) {
	tests := []struct {
		command string
		want    bool
	}{
		{"node", true},
		{"claude", true},
		{"2.1.34", true},
		{"10.0.1", true},
		{"bash", false},
		{"vim", false},
		{"zsh", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got := isClaudeProcess(tt.command)
			if got != tt.want {
				t.Errorf("isClaudeProcess(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

func TestIsClaudeTitle(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  bool
	}{
		{"sparkle idle indicator", "✳ claude-code", true},
		{"braille spinner low", "\u2800 running", true},
		{"braille spinner mid", "\u2840 task", true},
		{"braille spinner high", "\u28FF active", true},
		{"normal bash title", "bash", false},
		{"empty title", "", false},
		{"vim title", "vim main.go", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isClaudeTitle(tt.title)
			if got != tt.want {
				t.Errorf("isClaudeTitle(%q) = %v, want %v", tt.title, got, tt.want)
			}
		})
	}
}

func TestIsClaude(t *testing.T) {
	tests := []struct {
		name string
		info PaneInfo
		want bool
	}{
		{
			"title match only",
			PaneInfo{PaneID: "%0", PaneTitle: "✳ claude", CurrentCommand: "bash"},
			true,
		},
		{
			"process match only",
			PaneInfo{PaneID: "%1", PaneTitle: "some title", CurrentCommand: "node"},
			true,
		},
		{
			"both match",
			PaneInfo{PaneID: "%2", PaneTitle: "✳ claude", CurrentCommand: "node"},
			true,
		},
		{
			"neither match",
			PaneInfo{PaneID: "%3", PaneTitle: "bash", CurrentCommand: "bash"},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isClaude(tt.info)
			if got != tt.want {
				t.Errorf("isClaude(%+v) = %v, want %v", tt.info, got, tt.want)
			}
		})
	}
}

func TestParseAllPanes(t *testing.T) {
	output := "%0\t✳ claude\tnode\n%1\tbash\tbash\n%2\t\u2840 running\tclaude\n"

	panes := parseAllPanes(output)

	if len(panes) != 3 {
		t.Fatalf("expected 3 panes, got %d", len(panes))
	}

	expected := []PaneInfo{
		{PaneID: "%0", PaneTitle: "✳ claude", CurrentCommand: "node"},
		{PaneID: "%1", PaneTitle: "bash", CurrentCommand: "bash"},
		{PaneID: "%2", PaneTitle: "\u2840 running", CurrentCommand: "claude"},
	}

	for i, want := range expected {
		got := panes[i]
		if got.PaneID != want.PaneID || got.PaneTitle != want.PaneTitle || got.CurrentCommand != want.CurrentCommand {
			t.Errorf("pane[%d] = %+v, want %+v", i, got, want)
		}
	}
}

func TestParseAllPanes_Empty(t *testing.T) {
	panes := parseAllPanes("")
	if len(panes) != 0 {
		t.Errorf("expected 0 panes, got %d", len(panes))
	}
}

func TestDetectState_Running(t *testing.T) {
	// Simulated capture-pane output with a running action and elapsed time
	captureOutput := `
  some previous output

✻ Reading file… (esc to interrupt · 2m 30s · internal/main.go)

`

	runner := &tmux.FakeRunner{
		Outputs: map[string]string{
			fmt.Sprintf("%v", []string{"capture-pane", "-p", "-t", "%0"}): captureOutput,
		},
	}

	state, elapsed, err := DetectState(runner, "%0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != model.AgentStateRunning {
		t.Errorf("state = %v, want Running", state)
	}
	if elapsed == "" {
		t.Error("expected elapsed time to be non-empty")
	}
}

func TestDetectState_RunningTimeFirst(t *testing.T) {
	captureOutput := `
✻ Editing file… (1m 52s · esc to interrupt)

`

	runner := &tmux.FakeRunner{
		Outputs: map[string]string{
			fmt.Sprintf("%v", []string{"capture-pane", "-p", "-t", "%0"}): captureOutput,
		},
	}

	state, elapsed, err := DetectState(runner, "%0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != model.AgentStateRunning {
		t.Errorf("state = %v, want Running", state)
	}
	if elapsed == "" {
		t.Error("expected elapsed time to be non-empty")
	}
}

func TestDetectState_RunningFallback(t *testing.T) {
	captureOutput := `
✻ Reading file… (esc to interrupt)

`

	runner := &tmux.FakeRunner{
		Outputs: map[string]string{
			fmt.Sprintf("%v", []string{"capture-pane", "-p", "-t", "%0"}): captureOutput,
		},
	}

	state, _, err := DetectState(runner, "%0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != model.AgentStateRunning {
		t.Errorf("state = %v, want Running", state)
	}
}

func TestDetectState_Waiting(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			"allow once prompt",
			"  Yes, allow once\n  Yes, allow always\n  No, skip this step\n",
		},
		{
			"Y/n prompt",
			"  Do you want to proceed? (Y/n)\n",
		},
		{
			"yes/no prompt",
			"  Continue? (yes/no)\n",
		},
		{
			"trust prompt",
			"  Do you trust the files in this folder?\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &tmux.FakeRunner{
				Outputs: map[string]string{
					fmt.Sprintf("%v", []string{"capture-pane", "-p", "-t", "%0"}): tt.content,
				},
			}

			state, _, err := DetectState(runner, "%0")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if state != model.AgentStateWaiting {
				t.Errorf("state = %v, want Waiting for content %q", state, tt.name)
			}
		})
	}
}

func TestDetectState_Idle(t *testing.T) {
	captureOutput := `  some output

  ❯ `

	runner := &tmux.FakeRunner{
		Outputs: map[string]string{
			fmt.Sprintf("%v", []string{"capture-pane", "-p", "-t", "%0"}): captureOutput,
		},
	}

	state, _, err := DetectState(runner, "%0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != model.AgentStateIdle {
		t.Errorf("state = %v, want Idle", state)
	}
}

func TestDetectSessionAgents_NoSession(t *testing.T) {
	runner := &tmux.FakeRunner{
		Errors: map[string]error{
			fmt.Sprintf("%v", []string{"has-session", "-t", "my-session"}): fmt.Errorf("no session"),
		},
	}

	agents, err := DetectSessionAgents(runner, "my-session")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agents != nil {
		t.Errorf("expected nil agents for non-existent session, got %v", agents)
	}
}

func TestDetectSessionAgents_NoClaude(t *testing.T) {
	runner := &tmux.FakeRunner{
		Outputs: map[string]string{
			fmt.Sprintf("%v", []string{"has-session", "-t", "my-session"}):                                                                       "",
			fmt.Sprintf("%v", []string{"list-panes", "-s", "-t", "my-session", "-F", "#{pane_id}\t#{pane_title}\t#{pane_current_command}"}):       "%0\tbash\tbash\n%1\tvim\tvim\n",
		},
	}

	agents, err := DetectSessionAgents(runner, "my-session")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(agents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(agents))
	}
}

func TestDetectSessionAgents_OneClaude(t *testing.T) {
	captureIdle := "  ❯ "

	runner := &tmux.FakeRunner{
		Outputs: map[string]string{
			fmt.Sprintf("%v", []string{"has-session", "-t", "my-session"}):                                                                 "",
			fmt.Sprintf("%v", []string{"list-panes", "-s", "-t", "my-session", "-F", "#{pane_id}\t#{pane_title}\t#{pane_current_command}"}): "%0\t✳ claude\tnode\n%1\tbash\tbash\n",
			fmt.Sprintf("%v", []string{"capture-pane", "-p", "-t", "%0"}):                                                                  captureIdle,
		},
	}

	agents, err := DetectSessionAgents(runner, "my-session")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	if agents[0].PaneID != "%0" {
		t.Errorf("agent PaneID = %q, want %%0", agents[0].PaneID)
	}
	if agents[0].State != model.AgentStateIdle {
		t.Errorf("agent State = %v, want Idle", agents[0].State)
	}
}

func TestDetectSessionAgents_MultipleClaude(t *testing.T) {
	captureIdle := "  ❯ "
	captureRunning := "✻ Reading file… (esc to interrupt · 1m 30s · main.go)\n"

	runner := &tmux.FakeRunner{
		Outputs: map[string]string{
			fmt.Sprintf("%v", []string{"has-session", "-t", "my-session"}):                                                                 "",
			fmt.Sprintf("%v", []string{"list-panes", "-s", "-t", "my-session", "-F", "#{pane_id}\t#{pane_title}\t#{pane_current_command}"}): "%0\t✳ claude\tnode\n%1\t\u2840 task\tclaude\n%2\tbash\tbash\n",
			fmt.Sprintf("%v", []string{"capture-pane", "-p", "-t", "%0"}):                                                                  captureIdle,
			fmt.Sprintf("%v", []string{"capture-pane", "-p", "-t", "%1"}):                                                                  captureRunning,
		},
	}

	agents, err := DetectSessionAgents(runner, "my-session")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(agents))
	}

	// First agent should be idle
	if agents[0].State != model.AgentStateIdle {
		t.Errorf("agent[0] State = %v, want Idle", agents[0].State)
	}

	// Second agent should be running
	if agents[1].State != model.AgentStateRunning {
		t.Errorf("agent[1] State = %v, want Running", agents[1].State)
	}
}
