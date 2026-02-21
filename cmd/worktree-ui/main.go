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
		if _, err := tmux.SelectWorktreeSession(tmuxRunner, selected); err != nil {
			fmt.Fprintf(os.Stderr, "tmux error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	fmt.Print(selected)
}
