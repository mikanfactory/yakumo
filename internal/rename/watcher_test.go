package rename

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/mikanfactory/yakumo/internal/branchname"
	"github.com/mikanfactory/yakumo/internal/claude"
	"github.com/mikanfactory/yakumo/internal/git"
	"github.com/mikanfactory/yakumo/internal/tmux"
)

// pickSeq returns xs[i], clamping i to the last index. Zero value if xs is empty.
func pickSeq[T any](xs []T, i int) T {
	var zero T
	if len(xs) == 0 {
		return zero
	}
	if i >= len(xs) {
		i = len(xs) - 1
	}
	return xs[i]
}

type sequenceGenerator struct {
	results []string
	errs    []error
	calls   int
}

func (g *sequenceGenerator) GenerateBranchName(_ string) (string, error) {
	i := g.calls
	g.calls++
	return pickSeq(g.results, i), pickSeq(g.errs, i)
}

type cmdResult struct {
	out string
	err error
}

// sequenceCommandRunner implements git.CommandRunner and switches behavior
// based on the number of times a given key has been seen. `sequence[key]`
// lists outcomes per call (last entry repeats once exhausted). Falls back
// to outputs/errors maps when a key has no sequence configured.
type sequenceCommandRunner struct {
	outputs  map[string]string
	errors   map[string]error
	sequence map[string][]cmdResult
	calls    map[string]int
}

func (r *sequenceCommandRunner) Run(dir string, args ...string) (string, error) {
	if r.calls == nil {
		r.calls = map[string]int{}
	}
	key := fmt.Sprintf("%s:%v", dir, args)
	if seq, ok := r.sequence[key]; ok && len(seq) > 0 {
		idx := r.calls[key]
		r.calls[key]++
		res := pickSeq(seq, idx)
		return res.out, res.err
	}
	if err, ok := r.errors[key]; ok {
		r.calls[key]++
		return "", err
	}
	if out, ok := r.outputs[key]; ok {
		r.calls[key]++
		return out, nil
	}
	return "", fmt.Errorf("sequenceCommandRunner: no output for key %q", key)
}

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
	wtPath := "/Users/shoji/yakumo/south-korea"
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

	w := NewWatcher(cfg, reader, gen, runner, nil)
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
		WorktreePath: "/Users/shoji/yakumo/south-korea",
		Branch:       "shoji/south-korea",
		CreatedAt:    time.Now().UnixMilli(),
		PollInterval: 10 * time.Millisecond,
		Timeout:      50 * time.Millisecond,
	}

	w := NewWatcher(cfg, reader, gen, runner, nil)
	err := w.Run()
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("error should contain 'timeout', got: %v", err)
	}
}

func TestWatcher_Run_LLMError(t *testing.T) {
	wtPath := "/Users/shoji/yakumo/south-korea"
	createdAt := time.Now().UnixMilli()

	historyData := makeHistory(wtPath, "implement user dashboard with charts", createdAt+1000)

	reader := claude.FakeReader{Data: historyData}
	gen := branchname.FakeGenerator{Err: fmt.Errorf("LLM service unavailable")}
	runner := git.FakeCommandRunner{Outputs: map[string]string{}}

	cfg := WatcherConfig{
		WorktreePath:       wtPath,
		Branch:             "shoji/south-korea",
		CreatedAt:          createdAt,
		PollInterval:       10 * time.Millisecond,
		Timeout:            1 * time.Second,
		RenameRetryBackoff: 1 * time.Millisecond,
	}

	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	w := NewWatcher(cfg, reader, gen, runner, nil)
	w.SetLogger(logger)
	err := w.Run()
	if err == nil {
		t.Fatal("expected LLM error, got nil")
	}
	if !strings.Contains(err.Error(), fmt.Sprintf("after %d attempts", maxRenameAttempts)) {
		t.Errorf("error should mention 'after 3 attempts', got: %v", err)
	}
	if !strings.Contains(err.Error(), "generating branch name") {
		t.Errorf("error should wrap generating branch name error, got: %v", err)
	}
	output := buf.String()
	for _, phrase := range []string{"attempt 1/3", "attempt 2/3", "attempt 3/3"} {
		if !strings.Contains(output, phrase) {
			t.Errorf("log output should contain %q, got:\n%s", phrase, output)
		}
	}
}

