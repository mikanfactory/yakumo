package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureDirectoryTrusted_NewProject(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".claude.json")

	// Start with a config that has no projects
	initial := map[string]json.RawMessage{
		"numFreeTrialTurns": json.RawMessage(`5`),
	}
	data, _ := json.Marshal(initial)
	os.WriteFile(configPath, data, 0644)

	err := EnsureDirectoryTrusted(configPath, "/repos/my-worktree")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the file was updated correctly
	got := readConfig(t, configPath)
	projects := parseProjects(t, got)

	p, ok := projects["/repos/my-worktree"]
	if !ok {
		t.Fatal("expected project entry to exist")
	}
	if !p.HasTrustDialogAccepted {
		t.Error("expected hasTrustDialogAccepted to be true")
	}

	// Verify existing fields are preserved
	if _, ok := got["numFreeTrialTurns"]; !ok {
		t.Error("expected numFreeTrialTurns to be preserved")
	}
}

func TestEnsureDirectoryTrusted_AlreadyTrusted(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".claude.json")

	initial := map[string]json.RawMessage{
		"projects": json.RawMessage(`{"/repos/my-worktree":{"hasTrustDialogAccepted":true}}`),
	}
	data, _ := json.Marshal(initial)
	os.WriteFile(configPath, data, 0644)

	err := EnsureDirectoryTrusted(configPath, "/repos/my-worktree")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// File should remain unchanged (still trusted)
	got := readConfig(t, configPath)
	projects := parseProjects(t, got)
	if !projects["/repos/my-worktree"].HasTrustDialogAccepted {
		t.Error("expected hasTrustDialogAccepted to remain true")
	}
}

func TestEnsureDirectoryTrusted_FileNotExist(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".claude.json")

	err := EnsureDirectoryTrusted(configPath, "/repos/new-worktree")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := readConfig(t, configPath)
	projects := parseProjects(t, got)

	p, ok := projects["/repos/new-worktree"]
	if !ok {
		t.Fatal("expected project entry to exist")
	}
	if !p.HasTrustDialogAccepted {
		t.Error("expected hasTrustDialogAccepted to be true")
	}
}

func TestEnsureDirectoryTrusted_PreservesExistingProjects(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".claude.json")

	initial := map[string]json.RawMessage{
		"projects": json.RawMessage(`{"/repos/existing":{"hasTrustDialogAccepted":true}}`),
	}
	data, _ := json.Marshal(initial)
	os.WriteFile(configPath, data, 0644)

	err := EnsureDirectoryTrusted(configPath, "/repos/new-worktree")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := readConfig(t, configPath)
	projects := parseProjects(t, got)

	if !projects["/repos/existing"].HasTrustDialogAccepted {
		t.Error("expected existing project to be preserved")
	}
	if !projects["/repos/new-worktree"].HasTrustDialogAccepted {
		t.Error("expected new project to be added")
	}
}

func readConfig(t *testing.T, path string) map[string]json.RawMessage {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}
	var config map[string]json.RawMessage
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("parsing config: %v", err)
	}
	return config
}

func parseProjects(t *testing.T, config map[string]json.RawMessage) map[string]projectConfig {
	t.Helper()
	raw, ok := config["projects"]
	if !ok {
		t.Fatal("expected projects field")
	}
	var projects map[string]projectConfig
	if err := json.Unmarshal(raw, &projects); err != nil {
		t.Fatalf("parsing projects: %v", err)
	}
	return projects
}
