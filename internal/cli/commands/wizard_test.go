package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"open-game-studios/internal/assets"
	"open-game-studios/internal/integrations"
	"open-game-studios/internal/opencode"
	"open-game-studios/internal/persistence"
	"open-game-studios/internal/routing"
	"open-game-studios/internal/templates"
	"open-game-studios/internal/workflows/coregame"
	"open-game-studios/internal/workflows/modelrouting"
)

const legacyAutoFallback = "unconfigured" + ":auto"

func TestStepRuntimeContext_DoesNotInspectProviders(t *testing.T) {
	state := &wizardState{ProfileName: "Game-Studio"}
	stdout := captureStdout(t, func() {
		if err := stepRuntimeContext(state); err != nil {
			t.Fatalf("stepRuntimeContext returned error: %v", err)
		}
	})

	if len(state.Providers) != 0 {
		t.Fatalf("expected empty provider inventory, got %d", len(state.Providers))
	}
	if len(state.Snapshot.Providers) != 0 {
		t.Fatalf("expected empty snapshot providers, got %d", len(state.Snapshot.Providers))
	}
	if state.Snapshot.Profile != "Game-Studio" {
		t.Fatalf("expected snapshot profile Game-Studio, got %q", state.Snapshot.Profile)
	}
	if !strings.Contains(state.ProviderWarning, "did not inspect") {
		t.Fatalf("expected not-inspected provider notice, got %q", state.ProviderWarning)
	}
	if !strings.Contains(stdout, "not-inspected") {
		t.Fatalf("expected not-inspected runtime output, got: %q", stdout)
	}
	for _, tier := range []routing.Tier{routing.TierFast, routing.TierBalanced, routing.TierDeep} {
		selection := state.Policy.TierDefaults[tier]
		if selection.Provider != modelrouting.StatusNotConfigured || selection.Model != modelrouting.StatusNotConfigured {
			t.Fatalf("unexpected fallback tier selection for %s: %#v", tier, selection)
		}
	}
}

func TestUnconfiguredRoutingPolicy_DoesNotExposeAutoModel(t *testing.T) {
	policy := unconfiguredRoutingPolicy()

	for tier, selection := range tierDefaultsForArtifact(policy) {
		if selection == legacyAutoFallback {
			t.Fatalf("tier %s exposes legacy auto model as usable fallback", tier)
		}
		if selection != "not_configured:not_configured" {
			t.Fatalf("tier %s fallback=%q want not_configured:not_configured", tier, selection)
		}
	}
}

func TestStepSmokeValidation_DoesNotCreateJudgmentDayMarker(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".game-studio", "generated", "wizard"), 0o755); err != nil {
		t.Fatalf("create marker parent dir: %v", err)
	}

	paths := outputPaths{
		profile: filepath.Join(root, "profiles", "game-studio", "generated", "studio-profile.godot.md"),
		summary: filepath.Join(root, "profiles", "game-studio", "generated", "studio-profile.godot.summary.md"),
		pattern: filepath.Join(root, "profiles", "game-studio", "generated", "pack.godot.hybrid-map.json"),
		config:  filepath.Join(root, "profiles", "game-studio", "generated", "pack.godot.config.json"),
	}

	if err := writeSmokeFixtures(paths); err != nil {
		t.Fatalf("write smoke fixtures: %v", err)
	}

	wizardProfile := filepath.Join(root, ".game-studio", "profiles", "game-studio.md")
	if err := os.MkdirAll(filepath.Dir(wizardProfile), 0o755); err != nil {
		t.Fatalf("create wizard profile dir: %v", err)
	}
	fixture := "# Game-Studio\n\n## Providers\n- ownership: " + wizardProviderNotice + "\n- inventory: not-inspected\n\n## MCP Runtime Assistants\n- note: " + wizardMCPNotice + "\n"
	if err := os.WriteFile(wizardProfile, []byte(fixture), 0o644); err != nil {
		t.Fatalf("write wizard profile fixture: %v", err)
	}

	state := &wizardState{
		ProfileName:         "Game-Studio",
		ProfileArtifactPath: wizardProfile,
		SmokePaths:          paths,
	}

	if err := stepSmokeValidation(state); err != nil {
		t.Fatalf("stepSmokeValidation returned error: %v", err)
	}

	markerPath := filepath.Join(root, ".game-studio", "generated", "wizard", "judgment-day.required")
	if _, err := os.Stat(markerPath); !os.IsNotExist(err) {
		t.Fatalf("expected no judgment-day marker to be created, err=%v", err)
	}
}

func TestStepRoleOverrides_AdvancedAppliesValidOverride(t *testing.T) {
	snapshot := wizardRoutingSnapshotFixture()
	policy, err := routing.NewBalancedPolicy(snapshot)
	if err != nil {
		t.Fatalf("build balanced policy: %v", err)
	}

	state := &wizardState{
		Complexity: "advanced",
		Snapshot:   snapshot,
		Policy:     policy,
	}

	reader := bufio.NewReader(strings.NewReader("openai:gpt-4o\n\n\n"))
	if err := stepRoleOverrides(reader, false, state); err != nil {
		t.Fatalf("stepRoleOverrides returned error: %v", err)
	}

	override, ok := state.Policy.RoleOverrides["orchestrator"]
	if !ok {
		t.Fatalf("expected orchestrator override to be applied")
	}
	if override.Provider != "openai" || override.Model != "gpt-4o" {
		t.Fatalf("unexpected orchestrator override: %#v", override)
	}
}

func TestStepRoleOverrides_SimpleSkipsOverrides(t *testing.T) {
	snapshot := wizardRoutingSnapshotFixture()
	policy, err := routing.NewBalancedPolicy(snapshot)
	if err != nil {
		t.Fatalf("build balanced policy: %v", err)
	}

	state := &wizardState{
		Complexity: "simple",
		Snapshot:   snapshot,
		Policy:     policy,
	}

	reader := bufio.NewReader(strings.NewReader("openai:gpt-4o\n"))
	if err := stepRoleOverrides(reader, false, state); err != nil {
		t.Fatalf("stepRoleOverrides returned error: %v", err)
	}

	if len(state.Policy.RoleOverrides) != 0 {
		t.Fatalf("expected no role overrides in simple mode, got %d", len(state.Policy.RoleOverrides))
	}
}

func TestStepSDDPhaseRouting_AdvancedSkipsPhaseOverrides(t *testing.T) {
	snapshot := wizardRoutingSnapshotFixture()
	policy, err := routing.NewBalancedPolicy(snapshot)
	if err != nil {
		t.Fatalf("build balanced policy: %v", err)
	}

	state := &wizardState{
		Complexity: "advanced",
		Snapshot:   snapshot,
		Policy:     policy,
	}

	reader := bufio.NewReader(strings.NewReader("openai:gpt-4o-mini\n"))
	if err := stepSDDPhaseRouting(reader, false, state); err != nil {
		t.Fatalf("stepSDDPhaseRouting returned error: %v", err)
	}

	if len(state.Policy.PhaseOverrides) != 0 {
		t.Fatalf("expected no phase overrides in advanced mode, got %d", len(state.Policy.PhaseOverrides))
	}
}

func TestStepSDDPhaseRouting_ExpertAppliesPhaseOverride(t *testing.T) {
	snapshot := wizardRoutingSnapshotFixture()
	policy, err := routing.NewBalancedPolicy(snapshot)
	if err != nil {
		t.Fatalf("build balanced policy: %v", err)
	}

	state := &wizardState{
		Complexity: "expert",
		Snapshot:   snapshot,
		Policy:     policy,
	}

	reader := bufio.NewReader(strings.NewReader("openai:gpt-4o-mini\n\n\n\n\n\n\n\n"))
	if err := stepSDDPhaseRouting(reader, false, state); err != nil {
		t.Fatalf("stepSDDPhaseRouting returned error: %v", err)
	}

	override, ok := state.Policy.PhaseOverrides["explore"]
	if !ok {
		t.Fatalf("expected explore override to be applied")
	}
	if override.Provider != "openai" || override.Model != "gpt-4o-mini" {
		t.Fatalf("unexpected explore override: %#v", override)
	}
}

func TestStepTierRouting_RejectsModelOutsideSnapshot(t *testing.T) {
	snapshot := wizardRoutingSnapshotFixture()
	policy, err := routing.NewBalancedPolicy(snapshot)
	if err != nil {
		t.Fatalf("build balanced policy: %v", err)
	}

	state := &wizardState{
		Snapshot: snapshot,
		Policy:   policy,
	}

	reader := bufio.NewReader(strings.NewReader("fast=openai:not-available\n"))
	err = stepTierRouting(reader, false, state)
	if err == nil {
		t.Fatalf("expected invalid tier override error")
	}
	if !strings.Contains(err.Error(), "invalid tier override") {
		t.Fatalf("expected invalid tier override message, got: %v", err)
	}
}

func TestStepRoleOverrides_RejectsModelOutsideSnapshot(t *testing.T) {
	snapshot := wizardRoutingSnapshotFixture()
	policy, err := routing.NewBalancedPolicy(snapshot)
	if err != nil {
		t.Fatalf("build balanced policy: %v", err)
	}

	state := &wizardState{
		Complexity: "advanced",
		Snapshot:   snapshot,
		Policy:     policy,
	}

	reader := bufio.NewReader(strings.NewReader("openai:not-available\n\n\n"))
	err = stepRoleOverrides(reader, false, state)
	if err == nil {
		t.Fatalf("expected invalid role override error")
	}
	if !strings.Contains(err.Error(), "invalid selection") {
		t.Fatalf("expected invalid selection message, got: %v", err)
	}
}

func writeSmokeFixtures(paths outputPaths) error {
	for _, path := range []string{paths.profile, paths.summary, paths.pattern, paths.config} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
	}

	profile := "## Pack Metadata\n## MCP Placeholders\n## Core Game Workflow Contract\ncore-game-workflow/v1\n## Model Routing Contract\nmodel-routing/v1\ngame-concept\nrepair/change handoff\ntarget_engine: godot\npack_status: active\n- setup_preset: recommended\n- preset_source: default\n- routing_preset: recommended\n- routing_preset_source: derived_from_setup_preset\nengram: enabled=false\ncontext7: enabled=false\n"
	if err := os.WriteFile(paths.profile, []byte(profile), 0o644); err != nil {
		return err
	}

	summary := "## Artifact Set\n## Future Packs (Explicit Placeholders)\n## Core Game Workflow Metadata\ncore-game-workflow/v1\n## Model Routing Metadata\nmodel-routing/v1\ngdd-slice\nchange-brief\nrepair-brief\nunity: placeholder\nue5: placeholder\n- setup_preset: recommended\n- preset_source: default\n- routing_preset: recommended\n- routing_preset_source: derived_from_setup_preset\nengram: enabled=false, mode=placeholder, required=false\ncontext7: enabled=false, mode=placeholder, required=false\n"
	if err := os.WriteFile(paths.summary, []byte(summary), 0o644); err != nil {
		return err
	}

	pattern := map[string]any{
		"pack":                  map[string]any{"id": "godot-core"},
		"setup_preset":          "recommended",
		"preset_source":         presetSourceDefault,
		"routing_preset":        "recommended",
		"routing_preset_source": routingPresetSourceDerived,
		"hybrid_mapping": map[string]any{
			"layers": []any{map[string]any{"engine_binding": "godot"}},
		},
		"mcp_placeholders": map[string]any{
			"engram":   map[string]any{"enabled": false, "required": false, "mode": "placeholder"},
			"context7": map[string]any{"enabled": false, "required": false, "mode": "placeholder"},
		},
		"core_game_workflow": coreGameWorkflowFixture(),
		"model_routing":      modelRoutingFixture(),
		"sdd_handoff":        map[string]any{"enabled": true, "mode": "phase-driven"},
	}
	if err := writeJSONFixture(paths.pattern, pattern); err != nil {
		return err
	}

	config := map[string]any{
		"pack":                  map[string]any{"status": "active"},
		"setup_preset":          "recommended",
		"preset_source":         presetSourceDefault,
		"routing_preset":        "recommended",
		"routing_preset_source": routingPresetSourceDerived,
		"skill_agent_mapping":   []any{map[string]any{"skill": "godot-specialist"}},
		"mcp_placeholders": map[string]any{
			"engram":   map[string]any{"enabled": false, "required": false, "mode": "placeholder"},
			"context7": map[string]any{"enabled": false, "required": false, "mode": "placeholder"},
		},
		"core_game_workflow": coreGameWorkflowFixture(),
		"model_routing":      modelRoutingFixture(),
		"sdd_handoff":        map[string]any{"enabled": true, "mode": "phase-driven", "bootstrap_stub": "marker-driven"},
	}
	if err := writeJSONFixture(paths.config, config); err != nil {
		return err
	}

	return nil
}

func TestValidateModelRoutingRequiresOptionalCapabilitiesAsObjects(t *testing.T) {
	for _, id := range []string{"archive_summary", "visual_review", "godot_technical_review"} {
		t.Run(id, func(t *testing.T) {
			routing := smokeModelRoutingFixture(t)
			routing["capabilities"] = removeCapabilityObject(routing["capabilities"].([]any), id)

			err := validateModelRouting(map[string]any{"model_routing": routing}, "fixture.json")
			if err == nil {
				t.Fatalf("expected smoke to fail when %q is only listed in optional_capabilities", id)
			}
			if !strings.Contains(err.Error(), id) {
				t.Fatalf("expected error to mention missing capability %q, got: %v", id, err)
			}
		})
	}
}

func TestValidateModelRoutingAllowsOptionalCapabilitiesNotConfigured(t *testing.T) {
	routing := smokeModelRoutingFixture(t)

	if err := validateModelRouting(map[string]any{"model_routing": routing}, "fixture.json"); err != nil {
		t.Fatalf("expected smoke to pass with optional non-future capabilities present and not_configured: %v", err)
	}
}

func TestValidateModelRoutingPassesWithExactCapabilityCatalog(t *testing.T) {
	routing := smokeModelRoutingFixture(t)

	if err := validateModelRouting(map[string]any{"model_routing": routing}, "fixture.json"); err != nil {
		t.Fatalf("expected smoke to pass with exact model-routing/v1 capability catalog: %v", err)
	}
}

func TestValidateModelRoutingRejectsExtraCapabilityObject(t *testing.T) {
	routing := smokeModelRoutingFixture(t)
	capabilities := routing["capabilities"].([]any)
	routing["capabilities"] = append(capabilities, map[string]any{
		"id":                "unknown_extra_capability",
		"label":             "Unknown extra capability",
		"category":          "optional",
		"requirement":       "optional",
		"selected":          false,
		"configured_status": modelrouting.StatusNotConfigured,
		"provider":          modelrouting.StatusNotConfigured,
		"model":             modelrouting.StatusNotConfigured,
		"validation_status": modelrouting.ValidationWarning,
	})

	err := validateModelRouting(map[string]any{"model_routing": routing}, "fixture.json")
	if err == nil {
		t.Fatalf("expected smoke to fail when an extra capability object appears")
	}
	if !strings.Contains(err.Error(), "unknown_extra_capability") {
		t.Fatalf("expected error to mention extra capability id, got: %v", err)
	}
}

