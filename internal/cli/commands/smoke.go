package commands

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"open-game-studios/internal/templates"
	"open-game-studios/internal/workflows/modelrouting"
)

type SmokeInput struct {
	Args     []string
	Registry *templates.Registry
}

type SmokeSuiteInput struct {
	Args     []string
	Registry *templates.Registry
}

// Smoke expectations for the model-routing/v1 capability catalogs. These mirror
// the contract IDs so smoke rejects ambiguous optional/future classifications.
var smokeExpectedModelRoutingCapabilities = []string{
	string(modelrouting.CapabilityDefaultText),
	string(modelrouting.CapabilityReasoningHeavy),
	string(modelrouting.CapabilityFastText),
	string(modelrouting.CapabilityCodeApply),
	string(modelrouting.CapabilityQAReview),
	string(modelrouting.CapabilityArchiveSummary),
	string(modelrouting.CapabilityVisualReview),
	string(modelrouting.CapabilityGodotTechnicalReview),
	string(modelrouting.CapabilityImageGeneration),
	string(modelrouting.CapabilityImageReview),
	string(modelrouting.CapabilityAudioGeneration),
	string(modelrouting.CapabilityAudioReview),
}

var smokeExpectedOptionalModelRoutingCapabilities = []string{
	string(modelrouting.CapabilityArchiveSummary),
	string(modelrouting.CapabilityVisualReview),
	string(modelrouting.CapabilityGodotTechnicalReview),
}

var smokeExpectedFutureModelRoutingCapabilities = []string{
	string(modelrouting.CapabilityImageGeneration),
	string(modelrouting.CapabilityImageReview),
	string(modelrouting.CapabilityAudioGeneration),
	string(modelrouting.CapabilityAudioReview),
}

var smokeLegacyOptionalFutureCapabilitiesField = "optional_" + "future_" + "capabilities"

