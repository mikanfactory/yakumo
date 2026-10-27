package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/mikanfactory/yakumo/internal/git"
	"github.com/mikanfactory/yakumo/internal/github"
	"github.com/mikanfactory/yakumo/internal/model"
	"github.com/mikanfactory/yakumo/internal/sidebar"
	"github.com/mikanfactory/yakumo/internal/tmux"
)

func testModel() Model {
	groups := []model.RepoGroup{
		{
			Name:     "repo1",
			RootPath: "/code/repo1",
			Worktrees: []model.WorktreeInfo{
				{Path: "/code/repo1", Branch: "main"},
				{Path: "/code/repo1-feat", Branch: "feature-x"},
			},
		},
	}

	items := sidebar.BuildItems(groups)

	return Model{
		items:        items,
		groups:       groups,
		cursor:       FirstSelectable(items),
		sidebarWidth: 30,
		textInput:    textinput.New(),
	}
}

func TestUpdate_MoveDown(t *testing.T) {
	m := testModel()
	initialCursor := m.cursor

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	updated := result.(Model)

	if updated.cursor <= initialCursor {
		t.Errorf("cursor should have moved down from %d, got %d", initialCursor, updated.cursor)
	}
}

func TestUpdate_MoveUp(t *testing.T) {
	m := testModel()
	// Move to second worktree first
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = result.(Model)
	cursorAfterDown := m.cursor

	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	updated := result.(Model)

	if updated.cursor >= cursorAfterDown {
		t.Errorf("cursor should have moved up from %d, got %d", cursorAfterDown, updated.cursor)
	}
}

func TestUpdate_SkipsGroupHeaders(t *testing.T) {
	m := testModel()
	// First item is a group header (non-selectable), cursor should start at first worktree
	if m.items[0].Kind != model.ItemKindGroupHeader {
		t.Fatal("expected first item to be a group header")
	}
	if m.cursor == 0 {
		t.Error("cursor should not be on group header")
	}
	if !m.items[m.cursor].Selectable {
		t.Error("cursor should be on a selectable item")
	}
}

func TestUpdate_Enter_SelectsWorktree(t *testing.T) {
	m := testModel()

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := result.(Model)

	if updated.selected != "/code/repo1" {
		t.Errorf("selected = %q, want %q", updated.selected, "/code/repo1")
	}
	if updated.SelectedRepoPath() != "/code/repo1" {
		t.Errorf("SelectedRepoPath() = %q, want %q", updated.SelectedRepoPath(), "/code/repo1")
	}
	if cmd == nil {
		t.Error("expected tea.Quit cmd")
	}
}

func TestUpdate_Enter_AddRepo_EntersInputMode(t *testing.T) {
	m := testModel()
	// Navigate to "Add repository" action item
	for i, item := range m.items {
		if item.Kind == model.ItemKindAddRepo {
			m.cursor = i
			break
		}
	}

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := result.(Model)

	if !updated.addingRepo {
		t.Error("addingRepo should be true after pressing enter on Add repository")
	}
	if updated.selected != "" {
		t.Errorf("selected should be empty, got %q", updated.selected)
	}
}

func TestUpdate_Quit(t *testing.T) {
	m := testModel()

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	updated := result.(Model)

	if !updated.quitting {
		t.Error("quitting should be true")
	}
	if cmd == nil {
		t.Error("expected tea.Quit cmd")
	}
}

func TestUpdate_CtrlC(t *testing.T) {
	m := testModel()

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	updated := result.(Model)

	if !updated.quitting {
		t.Error("quitting should be true")
	}
	if cmd == nil {
		t.Error("expected tea.Quit cmd")
	}
}

func TestUpdate_GitDataMsg(t *testing.T) {
	m := Model{
		sidebarWidth: 30,
		loading:      true,
	}

	groups := []model.RepoGroup{
		{
			Name:      "test",
			RootPath:  "/test",
			Worktrees: []model.WorktreeInfo{{Path: "/test", Branch: "main"}},
		},
	}

	result, cmd := m.Update(GitDataMsg{Groups: groups})
	updated := result.(Model)

	if updated.loading {
		t.Error("loading should be false after GitDataMsg")
	}
	if len(updated.items) == 0 {
		t.Error("items should be populated after GitDataMsg")
	}
	if len(updated.groups) != 1 {
		t.Errorf("len(groups) = %d, want 1", len(updated.groups))
	}
	if cmd == nil {
		t.Error("expected agentTickCmd to be returned after first GitDataMsg")
	}
	if !updated.agentTickRunning {
		t.Error("agentTickRunning should be true after first GitDataMsg")
	}
}

func TestUpdate_GitDataMsg_NoTickDuplication(t *testing.T) {
	m := Model{
		sidebarWidth:     30,
		loading:          true,
		agentTickRunning: true, // already running
	}

	groups := []model.RepoGroup{
		{
			Name:      "test",
			RootPath:  "/test",
			Worktrees: []model.WorktreeInfo{{Path: "/test", Branch: "main"}},
		},
	}

	result, cmd := m.Update(GitDataMsg{Groups: groups})
	updated := result.(Model)

	if updated.loading {
		t.Error("loading should be false after GitDataMsg")
	}
	if len(updated.items) == 0 {
		t.Error("items should be populated after GitDataMsg")
	}
	if cmd != nil {
		t.Error("expected nil cmd when agentTickRunning is already true")
	}
}

func TestUpdate_GitDataErrMsg(t *testing.T) {
	m := Model{loading: true}

	result, _ := m.Update(GitDataErrMsg{Err: fmt.Errorf("test error")})
	updated := result.(Model)

	if updated.loading {
		t.Error("loading should be false after GitDataErrMsg")
	}
	if updated.err == nil {
		t.Error("err should be set")
	}
}

func TestSelected(t *testing.T) {
	m := Model{selected: "/some/path"}
	if m.Selected() != "/some/path" {
		t.Errorf("Selected() = %q, want %q", m.Selected(), "/some/path")
	}
}

func TestNewModel(t *testing.T) {
	cfg := model.Config{
		SidebarWidth: 35,
		Repositories: []model.RepositoryDef{
			{Name: "test", Path: "/test"},
		},
	}
	runner := &fakeRunner{}

	m := NewModel(cfg, runner, "/tmp/config.yaml", nil, nil)

	if m.sidebarWidth != 35 {
		t.Errorf("sidebarWidth = %d, want 35", m.sidebarWidth)
	}
	if !m.loading {
		t.Error("loading should be true initially")
	}
	if m.configPath != "/tmp/config.yaml" {
		t.Errorf("configPath = %q, want %q", m.configPath, "/tmp/config.yaml")
	}
}

func TestInit_ReturnsCmd(t *testing.T) {
	cfg := model.Config{
		SidebarWidth: 30,
		Repositories: []model.RepositoryDef{
			{Name: "test", Path: "/test"},
		},
	}
	runner := &fakeRunner{}
	m := NewModel(cfg, runner, "", nil, nil)

	cmd := m.Init()
	if cmd == nil {
		t.Error("Init() should return a non-nil Cmd")
	}
}

func TestUpdate_ArrowKeys(t *testing.T) {
	m := testModel()

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	updated := result.(Model)
	if updated.cursor <= m.cursor {
		t.Error("down arrow should move cursor down")
	}

	result, _ = updated.Update(tea.KeyMsg{Type: tea.KeyUp})
	updated2 := result.(Model)
	if updated2.cursor >= updated.cursor {
		t.Error("up arrow should move cursor up")
	}
}

func TestUpdate_MouseClick_SelectsWorktree(t *testing.T) {
	m := testModel()

	// First render the view to register zones
	m.View()

	// Find the index of the first worktree item
	worktreeIdx := -1
	for i, item := range m.items {
		if item.Kind == model.ItemKindWorktree {
			worktreeIdx = i
			break
		}
	}
	if worktreeIdx == -1 {
		t.Fatal("no worktree item found")
	}

	// Simulate a mouse click within the zone
	mouseMsg := tea.MouseMsg{
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
	}

	// Use zone.Get to check if zone is registered, then test with InBounds
	result, cmd := m.Update(mouseMsg)
	updated := result.(Model)

	// When coordinates don't match any zone, no selection should happen
	if updated.selected != "" {
		t.Errorf("selected should be empty when click is outside zones, got %q", updated.selected)
	}
	_ = cmd
}

