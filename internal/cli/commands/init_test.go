package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"open-game-studios/internal/persistence"
	"open-game-studios/internal/templates"
	"open-game-studios/internal/workdoc"
)

func TestEnsureWorkspaceManifest_ExistingManifestValidatesAllCriticalFields(t *testing.T) {
	root := t.TempDir()
	doc := sampleWorkdoc()

	manifestPath := filepath.Join(root, ".game-studio", "workspace.manifest.json")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatalf("create manifest dir: %v", err)
	}

	existing := validWorkspaceManifest(doc, "hybrid")
	if err := writeWorkspaceManifest(manifestPath, existing); err != nil {
		t.Fatalf("write fixture manifest: %v", err)
	}

	_, action, err := ensureWorkspaceManifest(root, doc, "hybrid")
	if err != nil {
		t.Fatalf("expected manifest validation to pass, got %v", err)
	}
	if action != "verified" {
		t.Fatalf("expected action 'verified', got %q", action)
	}
}

func TestEnsureWorkspaceManifest_ExistingManifestRejectsCriticalMismatch(t *testing.T) {
	tests := []struct {
		name          string
		mutate        func(manifest *workspaceManifest)
		expectedToken string
	}{
		{
			name: "mismatched profile",
			mutate: func(manifest *workspaceManifest) {
				manifest.ProfileName = "Other Profile"
			},
			expectedToken: "profile_name",
		},
		{
			name: "mismatched platform",
			mutate: func(manifest *workspaceManifest) {
				manifest.Platform = "unknown"
			},
			expectedToken: "platform",
		},
		{
			name: "mismatched primary engine",
			mutate: func(manifest *workspaceManifest) {
				manifest.PrimaryEngine = "unity"
			},
			expectedToken: "primary_engine",
		},
		{
			name: "mismatched persistence mode",
			mutate: func(manifest *workspaceManifest) {
				manifest.PersistenceMode = "local"
			},
			expectedToken: "persistence_mode",
		},
		{
			name: "mismatched supported packs",
			mutate: func(manifest *workspaceManifest) {
				manifest.SupportedPacks = []string{"unity", "godot"}
			},
			expectedToken: "supported_packs",
		},
		{
			name: "mismatched placeholder packs",
			mutate: func(manifest *workspaceManifest) {
				manifest.PlaceholderPacks = nil
			},
			expectedToken: "placeholder_packs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			doc := sampleWorkdoc()

			manifestPath := filepath.Join(root, ".game-studio", "workspace.manifest.json")
			if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
				t.Fatalf("create manifest dir: %v", err)
			}

			existing := validWorkspaceManifest(doc, "hybrid")
			tt.mutate(&existing)
			if err := writeWorkspaceManifest(manifestPath, existing); err != nil {
				t.Fatalf("write fixture manifest: %v", err)
			}

			_, _, err := ensureWorkspaceManifest(root, doc, "hybrid")
			if err == nil {
				t.Fatalf("expected manifest mismatch error")
			}
			if !strings.Contains(err.Error(), tt.expectedToken) {
				t.Fatalf("expected error to include %q, got: %v", tt.expectedToken, err)
			}
		})
	}
}

