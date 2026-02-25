package rename

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mikanfactory/yakumo/internal/branchname"
	"github.com/mikanfactory/yakumo/internal/claude"
	"github.com/mikanfactory/yakumo/internal/git"
)

func makeHistory(project, display string, timestamp int64) []byte {
	entry := claude.HistoryEntry{
		Display:   display,
		Project:   project,
		SessionID: "sess-001",
		Timestamp: timestamp,
	}
	data, _ := json.Marshal(entry)
	return append(data, '\n')
}

func TestWatcher_Run_Success(t *testing.T) {
	wtPath := "/Users/shoji/shikon/south-korea"
	createdAt := time.Now().UnixMilli()

	historyData := makeHistory(wtPath, "add user authentication with JWT tokens", createdAt+1000)

	reader := claude.FakeReader{Data: historyData}
	gen := branchname.FakeGenerator{Result: "add-jwt-auth"}
	runner := git.FakeCommandRunner{
		Outputs: map[string]string{
			fmt.Sprintf("%s:[branch -m shoji/south-korea shoji/add-jwt-auth]", wtPath): "",
		},
	}

	cfg := WatcherConfig{
		WorktreePath: wtPath,
		Branch:       "shoji/south-korea",
		CreatedAt:    createdAt,
		PollInterval: 10 * time.Millisecond,
		Timeout:      1 * time.Second,
	}

	w := NewWatcher(cfg, reader, gen, runner)
	err := w.Run()
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
}

func TestWatcher_Run_Timeout(t *testing.T) {
	reader := claude.FakeReader{Data: []byte{}} // empty history
	gen := branchname.FakeGenerator{Result: "unused"}
	runner := git.FakeCommandRunner{Outputs: map[string]string{}}

	cfg := WatcherConfig{
		WorktreePath: "/Users/shoji/shikon/south-korea",
		Branch:       "shoji/south-korea",
		CreatedAt:    time.Now().UnixMilli(),
		PollInterval: 10 * time.Millisecond,
		Timeout:      50 * time.Millisecond,
	}

	w := NewWatcher(cfg, reader, gen, runner)
	err := w.Run()
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("error should contain 'timeout', got: %v", err)
	}
}

func TestWatcher_Run_LLMError(t *testing.T) {
	wtPath := "/Users/shoji/shikon/south-korea"
	createdAt := time.Now().UnixMilli()

	historyData := makeHistory(wtPath, "implement user dashboard with charts", createdAt+1000)

	reader := claude.FakeReader{Data: historyData}
	gen := branchname.FakeGenerator{Err: fmt.Errorf("LLM service unavailable")}
	runner := git.FakeCommandRunner{Outputs: map[string]string{}}

	cfg := WatcherConfig{
		WorktreePath: wtPath,
		Branch:       "shoji/south-korea",
		CreatedAt:    createdAt,
		PollInterval: 10 * time.Millisecond,
		Timeout:      1 * time.Second,
	}

	w := NewWatcher(cfg, reader, gen, runner)
	err := w.Run()
	if err == nil {
		t.Fatal("expected LLM error, got nil")
	}
	if !strings.Contains(err.Error(), "generating branch name") {
		t.Errorf("error should mention generating branch name, got: %v", err)
	}
}

func TestWatcher_Run_RenameError(t *testing.T) {
	wtPath := "/Users/shoji/shikon/south-korea"
	createdAt := time.Now().UnixMilli()

	historyData := makeHistory(wtPath, "fix the login redirect bug in auth module", createdAt+1000)

	reader := claude.FakeReader{Data: historyData}
	gen := branchname.FakeGenerator{Result: "fix-login-redirect"}
	runner := git.FakeCommandRunner{
		Errors: map[string]error{
			fmt.Sprintf("%s:[branch -m shoji/south-korea shoji/fix-login-redirect]", wtPath): fmt.Errorf("branch already exists"),
		},
	}

	cfg := WatcherConfig{
		WorktreePath: wtPath,
		Branch:       "shoji/south-korea",
		CreatedAt:    createdAt,
		PollInterval: 10 * time.Millisecond,
		Timeout:      1 * time.Second,
	}

	w := NewWatcher(cfg, reader, gen, runner)
	err := w.Run()
	if err == nil {
		t.Fatal("expected rename error, got nil")
	}
	if !strings.Contains(err.Error(), "renaming branch") {
		t.Errorf("error should mention renaming branch, got: %v", err)
	}
}