func RunSmoke(input SmokeInput) error {
	fs := flag.NewFlagSet("smoke", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	engine := fs.String("engine", string(templates.EngineGodot), "target engine (godot)")
	outDir := fs.String("out-dir", "profiles/game-studio/generated", "output base directory")
	layout := fs.String("layout", "flat", "artifact layout (flat|pack)")
	if err := fs.Parse(input.Args); err != nil {
		return err
	}

	targetEngine := templates.Engine(strings.ToLower(strings.TrimSpace(*engine)))
	if err := input.Registry.ValidatePackLoadable(targetEngine); err != nil {
		return fmt.Errorf("smoke pack validation failed: %w", err)
	}

	paths, err := resolveOutputPaths(targetEngine, "", *outDir, *layout)
	if err != nil {
		return err
	}

	if err := runSmokeContentChecks(paths); err != nil {
		return err
	}

	fmt.Println("[smoke] output layout validation ok")
	fmt.Printf("[smoke] engine: %s\n", targetEngine)
	fmt.Printf("[smoke] layout: %s\n", strings.ToLower(strings.TrimSpace(*layout)))
	fmt.Printf("[smoke] profile: %s\n", filepath.Clean(paths.profile))
	fmt.Printf("[smoke] summary: %s\n", filepath.Clean(paths.summary))
	fmt.Printf("[smoke] pattern: %s\n", filepath.Clean(paths.pattern))
	fmt.Printf("[smoke] config: %s\n", filepath.Clean(paths.config))

	return nil
}

func RunSmokeSuite(input SmokeSuiteInput) error {
	fs := flag.NewFlagSet("smoke-suite", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	engine := fs.String("engine", string(templates.EngineGodot), "target engine (godot)")
	outDir := fs.String("out-dir", "profiles/game-studio/generated", "output base directory")
	layout := fs.String("layout", "flat", "artifact layout (flat|pack)")
	reportPath := fs.String("report", "", "optional smoke suite report path")
	handoffReportPath := fs.String("handoff-report", "", "optional sdd handoff smoke report path")
	if err := fs.Parse(input.Args); err != nil {
		return err
	}

	targetEngine := templates.Engine(strings.ToLower(strings.TrimSpace(*engine)))
	if err := input.Registry.ValidatePackLoadable(targetEngine); err != nil {
		return fmt.Errorf("smoke-suite pack validation failed: %w", err)
	}

	if err := validatePlaceholderPack(input.Registry, templates.EngineUnity); err != nil {
		return err
	}
	if err := validatePlaceholderPack(input.Registry, templates.EngineUE5); err != nil {
		return err
	}

	paths, err := resolveOutputPaths(targetEngine, "", *outDir, *layout)
	if err != nil {
		return err
	}

	if err := runSmokeContentChecks(paths); err != nil {
		return err
	}
	if err := validateMCPEndToEnd(paths); err != nil {
		return err
	}
	if err := runSDDHandoffBootstrapSmoke(paths); err != nil {
		return err
	}

	report := *reportPath
	if strings.TrimSpace(report) == "" {
		report = filepath.Join(*outDir, "smoke-suite.report.md")
	}
	if err := writeSmokeSuiteReport(report, targetEngine, strings.ToLower(strings.TrimSpace(*layout)), paths); err != nil {
		return err
	}

	handoffReport := *handoffReportPath
	if strings.TrimSpace(handoffReport) == "" {
		handoffReport = filepath.Join(*outDir, "sdd-handoff-smoke.report.md")
	}
	if err := writeSDDHandoffSmokeReport(handoffReport, targetEngine, strings.ToLower(strings.TrimSpace(*layout)), paths); err != nil {
		return err
	}

	fmt.Println("[smoke-suite] validation ok")
	fmt.Printf("[smoke-suite] engine: %s\n", targetEngine)
	fmt.Printf("[smoke-suite] layout: %s\n", strings.ToLower(strings.TrimSpace(*layout)))
	fmt.Printf("[smoke-suite] report: %s\n", filepath.Clean(report))
	fmt.Printf("[smoke-suite] handoff report: %s\n", filepath.Clean(handoffReport))

	return nil
}

func runSmokeContentChecks(paths outputPaths) error {
	for _, p := range []string{paths.profile, paths.summary, paths.pattern, paths.config} {
		if _, err := os.Stat(p); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("smoke missing artifact: %s", p)
			}
			return fmt.Errorf("smoke cannot access artifact %s: %w", p, err)
		}
	}

	profileRaw, err := os.ReadFile(paths.profile)
	if err != nil {
		return fmt.Errorf("smoke read profile: %w", err)
	}
	profileText := string(profileRaw)
	for _, token := range []string{"## Pack Metadata", "## MCP Placeholders", "## Core Game Workflow Contract", "core-game-workflow/v1", "## Model Routing Contract", "model-routing/v1", "game-concept", "repair/change handoff", "target_engine: godot", "pack_status: active"} {
		if !strings.Contains(profileText, token) {
			return fmt.Errorf("smoke invalid profile artifact content: missing %q", token)
		}
	}

	summaryRaw, err := os.ReadFile(paths.summary)
	if err != nil {
		return fmt.Errorf("smoke read summary: %w", err)
	}
	summaryText := string(summaryRaw)
	for _, token := range []string{"## Artifact Set", "## Future Packs (Explicit Placeholders)", "## Core Game Workflow Metadata", "core-game-workflow/v1", "## Model Routing Metadata", "model-routing/v1", "gdd-slice", "change-brief", "repair-brief", "unity: placeholder", "ue5: placeholder"} {
		if !strings.Contains(summaryText, token) {
			return fmt.Errorf("smoke invalid summary artifact content: missing %q", token)
		}
	}

	if err := validateJSONArtifact(paths.pattern, []string{"pack", "setup_preset", "preset_source", "routing_preset", "routing_preset_source", "hybrid_mapping", "core_game_workflow", "model_routing", "mcp_placeholders", "sdd_handoff"}); err != nil {
		return err
	}
	if err := validatePatternArtifact(paths.pattern); err != nil {
		return err
	}
	if err := validateJSONArtifact(paths.config, []string{"pack", "setup_preset", "preset_source", "routing_preset", "routing_preset_source", "skill_agent_mapping", "core_game_workflow", "model_routing", "mcp_placeholders", "sdd_handoff"}); err != nil {
		return err
	}
	if err := validateConfigArtifact(paths.config); err != nil {
		return err
	}
	if err := validatePresetConsistency(paths); err != nil {
		return err
	}

	return nil
}

func validatePresetConsistency(paths outputPaths) error {
	config, err := readJSONMap(paths.config)
	if err != nil {
		return err
	}
	pattern, err := readJSONMap(paths.pattern)
	if err != nil {
		return err
	}

	setupPreset, _ := config["setup_preset"].(string)
	if err := validateSetupPreset(setupPreset); err != nil {
		return fmt.Errorf("smoke invalid setup preset metadata in %s: %w", paths.config, err)
	}
	presetSource, _ := config["preset_source"].(string)
	if presetSource != presetSourceExplicitCLI && presetSource != presetSourcePersistedWorkspace && presetSource != presetSourceDefault {
		return fmt.Errorf("smoke invalid preset_source %q in %s", presetSource, paths.config)
	}
	routingPreset, _ := config["routing_preset"].(string)
	if routingPreset != setupPreset {
		return fmt.Errorf("smoke preset mismatch in %s: setup_preset=%q routing_preset=%q", paths.config, setupPreset, routingPreset)
	}
	routingSource, _ := config["routing_preset_source"].(string)
	if routingSource != routingPresetSourceDerived {
		return fmt.Errorf("smoke invalid routing_preset_source %q in %s", routingSource, paths.config)
	}

	for path, payload := range map[string]map[string]any{paths.config: config, paths.pattern: pattern} {
		if payload["setup_preset"] != setupPreset || payload["preset_source"] != presetSource || payload["routing_preset"] != routingPreset || payload["routing_preset_source"] != routingSource {
			return fmt.Errorf("smoke preset provenance mismatch in %s", path)
		}
		modelRouting, ok := payload["model_routing"].(map[string]any)
		if !ok {
			return fmt.Errorf("smoke missing model_routing in %s", path)
		}
		if modelRouting["setup_preset"] != setupPreset || modelRouting["preset_source"] != presetSource || modelRouting["routing_preset"] != routingPreset || modelRouting["routing_preset_source"] != routingSource {
			return fmt.Errorf("smoke model routing preset provenance mismatch in %s", path)
		}
	}

	metadata := map[string]string{
		"setup_preset":          setupPreset,
		"preset_source":         presetSource,
		"routing_preset":        routingPreset,
		"routing_preset_source": routingSource,
	}
	for _, path := range []string{paths.profile, paths.summary} {
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("smoke read preset metadata %s: %w", path, readErr)
		}
		for key, expected := range metadata {
			value, count := markdownMetadataValue(string(raw), key)
			if count != 1 || value != expected {
				return fmt.Errorf("smoke preset provenance mismatch in %s: %s=%q occurrences=%d expected=%q", path, key, value, count, expected)
			}
		}
	}
	return nil
}

