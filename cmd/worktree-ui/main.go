package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"worktree-ui/internal/branchname"
	"worktree-ui/internal/claude"
	"worktree-ui/internal/config"
	"worktree-ui/internal/git"
	"worktree-ui/internal/model"
	"worktree-ui/internal/tmux"
	"worktree-ui/internal/tui"
)

func main() {
	zone.NewGlobal()

	configPath := flag.String("config", "", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	resolvedConfigPath, err := config.ResolveConfigPath(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	runner := git.OSCommandRunner{}

	var tmuxRunner tmux.Runner
	if tmux.IsInsideTmux() {
		tmuxRunner = tmux.OSRunner{}
	}

	var claudeReader claude.Reader
	var branchNameGen branchname.Generator

	if claudePath, err := exec.LookPath("claude"); err == nil {
		if home, err := os.UserHomeDir(); err == nil {
			claudeReader = claude.OSReader{
				HistoryPath: filepath.Join(home, ".claude", "history.jsonl"),
			}
			branchNameGen = branchname.CLIGenerator{
				ClaudePath: claudePath,
			}
		}
	}

	m := tui.NewModel(cfg, runner, resolvedConfigPath, tmuxRunner, claudeReader, branchNameGen)

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	result, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	finalModel, ok := result.(tui.Model)
	if !ok || finalModel.Selected() == "" {
		return
	}

	selected := finalModel.Selected()

	if tmux.IsInsideTmux() {
		tmuxRunner := tmux.OSRunner{}
		layout, err := tmux.SelectWorktreeSession(tmuxRunner, selected)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tmux error: %v\n", err)
			os.Exit(1)
		}

		// Run startup commands only for newly created sessions
		if layout.BottomRight1.PaneID != "" {
			repo := findRepoByPath(cfg, finalModel.SelectedRepoPath())
			if repo.StartupCommand != "" {
				if err := tmux.SendKeys(tmuxRunner, layout.BottomRight1.PaneID, repo.StartupCommand); err != nil {
					fmt.Fprintf(os.Stderr, "startup command error: %v\n", err)
				}
			}

			// Launch diff-ui in top-right pane
			if diffUIPath := findDiffUI(); diffUIPath != "" {
				if err := tmux.SendKeys(tmuxRunner, layout.TopRight1.PaneID, diffUIPath); err != nil {
					fmt.Fprintf(os.Stderr, "diff-ui launch error: %v\n", err)
				}
			} else {
				fmt.Fprintf(os.Stderr, "warning: diff-ui not found (searched executable directory and PATH)\n")
			}
		}
		return
	}

	fmt.Print(selected)
}

func findDiffUI() string {
	// Look for diff-ui in the same directory as the current executable
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "diff-ui")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	// Fall back to PATH lookup
	if p, err := exec.LookPath("diff-ui"); err == nil {
		return p
	}
	return ""
}

func findRepoByPath(cfg model.Config, repoPath string) model.RepositoryDef {
	for _, repo := range cfg.Repositories {
		if repo.Path == repoPath {
			return repo
		}
	}
	return model.RepositoryDef{}
}