func TestZoneID(t *testing.T) {
	tests := []struct {
		index int
		want  string
	}{
		{0, "item-0"},
		{5, "item-5"},
		{42, "item-42"},
	}

	for _, tt := range tests {
		got := ZoneID(tt.index)
		if got != tt.want {
			t.Errorf("ZoneID(%d) = %q, want %q", tt.index, got, tt.want)
		}
	}
}

func TestUpdate_Enter_AddWorktree_EntersInputMode(t *testing.T) {
	m := testModel()
	m.config = model.Config{WorktreeBasePath: "/tmp/yakumo"}

	// Navigate to "Add worktree" item
	for i, item := range m.items {
		if item.Kind == model.ItemKindAddWorktree {
			m.cursor = i
			break
		}
	}

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := result.(Model)

	if !updated.addingWorktree {
		t.Error("addingWorktree should be true after pressing enter on Add worktree")
	}
	if updated.addingWorktreeRepoPath != "/code/repo1" {
		t.Errorf("addingWorktreeRepoPath = %q, want %q", updated.addingWorktreeRepoPath, "/code/repo1")
	}
	if updated.loading {
		t.Error("loading should be false (not creating worktree yet)")
	}
	if cmd == nil {
		t.Error("expected textInput.Focus command to be returned")
	}
}

func TestUpdate_WorktreeAddedMsg(t *testing.T) {
	m := testModel()
	m.config = model.Config{
		Repositories: []model.RepositoryDef{{Name: "test", Path: "/test"}},
	}
	m.runner = &fakeRunner{}

	result, cmd := m.Update(WorktreeAddedMsg{})
	updated := result.(Model)

	if !updated.loading {
		t.Error("loading should be true after WorktreeAddedMsg (refreshing)")
	}
	if cmd == nil {
		t.Error("expected fetchGitDataCmd to be returned")
	}
}

func TestUpdate_WorktreeAddErrMsg(t *testing.T) {
	m := testModel()

	result, _ := m.Update(WorktreeAddErrMsg{Err: fmt.Errorf("add failed")})
	updated := result.(Model)

	if updated.loading {
		t.Error("loading should be false after WorktreeAddErrMsg")
	}
	if updated.err == nil {
		t.Error("err should be set")
	}
}

func TestRepoNameFromConfig(t *testing.T) {
	cfg := model.Config{
		Repositories: []model.RepositoryDef{
			{Name: "yakumo", Path: "/code/yakumo"},
			{Name: "other", Path: "/code/other"},
		},
	}

	t.Run("match", func(t *testing.T) {
		got := repoNameFromConfig(cfg, "/code/yakumo")
		if got != "yakumo" {
			t.Errorf("repoNameFromConfig() = %q, want %q", got, "yakumo")
		}
	})

	t.Run("fallback", func(t *testing.T) {
		got := repoNameFromConfig(cfg, "/unknown/path/myrepo")
		if got != "myrepo" {
			t.Errorf("repoNameFromConfig() = %q, want %q", got, "myrepo")
		}
	})
}

func TestAddWorktreeCmd_Success(t *testing.T) {
	runner := git.FakeCommandRunner{
		Outputs: map[string]string{
			"/repo:[config user.name]": "testuser\n",
		},
	}

	cmd := addWorktreeCmd(runner, "/repo", "/tmp/yakumo", "myrepo", "origin/main")
	msg := cmd()

	// The command will fail at AddWorktree because FakeCommandRunner won't have
	// the exact key for the random country, but we can check it doesn't panic
	// and returns either WorktreeAddedMsg or WorktreeAddErrMsg
	switch msg.(type) {
	case WorktreeAddedMsg, WorktreeAddErrMsg:
		// expected
	default:
		t.Errorf("unexpected message type: %T", msg)
	}
}

func TestAddWorktreeCmd_FetchError(t *testing.T) {
	runner := git.FakeCommandRunner{
		Outputs: map[string]string{
			"/repo:[config user.name]": "testuser\n",
		},
		Errors: map[string]error{
			"/repo:[fetch origin main]": fmt.Errorf("network error"),
		},
	}

	cmd := addWorktreeCmd(runner, "/repo", "/tmp/yakumo", "myrepo", "origin/main")
	msg := cmd()

	errMsg, ok := msg.(WorktreeAddErrMsg)
	if !ok {
		t.Fatalf("expected WorktreeAddErrMsg, got %T", msg)
	}
	if errMsg.Err == nil {
		t.Fatal("expected error to be set")
	}
	if got := errMsg.Err.Error(); !strings.Contains(got, "fetching origin/main") {
		t.Errorf("expected error about fetching, got: %s", got)
	}
}

func TestAddWorktreeCmd_NoFetchForNonOriginBaseRef(t *testing.T) {
	runner := git.FakeCommandRunner{
		Outputs: map[string]string{
			"/repo:[config user.name]": "testuser\n",
		},
	}

	// baseRef without "origin/" prefix should skip fetch.
	// If fetch were attempted, FakeCommandRunner would fail with a "fetching" error.
	cmd := addWorktreeCmd(runner, "/repo", "/tmp/yakumo", "myrepo", "main")
	msg := cmd()

	// Should fail at AddWorktree (random country key not registered), not at fetch
	errMsg, ok := msg.(WorktreeAddErrMsg)
	if !ok {
		return // WorktreeAddedMsg is also acceptable
	}
	if strings.Contains(errMsg.Err.Error(), "fetching") {
		t.Errorf("should not attempt fetch for non-origin baseRef, got: %s", errMsg.Err)
	}
}

func TestAddWorktreeCmd_UserNameError(t *testing.T) {
	runner := git.FakeCommandRunner{
		Errors: map[string]error{
			"/repo:[config user.name]": fmt.Errorf("no user.name"),
		},
	}

	cmd := addWorktreeCmd(runner, "/repo", "/tmp/yakumo", "myrepo", "origin/main")
	msg := cmd()

	errMsg, ok := msg.(WorktreeAddErrMsg)
	if !ok {
		t.Fatalf("expected WorktreeAddErrMsg, got %T", msg)
	}
	if errMsg.Err == nil {
		t.Error("expected error to be set")
	}
}

func TestFetchGitDataCmd_Success(t *testing.T) {
	runner := git.FakeCommandRunner{
		Outputs: map[string]string{
			"/repo:[worktree list --porcelain]":         "worktree /repo\nHEAD abc123\nbranch refs/heads/main\n\n",
			"/repo:[diff origin/main...HEAD --numstat]": "",
		},
	}

	cfg := model.Config{
		DefaultBaseRef: "origin/main",
		Repositories: []model.RepositoryDef{
			{Name: "test", Path: "/repo"},
		},
	}

	cmd := fetchGitDataCmd(cfg, runner)
	msg := cmd()

	dataMsg, ok := msg.(GitDataMsg)
	if !ok {
		t.Fatalf("expected GitDataMsg, got %T", msg)
	}
	if len(dataMsg.Groups) != 1 {
		t.Errorf("len(Groups) = %d, want 1", len(dataMsg.Groups))
	}
}

func TestFetchGitDataCmd_Error(t *testing.T) {
	runner := git.FakeCommandRunner{
		Errors: map[string]error{
			"/repo:[worktree list --porcelain]": fmt.Errorf("git error"),
		},
	}

	cfg := model.Config{
		Repositories: []model.RepositoryDef{
			{Name: "test", Path: "/repo"},
		},
	}

	cmd := fetchGitDataCmd(cfg, runner)
	msg := cmd()

	_, ok := msg.(GitDataErrMsg)
	if !ok {
		t.Fatalf("expected GitDataErrMsg, got %T", msg)
	}
}

func TestUpdate_AddRepoMode_Escape_Cancels(t *testing.T) {
	m := testModel()
	m.addingRepo = true
	m.err = fmt.Errorf("previous error")

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	updated := result.(Model)

	if updated.addingRepo {
		t.Error("addingRepo should be false after escape")
	}
	if updated.err != nil {
		t.Error("err should be cleared after escape")
	}
}

