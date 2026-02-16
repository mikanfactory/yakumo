package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"

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
				{Path: "/code/repo", Branch: "main", Status: model.StatusInfo{Insertions: 888, Deletions: 89}},
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

	if !strings.Contains(view, "+888") {
		t.Errorf("view should contain '+888' insertions, got:\n%s", view)
	}
	if !strings.Contains(view, "-89") {
		t.Errorf("view should contain '-89' deletions, got:\n%s", view)
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

func TestFormatStatus_InsertionsAndDeletions(t *testing.T) {
	result := FormatStatus(model.StatusInfo{Insertions: 42, Deletions: 7})
	if !strings.Contains(result, "+42") {
		t.Error("should contain +42")
	}
	if !strings.Contains(result, "-7") {
		t.Error("should contain -7")
	}
}

func TestFormatStatus_InsertionsOnly(t *testing.T) {
	result := FormatStatus(model.StatusInfo{Insertions: 10})
	if !strings.Contains(result, "+10") {
		t.Error("should contain +10")
	}
	if strings.Contains(result, "-") {
		t.Error("should not contain deletions")
	}
}

func TestFormatStatus_DeletionsOnly(t *testing.T) {
	result := FormatStatus(model.StatusInfo{Deletions: 5})
	if !strings.Contains(result, "-5") {
		t.Error("should contain -5")
	}
	if strings.Contains(result, "+") {
		t.Error("should not contain insertions")
	}
}

func TestView_ShowsClickHelp(t *testing.T) {
	m := testModel()
	view := m.View()

	if !strings.Contains(view, "click") {
		t.Error("help text should mention 'click'")
	}
}

func TestView_AddRepoMode(t *testing.T) {
	m := testModel()
	m.addingRepo = true
	m.textInput = textinput.New()
	m.textInput.Focus()

	view := m.View()

	if !strings.Contains(view, "Add Repository") {
		t.Errorf("add repo view should contain title, got:\n%s", view)
	}
	if !strings.Contains(view, "enter") {
		t.Errorf("add repo view should contain help text, got:\n%s", view)
	}
	if !strings.Contains(view, "esc") {
		t.Errorf("add repo view should contain cancel help, got:\n%s", view)
	}
}

func TestView_AddRepoMode_WithError(t *testing.T) {
	m := testModel()
	m.addingRepo = true
	m.textInput = textinput.New()
	m.err = fmt.Errorf("not a git repository")

	view := m.View()

	if !strings.Contains(view, "not a git repository") {
		t.Errorf("add repo view should show error, got:\n%s", view)
	}
}

func TestView_AddRepoMode_Loading(t *testing.T) {
	m := testModel()
	m.addingRepo = true
	m.loading = true

	view := m.View()

	if !strings.Contains(view, "Validating") {
		t.Errorf("add repo loading view should show validating, got:\n%s", view)
	}
}

func TestAgentIcon_Empty(t *testing.T) {
	result := AgentIcon(nil)
	if result != "" {
		t.Errorf("empty agents should return empty string, got %q", result)
	}
}

func TestAgentIcon_Idle(t *testing.T) {
	agents := []model.AgentInfo{{PaneID: "%0", State: model.AgentStateIdle}}
	result := AgentIcon(agents)
	if !strings.Contains(result, iconAgent) {
		t.Errorf("idle icon should contain %q, got %q", iconAgent, result)
	}
}

func TestAgentIcon_Running(t *testing.T) {
	agents := []model.AgentInfo{{PaneID: "%0", State: model.AgentStateRunning, Elapsed: "2m"}}
	result := AgentIcon(agents)
	if !strings.Contains(result, iconAgent) {
		t.Errorf("running icon should contain %q, got %q", iconAgent, result)
	}
}

func TestAgentIcon_Waiting(t *testing.T) {
	agents := []model.AgentInfo{{PaneID: "%0", State: model.AgentStateWaiting}}
	result := AgentIcon(agents)
	if !strings.Contains(result, iconAgent) {
		t.Errorf("waiting icon should contain %q, got %q", iconAgent, result)
	}
}

func TestAgentIcon_HighestPriority(t *testing.T) {
	agents := []model.AgentInfo{
		{PaneID: "%0", State: model.AgentStateIdle},
		{PaneID: "%1", State: model.AgentStateRunning},
		{PaneID: "%2", State: model.AgentStateIdle},
	}
	result := AgentIcon(agents)
	if !strings.Contains(result, iconAgent) {
		t.Errorf("should show highest priority icon (running), got %q", result)
	}
}

func TestView_ShowsAgentIcon(t *testing.T) {
	groups := []model.RepoGroup{
		{
			Name:     "repo",
			RootPath: "/code/repo",
			Worktrees: []model.WorktreeInfo{
				{Path: "/code/repo", Branch: "main"},
			},
		},
	}
	items := sidebar.BuildItems(groups)
	for i := range items {
		if items[i].Kind == model.ItemKindWorktree {
			items[i].AgentStatus = []model.AgentInfo{
				{PaneID: "%0", State: model.AgentStateRunning, Elapsed: "5m"},
			}
		}
	}

	m := Model{
		items:        items,
		groups:       groups,
		cursor:       FirstSelectable(items),
		sidebarWidth: 60,
	}
	view := m.View()

	if !strings.Contains(view, iconAgent) {
		t.Errorf("view should contain agent icon, got:\n%s", view)
	}
}

func TestView_NoAgentIconWhenNoAgents(t *testing.T) {
	groups := []model.RepoGroup{
		{
			Name:     "repo",
			RootPath: "/code/repo",
			Worktrees: []model.WorktreeInfo{
				{Path: "/code/repo", Branch: "main"},
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

	if strings.Contains(view, iconAgent) {
		t.Errorf("view should not contain agent icons when no agents, got:\n%s", view)
	}
}

func TestView_NonSelectedWorktree(t *testing.T) {
	groups := []model.RepoGroup{
		{
			Name:     "repo",
			RootPath: "/code/repo",
			Worktrees: []model.WorktreeInfo{
				{Path: "/code/repo", Branch: "main"},
				{Path: "/code/repo-dev", Branch: "dev", Status: model.StatusInfo{Insertions: 12, Deletions: 3}},
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

func TestRenderWorktree_WithStatus(t *testing.T) {
	item := model.NavigableItem{
		Kind:   model.ItemKindWorktree,
		Label:  "feature-branch",
		Status: model.StatusInfo{Insertions: 50, Deletions: 10},
	}
	result := renderWorktree(item, false, 40)
	if strings.Contains(result, "\n") {
		t.Error("worktree with status should render as single line")
	}
	if !strings.Contains(result, "feature-branch") {
		t.Error("should contain branch name")
	}
	if !strings.Contains(result, "+50") {
		t.Error("should contain insertions")
	}
	if !strings.Contains(result, "-10") {
		t.Error("should contain deletions")
	}
}

func TestRenderWorktree_SingleLine_CleanStatus(t *testing.T) {
	item := model.NavigableItem{
		Kind:  model.ItemKindWorktree,
		Label: "main",
	}
	result := renderWorktree(item, false, 40)
	if strings.Contains(result, "\n") {
		t.Error("clean worktree should render as single line")
	}
	if !strings.Contains(result, "main") {
		t.Error("should contain branch name")
	}
}

func TestRenderWorktree_Selected_WithStatus(t *testing.T) {
	item := model.NavigableItem{
		Kind:   model.ItemKindWorktree,
		Label:  "dev",
		Status: model.StatusInfo{Insertions: 5, Deletions: 2},
	}
	result := renderWorktree(item, true, 40)
	if strings.Contains(result, "\n") {
		t.Error("selected worktree with status should render as single line")
	}
	if !strings.Contains(result, ">") {
		t.Error("should contain cursor")
	}
	if !strings.Contains(result, "dev") {
		t.Error("should contain branch name")
	}
	if !strings.Contains(result, "+5") {
		t.Error("should contain insertions")
	}
}

func TestView_ShowsArchiveHint(t *testing.T) {
	m := testModel()
	view := m.View()

	if !strings.Contains(view, "d: archive") {
		t.Errorf("help text should mention 'd: archive', got:\n%s", view)
	}
}

func TestView_ConfirmArchiveMode(t *testing.T) {
	m := testModel()
	m.confirmingArchive = true
	m.archiveTarget = m.cursor

	view := m.View()

	if !strings.Contains(view, "Archive Worktree") {
		t.Errorf("confirm view should contain title, got:\n%s", view)
	}
	if !strings.Contains(view, m.items[m.archiveTarget].Label) {
		t.Errorf("confirm view should contain branch name, got:\n%s", view)
	}
	if !strings.Contains(view, "enter") {
		t.Errorf("confirm view should contain confirm help, got:\n%s", view)
	}
	if !strings.Contains(view, "esc") {
		t.Errorf("confirm view should contain cancel help, got:\n%s", view)
	}
	if !strings.Contains(view, "The branch will be preserved") {
		t.Errorf("confirm view should explain branch preservation, got:\n%s", view)
	}
}

func TestView_ConfirmArchiveMode_WithError(t *testing.T) {
	m := testModel()
	m.confirmingArchive = true
	m.archiveTarget = m.cursor
	m.err = fmt.Errorf("worktree has uncommitted changes")

	view := m.View()

	if !strings.Contains(view, "worktree has uncommitted changes") {
		t.Errorf("confirm view should show error, got:\n%s", view)
	}
}

func TestView_ConfirmArchiveMode_Loading(t *testing.T) {
	m := testModel()
	m.confirmingArchive = true
	m.archiveTarget = m.cursor
	m.loading = true

	view := m.View()

	if !strings.Contains(view, "Removing worktree") {
		t.Errorf("confirm loading view should show removing message, got:\n%s", view)
	}
}
