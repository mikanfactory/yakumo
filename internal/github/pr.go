package github

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// PRView represents the JSON output from `gh pr view --json ...`.
type PRView struct {
	Title             string            `json:"title"`
	Body              string            `json:"body"`
	State             string            `json:"state"`
	MergeStateStatus  string            `json:"mergeStateStatus"`
	ReviewDecision    string            `json:"reviewDecision"`
	StatusCheckRollup []StatusCheckNode `json:"statusCheckRollup"`
	Comments          []CommentNode     `json:"comments"`
}

// StatusCheckNode represents a CI check or status check.
type StatusCheckNode struct {
	Name        string    `json:"name"`
	Context     string    `json:"context"`
	State       string    `json:"state"`
	Status      string    `json:"status"`
	Conclusion  string    `json:"conclusion"`
	StartedAt   time.Time `json:"startedAt"`
	CompletedAt time.Time `json:"completedAt"`
}

// CommentNode represents a PR comment.
type CommentNode struct {
	Author    CommentAuthor `json:"author"`
	Body      string        `json:"body"`
	CreatedAt time.Time     `json:"createdAt"`
}

// CommentAuthor represents the author of a comment.
type CommentAuthor struct {
	Login string `json:"login"`
}

// CheckName returns the display name for a status check.
func (s StatusCheckNode) CheckName() string {
	if s.Name != "" {
		return s.Name
	}
	return s.Context
}

// Passed returns whether the check succeeded.
func (s StatusCheckNode) Passed() bool {
	return s.Conclusion == "SUCCESS" || s.State == "SUCCESS"
}

// DurationString returns a human-readable duration string.
func (s StatusCheckNode) DurationString() string {
	if s.CompletedAt.IsZero() || s.StartedAt.IsZero() {
		return ""
	}
	d := s.CompletedAt.Sub(s.StartedAt)
	if d.Minutes() >= 1 {
		return fmt.Sprintf("%.0fm", d.Minutes())
	}
	return fmt.Sprintf("%.0fs", d.Seconds())
}

// Preview returns a truncated preview of the comment body.
func (c CommentNode) Preview(maxLen int) string {
	// Strip common HTML tags for preview
	body := c.Body
	body = strings.ReplaceAll(body, "\n", " ")
	body = strings.ReplaceAll(body, "\r", "")

	if len(body) > maxLen {
		return body[:maxLen] + "..."
	}
	return body
}

var prViewFields = "title,body,state,mergeStateStatus,reviewDecision,statusCheckRollup,comments"

// FetchPR runs `gh pr view` and returns the parsed PR data.
func FetchPR(runner Runner, dir string) (PRView, error) {
	out, err := runner.Run(dir, "pr", "view", "--json", prViewFields)
	if err != nil {
		return PRView{}, err
	}

	var pr PRView
	if err := json.Unmarshal([]byte(out), &pr); err != nil {
		return PRView{}, fmt.Errorf("failed to parse gh pr view output: %w", err)
	}

	return pr, nil
}

// MapMergeStateStatus converts GitHub's mergeStateStatus to a display string.
func MapMergeStateStatus(mergeState string, reviewDecision string) string {
	switch mergeState {
	case "CLEAN":
		return "Ready to merge"
	case "BLOCKED":
		if reviewDecision == "CHANGES_REQUESTED" {
			return "Changes requested"
		}
		return "Blocked"
	case "BEHIND":
		return "Behind base branch"
	case "UNSTABLE":
		return "Checks failing"
	case "DIRTY":
		return "Merge conflicts"
	default:
		return mergeState
	}
}