func markdownMetadataValue(content, key string) (string, int) {
	prefix := "- " + key + ":"
	value := ""
	count := 0
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, prefix) {
			continue
		}
		count++
		value = strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
	}
	return value, count
}

func readJSONMap(path string) (map[string]any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("smoke read json %s: %w", path, err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("smoke parse json %s: %w", path, err)
	}
	return payload, nil
}

func validateMCPEndToEnd(paths outputPaths) error {
	profileRaw, err := os.ReadFile(paths.profile)
	if err != nil {
		return fmt.Errorf("smoke read profile for mcp check: %w", err)
	}
	profileText := string(profileRaw)
	for _, token := range []string{"engram: enabled=false", "context7: enabled=false"} {
		if !strings.Contains(profileText, token) {
			return fmt.Errorf("smoke mcp mismatch in profile: missing %q", token)
		}
	}

	summaryRaw, err := os.ReadFile(paths.summary)
	if err != nil {
		return fmt.Errorf("smoke read summary for mcp check: %w", err)
	}
	summaryText := string(summaryRaw)
	for _, token := range []string{"engram: enabled=false, mode=placeholder, required=false", "context7: enabled=false, mode=placeholder, required=false"} {
		if !strings.Contains(summaryText, token) {
			return fmt.Errorf("smoke mcp mismatch in summary: missing %q", token)
		}
	}

	return nil
}

func runSDDHandoffBootstrapSmoke(paths outputPaths) error {
	patternRaw, err := os.ReadFile(paths.pattern)
	if err != nil {
		return fmt.Errorf("smoke handoff read pattern: %w", err)
	}
	configRaw, err := os.ReadFile(paths.config)
	if err != nil {
		return fmt.Errorf("smoke handoff read config: %w", err)
	}

	var pattern map[string]any
	if err := json.Unmarshal(patternRaw, &pattern); err != nil {
		return fmt.Errorf("smoke handoff parse pattern: %w", err)
	}
	var config map[string]any
	if err := json.Unmarshal(configRaw, &config); err != nil {
		return fmt.Errorf("smoke handoff parse config: %w", err)
	}

	patternHandoff, ok := pattern["sdd_handoff"].(map[string]any)
	if !ok {
		return fmt.Errorf("smoke handoff missing sdd_handoff in pattern")
	}
	configHandoff, ok := config["sdd_handoff"].(map[string]any)
	if !ok {
		return fmt.Errorf("smoke handoff missing sdd_handoff in config")
	}

	if err := validateHandoffMap(patternHandoff, true); err != nil {
		return fmt.Errorf("smoke handoff pattern invalid: %w", err)
	}
	if err := validateHandoffMap(configHandoff, false); err != nil {
		return fmt.Errorf("smoke handoff config invalid: %w", err)
	}

	patternMode, _ := patternHandoff["mode"].(string)
	configMode, _ := configHandoff["mode"].(string)
	if patternMode != configMode {
		return fmt.Errorf("smoke handoff mode mismatch pattern=%q config=%q", patternMode, configMode)
	}

	return nil
}

func validateHandoffMap(handoff map[string]any, requireBootstrapStub bool) error {
	enabled, ok := handoff["enabled"].(bool)
	if !ok || !enabled {
		return fmt.Errorf("enabled flag invalid")
	}
	mode, ok := handoff["mode"].(string)
	if !ok || mode != "phase-driven" {
		return fmt.Errorf("mode invalid")
	}
	if requireBootstrapStub {
		return nil
	}
	stub, ok := handoff["bootstrap_stub"].(string)
	if !ok || stub != "marker-driven" {
		return fmt.Errorf("bootstrap_stub invalid")
	}
	return nil
}

