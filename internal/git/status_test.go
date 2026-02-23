package git

import (
	"fmt"
	"testing"

	"github.com/mikanfactory/shiki/internal/model"
)

func TestGetBranchDiffStat(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   model.StatusInfo
	}{
		{
			name:   "clean branch",
			output: "",
			want:   model.StatusInfo{},
		},
		{
			name:   "single file",
			output: "10\t3\tmain.go\n",
			want:   model.StatusInfo{Insertions: 10, Deletions: 3},
		},
		{
			name:   "multiple files aggregated",
			output: "44\t4\trepo.go\n14\t20\tmodels.go\n",
			want:   model.StatusInfo{Insertions: 58, Deletions: 24},
		},
		{
			name:   "binary file counted as zero",
			output: "-\t-\timage.png\n5\t2\tmain.go\n",
			want:   model.StatusInfo{Insertions: 5, Deletions: 2},
		},
		{
			name:   "additions only",
			output: "100\t0\tnew.go\n",
			want:   model.StatusInfo{Insertions: 100, Deletions: 0},
		},
		{
			name:   "deletions only",
			output: "0\t50\told.go\n",
			want:   model.StatusInfo{Insertions: 0, Deletions: 50},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := FakeCommandRunner{
				Outputs: map[string]string{
					"/repo:[diff origin/main...HEAD --numstat]": tt.output,
				},
			}

			got, err := GetBranchDiffStat(runner, "/repo")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("GetBranchDiffStat = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestGetBranchDiffStat_ErrorReturnsEmpty(t *testing.T) {
	runner := FakeCommandRunner{
		Errors: map[string]error{
			"/repo:[diff origin/main...HEAD --numstat]": fmt.Errorf("origin/main not found"),
		},
	}

	got, err := GetBranchDiffStat(runner, "/repo")
	if err != nil {
		t.Fatalf("should not return error, got: %v", err)
	}

	want := model.StatusInfo{}
	if got != want {
		t.Errorf("GetBranchDiffStat = %+v, want empty %+v", got, want)
	}
}