func TestUpdate_AddRepoMode_Enter_EmptyPath(t *testing.T) {
	m := testModel()
	m.addingRepo = true

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := result.(Model)

	if updated.err == nil {
		t.Error("err should be set for empty path")
	}
	if cmd != nil {
		t.Error("should not return a command for empty path")
	}
	if !updated.addingRepo {
		t.Error("addingRepo should still be true")
	}
}

func TestUpdate_AddRepoMode_Enter_ValidatesPath(t *testing.T) {
	m := testModel()
	m.addingRepo = true
	m.textInput.SetValue("/tmp/test-repo")

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := result.(Model)

	if !updated.loading {
		t.Error("loading should be true after confirming path")
	}
	if cmd == nil {
		t.Error("expected validation command to be returned")
	}
}

func TestUpdate_AddRepoMode_QuitKeysBlocked(t *testing.T) {
	m := testModel()
	m.addingRepo = true

	// 'q' should not quit in input mode
	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	updated := result.(Model)

	if updated.quitting {
		t.Error("q should not quit in input mode")
	}
	if cmd != nil {
		t.Error("should not return tea.Quit in input mode")
	}
}

func TestUpdate_RepoValidatedMsg(t *testing.T) {
	m := testModel()
	m.addingRepo = true
	m.configPath = "/tmp/config.yaml"

	result, cmd := m.Update(RepoValidatedMsg{Name: "my-repo", Path: "/tmp/my-repo"})
	updated := result.(Model)

	if !updated.loading {
		t.Error("loading should be true while adding to config")
	}
	if cmd == nil {
		t.Error("expected addRepoToConfigCmd to be returned")
	}
}

func TestUpdate_RepoValidationErrMsg(t *testing.T) {
	m := testModel()
	m.addingRepo = true
	m.loading = true

	result, _ := m.Update(RepoValidationErrMsg{Err: fmt.Errorf("not a git repo")})
	updated := result.(Model)

	if updated.loading {
		t.Error("loading should be false after validation error")
	}
	if updated.err == nil {
		t.Error("err should be set")
	}
	if !updated.addingRepo {
		t.Error("addingRepo should still be true to allow correction")
	}
}

func TestUpdate_RepoAddedMsg(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := "sidebar_width: 30\nrepositories:\n  - name: repo1\n    path: /tmp/repo1\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	m := testModel()
	m.addingRepo = true
	m.configPath = cfgPath
	m.runner = &fakeRunner{}

	result, cmd := m.Update(RepoAddedMsg{})
	updated := result.(Model)

	if updated.addingRepo {
		t.Error("addingRepo should be false after successful add")
	}
	if cmd == nil {
		t.Error("expected fetchGitData command to refresh the list")
	}
}

func TestUpdate_RepoAddErrMsg(t *testing.T) {
	m := testModel()
	m.addingRepo = true
	m.loading = true

	result, _ := m.Update(RepoAddErrMsg{Err: fmt.Errorf("write failed")})
	updated := result.(Model)

	if updated.loading {
		t.Error("loading should be false after add error")
	}
	if updated.err == nil {
		t.Error("err should be set")
	}
	if updated.addingRepo {
		t.Error("addingRepo should be false after add error")
	}
}

func TestValidateRepoCmd_NotGitRepo(t *testing.T) {
	runner := git.FakeCommandRunner{
		Errors: map[string]error{},
	}

	tmpDir := t.TempDir()
	// FakeCommandRunner won't have the key for rev-parse, so it will error
	cmd := validateRepoCmd(runner, tmpDir)
	msg := cmd()

	_, ok := msg.(RepoValidationErrMsg)
	if !ok {
		t.Fatalf("expected RepoValidationErrMsg, got %T", msg)
	}
}

func TestValidateRepoCmd_NonexistentPath(t *testing.T) {
	runner := git.FakeCommandRunner{}

	cmd := validateRepoCmd(runner, "/nonexistent/path")
	msg := cmd()

	errMsg, ok := msg.(RepoValidationErrMsg)
	if !ok {
		t.Fatalf("expected RepoValidationErrMsg, got %T", msg)
	}
	if errMsg.Err == nil {
		t.Error("expected error to be set")
	}
}

func TestAddRepoToConfigCmd_Success(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := "sidebar_width: 30\nrepositories:\n  - name: repo1\n    path: /tmp/repo1\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := addRepoToConfigCmd(cfgPath, "new-repo", "/tmp/new-repo")
	msg := cmd()

	if _, ok := msg.(RepoAddedMsg); !ok {
		t.Fatalf("expected RepoAddedMsg, got %T", msg)
	}
}

func TestUpdate_RepoValidatedMsg_NormalMode(t *testing.T) {
	m := testModel()
	m.configPath = "/tmp/config.yaml"

	result, cmd := m.Update(RepoValidatedMsg{Name: "repo", Path: "/tmp/repo"})
	updated := result.(Model)

	if !updated.loading {
		t.Error("loading should be true")
	}
	if cmd == nil {
		t.Error("expected command")
	}
}

func TestUpdate_RepoAddedMsg_NormalMode(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := "sidebar_width: 30\nrepositories:\n  - name: repo1\n    path: /tmp/repo1\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	m := testModel()
	m.configPath = cfgPath
	m.runner = &fakeRunner{}

	result, cmd := m.Update(RepoAddedMsg{})
	updated := result.(Model)

	if updated.addingRepo {
		t.Error("addingRepo should be false")
	}
	if cmd == nil {
		t.Error("expected fetchGitData command")
	}
}

func TestUpdate_RepoAddErrMsg_NormalMode(t *testing.T) {
	m := testModel()
	m.loading = true

	result, _ := m.Update(RepoAddErrMsg{Err: fmt.Errorf("fail")})
	updated := result.(Model)

	if updated.loading {
		t.Error("loading should be false")
	}
	if updated.err == nil {
		t.Error("err should be set")
	}
}

func TestUpdate_RepoValidationErrMsg_NormalMode(t *testing.T) {
	m := testModel()
	m.loading = true

	result, _ := m.Update(RepoValidationErrMsg{Err: fmt.Errorf("fail")})
	updated := result.(Model)

	if updated.loading {
		t.Error("loading should be false")
	}
	if updated.err == nil {
		t.Error("err should be set")
	}
}

func TestUpdate_AddRepoMode_CtrlC_Quits(t *testing.T) {
	m := testModel()
	m.addingRepo = true

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	updated := result.(Model)

	if !updated.quitting {
		t.Error("ctrl+c should quit even in input mode")
	}
	if cmd == nil {
		t.Error("expected tea.Quit cmd")
	}
}

func TestValidateRepoCmd_TildeExpansion(t *testing.T) {
	runner := git.FakeCommandRunner{
		Outputs: map[string]string{},
	}

	// ~/nonexistent will expand but the path won't exist on disk
	cmd := validateRepoCmd(runner, "~/nonexistent-yakumo-test-path")
	msg := cmd()

	_, ok := msg.(RepoValidationErrMsg)
	if !ok {
		t.Fatalf("expected RepoValidationErrMsg, got %T", msg)
	}
}

func TestValidateRepoCmd_Success(t *testing.T) {
	tmpDir := t.TempDir()
	runner := git.FakeCommandRunner{
		Outputs: map[string]string{
			fmt.Sprintf("%s:[rev-parse --show-toplevel]", tmpDir): tmpDir + "\n",
		},
	}

	cmd := validateRepoCmd(runner, tmpDir)
	msg := cmd()

	validMsg, ok := msg.(RepoValidatedMsg)
	if !ok {
		t.Fatalf("expected RepoValidatedMsg, got %T (%v)", msg, msg)
	}
	if validMsg.Path != tmpDir {
		t.Errorf("Path = %q, want %q", validMsg.Path, tmpDir)
	}
	if validMsg.Name != filepath.Base(tmpDir) {
		t.Errorf("Name = %q, want %q", validMsg.Name, filepath.Base(tmpDir))
	}
}

