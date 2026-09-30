package commands

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"open-game-studios/internal/persistence"
	"open-game-studios/internal/templates"
	"open-game-studios/internal/toolcheck"
	"open-game-studios/internal/workdoc"
)

type InitInput struct {
	Args         []string
	Persistence  persistence.Placeholder
	EnvChecker   EnvironmentChecker
	ToolRegistry *toolcheck.Registry
}

func RunInit(input InitInput) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	root := fs.String("root", ".", "workspace root path")
	docPath := fs.String("doc", "GAME-STUDIO.md", "path to Game-Studio working document")
	if err := fs.Parse(input.Args); err != nil {
		return err
	}

	doc, err := workdoc.LoadAndValidate(*docPath)
	if err != nil {
		return fmt.Errorf("init validation failed: %w", err)
	}

	if input.EnvChecker != nil {
		results := input.EnvChecker.Execute(context.Background(), toolcheck.SelectTools(initRequiredToolIDs(doc, input.ToolRegistry)...))
		printEnvCheckResults("init preflight", results)
		if err := toolcheck.BlockingError(results); err != nil {
			return err
		}
	}

	report, err := ensureWorkspaceStructure(*root)
	if err != nil {
		return err
	}

	manifestPath, manifestAction, err := ensureWorkspaceManifest(*root, doc, input.Persistence.Mode())
	if err != nil {
		return err
	}

	seedReport, err := ensureSeedSkeletonFiles(*root, doc, input.Persistence.Mode())
	if err != nil {
		return err
	}

	fmt.Println("[init] Game-Studio workspace initialized")
	fmt.Printf("[init] profile: %s\n", doc.ProfileName)
	fmt.Printf("[init] primary engine: %s\n", doc.PrimaryEngine)
	fmt.Printf("[init] persistence mode: %s\n", input.Persistence.Mode())
	fmt.Printf("[init] validated doc: %s\n", doc.Path)
	fmt.Printf("[init] directories created: %d, verified existing: %d\n", report.Created, report.Existing)
	fmt.Printf("[init] workspace manifest: %s (%s)\n", manifestPath, manifestAction)
	fmt.Printf("[init] skeleton files created: %d, verified existing: %d\n", seedReport.Created, seedReport.Verified)

	if report.Created > 0 {
		fmt.Println("[init] created paths:")
		for _, path := range report.CreatedPaths {
			fmt.Printf("  - %s\n", path)
		}
	}
	if seedReport.Created > 0 {
		fmt.Println("[init] seeded files:")
		for _, path := range seedReport.CreatedPaths {
			fmt.Printf("  - %s\n", path)
		}
	}

	return nil
}

func initRequiredToolIDs(doc workdoc.Document, registry *toolcheck.Registry) []string {
	return requiredToolIDsFromResources(registry, doc.PrimaryEngine)
}

type workspaceManifest struct {
	ProfileName      string   `json:"profile_name"`
	Platform         string   `json:"platform"`
	PrimaryEngine    string   `json:"primary_engine"`
	PersistenceMode  string   `json:"persistence_mode"`
	SupportedPacks   []string `json:"supported_packs"`
	PlaceholderPacks []string `json:"placeholder_packs"`
	GeneratedAt      string   `json:"generated_at"`
}

type seedFileReport struct {
	Created      int
	Verified     int
	CreatedPaths []string
}

