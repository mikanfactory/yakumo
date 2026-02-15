package github

import (
	"fmt"
	"os/exec"
)

// Runner abstracts gh CLI command execution for testability.
type Runner interface {
	Run(dir string, args ...string) (string, error)
}

// OSRunner executes real gh commands via os/exec.
type OSRunner struct{}

func (r OSRunner) Run(dir string, args ...string) (string, error) {
	cmd := exec.Command("gh", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("gh %v failed: %s", args, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("gh %v failed: %w", args, err)
	}
	return string(out), nil
}

// FakeRunner is a test double that returns preset output and records calls.
type FakeRunner struct {
	Outputs map[string]string
	Errors  map[string]error
	Calls   [][]string
}

func (r *FakeRunner) key(dir string, args ...string) string {
	return fmt.Sprintf("%s:%v", dir, args)
}

func (r *FakeRunner) Run(dir string, args ...string) (string, error) {
	r.Calls = append(r.Calls, append([]string{dir}, args...))
	key := r.key(dir, args...)
	if r.Errors != nil {
		if err, ok := r.Errors[key]; ok {
			return "", err
		}
	}
	if r.Outputs != nil {
		if out, ok := r.Outputs[key]; ok {
			return out, nil
		}
	}
	return "", fmt.Errorf("FakeRunner: no output for key %q", key)
}