func validatePlaceholderPack(registry *templates.Registry, engine templates.Engine) error {
	pack, ok := registry.GetPack(engine)
	if !ok {
		return fmt.Errorf("smoke-suite missing placeholder pack metadata for %q", engine)
	}
	if pack.Supported {
		return fmt.Errorf("smoke-suite placeholder pack %q unexpectedly supported", engine)
	}
	if pack.Status != "placeholder" {
		return fmt.Errorf("smoke-suite placeholder pack %q invalid status %q", engine, pack.Status)
	}
	if pack.ProfileTemplateID == "" {
		return fmt.Errorf("smoke-suite placeholder pack %q missing profile template id", engine)
	}
	return nil
}

func writeSmokeSuiteReport(path string, engine templates.Engine, layout string, paths outputPaths) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("smoke-suite create report dir: %w", err)
	}

	report := fmt.Sprintf(`# Smoke Suite Report

status: PASS
engine: %s
layout: %s

## Validated Artifacts
- profile: %s
- summary: %s
- pattern: %s
- config: %s

## Checks
- registry pack loadable (godot)
- placeholder packs explicit (unity/ue5)
- profile content markers
- summary content markers
- pattern json + handoff markers
- config json + skill mapping markers
- core game workflow contract markers
- mcp placeholders end-to-end markers
- sdd handoff bootstrap markers
`, engine, layout, paths.profile, paths.summary, paths.pattern, paths.config)

	if err := os.WriteFile(path, []byte(report), 0o644); err != nil {
		return fmt.Errorf("smoke-suite write report: %w", err)
	}

	return nil
}

func writeSDDHandoffSmokeReport(path string, engine templates.Engine, layout string, paths outputPaths) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("smoke-suite create handoff report dir: %w", err)
	}

	report := fmt.Sprintf(`# SDD Handoff Smoke Report

status: PASS
engine: %s
layout: %s

## Inputs
- pattern: %s
- config: %s

## Verified
- pattern sdd_handoff enabled=true
- pattern sdd_handoff mode=phase-driven
- config sdd_handoff enabled=true
- config sdd_handoff mode=phase-driven
- config sdd_handoff bootstrap_stub=marker-driven
- pattern/config mode consistency
- core-game-workflow/v1 markers available for downstream handoff
`, engine, layout, paths.pattern, paths.config)

	if err := os.WriteFile(path, []byte(report), 0o644); err != nil {
		return fmt.Errorf("smoke-suite write handoff report: %w", err)
	}

	return nil
}

func validateJSONArtifact(path string, requiredKeys []string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("smoke read json %s: %w", path, err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("smoke parse json %s: %w", path, err)
	}

	for _, key := range requiredKeys {
		if _, ok := payload[key]; !ok {
			return fmt.Errorf("smoke missing key %q in %s", key, path)
		}
	}

	mcpRaw, ok := payload["mcp_placeholders"]
	if !ok {
		return fmt.Errorf("smoke missing mcp_placeholders in %s", path)
	}
	mcpMap, ok := mcpRaw.(map[string]any)
	if !ok {
		return fmt.Errorf("smoke invalid mcp_placeholders format in %s", path)
	}
	for _, mcp := range []string{"engram", "context7"} {
		entry, exists := mcpMap[mcp]
		if !exists {
			return fmt.Errorf("smoke missing mcp placeholder %q in %s", mcp, path)
		}
		entryMap, entryOK := entry.(map[string]any)
		if !entryOK {
			return fmt.Errorf("smoke invalid mcp placeholder %q format in %s", mcp, path)
		}
		enabled, hasEnabled := entryMap["enabled"].(bool)
		if !hasEnabled || enabled {
			return fmt.Errorf("smoke mcp placeholder %q must stay disabled by default in %s", mcp, path)
		}
		required, hasRequired := entryMap["required"].(bool)
		if !hasRequired || required {
			return fmt.Errorf("smoke mcp placeholder %q must not be required in %s", mcp, path)
		}
		mode, hasMode := entryMap["mode"].(string)
		if !hasMode || mode != "placeholder" {
			return fmt.Errorf("smoke invalid mcp placeholder %q mode in %s", mcp, path)
		}
	}

	return nil
}

func validatePatternArtifact(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("smoke read pattern %s: %w", path, err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("smoke parse pattern %s: %w", path, err)
	}

	hybridRaw, ok := payload["hybrid_mapping"].(map[string]any)
	if !ok {
		return fmt.Errorf("smoke invalid hybrid_mapping in %s", path)
	}
	layersRaw, ok := hybridRaw["layers"].([]any)
	if !ok || len(layersRaw) == 0 {
		return fmt.Errorf("smoke hybrid layers missing in %s", path)
	}
	for _, layer := range layersRaw {
		layerMap, layerOK := layer.(map[string]any)
		if !layerOK {
			return fmt.Errorf("smoke invalid hybrid layer in %s", path)
		}
		binding, ok := layerMap["engine_binding"].(string)
		if !ok || binding != "godot" {
			return fmt.Errorf("smoke invalid engine binding in %s", path)
		}
	}

	handoffRaw, ok := payload["sdd_handoff"].(map[string]any)
	if !ok {
		return fmt.Errorf("smoke invalid sdd_handoff in %s", path)
	}
	enabled, ok := handoffRaw["enabled"].(bool)
	if !ok || !enabled {
		return fmt.Errorf("smoke invalid sdd_handoff enabled flag in %s", path)
	}
	mode, ok := handoffRaw["mode"].(string)
	if !ok || mode != "phase-driven" {
		return fmt.Errorf("smoke invalid sdd_handoff mode in %s", path)
	}

	if err := validateCoreGameWorkflow(payload, path); err != nil {
		return err
	}
	return validateModelRouting(payload, path)
}

