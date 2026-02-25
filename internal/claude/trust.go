package claude

import (
	"encoding/json"
	"fmt"
	"os"
)

type projectConfig struct {
	HasTrustDialogAccepted bool `json:"hasTrustDialogAccepted"`
}

// EnsureDirectoryTrusted ensures the given directory is marked as trusted in
// the Claude CLI configuration file. This prevents the "Trust this directory?"
// prompt when launching claude in a new worktree.
func EnsureDirectoryTrusted(configPath string, dir string) error {
	config := make(map[string]json.RawMessage)

	data, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading config %s: %w", configPath, err)
	}
	if err == nil {
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("parsing config %s: %w", configPath, err)
		}
	}

	projects := make(map[string]projectConfig)
	if raw, ok := config["projects"]; ok {
		if err := json.Unmarshal(raw, &projects); err != nil {
			return fmt.Errorf("parsing projects field: %w", err)
		}
	}

	if p, ok := projects[dir]; ok && p.HasTrustDialogAccepted {
		return nil
	}

	projects[dir] = projectConfig{HasTrustDialogAccepted: true}

	projectsJSON, err := json.Marshal(projects)
	if err != nil {
		return fmt.Errorf("marshaling projects: %w", err)
	}
	config["projects"] = projectsJSON

	out, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(configPath, out, 0644); err != nil {
		return fmt.Errorf("writing config %s: %w", configPath, err)
	}

	return nil
}