func TestWatcher_Run_PreservesPrefix(t *testing.T) {
	wtPath := "/Users/shoji/shikon/south-korea"
	createdAt := time.Now().UnixMilli()

	historyData := makeHistory(wtPath, "refactor database connection pooling logic", createdAt+1000)

	reader := claude.FakeReader{Data: historyData}
	gen := branchname.FakeGenerator{Result: "refactor-db-pool"}
	runner := git.FakeCommandRunner{
		Outputs: map[string]string{
			// Expect the username prefix to be preserved
			fmt.Sprintf("%s:[branch -m mikan/south-korea mikan/refactor-db-pool]", wtPath): "",
		},
	}

	cfg := WatcherConfig{
		WorktreePath: wtPath,
		Branch:       "mikan/south-korea",
		CreatedAt:    createdAt,
		PollInterval: 10 * time.Millisecond,
		Timeout:      1 * time.Second,
	}

	w := NewWatcher(cfg, reader, gen, runner)
	err := w.Run()
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
}

func TestWatcher_Run_NoPrefixBranch(t *testing.T) {
	wtPath := "/Users/shoji/shikon/south-korea"
	createdAt := time.Now().UnixMilli()

	historyData := makeHistory(wtPath, "add dark mode support to settings page", createdAt+1000)

	reader := claude.FakeReader{Data: historyData}
	gen := branchname.FakeGenerator{Result: "add-dark-mode"}
	runner := git.FakeCommandRunner{
		Outputs: map[string]string{
			// No prefix: branch name used directly
			fmt.Sprintf("%s:[branch -m south-korea add-dark-mode]", wtPath): "",
		},
	}

	cfg := WatcherConfig{
		WorktreePath: wtPath,
		Branch:       "south-korea",
		CreatedAt:    createdAt,
		PollInterval: 10 * time.Millisecond,
		Timeout:      1 * time.Second,
	}

	w := NewWatcher(cfg, reader, gen, runner)
	err := w.Run()
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
}

func TestWatcher_Run_EmptyBranchName(t *testing.T) {
	wtPath := "/Users/shoji/shikon/south-korea"
	createdAt := time.Now().UnixMilli()

	historyData := makeHistory(wtPath, "do something with the codebase", createdAt+1000)

	reader := claude.FakeReader{Data: historyData}
	gen := branchname.FakeGenerator{Result: "---"} // sanitizes to empty
	runner := git.FakeCommandRunner{Outputs: map[string]string{}}

	cfg := WatcherConfig{
		WorktreePath: wtPath,
		Branch:       "shoji/south-korea",
		CreatedAt:    createdAt,
		PollInterval: 10 * time.Millisecond,
		Timeout:      1 * time.Second,
	}

	w := NewWatcher(cfg, reader, gen, runner)
	err := w.Run()
	if err == nil {
		t.Fatal("expected error for empty branch name, got nil")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error should mention empty, got: %v", err)
	}
}

func TestWatcher_Run_SkipsShortPrompts(t *testing.T) {
	wtPath := "/Users/shoji/shikon/south-korea"
	createdAt := time.Now().UnixMilli()

	// Only short prompts - should timeout since no meaningful prompt found
	historyData := makeHistory(wtPath, "hi", createdAt+1000)

	reader := claude.FakeReader{Data: historyData}
	gen := branchname.FakeGenerator{Result: "unused"}
	runner := git.FakeCommandRunner{Outputs: map[string]string{}}

	cfg := WatcherConfig{
		WorktreePath: wtPath,
		Branch:       "shoji/south-korea",
		CreatedAt:    createdAt,
		PollInterval: 10 * time.Millisecond,
		Timeout:      50 * time.Millisecond,
	}

	w := NewWatcher(cfg, reader, gen, runner)
	err := w.Run()
	if err == nil {
		t.Fatal("expected timeout, got nil")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("error should contain 'timeout', got: %v", err)
	}
}
