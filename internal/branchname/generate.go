package branchname

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/mikanfactory/yakumo/internal/git"
)

// Generator abstracts LLM calls for testability.
type Generator interface {
	GenerateBranchName(prompt string) (string, error)
}

// CLIGenerator calls the claude CLI to generate branch names.
type CLIGenerator struct {
	ClaudePath string
}

const systemPrompt = `You are a git branch name generator. Given a task description, generate a concise kebab-case branch name that summarizes the task.

Rules:
- Use lowercase kebab-case (e.g., "fix-login-redirect", "add-user-settings")
- Maximum 30 characters
- No prefixes like "feature/" or "fix/" -- just the descriptive part
- Output ONLY the branch name, nothing else
- No quotes, no explanation, just the raw branch name`

const maxBranchNameLength = 30

var validBranchChar = regexp.MustCompile(`[^a-z0-9-]`)
var multiHyphen = regexp.MustCompile(`-{2,}`)

func (g CLIGenerator) GenerateBranchName(prompt string) (string, error) {
	claudePath := g.ClaudePath
	if claudePath == "" {
		claudePath = "claude"
	}

	fullPrompt := systemPrompt + "\n\nTask description:\n" + prompt

	cmd := exec.Command(claudePath, "-p", fullPrompt,
		"--output-format", "text",
		"--model", "haiku",
		"--no-session-persistence",
	)

	cmd.Env = filterEnv(os.Environ(), "CLAUDECODE")

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("claude CLI failed: %w", err)
	}

	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return "", fmt.Errorf("empty output from claude CLI")
	}

	return SanitizeBranchName(raw), nil
}

// filterEnv returns a copy of env with the specified key removed.
func filterEnv(env []string, excludeKey string) []string {
	var filtered []string
	prefix := excludeKey + "="
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// FakeGenerator is a test double.
type FakeGenerator struct {
	Result string
	Err    error
}

func (g FakeGenerator) GenerateBranchName(_ string) (string, error) {
	return g.Result, g.Err
}

// SlugFromBranch extracts the slug portion from a branch name.
// "shoji/fix-login-redirect" → "fix-login-redirect"
// "fix-login-redirect" → "fix-login-redirect"
func SlugFromBranch(branch string) string {
	if parts := strings.SplitN(branch, "/", 2); len(parts) == 2 {
		return parts[1]
	}
	return branch
}

// SanitizeBranchName ensures the name is kebab-case, lowercase, and within the max length.
func SanitizeBranchName(name string) string {
	result := git.Slugify(name)

	// Additional cleanup
	result = validBranchChar.ReplaceAllString(result, "")
	result = multiHyphen.ReplaceAllString(result, "-")
	result = strings.Trim(result, "-")

	if len(result) > maxBranchNameLength {
		result = result[:maxBranchNameLength]
		result = strings.TrimRight(result, "-")
	}

	return result
}
