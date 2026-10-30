package archive

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeScript(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-yakumo")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDetached_PassesArgsToSubcommand(t *testing.T) {
	out := filepath.Join(t.TempDir(), "args")
	exe := writeScript(t, `echo "$@" > `+out+"\n")

	if err := Detached(exe)("/repo", "/wt"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if want := Subcommand + " --repo /repo --path /wt\n"; string(got) != want {
		t.Errorf("args = %q, want %q", got, want)
	}
}

func TestDetached_ReturnsStderrOnFailure(t *testing.T) {
	exe := writeScript(t, "echo 'worktree is dirty' >&2\nexit 1\n")

	err := Detached(exe)("/repo", "/wt")
	if err == nil || !strings.Contains(err.Error(), "worktree is dirty") {
		t.Errorf("expected stderr in error, got %v", err)
	}
}

func TestDetached_RunsInOwnSession(t *testing.T) {
	out := filepath.Join(t.TempDir(), "pgid")
	exe := writeScript(t, `echo $$ > `+out+"\nps -o pgid= -p $$ >> "+out+"\n")

	if err := Detached(exe)("/repo", "/wt"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(string(got))
	if len(fields) != 2 || fields[0] != fields[1] {
		t.Errorf("child should lead its own session (pid, pgid) = %v", fields)
	}
}