func TestValidateModelRoutingRejectsLegacyOptionalFutureCapabilitiesField(t *testing.T) {
	routing := smokeModelRoutingFixture(t)
	legacyMixedField := "optional_" + "future_" + "capabilities"
	routing[legacyMixedField] = []any{"archive_summary", "image_generation"}

	err := validateModelRouting(map[string]any{"model_routing": routing}, "fixture.json")
	if err == nil {
		t.Fatalf("expected smoke to fail when deprecated mixed capability field appears")
	}
	if !strings.Contains(err.Error(), legacyMixedField) {
		t.Fatalf("expected error to mention deprecated mixed capability field, got: %v", err)
	}
}

func TestValidateModelRoutingRequiresExactSeparatedCapabilityCatalogs(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{
			name: "optional appears in future",
			mutate: func(routing map[string]any) {
				routing["future_capabilities"] = []any{"image_generation", "image_review", "audio_generation", "audio_review", "archive_summary"}
			},
		},
		{
			name: "future appears in optional",
			mutate: func(routing map[string]any) {
				routing["optional_capabilities"] = []any{"archive_summary", "visual_review", "godot_technical_review", "image_generation"}
			},
		},
		{
			name: "expected optional missing",
			mutate: func(routing map[string]any) {
				routing["optional_capabilities"] = []any{"visual_review", "godot_technical_review"}
			},
		},
		{
			name: "expected future missing",
			mutate: func(routing map[string]any) {
				routing["future_capabilities"] = []any{"image_generation", "image_review", "audio_generation"}
			},
		},
		{
			name: "extra optional unknown",
			mutate: func(routing map[string]any) {
				routing["optional_capabilities"] = []any{"archive_summary", "visual_review", "godot_technical_review", "unknown_capability"}
			},
		},
		{
			name: "extra future unknown",
			mutate: func(routing map[string]any) {
				routing["future_capabilities"] = []any{"image_generation", "image_review", "audio_generation", "audio_review", "unknown_capability"}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			routing := smokeModelRoutingFixture(t)
			test.mutate(routing)

			if err := validateModelRouting(map[string]any{"model_routing": routing}, "fixture.json"); err == nil {
				t.Fatalf("expected smoke to fail for %s", test.name)
			}
		})
	}
}

func TestValidateModelRoutingPassesWithExactSeparatedCatalogs(t *testing.T) {
	routing := smokeModelRoutingFixture(t)
	routing["optional_capabilities"] = []any{"archive_summary", "visual_review", "godot_technical_review"}
	routing["future_capabilities"] = []any{"image_generation", "image_review", "audio_generation", "audio_review"}

	if err := validateModelRouting(map[string]any{"model_routing": routing}, "fixture.json"); err != nil {
		t.Fatalf("expected smoke to pass with exact optional/future catalogs: %v", err)
	}
}

func smokeModelRoutingFixture(t *testing.T) map[string]any {
	t.Helper()
	raw, err := json.Marshal(modelRoutingFixture())
	if err != nil {
		t.Fatalf("marshal model routing fixture: %v", err)
	}
	var routing map[string]any
	if err := json.Unmarshal(raw, &routing); err != nil {
		t.Fatalf("unmarshal model routing fixture: %v", err)
	}
	return routing
}

func removeCapabilityObject(items []any, id string) []any {
	out := make([]any, 0, len(items))
	for _, item := range items {
		object, ok := item.(map[string]any)
		if ok && object["id"] == id {
			continue
		}
		out = append(out, item)
	}
	return out
}

func modelRoutingFixture() map[string]any {
	contract := modelrouting.DefaultContract("recommended")
	capabilities := make([]any, 0, len(contract.Capabilities))
	for _, capability := range contract.Capabilities {
		capabilities = append(capabilities, map[string]any{
			"id":                string(capability.ID),
			"label":             capability.Label,
			"category":          capability.Category,
			"requirement":       capability.Requirement,
			"selected":          capability.Selected,
			"configured_status": capability.ConfiguredStatus,
			"provider":          capability.Provider,
			"model":             capability.Model,
			"validation_status": capability.ValidationStatus,
		})
	}
	routes := make([]any, 0, len(contract.PhaseRoutes))
	for _, route := range contract.PhaseRoutes {
		routes = append(routes, map[string]any{"phase_id": route.PhaseID, "capability": string(route.Capability), "status": route.Status, "validation_status": route.ValidationStatus})
	}
	bindings := make([]any, 0, len(contract.ProviderModelBindings))
	for _, binding := range contract.ProviderModelBindings {
		bindings = append(bindings, map[string]any{"capability": string(binding.Capability), "provider": binding.Provider, "model": binding.Model, "configured_status": binding.ConfiguredStatus, "binding_source": binding.BindingSource, "requires_user_configuration": binding.RequiresUserConfiguration})
	}
	return map[string]any{
		"version":                    contract.Version,
		"routing_boundary":           contract.Boundary,
		"core_game_workflow_version": contract.CoreWorkflowVersion,
		"setup_preset":               "recommended",
		"preset_source":              presetSourceDefault,
		"routing_preset":             contract.RoutingPreset,
		"routing_preset_source":      routingPresetSourceDerived,
		"capabilities":               capabilities,
		"phase_routes":               routes,
		"provider_model_bindings":    bindings,
		"capability_overrides":       []any{},
		"phase_overrides":            []any{},
		"override_status":            contract.OverrideStatus,
		"validation_result":          contract.ValidationResult,
		"optional_capabilities":      capabilityIDsToStrings(contract.OptionalCapabilities),
		"future_capabilities":        capabilityIDsToStrings(contract.FutureCapabilities),
		"routing_validation_result":  map[string]any{"status": contract.RoutingValidationResult.Status, "can_continue": contract.RoutingValidationResult.CanContinue},
		"no_execution_claims":        contract.NoExecutionClaims,
	}
}

func capabilityIDsToStrings(items []modelrouting.CapabilityID) []any {
	out := make([]any, 0, len(items))
	for _, item := range items {
		out = append(out, string(item))
	}
	return out
}

func assertWizardPhaseRoute(t *testing.T, routing wizardModelRoutingArtifact, phaseID string, capability modelrouting.CapabilityID) {
	t.Helper()
	for _, route := range routing.PhaseRoutes {
		if route.PhaseID == phaseID {
			if route.Capability != capability {
				t.Fatalf("phase route %q capability=%q want %q", phaseID, route.Capability, capability)
			}
			return
		}
	}
	t.Fatalf("phase route %q missing: %#v", phaseID, routing.PhaseRoutes)
}

func assertFutureModelCapabilitiesNotBlocking(t *testing.T, routing wizardModelRoutingArtifact) {
	t.Helper()
	if !routing.RoutingValidationResult.CanContinue || routing.RoutingValidationResult.Status == modelrouting.ValidationBlocker {
		t.Fatalf("future model capabilities should not block: %#v", routing.RoutingValidationResult)
	}
	for _, capability := range routing.Capabilities {
		if isFutureMediaCapability(capability.ID) && capability.ValidationStatus != modelrouting.ValidationNotSelected && capability.ValidationStatus != modelrouting.ValidationDeferred {
			t.Fatalf("future media capability should be non-blocking: %#v", capability)
		}
	}
}

func assertFutureModelCapabilitiesNotReady(t *testing.T, routing wizardModelRoutingArtifact) {
	t.Helper()
	for _, capability := range routing.Capabilities {
		if isFutureMediaCapability(capability.ID) && (capability.ConfiguredStatus == modelrouting.StatusConfigured || capability.ValidationStatus == modelrouting.ValidationPass) {
			t.Fatalf("future media capability must not be ready: %#v", capability)
		}
	}
}

func assertModelRoutingNoExecutionClaims(t *testing.T, claims map[string]bool) {
	t.Helper()
	for _, claim := range []string{"provider_api_calls", "model_downloads", "image_generation", "audio_generation", "sdd_execution", "godot_mutation", "comfyui_execution", "blender_execution", "auto_approval"} {
		value, ok := claims[claim]
		if !ok || value {
			t.Fatalf("model routing no-execution claim %q invalid: %#v", claim, claims)
		}
	}
}

func assertModelRoutingUsesNotConfigured(t *testing.T, routing wizardModelRoutingArtifact) {
	t.Helper()
	for _, capability := range routing.Capabilities {
		if capability.Provider != modelrouting.StatusNotConfigured || capability.Model != modelrouting.StatusNotConfigured {
			t.Fatalf("capability %q provider/model must be not_configured: %#v", capability.ID, capability)
		}
	}
	for _, binding := range routing.ProviderModelBindings {
		if binding.Provider != modelrouting.StatusNotConfigured || binding.Model != modelrouting.StatusNotConfigured || binding.ConfiguredStatus != modelrouting.StatusNotConfigured {
			t.Fatalf("binding must remain not_configured metadata: %#v", binding)
		}
	}
}

func assertFutureCapabilitiesOnlyFuture(t *testing.T, routing wizardModelRoutingArtifact) {
	t.Helper()
	for _, id := range []modelrouting.CapabilityID{modelrouting.CapabilityImageGeneration, modelrouting.CapabilityImageReview, modelrouting.CapabilityAudioGeneration, modelrouting.CapabilityAudioReview} {
		if !containsCapabilityID(routing.FutureCapabilities, id) {
			t.Fatalf("future_capabilities missing future capability %q: %#v", id, routing.FutureCapabilities)
		}
	}
	for _, id := range routing.FutureCapabilities {
		capability, ok := wizardCapabilityByID(routing.Capabilities, id)
		if !ok {
			t.Fatalf("future_capabilities contains unknown capability %q", id)
		}
		if capability.Requirement != "future" {
			t.Fatalf("future_capabilities contains non-future capability: %#v", capability)
		}
	}
}

func assertOptionalNonFutureCapabilities(t *testing.T, routing wizardModelRoutingArtifact) {
	t.Helper()
	for _, id := range []modelrouting.CapabilityID{modelrouting.CapabilityArchiveSummary, modelrouting.CapabilityVisualReview, modelrouting.CapabilityGodotTechnicalReview} {
		if !containsCapabilityID(routing.OptionalCapabilities, id) {
			t.Fatalf("optional_capabilities missing optional non-future %q: %#v", id, routing.OptionalCapabilities)
		}
		if containsCapabilityID(routing.FutureCapabilities, id) {
			t.Fatalf("optional non-future capability %q leaked into future lists", id)
		}
	}
}

func assertRoutingValidationDeferredOnlyFuture(t *testing.T, routing wizardModelRoutingArtifact) {
	t.Helper()
	for _, item := range routing.RoutingValidationResult.Deferred {
		for _, id := range []modelrouting.CapabilityID{modelrouting.CapabilityArchiveSummary, modelrouting.CapabilityVisualReview, modelrouting.CapabilityGodotTechnicalReview} {
			if strings.Contains(item, string(id)) {
				t.Fatalf("routing validation deferred contains optional non-future capability %q: %#v", id, routing.RoutingValidationResult.Deferred)
			}
		}
		matchedFuture := false
		for _, id := range []modelrouting.CapabilityID{modelrouting.CapabilityImageGeneration, modelrouting.CapabilityImageReview, modelrouting.CapabilityAudioGeneration, modelrouting.CapabilityAudioReview} {
			if strings.Contains(item, string(id)) {
				matchedFuture = true
			}
		}
		if !matchedFuture {
			t.Fatalf("routing validation deferred contains non-future entry %q: %#v", item, routing.RoutingValidationResult.Deferred)
		}
	}
}

func assertDeferredLanesDoNotContainModelRouting(t *testing.T, lanes []wizardDeferredLane) {
	t.Helper()
	for _, lane := range lanes {
		if lane.ID == "model-routing" || lane.ID == "model-routing/v1" {
			t.Fatalf("model-routing/v1 is implemented metadata and must not be deferred: %#v", lanes)
		}
	}
}

func wizardCapabilityByID(items []modelrouting.Capability, id modelrouting.CapabilityID) (modelrouting.Capability, bool) {
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
	}
	return modelrouting.Capability{}, false
}

func containsCapabilityID(items []modelrouting.CapabilityID, target modelrouting.CapabilityID) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func isFutureMediaCapability(id modelrouting.CapabilityID) bool {
	switch id {
	case modelrouting.CapabilityImageGeneration, modelrouting.CapabilityImageReview, modelrouting.CapabilityAudioGeneration, modelrouting.CapabilityAudioReview:
		return true
	default:
		return false
	}
}

func coreGameWorkflowFixture() map[string]any {
	contract := coregame.DefaultContract()
	phaseIDs := make([]any, 0, len(contract.PhaseIDs))
	for _, phase := range contract.PhaseIDs {
		phaseIDs = append(phaseIDs, string(phase))
	}
	narrativeModes := make([]any, 0, len(contract.NarrativeModes))
	for _, mode := range contract.NarrativeModes {
		narrativeModes = append(narrativeModes, string(mode))
	}
	classifications := make([]any, 0, len(contract.RepairClassifications))
	for _, classification := range contract.RepairClassifications {
		classifications = append(classifications, string(classification))
	}
	modeContracts := make([]any, 0, len(contract.ModeContracts))
	for _, mode := range contract.ModeContracts {
		sequence := make([]any, 0, len(mode.Sequence))
		for _, step := range mode.Sequence {
			sequence = append(sequence, step)
		}
		artifacts := make([]any, 0, len(mode.RelevantArtifacts))
		for _, artifact := range mode.RelevantArtifacts {
			artifacts = append(artifacts, string(artifact))
		}
		modeContracts = append(modeContracts, map[string]any{
			"id":                   mode.ID,
			"intent":               mode.Intent,
			"sequence":             sequence,
			"relevant_artifacts":   artifacts,
			"approval_requirement": mode.ApprovalRequirement,
			"downstream_behavior":  mode.DownstreamBehavior,
		})
	}
	approvalDefaults := make([]any, 0, len(contract.ApprovalPolicy.ArtifactDefaults))
	for _, approval := range contract.ApprovalPolicy.ArtifactDefaults {
		approvalDefaults = append(approvalDefaults, map[string]any{
			"artifact_id":    string(approval.ArtifactID),
			"status":         approval.Status,
			"approval_state": approval.ApprovalState,
			"auto_approved":  approval.AutoApproved,
		})
	}
	handoffFlow := make([]any, 0, len(contract.RepairHandoffFlow))
	for _, step := range contract.RepairHandoffFlow {
		handoffFlow = append(handoffFlow, step)
	}
	downstream := make([]any, 0, len(contract.DownstreamReferences))
	for _, reference := range contract.DownstreamReferences {
		consumes := make([]any, 0, len(reference.Consumes))
		for _, consume := range reference.Consumes {
			consumes = append(consumes, consume)
		}
		verifies := make([]any, 0, len(reference.Verifies))
		for _, verify := range reference.Verifies {
			verifies = append(verifies, verify)
		}
		downstream = append(downstream, map[string]any{
			"target":               reference.Target,
			"consumes":             consumes,
			"verifies":             verifies,
			"approval_requirement": reference.ApprovalRequirement,
		})
	}
	claims := make(map[string]any, len(contract.NoExecutionClaims))
	for claim, value := range contract.NoExecutionClaims {
		claims[claim] = value
	}

	return map[string]any{
		"contract_version":           contract.Version,
		"mode_contracts":             modeContracts,
		"phase_ids":                  phaseIDs,
		"narrative_modes":            narrativeModes,
		"repair_classifications":     classifications,
		"approval_policy":            map[string]any{"default_status": contract.ApprovalPolicy.DefaultStatus, "approval_state": contract.ApprovalPolicy.ApprovalState, "auto_approved": contract.ApprovalPolicy.AutoApproved, "decision_boundary": contract.ApprovalPolicy.DecisionBoundary, "artifact_defaults": approvalDefaults},
		"repair_change_handoff_flow": handoffFlow,
		"downstream_references":      downstream,
		"no_execution_claims":        claims,
	}
}