func ensureSeedSkeletonFiles(root string, doc workdoc.Document, persistenceMode string) (seedFileReport, error) {
	registry := templates.NewRegistry()
	validateProfileConfig := func(raw []byte) error {
		return validateProfileConfigSkeleton(raw, doc, persistenceMode)
	}

	profileConfig := map[string]any{
		"profile_name":      doc.ProfileName,
		"platform":          doc.Platform,
		"persistence_mode":  persistenceMode,
		"primary_engine":    doc.PrimaryEngine,
		"generated_by":      "game-studio init",
		"sdd_handoff":       true,
		"ccgs_preservation": "layered-hybrid",
		"mcp_placeholders": map[string]any{
			// Optional MCP adapters only. Memory uses the Pi-native Engram
			// companion; no MCP adapter is required or implicitly enabled.
			"engram":   map[string]any{"enabled": false, "mode": "placeholder", "required": false},
			"context7": map[string]any{"enabled": false, "mode": "placeholder", "required": false},
		},
	}

	report := seedFileReport{}
	if err := ensureJSONFile(filepath.Join(root, "profiles", "game-studio", "profile.config.json"), profileConfig, validateProfileConfig, &report); err != nil {
		return seedFileReport{}, err
	}

	readme := `# Generated Layout Notes

Default generator output base: profiles/game-studio/generated

- layout=flat
  - studio-profile.godot.md
  - studio-profile.godot.summary.md
  - pack.godot.hybrid-map.json
  - pack.godot.config.json

- layout=pack
  - packs/godot/profile.md
  - packs/godot/summary.md
  - packs/godot/hybrid-map.json
  - packs/godot/pack.config.json

Use:
- game-studio generate --layout flat
- game-studio generate --layout pack
- game-studio smoke --layout flat
- game-studio smoke-suite --layout flat

Smoke suite report:
- profiles/game-studio/generated/smoke-suite.report.md
- profiles/game-studio/generated/sdd-handoff-smoke.report.md
`
	if err := ensureTextFile(filepath.Join(root, "profiles", "game-studio", "generated", "README.layout.md"), readme, &report); err != nil {
		return seedFileReport{}, err
	}

	engines := []templates.Engine{templates.EngineGodot, templates.EngineUnity, templates.EngineUE5}
	for _, engine := range engines {
		pack, ok := registry.GetPack(engine)
		if !ok {
			continue
		}

		describePackConfig := func(raw []byte) error {
			return validatePackConfigSkeleton(raw, pack, doc, persistenceMode)
		}

		packConfig := map[string]any{
			"profile": doc.ProfileName,
			"engine":  pack.Engine,
			"pack": map[string]any{
				"id":      pack.ID,
				"version": pack.Version,
				"status":  pack.Status,
			},
			"supported":        pack.Supported,
			"layer_order":      pack.LayerOrder,
			"skill_agent_map":  pack.SkillAgentMap,
			"persistence_mode": persistenceMode,
			"sdd_handoff": map[string]any{
				"enabled":        true,
				"mode":           "phase-driven",
				"bootstrap_stub": "marker-driven",
			},
			"mcp_placeholders": map[string]any{
				"engram":   map[string]any{"enabled": false, "mode": "placeholder", "required": false},
				"context7": map[string]any{"enabled": false, "mode": "placeholder", "required": false},
			},
		}

		path := filepath.Join(root, "profiles", "game-studio", "packs", string(engine), "pack.config.json")
		if err := ensureJSONFile(path, packConfig, describePackConfig, &report); err != nil {
			return seedFileReport{}, err
		}
	}

	return report, nil
}

var expectedReadmeLayoutTokens = []string{
	"Default generator output base: profiles/game-studio/generated",
	"- layout=flat",
	"- game-studio smoke --layout flat",
	"- game-studio smoke-suite --layout flat",
	"profiles/game-studio/generated/smoke-suite.report.md",
	"profiles/game-studio/generated/sdd-handoff-smoke.report.md",
}

func ensureJSONFile(path string, payload any, validate func([]byte) error, report *seedFileReport) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create skeleton directory %s: %w", filepath.Dir(path), err)
	}

	if _, err := os.Stat(path); err == nil {
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("read skeleton %s: %w", path, readErr)
		}
		if validate == nil {
			var existing any
			if err := json.Unmarshal(raw, &existing); err != nil {
				return fmt.Errorf("validate skeleton %s: %w", path, err)
			}
		} else {
			if err := validate(raw); err != nil {
				return fmt.Errorf("validate skeleton %s: %w", path, err)
			}
		}
		report.Verified++
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("verify skeleton %s: %w", path, err)
	}

	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal skeleton %s: %w", path, err)
	}

	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return fmt.Errorf("write skeleton %s: %w", path, err)
	}

	report.Created++
	report.CreatedPaths = append(report.CreatedPaths, path)
	return nil
}

func ensureTextFile(path, content string, report *seedFileReport) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create text directory %s: %w", filepath.Dir(path), err)
	}

	if _, err := os.Stat(path); err == nil {
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("read text %s: %w", path, readErr)
		}
		if strings.TrimSpace(string(raw)) == "" {
			return fmt.Errorf("text file %s exists but empty", path)
		}
		if err := validateTextSkeleton(path, raw); err != nil {
			return fmt.Errorf("validate text skeleton %s: %w", path, err)
		}
		report.Verified++
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("verify text %s: %w", path, err)
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write text %s: %w", path, err)
	}

	report.Created++
	report.CreatedPaths = append(report.CreatedPaths, path)
	return nil
}