func validateConfigArtifact(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("smoke read config %s: %w", path, err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("smoke parse config %s: %w", path, err)
	}

	packRaw, ok := payload["pack"].(map[string]any)
	if !ok {
		return fmt.Errorf("smoke invalid pack object in %s", path)
	}
	status, ok := packRaw["status"].(string)
	if !ok || status != "active" {
		return fmt.Errorf("smoke invalid pack status in %s", path)
	}

	skillRaw, ok := payload["skill_agent_mapping"].([]any)
	if !ok || len(skillRaw) == 0 {
		return fmt.Errorf("smoke missing skill_agent_mapping in %s", path)
	}
	if !containsSkill(skillRaw, "godot-specialist") {
		return fmt.Errorf("smoke missing godot-specialist mapping in %s", path)
	}

	if err := validateCoreGameWorkflow(payload, path); err != nil {
		return err
	}
	return validateModelRouting(payload, path)
}

func validateModelRouting(payload map[string]any, path string) error {
	routingRaw, ok := payload["model_routing"].(map[string]any)
	if !ok {
		return fmt.Errorf("smoke missing model_routing in %s", path)
	}
	version, ok := routingRaw["version"].(string)
	if !ok || version != modelrouting.Version {
		return fmt.Errorf("smoke invalid model routing version in %s", path)
	}
	if routingRaw["core_game_workflow_version"] != "core-game-workflow/v1" {
		return fmt.Errorf("smoke model routing core workflow link invalid in %s", path)
	}
	if _, exists := routingRaw[smokeLegacyOptionalFutureCapabilitiesField]; exists {
		return fmt.Errorf("smoke model routing deprecated mixed capability field %q must not be present in %s", smokeLegacyOptionalFutureCapabilitiesField, path)
	}
	if err := validateModelRoutingCapabilities(routingRaw["capabilities"], path); err != nil {
		return err
	}
	if !sameJSONStringSet(routingRaw["optional_capabilities"], smokeExpectedOptionalModelRoutingCapabilities) {
		return fmt.Errorf("smoke model routing optional capability catalog must exactly match model-routing/v1 optional non-future IDs in %s", path)
	}
	if !sameJSONStringSet(routingRaw["future_capabilities"], smokeExpectedFutureModelRoutingCapabilities) {
		return fmt.Errorf("smoke model routing future capability catalog must exactly match model-routing/v1 future IDs in %s", path)
	}
	if !containsJSONObjectsWithString(routingRaw["phase_routes"], "phase_id", []string{"game-concept", "game-pillars", "core-loop", "player-fantasy", "mechanics-brief", "narrative-brief", "tone-and-mood", "story-constraints", "gdd-slice", "change-brief", "repair-brief", "sdd-apply", "qa-review"}) {
		return fmt.Errorf("smoke model routing phase routes incomplete in %s", path)
	}
	if err := validateModelRoutingNoExecutionClaims(routingRaw["no_execution_claims"], path); err != nil {
		return err
	}
	validation, ok := routingRaw["routing_validation_result"].(map[string]any)
	if !ok || validation["can_continue"] != true || validation["status"] == "blocker" {
		return fmt.Errorf("smoke model routing validation should be non-blocking in %s", path)
	}
	bindings, ok := routingRaw["provider_model_bindings"].([]any)
	if !ok || len(bindings) == 0 {
		return fmt.Errorf("smoke model routing bindings missing in %s", path)
	}
	for _, binding := range bindings {
		object, ok := binding.(map[string]any)
		if !ok || object["provider"] != "not_configured" || object["model"] != "not_configured" || object["binding_source"] != modelrouting.BindingSourceManualOrRuntimeOwned {
			return fmt.Errorf("smoke model routing binding invalid in %s", path)
		}
	}
	return nil
}