func writeJSONFixture(path string, payload map[string]any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func wizardRoutingSnapshotFixture() opencode.ModelSnapshot {
	return opencode.ModelSnapshot{
		Profile: "Game-Studio",
		Providers: []opencode.ProviderSnapshot{
			{Provider: "openai", Models: []string{"gpt-4o-mini", "gpt-4o"}},
			{Provider: "anthropic", Models: []string{"claude-3-5-sonnet"}},
		},
	}
}

func TestRunWizard_NonInteractiveFullFlow_EmitsExpectedArtifacts(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))

	outDir := filepath.Join(workspace, "out", "generated")
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")

	stdout := captureStdout(t, func() {
		err := RunWizard(WizardInput{
			Args: []string{
				"--metadata-only",
				"--non-interactive",
				"--profile", "Happy Path Profile",
				"--complexity", "advanced",
				"--start", "scratch",
				"--out-dir", outDir,
				"--final-artifact", finalPath,
			},
			Registry:    templates.NewRegistry(),
			Persistence: persistence.NewPlaceholder(),
		})
		if err != nil {
			t.Fatalf("RunWizard returned error: %v", err)
		}
	})

	for step := stepWelcomeNum; step <= stepSmokeNum; step++ {
		token := fmt.Sprintf("[wizard][step-%d]", step)
		if !strings.Contains(stdout, token) {
			t.Fatalf("expected wizard output to include %q", token)
		}
	}
	if !strings.Contains(stdout, "[wizard] staged setup flow completed") {
		t.Fatalf("expected completion marker, got: %s", stdout)
	}

	requiredArtifacts := []string{
		filepath.Join(outDir, "studio-profile.godot.md"),
		filepath.Join(outDir, "studio-profile.godot.summary.md"),
		filepath.Join(outDir, "pack.godot.hybrid-map.json"),
		filepath.Join(outDir, "pack.godot.config.json"),
		filepath.Join(workspace, ".game-studio", "profiles", "happy-path-profile.md"),
		finalPath,
	}
	for _, artifact := range requiredArtifacts {
		if _, err := os.Stat(artifact); err != nil {
			t.Fatalf("expected artifact %s to exist: %v", artifact, err)
		}
	}

	configRaw, err := os.ReadFile(filepath.Join(workspace, "openspec", "config.yaml"))
	if err != nil {
		t.Fatalf("read openspec config: %v", err)
	}
	configText := string(configRaw)
	for _, token := range []string{"sync_refresh_placeholder:", "status: deferred", "command_hint: \"deferred placeholder: sync/refresh command is not implemented in this build\""} {
		if !strings.Contains(configText, token) {
			t.Fatalf("expected config managed block token %q", token)
		}
	}
	if strings.Contains(configText, finalPath) {
		t.Fatalf("expected managed metadata to avoid absolute final artifact path")
	}
	if !strings.Contains(configText, "final_artifact: \"out/wizard/final.artifact.json\"") {
		t.Fatalf("expected managed metadata to store workspace-relative final artifact path")
	}
}

func TestRunWizard_DoesNotConsumeProviderCredentials(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)

	// Sentinel credentials and catalog that must never influence wizard output.
	authPath := filepath.Join(workspace, "auth.json")
	authJSON := `{
	  "providers": {
	    "sentinel-opencode-provider": {"connected": true}
	  }
	}`
	if err := os.WriteFile(authPath, []byte(authJSON), 0o644); err != nil {
		t.Fatalf("write sentinel auth fixture: %v", err)
	}
	t.Setenv("OPENCODE_AUTH_PATH", authPath)
	t.Setenv("OPENCODE_MODELS_PATH", filepath.Join(workspace, "sentinel-models.json"))

	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
	stdout := captureStdout(t, func() {
		if err := RunWizard(WizardInput{
			Args:        []string{"--metadata-only", "--non-interactive", "--final-artifact", finalPath},
			Registry:    templates.NewRegistry(),
			Persistence: persistence.NewPlaceholder(),
		}); err != nil {
			t.Fatalf("RunWizard returned error: %v", err)
		}
	})

	if strings.Contains(stdout, "sentinel-opencode-provider") {
		t.Fatalf("wizard must not surface discovered providers, got: %s", stdout)
	}
	if !strings.Contains(stdout, "not-inspected") {
		t.Fatalf("expected not-inspected runtime output, got: %s", stdout)
	}

	artifact := readWizardArtifactFixture(t, finalPath)
	if len(artifact.Providers) != 0 {
		t.Fatalf("expected empty provider inventory in final artifact, got %#v", artifact.Providers)
	}
	if !strings.Contains(artifact.ProviderNotice, "did not inspect") {
		t.Fatalf("expected not-inspected provider notice, got %q", artifact.ProviderNotice)
	}

	if _, err := os.Stat(filepath.Join(workspace, ".opencode")); !os.IsNotExist(err) {
		t.Fatalf("wizard must not create a .opencode directory, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace, ".game-studio", "profiles", "game-studio.md")); err != nil {
		t.Fatalf("expected wizard profile under .game-studio/profiles: %v", err)
	}
}

func TestRunWizard_DoesNotAutoConfigureLegacyTierDefaults(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)

	authPath := filepath.Join(workspace, "auth.json")
	authJSON := `{
	  "providers": {
	    "openai": {"connected": true},
	    "anthropic": {"connected": true}
	  }
	}`
	if err := os.WriteFile(authPath, []byte(authJSON), 0o644); err != nil {
		t.Fatalf("write auth fixture: %v", err)
	}
	t.Setenv("OPENCODE_AUTH_PATH", authPath)

	outDir := filepath.Join(workspace, "out", "generated")
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
	if err := RunWizard(WizardInput{Args: []string{"--metadata-only", "--non-interactive", "--out-dir", outDir, "--final-artifact", finalPath}, Registry: templates.NewRegistry(), Persistence: persistence.NewPlaceholder()}); err != nil {
		t.Fatalf("RunWizard returned error: %v", err)
	}

	artifact := readWizardArtifactFixture(t, finalPath)
	for tier, selection := range artifact.TierDefaults {
		if selection != "not_configured:not_configured" {
			t.Fatalf("final artifact tier %s selection=%q want not_configured:not_configured", tier, selection)
		}
	}
	profileRaw, err := os.ReadFile(filepath.Join(workspace, ".game-studio", "profiles", "game-studio.md"))
	if err != nil {
		t.Fatalf("read generated wizard profile: %v", err)
	}
	profile := string(profileRaw)
	for _, token := range []string{"- tier.fast: not_configured:not_configured", "- tier.balanced: not_configured:not_configured", "- tier.deep: not_configured:not_configured"} {
		if !strings.Contains(profile, token) {
			t.Fatalf("generated wizard profile missing %q", token)
		}
	}
}

func TestRunWizard_RejectsOutputPathsOutsideWorkspace_BeforeGeneration(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))

	outsideRoot := t.TempDir()
	outsideLink := filepath.Join(workspace, "escape-root")
	if err := os.Symlink(outsideRoot, outsideLink); err != nil {
		t.Skipf("symlink not supported in this environment: %v", err)
	}
	configPath := filepath.Join(workspace, "openspec", "config.yaml")
	beforeConfig, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read openspec config before run: %v", err)
	}

	tests := []struct {
		name          string
		outDir        string
		finalArtifact string
	}{
		{
			name:          "outside out-dir",
			outDir:        filepath.Join(outsideRoot, "wizard", "generated"),
			finalArtifact: filepath.Join(workspace, "out", "wizard", "final.artifact.json"),
		},
		{
			name:          "outside final artifact",
			outDir:        filepath.Join(workspace, "out", "generated"),
			finalArtifact: filepath.Join(outsideRoot, "wizard", "final.artifact.json"),
		},
		{
			name:          "symlinked out-dir escaping workspace",
			outDir:        filepath.Join(outsideLink, "wizard", "generated"),
			finalArtifact: filepath.Join(workspace, "out", "wizard", "final.artifact.json"),
		},
		{
			name:          "symlinked final artifact escaping workspace",
			outDir:        filepath.Join(workspace, "out", "generated"),
			finalArtifact: filepath.Join(outsideLink, "wizard", "final.artifact.json"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RunWizard(WizardInput{
				Args: []string{
					"--metadata-only",
					"--non-interactive",
					"--profile", "Boundary Escape",
					"--complexity", "advanced",
					"--start", "scratch",
					"--out-dir", tt.outDir,
					"--final-artifact", tt.finalArtifact,
				},
				Registry:    templates.NewRegistry(),
				Persistence: persistence.NewPlaceholder(),
			})
			if err == nil {
				t.Fatal("expected RunWizard to fail for workspace escape attempt")
			}
			if !strings.Contains(err.Error(), "outside workspace root") {
				t.Fatalf("expected workspace escape validation error, got: %v", err)
			}

			assertPathNotExist(t, filepath.Join(workspace, "out", "generated", "studio-profile.godot.md"))
			assertPathNotExist(t, filepath.Join(workspace, ".game-studio", "profiles", "boundary-escape.md"))
			assertPathNotExist(t, filepath.Join(workspace, "out", "wizard", "final.artifact.json"))
			assertPathNotExist(t, filepath.Join(tt.outDir, "studio-profile.godot.md"))
			assertPathNotExist(t, tt.outDir)
			assertPathNotExist(t, filepath.Join(outsideRoot, "wizard"))
			assertPathNotExist(t, tt.finalArtifact)
			assertPathNotExist(t, filepath.Dir(tt.finalArtifact))

			afterConfig, readErr := os.ReadFile(configPath)
			if readErr != nil {
				t.Fatalf("read openspec config after run: %v", readErr)
			}
			if string(beforeConfig) != string(afterConfig) {
				t.Fatal("expected openspec config to remain unchanged after rejected generation")
			}
		})
	}
}

func TestRunWizard_RejectsSymlinkedGameStudioEscapingWorkspace_BeforeGeneration(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))

	outsideRoot := t.TempDir()
	outsideProfile := filepath.Join(outsideRoot, "profiles")
	outsideLink := filepath.Join(workspace, ".game-studio")
	if err := os.Symlink(outsideRoot, outsideLink); err != nil {
		t.Skipf("symlink not supported in this environment: %v", err)
	}

	configPath := filepath.Join(workspace, "openspec", "config.yaml")
	beforeConfig, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read openspec config before run: %v", err)
	}

	outDir := filepath.Join(workspace, "out", "generated")
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")

	err = RunWizard(WizardInput{
		Args: []string{
			"--metadata-only",
			"--non-interactive",
			"--profile", "Escaping Game Studio",
			"--complexity", "advanced",
			"--start", "scratch",
			"--out-dir", outDir,
			"--final-artifact", finalPath,
		},
		Registry:    templates.NewRegistry(),
		Persistence: persistence.NewPlaceholder(),
	})
	if err == nil {
		t.Fatal("expected RunWizard to fail for symlinked .game-studio")
	}
	if !strings.Contains(err.Error(), "outside workspace root") {
		t.Fatalf("expected workspace escape validation error, got: %v", err)
	}

	assertPathNotExist(t, outDir)
	assertPathNotExist(t, finalPath)
	assertPathNotExist(t, filepath.Join(workspace, ".game-studio", "profiles", "escaping-game-studio.md"))
	assertPathNotExist(t, filepath.Join(outsideProfile, "escaping-game-studio.md"))
	assertPathNotExist(t, filepath.Join(outsideRoot, "workspace.config.json"))

	afterConfig, readErr := os.ReadFile(configPath)
	if readErr != nil {
		t.Fatalf("read openspec config after run: %v", readErr)
	}
	if string(beforeConfig) != string(afterConfig) {
		t.Fatal("expected openspec config to remain unchanged after rejected generation")
	}
}

func TestRunWizard_RejectsSymlinkedOpenspecEscapingWorkspace_BeforeGeneration(t *testing.T) {
	workspace := t.TempDir()
	setWorkingDirectory(t, workspace)
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))

	outsideRoot := t.TempDir()
	outsideOpenspec := filepath.Join(outsideRoot, "openspec")
	if err := os.MkdirAll(outsideOpenspec, 0o755); err != nil {
		t.Fatalf("create outside openspec dir: %v", err)
	}
	seed := "schema: spec-driven\nstrict_tdd: false\n"
	outsideConfig := filepath.Join(outsideOpenspec, "config.yaml")
	if err := os.WriteFile(outsideConfig, []byte(seed), 0o644); err != nil {
		t.Fatalf("write outside openspec config fixture: %v", err)
	}

	if err := os.Symlink(outsideOpenspec, filepath.Join(workspace, "openspec")); err != nil {
		t.Skipf("symlink not supported in this environment: %v", err)
	}

	beforeConfig, err := os.ReadFile(filepath.Join(workspace, "openspec", "config.yaml"))
	if err != nil {
		t.Fatalf("read outside openspec config before run: %v", err)
	}

	outDir := filepath.Join(workspace, "out", "generated")
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")

	err = RunWizard(WizardInput{
		Args: []string{
			"--metadata-only",
			"--non-interactive",
			"--profile", "Escaping Openspec",
			"--complexity", "advanced",
			"--start", "scratch",
			"--out-dir", outDir,
			"--final-artifact", finalPath,
		},
		Registry:    templates.NewRegistry(),
		Persistence: persistence.NewPlaceholder(),
	})
	if err == nil {
		t.Fatal("expected RunWizard to fail for symlinked openspec")
	}
	if !strings.Contains(err.Error(), "outside workspace root") {
		t.Fatalf("expected workspace escape validation error, got: %v", err)
	}

	assertPathNotExist(t, outDir)
	assertPathNotExist(t, finalPath)
	assertPathNotExist(t, filepath.Join(workspace, ".game-studio", "profiles", "escaping-openspec.md"))

	afterConfig, readErr := os.ReadFile(filepath.Join(workspace, "openspec", "config.yaml"))
	if readErr != nil {
		t.Fatalf("read outside openspec config after run: %v", readErr)
	}
	if string(beforeConfig) != string(afterConfig) {
		t.Fatal("expected openspec config to remain unchanged after rejected generation")
	}
	assertPathNotExist(t, filepath.Join(outsideOpenspec, "escape-test-marker.txt"))
}