func TestValidateProfileConfigSkeletonRejectsSemanticallyWrongFile(t *testing.T) {
	doc := sampleWorkdoc()
	base := map[string]any{
		"profile_name":      doc.ProfileName,
		"platform":          doc.Platform,
		"primary_engine":    doc.PrimaryEngine,
		"persistence_mode":  "hybrid",
		"generated_by":      "game-studio init",
		"sdd_handoff":       true,
		"ccgs_preservation": "layered-hybrid",
		"mcp_placeholders": map[string]any{
			"engram":   map[string]any{"enabled": false, "mode": "placeholder", "required": false},
			"context7": map[string]any{"enabled": false, "mode": "placeholder", "required": false},
		},
	}

	tests := []struct {
		name       string
		mutate     func(map[string]any)
		expectedAt string
	}{
		{
			name: "wrong generated_by is rejected",
			mutate: func(fixture map[string]any) {
				fixture["generated_by"] = "legacy-init"
			},
			expectedAt: "generated_by must be \"game-studio init\"",
		},
		{
			name: "legacy ccgs preservation is rejected",
			mutate: func(fixture map[string]any) {
				fixture["ccgs_preservation"] = "legacy"
			},
			expectedAt: "ccgs_preservation must be \"layered-hybrid\"",
		},
		{
			name: "required mcp placeholder is rejected",
			mutate: func(fixture map[string]any) {
				placeholders := fixture["mcp_placeholders"].(map[string]any)
				placeholders["engram"].(map[string]any)["required"] = true
			},
			expectedAt: "optional and never required",
		},
		{
			name: "extra top-level field is rejected",
			mutate: func(fixture map[string]any) {
				fixture["legacy_mode"] = true
			},
			expectedAt: "unexpected field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := mapClone(base)
			tt.mutate(fixture)

			raw, err := json.Marshal(fixture)
			if err != nil {
				t.Fatalf("marshal fixture: %v", err)
			}

			if err := validateProfileConfigSkeleton(raw, doc, "hybrid"); err == nil {
				t.Fatalf("expected validation error for %s", tt.name)
			} else if !strings.Contains(err.Error(), tt.expectedAt) {
				t.Fatalf("expected %q in error for %s, got %v", tt.expectedAt, tt.name, err)
			}
		})
	}
}

func TestValidateProfileConfigSkeletonAcceptsOptionalMCPPlaceholders(t *testing.T) {
	doc := sampleWorkdoc()

	cases := []struct {
		name         string
		placeholders any
	}{
		{
			name: "disabled adapters",
			placeholders: map[string]any{
				"engram":   map[string]any{"enabled": false, "mode": "placeholder", "required": false},
				"context7": map[string]any{"enabled": false, "mode": "placeholder", "required": false},
			},
		},
		{
			name: "subset of adapters",
			placeholders: map[string]any{
				"engram": map[string]any{"enabled": false, "mode": "placeholder", "required": false},
			},
		},
		{
			name:         "empty adapter map",
			placeholders: map[string]any{},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fixture := map[string]any{
				"profile_name":      doc.ProfileName,
				"platform":          doc.Platform,
				"primary_engine":    doc.PrimaryEngine,
				"persistence_mode":  "hybrid",
				"generated_by":      "game-studio init",
				"sdd_handoff":       true,
				"ccgs_preservation": "layered-hybrid",
				"mcp_placeholders":  tt.placeholders,
			}
			raw, err := json.Marshal(fixture)
			if err != nil {
				t.Fatalf("marshal fixture: %v", err)
			}
			if err := validateProfileConfigSkeleton(raw, doc, "hybrid"); err != nil {
				t.Fatalf("expected optional MCP placeholders to validate, got %v", err)
			}
		})
	}
}