func validateModelRoutingCapabilities(raw any, path string) error {
	items, ok := raw.([]any)
	if !ok {
		return fmt.Errorf("smoke model routing capabilities invalid in %s", path)
	}
	expected := make(map[string]struct{}, len(smokeExpectedModelRoutingCapabilities))
	for _, id := range smokeExpectedModelRoutingCapabilities {
		expected[id] = struct{}{}
	}
	seen := map[string]map[string]any{}
	for _, item := range items {
		object, ok := item.(map[string]any)
		if !ok {
			return fmt.Errorf("smoke model routing capability entry invalid in %s", path)
		}
		id, _ := object["id"].(string)
		if _, ok := expected[id]; !ok {
			return fmt.Errorf("smoke model routing capability %q is not part of model-routing/v1 in %s", id, path)
		}
		if _, exists := seen[id]; exists {
			return fmt.Errorf("smoke model routing capability %q duplicated in %s", id, path)
		}
		seen[id] = object
		if id == "" || object["provider"] != "not_configured" || object["model"] != "not_configured" {
			return fmt.Errorf("smoke model routing capability %q hardcodes provider/model in %s", id, path)
		}
		if (id == "image_generation" || id == "image_review" || id == "audio_generation" || id == "audio_review") && object["configured_status"] == "configured" {
			return fmt.Errorf("smoke model routing future capability %q must not be ready in %s", id, path)
		}
	}
	for _, id := range smokeExpectedModelRoutingCapabilities {
		item, ok := seen[id]
		if !ok {
			return fmt.Errorf("smoke model routing capability %q missing in %s", id, path)
		}
		if err := validateModelRoutingCapabilityObject(id, item, path); err != nil {
			return err
		}
	}
	return nil
}

func validateModelRoutingCapabilityObject(id string, object map[string]any, path string) error {
	category, _ := object["category"].(string)
	requirement, _ := object["requirement"].(string)
	configuredStatus, _ := object["configured_status"].(string)
	validationStatus, _ := object["validation_status"].(string)
	if category == "" || requirement == "" || configuredStatus == "" || validationStatus == "" {
		return fmt.Errorf("smoke model routing capability %q missing status metadata in %s", id, path)
	}
	if object["provider"] != modelrouting.StatusNotConfigured || object["model"] != modelrouting.StatusNotConfigured {
		return fmt.Errorf("smoke model routing capability %q must keep provider/model not_configured in %s", id, path)
	}
	if configuredStatus == modelrouting.StatusConfigured {
		return fmt.Errorf("smoke model routing capability %q must not be preconfigured in %s", id, path)
	}
	if isSmokeExpectedOptionalCapability(id) {
		if category != "optional" || requirement != "optional" {
			return fmt.Errorf("smoke model routing optional capability %q misclassified in %s", id, path)
		}
		if validationStatus == modelrouting.ValidationBlocker || validationStatus == modelrouting.ValidationDeferred {
			return fmt.Errorf("smoke model routing optional capability %q must remain non-blocking and non-future in %s", id, path)
		}
	}
	if isSmokeExpectedFutureCapability(id) && (category != "future" || requirement != "future") {
		return fmt.Errorf("smoke model routing future capability %q misclassified in %s", id, path)
	}
	return nil
}

func validateModelRoutingNoExecutionClaims(raw any, path string) error {
	claims, ok := raw.(map[string]any)
	if !ok {
		return fmt.Errorf("smoke model routing no-execution claims invalid in %s", path)
	}
	for _, claim := range []string{"provider_api_calls", "model_downloads", "image_generation", "audio_generation", "sdd_execution", "godot_mutation", "comfyui_execution", "blender_execution", "auto_approval"} {
		value, ok := claims[claim].(bool)
		if !ok || value {
			return fmt.Errorf("smoke model routing no-execution claim %q invalid in %s", claim, path)
		}
	}
	return nil
}

func validateCoreGameWorkflow(payload map[string]any, path string) error {
	workflowRaw, ok := payload["core_game_workflow"].(map[string]any)
	if !ok {
		return fmt.Errorf("smoke missing core_game_workflow in %s", path)
	}
	version, ok := workflowRaw["contract_version"].(string)
	if !ok || version != "core-game-workflow/v1" {
		return fmt.Errorf("smoke invalid core game workflow version in %s", path)
	}
	if !containsJSONStrings(workflowRaw["phase_ids"], []string{"game-concept", "game-pillars", "core-loop", "player-fantasy", "mechanics-brief", "narrative-brief", "tone-and-mood", "story-constraints", "gdd-slice", "change-brief", "repair-brief"}) {
		return fmt.Errorf("smoke core game workflow phase IDs incomplete in %s", path)
	}
	if !containsJSONStrings(workflowRaw["narrative_modes"], []string{"explicit_story", "environmental_story", "emergent_story", "minimal_context", "none_or_mechanics_first"}) {
		return fmt.Errorf("smoke core game workflow narrative modes incomplete in %s", path)
	}
	if !containsJSONStrings(workflowRaw["repair_classifications"], []string{"design_change", "implementation_bug", "tuning_balancing_issue", "movement_controls_issue", "animation_pose_issue", "asset_import_issue", "texture_visual_issue", "programming_logic_bug", "narrative_content_inconsistency", "mixed_unknown"}) {
		return fmt.Errorf("smoke core game workflow classifications incomplete in %s", path)
	}
	if !containsJSONObjectsWithString(workflowRaw["mode_contracts"], "id", []string{"zero-to-one creation", "direct phase invocation", "repair/change handoff"}) {
		return fmt.Errorf("smoke core game workflow mode contracts incomplete in %s", path)
	}
	if err := validateCoreGameModeContracts(workflowRaw["mode_contracts"], path); err != nil {
		return err
	}
	approvalRaw, ok := workflowRaw["approval_policy"].(map[string]any)
	if !ok || approvalRaw["default_status"] != "draft" || approvalRaw["approval_state"] != "pending_human_approval" || approvalRaw["auto_approved"] != false {
		return fmt.Errorf("smoke core game workflow approval policy invalid in %s", path)
	}
	if err := validateCoreGameArtifactDefaults(approvalRaw["artifact_defaults"], path); err != nil {
		return err
	}
	if !sameJSONStrings(workflowRaw["repair_change_handoff_flow"], []string{"reported issue", "triage", "classification", "relevant artifacts", "repair/change brief", "human approval", "downstream handoff", "verification"}) {
		return fmt.Errorf("smoke core game workflow handoff flow incomplete in %s", path)
	}
	if !containsJSONObjectsWithString(workflowRaw["downstream_references"], "target", []string{"Art Bible", "Visual Identity Anchor", "Asset Spec", "Asset Manifest", "Godot handoff", "SDD handoff", "QA/review checklist", "future image/audio workflows", "future model routing by phase/capability"}) {
		return fmt.Errorf("smoke core game workflow downstream references incomplete in %s", path)
	}
	if err := validateCoreGameDownstreamReferences(workflowRaw["downstream_references"], path); err != nil {
		return err
	}
	if err := validateCoreGameNoExecutionClaims(workflowRaw["no_execution_claims"], path); err != nil {
		return err
	}
	return nil
}