// TestRunWizard_RejectsSymlinkedGameStudioWithExistingOutsideProfiles covers the
// preservation gap where the escape target's deeper directory already exists.
// The old resolver stopped at the deepest existing ancestor (/outside/profiles),
// saw a directory (not a symlink), and accepted the lexical workspace path.
func TestRunWizard_RejectsSymlinkedGameStudioWithExistingOutsideProfiles(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))

	outsideRoot := t.TempDir()
	outsideProfiles := filepath.Join(outsideRoot, "profiles")
	if err := os.MkdirAll(outsideProfiles, 0o755); err != nil {
		t.Fatalf("create outside profiles dir: %v", err)
	}
	sentinelPath := filepath.Join(outsideProfiles, "game-studio.md")
	sentinelContent := "# unrelated existing profile that must not be clobbered\n"
	if err := os.WriteFile(sentinelPath, []byte(sentinelContent), 0o644); err != nil {
		t.Fatalf("write outside sentinel profile: %v", err)
	}
	if err := os.Symlink(outsideRoot, filepath.Join(workspace, ".game-studio")); err != nil {
		t.Skipf("symlink not supported in this environment: %v", err)
	}

	configPath := filepath.Join(workspace, "openspec", "config.yaml")
	beforeConfig, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read openspec config before run: %v", err)
	}

	outDir := filepath.Join(workspace, "out", "generated")
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")

	err = RunWizard(WizardInput{
		Args:        []string{"--metadata-only", "--non-interactive", "--profile", "Game-Studio", "--out-dir", outDir, "--final-artifact", finalPath},
		Registry:    templates.NewRegistry(),
		Persistence: persistence.NewPlaceholder(),
	})
	if err == nil {
		t.Fatal("expected RunWizard to refuse a .game-studio symlink with pre-existing outside profiles")
	}
	if !strings.Contains(err.Error(), "outside workspace root") {
		t.Fatalf("expected workspace escape error, got: %v", err)
	}

	afterSentinel, readErr := os.ReadFile(sentinelPath)
	if readErr != nil {
		t.Fatalf("read outside sentinel profile after run: %v", readErr)
	}
	if string(afterSentinel) != sentinelContent {
		t.Fatal("outside sentinel profile must remain unchanged")
	}
	assertPathNotExist(t, filepath.Join(outsideRoot, "workspace.config.json"))
	assertPathNotExist(t, outDir)
	assertPathNotExist(t, finalPath)

	afterConfig, readErr := os.ReadFile(configPath)
	if readErr != nil {
		t.Fatalf("read openspec config after run: %v", readErr)
	}
	if string(beforeConfig) != string(afterConfig) {
		t.Fatal("openspec config must remain unchanged after rejected generation")
	}
}

// TestRunWizard_RejectsNestedSymlinkAncestorBelowExistingDirectory covers a
// symlink escape introduced below an otherwise normal, already-existing
// directory rather than as the top-level workspace marker.
func TestRunWizard_RejectsNestedSymlinkAncestorBelowExistingDirectory(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))

	normalOut := filepath.Join(workspace, "out")
	if err := os.MkdirAll(normalOut, 0o755); err != nil {
		t.Fatalf("create normal out directory: %v", err)
	}
	outsideRoot := t.TempDir()
	escapeLink := filepath.Join(normalOut, "escape")
	if err := os.Symlink(outsideRoot, escapeLink); err != nil {
		t.Skipf("symlink not supported in this environment: %v", err)
	}

	configPath := filepath.Join(workspace, "openspec", "config.yaml")
	beforeConfig, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read openspec config before run: %v", err)
	}

	outDir := filepath.Join(escapeLink, "generated")
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")

	err = RunWizard(WizardInput{
		Args:        []string{"--metadata-only", "--non-interactive", "--profile", "Nested Escape", "--out-dir", outDir, "--final-artifact", finalPath},
		Registry:    templates.NewRegistry(),
		Persistence: persistence.NewPlaceholder(),
	})
	if err == nil {
		t.Fatal("expected RunWizard to refuse a nested symlink ancestor escaping the workspace")
	}
	if !strings.Contains(err.Error(), "outside workspace root") {
		t.Fatalf("expected workspace escape error, got: %v", err)
	}

	assertPathNotExist(t, filepath.Join(outsideRoot, "generated"))
	assertPathNotExist(t, filepath.Join(outsideRoot, "wizard"))
	assertPathNotExist(t, finalPath)
	assertPathNotExist(t, filepath.Join(workspace, ".game-studio", "workspace.config.json"))

	afterConfig, readErr := os.ReadFile(configPath)
	if readErr != nil {
		t.Fatalf("read openspec config after run: %v", readErr)
	}
	if string(beforeConfig) != string(afterConfig) {
		t.Fatal("openspec config must remain unchanged after rejected generation")
	}
}

// TestRunWizard_RefusesToClobberDifferingExistingProfile proves the collision
// refusal happens before the workspace config or any other artifact is written.
func TestRunWizard_RefusesToClobberDifferingExistingProfile(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))

	profilePath := filepath.Join(workspace, ".game-studio", "profiles", "game-studio.md")
	if err := os.MkdirAll(filepath.Dir(profilePath), 0o755); err != nil {
		t.Fatalf("create profile dir: %v", err)
	}
	existing := "# Hand-authored profile\n\nkeep me\n"
	if err := os.WriteFile(profilePath, []byte(existing), 0o644); err != nil {
		t.Fatalf("write existing profile fixture: %v", err)
	}

	configPath := filepath.Join(workspace, "openspec", "config.yaml")
	beforeConfig, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read openspec config before run: %v", err)
	}

	outDir := filepath.Join(workspace, "out", "generated")
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")

	err = RunWizard(WizardInput{
		Args:        []string{"--metadata-only", "--non-interactive", "--profile", "Game-Studio", "--out-dir", outDir, "--final-artifact", finalPath},
		Registry:    templates.NewRegistry(),
		Persistence: persistence.NewPlaceholder(),
	})
	if err == nil {
		t.Fatal("expected RunWizard to refuse overwriting a differing existing profile")
	}
	if !strings.Contains(err.Error(), "refusing to overwrite existing wizard profile") {
		t.Fatalf("expected actionable collision error, got: %v", err)
	}

	after, readErr := os.ReadFile(profilePath)
	if readErr != nil {
		t.Fatalf("read existing profile after run: %v", readErr)
	}
	if string(after) != existing {
		t.Fatal("existing differing profile must be preserved byte-for-byte")
	}
	assertPathNotExist(t, filepath.Join(workspace, ".game-studio", "workspace.config.json"))
	assertPathNotExist(t, outDir)
	assertPathNotExist(t, finalPath)

	afterConfig, readErr := os.ReadFile(configPath)
	if readErr != nil {
		t.Fatalf("read openspec config after run: %v", readErr)
	}
	if string(beforeConfig) != string(afterConfig) {
		t.Fatal("openspec config must remain unchanged when a profile collision is refused before any write")
	}
}

// TestRunWizard_RerunWithIdenticalProfileSucceeds keeps an idempotent rerun of
// unchanged, byte-identical generated content working.
func TestRunWizard_RerunWithIdenticalProfileSucceeds(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))

	profilePath := filepath.Join(workspace, ".game-studio", "profiles", "game-studio.md")
	args := []string{"--non-interactive", "--profile", "Game-Studio", "--out-dir", filepath.Join(workspace, "out", "generated"), "--final-artifact", filepath.Join(workspace, "out", "wizard", "final.artifact.json")}

	if err := RunWizard(WizardInput{Args: append([]string{"--metadata-only"}, args...), Registry: templates.NewRegistry(), Persistence: persistence.NewPlaceholder()}); err != nil {
		t.Fatalf("first RunWizard returned error: %v", err)
	}
	first, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("read generated profile: %v", err)
	}

	if err := RunWizard(WizardInput{Args: append([]string{"--metadata-only"}, args...), Registry: templates.NewRegistry(), Persistence: persistence.NewPlaceholder()}); err != nil {
		t.Fatalf("rerun over byte-identical profile must succeed, got: %v", err)
	}
	second, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("read profile after rerun: %v", err)
	}
	if string(first) != string(second) {
		t.Fatal("rerun must reuse the byte-identical profile")
	}
}

// TestRunWizard_AllowsWorkspaceInternalGameStudioSymlink guards the legitimate
// case: a .game-studio symlink whose physical target is still inside the
// workspace must not be rejected as an escape.
func TestRunWizard_AllowsWorkspaceInternalGameStudioSymlink(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))

	physicalGameStudio := filepath.Join(workspace, "internal-game-studio")
	if err := os.MkdirAll(physicalGameStudio, 0o755); err != nil {
		t.Fatalf("create internal game-studio dir: %v", err)
	}
	if err := os.Symlink(physicalGameStudio, filepath.Join(workspace, ".game-studio")); err != nil {
		t.Skipf("symlink not supported in this environment: %v", err)
	}

	outDir := filepath.Join(workspace, "out", "generated")
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
	if err := RunWizard(WizardInput{
		Args:        []string{"--metadata-only", "--non-interactive", "--profile", "Game-Studio", "--out-dir", outDir, "--final-artifact", finalPath},
		Registry:    templates.NewRegistry(),
		Persistence: persistence.NewPlaceholder(),
	}); err != nil {
		t.Fatalf("RunWizard must allow an in-workspace .game-studio symlink, got: %v", err)
	}

	if _, err := os.Stat(filepath.Join(physicalGameStudio, "profiles", "game-studio.md")); err != nil {
		t.Fatalf("expected profile at the physical in-workspace target: %v", err)
	}
	if _, err := os.Stat(filepath.Join(physicalGameStudio, "workspace.config.json")); err != nil {
		t.Fatalf("expected workspace config at the physical in-workspace target: %v", err)
	}
}

// TestRunWizard_RefusesLeafSymlinkProfileBeforeAnyWrite covers the full flow:
// a symlink at the ORIGINAL lexical profile leaf must be refused even when it
// resolves to byte-identical, in-workspace content. The physical path returned by
// resolveWizardSafePath alone would erase the leaf identity and let the reuse path
// succeed.
func TestRunWizard_RefusesLeafSymlinkProfileBeforeAnyWrite(t *testing.T) {
	tests := []struct {
		name            string
		setup           func(t *testing.T, workspace string) (linkPath, targetPath, targetContent string)
		targetPreserved bool
	}{
		{
			name: "symlink to byte-identical generated content inside workspace",
			setup: func(t *testing.T, workspace string) (string, string, string) {
				t.Helper()
				firstOut := filepath.Join(workspace, "first-out", "generated")
				firstFinal := filepath.Join(workspace, "first-out", "wizard", "final.artifact.json")
				captureStdout(t, func() {
					if err := RunWizard(WizardInput{
						Args:        []string{"--metadata-only", "--non-interactive", "--profile", "Game-Studio", "--out-dir", firstOut, "--final-artifact", firstFinal},
						Registry:    templates.NewRegistry(),
						Persistence: persistence.NewPlaceholder(),
					}); err != nil {
						t.Fatalf("first RunWizard returned error: %v", err)
					}
				})
				linkPath := filepath.Join(workspace, ".game-studio", "profiles", "game-studio.md")
				raw, err := os.ReadFile(linkPath)
				if err != nil {
					t.Fatalf("read first generated profile: %v", err)
				}
				targetPath := filepath.Join(workspace, ".game-studio", "profiles", "game-studio-real.md")
				if err := os.Rename(linkPath, targetPath); err != nil {
					t.Fatalf("move generated profile to symlink target: %v", err)
				}
				return linkPath, targetPath, string(raw)
			},
			targetPreserved: true,
		},
		{
			name: "symlink to differing content inside workspace",
			setup: func(t *testing.T, workspace string) (string, string, string) {
				t.Helper()
				profilesDir := filepath.Join(workspace, ".game-studio", "profiles")
				if err := os.MkdirAll(profilesDir, 0o755); err != nil {
					t.Fatalf("create profiles dir: %v", err)
				}
				targetPath := filepath.Join(profilesDir, "game-studio-real.md")
				content := "# hand-authored different profile\n"
				if err := os.WriteFile(targetPath, []byte(content), 0o644); err != nil {
					t.Fatalf("write differing symlink target: %v", err)
				}
				return filepath.Join(profilesDir, "game-studio.md"), targetPath, content
			},
			targetPreserved: true,
		},
		{
			name: "dangling symlink",
			setup: func(t *testing.T, workspace string) (string, string, string) {
				t.Helper()
				profilesDir := filepath.Join(workspace, ".game-studio", "profiles")
				if err := os.MkdirAll(profilesDir, 0o755); err != nil {
					t.Fatalf("create profiles dir: %v", err)
				}
				return filepath.Join(profilesDir, "game-studio.md"), filepath.Join(profilesDir, "missing-target.md"), ""
			},
			targetPreserved: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workspace := wizardWorkspaceFixture(t)
			setWorkingDirectory(t, workspace)
			t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))

			linkPath, targetPath, targetContent := tt.setup(t, workspace)
			if err := os.Symlink(filepath.Base(targetPath), linkPath); err != nil {
				t.Skipf("symlink not supported in this environment: %v", err)
			}

			workspaceConfigPath := filepath.Join(workspace, ".game-studio", "workspace.config.json")
			if err := os.Remove(workspaceConfigPath); err != nil && !os.IsNotExist(err) {
				t.Fatalf("clear workspace config sentinel: %v", err)
			}
			openspecConfigPath := filepath.Join(workspace, "openspec", "config.yaml")
			beforeOpenSpec, err := os.ReadFile(openspecConfigPath)
			if err != nil {
				t.Fatalf("read openspec config before run: %v", err)
			}

			outDir := filepath.Join(workspace, "second-out", "generated")
			finalPath := filepath.Join(workspace, "second-out", "wizard", "final.artifact.json")

			err = RunWizard(WizardInput{
				Args:        []string{"--metadata-only", "--non-interactive", "--profile", "Game-Studio", "--out-dir", outDir, "--final-artifact", finalPath},
				Registry:    templates.NewRegistry(),
				Persistence: persistence.NewPlaceholder(),
			})
			if err == nil {
				t.Fatal("expected RunWizard to refuse a symlink at the original profile leaf")
			}
			if !strings.Contains(err.Error(), "symlink") {
				t.Fatalf("expected symlink refusal, got: %v", err)
			}

			linkInfo, statErr := os.Lstat(linkPath)
			if statErr != nil {
				t.Fatalf("stat profile leaf after refusal: %v", statErr)
			}
			if linkInfo.Mode()&os.ModeSymlink == 0 {
				t.Fatal("profile leaf symlink must be preserved")
			}
			if tt.targetPreserved {
				afterTarget, readErr := os.ReadFile(targetPath)
				if readErr != nil {
					t.Fatalf("read symlink target after refusal: %v", readErr)
				}
				if string(afterTarget) != targetContent {
					t.Fatal("symlink target bytes must be preserved")
				}
			}

			assertPathNotExist(t, workspaceConfigPath)
			assertPathNotExist(t, outDir)
			assertPathNotExist(t, finalPath)

			afterOpenSpec, readErr := os.ReadFile(openspecConfigPath)
			if readErr != nil {
				t.Fatalf("read openspec config after refusal: %v", readErr)
			}
			if string(beforeOpenSpec) != string(afterOpenSpec) {
				t.Fatal("openspec config must remain unchanged when the leaf symlink is refused")
			}
		})
	}
}

