package github

import (
	"fmt"
	"testing"
)

func TestParseGitHubURL_BranchURL(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		wantBranch string
		wantOwner  string
		wantRepo   string
	}{
		{
			name:       "simple branch",
			url:        "https://github.com/owner/repo/tree/main",
			wantBranch: "main",
			wantOwner:  "owner",
			wantRepo:   "repo",
		},
		{
			name:       "branch with slashes",
			url:        "https://github.com/owner/repo/tree/feature/my-branch",
			wantBranch: "feature/my-branch",
			wantOwner:  "owner",
			wantRepo:   "repo",
		},
		{
			name:       "branch with username prefix",
			url:        "https://github.com/mikan/yakumo/tree/mikanfactory/fix-login",
			wantBranch: "mikanfactory/fix-login",
			wantOwner:  "mikan",
			wantRepo:   "yakumo",
		},
		{
			name:       "branch with multiple slashes",
			url:        "https://github.com/org/project/tree/feat/scope/description",
			wantBranch: "feat/scope/description",
			wantOwner:  "org",
			wantRepo:   "project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := ParseGitHubURL(tt.url)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if info.Type != URLTypeBranch {
				t.Errorf("type = %v, want URLTypeBranch", info.Type)
			}
			if info.Branch != tt.wantBranch {
				t.Errorf("branch = %q, want %q", info.Branch, tt.wantBranch)
			}
			if info.Owner != tt.wantOwner {
				t.Errorf("owner = %q, want %q", info.Owner, tt.wantOwner)
			}
			if info.Repo != tt.wantRepo {
				t.Errorf("repo = %q, want %q", info.Repo, tt.wantRepo)
			}
		})
	}
}

func TestParseGitHubURL_PRURL(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		wantNumber string
		wantOwner  string
		wantRepo   string
	}{
		{
			name:       "simple PR",
			url:        "https://github.com/owner/repo/pull/123",
			wantNumber: "123",
			wantOwner:  "owner",
			wantRepo:   "repo",
		},
		{
			name:       "PR with trailing slash",
			url:        "https://github.com/owner/repo/pull/456/",
			wantNumber: "456",
			wantOwner:  "owner",
			wantRepo:   "repo",
		},
		{
			name:       "PR with files tab",
			url:        "https://github.com/owner/repo/pull/789/files",
			wantNumber: "789",
			wantOwner:  "owner",
			wantRepo:   "repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := ParseGitHubURL(tt.url)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if info.Type != URLTypePR {
				t.Errorf("type = %v, want URLTypePR", info.Type)
			}
			if info.PRNumber != tt.wantNumber {
				t.Errorf("PRNumber = %q, want %q", info.PRNumber, tt.wantNumber)
			}
			if info.Owner != tt.wantOwner {
				t.Errorf("owner = %q, want %q", info.Owner, tt.wantOwner)
			}
			if info.Repo != tt.wantRepo {
				t.Errorf("repo = %q, want %q", info.Repo, tt.wantRepo)
			}
		})
	}
}

func TestParseGitHubURL_Invalid(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{name: "empty string", url: ""},
		{name: "not a URL", url: "not-a-url"},
		{name: "non-github URL", url: "https://gitlab.com/owner/repo/tree/main"},
		{name: "repo root only", url: "https://github.com/owner/repo"},
		{name: "tree without branch", url: "https://github.com/owner/repo/tree/"},
		{name: "pull without number", url: "https://github.com/owner/repo/pull/"},
		{name: "pull with non-numeric", url: "https://github.com/owner/repo/pull/abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseGitHubURL(tt.url)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestFetchPRBranch(t *testing.T) {
	prURL := "https://github.com/owner/repo/pull/42"
	key := fmt.Sprintf(".:%v", []string{"pr", "view", prURL, "--json", "headRefName"})

	runner := &FakeRunner{
		Outputs: map[string]string{
			key: `{"headRefName":"feature/my-branch"}` + "\n",
		},
	}

	branch, err := FetchPRBranch(runner, ".", prURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branch != "feature/my-branch" {
		t.Errorf("branch = %q, want %q", branch, "feature/my-branch")
	}
}

func TestBranchSlug(t *testing.T) {
	tests := []struct {
		branch string
		want   string
	}{
		{branch: "main", want: "main"},
		{branch: "feature/my-branch", want: "my-branch"},
		{branch: "mikanfactory/fix-login", want: "fix-login"},
		{branch: "feat/scope/description", want: "description"},
	}

	for _, tt := range tests {
		t.Run(tt.branch, func(t *testing.T) {
			got := BranchSlug(tt.branch)
			if got != tt.want {
				t.Errorf("BranchSlug(%q) = %q, want %q", tt.branch, got, tt.want)
			}
		})
	}
}
