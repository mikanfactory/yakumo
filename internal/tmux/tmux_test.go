package tmux

import (
	"fmt"
	"testing"
)

func TestParseWindowList(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		windowName string
		want       string
	}{
		{
			name:       "window found",
			output:     "main\t0\nfeature-x\t1\nhotfix\t2\n",
			windowName: "feature-x",
			want:       "1",
		},
		{
			name:       "window not found",
			output:     "main\t0\ndev\t1\n",
			windowName: "feature-x",
			want:       "",
		},
		{
			name:       "empty output",
			output:     "",
			windowName: "main",
			want:       "",
		},
		{
			name:       "partial name match should not match",
			output:     "feature-x-v2\t1\n",
			windowName: "feature-x",
			want:       "",
		},
		{
			name:       "first window",
			output:     "target\t0\nother\t1\n",
			windowName: "target",
			want:       "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseWindowList(tt.output, tt.windowName)
			if got != tt.want {
				t.Errorf("parseWindowList() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFindWindow(t *testing.T) {
	runner := &FakeRunner{
		Outputs: map[string]string{
			"[list-windows -F #{window_name}\t#{window_index}]": "main\t0\nmy-feature\t1\n",
		},
	}

	target, err := FindWindow(runner, "my-feature")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target != "1" {
		t.Errorf("expected target %q, got %q", "1", target)
	}
}

func TestFindWindow_NotFound(t *testing.T) {
	runner := &FakeRunner{
		Outputs: map[string]string{
			"[list-windows -F #{window_name}\t#{window_index}]": "main\t0\n",
		},
	}

	target, err := FindWindow(runner, "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target != "" {
		t.Errorf("expected empty target, got %q", target)
	}
}

func TestFindWindow_Error(t *testing.T) {
	runner := &FakeRunner{
		Errors: map[string]error{
			"[list-windows -F #{window_name}\t#{window_index}]": fmt.Errorf("tmux not running"),
		},
	}

	_, err := FindWindow(runner, "any")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSwitchToWindow(t *testing.T) {
	runner := &FakeRunner{
		Outputs: map[string]string{
			"[select-window -t 1]": "",
		},
	}

	err := SwitchToWindow(runner, "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSwitchToWindow_Error(t *testing.T) {
	runner := &FakeRunner{
		Errors: map[string]error{
			"[select-window -t 99]": fmt.Errorf("window not found"),
		},
	}

	err := SwitchToWindow(runner, "99")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCreateWindow(t *testing.T) {
	runner := &FakeRunner{
		Outputs: map[string]string{
			"[new-window -n my-worktree -c /path/to/worktree]": "",
		},
	}

	err := CreateWindow(runner, "my-worktree", "/path/to/worktree")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateWindow_Error(t *testing.T) {
	runner := &FakeRunner{
		Errors: map[string]error{
			"[new-window -n fail -c /bad]": fmt.Errorf("tmux error"),
		},
	}

	err := CreateWindow(runner, "fail", "/bad")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSelectWorktreeWindow_ExistingWindow(t *testing.T) {
	runner := &FakeRunner{
		Outputs: map[string]string{
			"[list-windows -F #{window_name}\t#{window_index}]": "main\t0\nmy-worktree\t2\n",
			"[select-window -t 2]": "",
		},
	}

	err := SelectWorktreeWindow(runner, "/repos/my-worktree")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.Calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(runner.Calls))
	}
	// Should not call new-window
	for _, call := range runner.Calls {
		if call[0] == "new-window" {
			t.Error("should not create new window when existing one found")
		}
	}
}

func TestSelectWorktreeWindow_NewWindow(t *testing.T) {
	runner := &FakeRunner{
		Outputs: map[string]string{
			"[list-windows -F #{window_name}\t#{window_index}]":                   "main\t0\n",
			"[new-window -n my-worktree -c /repos/my-worktree]": "",
		},
	}

	err := SelectWorktreeWindow(runner, "/repos/my-worktree")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.Calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(runner.Calls))
	}
	if runner.Calls[1][0] != "new-window" {
		t.Errorf("expected new-window call, got %v", runner.Calls[1])
	}
}

func TestSelectWorktreeWindow_ListError(t *testing.T) {
	runner := &FakeRunner{
		Errors: map[string]error{
			"[list-windows -F #{window_name}\t#{window_index}]": fmt.Errorf("tmux error"),
		},
	}

	err := SelectWorktreeWindow(runner, "/repos/any")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSendKeys_Success(t *testing.T) {
	runner := &FakeRunner{
		Outputs: map[string]string{
			"[send-keys -t %2 npm run dev Enter]": "",
		},
	}

	err := SendKeys(runner, "%2", "npm run dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(runner.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(runner.Calls))
	}
	call := runner.Calls[0]
	expected := []string{"send-keys", "-t", "%2", "npm run dev", "Enter"}
	if len(call) != len(expected) {
		t.Fatalf("call args length = %d, want %d", len(call), len(expected))
	}
	for i := range expected {
		if call[i] != expected[i] {
			t.Errorf("call[%d] = %q, want %q", i, call[i], expected[i])
		}
	}
}

func TestSendKeys_Error(t *testing.T) {
	runner := &FakeRunner{
		Errors: map[string]error{
			"[send-keys -t %2 bad-cmd Enter]": fmt.Errorf("pane not found"),
		},
	}

	err := SendKeys(runner, "%2", "bad-cmd")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSelectPane_Success(t *testing.T) {
	runner := &FakeRunner{
		Outputs: map[string]string{
			"[select-pane -t %0]": "",
		},
	}

	err := SelectPane(runner, "%0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(runner.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(runner.Calls))
	}
	call := runner.Calls[0]
	expected := []string{"select-pane", "-t", "%0"}
	if len(call) != len(expected) {
		t.Fatalf("call args length = %d, want %d", len(call), len(expected))
	}
	for i := range expected {
		if call[i] != expected[i] {
			t.Errorf("call[%d] = %q, want %q", i, call[i], expected[i])
		}
	}
}

func TestSelectPane_Error(t *testing.T) {
	runner := &FakeRunner{
		Errors: map[string]error{
			"[select-pane -t %99]": fmt.Errorf("pane not found"),
		},
	}

	err := SelectPane(runner, "%99")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestIsInsideTmux(t *testing.T) {
	original := IsInsideTmux
	t.Cleanup(func() { IsInsideTmux = original })

	IsInsideTmux = func() bool { return true }
	if !IsInsideTmux() {
		t.Error("expected true")
	}

	IsInsideTmux = func() bool { return false }
	if IsInsideTmux() {
		t.Error("expected false")
	}
}