// TestWriteWizardProfileContent_PreservesExistingTargets is the behavior-level
// contract for the profile collision policy: reuse identical content, refuse
// everything else without clobbering, and create fresh content exclusively.
func TestWriteWizardProfileContent_PreservesExistingTargets(t *testing.T) {
	t.Run("reuses byte-identical profile", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "game-studio.md")
		if err := os.WriteFile(path, []byte("same"), 0o644); err != nil {
			t.Fatalf("seed identical profile: %v", err)
		}
		if err := writeWizardProfileContent(path, "same"); err != nil {
			t.Fatalf("identical profile must be reused, got: %v", err)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read profile: %v", err)
		}
		if string(raw) != "same" {
			t.Fatalf("profile content changed: %q", raw)
		}
	})

	t.Run("refuses differing regular profile", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "game-studio.md")
		if err := os.WriteFile(path, []byte("hand-authored"), 0o644); err != nil {
			t.Fatalf("seed differing profile: %v", err)
		}
		err := writeWizardProfileContent(path, "generated")
		if err == nil {
			t.Fatal("expected refusal for differing existing profile")
		}
		if !strings.Contains(err.Error(), "refusing to overwrite existing wizard profile") {
			t.Fatalf("expected actionable refusal, got: %v", err)
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read profile: %v", readErr)
		}
		if string(raw) != "hand-authored" {
			t.Fatalf("differing profile must not be clobbered, got: %q", raw)
		}
	})

	t.Run("refuses symlink target without writing through it", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "real.md")
		if err := os.WriteFile(target, []byte("outside"), 0o644); err != nil {
			t.Fatalf("seed symlink target: %v", err)
		}
		link := filepath.Join(dir, "game-studio.md")
		if err := os.Symlink(target, link); err != nil {
			t.Skipf("symlink not supported in this environment: %v", err)
		}
		err := writeWizardProfileContent(link, "generated")
		if err == nil {
			t.Fatal("expected refusal for symlinked profile target")
		}
		if !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("expected symlink refusal, got: %v", err)
		}
		raw, readErr := os.ReadFile(target)
		if readErr != nil {
			t.Fatalf("read symlink target: %v", readErr)
		}
		if string(raw) != "outside" {
			t.Fatalf("symlink target must not be written through, got: %q", raw)
		}
	})

	t.Run("refuses non-regular target", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "game-studio.md")
		if err := os.Mkdir(path, 0o755); err != nil {
			t.Fatalf("create directory target: %v", err)
		}
		err := writeWizardProfileContent(path, "generated")
		if err == nil {
			t.Fatal("expected refusal for non-regular profile target")
		}
		if !strings.Contains(err.Error(), "not a regular file") {
			t.Fatalf("expected non-regular refusal, got: %v", err)
		}
	})

	t.Run("creates a fresh profile exclusively", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "game-studio.md")
		if err := writeWizardProfileContent(path, "generated"); err != nil {
			t.Fatalf("fresh profile write failed: %v", err)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read fresh profile: %v", err)
		}
		if string(raw) != "generated" {
			t.Fatalf("fresh profile content=%q want generated", raw)
		}
	})
}

func TestRunWizard_SanitizesUnsafeProfileNameForTemplateArtifacts(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))

	rawProfile := "Bad \"Quoted\"\nName"
	expectedProfile := "Bad 'Quoted' Name"
	outDir := filepath.Join(workspace, "out", "generated")
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")

	err := RunWizard(WizardInput{
		Args: []string{
			"--metadata-only",
			"--non-interactive",
			"--profile", rawProfile,
			"--complexity", "advanced",
			"--start", "scratch",
			"--out-dir", outDir,
			"--final-artifact", finalPath,
		},
		Registry:    templates.NewRegistry(),
		Persistence: persistence.NewPlaceholder(),
	})
	if err != nil {
		t.Fatalf("RunWizard returned error: %v", err)
	}

	artifact := readWizardArtifactFixture(t, finalPath)
	if artifact.Profile != expectedProfile {
		t.Fatalf("expected sanitized profile %q, got %q", expectedProfile, artifact.Profile)
	}
	if strings.ContainsAny(artifact.Profile, "\"\n\r") {
		t.Fatalf("expected sanitized profile to avoid unsafe chars, got %q", artifact.Profile)
	}

	var pattern map[string]any
	patternPath := filepath.Join(outDir, "pack.godot.hybrid-map.json")
	rawPattern, err := os.ReadFile(patternPath)
	if err != nil {
		t.Fatalf("read generated pattern artifact: %v", err)
	}
	if err := json.Unmarshal(rawPattern, &pattern); err != nil {
		t.Fatalf("pattern artifact must remain valid JSON: %v", err)
	}
	if got, ok := pattern["profile"].(string); !ok || got != expectedProfile {
		t.Fatalf("expected pattern profile %q, got %#v", expectedProfile, pattern["profile"])
	}

	markdownPath := filepath.Join(workspace, ".game-studio", "profiles", "bad-quoted-name.md")
	rawMarkdown, err := os.ReadFile(markdownPath)
	if err != nil {
		t.Fatalf("read generated wizard profile markdown: %v", err)
	}
	if !strings.Contains(string(rawMarkdown), "# "+expectedProfile) {
		t.Fatalf("expected generated markdown to include sanitized heading %q", "# "+expectedProfile)
	}
}

func TestRunWizard_GenerationAttemptsEngramWriteThrough(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))

	spyDir := filepath.Join(workspace, "spy-bin")
	if err := os.MkdirAll(spyDir, 0o755); err != nil {
		t.Fatalf("create spy bin dir: %v", err)
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

	err := RunWizard(WizardInput{
		Args: []string{
			"--metadata-only",
			"--non-interactive",
			"--profile", "Write Through Profile",
			"--complexity", "advanced",
			"--start", "scratch",
			"--out-dir", filepath.Join(workspace, "out", "generated"),
			"--final-artifact", filepath.Join(workspace, "out", "wizard", "final.artifact.json"),
		},
		Registry:    templates.NewRegistry(),
		Persistence: persistence.NewPlaceholder(),
	})
	if err != nil {
		t.Fatalf("RunWizard returned error: %v", err)
	}

	spyLog, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read engram spy log: %v", err)
	}
	logText := string(spyLog)
	if !strings.Contains(logText, "title=installer-wizard/final-artifact/godot") {
		t.Fatalf("expected engram save title in spy log, got: %s", logText)
	}

	start := strings.Index(logText, "body-begin\n")
	end := strings.LastIndex(logText, "body-end\n")
	if start == -1 || end == -1 || end <= start {
		t.Fatalf("expected engram body markers in spy log, got: %s", logText)
	}
	body := logText[start+len("body-begin\n") : end]

	if strings.Contains(body, workspace) {
		t.Fatalf("expected workspace-relative payload in engram body, got: %s", body)
	}
	if strings.Contains(body, "- none") {
		t.Fatalf("expected no '- none' artifact sentinel in engram body, got: %s", body)
	}
	if !strings.Contains(body, "summary_artifact: out/wizard/final.artifact.json") {
		t.Fatalf("expected relative summary path in engram body, got: %s", body)
	}
}

func TestRunWizard_ScratchRecordsNotInspectedProviders(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))

	outDir := filepath.Join(workspace, "out", "generated")
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")

	input := strings.Join([]string{
		"create_from_scratch", // use_mode
		"godot-core",          // engine_pack
		"recommended",         // setup_depth
		"No Auth Profile",     // profile name
		"godot-docs",          // connectors
		"godot-core",          // packs
		"formatter",           // tools
		"",                    // optional integrations
		"yes",                 // confirmation
	}, "\n") + "\n"

	stdout := captureStdout(t, func() {
		err := withStdinText(t, input, func() error {
			return RunWizard(WizardInput{
				Args: []string{
					"--metadata-only",
					"--profile", "Game-Studio",
					"--complexity", "advanced",
					"--start", "scratch",
					"--out-dir", outDir,
					"--final-artifact", finalPath,
				},
				Registry:    templates.NewRegistry(),
				Persistence: persistence.NewPlaceholder(),
			})
		})
		if err != nil {
			t.Fatalf("RunWizard returned error: %v", err)
		}
	})

	if !strings.Contains(stdout, wizardProviderNotice) {
		t.Fatalf("expected not-inspected provider notice in output, got: %s", stdout)
	}

	artifact := readWizardArtifactFixture(t, finalPath)
	if artifact.StartingPoint != "scratch" {
		t.Fatalf("expected starting_point scratch, got %q", artifact.StartingPoint)
	}
	if !strings.Contains(artifact.ProviderNotice, "did not inspect") {
		t.Fatalf("expected provider notice to state providers were not inspected, got %q", artifact.ProviderNotice)
	}
	if len(artifact.Providers) != 0 {
		t.Fatalf("expected empty provider list in machine artifact, got %#v", artifact.Providers)
	}
	if len(artifact.MCP["optional"]) != 0 {
		t.Fatalf("expected optional MCP list to remain empty, got %#v", artifact.MCP["optional"])
	}
}

func TestRunWizard_ConfigManagedBlock_QuotesUnsafeValues(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))

	profileName := "Bad: injected"
	err := RunWizard(WizardInput{
		Args: []string{
			"--metadata-only",
			"--non-interactive",
			"--profile", profileName,
			"--complexity", "advanced",
			"--start", "scratch",
			"--out-dir", filepath.Join(workspace, "out", "generated"),
			"--final-artifact", filepath.Join(workspace, "out", "wizard", "final.artifact.json"),
		},
		Registry:    templates.NewRegistry(),
		Persistence: persistence.NewPlaceholder(),
	})
	if err != nil {
		t.Fatalf("RunWizard returned error: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(workspace, "openspec", "config.yaml"))
	if err != nil {
		t.Fatalf("read openspec config: %v", err)
	}
	text := string(raw)

	if strings.Contains(text, "name: injected") {
		t.Fatalf("expected profile_name to be YAML-quoted/sanitized, got raw injected key")
	}
	if !strings.Contains(text, "profile_name: \"Bad: injected\"") {
		t.Fatalf("expected sanitized/quoted profile_name in managed metadata")
	}
	if strings.Contains(text, "--sync-refresh") {
		t.Fatalf("managed config must not advertise non-existent sync-refresh command")
	}
}

func TestRunWizard_ScratchWithoutExistingOpenspecConfig_BootstrapsManagedConfig(t *testing.T) {
	workspace := t.TempDir()
	setWorkingDirectory(t, workspace)
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))

	outDir := filepath.Join(workspace, "out", "generated")
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")

	err := RunWizard(WizardInput{
		Args: []string{
			"--metadata-only",
			"--non-interactive",
			"--profile", "Brand New",
			"--complexity", "advanced",
			"--start", "scratch",
			"--out-dir", outDir,
			"--final-artifact", finalPath,
		},
		Registry:    templates.NewRegistry(),
		Persistence: persistence.NewPlaceholder(),
	})
	if err != nil {
		t.Fatalf("RunWizard returned error: %v", err)
	}

	configRaw, err := os.ReadFile(filepath.Join(workspace, "openspec", "config.yaml"))
	if err != nil {
		t.Fatalf("read openspec config: %v", err)
	}
	configText := string(configRaw)

	for _, token := range []string{
		"# BEGIN GAME-STUDIO WIZARD (managed)",
		"profile_name: \"Brand New\"",
		"profile_artifact: \".game-studio/profiles/brand-new.md\"",
		"final_artifact: \"out/wizard/final.artifact.json\"",
	} {
		if !strings.Contains(configText, token) {
			t.Fatalf("expected openspec managed config token %q", token)
		}
	}
	if strings.Contains(configText, "profile_artifact: \"\"") {
		t.Fatalf("unexpected empty profile_artifact metadata: %s", configText)
	}
	if strings.Contains(configText, "final_artifact: \"\"") {
		t.Fatalf("unexpected empty final_artifact metadata: %s", configText)
	}
}

func TestWriteWizardManagedBlock_RejectsInvalidMetadataPath(t *testing.T) {
	workspace := t.TempDir()

	err := writeWizardManagedBlock(workspace, "Brand New", "", "")
	if err == nil {
		t.Fatal("expected writeWizardManagedBlock to fail with empty metadata paths")
	}
	if !strings.Contains(err.Error(), "resolve managed profile artifact path") {
		t.Fatalf("expected profile path error, got: %v", err)
	}
}

func TestRunWizard_FromNestedDirectory_WritesToWorkspaceRoot(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	nested := filepath.Join(workspace, "tmp", "nested", "cwd")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("create nested working directory: %v", err)
	}
	setWorkingDirectory(t, nested)
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))

	err := RunWizard(WizardInput{
		Args: []string{
			"--metadata-only",
			"--non-interactive",
			"--profile", "Anchored Profile",
			"--complexity", "advanced",
			"--start", "scratch",
			"--out-dir", "out/generated",
			"--final-artifact", "out/wizard/final.artifact.json",
		},
		Registry:    templates.NewRegistry(),
		Persistence: persistence.NewPlaceholder(),
	})
	if err != nil {
		t.Fatalf("RunWizard returned error: %v", err)
	}

	rootManagedPath := filepath.Join(workspace, "openspec", "config.yaml")
	nestedManagedPath := filepath.Join(nested, "openspec", "config.yaml")

	rawRoot, err := os.ReadFile(rootManagedPath)
	if err != nil {
		t.Fatalf("read root openspec config: %v", err)
	}
	if !strings.Contains(string(rawRoot), "# BEGIN GAME-STUDIO WIZARD (managed)") {
		t.Fatalf("expected managed block in workspace-root openspec config")
	}
	if _, err := os.Stat(nestedManagedPath); !os.IsNotExist(err) {
		t.Fatalf("expected no openspec/config.yaml written under nested cwd, err=%v", err)
	}

	if _, err := os.Stat(filepath.Join(workspace, ".game-studio", "profiles", "anchored-profile.md")); err != nil {
		t.Fatalf("expected profile generated under workspace root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace, "out", "wizard", "final.artifact.json")); err != nil {
		t.Fatalf("expected final artifact under workspace root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(nested, "out", "wizard", "final.artifact.json")); !os.IsNotExist(err) {
		t.Fatalf("expected no final artifact under nested cwd, err=%v", err)
	}
}

func TestRunWizard_ProfileRenameReflectedInGeneratedArtifacts(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))

	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
	profileName := "My Studio Alpha"

	input := strings.Join([]string{
		"create_from_scratch",
		"godot-core",
		"recommended",
		profileName,
		"godot-docs",
		"godot-core",
		"formatter",
		"",
		"yes",
	}, "\n") + "\n"

	err := withStdinText(t, input, func() error {
		return RunWizard(WizardInput{
			Args: []string{
				"--metadata-only",
				"--profile", "Game-Studio",
				"--complexity", "advanced",
				"--start", "scratch",
				"--out-dir", filepath.Join(workspace, "out", "generated"),
				"--final-artifact", finalPath,
			},
			Registry:    templates.NewRegistry(),
			Persistence: persistence.NewPlaceholder(),
		})
	})
	if err != nil {
		t.Fatalf("RunWizard returned error: %v", err)
	}

	rawProfilePath := filepath.Join(workspace, ".game-studio", "profiles", "my-studio-alpha.md")
	rawProfile, err := os.ReadFile(rawProfilePath)
	if err != nil {
		t.Fatalf("read generated profile markdown: %v", err)
	}
	if !strings.Contains(string(rawProfile), "# "+profileName) {
		t.Fatalf("expected renamed heading in profile markdown")
	}

	artifact := readWizardArtifactFixture(t, finalPath)
	if artifact.Profile != profileName {
		t.Fatalf("expected final artifact profile %q, got %q", profileName, artifact.Profile)
	}
}