func TestValidatePackConfigSkeletonRejectsSemanticallyWrongFile(t *testing.T) {
	doc := sampleWorkdoc()
	repo := templates.NewRegistry()
	pack, ok := repo.GetPack(templates.EngineGodot)
	if !ok {
		t.Fatalf("expected godot pack in registry")
	}

	base := map[string]any{
		"profile":          doc.ProfileName,
		"engine":           string(pack.Engine),
		"pack":             map[string]any{"id": pack.ID, "version": pack.Version, "status": pack.Status},
		"supported":        pack.Supported,
		"layer_order":      packLayerValuesFromMetadata(pack),
		"skill_agent_map":  packSkillMappingsFromTemplate(pack.SkillAgentMap),
		"persistence_mode": "hybrid",
		"mcp_placeholders": map[string]any{
			"engram":   map[string]any{"enabled": false, "mode": "placeholder", "required": false},
			"context7": map[string]any{"enabled": false, "mode": "placeholder", "required": false},
		},
		"sdd_handoff": map[string]any{
			"enabled":        true,
			"mode":           "phase-driven",
			"bootstrap_stub": "marker-driven",
		},
	}

	tests := []struct {
		name       string
		mutate     func(map[string]any)
		expectedAt string
	}{
		{
			name: "unsupported flag mismatch is rejected",
			mutate: func(fixture map[string]any) {
				fixture["supported"] = !pack.Supported
			},
			expectedAt: "supported must match registry metadata",
		},
		{
			name: "sdd handoff mode mismatch is rejected",
			mutate: func(fixture map[string]any) {
				handoff := fixture["sdd_handoff"].(map[string]any)
				handoff["mode"] = "sdd-bootstrap"
			},
			expectedAt: "sdd_handoff.mode must be \"phase-driven\"",
		},
		{
			name: "missing mandatory top-level property is rejected",
			mutate: func(fixture map[string]any) {
				delete(fixture, "skill_agent_map")
			},
			expectedAt: "missing required field \"skill_agent_map\"",
		},
		{
			name: "unexpected top-level property is rejected",
			mutate: func(fixture map[string]any) {
				fixture["legacy_flag"] = "stale"
			},
			expectedAt: "unexpected field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := mapClone(base)
			tt.mutate(fixture)

			raw, err := json.Marshal(fixture)
			if err != nil {
				t.Fatalf("marshal fixture: %v", err)
			}

			if err := validatePackConfigSkeleton(raw, pack, doc, "hybrid"); err == nil {
				t.Fatalf("expected validation error for %s", tt.name)
			} else if !strings.Contains(err.Error(), tt.expectedAt) {
				t.Fatalf("expected %q in error for %s, got %v", tt.expectedAt, tt.name, err)
			}
		})
	}
}

func TestValidateTextSkeleton_RejectsSemanticallyWrongLayout(t *testing.T) {
	bad := "# Generated Layout Notes\n\nDefault generator output base: .game-studio/generated\n"
	if err := validateTextSkeleton("README.layout.md", []byte(bad)); err == nil {
		t.Fatalf("expected validateTextSkeleton to reject stale layout text")
	}
}

func TestEnsureWorkspaceManifest_CreatesManifestWhenMissing(t *testing.T) {
	root := t.TempDir()
	doc := sampleWorkdoc()

	path, action, err := ensureWorkspaceManifest(root, doc, "hybrid")
	if err != nil {
		t.Fatalf("expected manifest to be created, got %v", err)
	}
	if action != "created" {
		t.Fatalf("expected action 'created', got %q", action)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read created manifest: %v", err)
	}

	var created workspaceManifest
	if err := json.Unmarshal(raw, &created); err != nil {
		t.Fatalf("parse created manifest: %v", err)
	}

	expected := validWorkspaceManifest(doc, "hybrid")
	created.GeneratedAt = expected.GeneratedAt
	if !reflect.DeepEqual(created, expected) {
		t.Fatalf("unexpected created manifest, got %#v", created)
	}
	if _, err := time.Parse(time.RFC3339, created.GeneratedAt); err != nil {
		t.Fatalf("generated_at must be RFC3339, got %q: %v", created.GeneratedAt, err)
	}
}

func sampleWorkdoc() workdoc.Document {
	return workdoc.Document{
		ProfileName:     "Game-Studio",
		PrimaryEngine:   "godot",
		Platform:        "Pi only",
		PersistenceMode: "hybrid",
	}
}

func mapClone(in map[string]any) map[string]any {
	raw, err := json.Marshal(in)
	if err != nil {
		return map[string]any{}
	}

	out := make(map[string]any)
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{}
	}

	return out
}

func packLayerValuesFromMetadata(pack templates.PackMetadata) []any {
	raw, err := json.Marshal(pack.LayerOrder)
	if err != nil {
		return []any{}
	}

	layers := make([]string, 0)
	if err := json.Unmarshal(raw, &layers); err != nil {
		return []any{}
	}

	out := make([]any, 0, len(layers))
	for _, layer := range layers {
		out = append(out, layer)
	}

	return out
}

func packSkillMappingsFromTemplate(bindings any) []any {
	raw, err := json.Marshal(bindings)
	if err != nil {
		return []any{}
	}

	out := make([]any, 0)
	if err := json.Unmarshal(raw, &out); err != nil {
		return []any{}
	}

	return out
}

