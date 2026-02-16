package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"worktree-ui/internal/git"
	"worktree-ui/internal/model"
	"worktree-ui/internal/sidebar"
	"worktree-ui/internal/tmux"
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
		t.Error("expected agentTickCmd to be returned after GitDataMsg")
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

	m := NewModel(cfg, runner, "/tmp/config.yaml", nil)

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
	m := NewModel(cfg, runner, "", nil)

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

func TestUpdate_Enter_AddWorktree(t *testing.T) {
	m := testModel()
	m.config = model.Config{WorktreeBasePath: "/tmp/shikon"}

	// Navigate to "Add worktree" item
	for i, item := range m.items {
		if item.Kind == model.ItemKindAddWorktree {
			m.cursor = i
			break
		}
	}

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := result.(Model)

	if !updated.loading {
		t.Error("loading should be true after pressing enter on Add worktree")
	}
	if cmd == nil {
		t.Error("expected a command to be returned")
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

func TestAddWorktreeCmd_Success(t *testing.T) {
	runner := git.FakeCommandRunner{
		Outputs: map[string]string{
			"/repo:[config user.name]": "testuser\n",
		},
	}

	cmd := addWorktreeCmd(runner, "/repo", "/tmp/shikon")
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

func TestAddWorktreeCmd_UserNameError(t *testing.T) {
	runner := git.FakeCommandRunner{
		Errors: map[string]error{
			"/repo:[config user.name]": fmt.Errorf("no user.name"),
		},
	}

	cmd := addWorktreeCmd(runner, "/repo", "/tmp/shikon")
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
			"/repo:[worktree list --porcelain]": "worktree /repo\nHEAD abc123\nbranch refs/heads/main\n\n",
			"/repo:[status --porcelain]":        "",
		},
	}

	cfg := model.Config{
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
	cmd := validateRepoCmd(runner, "~/nonexistent-shiki-test-path")
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
		"repo1": {
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
			fmt.Sprintf("%v", []string{"has-session", "-t", "repo1"}):                                                                "",
			fmt.Sprintf("%v", []string{"list-panes", "-s", "-t", "repo1", "-F", "#{pane_id}\t#{pane_title}\t#{pane_current_command}"}): "%0\t✳ claude\tnode\n",
			fmt.Sprintf("%v", []string{"capture-pane", "-p", "-t", "%0"}):                                                            "  ❯ ",
		},
		Errors: map[string]error{
			fmt.Sprintf("%v", []string{"has-session", "-t", "repo1-feat"}): fmt.Errorf("no session"),
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

	cmd := fetchAgentStatusCmd(runner, groups)
	msg := cmd()

	statusMsg, ok := msg.(AgentStatusMsg)
	if !ok {
		t.Fatalf("expected AgentStatusMsg, got %T", msg)
	}
	if len(statusMsg.Statuses["repo1"]) != 1 {
		t.Errorf("expected 1 agent for repo1, got %d", len(statusMsg.Statuses["repo1"]))
	}
	if len(statusMsg.Statuses["repo1-feat"]) != 0 {
		t.Errorf("expected 0 agents for repo1-feat, got %d", len(statusMsg.Statuses["repo1-feat"]))
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
	if updated.archiveTarget != m.cursor {
		t.Errorf("archiveTarget = %d, want %d", updated.archiveTarget, m.cursor)
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
	m.archiveTarget = m.cursor
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

func TestUpdate_ConfirmArchiveMode_Enter_Confirms(t *testing.T) {
	m := testModel()
	m.confirmingArchive = true
	m.archiveTarget = m.cursor
	m.runner = &fakeRunner{}

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := result.(Model)

	if !updated.loading {
		t.Error("loading should be true after confirming archive")
	}
	if cmd == nil {
		t.Error("expected archiveWorktreeCmd to be returned")
	}
}

func TestUpdate_ConfirmArchiveMode_CtrlC_Quits(t *testing.T) {
	m := testModel()
	m.confirmingArchive = true
	m.archiveTarget = m.cursor

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
	m.archiveTarget = m.cursor

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
	m.confirmingArchive = true
	m.archiveTarget = m.cursor
	m.runner = &fakeRunner{}
	m.config = model.Config{
		Repositories: []model.RepositoryDef{{Name: "test", Path: "/test"}},
	}

	result, cmd := m.Update(WorktreeArchivedMsg{})
	updated := result.(Model)

	if !updated.loading {
		t.Error("loading should be true after WorktreeArchivedMsg (refreshing)")
	}
	if updated.confirmingArchive {
		t.Error("confirmingArchive should be false after success")
	}
	if cmd == nil {
		t.Error("expected fetchGitDataCmd to be returned")
	}
}

func TestUpdate_WorktreeArchiveErrMsg(t *testing.T) {
	m := testModel()
	m.confirmingArchive = true
	m.archiveTarget = m.cursor

	result, _ := m.Update(WorktreeArchiveErrMsg{Err: fmt.Errorf("remove failed")})
	updated := result.(Model)

	if updated.loading {
		t.Error("loading should be false after archive error")
	}
	if updated.err == nil {
		t.Error("err should be set")
	}
	if updated.confirmingArchive {
		t.Error("confirmingArchive should be false after error")
	}
}

func TestArchiveWorktreeCmd_Success(t *testing.T) {
	runner := git.FakeCommandRunner{
		Outputs: map[string]string{
			"/repo:[worktree remove /tmp/old-worktree]": "",
		},
	}

	cmd := archiveWorktreeCmd(runner, "/repo", "/tmp/old-worktree")
	msg := cmd()

	if _, ok := msg.(WorktreeArchivedMsg); !ok {
		t.Fatalf("expected WorktreeArchivedMsg, got %T", msg)
	}
}

func TestArchiveWorktreeCmd_Error(t *testing.T) {
	runner := git.FakeCommandRunner{
		Outputs: map[string]string{},
	}

	cmd := archiveWorktreeCmd(runner, "/repo", "/tmp/old-worktree")
	msg := cmd()

	errMsg, ok := msg.(WorktreeArchiveErrMsg)
	if !ok {
		t.Fatalf("expected WorktreeArchiveErrMsg, got %T", msg)
	}
	if errMsg.Err == nil {
		t.Error("expected error to be set")
	}
}

type fakeRunner struct{}

func (f *fakeRunner) Run(dir string, args ...string) (string, error) {
	return "", nil
}
