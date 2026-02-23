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
	"worktree-ui/internal/diffui"
	"worktree-ui/internal/git"
	"worktree-ui/internal/github"
	"worktree-ui/internal/model"
	"worktree-ui/internal/tmux"
	"worktree-ui/internal/tui"
)

func main() {
	diffMode := flag.Bool("diff", false, "launch diff/PR review UI")
	configPath := flag.String("config", "", "path to config file")
	flag.Parse()

	if *diffMode {
		runDiffUI()
	} else {
		runWorktreeUI(*configPath)
	}
}

func runDiffUI() {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	gitRunner := git.OSCommandRunner{}
	ghRunner := github.OSRunner{}

	p := tea.NewProgram(
		diffui.NewModel(dir, gitRunner, ghRunner),
		tea.WithAltScreen(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runWorktreeUI(configPath string) {
	zone.NewGlobal()

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	resolvedConfigPath, err := config.ResolveConfigPath(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	runner := git.OSCommandRunner{}

	var tmuxRunner tmux.Runner
	if tmux.IsInsideTmux() {
		tmuxRunner = tmux.OSRunner{}
	}

	var ghRunner github.Runner
	if _, err := exec.LookPath("gh"); err == nil {
		ghRunner = github.OSRunner{}
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

	m := tui.NewModel(cfg, runner, resolvedConfigPath, tmuxRunner, ghRunner, claudeReader, branchNameGen)

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
			if diffCmd := diffUICommand(); diffCmd != "" {
				if err := tmux.SendKeys(tmuxRunner, layout.TopRight1.PaneID, diffCmd); err != nil {
					fmt.Fprintf(os.Stderr, "diff-ui launch error: %v\n", err)
				}
			}
		}
		return
	}

	fmt.Print(selected)
}

func diffUICommand() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return exe + " --diff"
}

func findRepoByPath(cfg model.Config, repoPath string) model.RepositoryDef {
	for _, repo := range cfg.Repositories {
		if repo.Path == repoPath {
			return repo
		}
	}
	return model.RepositoryDef{}
}
