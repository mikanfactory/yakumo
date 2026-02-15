package git

import (
	"strconv"
	"strings"
)

// DiffEntry represents a single file's diff statistics.
type DiffEntry struct {
	Path      string
	Additions int
	Deletions int
}

// GetDiffNumstat runs `git diff <base>...HEAD --numstat` and returns parsed entries.
func GetDiffNumstat(runner CommandRunner, dir string, base string) ([]DiffEntry, error) {
	out, err := runner.Run(dir, "diff", base+"...HEAD", "--numstat")
	if err != nil {
		return nil, err
	}
	return parseDiffNumstat(out), nil
}

// parseDiffNumstat parses the output of `git diff --numstat`.
// Format: "<additions>\t<deletions>\t<path>" per line.
// Binary files show "-\t-\t<path>".
func parseDiffNumstat(output string) []DiffEntry {
	var entries []DiffEntry
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}

		additions, errA := strconv.Atoi(parts[0])
		deletions, errD := strconv.Atoi(parts[1])
		if errA != nil || errD != nil {
			// Binary files show "-" for additions/deletions
			additions = 0
			deletions = 0
		}

		entries = append(entries, DiffEntry{
			Path:      parts[2],
			Additions: additions,
			Deletions: deletions,
		})
	}
	return entries
}

// GetCommitsBehind returns how many commits HEAD is behind the given base ref.
func GetCommitsBehind(runner CommandRunner, dir string, base string) (int, error) {
	out, err := runner.Run(dir, "rev-list", "--count", "HEAD.."+base)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0, err
	}
	return n, nil
}
