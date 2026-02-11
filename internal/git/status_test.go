package git

import (
	"fmt"
	"testing"

	"worktree-ui/internal/model"
)

func TestGetStatus(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   model.StatusInfo
	}{
		{
			name:   "clean repo",
			output: "",
			want:   model.StatusInfo{},
		},
		{
			name:   "modified files",
			output: " M file1.go\n M file2.go\n M file3.go\n",
			want:   model.StatusInfo{Modified: 3},
		},
		{
			name:   "added files (staged)",
			output: "A  newfile.go\nA  another.go\n",
			want:   model.StatusInfo{Added: 2},
		},
		{
			name:   "deleted files",
			output: " D old.go\nD  removed.go\n",
			want:   model.StatusInfo{Deleted: 2},
		},
		{
			name:   "untracked files",
			output: "?? newfile.txt\n?? another.txt\n",
			want:   model.StatusInfo{Untracked: 2},
		},
		{
			name:   "mixed changes",
			output: " M modified.go\nA  added.go\n D deleted.go\n?? untracked.txt\nMM both.go\n",
			want:   model.StatusInfo{Modified: 2, Added: 1, Deleted: 1, Untracked: 1},
		},
		{
			name:   "renamed file",
			output: "R  old.go -> new.go\n",
			want:   model.StatusInfo{Added: 1},
		},
		{
			name:   "staged modified",
			output: "M  staged.go\n",
			want:   model.StatusInfo{Modified: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := FakeCommandRunner{
				Outputs: map[string]string{
					"/repo:[status --porcelain]": tt.output,
				},
			}

			got, err := GetStatus(runner, "/repo")
			if err != nil {
				t.Fatalf("GetStatus failed: %v", err)
			}

			if got != tt.want {
				t.Errorf("GetStatus = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestGetStatus_Error(t *testing.T) {
	runner := FakeCommandRunner{
		Errors: map[string]error{
			"/repo:[status --porcelain]": fmt.Errorf("not a git repo"),
		},
	}

	_, err := GetStatus(runner, "/repo")
	if err == nil {
		t.Error("expected error, got nil")
	}
}
