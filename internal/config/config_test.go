package config

import (
	"os"
	"path/filepath"
	"testing"

	"worktree-ui/internal/model"
)

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := `sidebar_width: 35
repositories:
  - name: myrepo
    path: /home/user/myrepo
  - name: other
    path: /home/user/other
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromFile(cfgPath)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	if cfg.SidebarWidth != 35 {
		t.Errorf("SidebarWidth = %d, want 35", cfg.SidebarWidth)
	}
	if len(cfg.Repositories) != 2 {
		t.Fatalf("len(Repositories) = %d, want 2", len(cfg.Repositories))
	}
	if cfg.Repositories[0].Name != "myrepo" {
		t.Errorf("Repositories[0].Name = %q, want %q", cfg.Repositories[0].Name, "myrepo")
	}
	if cfg.Repositories[1].Path != "/home/user/other" {
		t.Errorf("Repositories[1].Path = %q, want %q", cfg.Repositories[1].Path, "/home/user/other")
	}
}

func TestLoadFromFile_DefaultSidebarWidth(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := `repositories:
  - name: repo1
    path: /tmp/repo1
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromFile(cfgPath)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	if cfg.SidebarWidth != DefaultSidebarWidth {
		t.Errorf("SidebarWidth = %d, want default %d", cfg.SidebarWidth, DefaultSidebarWidth)
	}
}

func TestLoadFromFile_NotFound(t *testing.T) {
	_, err := LoadFromFile("/nonexistent/config.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

func TestLoadFromFile_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	if err := os.WriteFile(cfgPath, []byte("{{invalid yaml"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFromFile(cfgPath)
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}

func TestLoadFromFile_EmptyRepositories(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := `sidebar_width: 30
repositories: []
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFromFile(cfgPath)
	if err == nil {
		t.Error("expected error for empty repositories, got nil")
	}
}

func TestResolveConfigPath(t *testing.T) {
	tests := []struct {
		name     string
		flagPath string
		wantErr  bool
	}{
		{
			name:     "explicit path exists",
			flagPath: "", // will be set in test body
			wantErr:  false,
		},
		{
			name:     "explicit path not exists",
			flagPath: "/nonexistent/path.yaml",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flagPath := tt.flagPath
			if tt.name == "explicit path exists" {
				dir := t.TempDir()
				flagPath = filepath.Join(dir, "config.yaml")
				if err := os.WriteFile(flagPath, []byte("repositories:\n  - name: x\n    path: /x"), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			result, err := ResolveConfigPath(flagPath)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.wantErr && result != flagPath {
				t.Errorf("result = %q, want %q", result, flagPath)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := `sidebar_width: 28
repositories:
  - name: testrepo
    path: /tmp/testrepo
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	want := model.Config{
		SidebarWidth: 28,
		Repositories: []model.RepositoryDef{
			{Name: "testrepo", Path: "/tmp/testrepo"},
		},
	}

	if cfg.SidebarWidth != want.SidebarWidth {
		t.Errorf("SidebarWidth = %d, want %d", cfg.SidebarWidth, want.SidebarWidth)
	}
	if len(cfg.Repositories) != len(want.Repositories) {
		t.Fatalf("len(Repositories) = %d, want %d", len(cfg.Repositories), len(want.Repositories))
	}
	if cfg.Repositories[0] != want.Repositories[0] {
		t.Errorf("Repositories[0] = %+v, want %+v", cfg.Repositories[0], want.Repositories[0])
	}
}
