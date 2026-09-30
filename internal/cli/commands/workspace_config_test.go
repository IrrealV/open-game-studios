package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"open-game-studios/internal/persistence"
	"open-game-studios/internal/templates"
)

func TestWorkspaceConfigAtomicRoundTrip(t *testing.T) {
	root := t.TempDir()
	want, err := newWorkspaceConfig("minimal", time.Date(2026, time.August, 29, 10, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("newWorkspaceConfig returned error: %v", err)
	}
	if err := writeWorkspaceConfigAtomic(root, want); err != nil {
		t.Fatalf("writeWorkspaceConfigAtomic returned error: %v", err)
	}

	got, exists, err := readWorkspaceConfig(root)
	if err != nil {
		t.Fatalf("readWorkspaceConfig returned error: %v", err)
	}
	if !exists {
		t.Fatal("workspace config should exist after atomic write")
	}
	if got != want {
		t.Fatalf("workspace config=%#v want %#v", got, want)
	}

	entries, err := os.ReadDir(filepath.Join(root, ".game-studio"))
	if err != nil {
		t.Fatalf("read workspace config directory: %v", err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp") {
			t.Fatalf("atomic write left temporary file %q", entry.Name())
		}
	}
}

func TestWorkspaceConfigAtomicWritePreservesExistingConfigOnValidationFailure(t *testing.T) {
	root := t.TempDir()
	initial, err := newWorkspaceConfig("minimal", time.Date(2026, time.August, 29, 10, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("newWorkspaceConfig returned error: %v", err)
	}
	if err := writeWorkspaceConfigAtomic(root, initial); err != nil {
		t.Fatalf("write initial workspace config: %v", err)
	}
	path := filepath.Join(root, filepath.FromSlash(workspaceConfigRelativePath))
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read initial workspace config: %v", err)
	}

	invalid := initial
	invalid.SetupPreset = "invalid"
	if err := writeWorkspaceConfigAtomic(root, invalid); err == nil {
		t.Fatal("invalid replacement should fail")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read workspace config after failed replacement: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("failed atomic replacement changed existing config\nbefore=%s\nafter=%s", before, after)
	}
}

func TestReadWorkspaceConfigRejectsInvalidContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{name: "malformed JSON", content: `{"schema_version":`, wantErr: "malformed"},
		{name: "unexpected field", content: `{"schema_version":"workspace-config/v1","setup_preset":"minimal","updated_at":"2026-08-29T10:30:00Z","extra":true}`, wantErr: "unexpected"},
		{name: "unsupported schema", content: `{"schema_version":"workspace-config/v2","setup_preset":"minimal","updated_at":"2026-08-29T10:30:00Z"}`, wantErr: "unsupported schema_version"},
		{name: "missing schema", content: `{"setup_preset":"minimal","updated_at":"2026-08-29T10:30:00Z"}`, wantErr: "unsupported schema_version"},
		{name: "missing preset", content: `{"schema_version":"workspace-config/v1","updated_at":"2026-08-29T10:30:00Z"}`, wantErr: "missing required setup_preset"},
		{name: "invalid preset", content: `{"schema_version":"workspace-config/v1","setup_preset":"Minimall","updated_at":"2026-08-29T10:30:00Z"}`, wantErr: "invalid setup_preset"},
		{name: "missing timestamp", content: `{"schema_version":"workspace-config/v1","setup_preset":"minimal"}`, wantErr: "missing required updated_at"},
		{name: "invalid timestamp", content: `{"schema_version":"workspace-config/v1","setup_preset":"minimal","updated_at":"yesterday"}`, wantErr: "expected RFC3339"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeRawWorkspaceConfig(t, root, tt.content)
			_, exists, err := readWorkspaceConfig(root)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("readWorkspaceConfig error=%v want containing %q", err, tt.wantErr)
			}
			if exists {
				t.Fatal("invalid workspace config must not be reported as valid")
			}
		})
	}
}

