package tmux

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

const (
	archivingOptionPrefix = "@yakumo_archiving_"
	archivingHashLen      = 12
)

// archivingOptionName hashes the path because option names cannot hold
// arbitrary characters such as "/".
func archivingOptionName(worktreePath string) string {
	sum := sha1.Sum([]byte(worktreePath))
	return archivingOptionPrefix + hex.EncodeToString(sum[:])[:archivingHashLen]
}

// MarkArchiving records that the process pid is archiving worktreePath.
func MarkArchiving(runner Runner, worktreePath string, pid int) error {
	value := fmt.Sprintf("%d:%s", pid, worktreePath)
	if _, err := runner.Run("set-option", "-g", archivingOptionName(worktreePath), value); err != nil {
		return fmt.Errorf("marking archive of %s: %w", worktreePath, err)
	}
	return nil
}

// UnmarkArchiving removes the archive mark for worktreePath.
func UnmarkArchiving(runner Runner, worktreePath string) error {
	if _, err := runner.Run("set-option", "-gu", archivingOptionName(worktreePath)); err != nil {
		return fmt.Errorf("unmarking archive of %s: %w", worktreePath, err)
	}
	return nil
}

// ListArchiving returns the worktree paths currently being archived. Marks
// whose process is no longer alive are left behind by crashed archives, so
// they are removed and skipped.
func ListArchiving(runner Runner, alive func(pid int) bool) ([]string, error) {
	out, err := runner.Run("show-options", "-g")
	if err != nil {
		return nil, fmt.Errorf("listing archive marks: %w", err)
	}

	var paths []string
	for _, line := range strings.Split(out, "\n") {
		name, value, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok || !strings.HasPrefix(name, archivingOptionPrefix) {
			continue
		}
		pidStr, path, ok := strings.Cut(strings.Trim(value, `"`), ":")
		if !ok {
			continue
		}
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}
		if !alive(pid) {
			runner.Run("set-option", "-gu", name) // ignore error (best-effort cleanup)
			continue
		}
		paths = append(paths, path)
	}
	return paths, nil
}