func validateProfileConfigSkeleton(raw []byte, doc workdoc.Document, persistenceMode string) error {
	type payload struct {
		ProfileName      string `json:"profile_name"`
		Platform         string `json:"platform"`
		PrimaryEngine    string `json:"primary_engine"`
		PersistenceMode  string `json:"persistence_mode"`
		GeneratedBy      string `json:"generated_by"`
		SddHandoff       bool   `json:"sdd_handoff"`
		CCGSPreservation string `json:"ccgs_preservation"`
	}

	var fixture payload
	var rawObject map[string]any
	if err := json.Unmarshal(raw, &fixture); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}
	if err := json.Unmarshal(raw, &rawObject); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}
	if err := validateExpectedObjectKeys(rawObject, []string{"profile_name", "platform", "primary_engine", "persistence_mode", "generated_by", "sdd_handoff", "ccgs_preservation", "mcp_placeholders"}, "profile config"); err != nil {
		return err
	}

	if strings.TrimSpace(fixture.ProfileName) == "" {
		return fmt.Errorf("profile_name must be non-empty")
	}
	if strings.TrimSpace(fixture.Platform) == "" {
		return fmt.Errorf("platform must be non-empty")
	}
	if strings.TrimSpace(fixture.PrimaryEngine) == "" {
		return fmt.Errorf("primary_engine must be non-empty")
	}
	if strings.TrimSpace(fixture.PersistenceMode) == "" {
		return fmt.Errorf("persistence_mode must be non-empty")
	}
	if strings.TrimSpace(fixture.GeneratedBy) == "" {
		return fmt.Errorf("generated_by must be non-empty")
	}
	if strings.TrimSpace(fixture.ProfileName) != strings.TrimSpace(doc.ProfileName) {
		return fmt.Errorf("profile_name must match document profile %q", doc.ProfileName)
	}
	if strings.TrimSpace(fixture.Platform) != strings.TrimSpace(doc.Platform) {
		return fmt.Errorf("platform must match document platform %q", doc.Platform)
	}
	if strings.TrimSpace(fixture.PrimaryEngine) != strings.TrimSpace(doc.PrimaryEngine) {
		return fmt.Errorf("primary_engine must match document primary engine %q", doc.PrimaryEngine)
	}
	if strings.TrimSpace(fixture.PersistenceMode) != strings.TrimSpace(persistenceMode) {
		return fmt.Errorf("persistence_mode must match init persistence mode %q", persistenceMode)
	}
	if strings.TrimSpace(fixture.GeneratedBy) != "game-studio init" {
		return fmt.Errorf("generated_by must be \"game-studio init\"")
	}
	if !fixture.SddHandoff {
		return fmt.Errorf("sdd_handoff must be true")
	}
	if strings.TrimSpace(fixture.CCGSPreservation) == "" {
		return fmt.Errorf("ccgs_preservation must be non-empty")
	}
	if fixture.CCGSPreservation != "layered-hybrid" {
		return fmt.Errorf("ccgs_preservation must be \"layered-hybrid\"")
	}

	// Validate the raw JSON subtree, not the typed fixture, so unknown entry
	// fields and null/type distinctions are not lost before validation.
	if err := validateMCPPlaceholdersMap(rawObject["mcp_placeholders"], "mcp_placeholders"); err != nil {
		return err
	}

	return nil
}