func TestWatcher_Run_RenameError(t *testing.T) {
	wtPath := "/Users/shoji/yakumo/south-korea"
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
		WorktreePath:       wtPath,
		Branch:             "shoji/south-korea",
		CreatedAt:          createdAt,
		PollInterval:       10 * time.Millisecond,
		Timeout:            1 * time.Second,
		RenameRetryBackoff: 1 * time.Millisecond,
	}

	w := NewWatcher(cfg, reader, gen, runner, nil)
	err := w.Run()
	if err == nil {
		t.Fatal("expected rename error, got nil")
	}
	if !strings.Contains(err.Error(), fmt.Sprintf("after %d attempts", maxRenameAttempts)) {
		t.Errorf("error should mention 'after 3 attempts', got: %v", err)
	}
	if !strings.Contains(err.Error(), "renaming branch") {
		t.Errorf("error should wrap renaming branch error, got: %v", err)
	}
}

func TestWatcher_Run_PreservesPrefix(t *testing.T) {
	wtPath := "/Users/shoji/yakumo/south-korea"
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

	w := NewWatcher(cfg, reader, gen, runner, nil)
	err := w.Run()
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
}

func TestWatcher_Run_NoPrefixBranch(t *testing.T) {
	wtPath := "/Users/shoji/yakumo/south-korea"
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

	w := NewWatcher(cfg, reader, gen, runner, nil)
	err := w.Run()
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
}

func TestWatcher_Run_EmptyBranchName(t *testing.T) {
	wtPath := "/Users/shoji/yakumo/south-korea"
	createdAt := time.Now().UnixMilli()

	historyData := makeHistory(wtPath, "do something with the codebase", createdAt+1000)

	reader := claude.FakeReader{Data: historyData}
	gen := branchname.FakeGenerator{Result: "---"} // sanitizes to empty
	runner := git.FakeCommandRunner{Outputs: map[string]string{}}

	cfg := WatcherConfig{
		WorktreePath:       wtPath,
		Branch:             "shoji/south-korea",
		CreatedAt:          createdAt,
		PollInterval:       10 * time.Millisecond,
		Timeout:            1 * time.Second,
		RenameRetryBackoff: 1 * time.Millisecond,
	}

	w := NewWatcher(cfg, reader, gen, runner, nil)
	err := w.Run()
	if err == nil {
		t.Fatal("expected error for empty branch name, got nil")
	}
	if !strings.Contains(err.Error(), fmt.Sprintf("after %d attempts", maxRenameAttempts)) {
		t.Errorf("error should mention 'after 3 attempts', got: %v", err)
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error should mention empty, got: %v", err)
	}
}

func TestWatcher_Run_LogsProgress(t *testing.T) {
	wtPath := "/Users/shoji/yakumo/south-korea"
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

	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	w := NewWatcher(cfg, reader, gen, runner, nil)
	w.SetLogger(logger)
	err := w.Run()
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	output := buf.String()
	expectedPhrases := []string{
		"started:",
		"polling:",
		"prompt detected:",
		"renameBranch: generating name",
		"renameBranch: renaming",
		"renameBranch: success",
	}
	for _, phrase := range expectedPhrases {
		if !strings.Contains(output, phrase) {
			t.Errorf("log output should contain %q, got:\n%s", phrase, output)
		}
	}
}

func TestWatcher_FindPrompt_LogsErrors(t *testing.T) {
	reader := claude.FakeReader{Err: fmt.Errorf("file not found")}
	gen := branchname.FakeGenerator{Result: "unused"}
	runner := git.FakeCommandRunner{Outputs: map[string]string{}}

	cfg := WatcherConfig{
		WorktreePath: "/tmp/test-wt",
		Branch:       "shoji/test",
		CreatedAt:    time.Now().UnixMilli(),
		PollInterval: 10 * time.Millisecond,
		Timeout:      50 * time.Millisecond,
	}

	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	w := NewWatcher(cfg, reader, gen, runner, nil)
	w.SetLogger(logger)
	_ = w.Run() // will timeout

	output := buf.String()
	if !strings.Contains(output, "ReadHistoryFile error") {
		t.Errorf("log output should contain ReadHistoryFile error, got:\n%s", output)
	}
}

