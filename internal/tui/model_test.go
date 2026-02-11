package tui

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"worktree-ui/internal/model"
	"worktree-ui/internal/sidebar"
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

func TestUpdate_Enter_NoopOnAction(t *testing.T) {
	m := testModel()
	// Navigate to "Add repository" action item
	for i, item := range m.items {
		if item.Kind == model.ItemKindAddRepo {
			m.cursor = i
			break
		}
	}

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := result.(Model)

	if updated.selected != "" {
		t.Errorf("selected should be empty for action items, got %q", updated.selected)
	}
	if cmd != nil {
		t.Error("should not quit on action item")
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

	result, _ := m.Update(GitDataMsg{Groups: groups})
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

	m := NewModel(cfg, runner)

	if m.sidebarWidth != 35 {
		t.Errorf("sidebarWidth = %d, want 35", m.sidebarWidth)
	}
	if !m.loading {
		t.Error("loading should be true initially")
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
	m := NewModel(cfg, runner)

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

type fakeRunner struct{}

func (f *fakeRunner) Run(dir string, args ...string) (string, error) {
	return "", nil
}