func validatePackConfigSkeleton(raw []byte, pack templates.PackMetadata, doc workdoc.Document, persistenceMode string) error {
	type packMeta struct {
		ID      string `json:"id"`
		Version string `json:"version"`
		Status  string `json:"status"`
	}

	type skillBinding struct {
		Skill string `json:"skill"`
		Agent string `json:"agent"`
		Role  string `json:"role"`
	}

	type handoff struct {
		Enabled       bool   `json:"enabled"`
		Mode          string `json:"mode"`
		BootstrapStub string `json:"bootstrap_stub"`
	}

	type payload struct {
		Profile         string         `json:"profile"`
		Engine          string         `json:"engine"`
		Supported       bool           `json:"supported"`
		Pack            packMeta       `json:"pack"`
		PersistenceMode string         `json:"persistence_mode"`
		LayerOrder      []string       `json:"layer_order"`
		SkillAgentMap   []skillBinding `json:"skill_agent_map"`
		Handoff         handoff        `json:"sdd_handoff"`
	}

	var fixture payload
	var rawObject map[string]any
	if err := json.Unmarshal(raw, &fixture); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}
	if err := json.Unmarshal(raw, &rawObject); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}
	if err := validateExpectedObjectKeys(rawObject, []string{"profile", "engine", "pack", "supported", "layer_order", "skill_agent_map", "mcp_placeholders", "sdd_handoff", "persistence_mode"}, "pack config"); err != nil {
		return err
	}
	packObject, ok := rawObject["pack"].(map[string]any)
	if !ok {
		return fmt.Errorf("pack must be object")
	}
	if err := validateExpectedObjectKeys(packObject, []string{"id", "version", "status"}, "pack object"); err != nil {
		return err
	}
	handoffObject, ok := rawObject["sdd_handoff"].(map[string]any)
	if !ok {
		return fmt.Errorf("sdd_handoff must be object")
	}
	if err := validateExpectedObjectKeys(handoffObject, []string{"enabled", "mode", "bootstrap_stub"}, "sdd_handoff"); err != nil {
		return err
	}

	if strings.TrimSpace(fixture.Profile) == "" {
		return fmt.Errorf("profile must be non-empty")
	}
	if strings.TrimSpace(fixture.Profile) != strings.TrimSpace(doc.ProfileName) {
		return fmt.Errorf("profile must match document profile %q", doc.ProfileName)
	}
	if strings.TrimSpace(fixture.Engine) == "" {
		return fmt.Errorf("engine must be non-empty")
	}
	if strings.TrimSpace(fixture.Engine) != strings.TrimSpace(string(pack.Engine)) {
		return fmt.Errorf("engine must match template metadata %q", pack.Engine)
	}
	if strings.TrimSpace(fixture.Pack.ID) == "" {
		return fmt.Errorf("pack.id must be non-empty")
	}
	if fixture.Pack.ID != pack.ID {
		return fmt.Errorf("pack.id must match registry metadata")
	}
	if strings.TrimSpace(fixture.Pack.Version) == "" {
		return fmt.Errorf("pack.version must be non-empty")
	}
	if fixture.Pack.Version != pack.Version {
		return fmt.Errorf("pack.version must match registry metadata")
	}
	if strings.TrimSpace(fixture.Pack.Status) == "" {
		return fmt.Errorf("pack.status must be non-empty")
	}
	if fixture.Pack.Status != pack.Status {
		return fmt.Errorf("pack.status must match registry metadata")
	}
	if fixture.Supported != pack.Supported {
		return fmt.Errorf("supported must match registry metadata")
	}
	if strings.TrimSpace(fixture.PersistenceMode) == "" {
		return fmt.Errorf("persistence_mode must be non-empty")
	}
	if fixture.PersistenceMode != persistenceMode {
		return fmt.Errorf("persistence_mode must match init persistence mode %q", persistenceMode)
	}
	if len(fixture.LayerOrder) == 0 {
		return fmt.Errorf("layer_order must be non-empty")
	}
	if len(fixture.LayerOrder) != len(pack.LayerOrder) {
		return fmt.Errorf("layer_order count mismatch")
	}
	for i, layer := range pack.LayerOrder {
		if strings.TrimSpace(fixture.LayerOrder[i]) != strings.TrimSpace(string(layer)) {
			return fmt.Errorf("layer_order mismatch")
		}
	}
	if len(pack.SkillAgentMap) != len(fixture.SkillAgentMap) {
		return fmt.Errorf("skill_agent_map length mismatch")
	}
	for i, expected := range pack.SkillAgentMap {
		actual := fixture.SkillAgentMap[i]
		if strings.TrimSpace(actual.Skill) != expected.Skill || strings.TrimSpace(actual.Agent) != expected.Agent || strings.TrimSpace(actual.Role) != expected.Role {
			return fmt.Errorf("skill_agent_map mismatch at index %d", i)
		}
	}
	for _, binding := range fixture.SkillAgentMap {
		if strings.TrimSpace(binding.Skill) == "" || strings.TrimSpace(binding.Agent) == "" || strings.TrimSpace(binding.Role) == "" {
			return fmt.Errorf("skill_agent_map entries must include skill, agent and role")
		}
	}
	// Validate the raw JSON subtree, not the typed fixture, so unknown entry
	// fields and null/type distinctions are not lost before validation.
	if err := validateMCPPlaceholdersMap(rawObject["mcp_placeholders"], "mcp_placeholders"); err != nil {
		return err
	}
	if !fixture.Handoff.Enabled {
		return fmt.Errorf("sdd_handoff.enabled must be true")
	}
	if strings.TrimSpace(fixture.Handoff.Mode) == "" {
		return fmt.Errorf("sdd_handoff.mode must be non-empty")
	}
	if fixture.Handoff.Mode != "phase-driven" {
		return fmt.Errorf("sdd_handoff.mode must be \"phase-driven\"")
	}
	if strings.TrimSpace(fixture.Handoff.BootstrapStub) == "" {
		return fmt.Errorf("sdd_handoff.bootstrap_stub must be non-empty")
	}
	if fixture.Handoff.BootstrapStub != "marker-driven" {
		return fmt.Errorf("sdd_handoff.bootstrap_stub must be \"marker-driven\"")
	}

	return nil
}