func TestReadWorkspaceConfigMissingIsNotCorruption(t *testing.T) {
	_, exists, err := readWorkspaceConfig(t.TempDir())
	if err != nil {
		t.Fatalf("missing workspace config returned error: %v", err)
	}
	if exists {
		t.Fatal("missing workspace config must report exists=false")
	}
}

func TestRunGenerateResolvesPersistedAndDefaultPresets(t *testing.T) {
	tests := []struct {
		name        string
		preset      string
		writeConfig bool
		wantSource  string
	}{
		{name: "persisted minimal", preset: "minimal", writeConfig: true, wantSource: presetSourcePersistedWorkspace},
		{name: "persisted recommended", preset: "recommended", writeConfig: true, wantSource: presetSourcePersistedWorkspace},
		{name: "persisted full", preset: "full", writeConfig: true, wantSource: presetSourcePersistedWorkspace},
		{name: "persisted custom", preset: "custom", writeConfig: true, wantSource: presetSourcePersistedWorkspace},
		{name: "missing config defaults to recommended", preset: "recommended", writeConfig: false, wantSource: presetSourceDefault},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, nested := generateWorkspaceFixture(t)
			if tt.writeConfig {
				config, err := newWorkspaceConfig(tt.preset, time.Now())
				if err != nil {
					t.Fatalf("newWorkspaceConfig returned error: %v", err)
				}
				if err := writeWorkspaceConfigAtomic(root, config); err != nil {
					t.Fatalf("writeWorkspaceConfigAtomic returned error: %v", err)
				}
			}
			setWorkingDirectory(t, nested)

			err := RunGenerate(GenerateInput{
				Args:        []string{"--out-dir", "generated", "--layout", "flat"},
				Registry:    templates.NewRegistry(),
				Persistence: persistence.NewPlaceholder(),
			})
			if err != nil {
				t.Fatalf("RunGenerate returned error: %v", err)
			}

			assertGeneratedPresetMetadata(t, filepath.Join(root, "generated"), tt.preset, tt.wantSource)
		})
	}
}

func TestRunGenerateExplicitOverrideDoesNotRewriteWorkspaceConfig(t *testing.T) {
	root, nested := generateWorkspaceFixture(t)
	config, err := newWorkspaceConfig("minimal", time.Date(2026, time.August, 29, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("newWorkspaceConfig returned error: %v", err)
	}
	if err := writeWorkspaceConfigAtomic(root, config); err != nil {
		t.Fatalf("writeWorkspaceConfigAtomic returned error: %v", err)
	}
	configPath := filepath.Join(root, filepath.FromSlash(workspaceConfigRelativePath))
	before, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read workspace config before generate: %v", err)
	}
	setWorkingDirectory(t, nested)

	err = RunGenerate(GenerateInput{
		Args:        []string{"--setup-depth", "full", "--out-dir", "generated"},
		Registry:    templates.NewRegistry(),
		Persistence: persistence.NewPlaceholder(),
	})
	if err != nil {
		t.Fatalf("RunGenerate returned error: %v", err)
	}
	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read workspace config after generate: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("explicit generate override rewrote persisted workspace config\nbefore=%s\nafter=%s", before, after)
	}
	assertGeneratedPresetMetadata(t, filepath.Join(root, "generated"), "full", presetSourceExplicitCLI)
}

