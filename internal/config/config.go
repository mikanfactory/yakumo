package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"worktree-ui/internal/model"
)

const DefaultSidebarWidth = 30

// LoadFromFile reads and parses a YAML config file.
func LoadFromFile(path string) (model.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return model.Config{}, fmt.Errorf("reading config file: %w", err)
	}

	var cfg model.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return model.Config{}, fmt.Errorf("parsing config file: %w", err)
	}

	if cfg.SidebarWidth == 0 {
		cfg.SidebarWidth = DefaultSidebarWidth
	}

	if len(cfg.Repositories) == 0 {
		return model.Config{}, fmt.Errorf("config must have at least one repository")
	}

	return cfg, nil
}

// ResolveConfigPath determines the config file path from flag or default location.
func ResolveConfigPath(flagPath string) (string, error) {
	if flagPath != "" {
		if _, err := os.Stat(flagPath); err != nil {
			return "", fmt.Errorf("config file not found: %s", flagPath)
		}
		return flagPath, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}

	defaultPath := filepath.Join(home, ".config", "denpasar", "config.yaml")
	if _, err := os.Stat(defaultPath); err != nil {
		return "", fmt.Errorf("default config not found at %s: create it or use --config flag", defaultPath)
	}

	return defaultPath, nil
}

// Load resolves the config path and loads the config.
func Load(flagPath string) (model.Config, error) {
	path, err := ResolveConfigPath(flagPath)
	if err != nil {
		return model.Config{}, err
	}
	return LoadFromFile(path)
}