// validateMCPPlaceholdersMap validates the raw `mcp_placeholders` JSON subtree.
// It intentionally receives the original `map[string]any` value instead of a
// typed struct so unknown entry fields and type/null distinctions survive to
// validation. The map may be empty or list a subset of the known optional
// adapters; every present entry must be exactly {enabled:false,
// mode:"placeholder", required:false}. Unknown adapter keys, unknown or missing
// entry fields, null values, bad types, and enabled/required true are rejected.
func validateMCPPlaceholdersMap(values any, context string) error {
	placeholders, ok := values.(map[string]any)
	if !ok {
		return fmt.Errorf("%s must be an object", context)
	}

	if err := validateAllowedObjectKeys(placeholders, []string{"engram", "context7"}, context+" keys"); err != nil {
		return err
	}

	for name, rawEntry := range placeholders {
		entry, ok := rawEntry.(map[string]any)
		if !ok {
			return fmt.Errorf("%s.%s must be an object", context, name)
		}
		if err := validateExpectedObjectKeys(entry, []string{"enabled", "mode", "required"}, context+"."+name); err != nil {
			return err
		}

		enabled, ok := entry["enabled"].(bool)
		if !ok {
			return fmt.Errorf("%s.%s.enabled must be boolean", context, name)
		}
		if enabled {
			return fmt.Errorf("%s.%s.enabled must be false; MCP adapters are optional and not implicitly enabled", context, name)
		}
		mode, ok := entry["mode"].(string)
		if !ok {
			return fmt.Errorf("%s.%s.mode must be string", context, name)
		}
		if mode != "placeholder" {
			return fmt.Errorf("%s.%s.mode must be \"placeholder\"", context, name)
		}
		required, ok := entry["required"].(bool)
		if !ok {
			return fmt.Errorf("%s.%s.required must be boolean", context, name)
		}
		if required {
			return fmt.Errorf("%s.%s.required must be false; MCP adapters are optional and never required", context, name)
		}
	}

	return nil
}

// validateAllowedObjectKeys rejects unknown keys while allowing any subset of
// the allowed keys to be absent. It complements validateExpectedObjectKeys,
// which also requires every allowed key to be present.
func validateAllowedObjectKeys(values map[string]any, allowed []string, context string) error {
	permitted := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		permitted[key] = struct{}{}
	}
	for key := range values {
		if _, ok := permitted[key]; !ok {
			return fmt.Errorf("%s has unexpected field %q", context, key)
		}
	}
	return nil
}

func validateExpectedObjectKeys(values map[string]any, expected []string, context string) error {
	required := make(map[string]struct{}, len(expected))
	for _, key := range expected {
		required[key] = struct{}{}
	}

	for key := range values {
		if _, ok := required[key]; !ok {
			return fmt.Errorf("%s has unexpected field %q", context, key)
		}
	}
	for _, key := range expected {
		if _, ok := values[key]; !ok {
			return fmt.Errorf("%s missing required field %q", context, key)
		}
	}

	return nil
}

func validateTextSkeleton(path string, raw []byte) error {
	base := filepath.Base(path)
	switch base {
	case "README.layout.md":
		text := string(raw)
		for _, token := range expectedReadmeLayoutTokens {
			if !strings.Contains(text, token) {
				return fmt.Errorf("README.layout.md missing required content %q", token)
			}
		}
	}

	return nil
}