func TestWatcher_Run_SkipsShortPrompts(t *testing.T) {
	wtPath := "/Users/shoji/yakumo/south-korea"
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

	w := NewWatcher(cfg, reader, gen, runner, nil)
	err := w.Run()
	if err == nil {
		t.Fatal("expected timeout, got nil")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("error should contain 'timeout', got: %v", err)
	}
}

func TestWatcher_Run_RenamesTmuxSession(t *testing.T) {
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
	tmuxRunner := &tmux.FakeRunner{
		Outputs: map[string]string{
			"[has-session -t =south-korea]":                  "",
			"[rename-session -t =south-korea add-jwt-auth]": "",
		},
	}

	cfg := WatcherConfig{
		WorktreePath: wtPath,
		Branch:       "shoji/south-korea",
		SessionName:  "south-korea",
		CreatedAt:    createdAt,
		PollInterval: 10 * time.Millisecond,
		Timeout:      1 * time.Second,
	}

	w := NewWatcher(cfg, reader, gen, runner, tmuxRunner)
	err := w.Run()
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	// Verify tmux rename-session was called
	found := false
	for _, call := range tmuxRunner.Calls {
		if len(call) >= 1 && call[0] == "rename-session" {
			found = true
		}
	}
	if !found {
		t.Error("expected tmux rename-session to be called")
	}
}

func TestWatcher_Run_TmuxRenameFailureNonFatal(t *testing.T) {
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
	tmuxRunner := &tmux.FakeRunner{
		Outputs: map[string]string{
			"[has-session -t =south-korea]": "",
		},
		Errors: map[string]error{
			"[rename-session -t =south-korea add-jwt-auth]": fmt.Errorf("tmux error"),
		},
	}

	cfg := WatcherConfig{
		WorktreePath: wtPath,
		Branch:       "shoji/south-korea",
		SessionName:  "south-korea",
		CreatedAt:    createdAt,
		PollInterval: 10 * time.Millisecond,
		Timeout:      1 * time.Second,
	}

	w := NewWatcher(cfg, reader, gen, runner, tmuxRunner)
	err := w.Run()
	// Should still succeed even if tmux rename fails
	if err != nil {
		t.Fatalf("expected success (tmux error is non-fatal), got error: %v", err)
	}
}

func TestWatcher_Run_RenamesTmuxSession_ResolvedBySlug(t *testing.T) {
	wtPath := "/Users/shoji/shikon/south-korea"
	createdAt := time.Now().UnixMilli()

	historyData := makeHistory(wtPath, "add user authentication with JWT tokens", createdAt+1000)

	reader := claude.FakeReader{Data: historyData}
	gen := branchname.FakeGenerator{Result: "add-jwt-auth"}
	runner := git.FakeCommandRunner{
		Outputs: map[string]string{
			fmt.Sprintf("%s:[branch -m mikanfactory/south-korea mikanfactory/add-jwt-auth]", wtPath): "",
			// ResolveSessionName calls getBranch which calls symbolic-ref
			fmt.Sprintf("%s:[symbolic-ref --short HEAD]", wtPath): "mikanfactory/south-korea\n",
		},
	}
	tmuxRunner := &tmux.FakeRunner{
		Outputs: map[string]string{
			// filepath.Base("south-korea") session does NOT exist
			// Branch slug "south-korea" session DOES exist (already renamed)
			"[has-session -t =south-korea]":                  "",
			"[rename-session -t =south-korea add-jwt-auth]": "",
		},
	}

	cfg := WatcherConfig{
		WorktreePath: wtPath,
		Branch:       "mikanfactory/south-korea",
		SessionName:  "south-korea",
		CreatedAt:    createdAt,
		PollInterval: 10 * time.Millisecond,
		Timeout:      1 * time.Second,
	}

	w := NewWatcher(cfg, reader, gen, runner, tmuxRunner)
	err := w.Run()
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	// Verify tmux rename-session was called with resolved name
	found := false
	for _, call := range tmuxRunner.Calls {
		if len(call) >= 3 && call[0] == "rename-session" && call[2] == "=south-korea" {
			found = true
		}
	}
	if !found {
		t.Error("expected tmux rename-session to be called with resolved session name")
	}
}

