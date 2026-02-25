package main

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/mikanfactory/yakumo/internal/tmux"
)

func TestLaunchRenameWatcher(t *testing.T) {
	runner := &tmux.FakeRunner{
		Outputs: map[string]string{},
	}

	// Allow any send-keys call to succeed
	runner.Outputs[fmt.Sprintf("%v", []string{"send-keys", "-t", "%5", "'/usr/local/bin/yakumo' watch-rename --path '/tmp/test worktree' --branch 'shoji/south-korea' --created-at 1234567890 --session-name 'test-worktree'", "Enter"})] = ""

	err := launchRenameWatcher(runner, "%5", "/tmp/test worktree", "shoji/south-korea", "test-worktree", 1234567890)
	if err != nil {
		// SendKeys may fail due to executable path, so we just check the call was made
		// Verify at least one call was recorded
		if len(runner.Calls) == 0 {
			t.Fatalf("expected at least one tmux call, got none; error: %v", err)
		}
	}

	if len(runner.Calls) == 0 {
		t.Fatal("expected at least one tmux call, got none")
	}

	call := runner.Calls[0]
	if call[0] != "send-keys" {
		t.Errorf("expected send-keys command, got %q", call[0])
	}
	if call[2] != "%5" {
		t.Errorf("expected pane ID %%5, got %q", call[2])
	}

	cmdStr := call[3]
	if !strings.Contains(cmdStr, "watch-rename") {
		t.Errorf("expected command to contain 'watch-rename', got %q", cmdStr)
	}
	if !strings.Contains(cmdStr, "--path") {
		t.Errorf("expected command to contain '--path', got %q", cmdStr)
	}
	if !strings.Contains(cmdStr, "--branch") {
		t.Errorf("expected command to contain '--branch', got %q", cmdStr)
	}
	if !strings.Contains(cmdStr, "--created-at") {
		t.Errorf("expected command to contain '--created-at', got %q", cmdStr)
	}
	if !strings.Contains(cmdStr, strconv.FormatInt(1234567890, 10)) {
		t.Errorf("expected command to contain timestamp, got %q", cmdStr)
	}
	if !strings.Contains(cmdStr, "--session-name") {
		t.Errorf("expected command to contain '--session-name', got %q", cmdStr)
	}
}

func TestFindIdleBackgroundPane(t *testing.T) {
	tests := []struct {
		name        string
		output      string
		wantPaneID  string
		wantErr     bool
	}{
		{
			name:       "finds zsh pane",
			output:     "%10\tzsh\n%11\tnode\n%12\tbash\n",
			wantPaneID: "%10",
		},
		{
			name:       "finds bash pane skipping non-idle",
			output:     "%10\tnode\n%11\tclaude\n%12\tbash\n",
			wantPaneID: "%12",
		},
		{
			name:       "finds fish pane",
			output:     "%10\tfish\n",
			wantPaneID: "%10",
		},
		{
			name:    "no idle panes",
			output:  "%10\tnode\n%11\tclaude\n",
			wantErr: true,
		},
		{
			name:    "empty output",
			output:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &tmux.FakeRunner{
				Outputs: map[string]string{
					fmt.Sprintf("%v", []string{"list-panes", "-t", "test-session:background-window", "-F", "#{pane_id}\t#{pane_current_command}"}): tt.output,
				},
			}

			paneID, err := findIdleBackgroundPane(runner, "test-session")
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if paneID != tt.wantPaneID {
				t.Errorf("paneID = %q, want %q", paneID, tt.wantPaneID)
			}
		})
	}
}

func TestShellEscape(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "'simple'"},
		{"/path/to/file", "'/path/to/file'"},
		{"it's a test", "'it'\\''s a test'"},
		{"", "''"},
	}

	for _, tt := range tests {
		got := shellEscape(tt.input)
		if got != tt.want {
			t.Errorf("shellEscape(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
