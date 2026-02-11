package tui

import (
	"fmt"
	"strings"
	"testing"

	"worktree-ui/internal/model"
	"worktree-ui/internal/sidebar"
)

func TestView_ShowsBranchNames(t *testing.T) {
	m := testModel()
	view := m.View()

	if !strings.Contains(view, "main") {
		t.Error("view should contain 'main' branch name")
	}
	if !strings.Contains(view, "feature-x") {
		t.Error("view should contain 'feature-x' branch name")
	}
}

func TestView_ShowsTitle(t *testing.T) {
	m := testModel()
	view := m.View()

	if !strings.Contains(view, "Workspaces") {
		t.Error("view should contain 'Workspaces' title")
	}
}

func TestView_ShowsRepoHeader(t *testing.T) {
	m := testModel()
	view := m.View()

	if !strings.Contains(view, "repo1") {
		t.Error("view should contain 'repo1' group header")
	}
}

func TestView_ShowsCursor(t *testing.T) {
	m := testModel()
	view := m.View()

	if !strings.Contains(view, ">") {
		t.Error("view should contain cursor '>'")
	}
}

func TestView_ShowsStatusBadge(t *testing.T) {
	groups := []model.RepoGroup{
		{
			Name:     "repo",
			RootPath: "/code/repo",
			Worktrees: []model.WorktreeInfo{
				{Path: "/code/repo", Branch: "main", Status: model.StatusInfo{Modified: 3, Added: 1}},
			},
		},
	}
	items := sidebar.BuildItems(groups)

	m := Model{
		items:        items,
		groups:       groups,
		cursor:       FirstSelectable(items),
		sidebarWidth: 40,
	}
	view := m.View()

	if !strings.Contains(view, "M3") {
		t.Errorf("view should contain 'M3' status badge, got:\n%s", view)
	}
	if !strings.Contains(view, "+1") {
		t.Errorf("view should contain '+1' status badge, got:\n%s", view)
	}
}

func TestView_ShowsHelp(t *testing.T) {
	m := testModel()
	view := m.View()

	if !strings.Contains(view, "quit") {
		t.Error("view should contain help text")
	}
}

func TestView_Quitting(t *testing.T) {
	m := testModel()
	m.quitting = true
	view := m.View()

	if view != "" {
		t.Errorf("quitting view should be empty, got %q", view)
	}
}

func TestView_Selected(t *testing.T) {
	m := testModel()
	m.selected = "/code/repo1"
	view := m.View()

	if view != "/code/repo1" {
		t.Errorf("selected view = %q, want %q", view, "/code/repo1")
	}
}

func TestView_Loading(t *testing.T) {
	m := Model{loading: true}
	view := m.View()

	if !strings.Contains(view, "Loading") {
		t.Error("loading view should contain 'Loading'")
	}
}

func TestView_Error(t *testing.T) {
	m := Model{err: fmt.Errorf("some error")}
	view := m.View()

	if !strings.Contains(view, "some error") {
		t.Error("error view should contain error message")
	}
}

func TestView_ShowsActionItems(t *testing.T) {
	m := testModel()
	view := m.View()

	if !strings.Contains(view, "Add repository") {
		t.Error("view should contain 'Add repository'")
	}
	if !strings.Contains(view, "Settings") {
		t.Error("view should contain 'Settings'")
	}
}

func TestView_SelectedActionItem(t *testing.T) {
	m := testModel()
	// Navigate cursor to Settings
	for i, item := range m.items {
		if item.Kind == model.ItemKindSettings {
			m.cursor = i
			break
		}
	}
	view := m.View()

	if !strings.Contains(view, "> Settings") {
		t.Errorf("selected Settings should have cursor, got:\n%s", view)
	}
}

func TestView_TruncatesLongBranch(t *testing.T) {
	groups := []model.RepoGroup{
		{
			Name:     "repo",
			RootPath: "/code/repo",
			Worktrees: []model.WorktreeInfo{
				{Path: "/code/repo", Branch: "mikanfactory/very-long-branch-name-that-exceeds-width"},
			},
		},
	}
	items := sidebar.BuildItems(groups)

	m := Model{
		items:        items,
		groups:       groups,
		cursor:       FirstSelectable(items),
		sidebarWidth: 20,
	}
	view := m.View()

	if strings.Contains(view, "mikanfactory/very-long-branch-name-that-exceeds-width") {
		t.Error("long branch name should be truncated")
	}
	if !strings.Contains(view, "…") {
		t.Error("truncated branch should contain ellipsis")
	}
}

func TestFormatStatus_Empty(t *testing.T) {
	result := FormatStatus(model.StatusInfo{})
	if result != "" {
		t.Errorf("empty status should return empty string, got %q", result)
	}
}

func TestFormatStatus_AllTypes(t *testing.T) {
	result := FormatStatus(model.StatusInfo{Modified: 1, Added: 2, Deleted: 3, Untracked: 4})
	if !strings.Contains(result, "M1") {
		t.Error("should contain M1")
	}
	if !strings.Contains(result, "+2") {
		t.Error("should contain +2")
	}
	if !strings.Contains(result, "-3") {
		t.Error("should contain -3")
	}
	if !strings.Contains(result, "?4") {
		t.Error("should contain ?4")
	}
}

func TestView_NonSelectedWorktree(t *testing.T) {
	groups := []model.RepoGroup{
		{
			Name:     "repo",
			RootPath: "/code/repo",
			Worktrees: []model.WorktreeInfo{
				{Path: "/code/repo", Branch: "main"},
				{Path: "/code/repo-dev", Branch: "dev", Status: model.StatusInfo{Modified: 1}},
			},
		},
	}
	items := sidebar.BuildItems(groups)

	m := Model{
		items:        items,
		groups:       groups,
		cursor:       FirstSelectable(items), // cursor on "main"
		sidebarWidth: 40,
	}
	view := m.View()

	// "dev" should be rendered without cursor
	if !strings.Contains(view, "dev") {
		t.Error("non-selected worktree should still be visible")
	}
}
