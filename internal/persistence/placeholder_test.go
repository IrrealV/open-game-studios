package persistence

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func Test_sanitizeArtifactPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "relative path stays relative", input: "out/wizard/final.artifact.json", want: "out/wizard/final.artifact.json"},
		{name: "opencode workspace path collapses", input: "/tmp/workspace/.opencode/profiles/game-studio.md", want: ".opencode/profiles/game-studio.md"},
		{name: "out workspace path collapses", input: "/tmp/workspace/out/generated/artifact.json", want: "out/generated/artifact.json"},
		{name: "game-studio workspace path collapses", input: "/tmp/workspace/.game-studio/generated/wizard/final.artifact.json", want: ".game-studio/generated/wizard/final.artifact.json"},
		{name: "unknown absolute path falls back to base", input: "/tmp/some/random/path/notes.txt", want: "notes.txt"},
		{name: "empty path stays empty", input: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeArtifactPath(tt.input); got != tt.want {
				t.Fatalf("sanitizeArtifactPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func Test_formatArtifactList_Empty(t *testing.T) {
	if got := formatArtifactList(nil); got != "" {
		t.Fatalf("expected empty artifact list formatting, got %q", got)
	}
	if got := formatArtifactList([]string{}); got != "" {
		t.Fatalf("expected empty artifact list formatting, got %q", got)
	}
	if got := formatArtifactList([]string{"out/item.json", "other.json"}); got != "- out/item.json\n- other.json" {
		t.Fatalf("unexpected artifact list formatting: %q", got)
	}
	if strings.Contains(formatArtifactList(nil), "- none") {
		t.Fatalf("unexpected none sentinel in empty artifact list")
	}
}

func TestWriteGeneration_UsesExplicitEngramExecutable(t *testing.T) {
	workspace := t.TempDir()

	spyDir := filepath.Join(workspace, "spy-bin")
	if err := os.MkdirAll(spyDir, 0o755); err != nil {
		t.Fatalf("create spy dir: %v", err)
	}
	logPath := filepath.Join(workspace, "explicit-engram.log")
	spyPath := filepath.Join(spyDir, "explicit-engram")
	spyScript := "#!/bin/sh\n" +
		"printf 'arg1=%s\\n' \"$1\" >> \"$ENGRAM_SPY_LOG\"\n" +
		"printf 'arg2=%s\\n' \"$2\" >> \"$ENGRAM_SPY_LOG\"\n" +
		"exit 0\n"
	if err := os.WriteFile(spyPath, []byte(spyScript), 0o755); err != nil {
		t.Fatalf("write explicit engram spy: %v", err)
	}

	t.Setenv("ENGRAM_SPY_LOG", logPath)
	// The spy directory deliberately contains no `engram` entry: PATH resolution
	// would fail, so a successful save proves the explicit executable was used.
	t.Setenv("PATH", spyDir)

	err := NewPlaceholder().WithEngramExecutable(spyPath).WriteGeneration(GenerationWriteThrough{
		ProfileName:     "Explicit Engram Profile",
		Engine:          "godot",
		PersistenceMode: "hybrid",
		SummaryArtifact: filepath.Join(workspace, "out", "wizard", "final.artifact.json"),
		GeneratedAt:     time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("WriteGeneration with explicit executable returned error: %v", err)
	}

	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("expected the explicit Engram executable to receive the save: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, "arg1=save") {
		t.Fatalf("expected the save subcommand to be passed as argv[1], got: %s", text)
	}
	if !strings.Contains(text, "arg2=installer-wizard/final-artifact/godot") {
		t.Fatalf("expected the write-through title as argv[2], got: %s", text)
	}
}

func TestWriteGeneration_BodyAvoidsAbsolutePathsAndIncludesSanitizedArtifacts(t *testing.T) {

	workspace := t.TempDir()

	spyDir := filepath.Join(workspace, "spy-bin")
	if err := os.MkdirAll(spyDir, 0o755); err != nil {
		t.Fatalf("create spy dir: %v", err)
	}

	logPath := filepath.Join(workspace, "engram-spy.log")
	spyScript := "#!/bin/sh\n" +
		"printf 'title=%s\\n' \"$2\" >> \"$ENGRAM_SPY_LOG\"\n" +
		"printf 'body-begin\\n' >> \"$ENGRAM_SPY_LOG\"\n" +
		"printf '%s\\n' \"$3\" >> \"$ENGRAM_SPY_LOG\"\n" +
		"printf 'body-end\\n' >> \"$ENGRAM_SPY_LOG\"\n" +
		"exit 0\n"

	engramPath := filepath.Join(spyDir, "engram")
	if err := os.WriteFile(engramPath, []byte(spyScript), 0o755); err != nil {
		t.Fatalf("write engram spy script: %v", err)
	}

	t.Setenv("ENGRAM_SPY_LOG", logPath)
	t.Setenv("PATH", spyDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	err := NewPlaceholder().WriteGeneration(GenerationWriteThrough{
		ProfileName:     "Write Through Profile",
		Engine:          "godot",
		PersistenceMode: "hybrid",
		SummaryArtifact: filepath.Join(workspace, "out", "wizard", "final.artifact.json"),
		GeneratedAt:     time.Now().UTC(),
		EmittedArtifacts: []string{
			filepath.Join(workspace, "out", "generated", "studio-profile.godot.md"),
			filepath.Join(workspace, "out", "generated", "pack.godot.config.json"),
			filepath.Join(workspace, ".opencode", "profiles", "write-through-profile.md"),
			filepath.Join(workspace, ".game-studio", "generated", "wizard", "final.artifact.json"),
			filepath.Join("/", "tmp", "not-a-workspace", "other.txt"),
		},
	})
	if err != nil {
		t.Fatalf("WriteGeneration returned error: %v", err)
	}

	spyLog, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read engram spy log: %v", err)
	}

	text := string(spyLog)
	if !strings.Contains(text, "title=installer-wizard/final-artifact/godot") {
		t.Fatalf("expected engram save title in spy log, got: %s", text)
	}

	start := strings.Index(text, "body-begin\n")
	end := strings.LastIndex(text, "body-end\n")
	if start == -1 || end == -1 || end <= start {
		t.Fatalf("invalid body markers in spy log: %s", text)
	}
	body := text[start+len("body-begin\n") : end]

	if strings.Contains(body, workspace+string(filepath.Separator)) {
		t.Fatalf("expected workspace-relative payload, got absolute path in body: %s", body)
	}
	if strings.Contains(body, "- none") {
		t.Fatalf("expected sanitized artifact list without '- none' sentinel: %s", body)
	}
	if !strings.Contains(body, "summary_artifact: out/wizard/final.artifact.json") {
		t.Fatalf("expected relative summary_artifact path in body, got: %s", body)
	}
	for _, expected := range []string{
		"- out/generated/studio-profile.godot.md",
		"- out/generated/pack.godot.config.json",
		"- .opencode/profiles/write-through-profile.md",
		"- .game-studio/generated/wizard/final.artifact.json",
		"- other.txt",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected body to include emitted artifact %q, got: %s", expected, body)
		}
	}
}