func ensureWorkspaceManifest(root string, doc workdoc.Document, persistenceMode string) (string, string, error) {
	manifestPath := filepath.Join(root, ".game-studio", "workspace.manifest.json")
	if _, err := os.Stat(manifestPath); err == nil {
		raw, readErr := os.ReadFile(manifestPath)
		if readErr != nil {
			return "", "", fmt.Errorf("read manifest %s: %w", manifestPath, readErr)
		}

		var existing workspaceManifest
		if unmarshalErr := json.Unmarshal(raw, &existing); unmarshalErr != nil {
			return "", "", fmt.Errorf("parse manifest %s: %w", manifestPath, unmarshalErr)
		}

		expected := workspaceManifestFromDocument(doc, persistenceMode)
		if err := validateWorkspaceManifest(manifestPath, existing, expected); err != nil {
			return "", "", err
		}

		return manifestPath, "verified", nil
	} else if !os.IsNotExist(err) {
		return "", "", fmt.Errorf("verify manifest %s: %w", manifestPath, err)
	}

	manifest := workspaceManifestFromDocument(doc, persistenceMode)
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		return "", "", fmt.Errorf("create manifest directory %s: %w", filepath.Dir(manifestPath), err)
	}

	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", "", fmt.Errorf("marshal workspace manifest: %w", err)
	}

	if err := os.WriteFile(manifestPath, raw, 0o644); err != nil {
		return "", "", fmt.Errorf("write manifest %s: %w", manifestPath, err)
	}

	return manifestPath, "created", nil
}

func workspaceManifestFromDocument(doc workdoc.Document, persistenceMode string) workspaceManifest {
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

func validateWorkspaceManifest(manifestPath string, existing workspaceManifest, expected workspaceManifest) error {
	var mismatch []string
	if existing.ProfileName != expected.ProfileName {
		mismatch = append(mismatch, fmt.Sprintf("profile_name: %q vs %q", existing.ProfileName, expected.ProfileName))
	}
	if existing.Platform != expected.Platform {
		mismatch = append(mismatch, fmt.Sprintf("platform: %q vs %q", existing.Platform, expected.Platform))
	}
	if existing.PrimaryEngine != expected.PrimaryEngine {
		mismatch = append(mismatch, fmt.Sprintf("primary_engine: %q vs %q", existing.PrimaryEngine, expected.PrimaryEngine))
	}
	if existing.PersistenceMode != expected.PersistenceMode {
		mismatch = append(mismatch, fmt.Sprintf("persistence_mode: %q vs %q", existing.PersistenceMode, expected.PersistenceMode))
	}
	if !reflect.DeepEqual(existing.SupportedPacks, expected.SupportedPacks) {
		mismatch = append(mismatch, fmt.Sprintf("supported_packs: %#v vs %#v", existing.SupportedPacks, expected.SupportedPacks))
	}
	if !reflect.DeepEqual(existing.PlaceholderPacks, expected.PlaceholderPacks) {
		mismatch = append(mismatch, fmt.Sprintf("placeholder_packs: %#v vs %#v", existing.PlaceholderPacks, expected.PlaceholderPacks))
	}
	if len(mismatch) > 0 {
		return fmt.Errorf("manifest mismatch in %s: %s", manifestPath, strings.Join(mismatch, "; "))
	}

	return nil
}

type structureReport struct {
	Created      int
	Existing     int
	CreatedPaths []string
}

func ensureWorkspaceStructure(root string) (structureReport, error) {
	expectedDirs := []string{
		".game-studio",
		".game-studio/state",
		".game-studio/generated",
		"profiles",
		"profiles/game-studio",
		"profiles/game-studio/packs",
		"profiles/game-studio/packs/godot",
		"profiles/game-studio/packs/unity",
		"profiles/game-studio/packs/ue5",
		"openspec/changes",
		"openspec/changes/archive",
		"openspec/specs",
	}

	report := structureReport{}
	for _, rel := range expectedDirs {
		path := filepath.Join(root, rel)
		if _, err := os.Stat(path); err == nil {
			report.Existing++
			continue
		} else if !os.IsNotExist(err) {
			return structureReport{}, fmt.Errorf("verify %s: %w", path, err)
		}

		if err := os.MkdirAll(path, 0o755); err != nil {
			return structureReport{}, fmt.Errorf("create %s: %w", path, err)
		}

		report.Created++
		report.CreatedPaths = append(report.CreatedPaths, path)
	}

	return report, nil
}
