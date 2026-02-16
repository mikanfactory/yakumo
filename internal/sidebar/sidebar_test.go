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
				{Path: "/code/myrepo-feat", Branch: "feature-x", Status: model.StatusInfo{Insertions: 20, Deletions: 5}},
			},
		},
	}

	items := BuildItems(groups)

	// Expected: header + 2 worktrees + add worktree + add repo + settings = 6
	if len(items) != 6 {
		t.Fatalf("len(items) = %d, want 6", len(items))
	}

	// Group header
	assertItem(t, items[0], model.ItemKindGroupHeader, "myrepo", false)
	// Worktrees
	assertItem(t, items[1], model.ItemKindWorktree, "main", true)
	if items[1].WorktreePath != "/code/myrepo" {
		t.Errorf("items[1].WorktreePath = %q, want %q", items[1].WorktreePath, "/code/myrepo")
	}
	assertItem(t, items[2], model.ItemKindWorktree, "feature-x", true)
	if items[2].Status.Insertions != 20 {
		t.Errorf("items[2].Status.Insertions = %d, want 20", items[2].Status.Insertions)
	}
	// Add worktree
	assertItem(t, items[3], model.ItemKindAddWorktree, "+ Add worktree", true)
	if items[3].RepoRootPath != "/code/myrepo" {
		t.Errorf("items[3].RepoRootPath = %q, want %q", items[3].RepoRootPath, "/code/myrepo")
	}
	// Action items
	assertItem(t, items[4], model.ItemKindAddRepo, "+ Add repository", true)
	assertItem(t, items[5], model.ItemKindSettings, "Settings", true)
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

	// header1 + 1 wt + add-wt1 + header2 + 2 wts + add-wt2 + add + settings = 9
	if len(items) != 9 {
		t.Fatalf("len(items) = %d, want 9", len(items))
	}

	assertItem(t, items[0], model.ItemKindGroupHeader, "repo1", false)
	assertItem(t, items[1], model.ItemKindWorktree, "main", true)
	assertItem(t, items[2], model.ItemKindAddWorktree, "+ Add worktree", true)
	assertItem(t, items[3], model.ItemKindGroupHeader, "repo2", false)
	assertItem(t, items[4], model.ItemKindWorktree, "develop", true)
	assertItem(t, items[5], model.ItemKindWorktree, "hotfix", true)
	assertItem(t, items[6], model.ItemKindAddWorktree, "+ Add worktree", true)
	assertItem(t, items[7], model.ItemKindAddRepo, "+ Add repository", true)
	assertItem(t, items[8], model.ItemKindSettings, "Settings", true)
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

	// header + add-wt + add + settings = 4
	if len(items) != 4 {
		t.Fatalf("len(items) = %d, want 4", len(items))
	}

	assertItem(t, items[0], model.ItemKindGroupHeader, "empty-repo", false)
	assertItem(t, items[1], model.ItemKindAddWorktree, "+ Add worktree", true)
	if items[1].RepoRootPath != "/code/empty-repo" {
		t.Errorf("items[1].RepoRootPath = %q, want %q", items[1].RepoRootPath, "/code/empty-repo")
	}
	assertItem(t, items[2], model.ItemKindAddRepo, "+ Add repository", true)
}

func TestBuildItems_IsBare(t *testing.T) {
	groups := []model.RepoGroup{
		{
			Name:     "repo",
			RootPath: "/code/repo",
			Worktrees: []model.WorktreeInfo{
				{Path: "/code/repo", Branch: "main", IsBare: true},
				{Path: "/code/repo-feat", Branch: "feat", IsBare: false},
			},
		},
	}

	items := BuildItems(groups)

	// items[0] = header, items[1] = bare worktree, items[2] = normal worktree
	if !items[1].IsBare {
		t.Error("items[1].IsBare should be true for bare worktree")
	}
	if items[2].IsBare {
		t.Error("items[2].IsBare should be false for normal worktree")
	}
}

func TestBuildItems_RepoRootPath_OnWorktree(t *testing.T) {
	groups := []model.RepoGroup{
		{
			Name:     "repo",
			RootPath: "/code/repo",
			Worktrees: []model.WorktreeInfo{
				{Path: "/code/repo", Branch: "main"},
				{Path: "/code/repo-feat", Branch: "feat"},
			},
		},
	}

	items := BuildItems(groups)

	// items[1] and items[2] are worktrees
	if items[1].RepoRootPath != "/code/repo" {
		t.Errorf("items[1].RepoRootPath = %q, want %q", items[1].RepoRootPath, "/code/repo")
	}
	if items[2].RepoRootPath != "/code/repo" {
		t.Errorf("items[2].RepoRootPath = %q, want %q", items[2].RepoRootPath, "/code/repo")
	}
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
