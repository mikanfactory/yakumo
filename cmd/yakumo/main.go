package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/mikanfactory/yakumo/internal/claude"
	"github.com/mikanfactory/yakumo/internal/config"
	"github.com/mikanfactory/yakumo/internal/diffui"
	"github.com/mikanfactory/yakumo/internal/git"
	"github.com/mikanfactory/yakumo/internal/github"
	"github.com/mikanfactory/yakumo/internal/model"
	"github.com/mikanfactory/yakumo/internal/sessionsetup"
	"github.com/mikanfactory/yakumo/internal/tmux"
	"github.com/mikanfactory/yakumo/internal/tui"
)

const usage = `Usage: yakumo [command]

Commands:
  (default)         Launch worktree UI
  diff-ui           Launch diff/PR review UI
  swap-center       Swap center pane with background
  swap-right-below  Swap right-below pane with background

Flags (worktree UI only):
  --config <path>   Path to config file
`

func main() {
	if len(os.Args) < 2 {
		runWorktreeUI("")
		return
	}

	switch os.Args[1] {
	case "diff-ui":
		runDiffUI()
	case "swap-center":
		runSwapCenter()
	case "swap-right-below":
		runSwapRightBelow()
	case "--diff":
		fmt.Fprintln(os.Stderr, "Warning: --diff is deprecated, use 'yakumo diff-ui' instead")
		runDiffUI()
	case "--help", "-h", "help":
		fmt.Print(usage)
	default:
		fs := flag.NewFlagSet("yakumo", flag.ExitOnError)
		fs.Usage = func() { fmt.Print(usage) }
		configPath := fs.String("config", "", "path to config file")
		fs.Parse(os.Args[1:])
		runWorktreeUI(*configPath)
	}
}

func runDiffUI() {
	zone.NewGlobal()

	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	gitRunner := git.OSCommandRunner{}
	if _, err := exec.LookPath("gh"); err != nil {
		fmt.Fprintln(os.Stderr, "error: gh CLI is required for diff-ui")
		os.Exit(1)
	}
	ghRunner := github.OSRunner{}

	baseRef := resolveBaseRef()
	p := tea.NewProgram(
		diffui.NewModel(dir, gitRunner, ghRunner, baseRef),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func setupDebugLog() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	logPath := filepath.Join(home, ".config", "yakumo", "debug.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	log.SetOutput(f)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
}

func runWorktreeUI(configPath string) {
	setupDebugLog()
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
		if err := tmux.EnsureMainSession(tmuxRunner); err != nil {
			log.Printf("[main] EnsureMainSession failed (non-fatal): %v", err)
		}
	}

	var ghRunner github.Runner
	if _, err := exec.LookPath("gh"); err == nil {
		ghRunner = github.OSRunner{}
	}

	m := tui.NewModel(cfg, runner, resolvedConfigPath, tmuxRunner, ghRunner)
	if tmuxRunner != nil {
		m = m.WithSessionSetup(newSessionSetup(cfg, tmuxRunner, runner))
	}

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	result, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	finalModel, ok := result.(tui.Model)
	if !ok || finalModel.Selected() == "" || tmuxRunner != nil {
		return
	}

	fmt.Print(finalModel.Selected())
}

func newSessionSetup(cfg model.Config, tmuxRunner tmux.Runner, gitRunner git.CommandRunner) tui.SessionSetupFunc {
	claudeCommand := ""
	if _, err := exec.LookPath("claude"); err == nil {
		claudeCommand = "claude"
	}

	return func(worktreePath, repoRootPath string) error {
		return sessionsetup.Setup(sessionsetup.Options{
			TmuxRunner:     tmuxRunner,
			GitRunner:      gitRunner,
			WorktreePath:   worktreePath,
			StartupCommand: findRepoByPath(cfg, repoRootPath).StartupCommand,
			DiffUICommand:  diffUICommand(),
			ClaudeCommand:  claudeCommand,
			TrustDirectory: trustClaudeDirectory,
		})
	}
}

func trustClaudeDirectory(worktreePath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	return claude.EnsureDirectoryTrusted(filepath.Join(home, ".claude.json"), worktreePath)
}

func runSwapCenter() {
	if !tmux.IsInsideTmux() {
		fmt.Fprintln(os.Stderr, "error: swap-center requires running inside tmux")
		os.Exit(1)
	}
	runner := tmux.OSRunner{}
	if err := tmux.SwapCenter(runner); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runSwapRightBelow() {
	if !tmux.IsInsideTmux() {
		fmt.Fprintln(os.Stderr, "error: swap-right-below requires running inside tmux")
		os.Exit(1)
	}
	runner := tmux.OSRunner{}
	if err := tmux.SwapRightBelow(runner); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func diffUICommand() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return exe + " diff-ui"
}

func resolveBaseRef() string {
	baseRef := config.DefaultBaseRef
	path, err := config.ResolveConfigPath("")
	if err != nil {
		return baseRef
	}
	cfg, err := config.LoadFromFile(path)
	if err != nil {
		return baseRef
	}
	if cfg.DefaultBaseRef != "" {
		baseRef = cfg.DefaultBaseRef
	}
	return baseRef
}

func findRepoByPath(cfg model.Config, repoPath string) model.RepositoryDef {
	for _, repo := range cfg.Repositories {
		if repo.Path == repoPath {
			return repo
		}
	}
	return model.RepositoryDef{}
}
