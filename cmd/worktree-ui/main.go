package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"worktree-ui/internal/config"
	"worktree-ui/internal/git"
	"worktree-ui/internal/tui"
)

func main() {
	configPath := flag.String("config", "", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	runner := git.OSCommandRunner{}
	m := tui.NewModel(cfg, runner)

	p := tea.NewProgram(m, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if finalModel, ok := result.(tui.Model); ok && finalModel.Selected() != "" {
		fmt.Print(finalModel.Selected())
	}
}