func validateCoreGameModeContracts(raw any, path string) error {
	required := map[string][]string{
		"zero-to-one creation":    {"idea/existing game", "concept interview", "game concept", "game pillars", "core loop", "player fantasy", "mechanics scope", "narrative mode", "narrative brief/minimal narrative contract", "tone and mood", "story constraints", "gdd slice", "human approval gate", "downstream"},
		"direct phase invocation": {"select phase", "load relevant artifacts", "draft phase output", "human approval gate", "update downstream references"},
		"repair/change handoff":   {"reported issue", "triage", "classification", "relevant artifacts", "repair/change brief", "human approval", "downstream handoff", "verification"},
	}
	items, ok := raw.([]any)
	if !ok {
		return fmt.Errorf("smoke core game workflow mode contracts invalid in %s", path)
	}
	seen := map[string]map[string]any{}
	for _, item := range items {
		object, ok := item.(map[string]any)
		if !ok {
			return fmt.Errorf("smoke core game workflow mode contract entry invalid in %s", path)
		}
		id, ok := object["id"].(string)
		if !ok || id == "" {
			return fmt.Errorf("smoke core game workflow mode contract id missing in %s", path)
		}
		seen[id] = object
	}
	for id, sequence := range required {
		item, ok := seen[id]
		if !ok {
			return fmt.Errorf("smoke core game workflow mode contract %q missing in %s", id, path)
		}
		approval, _ := item["approval_requirement"].(string)
		downstream, _ := item["downstream_behavior"].(string)
		if !sameJSONStrings(item["sequence"], sequence) || !hasJSONArrayItems(item["relevant_artifacts"]) || !strings.Contains(strings.ToLower(approval), "human approval") || strings.TrimSpace(downstream) == "" {
			return fmt.Errorf("smoke core game workflow mode contract %q incomplete in %s", id, path)
		}
	}
	return nil
}

func validateCoreGameArtifactDefaults(raw any, path string) error {
	items, ok := raw.([]any)
	if !ok {
		return fmt.Errorf("smoke core game workflow artifact defaults missing in %s", path)
	}
	required := map[string]struct{}{
		"game-concept":      {},
		"game-pillars":      {},
		"core-loop":         {},
		"player-fantasy":    {},
		"mechanics-brief":   {},
		"narrative-brief":   {},
		"tone-and-mood":     {},
		"story-constraints": {},
		"gdd-slice":         {},
		"change-brief":      {},
		"repair-brief":      {},
	}
	seen := map[string]map[string]any{}
	for _, item := range items {
		object, ok := item.(map[string]any)
		if !ok {
			return fmt.Errorf("smoke core game workflow artifact default invalid in %s", path)
		}
		id, ok := object["artifact_id"].(string)
		if !ok || id == "" {
			return fmt.Errorf("smoke core game workflow artifact default id missing in %s", path)
		}
		if _, ok := required[id]; !ok {
			return fmt.Errorf("smoke core game workflow artifact default %q unknown in %s", id, path)
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("smoke core game workflow artifact default %q duplicated in %s", id, path)
		}
		if object["status"] != "draft" || object["approval_state"] != "pending_human_approval" || object["auto_approved"] != false {
			return fmt.Errorf("smoke core game workflow artifact default %q invalid in %s", id, path)
		}
		seen[id] = object
	}
	for id := range required {
		item, ok := seen[id]
		if !ok || item["status"] != "draft" || item["approval_state"] != "pending_human_approval" || item["auto_approved"] != false {
			return fmt.Errorf("smoke core game workflow artifact default %q invalid in %s", id, path)
		}
	}
	return nil
}

