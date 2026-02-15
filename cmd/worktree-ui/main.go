package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

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

	runner := git.OSCommandRunner{}
	m := tui.NewModel(cfg, runner)

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