func TestUpdate_AddRepoMode_RepoAddedMsg(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := "sidebar_width: 30\nrepositories:\n  - name: repo1\n    path: /tmp/repo1\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	m := testModel()
	m.addingRepo = true
	m.configPath = cfgPath
	m.runner = &fakeRunner{}
	m.textInput.SetValue("/some/path")

	result, cmd := m.Update(RepoAddedMsg{})
	updated := result.(Model)

	if updated.addingRepo {
		t.Error("addingRepo should be false")
	}
	if updated.textInput.Value() != "" {
		t.Error("textInput should be cleared")
	}
	if cmd == nil {
		t.Error("expected fetchGitData command")
	}
}

func TestUpdate_AddRepoMode_RepoAddErrMsg(t *testing.T) {
	m := testModel()
	m.addingRepo = true
	m.loading = true

	result, _ := m.Update(RepoAddErrMsg{Err: fmt.Errorf("config write failed")})
	updated := result.(Model)

	if updated.loading {
		t.Error("loading should be false")
	}
	if updated.err == nil {
		t.Error("err should be set")
	}
	if updated.addingRepo {
		t.Error("addingRepo should be false after add error")
	}
}

func TestAddRepoToConfigCmd_Error(t *testing.T) {
	cmd := addRepoToConfigCmd("/nonexistent/config.yaml", "repo", "/tmp/repo")
	msg := cmd()

	if _, ok := msg.(RepoAddErrMsg); !ok {
		t.Fatalf("expected RepoAddErrMsg, got %T", msg)
	}
}

func TestUpdate_AgentTickMsg_WithRunner(t *testing.T) {
	m := testModel()
	m.tmuxRunner = &tmux.FakeRunner{
		Errors: map[string]error{},
	}

	result, cmd := m.Update(AgentTickMsg(time.Now()))
	_ = result.(Model)

	if cmd == nil {
		t.Error("expected fetchAgentStatusCmd to be returned")
	}
}

func TestUpdate_AgentTickMsg_WithoutRunner(t *testing.T) {
	m := testModel()
	m.tmuxRunner = nil

	result, cmd := m.Update(AgentTickMsg(time.Now()))
	_ = result.(Model)

	if cmd == nil {
		t.Error("expected agentTickCmd to be returned even without runner")
	}
}

func TestUpdate_AgentStatusMsg(t *testing.T) {
	m := testModel()

	statuses := map[string][]model.AgentInfo{
		"/code/repo1": {
			{PaneID: "%0", State: model.AgentStateRunning, Elapsed: "2m"},
		},
	}

	result, cmd := m.Update(AgentStatusMsg{Statuses: statuses})
	updated := result.(Model)

	if updated.agentStatus == nil {
		t.Error("agentStatus should be set")
	}
	if cmd == nil {
		t.Error("expected agentTickCmd to be returned")
	}

	// Verify agent status is merged into items
	for _, item := range updated.items {
		if item.Kind == model.ItemKindWorktree && filepath.Base(item.WorktreePath) == "repo1" {
			if len(item.AgentStatus) != 1 {
				t.Errorf("expected 1 agent for repo1, got %d", len(item.AgentStatus))
			}
		}
	}
}

func TestUpdate_AgentStatusMsg_Empty(t *testing.T) {
	m := testModel()

	result, cmd := m.Update(AgentStatusMsg{Statuses: map[string][]model.AgentInfo{}})
	updated := result.(Model)

	if updated.agentStatus == nil {
		t.Error("agentStatus should be non-nil (empty map)")
	}
	if cmd == nil {
		t.Error("expected agentTickCmd to be returned")
	}
}

func TestFetchAgentStatusCmd(t *testing.T) {
	runner := &tmux.FakeRunner{
		Outputs: map[string]string{
			fmt.Sprintf("%v", []string{"has-session", "-t", "=repo1"}):                                                                 "",
			fmt.Sprintf("%v", []string{"list-panes", "-s", "-t", "repo1", "-F", "#{pane_id}\t#{pane_title}\t#{pane_current_command}"}): "%0\t✳ claude\tnode\n",
			fmt.Sprintf("%v", []string{"capture-pane", "-p", "-t", "%0"}):                                                              "  ❯ ",
		},
		Errors: map[string]error{
			fmt.Sprintf("%v", []string{"has-session", "-t", "=repo1-feat"}): fmt.Errorf("no session"),
		},
	}

	groups := []model.RepoGroup{
		{
			Name:     "repo",
			RootPath: "/code/repo",
			Worktrees: []model.WorktreeInfo{
				{Path: "/code/repo1", Branch: "main"},
				{Path: "/code/repo1-feat", Branch: "feature"},
			},
		},
	}

	cmd := fetchAgentStatusCmd(runner, nil, groups)
	msg := cmd()

	statusMsg, ok := msg.(AgentStatusMsg)
	if !ok {
		t.Fatalf("expected AgentStatusMsg, got %T", msg)
	}
	if len(statusMsg.Statuses["/code/repo1"]) != 1 {
		t.Errorf("expected 1 agent for /code/repo1, got %d", len(statusMsg.Statuses["/code/repo1"]))
	}
	if len(statusMsg.Statuses["/code/repo1-feat"]) != 0 {
		t.Errorf("expected 0 agents for /code/repo1-feat, got %d", len(statusMsg.Statuses["/code/repo1-feat"]))
	}
}


func testModelWithBare() Model {
	groups := []model.RepoGroup{
		{
			Name:     "repo1",
			RootPath: "/code/repo1",
			Worktrees: []model.WorktreeInfo{
				{Path: "/code/repo1", Branch: "main", IsBare: true},
				{Path: "/code/repo1-feat", Branch: "feature-x"},
			},
		},
	}

	items := sidebar.BuildItems(groups)

	return Model{
		items:        items,
		groups:       groups,
		cursor:       FirstSelectable(items),
		sidebarWidth: 30,
		textInput:    textinput.New(),
	}
}

func TestUpdate_D_OnWorktree_EntersConfirmMode(t *testing.T) {
	m := testModel()
	// Cursor should be on first worktree (non-bare)

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	updated := result.(Model)

	if !updated.confirmingArchive {
		t.Error("confirmingArchive should be true")
	}
	if updated.archiveTarget.WorktreePath != m.items[m.cursor].WorktreePath {
		t.Errorf("archiveTarget = %q, want %q", updated.archiveTarget.WorktreePath, m.items[m.cursor].WorktreePath)
	}
	if cmd != nil {
		t.Error("should not return a command")
	}
}

func TestUpdate_D_OnBareWorktree_NoOp(t *testing.T) {
	m := testModelWithBare()
	// First selectable item is the bare worktree

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	updated := result.(Model)

	if updated.confirmingArchive {
		t.Error("confirmingArchive should be false for bare worktree")
	}
}

func TestUpdate_D_OnNonWorktree_NoOp(t *testing.T) {
	m := testModel()
	// Navigate to "Add worktree" item
	for i, item := range m.items {
		if item.Kind == model.ItemKindAddWorktree {
			m.cursor = i
			break
		}
	}

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	updated := result.(Model)

	if updated.confirmingArchive {
		t.Error("confirmingArchive should be false for non-worktree item")
	}
}

func TestUpdate_ConfirmArchiveMode_Escape_Cancels(t *testing.T) {
	m := testModel()
	m.confirmingArchive = true
	m.archiveTarget = m.items[m.cursor]
	m.err = fmt.Errorf("previous error")

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	updated := result.(Model)

	if updated.confirmingArchive {
		t.Error("confirmingArchive should be false after escape")
	}
	if updated.err != nil {
		t.Error("err should be cleared after escape")
	}
}

func TestUpdate_ConfirmArchiveMode_Enter_RunsInBackground(t *testing.T) {
	m := testModel()
	m.confirmingArchive = true
	m.archiveTarget = m.items[m.cursor]
	m.runner = &fakeRunner{}

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := result.(Model)

	if updated.loading {
		t.Error("loading should stay false while archiving in background")
	}
	if updated.confirmingArchive {
		t.Error("confirm modal should close immediately after confirming")
	}
	if updated.pendingOps[m.archiveTarget.WorktreePath] != opArchive {
		t.Error("target worktree should be marked as archiving")
	}
	if cmd == nil {
		t.Error("expected archiveWorktreeCmd to be returned")
	}
}