func validateCoreGameDownstreamReferences(raw any, path string) error {
	required := map[string][]string{
		"Art Bible":                    {"approved intent", "tone and mood", "gdd slice"},
		"Visual Identity Anchor":       {"approved intent", "tone and mood", "gdd slice"},
		"Asset Spec":                   {"approved intent", "gdd slice", "mechanics brief", "Art Bible"},
		"Asset Manifest":               {"Asset Spec", "provenance", "status"},
		"Godot handoff":                {"gdd slice", "mechanics brief", "change brief", "repair brief"},
		"SDD handoff":                  {"change brief", "repair brief", "acceptance criteria"},
		"QA/review checklist":          {"approved intent", "acceptance criteria", "verification plan"},
		"future image/audio workflows": {"Art Bible", "Asset Spec", "GDD constraints"},
		"future model routing by phase/capability": {"phase IDs", "capabilities"},
	}
	items, ok := raw.([]any)
	if !ok {
		return fmt.Errorf("smoke core game workflow downstream references invalid in %s", path)
	}
	seen := map[string]map[string]any{}
	for _, item := range items {
		object, ok := item.(map[string]any)
		if !ok {
			return fmt.Errorf("smoke core game workflow downstream reference invalid in %s", path)
		}
		target, ok := object["target"].(string)
		if !ok || strings.TrimSpace(target) == "" {
			return fmt.Errorf("smoke core game workflow downstream target missing in %s", path)
		}
		seen[target] = object
	}
	for target, consumes := range required {
		item, ok := seen[target]
		approval, _ := item["approval_requirement"].(string)
		if !ok || !containsJSONStrings(item["consumes"], consumes) || strings.TrimSpace(approval) == "" {
			return fmt.Errorf("smoke core game workflow downstream reference %q incomplete in %s", target, path)
		}
		if target == "QA/review checklist" && !containsJSONStrings(item["verifies"], []string{"approved intent", "acceptance criteria", "verification plan"}) {
			return fmt.Errorf("smoke core game workflow QA verification incomplete in %s", path)
		}
	}
	return nil
}

func validateCoreGameNoExecutionClaims(raw any, path string) error {
	claims, ok := raw.(map[string]any)
	if !ok {
		return fmt.Errorf("smoke core game workflow no execution claims missing in %s", path)
	}
	for _, claim := range []string{"debugging_executed", "image_generated", "audio_generated", "playtesting_run", "engine_files_mutated", "collaboration_workflow_executed", "claude_native_mechanics_enabled"} {
		value, ok := claims[claim].(bool)
		if !ok || value {
			return fmt.Errorf("smoke core game workflow no execution claim %q invalid in %s", claim, path)
		}
	}
	return nil
}

func hasJSONArrayItems(raw any) bool {
	items, ok := raw.([]any)
	return ok && len(items) > 0
}

func containsJSONObjectsWithString(raw any, field string, required []string) bool {
	items, ok := raw.([]any)
	if !ok {
		return false
	}
	seen := map[string]struct{}{}
	for _, item := range items {
		object, ok := item.(map[string]any)
		if !ok {
			return false
		}
		value, ok := object[field].(string)
		if ok {
			seen[value] = struct{}{}
		}
	}
	for _, requiredItem := range required {
		if _, ok := seen[requiredItem]; !ok {
			return false
		}
	}
	return true
}

func containsJSONStrings(raw any, required []string) bool {
	items, ok := raw.([]any)
	if !ok {
		return false
	}
	seen := map[string]struct{}{}
	for _, item := range items {
		value, ok := item.(string)
		if ok {
			seen[value] = struct{}{}
		}
	}
	for _, requiredItem := range required {
		if _, ok := seen[requiredItem]; !ok {
			return false
		}
	}
	return true
}

func sameJSONStringSet(raw any, required []string) bool {
	items, ok := raw.([]any)
	if !ok || len(items) != len(required) {
		return false
	}
	seen := map[string]struct{}{}
	for _, item := range items {
		value, ok := item.(string)
		if !ok {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	for _, requiredItem := range required {
		if _, ok := seen[requiredItem]; !ok {
			return false
		}
	}
	return true
}

func isSmokeExpectedOptionalCapability(id string) bool {
	for _, item := range smokeExpectedOptionalModelRoutingCapabilities {
		if id == item {
			return true
		}
	}
	return false
}

func isSmokeExpectedFutureCapability(id string) bool {
	for _, item := range smokeExpectedFutureModelRoutingCapabilities {
		if id == item {
			return true
		}
	}
	return false
}

func sameJSONStrings(raw any, required []string) bool {
	items, ok := raw.([]any)
	if !ok || len(items) != len(required) {
		return false
	}
	for idx, item := range items {
		value, ok := item.(string)
		if !ok || value != required[idx] {
			return false
		}
	}
	return true
}

func containsSkill(items []any, skill string) bool {
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		value, ok := entry["skill"].(string)
		if ok && value == skill {
			return true
		}
	}
	return false
}