func TestRunGenerateExplicitOverrideRejectsInvalidWorkspaceConfigBeforeRendering(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{name: "malformed JSON", content: `{"schema_version":`, wantErr: "malformed"},
		{name: "unsupported schema", content: `{"schema_version":"workspace-config/v2","setup_preset":"minimal","updated_at":"2026-08-29T10:30:00Z"}`, wantErr: "unsupported schema_version"},
		{name: "unknown field", content: `{"schema_version":"workspace-config/v1","setup_preset":"minimal","updated_at":"2026-08-29T10:30:00Z","extra":true}`, wantErr: "unexpected"},
		{name: "invalid persisted preset", content: `{"schema_version":"workspace-config/v1","setup_preset":"Minimall","updated_at":"2026-08-29T10:30:00Z"}`, wantErr: "invalid setup_preset"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, nested := generateWorkspaceFixture(t)
			writeRawWorkspaceConfig(t, root, tt.content)
			configPath := filepath.Join(root, filepath.FromSlash(workspaceConfigRelativePath))
			before, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatalf("read workspace config before generate: %v", err)
			}
			setWorkingDirectory(t, nested)

			err = RunGenerate(GenerateInput{
				Args:        []string{"--setup-depth", "full", "--out-dir", "generated"},
				Registry:    templates.NewRegistry(),
				Persistence: persistence.NewPlaceholder(),
			})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("RunGenerate error=%v want containing %q", err, tt.wantErr)
			}
			if _, statErr := os.Stat(filepath.Join(root, "generated")); !os.IsNotExist(statErr) {
				t.Fatalf("invalid config rendered output directory, stat error=%v", statErr)
			}
			after, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatalf("read workspace config after generate: %v", err)
			}
			if string(after) != string(before) {
				t.Fatalf("explicit generate override changed invalid workspace config\nbefore=%s\nafter=%s", before, after)
			}
		})
	}
}

func TestRunGenerateRejectsInvalidWorkspaceConfigBeforeRendering(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{name: "corrupt JSON", content: `{"schema_version":`, wantErr: "malformed"},
		{name: "invalid preset", content: `{"schema_version":"workspace-config/v1","setup_preset":"Minimall","updated_at":"2026-08-29T10:30:00Z"}`, wantErr: "invalid setup_preset"},
		{name: "unsupported schema", content: `{"schema_version":"workspace-config/v2","setup_preset":"minimal","updated_at":"2026-08-29T10:30:00Z"}`, wantErr: "unsupported schema_version"},
		{name: "missing required field", content: `{"schema_version":"workspace-config/v1","updated_at":"2026-08-29T10:30:00Z"}`, wantErr: "missing required setup_preset"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, nested := generateWorkspaceFixture(t)
			writeRawWorkspaceConfig(t, root, tt.content)
			setWorkingDirectory(t, nested)

			err := RunGenerate(GenerateInput{
				Args:        []string{"--out-dir", "generated"},
				Registry:    templates.NewRegistry(),
				Persistence: persistence.NewPlaceholder(),
			})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("RunGenerate error=%v want containing %q", err, tt.wantErr)
			}
			if _, statErr := os.Stat(filepath.Join(root, "generated")); !os.IsNotExist(statErr) {
				t.Fatalf("invalid config rendered output directory, stat error=%v", statErr)
			}
		})
	}
}

func TestSmokeRejectsContradictoryGeneratedPresetMetadata(t *testing.T) {
	root, nested := generateWorkspaceFixture(t)
	config, err := newWorkspaceConfig("minimal", time.Now())
	if err != nil {
		t.Fatalf("newWorkspaceConfig returned error: %v", err)
	}
	if err := writeWorkspaceConfigAtomic(root, config); err != nil {
		t.Fatalf("writeWorkspaceConfigAtomic returned error: %v", err)
	}
	setWorkingDirectory(t, nested)
	if err := RunGenerate(GenerateInput{
		Args:        []string{"--out-dir", "generated"},
		Registry:    templates.NewRegistry(),
		Persistence: persistence.NewPlaceholder(),
	}); err != nil {
		t.Fatalf("RunGenerate returned error: %v", err)
	}

	paths, err := resolveOutputPaths(templates.EngineGodot, "", filepath.Join(root, "generated"), "flat")
	if err != nil {
		t.Fatalf("resolveOutputPaths returned error: %v", err)
	}
	raw, err := os.ReadFile(paths.pattern)
	if err != nil {
		t.Fatalf("read hybrid map: %v", err)
	}
	var pattern map[string]any
	if err := json.Unmarshal(raw, &pattern); err != nil {
		t.Fatalf("parse hybrid map: %v", err)
	}
	pattern["model_routing"].(map[string]any)["routing_preset"] = "recommended"
	corrupt, err := json.MarshalIndent(pattern, "", "  ")
	if err != nil {
		t.Fatalf("encode contradictory hybrid map: %v", err)
	}
	if err := os.WriteFile(paths.pattern, corrupt, 0o644); err != nil {
		t.Fatalf("write contradictory hybrid map: %v", err)
	}

	err = runSmokeContentChecks(paths)
	if err == nil || !strings.Contains(err.Error(), "preset provenance mismatch") {
		t.Fatalf("smoke error=%v want preset provenance mismatch", err)
	}
}

