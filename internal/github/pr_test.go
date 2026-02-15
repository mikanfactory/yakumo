package github

import (
	"fmt"
	"testing"
	"time"
)

func TestFetchPR(t *testing.T) {
	jsonOutput := `{
		"title": "feat: add auth flow",
		"body": "## What\nAdds authentication",
		"state": "OPEN",
		"mergeStateStatus": "CLEAN",
		"reviewDecision": "APPROVED",
		"statusCheckRollup": [
			{
				"name": "CI",
				"conclusion": "SUCCESS",
				"startedAt": "2025-01-01T00:00:00Z",
				"completedAt": "2025-01-01T00:03:00Z"
			}
		],
		"comments": [
			{
				"author": {"login": "reviewer"},
				"body": "LGTM, looks good to merge",
				"createdAt": "2025-01-01T01:00:00Z"
			}
		]
	}`

	runner := &FakeRunner{
		Outputs: map[string]string{
			fmt.Sprintf("/repo:[pr view --json %s]", prViewFields): jsonOutput,
		},
	}

	pr, err := FetchPR(runner, "/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pr.Title != "feat: add auth flow" {
		t.Errorf("title = %q, want %q", pr.Title, "feat: add auth flow")
	}
	if pr.State != "OPEN" {
		t.Errorf("state = %q, want %q", pr.State, "OPEN")
	}
	if pr.MergeStateStatus != "CLEAN" {
		t.Errorf("mergeStateStatus = %q, want %q", pr.MergeStateStatus, "CLEAN")
	}
	if len(pr.StatusCheckRollup) != 1 {
		t.Fatalf("expected 1 check, got %d", len(pr.StatusCheckRollup))
	}
	if pr.StatusCheckRollup[0].Name != "CI" {
		t.Errorf("check name = %q, want %q", pr.StatusCheckRollup[0].Name, "CI")
	}
	if len(pr.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(pr.Comments))
	}
	if pr.Comments[0].Author.Login != "reviewer" {
		t.Errorf("comment author = %q, want %q", pr.Comments[0].Author.Login, "reviewer")
	}
}

func TestFetchPR_NoPR(t *testing.T) {
	runner := &FakeRunner{
		Errors: map[string]error{
			fmt.Sprintf("/repo:[pr view --json %s]", prViewFields): fmt.Errorf("no pull requests found"),
		},
	}

	_, err := FetchPR(runner, "/repo")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFetchPR_InvalidJSON(t *testing.T) {
	runner := &FakeRunner{
		Outputs: map[string]string{
			fmt.Sprintf("/repo:[pr view --json %s]", prViewFields): "not json",
		},
	}

	_, err := FetchPR(runner, "/repo")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestStatusCheckNode_CheckName(t *testing.T) {
	tests := []struct {
		name    string
		node    StatusCheckNode
		want    string
	}{
		{name: "with name", node: StatusCheckNode{Name: "CI", Context: "ci/build"}, want: "CI"},
		{name: "no name uses context", node: StatusCheckNode{Context: "ci/build"}, want: "ci/build"},
		{name: "both empty", node: StatusCheckNode{}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.node.CheckName(); got != tt.want {
				t.Errorf("CheckName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStatusCheckNode_Passed(t *testing.T) {
	tests := []struct {
		name string
		node StatusCheckNode
		want bool
	}{
		{name: "conclusion SUCCESS", node: StatusCheckNode{Conclusion: "SUCCESS"}, want: true},
		{name: "state SUCCESS", node: StatusCheckNode{State: "SUCCESS"}, want: true},
		{name: "conclusion FAILURE", node: StatusCheckNode{Conclusion: "FAILURE"}, want: false},
		{name: "pending", node: StatusCheckNode{State: "PENDING"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.node.Passed(); got != tt.want {
				t.Errorf("Passed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStatusCheckNode_DurationString(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		node StatusCheckNode
		want string
	}{
		{
			name: "3 minutes",
			node: StatusCheckNode{StartedAt: base, CompletedAt: base.Add(3 * time.Minute)},
			want: "3m",
		},
		{
			name: "40 seconds",
			node: StatusCheckNode{StartedAt: base, CompletedAt: base.Add(40 * time.Second)},
			want: "40s",
		},
		{
			name: "zero times",
			node: StatusCheckNode{},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.node.DurationString(); got != tt.want {
				t.Errorf("DurationString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCommentNode_Preview(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		maxLen int
		want   string
	}{
		{name: "short body", body: "LGTM", maxLen: 80, want: "LGTM"},
		{name: "long body", body: "This is a very long review comment that goes on and on", maxLen: 20, want: "This is a very long ..."},
		{name: "multiline", body: "Line 1\nLine 2\nLine 3", maxLen: 80, want: "Line 1 Line 2 Line 3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := CommentNode{Body: tt.body}
			if got := c.Preview(tt.maxLen); got != tt.want {
				t.Errorf("Preview() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMapMergeStateStatus(t *testing.T) {
	tests := []struct {
		mergeState     string
		reviewDecision string
		want           string
	}{
		{"CLEAN", "", "Ready to merge"},
		{"BLOCKED", "CHANGES_REQUESTED", "Changes requested"},
		{"BLOCKED", "", "Blocked"},
		{"BEHIND", "", "Behind base branch"},
		{"UNSTABLE", "", "Checks failing"},
		{"DIRTY", "", "Merge conflicts"},
		{"UNKNOWN", "", "UNKNOWN"},
	}
	for _, tt := range tests {
		t.Run(tt.mergeState, func(t *testing.T) {
			if got := MapMergeStateStatus(tt.mergeState, tt.reviewDecision); got != tt.want {
				t.Errorf("MapMergeStateStatus(%q, %q) = %q, want %q",
					tt.mergeState, tt.reviewDecision, got, tt.want)
			}
		})
	}
}
