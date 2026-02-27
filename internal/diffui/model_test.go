package diffui

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mikanfactory/yakumo/internal/tmux"
)

func TestIsShellCommand(t *testing.T) {
	tests := []struct {
		cmd  string
		want bool
	}{
		{"zsh", true},
		{"-zsh", true},
		{"bash", true},
		{"-bash", true},
		{"fish", true},
		{"sh", true},
		{"dash", true},
		{"ksh", true},
		{"ZSH", true},
		{"node", false},
		{"claude", false},
		{"vim", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			got := isShellCommand(tt.cmd)
			if got != tt.want {
				t.Errorf("isShellCommand(%q) = %v, want %v", tt.cmd, got, tt.want)
			}
		})
	}
}

func TestEnterOpensVimInIdleCenterPane(t *testing.T) {
	t.Setenv("TMUX_PANE", "")

	runner := &tmux.FakeRunner{
		Outputs: map[string]string{
			"[display-message -p -t dev:main-window.0 #{pane_current_command}]":       "node",
			"[display-message -p -t dev:background-window.0 #{pane_current_command}]": "zsh",
			"[send-keys -t dev:background-window.0 vim '/repo/file.go' Enter]":        "",
			"[swap-pane -d -s dev:background-window.0 -t dev:main-window.0]":          "",
			"[select-pane -t dev:main-window.0]":                                      "",
		},
	}

	m := Model{
		activeTab:   TabChanges,
		repoDir:     "/repo",
		tmuxRunner:  runner,
		sessionName: "dev",
		changes: ChangesModel{
			files:  []ChangedFile{{Path: "file.go"}},
			cursor: 0,
		},
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a command, got nil")
	}

	result := cmd()

	msg, ok := result.(OpenVimResultMsg)
	if !ok {
		t.Fatalf("expected OpenVimResultMsg, got %T", result)
	}
	if msg.Err != nil {
		t.Errorf("expected no error, got %v", msg.Err)
	}

	// Verify: pane check (main-window.0) + pane check (bg.0) + send-keys + swap-pane + select-pane
	// Session name is cached so no display-message call for it.
	if len(runner.Calls) != 5 {
		t.Fatalf("expected 5 tmux calls, got %d: %v", len(runner.Calls), runner.Calls)
	}
	if runner.Calls[2][0] != "send-keys" {
		t.Errorf("expected send-keys call, got %v", runner.Calls[2])
	}
	if runner.Calls[3][0] != "swap-pane" {
		t.Errorf("expected swap-pane call, got %v", runner.Calls[3])
	}
	if runner.Calls[4][0] != "select-pane" {
		t.Errorf("expected select-pane call, got %v", runner.Calls[4])
	}
}

