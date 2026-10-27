package sessionsetup

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mikanfactory/yakumo/internal/git"
)

type fakeTmux struct {
	existingSessions map[string]bool
	failPrefix       string
	calls            [][]string
}

func (f *fakeTmux) Run(args ...string) (string, error) {
	f.calls = append(f.calls, args)
	joined := strings.Join(args, " ")
	if f.failPrefix != "" && strings.HasPrefix(joined, f.failPrefix) {
		return "", fmt.Errorf("fake failure: %s", joined)
	}
	switch args[0] {
	case "has-session":
		if f.existingSessions[strings.TrimPrefix(args[2], "=")] {
			return "", nil
		}
		return "", fmt.Errorf("no session")
	case "list-panes":
		if strings.HasSuffix(args[2], ":main-window") {
			return "%0\n%1\n%2\n", nil
		}
		return "%3\n%4\n%5\n%6\n", nil
	}
	return "", nil
}

func (f *fakeTmux) called(prefix string) bool {
	for _, c := range f.calls {
		if strings.HasPrefix(strings.Join(c, " "), prefix) {
			return true
		}
	}
	return false
}

func TestSetup_ExistingSession_OnlySwitches(t *testing.T) {
	tm := &fakeTmux{existingSessions: map[string]bool{"feat": true}}

	err := Setup(Options{
		TmuxRunner:    tm,
		WorktreePath:  "/wt/feat",
		DiffUICommand: "yakumo diff-ui",
		ClaudeCommand: "claude",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tm.called("switch-client -t =feat") {
		t.Error("expected switch-client to existing session")
	}
	if tm.called("send-keys") {
		t.Error("send-keys should not be called for existing session")
	}
	if tm.called("new-session") {
		t.Error("new-session should not be called for existing session")
	}
}

func TestSetup_NewSession_LaunchesCommandsAndFocuses(t *testing.T) {
	tm := &fakeTmux{}
	var trusted string

	err := Setup(Options{
		TmuxRunner:     tm,
		GitRunner:      git.FakeCommandRunner{},
		WorktreePath:   "/wt/feat",
		DiffUICommand:  "yakumo diff-ui",
		ClaudeCommand:  "claude",
		TrustDirectory: func(p string) error { trusted = p; return nil },
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tm.called("new-session -d -s feat") {
		t.Error("expected new-session")
	}
	if !tm.called("send-keys -t %1 yakumo diff-ui Enter") {
		t.Error("expected diff-ui launched in top-right pane")
	}
	if !tm.called("send-keys -t %0 claude Enter") {
		t.Error("expected claude launched in center pane")
	}
	if !tm.called("select-pane -t %0") {
		t.Error("expected center pane focused")
	}
	if trusted != "/wt/feat" {
		t.Errorf("trusted = %q, want /wt/feat", trusted)
	}
}

func TestSetup_NewSession_SkipsEmptyCommands(t *testing.T) {
	tm := &fakeTmux{}

	err := Setup(Options{TmuxRunner: tm, WorktreePath: "/wt/feat"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tm.called("send-keys") {
		t.Error("send-keys should not be called when commands are empty")
	}
	if !tm.called("select-pane -t %0") {
		t.Error("expected center pane focused")
	}
}

func TestSetup_NewSession_LaunchFailuresAreNonFatal(t *testing.T) {
	tm := &fakeTmux{failPrefix: "send-keys"}

	err := Setup(Options{
		TmuxRunner:     tm,
		WorktreePath:   "/wt/feat",
		DiffUICommand:  "yakumo diff-ui",
		ClaudeCommand:  "claude",
		TrustDirectory: func(string) error { return fmt.Errorf("trust failed") },
	})
	if err != nil {
		t.Fatalf("launch failures should be non-fatal, got %v", err)
	}
}

func TestSetup_SessionCreationError(t *testing.T) {
	tm := &fakeTmux{failPrefix: "new-session"}

	err := Setup(Options{TmuxRunner: tm, WorktreePath: "/wt/feat"})
	if err == nil {
		t.Fatal("expected error when session creation fails")
	}
}
