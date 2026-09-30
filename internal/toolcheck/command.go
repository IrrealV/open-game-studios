package toolcheck

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type CommandResult struct {
	Path   string
	Stdout string
	Stderr string
}

type CommandRunner interface {
	LookPath(name string) (string, error)
	Run(ctx context.Context, name string, args ...string) (CommandResult, error)
}

type ExecRunner struct {
	Timeout time.Duration
}

func (r ExecRunner) LookPath(name string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%s was not found on PATH; install it or add its install directory to PATH", name)
	}
	return path, nil
}

func (r ExecRunner) Run(ctx context.Context, name string, args ...string) (CommandResult, error) {
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, name, args...)
	out, err := cmd.CombinedOutput()
	result := CommandResult{Stdout: string(out)}
	if err != nil {
		text := strings.TrimSpace(string(out))
		if text == "" {
			text = err.Error()
		}
		return result, fmt.Errorf("run %s %s: %s", name, strings.Join(args, " "), text)
	}
	return result, nil
}
