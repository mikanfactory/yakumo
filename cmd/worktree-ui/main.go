package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

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

	m := tui.NewModel(cfg, runner, resolvedConfigPath, tmuxRunner)

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

		// Run startup command only for newly created sessions
		if layout.BottomRight1.PaneID != "" {
			repo := findRepoByPath(cfg, finalModel.SelectedRepoPath())
			if repo.StartupCommand != "" {
				if err := tmux.SendKeys(tmuxRunner, layout.BottomRight1.PaneID, repo.StartupCommand); err != nil {
					fmt.Fprintf(os.Stderr, "startup command error: %v\n", err)
				}
			}
		}
		return
	}

	fmt.Print(selected)
}

func findRepoByPath(cfg model.Config, repoPath string) model.RepositoryDef {
	for _, repo := range cfg.Repositories {
		if repo.Path == repoPath {
			return repo
		}
	}
	return model.RepositoryDef{}
}