func validWorkspaceManifest(doc workdoc.Document, persistenceMode string) workspaceManifest {
	return workspaceManifest{
		ProfileName:      doc.ProfileName,
		Platform:         doc.Platform,
		PrimaryEngine:    doc.PrimaryEngine,
		PersistenceMode:  persistenceMode,
		SupportedPacks:   []string{"godot"},
		PlaceholderPacks: []string{"unity", "ue5"},
		GeneratedAt:      time.Now().UTC().Format(time.RFC3339),
	}
}

func writeWorkspaceManifest(path string, manifest workspaceManifest) error {
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

// profileConfigRawWithMCP serializes a valid profile config whose only variable
// part is the raw mcp_placeholders subtree, so validation sees the exact JSON.
func profileConfigRawWithMCP(t *testing.T, placeholders any) []byte {
	t.Helper()
	doc := sampleWorkdoc()
	raw, err := json.Marshal(map[string]any{
		"profile_name":      doc.ProfileName,
		"platform":          doc.Platform,
		"primary_engine":    doc.PrimaryEngine,
		"persistence_mode":  "hybrid",
		"generated_by":      "game-studio init",
		"sdd_handoff":       true,
		"ccgs_preservation": "layered-hybrid",
		"mcp_placeholders":  placeholders,
	})
	if err != nil {
		t.Fatalf("marshal profile fixture: %v", err)
	}
	return raw
}

// packConfigRawWithMCP serializes a valid pack config whose only variable part
// is the raw mcp_placeholders subtree.
func packConfigRawWithMCP(t *testing.T, placeholders any) ([]byte, templates.PackMetadata) {
	t.Helper()
	pack, ok := templates.NewRegistry().GetPack(templates.EngineGodot)
	if !ok {
		t.Fatal("expected godot pack metadata")
	}
	raw, err := json.Marshal(map[string]any{
		"profile":          sampleWorkdoc().ProfileName,
		"engine":           string(pack.Engine),
		"pack":             map[string]any{"id": pack.ID, "version": pack.Version, "status": pack.Status},
		"supported":        pack.Supported,
		"layer_order":      packLayerValuesFromMetadata(pack),
		"skill_agent_map":  packSkillMappingsFromTemplate(pack.SkillAgentMap),
		"persistence_mode": "hybrid",
		"mcp_placeholders": placeholders,
		"sdd_handoff":      map[string]any{"enabled": true, "mode": "phase-driven", "bootstrap_stub": "marker-driven"},
	})
	if err != nil {
		t.Fatalf("marshal pack fixture: %v", err)
	}
	return raw, pack
}

func assertSkeletonValidation(t *testing.T, err error, wantErr string) {
	t.Helper()
	if wantErr == "" {
		if err != nil {
			t.Fatalf("expected fixture to validate, got %v", err)
		}
		return
	}
	if err == nil {
		t.Fatalf("expected validation error containing %q", wantErr)
	}
	if !strings.Contains(err.Error(), wantErr) {
		t.Fatalf("expected error containing %q, got %v", wantErr, err)
	}
}

// TestConfigSkeletonValidatorsSeeRawMCPSubtree guards F1: both entrypoints must
// validate the original raw JSON subtree so unknown entry fields, missing fields,
// null values, and wrong types cannot be masked by a lossy typed decode.
func TestConfigSkeletonValidatorsSeeRawMCPSubtree(t *testing.T) {
	disabled := func() map[string]any {
		return map[string]any{"enabled": false, "mode": "placeholder", "required": false}
	}

	cases := []struct {
		name    string
		value   any
		wantErr string
	}{
		{name: "disabled adapters accepted", value: map[string]any{"engram": disabled(), "context7": disabled()}},
		{name: "empty map accepted", value: map[string]any{}},
		{name: "subset accepted", value: map[string]any{"context7": disabled()}},
		{name: "null rejected", value: nil, wantErr: "must be an object"},
		{name: "string rejected", value: "bogus", wantErr: "must be an object"},
		{name: "unknown adapter rejected", value: map[string]any{"browser": disabled()}, wantErr: "unexpected field"},
		{name: "null entry rejected", value: map[string]any{"engram": nil}, wantErr: "must be an object"},
		{name: "string entry rejected", value: map[string]any{"engram": "bogus"}, wantErr: "must be an object"},
		{name: "extra entry field rejected", value: map[string]any{"engram": map[string]any{"enabled": false, "mode": "placeholder", "required": false, "extra": true}}, wantErr: "unexpected field"},
		{name: "missing entry field rejected", value: map[string]any{"engram": map[string]any{"enabled": false, "required": false}}, wantErr: "missing required field"},
		{name: "enabled true rejected", value: map[string]any{"engram": map[string]any{"enabled": true, "mode": "placeholder", "required": false}}, wantErr: "enabled must be false"},
		{name: "enabled wrong type rejected", value: map[string]any{"engram": map[string]any{"enabled": "false", "mode": "placeholder", "required": false}}, wantErr: "enabled must be boolean"},
		{name: "mode wrong type rejected", value: map[string]any{"engram": map[string]any{"enabled": false, "mode": true, "required": false}}, wantErr: "mode must be string"},
		{name: "required wrong type rejected", value: map[string]any{"engram": map[string]any{"enabled": false, "mode": "placeholder", "required": "no"}}, wantErr: "required must be boolean"},
		{name: "required true rejected", value: map[string]any{"engram": map[string]any{"enabled": false, "mode": "placeholder", "required": true}}, wantErr: "optional and never required"},
	}

	for _, tt := range cases {
		t.Run("profile/"+tt.name, func(t *testing.T) {
			assertSkeletonValidation(t, validateProfileConfigSkeleton(profileConfigRawWithMCP(t, tt.value), sampleWorkdoc(), "hybrid"), tt.wantErr)
		})
		t.Run("pack/"+tt.name, func(t *testing.T) {
			raw, pack := packConfigRawWithMCP(t, tt.value)
			assertSkeletonValidation(t, validatePackConfigSkeleton(raw, pack, sampleWorkdoc(), "hybrid"), tt.wantErr)
		})
	}
}

// TestRunInit_PreservesInvalidExistingConfigWithoutFalseVerified guards F1: an
// existing config that fails raw-subtree validation must be refused, left
// byte-for-byte unchanged, and never reported as verified. Init performs no
// memory write, so a disabled Engram placeholder is sufficient and no real
// Engram is touched.
func TestRunInit_PreservesInvalidExistingConfigWithoutFalseVerified(t *testing.T) {
	root := t.TempDir()
	docPath := filepath.Join(root, "GAME-STUDIO.md")
	writeWorkdocFixture(t, docPath, "godot")

	configPath := filepath.Join(root, "profiles", "game-studio", "profile.config.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("create config dir: %v", err)
	}
	invalid := map[string]any{
		"profile_name":      "Game-Studio",
		"platform":          "Pi only",
		"primary_engine":    "godot",
		"persistence_mode":  "hybrid",
		"generated_by":      "game-studio init",
		"sdd_handoff":       true,
		"ccgs_preservation": "layered-hybrid",
		"mcp_placeholders": map[string]any{
			// enabled=true is a raw-subtree violation the typed decode would hide.
			"engram": map[string]any{"enabled": true, "mode": "placeholder", "required": false},
		},
	}
	raw, err := json.MarshalIndent(invalid, "", "  ")
	if err != nil {
		t.Fatalf("marshal invalid config: %v", err)
	}
	if err := os.WriteFile(configPath, raw, 0o644); err != nil {
		t.Fatalf("write invalid config: %v", err)
	}

	err = RunInit(InitInput{
		Args:        []string{"--root", root, "--doc", docPath},
		Persistence: persistence.NewPlaceholder().WithEngramEnabled(false),
	})
	if err == nil {
		t.Fatal("expected init to reject the invalid existing config")
	}
	if !strings.Contains(err.Error(), "enabled must be false") {
		t.Fatalf("expected raw-subtree validation error, got %v", err)
	}

	after, readErr := os.ReadFile(configPath)
	if readErr != nil {
		t.Fatalf("read preserved config: %v", readErr)
	}
	if string(after) != string(raw) {
		t.Fatal("invalid existing config must not be overwritten by init")
	}
}
