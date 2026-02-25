package git

import (
	"testing"
)

func TestListWorktrees(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		wantCount  int
		wantFirst  WorktreeResult
		wantSecond WorktreeResult
	}{
		{
			name: "two worktrees",
			output: "worktree /Users/user/code/repo\n" +
				"HEAD abc123def456\n" +
				"branch refs/heads/main\n" +
				"\n" +
				"worktree /Users/user/code/repo-feature\n" +
				"HEAD def789abc012\n" +
				"branch refs/heads/mikanfactory/feature-x\n" +
				"\n",
			wantCount: 2,
			wantFirst: WorktreeResult{
				Path:   "/Users/user/code/repo",
				Branch: "main",
			},
			wantSecond: WorktreeResult{
				Path:   "/Users/user/code/repo-feature",
				Branch: "mikanfactory/feature-x",
			},
		},
		{
			name: "detached HEAD",
			output: "worktree /Users/user/code/repo\n" +
				"HEAD abc123def456\n" +
				"detached\n" +
				"\n",
			wantCount: 1,
			wantFirst: WorktreeResult{
				Path:   "/Users/user/code/repo",
				Branch: "(detached)",
			},
		},
		{
			name: "bare repo",
			output: "worktree /Users/user/code/repo\n" +
				"HEAD abc123def456\n" +
				"branch refs/heads/main\n" +
				"bare\n" +
				"\n",
			wantCount: 1,
			wantFirst: WorktreeResult{
				Path:   "/Users/user/code/repo",
				Branch: "main",
				IsBare: true,
			},
		},
		{
			name:      "empty output",
			output:    "",
			wantCount: 0,
		},
		{
			name: "single worktree no trailing newline",
			output: "worktree /repo\n" +
				"HEAD abc123\n" +
				"branch refs/heads/dev\n",
			wantCount: 1,
			wantFirst: WorktreeResult{
				Path:   "/repo",
				Branch: "dev",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := FakeCommandRunner{
				Outputs: map[string]string{
					"/repo:[worktree list --porcelain]": tt.output,
				},
			}

			results, err := ListWorktrees(runner, "/repo")
			if err != nil {
				t.Fatalf("ListWorktrees failed: %v", err)
			}

			if len(results) != tt.wantCount {
				t.Fatalf("len(results) = %d, want %d", len(results), tt.wantCount)
			}

			if tt.wantCount >= 1 {
				assertWorktree(t, results[0], tt.wantFirst, "first")
			}
			if tt.wantCount >= 2 {
				assertWorktree(t, results[1], tt.wantSecond, "second")
			}
		})
	}
}

