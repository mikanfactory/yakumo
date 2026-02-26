package pathcomplete

import (
	"os"
	"sort"
	"strings"
)

// DirLister reads directory entries for a given path.
type DirLister func(path string) ([]os.DirEntry, error)

// DefaultDirLister reads directory entries using os.ReadDir.
func DefaultDirLister(path string) ([]os.DirEntry, error) {
	return os.ReadDir(path)
}

// ListDirSuggestions returns directory path suggestions for the given input.
// It expands ~/ to homeDir, lists only directories, and returns at most maxResults.
// Suggestions include a trailing "/" so the user can continue typing the next segment.
// If the input starts with ~/, suggestions also use the ~/ prefix.
func ListDirSuggestions(input, homeDir string, lister DirLister, maxResults int) []string {
	if input == "" {
		return nil
	}

	useTilde := strings.HasPrefix(input, "~/")
	expanded := expandTilde(input, homeDir)

	// Determine the directory to read and the prefix to filter by.
	dir, prefix := splitDirPrefix(expanded)

	entries, err := lister(dir)
	if err != nil {
		return nil
	}

	// Ensure dir has trailing slash for path construction.
	dirSlash := dir
	if !strings.HasSuffix(dirSlash, "/") {
		dirSlash += "/"
	}

	var suggestions []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if prefix != "" && !strings.HasPrefix(name, prefix) {
			continue
		}

		fullPath := dirSlash + name + "/"
		if useTilde {
			homePrefix := homeDir
			if !strings.HasSuffix(homePrefix, "/") {
				homePrefix += "/"
			}
			fullPath = "~/" + strings.TrimPrefix(fullPath, homePrefix)
		}

		suggestions = append(suggestions, fullPath)
	}

	sort.Strings(suggestions)
	if len(suggestions) > maxResults {
		suggestions = suggestions[:maxResults]
	}

	return suggestions
}

// ExtractDir returns the directory portion of input for re-fetch detection.
// It expands ~/ to homeDir. If input ends with "/", it returns the expanded path
// with trailing "/". Otherwise it returns the parent directory with trailing "/".
// Returns "" for empty input.
func ExtractDir(input, homeDir string) string {
	if input == "" {
		return ""
	}

	expanded := expandTilde(input, homeDir)

	if strings.HasSuffix(expanded, "/") {
		return expanded
	}

	// Find last "/" to get parent directory.
	idx := strings.LastIndex(expanded, "/")
	if idx < 0 {
		return ""
	}
	return expanded[:idx+1]
}

// expandTilde replaces a leading ~/ with homeDir, preserving trailing slashes.
func expandTilde(input, homeDir string) string {
	if !strings.HasPrefix(input, "~/") {
		return input
	}
	rest := input[2:]
	if rest == "" {
		return homeDir + "/"
	}
	result := homeDir + "/" + rest
	return result
}

// splitDirPrefix splits expanded path into (directory to read, filename prefix).
// If the path ends with "/", prefix is empty.
func splitDirPrefix(expanded string) (dir, prefix string) {
	if strings.HasSuffix(expanded, "/") {
		dir = strings.TrimSuffix(expanded, "/")
		if dir == "" {
			dir = "/"
		}
		return dir, ""
	}

	idx := strings.LastIndex(expanded, "/")
	if idx < 0 {
		return ".", expanded
	}
	dir = expanded[:idx]
	if dir == "" {
		dir = "/"
	}
	prefix = expanded[idx+1:]
	return dir, prefix
}