func TestWatcher_Run_RenamesTmuxSession_FallbackToSlug(t *testing.T) {
	// Test case: directory-based session name doesn't exist,
	// but branch slug session does (session was previously renamed to slug)
	wtPath := "/Users/shoji/shikon/saint-pierre-and-miquelon"
	createdAt := time.Now().UnixMilli()

	historyData := makeHistory(wtPath, "fix the diff UI session error", createdAt+1000)

	reader := claude.FakeReader{Data: historyData}
	gen := branchname.FakeGenerator{Result: "fix-diffui-session-error"}
	runner := git.FakeCommandRunner{
		Outputs: map[string]string{
			fmt.Sprintf("%s:[branch -m mikanfactory/saint-pierre-and-miquelon mikanfactory/fix-diffui-session-error]", wtPath): "",
			// ResolveSessionName calls getBranch
			fmt.Sprintf("%s:[symbolic-ref --short HEAD]", wtPath): "mikanfactory/saint-pierre-and-miquelon\n",
		},
	}
	tmuxRunner := &tmux.FakeRunner{
		Outputs: map[string]string{
			// directory-based name exists
			"[has-session -t =saint-pierre-and-miquelon]":                           "",
			"[rename-session -t =saint-pierre-and-miquelon fix-diffui-session-error]": "",
		},
		Errors: map[string]error{},
	}

	cfg := WatcherConfig{
		WorktreePath: wtPath,
		Branch:       "mikanfactory/saint-pierre-and-miquelon",
		SessionName:  "saint-pierre-and-miquelon",
		CreatedAt:    createdAt,
		PollInterval: 10 * time.Millisecond,
		Timeout:      1 * time.Second,
	}

	w := NewWatcher(cfg, reader, gen, runner, tmuxRunner)
	err := w.Run()
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	// Verify rename-session was called with the resolved name
	found := false
	for _, call := range tmuxRunner.Calls {
		if len(call) >= 4 && call[0] == "rename-session" && call[2] == "=saint-pierre-and-miquelon" && call[3] == "fix-diffui-session-error" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected rename-session with resolved name, calls: %v", tmuxRunner.Calls)
	}
}

func TestWatcher_Run_RetryRecoversAfterLLMError(t *testing.T) {
	wtPath := "/Users/shoji/yakumo/south-korea"
	createdAt := time.Now().UnixMilli()

	historyData := makeHistory(wtPath, "implement user dashboard with charts", createdAt+1000)

	reader := claude.FakeReader{Data: historyData}
	gen := &sequenceGenerator{
		results: []string{"", "add-jwt-auth"},
		errs:    []error{fmt.Errorf("LLM service unavailable"), nil},
	}
	runner := git.FakeCommandRunner{
		Outputs: map[string]string{
			fmt.Sprintf("%s:[branch -m shoji/south-korea shoji/add-jwt-auth]", wtPath): "",
		},
	}

	cfg := WatcherConfig{
		WorktreePath:       wtPath,
		Branch:             "shoji/south-korea",
		CreatedAt:          createdAt,
		PollInterval:       10 * time.Millisecond,
		Timeout:            1 * time.Second,
		RenameRetryBackoff: 1 * time.Millisecond,
	}

	w := NewWatcher(cfg, reader, gen, runner, nil)
	err := w.Run()
	if err != nil {
		t.Fatalf("expected success after retry, got error: %v", err)
	}
	if gen.calls != 2 {
		t.Errorf("expected 2 generator calls, got %d", gen.calls)
	}
}