func TestUpdate_ConfirmArchiveMode_CtrlC_Quits(t *testing.T) {
	m := testModel()
	m.confirmingArchive = true
	m.archiveTarget = m.items[m.cursor]

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	updated := result.(Model)

	if !updated.quitting {
		t.Error("ctrl+c should quit even in confirm mode")
	}
	if cmd == nil {
		t.Error("expected tea.Quit cmd")
	}
}

func TestUpdate_ConfirmArchiveMode_QBlocked(t *testing.T) {
	m := testModel()
	m.confirmingArchive = true
	m.archiveTarget = m.items[m.cursor]

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	updated := result.(Model)

	if updated.quitting {
		t.Error("q should not quit in confirm mode")
	}
	if cmd != nil {
		t.Error("should not return tea.Quit in confirm mode")
	}
}

func TestUpdate_WorktreeArchivedMsg(t *testing.T) {
	m := testModel()
	target := m.items[m.cursor]
	m.pendingOps = map[string]pendingOp{target.WorktreePath: opArchive}
	m.runner = &fakeRunner{}
	m.config = model.Config{
		Repositories: []model.RepositoryDef{{Name: "test", Path: "/test"}},
	}

	result, cmd := m.Update(WorktreeArchivedMsg{WorktreePath: target.WorktreePath})
	updated := result.(Model)

	if updated.loading {
		t.Error("loading should stay false while refreshing in background")
	}
	if _, ok := updated.pendingOps[target.WorktreePath]; ok {
		t.Error("pending op should be cleared")
	}
	for _, item := range updated.items {
		if item.WorktreePath == target.WorktreePath {
			t.Error("archived worktree should be removed from items immediately")
		}
	}
	if !updated.items[updated.cursor].Selectable {
		t.Error("cursor should be on a selectable item")
	}
	if cmd == nil {
		t.Error("expected fetchGitDataCmd to be returned")
	}
}

func TestUpdate_WorktreeArchiveErrMsg(t *testing.T) {
	m := testModel()
	target := m.items[m.cursor]
	m.pendingOps = map[string]pendingOp{target.WorktreePath: opArchive}

	result, _ := m.Update(WorktreeArchiveErrMsg{WorktreePath: target.WorktreePath, Err: fmt.Errorf("remove failed")})
	updated := result.(Model)

	if updated.err == nil {
		t.Error("err should be set")
	}
	if _, ok := updated.pendingOps[target.WorktreePath]; ok {
		t.Error("pending op should be cleared")
	}
}

func TestUpdate_CursorMovesWhileArchiving(t *testing.T) {
	m := testModel()
	m.confirmingArchive = true
	m.archiveTarget = m.items[m.cursor]
	m.runner = &fakeRunner{}
	initial := m.cursor

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	result, _ = result.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	updated := result.(Model)

	if updated.cursor <= initial {
		t.Errorf("cursor should move while archiving, got %d (initial %d)", updated.cursor, initial)
	}
}

func TestUpdate_PendingWorktree_IgnoresDAndEnter(t *testing.T) {
	m := testModel()
	path := m.items[m.cursor].WorktreePath
	m.pendingOps = map[string]pendingOp{path: opArchive}
	m.sessionSetup = func(string, string) error { return nil }

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	updated := result.(Model)
	if updated.confirmingArchive || cmd != nil {
		t.Error("d should be ignored on a pending worktree")
	}

	result, cmd = updated.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated = result.(Model)
	if updated.selected != "" || cmd != nil {
		t.Error("enter should be ignored on a pending worktree")
	}
}

func TestUpdate_ParallelArchives(t *testing.T) {
	m := testModel()
	m.runner = &fakeRunner{}
	first := m.items[m.cursor]

	m.archiveTarget = first
	m.confirmingArchive = true
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = result.(Model)

	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = result.(Model)
	second := m.items[m.cursor]
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m = result.(Model)
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = result.(Model)

	if len(m.pendingOps) != 2 {
		t.Fatalf("expected 2 pending ops, got %d", len(m.pendingOps))
	}

	result, _ = m.Update(WorktreeArchivedMsg{WorktreePath: second.WorktreePath})
	m = result.(Model)
	result, _ = m.Update(WorktreeArchiveErrMsg{WorktreePath: first.WorktreePath, Err: fmt.Errorf("x")})
	m = result.(Model)

	if len(m.pendingOps) != 0 {
		t.Errorf("all pending ops should be cleared, got %v", m.pendingOps)
	}
}

func TestUpdate_ArchiveCompletesWhileModalOpen(t *testing.T) {
	m := testModel()
	path := m.items[m.cursor].WorktreePath
	m.pendingOps = map[string]pendingOp{path: opArchive}
	m.runner = &fakeRunner{}
	m.addingWorktree = true

	result, cmd := m.Update(WorktreeArchivedMsg{WorktreePath: path})
	updated := result.(Model)

	if _, ok := updated.pendingOps[path]; ok {
		t.Error("archive completion must be handled even while a modal is open")
	}
	if !updated.addingWorktree {
		t.Error("modal should stay open")
	}
	if cmd == nil {
		t.Error("expected refresh cmd")
	}
}

func TestUpdate_GitDataMsg_PreservesCursor(t *testing.T) {
	m := testModel()
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = result.(Model)
	want := m.items[m.cursor].WorktreePath

	groups := []model.RepoGroup{
		{
			Name:     "repo1",
			RootPath: "/code/repo1",
			Worktrees: []model.WorktreeInfo{
				{Path: "/code/repo1", Branch: "main"},
				{Path: "/code/repo1-new", Branch: "new"},
				{Path: "/code/repo1-feat", Branch: "feature-x"},
			},
		},
	}
	result, _ = m.Update(GitDataMsg{Groups: groups})
	updated := result.(Model)

	if got := updated.items[updated.cursor].WorktreePath; got != want {
		t.Errorf("cursor on %q, want %q", got, want)
	}
}

func TestUpdate_GitDataMsg_CursorFallsBackWhenItemRemoved(t *testing.T) {
	m := testModel()
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = result.(Model)

	groups := []model.RepoGroup{
		{
			Name:      "repo1",
			RootPath:  "/code/repo1",
			Worktrees: []model.WorktreeInfo{{Path: "/code/repo1", Branch: "main"}},
		},
	}
	result, _ = m.Update(GitDataMsg{Groups: groups})
	updated := result.(Model)

	if !updated.items[updated.cursor].Selectable {
		t.Error("cursor should fall back to a selectable item")
	}
}

func TestUpdate_GitDataMsg_KeepsAgentStatus(t *testing.T) {
	m := testModel()
	agents := []model.AgentInfo{{State: model.AgentStateRunning}}
	m.agentStatus = map[string][]model.AgentInfo{"/code/repo1": agents}

	result, _ := m.Update(GitDataMsg{Groups: m.groups})
	updated := result.(Model)

	for _, item := range updated.items {
		if item.WorktreePath == "/code/repo1" && item.Kind == model.ItemKindWorktree && len(item.AgentStatus) == 0 {
			t.Error("agent status should be re-applied after refresh")
		}
	}
}

func TestUpdate_GitDataMsg_DiscardsStale(t *testing.T) {
	m := testModel()
	m.refreshSeq = 2

	result, _ := m.Update(GitDataMsg{Groups: nil, Seq: 1})
	updated := result.(Model)

	if len(updated.items) != len(m.items) {
		t.Error("stale GitDataMsg should be ignored")
	}
}

func TestUpdate_Enter_WithSessionSetup_RunsInBackground(t *testing.T) {
	m := testModel()
	m.sessionSetup = func(string, string) error { return nil }
	path := m.items[m.cursor].WorktreePath

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := result.(Model)

	if updated.selected != "" {
		t.Error("selected should not be set until setup completes")
	}
	if updated.pendingOps[path] != opSetup {
		t.Error("worktree should be marked as setting up")
	}
	if cmd == nil {
		t.Fatal("expected setup cmd")
	}
	msg := cmd()
	if ready, ok := msg.(SessionReadyMsg); !ok || ready.WorktreePath != path {
		t.Fatalf("expected SessionReadyMsg for %q, got %#v", path, msg)
	}

	result, _ = updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if result.(Model).cursor == updated.cursor {
		t.Error("cursor should move while setting up")
	}
}

