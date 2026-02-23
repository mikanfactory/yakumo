package git

import (
	"github.com/mikanfactory/shiki/internal/model"
)

const defaultBase = "origin/main"

// GetBranchDiffStat runs `git diff origin/main...HEAD --numstat` and returns
// aggregated line insertion/deletion counts for the branch.
func GetBranchDiffStat(runner CommandRunner, worktreePath string) (model.StatusInfo, error) {
	entries, err := GetDiffNumstat(runner, worktreePath, defaultBase)
	if err != nil {
		return model.StatusInfo{}, nil
	}

	var info model.StatusInfo
	for _, e := range entries {
		info.Insertions += e.Additions
		info.Deletions += e.Deletions
	}
	return info, nil
}