func TestWatcher_Run_RetryRecoversAfterEmptySanitize(t *testing.T) {
	wtPath := "/Users/shoji/yakumo/south-korea"
	createdAt := time.Now().UnixMilli()

	historyData := makeHistory(wtPath, "implement user dashboard with charts", createdAt+1000)

	reader := claude.FakeReader{Data: historyData}
	gen := &sequenceGenerator{
		results: []string{"---", "add-jwt-auth"},
		errs:    []error{nil, nil},
	}
	runner := git.FakeCommandRunner{
		Outputs: map[string]string{
			fmt.Sprintf("%s:[branch -m shoji/south-korea shoji/add-jwt-auth]", wtPath): "",
		},
	}

	cfg := WatcherConfig{
		WorktreePath:       wtPath,
		Branch:             "shoji/south-korea",
		CreatedAt:          createdAt,
		PollInterval:       10 * time.Millisecond,
		Timeout:            1 * time.Second,
		RenameRetryBackoff: 1 * time.Millisecond,
	}

	w := NewWatcher(cfg, reader, gen, runner, nil)
	err := w.Run()
	if err != nil {
		t.Fatalf("expected success after retry, got error: %v", err)
	}
	if gen.calls != 2 {
		t.Errorf("expected 2 generator calls, got %d", gen.calls)
	}
}

func TestWatcher_Run_RetryRecoversAfterGitError(t *testing.T) {
	wtPath := "/Users/shoji/yakumo/south-korea"
	createdAt := time.Now().UnixMilli()

	historyData := makeHistory(wtPath, "implement user dashboard with charts", createdAt+1000)

	reader := claude.FakeReader{Data: historyData}
	gen := branchname.FakeGenerator{Result: "add-jwt-auth"}
	renameKey := fmt.Sprintf("%s:[branch -m shoji/south-korea shoji/add-jwt-auth]", wtPath)
	runner := &sequenceCommandRunner{
		sequence: map[string][]cmdResult{
			renameKey: {
				{out: "", err: fmt.Errorf("transient git error")},
				{out: "", err: nil},
			},
		},
	}

	cfg := WatcherConfig{
		WorktreePath:       wtPath,
		Branch:             "shoji/south-korea",
		CreatedAt:          createdAt,
		PollInterval:       10 * time.Millisecond,
		Timeout:            1 * time.Second,
		RenameRetryBackoff: 1 * time.Millisecond,
	}

	w := NewWatcher(cfg, reader, gen, runner, nil)
	err := w.Run()
	if err != nil {
		t.Fatalf("expected success after retry, got error: %v", err)
	}
	if got := runner.calls[renameKey]; got != 2 {
		t.Errorf("expected branch -m to be invoked twice, got %d", got)
	}
}

func TestWatcher_Run_AllAttemptsFailErrorWrapping(t *testing.T) {
	wtPath := "/Users/shoji/yakumo/south-korea"
	createdAt := time.Now().UnixMilli()

	historyData := makeHistory(wtPath, "implement user dashboard with charts", createdAt+1000)

	llmErr := fmt.Errorf("LLM service unavailable")
	reader := claude.FakeReader{Data: historyData}
	gen := branchname.FakeGenerator{Err: llmErr}
	runner := git.FakeCommandRunner{Outputs: map[string]string{}}

	cfg := WatcherConfig{
		WorktreePath:       wtPath,
		Branch:             "shoji/south-korea",
		CreatedAt:          createdAt,
		PollInterval:       10 * time.Millisecond,
		Timeout:            1 * time.Second,
		RenameRetryBackoff: 1 * time.Millisecond,
	}

	w := NewWatcher(cfg, reader, gen, runner, nil)
	err := w.Run()
	if err == nil {
		t.Fatal("expected error after all attempts fail, got nil")
	}
	if !strings.Contains(err.Error(), fmt.Sprintf("after %d attempts", maxRenameAttempts)) {
		t.Errorf("error should mention attempts count, got: %v", err)
	}
	if !errors.Is(err, llmErr) {
		t.Errorf("error should wrap original LLM error via errors.Is, got: %v", err)
	}
}