func TestRunWizard_ResourceSelectionDoesNotMixMCPSelections(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))

	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")

	input := strings.Join([]string{
		"create_from_scratch",    // use_mode
		"godot-core",             // engine_pack
		"recommended",            // setup_depth
		"Split MCP Profile",      // profile
		"godot-docs,asset-store", // connectors
		"godot-core",             // packs
		"formatter,linter",       // tools
		"",                       // optional integrations
		"yes",                    // confirm
	}, "\n") + "\n"

	err := withStdinText(t, input, func() error {
		return RunWizard(WizardInput{
			Args: []string{
				"--metadata-only",
				"--profile", "Game-Studio",
				"--complexity", "advanced",
				"--start", "scratch",
				"--out-dir", filepath.Join(workspace, "out", "generated"),
				"--final-artifact", finalPath,
			},
			Registry:    templates.NewRegistry(),
			Persistence: persistence.NewPlaceholder(),
		})
	})
	if err != nil {
		t.Fatalf("RunWizard returned error: %v", err)
	}

	artifact := readWizardArtifactFixture(t, finalPath)
	for _, picked := range append(append([]string{}, artifact.Connectors...), append(artifact.Packs, artifact.Tools...)...) {
		if picked == "engram" || picked == "context7" || picked == "browser" || picked == "memory" {
			t.Fatalf("resource selections must not include MCP values, found %q", picked)
		}
	}

	if len(artifact.MCP["required"]) != 0 {
		t.Fatalf("staged wizard must not require MCP adapters, got %#v", artifact.MCP["required"])
	}
	if len(artifact.MCP["optional"]) != 0 {
		t.Fatalf("expected optional MCPs to stay deferred in staged flow, got %#v", artifact.MCP["optional"])
	}
}

func TestRunWizard_OptionalIntegrationsSelectionAndMetronousConsent(t *testing.T) {
	tests := []struct {
		name           string
		optionals      string
		consentAnswers []string
		wantIDs        []string
		wantConsent    bool
	}{
		{name: "engram monitor selected", optionals: integrations.EngramMonitorID, wantIDs: []string{integrations.EngramMonitorID}},
		{name: "metronous consent granted", optionals: integrations.MetronousID, consentAnswers: []string{"yes"}, wantIDs: []string{integrations.MetronousID}, wantConsent: true},
		{name: "metronous consent declined", optionals: integrations.MetronousID, consentAnswers: []string{"no"}, wantIDs: []string{}},
		{name: "metronous consent missing", optionals: integrations.MetronousID, consentAnswers: []string{""}, wantIDs: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workspace := wizardWorkspaceFixture(t)
			setWorkingDirectory(t, workspace)
			t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))
			finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")

			answers := []string{
				"create_from_scratch",
				"godot-core",
				"recommended",
				"Optional Profile",
				"godot-docs",
				"godot-core",
				"formatter",
				tt.optionals,
			}
			answers = append(answers, tt.consentAnswers...)
			answers = append(answers, "yes")

			err := withStdinText(t, strings.Join(answers, "\n")+"\n", func() error {
				return RunWizard(WizardInput{
					Args:        []string{"--metadata-only", "--final-artifact", finalPath},
					Registry:    templates.NewRegistry(),
					Persistence: persistence.NewPlaceholder(),
				})
			})
			if err != nil {
				t.Fatalf("RunWizard returned error: %v", err)
			}

			artifact := readWizardArtifactFixture(t, finalPath)
			gotIDs := make([]string, 0, len(artifact.OptionalIntegrations.Selected))
			for _, selection := range artifact.OptionalIntegrations.Selected {
				gotIDs = append(gotIDs, selection.ID)
				if selection.ID == integrations.MetronousID && selection.Consent[integrations.TelemetryPrivacyConsentID] != tt.wantConsent {
					t.Fatalf("metronous consent=%#v want telemetry_privacy=%v", selection.Consent, tt.wantConsent)
				}
			}
			if !sameStrings(gotIDs, tt.wantIDs) {
				t.Fatalf("selected optionals=%v want %v", gotIDs, tt.wantIDs)
			}

			profileRaw, err := os.ReadFile(filepath.Join(workspace, ".game-studio", "profiles", "optional-profile.md"))
			if err != nil {
				t.Fatalf("read generated profile: %v", err)
			}
			profile := string(profileRaw)
			if !strings.Contains(profile, "## Optional Integrations") || strings.Contains(profile, "systemctl") || strings.Contains(profile, "git clone") {
				t.Fatalf("profile must include metadata-only optional section without service/install instructions: %s", profile)
			}
			if !sameStrings(artifact.Tools, []string{"formatter"}) {
				t.Fatalf("tools changed by optional integrations: %#v", artifact.Tools)
			}
			if len(artifact.MCP["required"]) != 0 || len(artifact.MCP["optional"]) != 0 {
				t.Fatalf("optional integrations must not require MCP adapters: %#v", artifact.MCP)
			}
		})
	}
}

func TestRunWizard_CustomAssetAdapterIntegrationIsMetadataOnly(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")

	const assetAdapterID = "aseprite-asset-adapter"
	catalog := integrations.NewRegistry(integrations.Integration{
		ID:              assetAdapterID,
		Label:           "Aseprite Asset Adapter",
		Category:        "asset-adapter",
		Description:     "Metadata-only future asset adapter selection.",
		DefaultSelected: false,
		Hints:           []string{"Selection records intent only; no install, clone, build, or run is performed."},
	})

	input := strings.Join([]string{
		"create_from_scratch",
		"godot-core",
		"recommended",
		"Asset Adapter Profile",
		"godot-docs",
		"godot-core",
		"formatter",
		assetAdapterID,
		"yes",
	}, "\n") + "\n"

	stdout := captureStdout(t, func() {
		err := withStdinText(t, input, func() error {
			return RunWizard(WizardInput{
				Args:                       []string{"--metadata-only", "--final-artifact", finalPath},
				Registry:                   templates.NewRegistry(),
				Persistence:                persistence.NewPlaceholder(),
				OptionalIntegrationCatalog: catalog,
			})
		})
		if err != nil {
			t.Fatalf("RunWizard returned error: %v", err)
		}
	})

	if !strings.Contains(stdout, "asset-adapter") || !strings.Contains(stdout, "metadata only; not installed or started") {
		t.Fatalf("expected asset-adapter metadata-only runtime output, got: %s", stdout)
	}
	for _, forbidden := range []string{"git clone", "go build", "npm install", "systemctl", "service start", "service stop"} {
		if strings.Contains(stdout, forbidden) {
			t.Fatalf("asset-adapter flow must not emit external action %q: %s", forbidden, stdout)
		}
	}

	artifact := readWizardArtifactFixture(t, finalPath)
	if len(artifact.OptionalIntegrations.Selected) != 0 {
		t.Fatalf("asset adapter with insufficient metadata must not appear as generic optional: %#v", artifact.OptionalIntegrations.Selected)
	}
	if len(artifact.OptionalIntegrations.UnknownOptional) != 1 || artifact.OptionalIntegrations.UnknownOptional[0].ID != assetAdapterID || artifact.OptionalIntegrations.UnknownOptional[0].Status != "deferred_review" {
		t.Fatalf("unknown optional=%#v, want deferred_review for %q", artifact.OptionalIntegrations.UnknownOptional, assetAdapterID)
	}
	if !sameStrings(artifact.Tools, []string{"formatter"}) {
		t.Fatalf("tools changed by asset-adapter optional integration: %#v", artifact.Tools)
	}
	if len(artifact.MCP["required"]) != 0 || len(artifact.MCP["optional"]) != 0 {
		t.Fatalf("asset-adapter optional integration must not require MCP adapters: %#v", artifact.MCP)
	}
}

func TestRunWizard_DeferredVisualAssetAdapterRequestRecordsFutureScope(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")

	input := strings.Join([]string{
		"create_from_scratch",
		"godot-core",
		"recommended",
		"Deferred Adapter Profile",
		"godot-docs",
		"godot-core",
		"formatter",
		"hunyuan3d",
		"yes",
	}, "\n") + "\n"

	stdout := captureStdout(t, func() {
		err := withStdinText(t, input, func() error {
			return RunWizard(WizardInput{Args: []string{"--metadata-only", "--final-artifact", finalPath}, Registry: templates.NewRegistry(), Persistence: persistence.NewPlaceholder()})
		})
		if err != nil {
			t.Fatalf("RunWizard returned error: %v", err)
		}
	})

	if !strings.Contains(stdout, "deferred future scope") || !strings.Contains(stdout, "hunyuan3d") {
		t.Fatalf("expected deferred adapter warning, got: %s", stdout)
	}
	artifact := readWizardArtifactFixture(t, finalPath)
	if len(artifact.OptionalIntegrations.Selected) != 0 || len(artifact.AssetPipeline.SelectedAdapters) != 0 {
		t.Fatalf("deferred adapter must not be selected: %#v", artifact.AssetPipeline.SelectedAdapters)
	}
	if len(artifact.AssetPipeline.DeferredRequests) != 1 || !strings.Contains(artifact.AssetPipeline.DeferredRequests[0], "hunyuan3d") || !strings.Contains(artifact.AssetPipeline.DeferredRequests[0], "future") {
		t.Fatalf("deferred request reason not recorded: %#v", artifact.AssetPipeline.DeferredRequests)
	}
}

func TestRunWizard_DefaultVisualAssetAdaptersPersistAsMetadataOnly(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")

	input := strings.Join([]string{
		"visual_artifacts_only",
		"godot-core",
		"recommended",
		"Visual Assets Profile",
		"godot-docs",
		"godot-core",
		"formatter",
		integrations.ComfyUIWorkflowsID + "," + integrations.BlenderReferenceModelingID,
		"yes",
	}, "\n") + "\n"

	stdout := captureStdout(t, func() {
		err := withStdinText(t, input, func() error {
			return RunWizard(WizardInput{Args: []string{"--metadata-only", "--final-artifact", finalPath}, Registry: templates.NewRegistry(), Persistence: persistence.NewPlaceholder()})
		})
		if err != nil {
			t.Fatalf("RunWizard returned error: %v", err)
		}
	})

	if !strings.Contains(stdout, "asset_pipeline") || !strings.Contains(stdout, "metadata only; no assets generated") || !strings.Contains(stdout, "visual_workflow") {
		t.Fatalf("expected metadata-only asset pipeline summary, got: %s", stdout)
	}
	for _, forbidden := range []string{"tools installed", "models downloaded", "APIs called", "assets generated"} {
		if strings.Contains(stdout, forbidden+": true") {
			t.Fatalf("wizard stdout must not claim %q: %s", forbidden, stdout)
		}
	}

	artifact := readWizardArtifactFixture(t, finalPath)
	if artifact.AssetPipeline.ContractVersion != assets.SchemaVersion {
		t.Fatalf("asset pipeline contract=%q want %q", artifact.AssetPipeline.ContractVersion, assets.SchemaVersion)
	}
	gotIDs := []string{}
	for _, adapter := range artifact.AssetPipeline.SelectedAdapters {
		gotIDs = append(gotIDs, adapter.ID)
		if adapter.ExecutionOwnership != assets.ExecutionMetadataOnly {
			t.Fatalf("adapter must be metadata only: %#v", adapter)
		}
	}
	if !sameStrings(gotIDs, []string{integrations.ComfyUIWorkflowsID, integrations.BlenderReferenceModelingID}) {
		t.Fatalf("selected asset adapters=%v", gotIDs)
	}
	if len(artifact.OptionalIntegrations.Selected) != 0 {
		t.Fatalf("visual workflow adapters must not appear as generic optionals: %#v", artifact.OptionalIntegrations.Selected)
	}
	workflowAdapterIDs := []string{}
	for _, adapter := range artifact.WorkflowAdapters {
		workflowAdapterIDs = append(workflowAdapterIDs, adapter.ID)
		if adapter.Category != "workflow_adapter" || adapter.Status != "metadata_selected" {
			t.Fatalf("workflow adapter taxonomy must be explicit: %#v", adapter)
		}
	}
	if !sameStrings(workflowAdapterIDs, []string{integrations.ComfyUIWorkflowsID, integrations.BlenderReferenceModelingID}) {
		t.Fatalf("workflow adapters=%v", workflowAdapterIDs)
	}
	for claim, value := range artifact.AssetPipeline.NoExecutionClaims {
		if value {
			t.Fatalf("no-execution claim %q must be false", claim)
		}
	}
	if artifact.VisualWorkflow.ContractVersion != assets.VisualWorkflowVersion {
		t.Fatalf("visual workflow contract=%q want %q", artifact.VisualWorkflow.ContractVersion, assets.VisualWorkflowVersion)
	}
	if artifact.VisualWorkflow.ArtBible.ApprovalState != assets.VisualStatusPendingApproval || artifact.VisualWorkflow.Readiness.Status == assets.VisualStatusReadyForAdapter {
		t.Fatalf("visual workflow must not imply approved/ready art: %#v", artifact.VisualWorkflow)
	}
	for claim, value := range artifact.VisualWorkflow.NoExecutionClaims {
		if value {
			t.Fatalf("visual workflow no-execution claim %q must be false", claim)
		}
	}
	for _, route := range []string{"/art-bible", "/asset-spec", "/asset-audit"} {
		if !containsString(artifact.VisualWorkflow.GuidanceRoutes, route) {
			t.Fatalf("visual workflow route %q missing: %#v", route, artifact.VisualWorkflow.GuidanceRoutes)
		}
	}

	profileRaw, err := os.ReadFile(filepath.Join(workspace, ".game-studio", "profiles", "visual-assets-profile.md"))
	if err != nil {
		t.Fatalf("read wizard profile: %v", err)
	}
	profile := string(profileRaw)
	if !strings.Contains(profile, integrations.ComfyUIWorkflowsID) || !strings.Contains(profile, integrations.BlenderReferenceModelingID) || !strings.Contains(profile, "/art-bible") || !strings.Contains(profile, "no art generated") {
		t.Fatalf("wizard profile missing selected asset adapter preferences: %s", profile)
	}
	for _, forbidden := range []string{"git clone", "go build", "npm install", "systemctl", "service start", "downloaded model", "called API", "generated asset"} {
		if strings.Contains(profile, forbidden) {
			t.Fatalf("wizard profile must remain metadata-only, found %q: %s", forbidden, profile)
		}
	}
}

func TestRunWizard_NonInteractiveDefaultsNoOptionalIntegrations(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")

	if err := RunWizard(WizardInput{Args: []string{"--metadata-only", "--non-interactive", "--final-artifact", finalPath}, Registry: templates.NewRegistry(), Persistence: persistence.NewPlaceholder()}); err != nil {
		t.Fatalf("RunWizard returned error: %v", err)
	}
	artifact := readWizardArtifactFixture(t, finalPath)
	if len(artifact.OptionalIntegrations.Selected) != 0 {
		t.Fatalf("non-interactive optional integrations=%#v, want empty", artifact.OptionalIntegrations.Selected)
	}
	if artifact.UseMode != "create_from_scratch" || artifact.SetupDepth != "recommended" || artifact.EnginePack != "godot-core" {
		t.Fatalf("expected staged defaults in final artifact, got use_mode=%q setup_depth=%q engine_pack=%q", artifact.UseMode, artifact.SetupDepth, artifact.EnginePack)
	}
	if len(artifact.NextSteps) == 0 {
		t.Fatalf("expected next steps in final artifact")
	}
}

