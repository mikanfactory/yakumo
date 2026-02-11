package sidebar

import (
	"testing"

	"worktree-ui/internal/model"
)

func TestBuildItems_SingleRepo(t *testing.T) {
	groups := []model.RepoGroup{
		{
			Name:     "myrepo",
			RootPath: "/code/myrepo",
			Worktrees: []model.WorktreeInfo{
				{Path: "/code/myrepo", Branch: "main"},
				{Path: "/code/myrepo-feat", Branch: "feature-x", Status: model.StatusInfo{Modified: 2}},
			},
		},
	}

	items := BuildItems(groups)

	// Expected: header + 2 worktrees + add repo + settings = 5
	if len(items) != 5 {
		t.Fatalf("len(items) = %d, want 5", len(items))
	}

	// Group header
	assertItem(t, items[0], model.ItemKindGroupHeader, "myrepo", false)
	// Worktrees
	assertItem(t, items[1], model.ItemKindWorktree, "main", true)
	if items[1].WorktreePath != "/code/myrepo" {
		t.Errorf("items[1].WorktreePath = %q, want %q", items[1].WorktreePath, "/code/myrepo")
	}
	assertItem(t, items[2], model.ItemKindWorktree, "feature-x", true)
	if items[2].Status.Modified != 2 {
		t.Errorf("items[2].Status.Modified = %d, want 2", items[2].Status.Modified)
	}
	// Action items
	assertItem(t, items[3], model.ItemKindAddRepo, "+ Add repository", true)
	assertItem(t, items[4], model.ItemKindSettings, "Settings", true)
}

func TestBuildItems_MultipleRepos(t *testing.T) {
	groups := []model.RepoGroup{
		{
			Name:     "repo1",
			RootPath: "/code/repo1",
			Worktrees: []model.WorktreeInfo{
				{Path: "/code/repo1", Branch: "main"},
			},
		},
		{
			Name:     "repo2",
			RootPath: "/code/repo2",
			Worktrees: []model.WorktreeInfo{
				{Path: "/code/repo2", Branch: "develop"},
				{Path: "/code/repo2-hotfix", Branch: "hotfix"},
			},
		},
	}

	items := BuildItems(groups)

	// header1 + 1 wt + header2 + 2 wts + add + settings = 7
	if len(items) != 7 {
		t.Fatalf("len(items) = %d, want 7", len(items))
	}

	assertItem(t, items[0], model.ItemKindGroupHeader, "repo1", false)
	assertItem(t, items[1], model.ItemKindWorktree, "main", true)
	assertItem(t, items[2], model.ItemKindGroupHeader, "repo2", false)
	assertItem(t, items[3], model.ItemKindWorktree, "develop", true)
	assertItem(t, items[4], model.ItemKindWorktree, "hotfix", true)
	assertItem(t, items[5], model.ItemKindAddRepo, "+ Add repository", true)
	assertItem(t, items[6], model.ItemKindSettings, "Settings", true)
}

func TestBuildItems_EmptyGroups(t *testing.T) {
	items := BuildItems(nil)

	// add + settings = 2
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}

	assertItem(t, items[0], model.ItemKindAddRepo, "+ Add repository", true)
	assertItem(t, items[1], model.ItemKindSettings, "Settings", true)
}

func TestBuildItems_RepoWithNoWorktrees(t *testing.T) {
	groups := []model.RepoGroup{
		{
			Name:      "empty-repo",
			RootPath:  "/code/empty-repo",
			Worktrees: nil,
		},
	}

	items := BuildItems(groups)

	// header + add + settings = 3
	if len(items) != 3 {
		t.Fatalf("len(items) = %d, want 3", len(items))
	}

	assertItem(t, items[0], model.ItemKindGroupHeader, "empty-repo", false)
	assertItem(t, items[1], model.ItemKindAddRepo, "+ Add repository", true)
}

func assertItem(t *testing.T, item model.NavigableItem, kind model.ItemKind, label string, selectable bool) {
	t.Helper()
	if item.Kind != kind {
		t.Errorf("Kind = %d, want %d", item.Kind, kind)
	}
	if item.Label != label {
		t.Errorf("Label = %q, want %q", item.Label, label)
	}
	if item.Selectable != selectable {
		t.Errorf("Selectable = %v, want %v", item.Selectable, selectable)
	}
}