func TestEnterOpensVimInMainCenterPane_NoSwap(t *testing.T) {
	t.Setenv("TMUX_PANE", "")

	runner := &tmux.FakeRunner{
		Outputs: map[string]string{
			"[display-message -p -t dev:main-window.0 #{pane_current_command}]": "-zsh",
			"[send-keys -t dev:main-window.0 vim '/repo/main.go' Enter]":        "",
			"[select-pane -t dev:main-window.0]":                                "",
		},
	}

	m := Model{
		activeTab:   TabChanges,
		repoDir:     "/repo",
		tmuxRunner:  runner,
		sessionName: "dev",
		changes: ChangesModel{
			files:  []ChangedFile{{Path: "main.go"}},
			cursor: 0,
		},
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	result := cmd()

	msg := result.(OpenVimResultMsg)
	if msg.Err != nil {
		t.Errorf("expected no error, got %v", msg.Err)
	}

	// No swap-pane call since idle pane is already main-window.0
	for _, call := range runner.Calls {
		if call[0] == "swap-pane" {
			t.Error("should not swap when idle pane is already main-window.0")
		}
	}
}

func TestEnterAllCenterPanesBusy(t *testing.T) {
	t.Setenv("TMUX_PANE", "")

	runner := &tmux.FakeRunner{
		Outputs: map[string]string{
			"[display-message -p -t dev:main-window.0 #{pane_current_command}]":       "node",
			"[display-message -p -t dev:background-window.0 #{pane_current_command}]": "claude",
			"[display-message -p -t dev:background-window.1 #{pane_current_command}]": "vim",
		},
	}

	m := Model{
		activeTab:   TabChanges,
		repoDir:     "/repo",
		tmuxRunner:  runner,
		sessionName: "dev",
		changes: ChangesModel{
			files:  []ChangedFile{{Path: "file.go"}},
			cursor: 0,
		},
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	result := cmd()

	msg, ok := result.(OpenVimResultMsg)
	if !ok {
		t.Fatalf("expected OpenVimResultMsg, got %T", result)
	}
	if msg.Err == nil {
		t.Fatal("expected error when all panes are busy")
	}
}

func TestEnterFallsBackToCurrentSessionName(t *testing.T) {
	t.Setenv("TMUX_PANE", "")
	runner := &tmux.FakeRunner{
		Outputs: map[string]string{
			"[display-message -p #{session_name}]":                              "dev",
			"[display-message -p -t dev:main-window.0 #{pane_current_command}]": "-zsh",
			"[send-keys -t dev:main-window.0 vim '/repo/file.go' Enter]":        "",
			"[select-pane -t dev:main-window.0]":                                "",
		},
	}

	m := Model{
		activeTab:  TabChanges,
		repoDir:    "/repo",
		tmuxRunner: runner,
		// sessionName is empty — should fall back to CurrentSessionName
		changes: ChangesModel{
			files:  []ChangedFile{{Path: "file.go"}},
			cursor: 0,
		},
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a command, got nil")
	}

	result := cmd()
	msg, ok := result.(OpenVimResultMsg)
	if !ok {
		t.Fatalf("expected OpenVimResultMsg, got %T", result)
	}
	if msg.Err != nil {
		t.Errorf("expected no error, got %v", msg.Err)
	}

	// Verify: display-message for session + pane check + send-keys + select-pane
	if len(runner.Calls) != 4 {
		t.Fatalf("expected 4 tmux calls, got %d: %v", len(runner.Calls), runner.Calls)
	}
	if runner.Calls[0][0] != "display-message" {
		t.Errorf("expected display-message call for session, got %v", runner.Calls[0])
	}
}

func TestOpenVimResultMsg_SetsStatusMsg(t *testing.T) {
	m := Model{}

	updated, _ := m.Update(OpenVimResultMsg{Err: fmt.Errorf("test error")})
	model := updated.(Model)

	if model.statusMsg != "test error" {
		t.Errorf("expected statusMsg %q, got %q", "test error", model.statusMsg)
	}
}

func TestKeyPress_ClearsStatusMsg(t *testing.T) {
	m := Model{
		statusMsg: "some error",
		activeTab: TabChanges,
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model := updated.(Model)

	if model.statusMsg != "" {
		t.Errorf("expected statusMsg to be cleared, got %q", model.statusMsg)
	}
}

func TestEnterFallsBackWithoutTmux(t *testing.T) {
	m := Model{
		activeTab:  TabChanges,
		repoDir:    "/repo",
		tmuxRunner: nil,
		changes: ChangesModel{
			files:  []ChangedFile{{Path: "file.go"}},
			cursor: 0,
		},
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a command for ExecProcess fallback, got nil")
	}
}

func TestEnterOnEmptyFileList(t *testing.T) {
	m := Model{
		activeTab:  TabChanges,
		tmuxRunner: &tmux.FakeRunner{},
		changes:    ChangesModel{files: []ChangedFile{}},
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("expected nil command when file list is empty")
	}
}

func TestEnterOnChecksTab(t *testing.T) {
	m := Model{
		activeTab:  TabChecks,
		tmuxRunner: &tmux.FakeRunner{},
		changes: ChangesModel{
			files: []ChangedFile{{Path: "file.go"}},
		},
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("expected nil command when on Checks tab")
	}
}

func TestOKeyOpensPR_OnChecksTab(t *testing.T) {
	m := Model{
		activeTab: TabChecks,
		checks: ChecksModel{
			prURL: "https://github.com/owner/repo/pull/1",
		},
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if cmd == nil {
		t.Fatal("expected a command for opening PR, got nil")
	}

	result := cmd()
	msg, ok := result.(OpenPRResultMsg)
	if !ok {
		t.Fatalf("expected OpenPRResultMsg, got %T", result)
	}
	// cmd.Start() will fail in test env (no browser), but the type is correct
	_ = msg
}

func TestOKeyNoop_WhenPRURLEmpty(t *testing.T) {
	m := Model{
		activeTab: TabChecks,
		checks: ChecksModel{
			prURL: "",
		},
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if cmd != nil {
		t.Error("expected nil command when prURL is empty")
	}
}

func TestOKeyNoop_OnChangesTab(t *testing.T) {
	m := Model{
		activeTab: TabChanges,
		checks: ChecksModel{
			prURL: "https://github.com/owner/repo/pull/1",
		},
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if cmd != nil {
		t.Error("expected nil command when on Changes tab")
	}
}