func assertGeneratedPresetMetadata(t *testing.T, outDir, preset, source string) {
	t.Helper()
	paths, err := resolveOutputPaths(templates.EngineGodot, "", outDir, "flat")
	if err != nil {
		t.Fatalf("resolveOutputPaths returned error: %v", err)
	}
	if err := runSmokeContentChecks(paths); err != nil {
		t.Fatalf("generated outputs failed smoke validation: %v", err)
	}

	for _, path := range []string{paths.profile, paths.summary} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read generated markdown %s: %v", path, err)
		}
		for _, marker := range []string{
			"- setup_preset: " + preset,
			"- preset_source: " + source,
			"- routing_preset: " + preset,
			"- routing_preset_source: " + routingPresetSourceDerived,
		} {
			if !strings.Contains(string(raw), marker) {
				t.Fatalf("generated markdown %s missing %q", path, marker)
			}
		}
	}

	for _, path := range []string{paths.config, paths.pattern} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read generated JSON %s: %v", path, err)
		}
		var payload map[string]any
		if err := json.Unmarshal(raw, &payload); err != nil {
			t.Fatalf("parse generated JSON %s: %v", path, err)
		}
		assertPresetFields(t, path, payload, preset, source)
		modelRouting, ok := payload["model_routing"].(map[string]any)
		if !ok {
			t.Fatalf("model_routing missing from %s", path)
		}
		assertPresetFields(t, path+" model_routing", modelRouting, preset, source)
	}
}

func assertPresetFields(t *testing.T, label string, payload map[string]any, preset, source string) {
	t.Helper()
	want := map[string]string{
		"setup_preset":          preset,
		"preset_source":         source,
		"routing_preset":        preset,
		"routing_preset_source": routingPresetSourceDerived,
	}
	for key, value := range want {
		if payload[key] != value {
			t.Fatalf("%s %s=%#v want %q", label, key, payload[key], value)
		}
	}
}

func generateWorkspaceFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	gameStudioDir := filepath.Join(root, ".game-studio")
	if err := os.MkdirAll(gameStudioDir, 0o755); err != nil {
		t.Fatalf("create .game-studio directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gameStudioDir, "workspace.manifest.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write workspace manifest marker: %v", err)
	}
	doc := `## 1) Vision

## 2) Product Definition

- User-facing profile count: ` + "`Game-Studio`" + `
- Primary engine target (phase 1): godot
- Platform: Pi only
- Persistence mode: hybrid

## 3) Non-Negotiable Constraints

## 4) Architecture Direction (Working)

## 6) CCGS Preservation Notes
`
	if err := os.WriteFile(filepath.Join(root, "GAME-STUDIO.md"), []byte(doc), 0o644); err != nil {
		t.Fatalf("write working document: %v", err)
	}
	nested := filepath.Join(root, "nested", "working", "directory")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("create nested working directory: %v", err)
	}
	return root, nested
}

func writeRawWorkspaceConfig(t *testing.T, root, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(workspaceConfigRelativePath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create workspace config directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write raw workspace config: %v", err)
	}
}