func TestListWorktrees_Error(t *testing.T) {
	runner := FakeCommandRunner{
		Outputs: map[string]string{},
	}

	_, err := ListWorktrees(runner, "/nonexistent")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

type WorktreeResult struct {
	Path   string
	Branch string
	IsBare bool
}

func TestToWorktreeInfo(t *testing.T) {
	entries := []worktreeEntry{
		{Path: "/repo1", Branch: "main", IsBare: false},
		{Path: "/repo2", Branch: "dev", IsBare: true},
	}

	infos := ToWorktreeInfo(entries)

	if len(infos) != 2 {
		t.Fatalf("len(infos) = %d, want 2", len(infos))
	}
	if infos[0].Path != "/repo1" || infos[0].Branch != "main" {
		t.Errorf("infos[0] = %+v, want Path=/repo1 Branch=main", infos[0])
	}
	if infos[1].Path != "/repo2" || infos[1].Branch != "dev" {
		t.Errorf("infos[1] = %+v, want Path=/repo2 Branch=dev", infos[1])
	}
}

func TestToWorktreeInfo_IsBare(t *testing.T) {
	entries := []worktreeEntry{
		{Path: "/repo", Branch: "main", IsBare: true},
		{Path: "/repo-feat", Branch: "feat", IsBare: false},
	}

	infos := ToWorktreeInfo(entries)

	if !infos[0].IsBare {
		t.Error("infos[0].IsBare should be true")
	}
	if infos[1].IsBare {
		t.Error("infos[1].IsBare should be false")
	}
}

func TestAddWorktree(t *testing.T) {
	runner := FakeCommandRunner{
		Outputs: map[string]string{
			"/repo:[worktree add /tmp/new-worktree -b user/feature origin/main]": "",
		},
	}

	err := AddWorktree(runner, "/repo", "/tmp/new-worktree", "user/feature")
	if err != nil {
		t.Fatalf("AddWorktree failed: %v", err)
	}
}

func TestAddWorktree_Error(t *testing.T) {
	runner := FakeCommandRunner{
		Outputs: map[string]string{},
	}

	err := AddWorktree(runner, "/repo", "/tmp/new-worktree", "user/feature")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestRenameBranch(t *testing.T) {
	runner := FakeCommandRunner{
		Outputs: map[string]string{
			"/tmp/worktree:[branch -m user/south-korea user/fix-login]": "",
		},
	}

	err := RenameBranch(runner, "/tmp/worktree", "user/south-korea", "user/fix-login")
	if err != nil {
		t.Fatalf("RenameBranch failed: %v", err)
	}
}

func TestRenameBranch_Error(t *testing.T) {
	runner := FakeCommandRunner{
		Outputs: map[string]string{},
	}

	err := RenameBranch(runner, "/tmp/worktree", "user/south-korea", "user/fix-login")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestRemoveWorktree(t *testing.T) {
	runner := FakeCommandRunner{
		Outputs: map[string]string{
			"/repo:[worktree remove /tmp/old-worktree]": "",
		},
	}

	err := RemoveWorktree(runner, "/repo", "/tmp/old-worktree")
	if err != nil {
		t.Fatalf("RemoveWorktree failed: %v", err)
	}
}

func TestRemoveWorktree_Error(t *testing.T) {
	runner := FakeCommandRunner{
		Outputs: map[string]string{},
	}

	err := RemoveWorktree(runner, "/repo", "/tmp/old-worktree")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestFetchBranch(t *testing.T) {
	runner := FakeCommandRunner{
		Outputs: map[string]string{
			"/repo:[fetch origin feature/my-branch]": "",
		},
	}

	err := FetchBranch(runner, "/repo", "feature/my-branch")
	if err != nil {
		t.Fatalf("FetchBranch failed: %v", err)
	}
}

func TestFetchBranch_Error(t *testing.T) {
	runner := FakeCommandRunner{
		Outputs: map[string]string{},
	}

	err := FetchBranch(runner, "/repo", "nonexistent-branch")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestAddWorktreeFromBranch(t *testing.T) {
	runner := FakeCommandRunner{
		Outputs: map[string]string{
			"/repo:[worktree add /tmp/new-worktree feature/my-branch]": "",
		},
	}

	err := AddWorktreeFromBranch(runner, "/repo", "/tmp/new-worktree", "feature/my-branch")
	if err != nil {
		t.Fatalf("AddWorktreeFromBranch failed: %v", err)
	}
}

func TestAddWorktreeFromBranch_Error(t *testing.T) {
	runner := FakeCommandRunner{
		Outputs: map[string]string{},
	}

	err := AddWorktreeFromBranch(runner, "/repo", "/tmp/new-worktree", "feature/my-branch")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func assertWorktree(t *testing.T, got worktreeEntry, want WorktreeResult, label string) {
	t.Helper()
	if got.Path != want.Path {
		t.Errorf("%s.Path = %q, want %q", label, got.Path, want.Path)
	}
	if got.Branch != want.Branch {
		t.Errorf("%s.Branch = %q, want %q", label, got.Branch, want.Branch)
	}
	if got.IsBare != want.IsBare {
		t.Errorf("%s.IsBare = %v, want %v", label, got.IsBare, want.IsBare)
	}
}
