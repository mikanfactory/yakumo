package tmux

import (
	"fmt"
	"testing"
)

// --- parsePaneIDs tests ---

func TestParsePaneIDs_MultiplePanes(t *testing.T) {
	got := parsePaneIDs("%0\n%1\n%2\n")
	want := []string{"%0", "%1", "%2"}
	if len(got) != len(want) {
		t.Fatalf("got %d pane IDs, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("pane[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParsePaneIDs_SinglePane(t *testing.T) {
	got := parsePaneIDs("%5\n")
	if len(got) != 1 || got[0] != "%5" {
		t.Errorf("got %v, want [%%5]", got)
	}
}

func TestParsePaneIDs_EmptyOutput(t *testing.T) {
	got := parsePaneIDs("")
	if len(got) != 0 {
		t.Errorf("got %v, want empty slice", got)
	}
}

func TestParsePaneIDs_TrailingWhitespace(t *testing.T) {
	got := parsePaneIDs("%0\n%1\n  \n")
	want := []string{"%0", "%1"}
	if len(got) != len(want) {
		t.Fatalf("got %d pane IDs, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("pane[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// --- buildSessionLayout tests ---

func TestBuildSessionLayout_ValidInput(t *testing.T) {
	mainIDs := []string{"%0", "%1", "%2"}
	bgIDs := []string{"%3", "%4", "%5", "%6"}

	layout, err := buildSessionLayout("my-session", mainIDs, bgIDs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if layout.SessionName != "my-session" {
		t.Errorf("SessionName = %q, want %q", layout.SessionName, "my-session")
	}

	tests := []struct {
		name   string
		pane   Pane
		area   PaneArea
		index  int
		paneID string
	}{
		{"Center1", layout.Center1, PaneAreaCenter, 1, "%0"},
		{"TopRight1", layout.TopRight1, PaneAreaTopRight, 1, "%1"},
		{"BottomRight1", layout.BottomRight1, PaneAreaBottomRight, 1, "%2"},
		{"Center2", layout.Center2, PaneAreaCenter, 2, "%3"},
		{"Center3", layout.Center3, PaneAreaCenter, 3, "%4"},
		{"BottomRight2", layout.BottomRight2, PaneAreaBottomRight, 2, "%5"},
		{"BottomRight3", layout.BottomRight3, PaneAreaBottomRight, 3, "%6"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.pane.Area != tt.area {
				t.Errorf("Area = %d, want %d", tt.pane.Area, tt.area)
			}
			if tt.pane.Index != tt.index {
				t.Errorf("Index = %d, want %d", tt.pane.Index, tt.index)
			}
			if tt.pane.PaneID != tt.paneID {
				t.Errorf("PaneID = %q, want %q", tt.pane.PaneID, tt.paneID)
			}
		})
	}
}

func TestBuildSessionLayout_WrongMainCount(t *testing.T) {
	_, err := buildSessionLayout("s", []string{"%0", "%1"}, []string{"%3", "%4", "%5", "%6", "%7"})
	if err == nil {
		t.Fatal("expected error for wrong main pane count")
	}
}

func TestBuildSessionLayout_WrongBgCount(t *testing.T) {
	_, err := buildSessionLayout("s", []string{"%0", "%1", "%2"}, []string{"%3", "%4", "%5"})
	if err == nil {
		t.Fatal("expected error for wrong background pane count")
	}
}

// --- HasSession tests ---

func TestHasSession_Exists(t *testing.T) {
	runner := &FakeRunner{
		Outputs: map[string]string{
			"[has-session -t my-session]": "",
		},
	}

	exists, err := HasSession(runner, "my-session")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected session to exist")
	}
}

func TestHasSession_NotExists(t *testing.T) {
	runner := &FakeRunner{
		Errors: map[string]error{
			"[has-session -t nonexistent]": fmt.Errorf("session not found"),
		},
	}

	exists, err := HasSession(runner, "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected session to not exist")
	}
}

// --- KillSession tests ---

func TestKillSession_Success(t *testing.T) {
	runner := &FakeRunner{
		Outputs: map[string]string{
			"[kill-session -t my-session]": "",
		},
	}

	err := KillSession(runner, "my-session")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(runner.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(runner.Calls))
	}
}

func TestKillSession_Error(t *testing.T) {
	runner := &FakeRunner{
		Errors: map[string]error{
			"[kill-session -t nonexistent]": fmt.Errorf("session not found"),
		},
	}

	err := KillSession(runner, "nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- RenameSession tests ---

func TestRenameSession_Success(t *testing.T) {
	runner := &FakeRunner{
		Outputs: map[string]string{
			"[rename-session -t old-name new-name]": "",
		},
	}

	err := RenameSession(runner, "old-name", "new-name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(runner.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(runner.Calls))
	}
}

func TestRenameSession_Error(t *testing.T) {
	runner := &FakeRunner{
		Errors: map[string]error{
			"[rename-session -t nonexistent new-name]": fmt.Errorf("session not found"),
		},
	}

	err := RenameSession(runner, "nonexistent", "new-name")
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- ResolveSessionName tests ---

func TestResolveSessionName_DefaultExists(t *testing.T) {
	runner := &FakeRunner{
		Outputs: map[string]string{
			"[has-session -t south-korea]": "",
		},
	}

	name := ResolveSessionName(runner, "/repos/south-korea", nil)
	if name != "south-korea" {
		t.Errorf("got %q, want %q", name, "south-korea")
	}
}

func TestResolveSessionName_SlugExists(t *testing.T) {
	runner := &FakeRunner{
		Errors: map[string]error{
			"[has-session -t south-korea]": fmt.Errorf("not found"),
		},
		Outputs: map[string]string{
			"[has-session -t fix-login]": "",
		},
	}

	getBranch := func(worktreePath string) (string, error) {
		return "shoji/fix-login", nil
	}

	name := ResolveSessionName(runner, "/repos/south-korea", getBranch)
	if name != "fix-login" {
		t.Errorf("got %q, want %q", name, "fix-login")
	}
}

func TestResolveSessionName_NeitherExists(t *testing.T) {
	runner := &FakeRunner{
		Errors: map[string]error{
			"[has-session -t south-korea]": fmt.Errorf("not found"),
			"[has-session -t fix-login]":   fmt.Errorf("not found"),
		},
	}

	getBranch := func(worktreePath string) (string, error) {
		return "shoji/fix-login", nil
	}

	name := ResolveSessionName(runner, "/repos/south-korea", getBranch)
	if name != "south-korea" {
		t.Errorf("got %q, want %q", name, "south-korea")
	}
}

func TestResolveSessionName_NilBranchGetter(t *testing.T) {
	runner := &FakeRunner{
		Errors: map[string]error{
			"[has-session -t south-korea]": fmt.Errorf("not found"),
		},
	}

	name := ResolveSessionName(runner, "/repos/south-korea", nil)
	if name != "south-korea" {
		t.Errorf("got %q, want %q", name, "south-korea")
	}
}

func TestResolveSessionName_BranchGetterError(t *testing.T) {
	runner := &FakeRunner{
		Errors: map[string]error{
			"[has-session -t south-korea]": fmt.Errorf("not found"),
		},
	}

	getBranch := func(worktreePath string) (string, error) {
		return "", fmt.Errorf("git error")
	}

	name := ResolveSessionName(runner, "/repos/south-korea", getBranch)
	if name != "south-korea" {
		t.Errorf("got %q, want %q", name, "south-korea")
	}
}

func TestResolveSessionName_NoPrefixBranch(t *testing.T) {
	runner := &FakeRunner{
		Errors: map[string]error{
			"[has-session -t south-korea]": fmt.Errorf("not found"),
		},
		Outputs: map[string]string{
			"[has-session -t fix-login]": "",
		},
	}

	getBranch := func(worktreePath string) (string, error) {
		return "fix-login", nil
	}

	name := ResolveSessionName(runner, "/repos/south-korea", getBranch)
	if name != "fix-login" {
		t.Errorf("got %q, want %q", name, "fix-login")
	}
}

// --- SwitchToSession tests ---

func TestSwitchToSession_Success(t *testing.T) {
	runner := &FakeRunner{
		Outputs: map[string]string{
			"[switch-client -t my-session]":                    "",
			"[select-window -t my-session:main-window]": "",
		},
	}

	err := SwitchToSession(runner, "my-session")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(runner.Calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(runner.Calls))
	}
}

func TestSwitchToSession_SwitchError(t *testing.T) {
	runner := &FakeRunner{
		Errors: map[string]error{
			"[switch-client -t bad]": fmt.Errorf("switch failed"),
		},
	}

	err := SwitchToSession(runner, "bad")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSwitchToSession_SelectWindowError(t *testing.T) {
	runner := &FakeRunner{
		Outputs: map[string]string{
			"[switch-client -t my-session]": "",
		},
		Errors: map[string]error{
			"[select-window -t my-session:main-window]": fmt.Errorf("window not found"),
		},
	}

	err := SwitchToSession(runner, "my-session")
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- createMainWindow tests ---

func TestCreateMainWindow_Success(t *testing.T) {
	runner := &FakeRunner{
		Outputs: map[string]string{
			"[rename-window -t my-session:0 main-window]":                    "",
			"[split-window -h -t my-session:main-window -c /path -p 25]":    "",
			"[split-window -v -t my-session:main-window.1 -c /path]": "",
		},
	}

	err := createMainWindow(runner, "my-session", "/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(runner.Calls) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(runner.Calls))
	}
}

func TestCreateMainWindow_RenameError(t *testing.T) {
	runner := &FakeRunner{
		Errors: map[string]error{
			"[rename-window -t s:0 main-window]": fmt.Errorf("rename failed"),
		},
	}

	err := createMainWindow(runner, "s", "/path")
	if err == nil {
		t.Fatal("expected error")
	}
	if len(runner.Calls) != 1 {
		t.Errorf("expected 1 call, got %d", len(runner.Calls))
	}
}

func TestCreateMainWindow_FirstSplitError(t *testing.T) {
	runner := &FakeRunner{
		Outputs: map[string]string{
			"[rename-window -t s:0 main-window]": "",
		},
		Errors: map[string]error{
			"[split-window -h -t s:main-window -c /path -p 25]": fmt.Errorf("split failed"),
		},
	}

	err := createMainWindow(runner, "s", "/path")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCreateMainWindow_SecondSplitError(t *testing.T) {
	runner := &FakeRunner{
		Outputs: map[string]string{
			"[rename-window -t s:0 main-window]":          "",
			"[split-window -h -t s:main-window -c /path -p 25]": "",
		},
		Errors: map[string]error{
			"[split-window -v -t s:main-window.1 -c /path]": fmt.Errorf("split failed"),
		},
	}

	err := createMainWindow(runner, "s", "/path")
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- listPaneIDs tests ---

func TestListPaneIDs_Success(t *testing.T) {
	runner := &FakeRunner{
		Outputs: map[string]string{
			"[list-panes -t s:main-window -F #{pane_id}]": "%0\n%1\n%2\n",
		},
	}

	ids, err := listPaneIDs(runner, "s", "main-window")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("expected 3 IDs, got %d", len(ids))
	}
}

func TestListPaneIDs_Error(t *testing.T) {
	runner := &FakeRunner{
		Errors: map[string]error{
			"[list-panes -t s:w -F #{pane_id}]": fmt.Errorf("list failed"),
		},
	}

	_, err := listPaneIDs(runner, "s", "w")
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- createBackgroundWindow tests ---

func TestCreateBackgroundWindow_Success(t *testing.T) {
	runner := &FakeRunner{
		Outputs: map[string]string{
			"[new-window -t s -n background-window -c /path]":          "",
			"[split-window -v -t s:background-window -c /path]": "",
		},
	}

	err := createBackgroundWindow(runner, "s", "/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 1 new-window + 3 split-window = 4 calls
	if len(runner.Calls) != 4 {
		t.Fatalf("expected 4 calls, got %d", len(runner.Calls))
	}
}

func TestCreateBackgroundWindow_NewWindowError(t *testing.T) {
	runner := &FakeRunner{
		Errors: map[string]error{
			"[new-window -t s -n background-window -c /path]": fmt.Errorf("new-window failed"),
		},
	}

	err := createBackgroundWindow(runner, "s", "/path")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCreateBackgroundWindow_SplitError(t *testing.T) {
	runner := &FakeRunner{
		Outputs: map[string]string{
			"[new-window -t s -n background-window -c /path]": "",
		},
		Errors: map[string]error{
			"[split-window -v -t s:background-window -c /path]": fmt.Errorf("split failed"),
		},
	}

	err := createBackgroundWindow(runner, "s", "/path")
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- CreateSessionLayout tests ---

func newFullSessionRunner(session string, dir string) *FakeRunner {
	return &FakeRunner{
		Outputs: map[string]string{
			fmt.Sprintf("[new-session -d -s %s -c %s]", session, dir):                             "",
			fmt.Sprintf("[rename-window -t %s:0 main-window]", session):                            "",
			fmt.Sprintf("[split-window -h -t %s:main-window -c %s -p 25]", session, dir):            "",
			fmt.Sprintf("[split-window -v -t %s:main-window.1 -c %s]", session, dir):               "",
			fmt.Sprintf("[list-panes -t %s:main-window -F #{pane_id}]", session):                   "%0\n%1\n%2\n",
			fmt.Sprintf("[new-window -t %s -n background-window -c %s]", session, dir):             "",
			fmt.Sprintf("[split-window -v -t %s:background-window -c %s]", session, dir):           "",
			fmt.Sprintf("[list-panes -t %s:background-window -F #{pane_id}]", session):             "%3\n%4\n%5\n%6\n",
		},
	}
}

func TestCreateSessionLayout_Success(t *testing.T) {
	runner := newFullSessionRunner("feat", "/repos/feat")

	layout, err := CreateSessionLayout(runner, "feat", "/repos/feat", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if layout.SessionName != "feat" {
		t.Errorf("SessionName = %q, want %q", layout.SessionName, "feat")
	}
	if layout.Center1.PaneID != "%0" {
		t.Errorf("Center1.PaneID = %q, want %%0", layout.Center1.PaneID)
	}
	if layout.BottomRight3.PaneID != "%6" {
		t.Errorf("BottomRight3.PaneID = %q, want %%6", layout.BottomRight3.PaneID)
	}
}

func TestCreateSessionLayout_NewSessionError(t *testing.T) {
	runner := &FakeRunner{
		Errors: map[string]error{
			"[new-session -d -s s -c /p]": fmt.Errorf("session error"),
		},
	}

	_, err := CreateSessionLayout(runner, "s", "/p", "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCreateSessionLayout_MainWindowError(t *testing.T) {
	runner := &FakeRunner{
		Outputs: map[string]string{
			"[new-session -d -s s -c /p]": "",
		},
		Errors: map[string]error{
			"[rename-window -t s:0 main-window]": fmt.Errorf("rename error"),
		},
	}

	_, err := CreateSessionLayout(runner, "s", "/p", "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCreateSessionLayout_ListMainPanesError(t *testing.T) {
	runner := &FakeRunner{
		Outputs: map[string]string{
			"[new-session -d -s s -c /p]":                  "",
			"[rename-window -t s:0 main-window]":           "",
			"[split-window -h -t s:main-window -c /p -p 25]":     "",
			"[split-window -v -t s:main-window.1 -c /p]":   "",
		},
		Errors: map[string]error{
			"[list-panes -t s:main-window -F #{pane_id}]": fmt.Errorf("list error"),
		},
	}

	_, err := CreateSessionLayout(runner, "s", "/p", "")
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- CreateSessionLayout startup command tests ---

func TestCreateSessionLayout_WithStartupCommand(t *testing.T) {
	runner := newFullSessionRunner("feat", "/repos/feat")
	// Add run-shell for startup command
	runner.Outputs["[run-shell -c /repos/feat npm run dev]"] = ""

	layout, err := CreateSessionLayout(runner, "feat", "/repos/feat", "npm run dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if layout.Center1.PaneID != "%0" {
		t.Errorf("Center1.PaneID = %q, want %%0", layout.Center1.PaneID)
	}

	// Verify run-shell was called before rename-window (split)
	runShellIdx := -1
	renameIdx := -1
	for i, call := range runner.Calls {
		if len(call) >= 1 && call[0] == "run-shell" {
			runShellIdx = i
		}
		if len(call) >= 1 && call[0] == "rename-window" {
			renameIdx = i
		}
	}
	if runShellIdx == -1 {
		t.Fatal("expected run-shell call for startup command")
	}
	if renameIdx == -1 {
		t.Fatal("expected rename-window call")
	}
	if runShellIdx >= renameIdx {
		t.Errorf("run-shell (idx=%d) should be called before rename-window (idx=%d)", runShellIdx, renameIdx)
	}
}

func TestCreateSessionLayout_EmptyStartupCommand(t *testing.T) {
	runner := newFullSessionRunner("feat", "/repos/feat")

	_, err := CreateSessionLayout(runner, "feat", "/repos/feat", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify no run-shell was called
	for _, call := range runner.Calls {
		if call[0] == "run-shell" {
			t.Error("should not call run-shell when startup command is empty")
		}
	}
}

// --- SelectWorktreeSession tests ---

func TestSelectWorktreeSession_ExistingSession(t *testing.T) {
	runner := &FakeRunner{
		Outputs: map[string]string{
			"[has-session -t my-worktree]":                    "",
			"[switch-client -t my-worktree]":                  "",
			"[select-window -t my-worktree:main-window]":      "",
		},
	}

	layout, err := SelectWorktreeSession(runner, "/repos/my-worktree", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if layout.SessionName != "my-worktree" {
		t.Errorf("SessionName = %q, want %q", layout.SessionName, "my-worktree")
	}
	// Should NOT call new-session
	for _, call := range runner.Calls {
		if call[0] == "new-session" {
			t.Error("should not create new session when existing one found")
		}
	}
}

func TestSelectWorktreeSession_NewSession(t *testing.T) {
	runner := &FakeRunner{
		Errors: map[string]error{
			"[has-session -t feat]": fmt.Errorf("not found"),
		},
		Outputs: map[string]string{
			"[new-session -d -s feat -c /repos/feat]":                              "",
			"[rename-window -t feat:0 main-window]":                                "",
			"[split-window -h -t feat:main-window -c /repos/feat -p 25]":                 "",
			"[split-window -v -t feat:main-window.1 -c /repos/feat]":               "",
			"[list-panes -t feat:main-window -F #{pane_id}]":                       "%0\n%1\n%2\n",
			"[new-window -t feat -n background-window -c /repos/feat]":             "",
			"[split-window -v -t feat:background-window -c /repos/feat]":           "",
			"[list-panes -t feat:background-window -F #{pane_id}]":                 "%3\n%4\n%5\n%6\n",
			"[switch-client -t feat]":                                               "",
			"[select-window -t feat:main-window]":                                  "",
		},
	}

	layout, err := SelectWorktreeSession(runner, "/repos/feat", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if layout.SessionName != "feat" {
		t.Errorf("SessionName = %q, want %q", layout.SessionName, "feat")
	}
	if layout.Center1.PaneID != "%0" {
		t.Errorf("Center1.PaneID = %q, want %%0", layout.Center1.PaneID)
	}
}

func TestSelectWorktreeSession_CreateError(t *testing.T) {
	runner := &FakeRunner{
		Errors: map[string]error{
			"[has-session -t bad]":              fmt.Errorf("not found"),
			"[new-session -d -s bad -c /bad]": fmt.Errorf("create failed"),
		},
	}

	_, err := SelectWorktreeSession(runner, "/bad", "", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSelectWorktreeSession_SwitchAfterCreateError(t *testing.T) {
	runner := &FakeRunner{
		Errors: map[string]error{
			"[has-session -t feat]":  fmt.Errorf("not found"),
			"[switch-client -t feat]": fmt.Errorf("switch failed"),
		},
		Outputs: map[string]string{
			"[new-session -d -s feat -c /repos/feat]":                      "",
			"[rename-window -t feat:0 main-window]":                        "",
			"[split-window -h -t feat:main-window -c /repos/feat -p 25]":         "",
			"[split-window -v -t feat:main-window.1 -c /repos/feat]":       "",
			"[list-panes -t feat:main-window -F #{pane_id}]":               "%0\n%1\n%2\n",
			"[new-window -t feat -n background-window -c /repos/feat]":     "",
			"[split-window -v -t feat:background-window -c /repos/feat]":   "",
			"[list-panes -t feat:background-window -F #{pane_id}]":         "%3\n%4\n%5\n%6\n",
		},
	}

	_, err := SelectWorktreeSession(runner, "/repos/feat", "", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}
