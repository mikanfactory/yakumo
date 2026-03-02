package tmux

import (
	"fmt"
	"testing"
)

func TestEnsureMainSession_AlreadyExists(t *testing.T) {
	runner := &FakeRunner{
		Outputs: map[string]string{
			"[has-session -t yakumo-main]": "",
		},
	}

	err := EnsureMainSession(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only call has-session, not new-session
	for _, call := range runner.Calls {
		if len(call) >= 1 && call[0] == "new-session" {
			t.Error("should not create new session when one already exists")
		}
	}
}

func TestEnsureMainSession_CreatesNew(t *testing.T) {
	runner := &FakeRunner{
		Errors: map[string]error{
			"[has-session -t yakumo-main]": fmt.Errorf("session not found"),
		},
		Outputs: map[string]string{},
	}
	// Allow new-session and send-keys to succeed (FakeRunner returns error for unknown keys,
	// so we need to register them)
	// The homeDir will vary, so we match broadly by checking calls instead
	runner.Outputs["[has-session -t yakumo-main]"] = "" // won't be used because error takes precedence

	// We need to allow the new-session call to succeed regardless of homeDir
	// Use a custom approach: add a wildcard-like entry
	// Actually, FakeRunner uses exact key matching, so let's just check the calls

	// Re-create with a simpler approach - use a runner that accepts any new-session call
	runner2 := &flexFakeRunner{
		allowPrefix: []string{"new-session", "send-keys"},
		errors: map[string]error{
			"[has-session -t yakumo-main]": fmt.Errorf("session not found"),
		},
		outputs: map[string]string{},
	}

	err := EnsureMainSession(runner2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify new-session was called
	found := false
	for _, call := range runner2.calls {
		if len(call) >= 1 && call[0] == "new-session" {
			found = true
			if call[1] != "-d" || call[2] != "-s" || call[3] != MainSessionName {
				t.Errorf("unexpected new-session args: %v", call)
			}
			break
		}
	}
	if !found {
		t.Error("expected new-session to be called")
	}
}

func TestEnsureMainSession_CreateError(t *testing.T) {
	runner := &flexFakeRunner{
		errors: map[string]error{
			"[has-session -t yakumo-main]": fmt.Errorf("session not found"),
		},
		failPrefix: []string{"new-session"},
	}

	err := EnsureMainSession(runner)
	if err == nil {
		t.Fatal("expected error when new-session fails")
	}
}

func TestIsCurrentSession_Match(t *testing.T) {
	t.Setenv("TMUX_PANE", "")
	runner := &FakeRunner{
		Outputs: map[string]string{
			"[display-message -p #{session_name}]": "south-korea\n",
		},
	}

	if !IsCurrentSession(runner, "south-korea") {
		t.Error("expected IsCurrentSession to return true")
	}
}

func TestIsCurrentSession_NoMatch(t *testing.T) {
	t.Setenv("TMUX_PANE", "")
	runner := &FakeRunner{
		Outputs: map[string]string{
			"[display-message -p #{session_name}]": "other-session\n",
		},
	}

	if IsCurrentSession(runner, "south-korea") {
		t.Error("expected IsCurrentSession to return false")
	}
}

func TestIsCurrentSession_Error(t *testing.T) {
	t.Setenv("TMUX_PANE", "")
	runner := &FakeRunner{
		Errors: map[string]error{
			"[display-message -p #{session_name}]": fmt.Errorf("tmux error"),
		},
	}

	if IsCurrentSession(runner, "south-korea") {
		t.Error("expected IsCurrentSession to return false on error")
	}
}

func TestSwitchToMainSession_Success(t *testing.T) {
	runner := &FakeRunner{
		Outputs: map[string]string{
			"[has-session -t yakumo-main]":      "",
			"[switch-client -t yakumo-main]":    "",
		},
	}

	err := SwitchToMainSession(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify switch-client was called
	found := false
	for _, call := range runner.Calls {
		if len(call) >= 2 && call[0] == "switch-client" && call[2] == MainSessionName {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected switch-client to yakumo-main")
	}
}

func TestSwitchToMainSession_CreatesAndSwitches(t *testing.T) {
	runner := &flexFakeRunner{
		errors: map[string]error{
			"[has-session -t yakumo-main]": fmt.Errorf("session not found"),
		},
		allowPrefix: []string{"new-session", "send-keys", "switch-client"},
		outputs:     map[string]string{},
	}

	err := SwitchToMainSession(runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify both new-session and switch-client were called
	newSessionCalled := false
	switchCalled := false
	for _, call := range runner.calls {
		if len(call) >= 1 && call[0] == "new-session" {
			newSessionCalled = true
		}
		if len(call) >= 1 && call[0] == "switch-client" {
			switchCalled = true
		}
	}
	if !newSessionCalled {
		t.Error("expected new-session to be called")
	}
	if !switchCalled {
		t.Error("expected switch-client to be called")
	}
}

func TestSwitchToMainSession_SwitchError(t *testing.T) {
	runner := &FakeRunner{
		Outputs: map[string]string{
			"[has-session -t yakumo-main]": "",
		},
		Errors: map[string]error{
			"[switch-client -t yakumo-main]": fmt.Errorf("switch failed"),
		},
	}

	err := SwitchToMainSession(runner)
	if err == nil {
		t.Fatal("expected error when switch-client fails")
	}
}

// flexFakeRunner is a test helper that allows commands matching certain prefixes
// to succeed without explicit registration.
type flexFakeRunner struct {
	calls       [][]string
	outputs     map[string]string
	errors      map[string]error
	allowPrefix []string
	failPrefix  []string
}

func (r *flexFakeRunner) Run(args ...string) (string, error) {
	r.calls = append(r.calls, args)
	key := fmt.Sprintf("%v", args)

	if r.errors != nil {
		if err, ok := r.errors[key]; ok {
			return "", err
		}
	}
	if r.outputs != nil {
		if out, ok := r.outputs[key]; ok {
			return out, nil
		}
	}

	// Check fail prefixes
	if len(args) > 0 {
		for _, prefix := range r.failPrefix {
			if args[0] == prefix {
				return "", fmt.Errorf("flexFakeRunner: %s failed", prefix)
			}
		}
	}

	// Check allow prefixes
	if len(args) > 0 {
		for _, prefix := range r.allowPrefix {
			if args[0] == prefix {
				return "", nil
			}
		}
	}

	return "", fmt.Errorf("flexFakeRunner: no output for key %q", key)
}