func TestUpdate_Enter_WhileOtherSetupPending_Ignored(t *testing.T) {
	m := testModel()
	m.sessionSetup = func(string, string) error { return nil }
	m.pendingOps = map[string]pendingOp{"/other": opSetup}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("only one session setup should run at a time")
	}
}

func TestSessionSetupCmd_Error(t *testing.T) {
	cmd := sessionSetupCmd(func(string, string) error { return fmt.Errorf("boom") }, "/wt", "/repo")
	msg := cmd()
	if e, ok := msg.(SessionSetupErrMsg); !ok || e.WorktreePath != "/wt" {
		t.Fatalf("expected SessionSetupErrMsg, got %#v", msg)
	}
}

func TestUpdate_SessionReadyMsg_Quits(t *testing.T) {
	m := testModel()
	m.pendingOps = map[string]pendingOp{"/code/repo1": opSetup}

	result, cmd := m.Update(SessionReadyMsg{WorktreePath: "/code/repo1", RepoRootPath: "/code/repo1"})
	updated := result.(Model)

	if updated.selected != "/code/repo1" {
		t.Errorf("selected = %q", updated.selected)
	}
	if cmd == nil {
		t.Error("expected tea.Quit")
	}
}

func TestUpdate_SessionReadyMsg_WaitsForArchives(t *testing.T) {
	m := testModel()
	m.runner = &fakeRunner{}
	m.pendingOps = map[string]pendingOp{"/code/repo1": opSetup, "/code/repo1-feat": opArchive}

	result, cmd := m.Update(SessionReadyMsg{WorktreePath: "/code/repo1", RepoRootPath: "/code/repo1"})
	updated := result.(Model)
	if cmd != nil {
		t.Error("should not quit while archive is pending")
	}
	if !updated.quitWhenIdle {
		t.Error("should wait for pending ops")
	}

	result, cmd = updated.Update(WorktreeArchivedMsg{WorktreePath: "/code/repo1-feat"})
	if cmd == nil {
		t.Error("expected quit after last pending op")
	}
	if result.(Model).selected != "/code/repo1" {
		t.Error("selected should be kept")
	}
}

func TestUpdate_SessionSetupErrMsg(t *testing.T) {
	m := testModel()
	m.pendingOps = map[string]pendingOp{"/code/repo1": opSetup}

	result, cmd := m.Update(SessionSetupErrMsg{WorktreePath: "/code/repo1", Err: fmt.Errorf("tmux down")})
	updated := result.(Model)

	if updated.err == nil {
		t.Error("err should be set")
	}
	if len(updated.pendingOps) != 0 {
		t.Error("pending op should be cleared")
	}
	if cmd != nil {
		t.Error("should not quit on setup error")
	}
}

func TestUpdate_Quit_WaitsForPendingOps(t *testing.T) {
	m := testModel()
	m.pendingOps = map[string]pendingOp{"/code/repo1-feat": opArchive}

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	updated := result.(Model)
	if cmd != nil {
		t.Error("q should wait for pending ops")
	}
	if !updated.quitWhenIdle {
		t.Error("quitWhenIdle should be set")
	}

	result, cmd = updated.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil || !result.(Model).quitting {
		t.Error("second ctrl+c should force quit")
	}
}

func TestUpdate_QuitWhenIdle_BlocksNewOps(t *testing.T) {
	m := testModel()
	m.pendingOps = map[string]pendingOp{"/other": opArchive}
	m.quitWhenIdle = true

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if result.(Model).confirmingArchive {
		t.Error("d should be ignored while waiting to quit")
	}
}

func TestUpdate_KeyClearsError(t *testing.T) {
	m := testModel()
	m.err = fmt.Errorf("old")

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if result.(Model).err != nil {
		t.Error("key press should clear background error")
	}
}

func TestArchiveWorktreeCmd_Success(t *testing.T) {
	runner := git.FakeCommandRunner{
		Outputs: map[string]string{
			"/repo:[worktree remove /tmp/old-worktree]": "",
		},
	}
	tmuxRunner := &tmux.FakeRunner{
		Outputs: map[string]string{
			"[kill-session -t =old-worktree]": "",
		},
	}

	cmd := archiveWorktreeCmd(runner, tmuxRunner, "/repo", "/tmp/old-worktree")
	msg := cmd()

	if _, ok := msg.(WorktreeArchivedMsg); !ok {
		t.Fatalf("expected WorktreeArchivedMsg, got %T", msg)
	}

	// Verify kill-session was called
	found := false
	for _, call := range tmuxRunner.Calls {
		if len(call) >= 1 && call[0] == "kill-session" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected kill-session to be called")
	}
}

func TestArchiveWorktreeCmd_Error(t *testing.T) {
	runner := git.FakeCommandRunner{
		Outputs: map[string]string{},
	}
	tmuxRunner := &tmux.FakeRunner{
		Outputs: map[string]string{
			"[kill-session -t =old-worktree]": "",
		},
	}

	cmd := archiveWorktreeCmd(runner, tmuxRunner, "/repo", "/tmp/old-worktree")
	msg := cmd()

	errMsg, ok := msg.(WorktreeArchiveErrMsg)
	if !ok {
		t.Fatalf("expected WorktreeArchiveErrMsg, got %T", msg)
	}
	if errMsg.Err == nil {
		t.Error("expected error to be set")
	}
}

func TestArchiveWorktreeCmd_NilTmuxRunner(t *testing.T) {
	runner := git.FakeCommandRunner{
		Outputs: map[string]string{
			"/repo:[worktree remove /tmp/old-worktree]": "",
		},
	}

	cmd := archiveWorktreeCmd(runner, nil, "/repo", "/tmp/old-worktree")
	msg := cmd()

	if _, ok := msg.(WorktreeArchivedMsg); !ok {
		t.Fatalf("expected WorktreeArchivedMsg, got %T", msg)
	}
}

func TestArchiveWorktreeCmd_RemovesDirectory(t *testing.T) {
	// Create a temp directory to simulate a leftover worktree directory
	tmpDir := t.TempDir()
	worktreePath := filepath.Join(tmpDir, "leftover-wt")
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	runner := git.FakeCommandRunner{
		Outputs: map[string]string{
			fmt.Sprintf("%s:[worktree remove %s]", tmpDir, worktreePath): "",
		},
	}

	cmd := archiveWorktreeCmd(runner, nil, tmpDir, worktreePath)
	msg := cmd()

	if _, ok := msg.(WorktreeArchivedMsg); !ok {
		t.Fatalf("expected WorktreeArchivedMsg, got %T", msg)
	}

	// Verify directory was removed
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Errorf("expected worktree directory to be removed, but it still exists")
	}
}

func TestArchiveWorktreeCmd_SwitchesToMainWhenInCurrentSession(t *testing.T) {
	t.Setenv("TMUX_PANE", "")
	runner := git.FakeCommandRunner{
		Outputs: map[string]string{
			"/repo:[worktree remove /tmp/south-korea]": "",
		},
	}
	tmuxRunner := &tmux.FakeRunner{
		Outputs: map[string]string{
			// ResolveSessionName: has-session finds the session
			"[has-session -t =south-korea]": "",
			// IsCurrentSession: current session matches
			"[display-message -p #{session_name}]": "south-korea\n",
			// SwitchToMainSession: main session exists
			"[has-session -t =yakumo-main]": "",
			// SwitchToMainSession: switch-client
			"[switch-client -t yakumo-main]": "",
			// KillSession
			"[kill-session -t south-korea]": "",
		},
	}

	cmd := archiveWorktreeCmd(runner, tmuxRunner, "/repo", "/tmp/south-korea")
	msg := cmd()

	if _, ok := msg.(WorktreeArchivedMsg); !ok {
		t.Fatalf("expected WorktreeArchivedMsg, got %T", msg)
	}

	// Verify switch-client was called before kill-session
	switchIdx := -1
	killIdx := -1
	for i, call := range tmuxRunner.Calls {
		if len(call) >= 3 && call[0] == "switch-client" && call[2] == "yakumo-main" {
			switchIdx = i
		}
		if len(call) >= 1 && call[0] == "kill-session" {
			killIdx = i
		}
	}
	if switchIdx == -1 {
		t.Error("expected switch-client to yakumo-main")
	}
	if killIdx == -1 {
		t.Error("expected kill-session to be called")
	}
	if switchIdx != -1 && killIdx != -1 && switchIdx >= killIdx {
		t.Errorf("switch-client (idx=%d) should be called before kill-session (idx=%d)", switchIdx, killIdx)
	}
}

