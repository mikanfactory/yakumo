package pathcomplete

import (
	"io/fs"
	"os"
	"testing"
	"time"
)

// fakeDirEntry implements fs.DirEntry for testing.
type fakeDirEntry struct {
	name  string
	isDir bool
}

func (f fakeDirEntry) Name() string               { return f.name }
func (f fakeDirEntry) IsDir() bool                 { return f.isDir }
func (f fakeDirEntry) Type() fs.FileMode           { return 0 }
func (f fakeDirEntry) Info() (fs.FileInfo, error)   { return fakeFileInfo{name: f.name, isDir: f.isDir}, nil }

type fakeFileInfo struct {
	name  string
	isDir bool
}

func (f fakeFileInfo) Name() string      { return f.name }
func (f fakeFileInfo) Size() int64       { return 0 }
func (f fakeFileInfo) Mode() fs.FileMode { return 0 }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool       { return f.isDir }
func (f fakeFileInfo) Sys() interface{}  { return nil }

func fakeLister(entries map[string][]os.DirEntry) DirLister {
	return func(path string) ([]os.DirEntry, error) {
		if e, ok := entries[path]; ok {
			return e, nil
		}
		return nil, os.ErrNotExist
	}
}

func TestListDirSuggestions_EmptyInput(t *testing.T) {
	lister := fakeLister(map[string][]os.DirEntry{})
	result := ListDirSuggestions("", "/home/user", lister, 10)
	if result != nil {
		t.Errorf("expected nil for empty input, got %v", result)
	}
}

func TestListDirSuggestions_RootSlash(t *testing.T) {
	lister := fakeLister(map[string][]os.DirEntry{
		"/": {
			fakeDirEntry{name: "usr", isDir: true},
			fakeDirEntry{name: "etc", isDir: true},
			fakeDirEntry{name: "file.txt", isDir: false},
		},
	})
	result := ListDirSuggestions("/", "/home/user", lister, 10)
	expected := []string{"/etc/", "/usr/"}
	if len(result) != len(expected) {
		t.Fatalf("expected %d results, got %d: %v", len(expected), len(result), result)
	}
	for i, v := range expected {
		if result[i] != v {
			t.Errorf("result[%d] = %q, want %q", i, result[i], v)
		}
	}
}

func TestListDirSuggestions_TildeExpansion(t *testing.T) {
	lister := fakeLister(map[string][]os.DirEntry{
		"/home/user": {
			fakeDirEntry{name: "Documents", isDir: true},
			fakeDirEntry{name: "Downloads", isDir: true},
			fakeDirEntry{name: ".bashrc", isDir: false},
		},
	})
	result := ListDirSuggestions("~/", "/home/user", lister, 10)
	expected := []string{"~/Documents/", "~/Downloads/"}
	if len(result) != len(expected) {
		t.Fatalf("expected %d results, got %d: %v", len(expected), len(result), result)
	}
	for i, v := range expected {
		if result[i] != v {
			t.Errorf("result[%d] = %q, want %q", i, result[i], v)
		}
	}
}

func TestListDirSuggestions_PartialMatch(t *testing.T) {
	lister := fakeLister(map[string][]os.DirEntry{
		"/usr": {
			fakeDirEntry{name: "local", isDir: true},
			fakeDirEntry{name: "lib", isDir: true},
			fakeDirEntry{name: "bin", isDir: true},
		},
	})
	result := ListDirSuggestions("/usr/lo", "/home/user", lister, 10)
	expected := []string{"/usr/local/"}
	if len(result) != len(expected) {
		t.Fatalf("expected %d results, got %d: %v", len(expected), len(result), result)
	}
	if result[0] != expected[0] {
		t.Errorf("result[0] = %q, want %q", result[0], expected[0])
	}
}

func TestListDirSuggestions_NonexistentPath(t *testing.T) {
	lister := fakeLister(map[string][]os.DirEntry{})
	result := ListDirSuggestions("/nonexistent/path", "/home/user", lister, 10)
	if len(result) != 0 {
		t.Errorf("expected empty for nonexistent path, got %v", result)
	}
}

