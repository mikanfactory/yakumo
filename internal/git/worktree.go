package git

import (
	"strings"

	"worktree-ui/internal/model"
)

type worktreeEntry struct {
	Path   string
	Branch string
	IsBare bool
}

// ListWorktrees runs `git worktree list --porcelain` and parses the output.
func ListWorktrees(runner CommandRunner, repoPath string) ([]worktreeEntry, error) {
	out, err := runner.Run(repoPath, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(out) == "" {
		return nil, nil
	}

	return parseWorktreePorcelain(out), nil
}

func parseWorktreePorcelain(output string) []worktreeEntry {
	blocks := splitBlocks(output)
	entries := make([]worktreeEntry, 0, len(blocks))

	for _, block := range blocks {
		entry := parseBlock(block)
		if entry.Path != "" {
			entries = append(entries, entry)
		}
	}

	return entries
}

func splitBlocks(output string) []string {
	output = strings.TrimRight(output, "\n")
	if output == "" {
		return nil
	}

	var blocks []string
	var current []string

	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			if len(current) > 0 {
				blocks = append(blocks, strings.Join(current, "\n"))
				current = nil
			}
		} else {
			current = append(current, line)
		}
	}

	if len(current) > 0 {
		blocks = append(blocks, strings.Join(current, "\n"))
	}

	return blocks
}

func parseBlock(block string) worktreeEntry {
	var entry worktreeEntry

	for _, line := range strings.Split(block, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			entry.Path = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "branch "):
			ref := strings.TrimPrefix(line, "branch ")
			entry.Branch = strings.TrimPrefix(ref, "refs/heads/")
		case line == "detached":
			entry.Branch = "(detached)"
		case line == "bare":
			entry.IsBare = true
		}
	}

	return entry
}

// AddWorktree creates a new worktree with a new branch.
func AddWorktree(runner CommandRunner, repoPath, newPath, branch string) error {
	_, err := runner.Run(repoPath, "worktree", "add", newPath, "-b", branch)
	return err
}

// FetchBranch fetches a specific branch from origin.
func FetchBranch(runner CommandRunner, repoPath, branch string) error {
	_, err := runner.Run(repoPath, "fetch", "origin", branch)
	return err
}

// AddWorktreeFromBranch creates a new worktree from an existing branch.
func AddWorktreeFromBranch(runner CommandRunner, repoPath, newPath, branch string) error {
	_, err := runner.Run(repoPath, "worktree", "add", newPath, branch)
	return err
}

// RenameBranch renames a branch in the given worktree directory.
func RenameBranch(runner CommandRunner, worktreePath, oldBranch, newBranch string) error {
	_, err := runner.Run(worktreePath, "branch", "-m", oldBranch, newBranch)
	return err
}

// RemoveWorktree removes an existing worktree.
func RemoveWorktree(runner CommandRunner, repoPath, worktreePath string) error {
	_, err := runner.Run(repoPath, "worktree", "remove", worktreePath)
	return err
}

// ToWorktreeInfo converts parsed entries to model.WorktreeInfo slices.
func ToWorktreeInfo(entries []worktreeEntry) []model.WorktreeInfo {
	infos := make([]model.WorktreeInfo, len(entries))
	for i, e := range entries {
		infos[i] = model.WorktreeInfo{
			Path:   e.Path,
			Branch: e.Branch,
			IsBare: e.IsBare,
		}
	}
	return infos
}
