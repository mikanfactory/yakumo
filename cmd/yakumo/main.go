package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/mikanfactory/yakumo/internal/branchname"
	"github.com/mikanfactory/yakumo/internal/claude"
	"github.com/mikanfactory/yakumo/internal/config"
	"github.com/mikanfactory/yakumo/internal/diffui"
	"github.com/mikanfactory/yakumo/internal/git"
	"github.com/mikanfactory/yakumo/internal/github"
	"github.com/mikanfactory/yakumo/internal/model"
	"github.com/mikanfactory/yakumo/internal/rename"
	"github.com/mikanfactory/yakumo/internal/tmux"
	"github.com/mikanfactory/yakumo/internal/tui"
)

const usage = `Usage: yakumo [command]

Commands:
  (default)         Launch worktree UI
  diff-ui           Launch diff/PR review UI
  swap-center       Swap center pane with background
  swap-right-below  Swap right-below pane with background
  watch-rename      Watch for Claude prompt and rename branch (internal)

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
	case "watch-rename":
		runWatchRename()
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
		repo := findRepoByPath(cfg, finalModel.SelectedRepoPath())
		layout, err := tmux.SelectWorktreeSession(tmuxRunner, selected, repo.StartupCommand)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tmux error: %v\n", err)
			os.Exit(1)
		}

		// Run additional commands only for newly created sessions
		if layout.BottomRight1.PaneID != "" {
			// Launch diff-ui in top-right pane
			if diffCmd := diffUICommand(); diffCmd != "" {
				if err := tmux.SendKeys(tmuxRunner, layout.TopRight1.PaneID, diffCmd); err != nil {
					fmt.Fprintf(os.Stderr, "diff-ui launch error: %v\n", err)
				}
			}

			// Ensure claude trust and launch claude CLI in center pane
			if _, err := exec.LookPath("claude"); err == nil {
				if home, err := os.UserHomeDir(); err == nil {
					configPath := filepath.Join(home, ".claude.json")
					if trustErr := claude.EnsureDirectoryTrusted(configPath, selected); trustErr != nil {
						fmt.Fprintf(os.Stderr, "claude trust warning: %v\n", trustErr)
					}
				}
				if err := tmux.SendKeys(tmuxRunner, layout.Center1.PaneID, "claude"); err != nil {
					fmt.Fprintf(os.Stderr, "claude launch error: %v\n", err)
				}
			}

			// Focus center pane after all commands are sent
			if err := tmux.SelectPane(tmuxRunner, layout.Center1.PaneID); err != nil {
				fmt.Fprintf(os.Stderr, "select pane error: %v\n", err)
			}
		}

		// Spawn background rename watcher if there is a pending rename
		if renameInfo := finalModel.PendingRename(selected); renameInfo != nil {
			spawnRenameWatcher(selected, renameInfo.OriginalBranch, renameInfo.CreatedAt)
		}

		return
	}

	fmt.Print(selected)
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

func runWatchRename() {
	fs := flag.NewFlagSet("watch-rename", flag.ExitOnError)
	wtPath := fs.String("path", "", "absolute path to the worktree")
	branch := fs.String("branch", "", "original branch name")
	createdAtStr := fs.String("created-at", "", "unix millisecond timestamp")
	fs.Parse(os.Args[2:])

	if *wtPath == "" || *branch == "" || *createdAtStr == "" {
		fmt.Fprintln(os.Stderr, "watch-rename requires --path, --branch, and --created-at flags")
		os.Exit(1)
	}

	createdAt, err := strconv.ParseInt(*createdAtStr, 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid --created-at: %v\n", err)
		os.Exit(1)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		os.Exit(1)
	}

	claudePath, err := exec.LookPath("claude")
	if err != nil {
		os.Exit(1)
	}

	reader := claude.OSReader{
		HistoryPath: filepath.Join(home, ".claude", "history.jsonl"),
	}
	gen := branchname.CLIGenerator{ClaudePath: claudePath}
	runner := git.OSCommandRunner{}

	cfg := rename.WatcherConfig{
		WorktreePath: *wtPath,
		Branch:       *branch,
		CreatedAt:    createdAt,
		PollInterval: 2 * time.Second,
		Timeout:      10 * time.Minute,
	}

	w := rename.NewWatcher(cfg, reader, gen, runner)
	if err := w.Run(); err != nil {
		// Silently exit; this is a background process
		os.Exit(1)
	}
}

func spawnRenameWatcher(worktreePath, branch string, createdAt int64) {
	exe, err := os.Executable()
	if err != nil {
		return
	}

	cmd := exec.Command(exe, "watch-rename",
		"--path", worktreePath,
		"--branch", branch,
		"--created-at", strconv.FormatInt(createdAt, 10),
	)
	// Detach from parent process
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	_ = cmd.Start()
}

func findRepoByPath(cfg model.Config, repoPath string) model.RepositoryDef {
	for _, repo := range cfg.Repositories {
		if repo.Path == repoPath {
			return repo
		}
	}
	return model.RepositoryDef{}
}
