package git

import (
	"fmt"
	"testing"
)

func TestOSCommandRunner_GitVersion(t *testing.T) {
	runner := OSCommandRunner{}
	out, err := runner.Run(".", "--version")
	if err != nil {
		t.Fatalf("git --version failed: %v", err)
	}
	if out == "" {
		t.Error("expected non-empty output from git --version")
	}
}

func TestFakeCommandRunner_ReturnsOutput(t *testing.T) {
	runner := FakeCommandRunner{
		Outputs: map[string]string{
			"/repo:[worktree list --porcelain]": "worktree /repo\nbranch refs/heads/main\n",
		},
	}

	out, err := runner.Run("/repo", "worktree", "list", "--porcelain")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "worktree /repo\nbranch refs/heads/main\n" {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestFakeCommandRunner_ReturnsError(t *testing.T) {
	runner := FakeCommandRunner{
		Errors: map[string]error{
			"/repo:[status --porcelain]": fmt.Errorf("git failed"),
		},
	}

	_, err := runner.Run("/repo", "status", "--porcelain")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFakeCommandRunner_NoOutput(t *testing.T) {
	runner := FakeCommandRunner{
		Outputs: map[string]string{},
	}

	_, err := runner.Run("/repo", "unknown")
	if err == nil {
		t.Fatal("expected error for missing key, got nil")
	}
}
