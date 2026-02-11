package git

import (
	"strings"

	"worktree-ui/internal/model"
)

// GetStatus runs `git status --porcelain` and returns aggregated file counts.
func GetStatus(runner CommandRunner, worktreePath string) (model.StatusInfo, error) {
	out, err := runner.Run(worktreePath, "status", "--porcelain")
	if err != nil {
		return model.StatusInfo{}, err
	}

	return parseStatusPorcelain(out), nil
}

func parseStatusPorcelain(output string) model.StatusInfo {
	if strings.TrimSpace(output) == "" {
		return model.StatusInfo{}
	}

	var info model.StatusInfo

	for _, line := range strings.Split(strings.TrimRight(output, "\n"), "\n") {
		if len(line) < 2 {
			continue
		}

		index := line[0]
		work := line[1]

		switch {
		case index == '?' && work == '?':
			info.Untracked++
		case index == 'A' || index == 'R':
			info.Added++
		case index == 'D' || work == 'D':
			info.Deleted++
		case index == 'M' || work == 'M':
			info.Modified++
		}
	}

	return info
}