func TestRunWizard_EnginePackMetadataClarifiesDeferredPlaceholders(t *testing.T) {
	tests := []struct {
		name                            string
		enginePack                      string
		wantEnginePack                  string
		wantLabel                       string
		wantStatus                      string
		wantGenerationDeferred          bool
		wantActiveGenerationSupported   bool
		wantGeneratedPack               string
		forbiddenSelectedCoreLabelToken string
	}{
		{
			name:                          "godot core remains active",
			enginePack:                    "godot-core",
			wantEnginePack:                "godot-core",
			wantLabel:                     "Godot core engine pack",
			wantStatus:                    "active",
			wantGenerationDeferred:        false,
			wantActiveGenerationSupported: true,
			wantGeneratedPack:             "godot-core",
		},
		{
			name:                            "unity is deferred placeholder",
			enginePack:                      "unity",
			wantEnginePack:                  "unity",
			wantLabel:                       "Unity engine pack placeholder",
			wantStatus:                      "deferred",
			wantGenerationDeferred:          true,
			wantActiveGenerationSupported:   false,
			wantGeneratedPack:               "godot-core",
			forbiddenSelectedCoreLabelToken: "Godot core engine pack",
		},
		{
			name:                            "ue5 is deferred placeholder",
			enginePack:                      "ue5",
			wantEnginePack:                  "ue5",
			wantLabel:                       "Unreal Engine 5 engine pack placeholder",
			wantStatus:                      "deferred",
			wantGenerationDeferred:          true,
			wantActiveGenerationSupported:   false,
			wantGeneratedPack:               "godot-core",
			forbiddenSelectedCoreLabelToken: "Godot core engine pack",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workspace := wizardWorkspaceFixture(t)
			setWorkingDirectory(t, workspace)
			t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))
			finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")

			stdout := captureStdout(t, func() {
				if err := RunWizard(WizardInput{Args: []string{"--metadata-only", "--non-interactive", "--engine-pack", tt.enginePack, "--final-artifact", finalPath}, Registry: templates.NewRegistry(), Persistence: persistence.NewPlaceholder()}); err != nil {
					t.Fatalf("RunWizard returned error: %v", err)
				}
			})

			if !strings.Contains(stdout, "Unity/UE5 are future placeholders. Godot is the only active engine pack for generated profiles today.") {
				t.Fatalf("expected engine boundary in summary, got: %s", stdout)
			}

			artifact := readWizardArtifactFixture(t, finalPath)
			if artifact.EnginePack != tt.wantEnginePack || artifact.EnginePackStatus != tt.wantStatus {
				t.Fatalf("unexpected engine metadata: engine_pack=%q status=%q", artifact.EnginePack, artifact.EnginePackStatus)
			}
			if artifact.GenerationDeferred != tt.wantGenerationDeferred || artifact.ActiveGenerationSupported != tt.wantActiveGenerationSupported {
				t.Fatalf("unexpected generation flags: deferred=%t active_supported=%t", artifact.GenerationDeferred, artifact.ActiveGenerationSupported)
			}
			if !containsString(artifact.Packs, tt.wantGeneratedPack) {
				t.Fatalf("expected generated pack %q, got %#v", tt.wantGeneratedPack, artifact.Packs)
			}

			engineCore := selectedCoreByID(artifact.SelectedCore, tt.wantEnginePack)
			if engineCore == nil {
				t.Fatalf("expected selected_core engine %q in %#v", tt.wantEnginePack, artifact.SelectedCore)
			}
			if engineCore.Label != tt.wantLabel || engineCore.Status != tt.wantStatus {
				t.Fatalf("unexpected selected_core engine item: %#v", *engineCore)
			}
			if tt.forbiddenSelectedCoreLabelToken != "" && selectedCoreHasLabel(artifact.SelectedCore, tt.forbiddenSelectedCoreLabelToken) {
				t.Fatalf("selected_core must not label deferred engine as Godot: %#v", artifact.SelectedCore)
			}
		})
	}
}

