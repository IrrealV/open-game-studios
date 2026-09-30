package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestMain keeps the real Engram CLI out of the test process.
//
// Wizard generation performs a real external write-through via
// persistence.SaveEngramCLI, which shells out to a host `engram` binary when
// one is on PATH. Tests must exercise that path only through fake/spy
// executables (see TestRunWizard_GenerationAttemptsEngramWriteThrough), never
// the user's Engram database. Prepending a no-op shim makes every other test
// resolve `engram` locally; tests that need a spy prepend their own directory.
func TestMain(m *testing.M) {
	// Never fall back to the host CLI if the protective fixture cannot be used.
	if runtime.GOOS == "windows" {
		fmt.Fprintln(os.Stderr, "commands tests require the POSIX Engram fixture; use Linux/WSL")
		os.Exit(1)
	}
	shimDir, err := os.MkdirTemp("", "ogs-engram-shim-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "create Engram test fixture:", err)
		os.Exit(1)
	}

	shim := filepath.Join(shimDir, "engram")
	if err := os.WriteFile(shim, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		_ = os.RemoveAll(shimDir)
		fmt.Fprintln(os.Stderr, "write Engram test fixture:", err)
		os.Exit(1)
	}
	if err := os.Setenv("PATH", shimDir+string(os.PathListSeparator)+os.Getenv("PATH")); err != nil {
		_ = os.RemoveAll(shimDir)
		fmt.Fprintln(os.Stderr, "activate Engram test fixture:", err)
		os.Exit(1)
	}

	code := m.Run()
	_ = os.RemoveAll(shimDir)
	os.Exit(code)
}
