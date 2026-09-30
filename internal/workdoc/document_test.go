package workdoc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAndValidate_ValidDocument(t *testing.T) {
	docPath := writeFixtureDoc(t, documentFixture(documentFixtureOpts{
		profile:     "Game-Studio",
		engine:      "Godot",
		platform:    "Pi only",
		persistence: "hybrid",
	}))

	doc, err := LoadAndValidate(docPath)
	if err != nil {
		t.Fatalf("LoadAndValidate() failed: %v", err)
	}

	if doc.ProfileName != "Game-Studio" {
		t.Fatalf("expected default profile name, got %q", doc.ProfileName)
	}
	if doc.PrimaryEngine != "godot" {
		t.Fatalf("expected primary engine godot, got %q", doc.PrimaryEngine)
	}
	if doc.Platform != "Pi only" {
		t.Fatalf("expected platform Pi only, got %q", doc.Platform)
	}
	if !strings.Contains(doc.PersistenceMode, "hybrid") {
		t.Fatalf("expected persistence mode hybrid, got %q", doc.PersistenceMode)
	}
}

func TestLoadAndValidate_AcceptsPiOnlyCaseInsensitively(t *testing.T) {
	docPath := writeFixtureDoc(t, documentFixture(documentFixtureOpts{
		platform:    "PI ONLY",
		engine:      "Godot",
		persistence: "hybrid",
	}))

	doc, err := LoadAndValidate(docPath)
	if err != nil {
		t.Fatalf("LoadAndValidate() rejected case-variant platform: %v", err)
	}
	if !strings.EqualFold(doc.Platform, CanonicalPlatform) {
		t.Fatalf("expected case-insensitive Pi platform, got %q", doc.Platform)
	}
}

func TestLoadAndValidate_RejectsMissingSection(t *testing.T) {
	docPath := writeFixtureDoc(t, documentFixture(documentFixtureOpts{
		profile:     "Game-Studio",
		engine:      "Godot",
		platform:    "Pi only",
		persistence: "hybrid",
		omitFourth:  true,
	}))

	_, err := LoadAndValidate(docPath)
	if err == nil {
		t.Fatalf("expected missing section error")
	}
	if !strings.Contains(err.Error(), `required section "## 4) Architecture Direction (Working)"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadAndValidate_RejectsInvalidProfileDeclaration(t *testing.T) {
	docPath := writeFixtureDoc(t, documentFixture(documentFixtureOpts{
		profileLine: "- User-facing profile count: Game-Studio",
		engine:      "Godot",
		platform:    "Pi only",
		persistence: "hybrid",
	}))

	_, err := LoadAndValidate(docPath)
	if err == nil {
		t.Fatalf("expected backtick validation error")
	}
	if !strings.Contains(err.Error(), "profile declaration must include backticks") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadAndValidate_RejectsUnsupportedPlatformOrPersistence(t *testing.T) {
	tests := []struct {
		name         string
		platform     string
		persistence  string
		expectedHint string
	}{
		{name: "unsupported platform", platform: "Claude Code only", persistence: "hybrid", expectedHint: "unsupported platform"},
		{name: "legacy opencode platform rejected", platform: "OpenCode only", persistence: "hybrid", expectedHint: "unsupported platform"},
		{name: "unknown platform rejected", platform: "Some Future Runtime", persistence: "hybrid", expectedHint: "unsupported platform"},
		{name: "unsupported persistence", platform: "Pi only", persistence: "local", expectedHint: "unsupported persistence mode"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			docPath := writeFixtureDoc(t, documentFixture(documentFixtureOpts{
				platform:    tt.platform,
				persistence: tt.persistence,
				engine:      "Godot",
			}))

			_, err := LoadAndValidate(docPath)
			if err == nil {
				t.Fatalf("expected validation error")
			}
			if !strings.Contains(err.Error(), tt.expectedHint) {
				t.Fatalf("expected error containing %q, got %v", tt.expectedHint, err)
			}
		})
	}
}

func TestDocumentHelpers(t *testing.T) {
	if line, ok := findLineWithPrefix("  - key: value", "- key:"); !ok || line != "- key: value" {
		t.Fatalf("expected trimmed line match, got %q %v", line, ok)
	}

	value, ok := findValueWithPrefix("- key: value", "- key:")
	if !ok || value != "value" {
		t.Fatalf("expected value extraction, got %q %v", value, ok)
	}

	v, ok := extractBacktickValue("- User-facing profile count: `Game-Studio`")
	if !ok || v != "Game-Studio" {
		t.Fatalf("expected backtick extraction, got %q %v", v, ok)
	}
}

func writeFixtureDoc(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "GAME-STUDIO.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture document: %v", err)
	}
	return path
}

type documentFixtureOpts struct {
	profile     string
	profileLine string
	engine      string
	platform    string
	persistence string
	omitFourth  bool
	omitSixth   bool
}

func documentFixture(opts documentFixtureOpts) string {
	if opts.profile == "" {
		opts.profile = "Game-Studio"
	}
	if opts.profileLine == "" {
		opts.profileLine = "- User-facing profile count: `" + opts.profile + "`"
	}
	if opts.engine == "" {
		opts.engine = "Godot"
	}
	if opts.platform == "" {
		opts.platform = "Pi only"
	}
	if opts.persistence == "" {
		opts.persistence = "hybrid"
	}
	includeFourth := !opts.omitFourth
	includeSixth := !opts.omitSixth

	parts := []string{
		"# Game-Studio Working Document",
		"",
		"## 1) Vision",
		"## 2) Product Definition",
		"## 3) Non-Negotiable Constraints",
	}
	if includeFourth {
		parts = append(parts, "## 4) Architecture Direction (Working)")
	}
	parts = append(parts, "## 5) First Implementation Batch")
	if includeSixth {
		parts = append(parts, "## 6) CCGS Preservation Notes")
	}

	parts = append(parts,
		"- User-facing profile count: `Game-Studio`",
		"- Primary engine target (phase 1): "+opts.engine,
		"- Platform: "+opts.platform,
		"- Persistence mode: "+opts.persistence,
	)

	if opts.profileLine != "- User-facing profile count: `Game-Studio`" {
		parts[len(parts)-4] = opts.profileLine
	}

	return strings.Join(parts, "\n") + "\n"
}