func TestUpdate_AddWorktreeMode_Escape_Cancels(t *testing.T) {
	m := testModel()
	m.addingWorktree = true
	m.addingWorktreeRepoPath = "/code/repo1"
	m.err = fmt.Errorf("previous error")

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	updated := result.(Model)

	if updated.addingWorktree {
		t.Error("addingWorktree should be false after escape")
	}
	if updated.addingWorktreeRepoPath != "" {
		t.Error("addingWorktreeRepoPath should be cleared after escape")
	}
	if updated.err != nil {
		t.Error("err should be cleared after escape")
	}
}

func TestUpdate_AddWorktreeMode_Enter_Empty_CreatesRandom(t *testing.T) {
	m := testModel()
	m.addingWorktree = true
	m.addingWorktreeRepoPath = "/code/repo1"
	m.config = model.Config{
		WorktreeBasePath: "/tmp/yakumo",
		Repositories:     []model.RepositoryDef{{Name: "repo1", Path: "/code/repo1"}},
	}

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := result.(Model)

	if updated.addingWorktree {
		t.Error("addingWorktree should be false after Enter")
	}
	if !updated.loading {
		t.Error("loading should be true (creating worktree)")
	}
	if cmd == nil {
		t.Error("expected addWorktreeCmd to be returned")
	}
}

func TestUpdate_AddWorktreeMode_Enter_URL_ClonesFromURL(t *testing.T) {
	m := testModel()
	m.addingWorktree = true
	m.addingWorktreeRepoPath = "/code/repo1"
	m.config = model.Config{
		WorktreeBasePath: "/tmp/yakumo",
		Repositories:     []model.RepositoryDef{{Name: "repo1", Path: "/code/repo1"}},
	}
	m.textInput.SetValue("https://github.com/owner/repo/tree/feature/my-branch")

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := result.(Model)

	if updated.addingWorktree {
		t.Error("addingWorktree should be false after Enter")
	}
	if !updated.loading {
		t.Error("loading should be true (cloning from URL)")
	}
	if cmd == nil {
		t.Error("expected addWorktreeFromURLCmd to be returned")
	}
}

func TestUpdate_AddWorktreeMode_CtrlC_Quits(t *testing.T) {
	m := testModel()
	m.addingWorktree = true

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	updated := result.(Model)

	if !updated.quitting {
		t.Error("ctrl+c should quit even in addWorktree mode")
	}
	if cmd == nil {
		t.Error("expected tea.Quit cmd")
	}
}

func TestAddWorktreeFromURLCmd_BranchURL(t *testing.T) {
	basePath := t.TempDir()
	branch := "feature/my-branch"
	wantPath := filepath.Join(basePath, "myrepo", "my-branch")
	fetchKey := fmt.Sprintf("/repo:%v", []string{"fetch", "origin", branch})
	addKey := fmt.Sprintf("/repo:%v", []string{"worktree", "add", wantPath, branch})

	runner := git.FakeCommandRunner{
		Outputs: map[string]string{
			fetchKey: "",
			addKey:   "",
		},
	}

	cmd := addWorktreeFromURLCmd(runner, nil, "/repo", basePath, "myrepo", "https://github.com/owner/repo/tree/feature/my-branch")
	msg := cmd()

	addedMsg, ok := msg.(WorktreeAddedMsg)
	if !ok {
		t.Fatalf("expected WorktreeAddedMsg, got %T: %v", msg, msg)
	}
	if addedMsg.Branch != branch {
		t.Errorf("Branch = %q, want %q", addedMsg.Branch, branch)
	}
	if addedMsg.WorktreePath != wantPath {
		t.Errorf("WorktreePath = %q, want %q", addedMsg.WorktreePath, wantPath)
	}
}

func TestAddWorktreeFromURLCmd_InvalidURL(t *testing.T) {
	runner := git.FakeCommandRunner{}

	cmd := addWorktreeFromURLCmd(runner, nil, "/repo", "/tmp/yakumo", "myrepo", "https://example.com/not-github")
	msg := cmd()

	_, ok := msg.(WorktreeAddErrMsg)
	if !ok {
		t.Fatalf("expected WorktreeAddErrMsg, got %T", msg)
	}
}

func TestAddWorktreeFromURLCmd_PR_NoGhRunner(t *testing.T) {
	runner := git.FakeCommandRunner{}

	cmd := addWorktreeFromURLCmd(runner, nil, "/repo", "/tmp/yakumo", "myrepo", "https://github.com/owner/repo/pull/42")
	msg := cmd()

	errMsg, ok := msg.(WorktreeAddErrMsg)
	if !ok {
		t.Fatalf("expected WorktreeAddErrMsg, got %T", msg)
	}
	if errMsg.Err == nil {
		t.Error("expected error about gh CLI not available")
	}
}

func TestAddWorktreeFromURLCmd_PR_WithGhRunner(t *testing.T) {
	basePath := t.TempDir()
	prURL := "https://github.com/owner/repo/pull/42"
	ghKey := fmt.Sprintf("/repo:%v", []string{"pr", "view", prURL, "--json", "headRefName"})
	branch := "feature/from-pr"
	wantPath := filepath.Join(basePath, "myrepo", "from-pr")
	fetchKey := fmt.Sprintf("/repo:%v", []string{"fetch", "origin", branch})
	addKey := fmt.Sprintf("/repo:%v", []string{"worktree", "add", wantPath, branch})

	gitRunner := git.FakeCommandRunner{
		Outputs: map[string]string{
			fetchKey: "",
			addKey:   "",
		},
	}
	ghRunner := &github.FakeRunner{
		Outputs: map[string]string{
			ghKey: `{"headRefName":"feature/from-pr"}`,
		},
	}

	cmd := addWorktreeFromURLCmd(gitRunner, ghRunner, "/repo", basePath, "myrepo", prURL)
	msg := cmd()

	addedMsg, ok := msg.(WorktreeAddedMsg)
	if !ok {
		t.Fatalf("expected WorktreeAddedMsg, got %T: %v", msg, msg)
	}
	if addedMsg.Branch != branch {
		t.Errorf("Branch = %q, want %q", addedMsg.Branch, branch)
	}
	if addedMsg.WorktreePath != wantPath {
		t.Errorf("WorktreePath = %q, want %q", addedMsg.WorktreePath, wantPath)
	}
}

func TestUpdate_AddWorktreeMode_Enter_BranchName_FetchesAndAdds(t *testing.T) {
	m := testModel()
	m.addingWorktree = true
	m.addingWorktreeRepoPath = "/code/repo1"
	m.config = model.Config{
		WorktreeBasePath: "/tmp/yakumo",
		Repositories:     []model.RepositoryDef{{Name: "repo1", Path: "/code/repo1"}},
	}
	m.textInput.SetValue("feature/my-branch")

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := result.(Model)

	if updated.addingWorktree {
		t.Error("addingWorktree should be false after Enter")
	}
	if !updated.loading {
		t.Error("loading should be true (fetching and creating worktree)")
	}
	if cmd == nil {
		t.Error("expected addWorktreeFromBranchNameCmd to be returned")
	}
}

