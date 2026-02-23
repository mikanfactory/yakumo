package github

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// URLType identifies the kind of GitHub URL parsed.
type URLType int

const (
	URLTypeBranch URLType = iota
	URLTypePR
)

// URLInfo holds the parsed result of a GitHub URL.
type URLInfo struct {
	Type     URLType
	Owner    string
	Repo     string
	Branch   string // populated for branch URLs
	PRNumber string // populated for PR URLs
}

// ParseGitHubURL parses a GitHub branch or PR URL and extracts its components.
func ParseGitHubURL(rawURL string) (URLInfo, error) {
	if rawURL == "" {
		return URLInfo{}, fmt.Errorf("empty URL")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return URLInfo{}, fmt.Errorf("invalid URL: %w", err)
	}

	if parsed.Host != "github.com" {
		return URLInfo{}, fmt.Errorf("not a GitHub URL: %s", parsed.Host)
	}

	// path: /owner/repo/tree/branch-name or /owner/repo/pull/123
	path := strings.TrimPrefix(parsed.Path, "/")
	path = strings.TrimSuffix(path, "/")
	segments := strings.SplitN(path, "/", 4)

	if len(segments) < 4 {
		return URLInfo{}, fmt.Errorf("unsupported GitHub URL format: need /owner/repo/tree|pull/...")
	}

	owner := segments[0]
	repo := segments[1]
	kind := segments[2]
	rest := segments[3]

	switch kind {
	case "tree":
		if rest == "" {
			return URLInfo{}, fmt.Errorf("branch name is empty")
		}
		return URLInfo{
			Type:   URLTypeBranch,
			Owner:  owner,
			Repo:   repo,
			Branch: rest,
		}, nil

	case "pull":
		// rest may be "123" or "123/files" etc.
		numberStr := strings.SplitN(rest, "/", 2)[0]
		if numberStr == "" {
			return URLInfo{}, fmt.Errorf("PR number is empty")
		}
		if _, err := strconv.Atoi(numberStr); err != nil {
			return URLInfo{}, fmt.Errorf("invalid PR number: %q", numberStr)
		}
		return URLInfo{
			Type:     URLTypePR,
			Owner:    owner,
			Repo:     repo,
			PRNumber: numberStr,
		}, nil

	default:
		return URLInfo{}, fmt.Errorf("unsupported GitHub URL type: %q (expected tree or pull)", kind)
	}
}

// prBranchResponse represents the JSON from `gh pr view --json headRefName`.
type prBranchResponse struct {
	HeadRefName string `json:"headRefName"`
}

// FetchPRBranch uses the gh CLI to get the branch name for a PR URL.
func FetchPRBranch(runner Runner, dir string, prURL string) (string, error) {
	out, err := runner.Run(dir, "pr", "view", prURL, "--json", "headRefName")
	if err != nil {
		return "", fmt.Errorf("fetching PR branch: %w", err)
	}

	var resp prBranchResponse
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &resp); err != nil {
		return "", fmt.Errorf("parsing PR branch response: %w", err)
	}

	if resp.HeadRefName == "" {
		return "", fmt.Errorf("PR has no head branch")
	}

	return resp.HeadRefName, nil
}

// BranchSlug returns the last segment of a branch name for use as a directory name.
// e.g. "feature/my-branch" -> "my-branch", "main" -> "main"
func BranchSlug(branch string) string {
	if idx := strings.LastIndex(branch, "/"); idx >= 0 {
		return branch[idx+1:]
	}
	return branch
}