func TestRunWizard_DeprecatedFlagsMapOnlyWhenReplacementNotExplicit(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		wantUseMode    string
		wantSetupDepth string
		wantErr        string
	}{
		{
			name:        "start existing maps to use mode",
			args:        []string{"--start", "existing"},
			wantUseMode: "existing_game",
		},
		{
			name:        "start import maps to SDD handoff use mode",
			args:        []string{"--start", "import"},
			wantUseMode: "godot_sdd_handoff",
		},
		{
			name:           "complexity expert maps to full setup depth",
			args:           []string{"--complexity", "expert"},
			wantSetupDepth: "full",
		},
		{
			name:        "explicit use mode wins over deprecated start",
			args:        []string{"--use-mode", "create_from_scratch", "--start", "existing"},
			wantUseMode: "create_from_scratch",
		},
		{
			name:    "invalid explicit use mode does not fallback to deprecated start",
			args:    []string{"--use-mode", "invalid", "--start", "existing"},
			wantErr: `invalid use mode: "invalid"`,
		},
		{
			name:           "explicit setup depth wins over deprecated complexity",
			args:           []string{"--setup-depth", "minimal", "--complexity", "expert"},
			wantSetupDepth: "minimal",
		},
		{
			name:    "invalid explicit setup depth does not fallback to deprecated complexity",
			args:    []string{"--setup-depth", "invalid", "--complexity", "expert"},
			wantErr: `invalid setup depth: "invalid"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workspace := wizardWorkspaceFixture(t)
			setWorkingDirectory(t, workspace)
			t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))
			finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
			args := append([]string{"--non-interactive", "--final-artifact", finalPath}, tt.args...)

			err := RunWizard(WizardInput{Args: append([]string{"--metadata-only"}, args...), Registry: templates.NewRegistry(), Persistence: persistence.NewPlaceholder()})
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("RunWizard error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("RunWizard returned error: %v", err)
			}

			artifact := readWizardArtifactFixture(t, finalPath)
			if tt.wantUseMode != "" && artifact.UseMode != tt.wantUseMode {
				t.Fatalf("use_mode=%q want %q", artifact.UseMode, tt.wantUseMode)
			}
			if tt.wantSetupDepth != "" && artifact.SetupDepth != tt.wantSetupDepth {
				t.Fatalf("setup_depth=%q want %q", artifact.SetupDepth, tt.wantSetupDepth)
			}
		})
	}
}

func TestRunWizard_MinimalSetupSelectsOnlyEssentials(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")

	if err := RunWizard(WizardInput{Args: []string{"--metadata-only", "--non-interactive", "--setup-depth", "minimal", "--final-artifact", finalPath}, Registry: templates.NewRegistry(), Persistence: persistence.NewPlaceholder()}); err != nil {
		t.Fatalf("RunWizard returned error: %v", err)
	}

	artifact := readWizardArtifactFixture(t, finalPath)
	if artifact.SetupDepth != "minimal" {
		t.Fatalf("setup_depth=%q want minimal", artifact.SetupDepth)
	}
	if !sameStrings(artifact.Connectors, []string{"godot-docs"}) || !sameStrings(artifact.Packs, []string{"godot-core"}) || !sameStrings(artifact.Tools, []string{"formatter"}) {
		t.Fatalf("minimal setup selected unexpected resources: connectors=%#v packs=%#v tools=%#v", artifact.Connectors, artifact.Packs, artifact.Tools)
	}
	if len(artifact.OptionalIntegrations.Selected) != 0 || len(artifact.MCP["optional"]) != 0 {
		t.Fatalf("minimal setup must not select optionals: optionals=%#v mcp=%#v", artifact.OptionalIntegrations.Selected, artifact.MCP)
	}
	if len(artifact.WorkflowAdapters) != 0 || len(artifact.AssetPipeline.SelectedAdapters) != 0 {
		t.Fatalf("minimal setup must not activate visual workflow adapters: workflow=%#v asset=%#v", artifact.WorkflowAdapters, artifact.AssetPipeline.SelectedAdapters)
	}
	if artifact.ModelRoutingVersion != modelrouting.Version || artifact.ModelRouting.RoutingPreset != "minimal" {
		t.Fatalf("minimal model routing mismatch: %#v", artifact.ModelRouting)
	}
	if artifact.PresetSource != presetSourcePersistedWorkspace || artifact.RoutingPresetSource != routingPresetSourceDerived || artifact.ModelRouting.PresetSource != presetSourcePersistedWorkspace || artifact.ModelRouting.RoutingPresetSource != routingPresetSourceDerived {
		t.Fatalf("minimal preset provenance mismatch: artifact=%#v model_routing=%#v", artifact.PresetSource, artifact.ModelRouting.PresetSource)
	}
	assertOptionalNonFutureCapabilities(t, artifact.ModelRouting)
	assertFutureCapabilitiesOnlyFuture(t, artifact.ModelRouting)
	assertFutureModelCapabilitiesNotReady(t, artifact.ModelRouting)
	assertFutureModelCapabilitiesNotBlocking(t, artifact.ModelRouting)
	for _, lane := range artifact.VisualWorkflowLanes {
		if lane.Status != "deferred" {
			t.Fatalf("minimal visual lanes must be deferred, got %#v", artifact.VisualWorkflowLanes)
		}
	}
}

func TestRunWizard_RecommendedPresetIsMetadataOnlyWithoutAutoConsent(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")

	if err := RunWizard(WizardInput{Args: []string{"--metadata-only", "--non-interactive", "--setup-depth", "recommended", "--final-artifact", finalPath}, Registry: templates.NewRegistry(), Persistence: persistence.NewPlaceholder()}); err != nil {
		t.Fatalf("RunWizard returned error: %v", err)
	}

	artifact := readWizardArtifactFixture(t, finalPath)
	if artifact.SetupPreset != "recommended" || artifact.TaxonomyVersion != wizardTaxonomyVersion {
		t.Fatalf("recommended artifact taxonomy mismatch: setup_preset=%q taxonomy=%q", artifact.SetupPreset, artifact.TaxonomyVersion)
	}
	if !containsCoreID(artifact.SelectedCore, "core-game-workflow/v1") || !containsCoreID(artifact.SelectedCore, "smoke-verify") {
		t.Fatalf("recommended must include core workflow and smoke/verify: %#v", artifact.SelectedCore)
	}
	if len(artifact.AcceptedConsents) != 0 || len(artifact.OptionalIntegrations.Selected) != 0 || len(artifact.WorkflowAdapters) != 0 {
		t.Fatalf("recommended must stay metadata-only without auto-consent/adapters: accepted=%#v optional=%#v adapters=%#v", artifact.AcceptedConsents, artifact.OptionalIntegrations.Selected, artifact.WorkflowAdapters)
	}
	if len(artifact.VisualWorkflowLanes) == 0 || artifact.VisualWorkflowLanes[0].Status != "metadata_selected" {
		t.Fatalf("recommended must include stable visual workflow metadata lanes: %#v", artifact.VisualWorkflowLanes)
	}
	for claim, value := range artifact.AssetPipeline.NoExecutionClaims {
		if value {
			t.Fatalf("asset pipeline no-execution claim %q must remain false", claim)
		}
	}
	if artifact.ModelRoutingVersion != modelrouting.Version || artifact.RoutingPreset != "recommended" {
		t.Fatalf("recommended model routing mismatch: version=%q preset=%q", artifact.ModelRoutingVersion, artifact.RoutingPreset)
	}
	assertDeferredLanesDoNotContainModelRouting(t, artifact.DeferredLanes)
	if !artifact.LegacyRouting || artifact.LegacyRoutingBoundary != legacyRoutingBoundary || !artifact.NoModelExecution {
		t.Fatalf("legacy routing boundary metadata missing: legacy=%t boundary=%q no_model_execution=%t", artifact.LegacyRouting, artifact.LegacyRoutingBoundary, artifact.NoModelExecution)
	}
	for tier, selection := range artifact.TierDefaults {
		if selection == legacyAutoFallback {
			t.Fatalf("final artifact tier %s exposes legacy auto fallback", tier)
		}
	}
	assertModelRoutingNoExecutionClaims(t, artifact.ModelRouting.NoExecutionClaims)
	assertModelRoutingUsesNotConfigured(t, artifact.ModelRouting)
	assertOptionalNonFutureCapabilities(t, artifact.ModelRouting)
	assertFutureCapabilitiesOnlyFuture(t, artifact.ModelRouting)
	assertFutureModelCapabilitiesNotReady(t, artifact.ModelRouting)
	assertWizardPhaseRoute(t, artifact.ModelRouting, "game-concept", modelrouting.CapabilityReasoningHeavy)
	assertWizardPhaseRoute(t, artifact.ModelRouting, "narrative-brief", modelrouting.CapabilityReasoningHeavy)
	assertWizardPhaseRoute(t, artifact.ModelRouting, "gdd-slice", modelrouting.CapabilityReasoningHeavy)
	assertWizardPhaseRoute(t, artifact.ModelRouting, "sdd-apply", modelrouting.CapabilityCodeApply)
	assertWizardPhaseRoute(t, artifact.ModelRouting, "sdd-archive", modelrouting.CapabilityFastText)
	assertWizardPhaseRoute(t, artifact.ModelRouting, "archive-summary", modelrouting.CapabilityFastText)
	assertWizardPhaseRoute(t, artifact.ModelRouting, "visual-review", modelrouting.CapabilityVisualReview)
	assertWizardPhaseRoute(t, artifact.ModelRouting, "qa-review", modelrouting.CapabilityQAReview)
}

func TestRunWizard_FullPresetDoesNotAcceptConsentSensitiveWithoutExplicitConsent(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")

	if err := RunWizard(WizardInput{Args: []string{"--metadata-only", "--non-interactive", "--setup-depth", "full", "--final-artifact", finalPath}, Registry: templates.NewRegistry(), Persistence: persistence.NewPlaceholder()}); err != nil {
		t.Fatalf("RunWizard returned error: %v", err)
	}

	artifact := readWizardArtifactFixture(t, finalPath)
	rawArtifact, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatalf("read final artifact: %v", err)
	}
	legacyMixedField := "optional_" + "future_" + "capabilities"
	if strings.Contains(string(rawArtifact), legacyMixedField) {
		t.Fatalf("final artifact must not contain deprecated mixed capability field %q: %s", legacyMixedField, string(rawArtifact))
	}
	if artifact.SetupPreset != "full" {
		t.Fatalf("setup_preset=%q want full", artifact.SetupPreset)
	}
	if len(artifact.AcceptedConsents) != 0 || len(artifact.RejectedConsents) != 0 || !containsString(artifact.DeferredConsents, integrations.MetronousID+":"+integrations.TelemetryPrivacyConsentID) {
		t.Fatalf("full must not auto-accept consent-sensitive integrations: accepted=%#v rejected=%#v deferred=%#v", artifact.AcceptedConsents, artifact.RejectedConsents, artifact.DeferredConsents)
	}
	if len(artifact.WorkflowAdapters) == 0 {
		t.Fatalf("full should preselect safe metadata workflow adapters without executing them")
	}
	if len(artifact.AssetPipeline.SelectedAdapters) == 0 {
		t.Fatalf("full should record adapter metadata selections")
	}
	for _, adapter := range artifact.AssetPipeline.SelectedAdapters {
		if adapter.ExecutionOwnership != assets.ExecutionMetadataOnly {
			t.Fatalf("full adapter must be metadata-only: %#v", adapter)
		}
	}
	assertFutureModelCapabilitiesNotReady(t, artifact.ModelRouting)
	assertFutureCapabilitiesOnlyFuture(t, artifact.ModelRouting)
	assertOptionalNonFutureCapabilities(t, artifact.ModelRouting)
	assertRoutingValidationDeferredOnlyFuture(t, artifact.ModelRouting)
}

func TestRunWizard_CustomSelectionsRemainCategorized(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")

	input := strings.Join([]string{
		"create_from_scratch",
		"godot-core",
		"custom",
		"Custom Taxonomy Profile",
		"godot-docs",
		"godot-core",
		"formatter",
		integrations.EngramMonitorID + "," + integrations.ComfyUIWorkflowsID,
		"yes",
	}, "\n") + "\n"

	stdout := captureStdout(t, func() {
		err := withStdinText(t, input, func() error {
			return RunWizard(WizardInput{Args: []string{"--metadata-only", "--final-artifact", finalPath}, Registry: templates.NewRegistry(), Persistence: persistence.NewPlaceholder()})
		})
		if err != nil {
			t.Fatalf("RunWizard returned error: %v", err)
		}
	})
	if !strings.Contains(stdout, "Categories are separated") || !strings.Contains(stdout, "workflow_adapter") || !strings.Contains(stdout, "consent_sensitive") {
		t.Fatalf("custom review must expose separated categories, got: %s", stdout)
	}
	artifact := readWizardArtifactFixture(t, finalPath)
	if len(artifact.OptionalIntegrations.Selected) != 1 || artifact.OptionalIntegrations.Selected[0].ID != integrations.EngramMonitorID {
		t.Fatalf("custom optional category mismatch: %#v", artifact.OptionalIntegrations.Selected)
	}
	if len(artifact.WorkflowAdapters) != 1 || artifact.WorkflowAdapters[0].ID != integrations.ComfyUIWorkflowsID {
		t.Fatalf("custom workflow adapter category mismatch: %#v", artifact.WorkflowAdapters)
	}
	if artifact.ModelRouting.OverrideStatus != modelrouting.StatusNotConfigured || artifact.ModelRouting.ValidationResult == "" || len(artifact.ModelRouting.CapabilityOverrides) != 0 || len(artifact.ModelRouting.PhaseOverrides) != 0 {
		t.Fatalf("custom model routing overrides must remain metadata-only: %#v", artifact.ModelRouting)
	}
}

func TestRunWizard_DeferredLanesDoNotBlockGeneration(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")

	stdout := captureStdout(t, func() {
		if err := RunWizard(WizardInput{Args: []string{"--metadata-only", "--non-interactive", "--use-mode", "visual_artifacts_only", "--final-artifact", finalPath}, Registry: templates.NewRegistry(), Persistence: persistence.NewPlaceholder()}); err != nil {
			t.Fatalf("RunWizard returned error: %v", err)
		}
	})

	if !strings.Contains(stdout, "deferred_lanes") || !strings.Contains(stdout, "provider-native-image-generation") || !strings.Contains(stdout, "smoke validation passed") {
		t.Fatalf("expected deferred lane summary and successful smoke, got: %s", stdout)
	}
	artifact := readWizardArtifactFixture(t, finalPath)
	if len(artifact.DeferredLanes) == 0 {
		t.Fatalf("expected deferred lanes in artifact")
	}
	assertDeferredLanesDoNotContainModelRouting(t, artifact.DeferredLanes)
}

func TestStepSummaryScreen_IncludesEssentialStagedReview(t *testing.T) {
	state := wizardState{
		UseMode:       "repair_change_workflow",
		EnginePack:    "godot-core",
		SetupDepth:    "recommended",
		Complexity:    "advanced",
		StartingPoint: "existing",
		ProfileName:   "Review Profile",
		Connectors:    []string{"godot-docs"},
		Packs:         []string{"godot-core"},
		Tools:         []string{"formatter"},
		MCPRequired:   []string{},
		MCPOptional:   []string{},
		Policy: routing.Policy{Preset: routing.PresetBalanced, TierDefaults: map[routing.Tier]routing.Selection{
			routing.TierFast:     {Provider: "unconfigured", Model: "auto"},
			routing.TierBalanced: {Provider: "unconfigured", Model: "auto"},
			routing.TierDeep:     {Provider: "unconfigured", Model: "auto"},
		}, PhaseOverrides: map[string]routing.Selection{}, RoleOverrides: map[string]routing.Selection{}},
		DeferredLanes: defaultDeferredLanes(),
	}

	stdout := captureStdout(t, func() { stepSummaryScreen(state) })
	for _, token := range []string{"use_mode: repair_change_workflow", "engine_pack: godot-core", "setup_depth: recommended", "core_game_workflow", "human approval required", "deferred_lanes"} {
		if !strings.Contains(stdout, token) {
			t.Fatalf("expected review summary token %q, got: %s", token, stdout)
		}
	}
}

func TestRunWizard_ConfirmationBoundaryGeneration(t *testing.T) {
	tests := []struct {
		name          string
		confirmAnswer string
		wantGenerated bool
	}{
		{name: "confirm-yes-generates", confirmAnswer: "yes", wantGenerated: true},
		{name: "confirm-no-cancels", confirmAnswer: "no", wantGenerated: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workspace := wizardWorkspaceFixture(t)
			setWorkingDirectory(t, workspace)
			t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))

			finalPath := filepath.Join(workspace, "out", "wizard", tt.name+".final.artifact.json")

			input := strings.Join([]string{
				"create_from_scratch",
				"godot-core",
				"minimal",
				"Boundary Profile",
				"godot-docs",
				"godot-core",
				"formatter",
				"",
				tt.confirmAnswer,
			}, "\n") + "\n"

			stdout := captureStdout(t, func() {
				err := withStdinText(t, input, func() error {
					return RunWizard(WizardInput{
						Args: []string{
							"--metadata-only",
							"--profile", "Game-Studio",
							"--complexity", "simple",
							"--start", "scratch",
							"--out-dir", filepath.Join(workspace, "out", "generated"),
							"--final-artifact", finalPath,
						},
						Registry:    templates.NewRegistry(),
						Persistence: persistence.NewPlaceholder(),
					})
				})
				if err != nil {
					t.Fatalf("RunWizard returned error: %v", err)
				}
			})

			_, err := os.Stat(finalPath)
			gotGenerated := err == nil
			if gotGenerated != tt.wantGenerated {
				t.Fatalf("generated=%v want=%v (stat err=%v)", gotGenerated, tt.wantGenerated, err)
			}

			configRaw, readErr := os.ReadFile(filepath.Join(workspace, "openspec", "config.yaml"))
			if readErr != nil {
				t.Fatalf("read openspec config: %v", readErr)
			}
			containsManaged := strings.Contains(string(configRaw), "# BEGIN GAME-STUDIO WIZARD (managed)")
			if containsManaged != tt.wantGenerated {
				t.Fatalf("managed block present=%v want=%v", containsManaged, tt.wantGenerated)
			}

			workspaceConfigPath := filepath.Join(workspace, filepath.FromSlash(workspaceConfigRelativePath))
			_, workspaceConfigErr := os.Stat(workspaceConfigPath)
			workspaceConfigExists := workspaceConfigErr == nil
			if workspaceConfigExists != tt.wantGenerated {
				t.Fatalf("workspace config exists=%v want=%v (stat err=%v)", workspaceConfigExists, tt.wantGenerated, workspaceConfigErr)
			}
			if tt.wantGenerated {
				workspaceConfig, exists, readConfigErr := readWorkspaceConfig(workspace)
				if readConfigErr != nil || !exists {
					t.Fatalf("read confirmed workspace config: exists=%v err=%v", exists, readConfigErr)
				}
				if workspaceConfig.SetupPreset != "minimal" {
					t.Fatalf("confirmed workspace setup_preset=%q want minimal", workspaceConfig.SetupPreset)
				}
			}

			if !tt.wantGenerated && !strings.Contains(stdout, "[wizard] cancelled before generation") {
				t.Fatalf("expected cancellation message for no-confirmation path")
			}
		})
	}
}

func TestRunWizardConfirmedSelectionUpdatesWorkspaceConfig(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))
	initial, err := newWorkspaceConfig("minimal", time.Date(2026, time.August, 29, 8, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("newWorkspaceConfig returned error: %v", err)
	}
	if err := writeWorkspaceConfigAtomic(workspace, initial); err != nil {
		t.Fatalf("write initial workspace config: %v", err)
	}

	err = RunWizard(WizardInput{
		Args: []string{
			"--metadata-only",
			"--non-interactive",
			"--setup-depth", "full",
			"--out-dir", "out/generated",
			"--final-artifact", "out/wizard/final.artifact.json",
		},
		Registry:    templates.NewRegistry(),
		Persistence: persistence.NewPlaceholder(),
	})
	if err != nil {
		t.Fatalf("RunWizard returned error: %v", err)
	}

	updated, exists, err := readWorkspaceConfig(workspace)
	if err != nil || !exists {
		t.Fatalf("read updated workspace config: exists=%v err=%v", exists, err)
	}
	if updated.SetupPreset != "full" {
		t.Fatalf("updated setup_preset=%q want full", updated.SetupPreset)
	}
	if updated.UpdatedAt == initial.UpdatedAt {
		t.Fatalf("updated_at was not refreshed: %q", updated.UpdatedAt)
	}
}

func TestRunWizardCancellationPreservesExistingWorkspaceConfig(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))
	initial, err := newWorkspaceConfig("minimal", time.Date(2026, time.August, 29, 8, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("newWorkspaceConfig returned error: %v", err)
	}
	if err := writeWorkspaceConfigAtomic(workspace, initial); err != nil {
		t.Fatalf("write initial workspace config: %v", err)
	}
	configPath := filepath.Join(workspace, filepath.FromSlash(workspaceConfigRelativePath))
	before, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read initial workspace config: %v", err)
	}

	input := strings.Join([]string{
		"create_from_scratch",
		"godot-core",
		"full",
		"Cancelled Profile",
		"godot-docs",
		"godot-core",
		"formatter",
		"",
		"no",
	}, "\n") + "\n"
	err = withStdinText(t, input, func() error {
		return RunWizard(WizardInput{
			Args:        []string{"--metadata-only", "--final-artifact", "out/wizard/final.artifact.json"},
			Registry:    templates.NewRegistry(),
			Persistence: persistence.NewPlaceholder(),
		})
	})
	if err != nil {
		t.Fatalf("RunWizard returned error: %v", err)
	}
	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read workspace config after cancellation: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("cancelled wizard persisted unconfirmed selection\nbefore=%s\nafter=%s", before, after)
	}
}

func wizardWorkspaceFixture(t *testing.T) string {
	t.Helper()
	workspace := t.TempDir()
	openspecDir := filepath.Join(workspace, "openspec")
	if err := os.MkdirAll(openspecDir, 0o755); err != nil {
		t.Fatalf("create openspec directory: %v", err)
	}
	config := "schema: spec-driven\nstrict_tdd: false\n"
	if err := os.WriteFile(filepath.Join(openspecDir, "config.yaml"), []byte(config), 0o644); err != nil {
		t.Fatalf("write openspec config: %v", err)
	}
	return workspace
}

func assertPathNotExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		if err == nil {
			t.Fatalf("expected path %q to not exist", path)
		}
		t.Fatalf("expected path %q to be untouched, got stat error %v", path, err)
	}
}

func setWorkingDirectory(t *testing.T, dir string) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		if chdirErr := os.Chdir(previous); chdirErr != nil {
			t.Fatalf("restore working directory: %v", chdirErr)
		}
	})
}

func withStdinText(t *testing.T, input string, run func() error) error {
	t.Helper()
	tmpFile := filepath.Join(t.TempDir(), "stdin.txt")
	if err := os.WriteFile(tmpFile, []byte(input), 0o644); err != nil {
		t.Fatalf("write stdin fixture: %v", err)
	}

	file, err := os.Open(tmpFile)
	if err != nil {
		t.Fatalf("open stdin fixture: %v", err)
	}
	defer file.Close()

	previous := os.Stdin
	os.Stdin = file
	t.Cleanup(func() {
		os.Stdin = previous
	})

	return run()
}

func captureStdout(t *testing.T, run func()) string {
	t.Helper()
	previous := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	os.Stdout = w

	outCh := make(chan string, 1)
	go func() {
		var builder strings.Builder
		_, _ = io.Copy(&builder, r)
		outCh <- builder.String()
	}()

	run()
	_ = w.Close()
	os.Stdout = previous

	return <-outCh
}

func readWizardArtifactFixture(t *testing.T, path string) wizardFinalArtifact {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read final artifact: %v", err)
	}
	var artifact wizardFinalArtifact
	if err := json.Unmarshal(raw, &artifact); err != nil {
		t.Fatalf("parse final artifact: %v", err)
	}
	for _, key := range []string{"fast", "balanced", "deep"} {
		if _, ok := artifact.TierDefaults[key]; !ok {
			t.Fatalf("expected tier default %q in final artifact", key)
		}
	}
	if _, err := time.Parse(time.RFC3339, artifact.GeneratedAt); err != nil {
		t.Fatalf("expected generated_at in RFC3339 format, got %q: %v", artifact.GeneratedAt, err)
	}
	if artifact.CoreGameWorkflow.ApprovalPolicy.AutoApproved || artifact.CoreGameWorkflow.ApprovalPolicy.ApprovalState != "pending_human_approval" {
		t.Fatalf("expected core game workflow final artifact to require human approval: %#v", artifact.CoreGameWorkflow.ApprovalPolicy)
	}
	for _, approval := range artifact.CoreGameWorkflow.ApprovalPolicy.ArtifactDefaults {
		if approval.ArtifactID == coregame.PhaseGDDSlice && (approval.Status != "draft" || approval.ApprovalState != "pending_human_approval" || approval.AutoApproved) {
			t.Fatalf("expected GDD slice to stay draft/pending and not auto-approved: %#v", approval)
		}
	}
	if len(artifact.CoreGameWorkflow.ModeContracts) != 3 {
		t.Fatalf("expected structured core game mode contracts in final artifact: %#v", artifact.CoreGameWorkflow.ModeContracts)
	}
	if len(artifact.CoreGameWorkflow.DownstreamReferences) == 0 || artifact.CoreGameWorkflow.DownstreamReferences[0].Target == "" || len(artifact.CoreGameWorkflow.DownstreamReferences[0].Consumes) == 0 {
		t.Fatalf("expected structured downstream relationships in final artifact: %#v", artifact.CoreGameWorkflow.DownstreamReferences)
	}
	return artifact
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func containsCoreID(items []wizardTaxonomyItem, target string) bool {
	for _, item := range items {
		if item.ID == target {
			return true
		}
	}
	return false
}

func selectedCoreByID(items []wizardTaxonomyItem, target string) *wizardTaxonomyItem {
	for index := range items {
		if items[index].ID == target {
			return &items[index]
		}
	}
	return nil
}

func selectedCoreHasLabel(items []wizardTaxonomyItem, target string) bool {
	for _, item := range items {
		if item.Label == target {
			return true
		}
	}
	return false
}

func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestMetadataPathValue_NormalizesAndValidates(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		root      string
		want      string
		wantErr   bool
		wantError string
	}{
		{name: "relative path preserved", value: "out/wizard/final.artifact.json", root: "/tmp/workspace", want: "out/wizard/final.artifact.json"},
		{name: "absolute path under workspace", value: "/tmp/workspace/out/wizard/final.artifact.json", root: "/tmp/workspace", want: "out/wizard/final.artifact.json"},
		{name: "empty path rejected", value: "", root: "/tmp/workspace", wantErr: true, wantError: "metadata path is empty"},
		{name: "relative parent escape rejected", value: "../sneaky/artifact.json", root: "/tmp/workspace", wantErr: true, wantError: "outside workspace root"},
		{name: "absolute outside workspace rejected", value: "/other/workspace/artifact.json", root: "/tmp/workspace", wantErr: true, wantError: "outside workspace root"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := metadataPathValue(tt.value, tt.root)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for value %q", tt.value)
				}
				if tt.wantError != "" && !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("expected error containing %q, got %v", tt.wantError, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for value %q: %v", tt.value, err)
			}
			if got != tt.want {
				t.Fatalf("metadataPathValue(%q)= %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}