func TestListDirSuggestions_FilesExcluded(t *testing.T) {
	lister := fakeLister(map[string][]os.DirEntry{
		"/data": {
			fakeDirEntry{name: "docs", isDir: true},
			fakeDirEntry{name: "readme.md", isDir: false},
			fakeDirEntry{name: "config.yaml", isDir: false},
		},
	})
	result := ListDirSuggestions("/data/", "/home/user", lister, 10)
	if len(result) != 1 {
		t.Fatalf("expected 1 result (dirs only), got %d: %v", len(result), result)
	}
	if result[0] != "/data/docs/" {
		t.Errorf("result[0] = %q, want %q", result[0], "/data/docs/")
	}
}

func TestListDirSuggestions_MaxResults(t *testing.T) {
	lister := fakeLister(map[string][]os.DirEntry{
		"/many": {
			fakeDirEntry{name: "a", isDir: true},
			fakeDirEntry{name: "b", isDir: true},
			fakeDirEntry{name: "c", isDir: true},
			fakeDirEntry{name: "d", isDir: true},
			fakeDirEntry{name: "e", isDir: true},
		},
	})
	result := ListDirSuggestions("/many/", "/home/user", lister, 3)
	if len(result) != 3 {
		t.Errorf("expected 3 results (maxResults), got %d: %v", len(result), result)
	}
}

func TestListDirSuggestions_TildePrefixPreserved(t *testing.T) {
	lister := fakeLister(map[string][]os.DirEntry{
		"/home/user/projects": {
			fakeDirEntry{name: "myapp", isDir: true},
		},
	})
	result := ListDirSuggestions("~/projects/", "/home/user", lister, 10)
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(result), result)
	}
	if result[0] != "~/projects/myapp/" {
		t.Errorf("result[0] = %q, want %q", result[0], "~/projects/myapp/")
	}
}

func TestListDirSuggestions_TildePartialMatch(t *testing.T) {
	lister := fakeLister(map[string][]os.DirEntry{
		"/home/user": {
			fakeDirEntry{name: "Documents", isDir: true},
			fakeDirEntry{name: "Downloads", isDir: true},
			fakeDirEntry{name: "Desktop", isDir: true},
		},
	})
	result := ListDirSuggestions("~/Do", "/home/user", lister, 10)
	expected := []string{"~/Documents/", "~/Downloads/"}
	if len(result) != len(expected) {
		t.Fatalf("expected %d results, got %d: %v", len(expected), len(result), result)
	}
	for i, v := range expected {
		if result[i] != v {
			t.Errorf("result[%d] = %q, want %q", i, result[i], v)
		}
	}
}

func TestExtractDir_Empty(t *testing.T) {
	result := ExtractDir("", "/home/user")
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestExtractDir_RootSlash(t *testing.T) {
	result := ExtractDir("/", "/home/user")
	if result != "/" {
		t.Errorf("expected %q, got %q", "/", result)
	}
}

func TestExtractDir_TrailingSlash(t *testing.T) {
	result := ExtractDir("/usr/local/", "/home/user")
	if result != "/usr/local/" {
		t.Errorf("expected %q, got %q", "/usr/local/", result)
	}
}

func TestExtractDir_PartialInput(t *testing.T) {
	result := ExtractDir("/usr/lo", "/home/user")
	if result != "/usr/" {
		t.Errorf("expected %q, got %q", "/usr/", result)
	}
}

func TestExtractDir_TildeExpansion(t *testing.T) {
	result := ExtractDir("~/projects/my", "/home/user")
	if result != "/home/user/projects/" {
		t.Errorf("expected %q, got %q", "/home/user/projects/", result)
	}
}

func TestExtractDir_TildeOnly(t *testing.T) {
	result := ExtractDir("~/", "/home/user")
	if result != "/home/user/" {
		t.Errorf("expected %q, got %q", "/home/user/", result)
	}
}
