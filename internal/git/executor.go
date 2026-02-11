package git

import (
	"fmt"
	"os/exec"
)

// CommandRunner abstracts shell command execution for testability.
type CommandRunner interface {
	Run(dir string, args ...string) (string, error)
}

// OSCommandRunner executes real git commands via os/exec.
type OSCommandRunner struct{}

func (r OSCommandRunner) Run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git %v failed: %s", args, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("git %v failed: %w", args, err)
	}
	return string(out), nil
}

// FakeCommandRunner is a test double that returns preset output.
type FakeCommandRunner struct {
	Outputs map[string]string
	Errors  map[string]error
}

func (r FakeCommandRunner) key(dir string, args ...string) string {
	return fmt.Sprintf("%s:%v", dir, args)
}

func (r FakeCommandRunner) Run(dir string, args ...string) (string, error) {
	key := r.key(dir, args...)
	if err, ok := r.Errors[key]; ok {
		return "", err
	}
	if out, ok := r.Outputs[key]; ok {
		return out, nil
	}
	return "", fmt.Errorf("FakeCommandRunner: no output for key %q", key)
}
