package toolcheck

import (
	"context"
	"fmt"
	"strings"
)

type Runner struct {
	Registry      *Registry
	CommandRunner CommandRunner
	FailFast      bool
}

func NewRunner(registry *Registry, commandRunner CommandRunner) *Runner {
	if registry == nil {
		registry = NewDefaultRegistry()
	}
	if commandRunner == nil {
		commandRunner = ExecRunner{}
	}
	return &Runner{Registry: registry, CommandRunner: commandRunner}
}

func (r *Runner) Execute(ctx context.Context, selection Selection) []Result {
	runner := r.CommandRunner
	if runner == nil {
		runner = ExecRunner{}
	}
	registry := r.Registry
	if registry == nil {
		registry = NewDefaultRegistry()
	}

	resolved := registry.Resolve(selection)
	results := append([]Result{}, resolved.Results...)
	if r.FailFast && HasBlocking(results) {
		return results
	}
	for _, check := range resolved.Checks {
		result := check.Run(ctx, runner)
		results = append(results, result)
		if r.FailFast && result.Blocking() {
			break
		}
	}
	return results
}

func BlockingError(results []Result) error {
	blocking := BlockingResults(results)
	if len(blocking) == 0 {
		return nil
	}
	lines := make([]string, 0, len(blocking))
	for _, result := range blocking {
		lines = append(lines, result.String())
	}
	return fmt.Errorf("environment preflight failed:\n%s", strings.Join(lines, "\n"))
}
