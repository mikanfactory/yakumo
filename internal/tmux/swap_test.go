package tmux

import (
	"errors"
	"testing"
)

func TestCurrentSessionName(t *testing.T) {
	t.Run("success without TMUX_PANE", func(t *testing.T) {
		t.Setenv("TMUX_PANE", "")
		runner := &FakeRunner{
			Outputs: map[string]string{
				"[display-message -p #{session_name}]": "my-session\n",
			},
		}
		name, err := CurrentSessionName(runner)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if name != "my-session" {
			t.Errorf("expected %q, got %q", "my-session", name)
		}
	})

	t.Run("success with TMUX_PANE", func(t *testing.T) {
		t.Setenv("TMUX_PANE", "%5")
		runner := &FakeRunner{
			Outputs: map[string]string{
				"[display-message -p -t %5 #{session_name}]": "correct-session\n",
			},
		}
		name, err := CurrentSessionName(runner)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if name != "correct-session" {
			t.Errorf("expected %q, got %q", "correct-session", name)
		}
	})

	t.Run("error", func(t *testing.T) {
		t.Setenv("TMUX_PANE", "")
		runner := &FakeRunner{
			Errors: map[string]error{
				"[display-message -p #{session_name}]": errors.New("not in tmux"),
			},
		}
		_, err := CurrentSessionName(runner)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestSwapCenter(t *testing.T) {
	t.Setenv("TMUX_PANE", "")

	t.Run("success", func(t *testing.T) {
		t.Setenv("TMUX_PANE", "")
		runner := &FakeRunner{
			Outputs: map[string]string{
				"[display-message -p #{session_name}]":                                              "dev",
				"[swap-pane -d -s dev:main-window.0 -t dev:background-window.0]":                   "",
				"[swap-pane -d -s dev:background-window.0 -t dev:background-window.1]":             "",
			},
		}

		err := SwapCenter(runner)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(runner.Calls) != 3 {
			t.Errorf("expected 3 calls, got %d", len(runner.Calls))
		}
	})

	t.Run("session name error", func(t *testing.T) {
		t.Setenv("TMUX_PANE", "")
		runner := &FakeRunner{
			Errors: map[string]error{
				"[display-message -p #{session_name}]": errors.New("fail"),
			},
		}

		err := SwapCenter(runner)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("first swap error", func(t *testing.T) {
		t.Setenv("TMUX_PANE", "")
		runner := &FakeRunner{
			Outputs: map[string]string{
				"[display-message -p #{session_name}]": "dev",
			},
			Errors: map[string]error{
				"[swap-pane -d -s dev:main-window.0 -t dev:background-window.0]": errors.New("pane not found"),
			},
		}

		err := SwapCenter(runner)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("second swap error", func(t *testing.T) {
		t.Setenv("TMUX_PANE", "")
		runner := &FakeRunner{
			Outputs: map[string]string{
				"[display-message -p #{session_name}]":                            "dev",
				"[swap-pane -d -s dev:main-window.0 -t dev:background-window.0]": "",
			},
			Errors: map[string]error{
				"[swap-pane -d -s dev:background-window.0 -t dev:background-window.1]": errors.New("pane not found"),
			},
		}

		err := SwapCenter(runner)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestSwapRightBelow(t *testing.T) {
	t.Setenv("TMUX_PANE", "")

	t.Run("success", func(t *testing.T) {
		t.Setenv("TMUX_PANE", "")
		runner := &FakeRunner{
			Outputs: map[string]string{
				"[display-message -p #{session_name}]":                                  "dev",
				"[swap-pane -d -s dev:main-window.2 -t dev:background-window.2]":        "",
				"[swap-pane -d -s dev:background-window.2 -t dev:background-window.3]":  "",
			},
		}

		err := SwapRightBelow(runner)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(runner.Calls) != 3 {
			t.Errorf("expected 3 calls, got %d", len(runner.Calls))
		}
	})

	t.Run("session name error", func(t *testing.T) {
		t.Setenv("TMUX_PANE", "")
		runner := &FakeRunner{
			Errors: map[string]error{
				"[display-message -p #{session_name}]": errors.New("fail"),
			},
		}

		err := SwapRightBelow(runner)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("first swap error", func(t *testing.T) {
		t.Setenv("TMUX_PANE", "")
		runner := &FakeRunner{
			Outputs: map[string]string{
				"[display-message -p #{session_name}]": "dev",
			},
			Errors: map[string]error{
				"[swap-pane -d -s dev:main-window.2 -t dev:background-window.2]": errors.New("pane not found"),
			},
		}

		err := SwapRightBelow(runner)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("second swap error", func(t *testing.T) {
		t.Setenv("TMUX_PANE", "")
		runner := &FakeRunner{
			Outputs: map[string]string{
				"[display-message -p #{session_name}]":                            "dev",
				"[swap-pane -d -s dev:main-window.2 -t dev:background-window.2]": "",
			},
			Errors: map[string]error{
				"[swap-pane -d -s dev:background-window.2 -t dev:background-window.3]": errors.New("pane not found"),
			},
		}

		err := SwapRightBelow(runner)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
