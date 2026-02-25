package git

import (
	"fmt"
	"testing"
)

func TestGetDiffNumstat(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   []DiffEntry
	}{
		{
			name:   "single file",
			output: "10\t3\tsrc/main.go\n",
			want:   []DiffEntry{{Path: "src/main.go", Additions: 10, Deletions: 3}},
		},
		{
			name:   "multiple files",
			output: "44\t4\tbackend/repo.py\n14\t20\tbackend/models.py\n",
			want: []DiffEntry{
				{Path: "backend/repo.py", Additions: 44, Deletions: 4},
				{Path: "backend/models.py", Additions: 14, Deletions: 20},
			},
		},
		{
			name:   "binary file",
			output: "-\t-\timage.png\n",
			want:   []DiffEntry{{Path: "image.png", Additions: 0, Deletions: 0}},
		},
		{
			name:   "empty output",
			output: "",
			want:   nil,
		},
		{
			name:   "additions only",
			output: "5\t0\tnew_file.go\n",
			want:   []DiffEntry{{Path: "new_file.go", Additions: 5, Deletions: 0}},
		},
		{
			name:   "deletions only",
			output: "0\t12\tremoved.go\n",
			want:   []DiffEntry{{Path: "removed.go", Additions: 0, Deletions: 12}},
		},
		{
			name:   "rename with arrow",
			output: "5\t2\told.go => new.go\n",
			want:   []DiffEntry{{Path: "old.go => new.go", Additions: 5, Deletions: 2}},
		},
		{
			name:   "whitespace lines ignored",
			output: "\n10\t3\tmain.go\n\n",
			want:   []DiffEntry{{Path: "main.go", Additions: 10, Deletions: 3}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := FakeCommandRunner{
				Outputs: map[string]string{
					"/repo:[diff origin/main...HEAD --numstat]": tt.output,
				},
			}

			got, err := GetDiffNumstat(runner, "/repo", "origin/main")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tt.want) {
				t.Fatalf("got %d entries, want %d", len(got), len(tt.want))
			}

			for i, g := range got {
				w := tt.want[i]
				if g.Path != w.Path || g.Additions != w.Additions || g.Deletions != w.Deletions {
					t.Errorf("entry[%d] = %+v, want %+v", i, g, w)
				}
			}
		})
	}
}

func TestGetDiffNumstat_Error(t *testing.T) {
	runner := FakeCommandRunner{
		Errors: map[string]error{
			"/repo:[diff origin/main...HEAD --numstat]": fmt.Errorf("not a git repo"),
		},
	}

	_, err := GetDiffNumstat(runner, "/repo", "origin/main")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetAllChanges(t *testing.T) {
	t.Run("committed only", func(t *testing.T) {
		runner := FakeCommandRunner{
			Outputs: map[string]string{
				"/repo:[diff origin/main...HEAD --numstat]": "10\t3\tmain.go\n",
				"/repo:[diff HEAD --numstat]":               "",
			},
		}

		got, err := GetAllChanges(runner, "/repo", "origin/main")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("got %d entries, want 1", len(got))
		}
		if got[0].Path != "main.go" || got[0].Additions != 10 || got[0].Deletions != 3 {
			t.Errorf("got %+v, want {main.go 10 3}", got[0])
		}
	})

	t.Run("uncommitted only", func(t *testing.T) {
		runner := FakeCommandRunner{
			Outputs: map[string]string{
				"/repo:[diff origin/main...HEAD --numstat]": "",
				"/repo:[diff HEAD --numstat]":               "5\t2\tnew_file.go\n",
			},
		}

		got, err := GetAllChanges(runner, "/repo", "origin/main")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("got %d entries, want 1", len(got))
		}
		if got[0].Path != "new_file.go" || got[0].Additions != 5 || got[0].Deletions != 2 {
			t.Errorf("got %+v, want {new_file.go 5 2}", got[0])
		}
	})

	t.Run("committed and uncommitted with overlap", func(t *testing.T) {
		runner := FakeCommandRunner{
			Outputs: map[string]string{
				"/repo:[diff origin/main...HEAD --numstat]": "10\t3\tmain.go\n5\t1\tutils.go\n",
				"/repo:[diff HEAD --numstat]":               "2\t1\tmain.go\n7\t0\tnew.go\n",
			},
		}

		got, err := GetAllChanges(runner, "/repo", "origin/main")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 3 {
			t.Fatalf("got %d entries, want 3", len(got))
		}

		// main.go should be merged: 10+2=12 additions, 3+1=4 deletions
		if got[0].Path != "main.go" || got[0].Additions != 12 || got[0].Deletions != 4 {
			t.Errorf("entry[0] = %+v, want {main.go 12 4}", got[0])
		}
		// utils.go committed only
		if got[1].Path != "utils.go" || got[1].Additions != 5 || got[1].Deletions != 1 {
			t.Errorf("entry[1] = %+v, want {utils.go 5 1}", got[1])
		}
		// new.go uncommitted only
		if got[2].Path != "new.go" || got[2].Additions != 7 || got[2].Deletions != 0 {
			t.Errorf("entry[2] = %+v, want {new.go 7 0}", got[2])
		}
	})

	t.Run("uncommitted error falls back to committed", func(t *testing.T) {
		runner := FakeCommandRunner{
			Outputs: map[string]string{
				"/repo:[diff origin/main...HEAD --numstat]": "10\t3\tmain.go\n",
			},
			Errors: map[string]error{
				"/repo:[diff HEAD --numstat]": fmt.Errorf("no HEAD"),
			},
		}

		got, err := GetAllChanges(runner, "/repo", "origin/main")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("got %d entries, want 1", len(got))
		}
		if got[0].Path != "main.go" {
			t.Errorf("got path %q, want main.go", got[0].Path)
		}
	})

	t.Run("committed error propagates", func(t *testing.T) {
		runner := FakeCommandRunner{
			Errors: map[string]error{
				"/repo:[diff origin/main...HEAD --numstat]": fmt.Errorf("not a git repo"),
			},
		}

		_, err := GetAllChanges(runner, "/repo", "origin/main")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestGetCommitsBehind(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   int
	}{
		{name: "zero", output: "0\n", want: 0},
		{name: "some commits", output: "17\n", want: 17},
		{name: "trailing whitespace", output: "  5  \n", want: 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := FakeCommandRunner{
				Outputs: map[string]string{
					"/repo:[rev-list --count HEAD..origin/main]": tt.output,
				},
			}

			got, err := GetCommitsBehind(runner, "/repo", "origin/main")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestGetCommitsBehind_Error(t *testing.T) {
	runner := FakeCommandRunner{
		Errors: map[string]error{
			"/repo:[rev-list --count HEAD..origin/main]": fmt.Errorf("failed"),
		},
	}

	_, err := GetCommitsBehind(runner, "/repo", "origin/main")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