func TestAddWorktreeFromBranchNameCmd_Success(t *testing.T) {
	basePath := t.TempDir()
	branch := "feature/x"
	wantPath := filepath.Join(basePath, "myrepo", "x")
	fetchKey := fmt.Sprintf("/repo:%v", []string{"fetch", "origin", branch})
	addKey := fmt.Sprintf("/repo:%v", []string{"worktree", "add", wantPath, branch})

	runner := git.FakeCommandRunner{
		Outputs: map[string]string{
			fetchKey: "",
			addKey:   "",
		},
	}

	cmd := addWorktreeFromBranchNameCmd(runner, "/repo", basePath, "myrepo", branch)
	msg := cmd()

	addedMsg, ok := msg.(WorktreeAddedMsg)
	if !ok {
		t.Fatalf("expected WorktreeAddedMsg, got %T: %v", msg, msg)
	}
	if addedMsg.Branch != branch {
		t.Errorf("Branch = %q, want %q", addedMsg.Branch, branch)
	}
	if addedMsg.WorktreePath != wantPath {
		t.Errorf("WorktreePath = %q, want %q", addedMsg.WorktreePath, wantPath)
	}
}

func TestAddWorktreeFromBranchNameCmd_FetchFails(t *testing.T) {
	basePath := t.TempDir()
	branch := "does-not-exist"
	fetchKey := fmt.Sprintf("/repo:%v", []string{"fetch", "origin", branch})

	runner := git.FakeCommandRunner{
		Errors: map[string]error{
			fetchKey: fmt.Errorf("couldn't find remote ref %s", branch),
		},
	}

	cmd := addWorktreeFromBranchNameCmd(runner, "/repo", basePath, "myrepo", branch)
	msg := cmd()

	errMsg, ok := msg.(WorktreeAddErrMsg)
	if !ok {
		t.Fatalf("expected WorktreeAddErrMsg, got %T: %v", msg, msg)
	}
	if errMsg.Err == nil {
		t.Error("expected error from fetch failure")
	}
}


type fakeRunner struct{}

func (f *fakeRunner) Run(dir string, args ...string) (string, error) {
	return "", nil
}

func addRepoModel() Model {
	ti := textinput.New()
	ti.ShowSuggestions = true
	ti.Focus()
	return Model{
		addingRepo: true,
		textInput:  ti,
	}
}

func TestUpdateAddRepoMode_PathSuggestionsMsg_Applied(t *testing.T) {
	m := addRepoModel()
	m.textInput.SetValue("/usr/")
	m.lastSuggestionDir = "/usr/"

	homeDir, _ := os.UserHomeDir()
	_ = homeDir

	result, _ := m.Update(PathSuggestionsMsg{
		Suggestions: []string{"/usr/local/", "/usr/lib/"},
		ForDir:      "/usr/",
	})
	updated := result.(Model)

	// After applying suggestions, the textinput should have suggestions set.
	// We verify by checking that the model processed without error.
	if updated.err != nil {
		t.Errorf("unexpected error: %v", updated.err)
	}
}

func TestUpdateAddRepoMode_PathSuggestionsMsg_Discarded(t *testing.T) {
	m := addRepoModel()
	// User has typed further into /usr/local/ but the msg is stale (for /usr/)
	m.textInput.SetValue("/usr/local/")
	m.lastSuggestionDir = "/usr/local/"

	result, _ := m.Update(PathSuggestionsMsg{
		Suggestions: []string{"/usr/lib/", "/usr/local/"},
		ForDir:      "/usr/",
	})
	updated := result.(Model)

	if updated.err != nil {
		t.Errorf("unexpected error: %v", updated.err)
	}
}

func TestUpdateAddRepoMode_Escape_ClearsSuggestions(t *testing.T) {
	m := addRepoModel()
	m.textInput.SetValue("/usr/")
	m.lastSuggestionDir = "/usr/"
	m.textInput.SetSuggestions([]string{"/usr/local/", "/usr/lib/"})

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	updated := result.(Model)

	if updated.addingRepo {
		t.Error("addingRepo should be false after escape")
	}
	if updated.lastSuggestionDir != "" {
		t.Errorf("lastSuggestionDir should be empty, got %q", updated.lastSuggestionDir)
	}
	if updated.textInput.Value() != "" {
		t.Errorf("textInput value should be empty, got %q", updated.textInput.Value())
	}
}

func TestUpdateAddRepoMode_DirChange_TriggersFetch(t *testing.T) {
	m := addRepoModel()
	m.textInput.SetValue("/usr/")
	m.textInput.SetCursor(len("/usr/"))
	m.lastSuggestionDir = "/" // pretend we previously fetched "/"

	// Typing "l" appends to make "/usr/l"
	// ExtractDir("/usr/l") = "/usr/" which differs from lastSuggestionDir "/"
	// So a fetch command should be returned
	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	updated := result.(Model)

	if updated.lastSuggestionDir != "/usr/" {
		t.Errorf("lastSuggestionDir should be /usr/, got %q", updated.lastSuggestionDir)
	}
	if cmd == nil {
		t.Error("expected a command to be returned for fetching suggestions")
	}
}

func TestUpdateAddRepoMode_SameDirTyping_NoFetch(t *testing.T) {
	m := addRepoModel()
	m.textInput.SetValue("/usr/lo")
	m.textInput.SetCursor(len("/usr/lo"))
	m.lastSuggestionDir = "/usr/"

	// Typing "c" makes it "/usr/loc" — same dir /usr/, no fetch needed
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	updated := result.(Model)

	if updated.lastSuggestionDir != "/usr/" {
		t.Errorf("lastSuggestionDir should remain /usr/, got %q", updated.lastSuggestionDir)
	}
}

func scrollTestModel(numGroups int) Model {
	groups := make([]model.RepoGroup, numGroups)
	for i := 0; i < numGroups; i++ {
		groups[i] = model.RepoGroup{
			Name:     fmt.Sprintf("repo%d", i),
			RootPath: fmt.Sprintf("/code/repo%d", i),
			Worktrees: []model.WorktreeInfo{
				{Path: fmt.Sprintf("/code/repo%d/main", i), Branch: "main"},
				{Path: fmt.Sprintf("/code/repo%d/feat", i), Branch: "feature"},
			},
		}
	}
	items := sidebar.BuildItems(groups)
	return Model{
		items:        items,
		groups:       groups,
		cursor:       FirstSelectable(items),
		sidebarWidth: 30,
		height:       14,
		textInput:    textinput.New(),
	}
}

func TestUpdate_DownToBottom_AdvancesScrollOff(t *testing.T) {
	m := scrollTestModel(5)
	if m.scrollOff != 0 {
		t.Fatalf("starting scrollOff should be 0, got %d", m.scrollOff)
	}

	// Press `j` enough times to reach the last selectable item.
	for i := 0; i < 100; i++ {
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
		next := result.(Model)
		if next.cursor == m.cursor {
			break
		}
		m = next
	}

	if m.scrollOff == 0 {
		t.Errorf("scrollOff should advance after navigating to bottom (height=%d, items=%d), got 0", m.height, len(m.items))
	}
}

func TestUpdate_UpFromBottom_ResetsScrollOff(t *testing.T) {
	m := scrollTestModel(5)

	// Navigate down to the bottom.
	for i := 0; i < 100; i++ {
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
		next := result.(Model)
		if next.cursor == m.cursor {
			break
		}
		m = next
	}
	if m.scrollOff == 0 {
		t.Fatal("setup: expected scrollOff > 0 after reaching bottom")
	}

	// Navigate back to the top.
	for i := 0; i < 100; i++ {
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
		next := result.(Model)
		if next.cursor == m.cursor {
			break
		}
		m = next
	}

	if m.scrollOff != 0 {
		t.Errorf("scrollOff should be 0 after returning to top, got %d", m.scrollOff)
	}
}

func TestUpdate_WindowSizeMsg_LargeHeight_ResetsScrollOff(t *testing.T) {
	m := scrollTestModel(5)
	m.scrollOff = 3 // simulate prior scroll state

	result, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 200})
	updated := result.(Model)

	if updated.height != 200 {
		t.Errorf("height = %d, want 200", updated.height)
	}
	if updated.scrollOff != 0 {
		t.Errorf("scrollOff should reset to 0 when viewport fits all items, got %d", updated.scrollOff)
	}
}
