package github

import (
	"fmt"
	"testing"
)

func TestFakeRunner_ReturnsOutput(t *testing.T) {
	runner := &FakeRunner{
		Outputs: map[string]string{
			"/repo:[pr view --json title]": `{"title":"test PR"}`,
		},
	}

	out, err := runner.Run("/repo", "pr", "view", "--json", "title")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != `{"title":"test PR"}` {
		t.Errorf("got %q, want %q", out, `{"title":"test PR"}`)
	}
}

func TestFakeRunner_ReturnsError(t *testing.T) {
	runner := &FakeRunner{
		Errors: map[string]error{
			"/repo:[pr view]": fmt.Errorf("no PR found"),
		},
	}

	_, err := runner.Run("/repo", "pr", "view")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFakeRunner_RecordsCalls(t *testing.T) {
	runner := &FakeRunner{
		Outputs: map[string]string{
			"/repo:[pr view]": "{}",
		},
	}

	_, _ = runner.Run("/repo", "pr", "view")

	if len(runner.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(runner.Calls))
	}
	if runner.Calls[0][0] != "/repo" || runner.Calls[0][1] != "pr" || runner.Calls[0][2] != "view" {
		t.Errorf("unexpected call: %v", runner.Calls[0])
	}
}

func TestFakeRunner_NoMatchReturnsError(t *testing.T) {
	runner := &FakeRunner{}

	_, err := runner.Run("/repo", "unknown")
	if err == nil {
		t.Fatal("expected error for unmatched key")
	}
}
