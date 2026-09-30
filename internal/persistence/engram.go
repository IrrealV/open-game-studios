package persistence

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type SaveInput struct {
	Title   string
	Body    string
	Type    string
	Scope   string
	Project string
}

func SaveEngramCLI(input SaveInput) error {
	return saveEngramCLI(input, "")
}

// saveEngramCLI runs the real Engram save command with the historical bounded
// timeout. An empty executable keeps the default engram-on-PATH resolution; an
// explicit executable (the backend-reported Engram binary) is used verbatim so a
// managed install can be reached without changing the ambient PATH.
func saveEngramCLI(input SaveInput, executable string) error {
	return saveEngramCLIContext(context.Background(), input, executable)
}

// saveEngramCLIContext is the context-aware form used by the wizard. The 10s
// save timeout is derived from the caller context, so a canceled wizard run
// aborts the memory write instead of blocking on it.
func saveEngramCLIContext(ctx context.Context, input SaveInput, executable string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	exe := strings.TrimSpace(executable)
	if exe == "" {
		resolved, err := exec.LookPath("engram")
		if err != nil {
			return fmt.Errorf("engram cli unavailable: %w", err)
		}
		exe = resolved
	}

	title := strings.TrimSpace(input.Title)
	body := strings.TrimSpace(input.Body)
	if title == "" || body == "" {
		return fmt.Errorf("engram save input requires non-empty title and body")
	}

	args := []string{"save", title, body}
	if strings.TrimSpace(input.Type) != "" {
		args = append(args, "--type", strings.TrimSpace(input.Type))
	}
	if strings.TrimSpace(input.Project) != "" {
		args = append(args, "--project", strings.TrimSpace(input.Project))
	}
	if strings.TrimSpace(input.Scope) != "" {
		args = append(args, "--scope", strings.TrimSpace(input.Scope))
	}

	runCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(runCtx, exe, args...)
	raw, err := cmd.CombinedOutput()
	if err != nil {
		output := strings.TrimSpace(string(raw))
		if output == "" {
			return fmt.Errorf("engram save failed: %w", err)
		}
		return fmt.Errorf("engram save failed: %w (%s)", err, output)
	}

	return nil
}
