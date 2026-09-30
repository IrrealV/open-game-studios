package commands

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"open-game-studios/internal/toolcheck"
)

type EnvCheckInput struct {
	Args    []string
	Checker EnvironmentChecker
}

type EnvironmentChecker interface {
	Execute(context.Context, toolcheck.Selection) []toolcheck.Result
}

type repeatedToolFlags []string

func (f *repeatedToolFlags) String() string { return strings.Join(*f, ",") }
func (f *repeatedToolFlags) Set(value string) error {
	for _, part := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			*f = append(*f, trimmed)
		}
	}
	return nil
}

func RunEnvCheck(input EnvCheckInput) error {
	fs := flag.NewFlagSet("env-check", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var tools repeatedToolFlags
	fs.Var(&tools, "tool", "tool id to check; repeat or comma-separate for multiple tools")
	if err := fs.Parse(input.Args); err != nil {
		return err
	}

	checker := input.Checker
	if checker == nil {
		checker = toolcheck.NewRunner(toolcheck.NewDefaultRegistry(), nil)
	}

	selection := toolcheck.SelectAll()
	if len(tools) > 0 {
		selection = toolcheck.SelectTools(tools...)
	}
	results := checker.Execute(context.Background(), selection)
	printEnvCheckResults("env-check", results)
	return toolcheck.BlockingError(results)
}

func printEnvCheckResults(prefix string, results []toolcheck.Result) {
	if len(results) == 0 {
		fmt.Printf("[%s] no tool checks selected\n", prefix)
		return
	}
	for _, result := range results {
		fmt.Printf("[%s] %s: %s\n", prefix, result.ToolID, result.Status)
		if result.Reason != "" {
			fmt.Printf("  reason: %s\n", result.Reason)
		}
		if result.Attempted != "" {
			fmt.Printf("  attempted: %s\n", result.Attempted)
		}
		if result.Remediation != "" {
			fmt.Printf("  remediation: %s\n", result.Remediation)
		}
	}
}

func DefaultEnvironmentChecker() EnvironmentChecker {
	return toolcheck.NewRunner(toolcheck.NewDefaultRegistry(), nil)
}
