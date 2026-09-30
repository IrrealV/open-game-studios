package modelrouting

import (
	"strings"
	"testing"

	"open-game-studios/internal/workflows/coregame"
)

func TestDefaultContractDefinesModelRoutingV1Capabilities(t *testing.T) {
	contract := DefaultContract("recommended")

	if err := Validate(contract); err != nil {
		t.Fatalf("default model routing contract should validate: %v", err)
	}
	if contract.Version != Version {
		t.Fatalf("version=%q want %q", contract.Version, Version)
	}
	for _, id := range contractCapabilityCatalogIDs() {
		capability, ok := capabilityByID(contract.Capabilities, id)
		if !ok {
			t.Fatalf("capability %q missing", id)
		}
		if capability.Provider != StatusNotConfigured || capability.Model != StatusNotConfigured {
			t.Fatalf("capability %q must not hardcode provider/model: %#v", id, capability)
		}
	}
}

func TestValidateRequiresOptionalCapabilitiesDefinedBySpec(t *testing.T) {
	tests := []CapabilityID{
		CapabilityArchiveSummary,
		CapabilityVisualReview,
		CapabilityGodotTechnicalReview,
	}

	for _, id := range tests {
		t.Run(string(id), func(t *testing.T) {
			contract := DefaultContract("recommended")
			contract.Capabilities = removeCapability(contract.Capabilities, id)

			if err := Validate(contract); err == nil {
				t.Fatalf("Validate() must reject missing capability %q", id)
			}
		})
	}
}

func TestValidateDoesNotRequireOptionalOrFutureCapabilitiesConfigured(t *testing.T) {
	contract := DefaultContract("full")

	for _, id := range []CapabilityID{CapabilityArchiveSummary, CapabilityVisualReview, CapabilityGodotTechnicalReview, CapabilityImageGeneration, CapabilityImageReview, CapabilityAudioGeneration, CapabilityAudioReview} {
		capability, ok := capabilityByID(contract.Capabilities, id)
		if !ok {
			t.Fatalf("capability %q missing", id)
		}
		if capability.ConfiguredStatus != StatusNotConfigured {
			t.Fatalf("capability %q configured_status=%q want not_configured", id, capability.ConfiguredStatus)
		}
	}

	if err := Validate(contract); err != nil {
		t.Fatalf("Validate() must permit optional/future capabilities to remain not_configured: %v", err)
	}
}

func TestCapabilityCatalogListsAreIndependentFromSelection(t *testing.T) {
	tests := []string{"minimal", "recommended"}

	for _, preset := range tests {
		t.Run(preset, func(t *testing.T) {
			contract := DefaultContract(preset)

			assertCapabilityIDsEqual(t, contract.OptionalCapabilities, []CapabilityID{CapabilityArchiveSummary, CapabilityVisualReview, CapabilityGodotTechnicalReview})
			assertCapabilityIDsEqual(t, contract.FutureCapabilities, []CapabilityID{CapabilityImageGeneration, CapabilityImageReview, CapabilityAudioGeneration, CapabilityAudioReview})

			for _, id := range append(append([]CapabilityID{}, contract.OptionalCapabilities...), contract.FutureCapabilities...) {
				capability, ok := capabilityByID(contract.Capabilities, id)
				if !ok {
					t.Fatalf("catalog capability %q missing from capabilities", id)
				}
				if capability.Selected {
					t.Fatalf("%s catalog capability %q should not be selected by default: %#v", preset, id, capability)
				}
			}
		})
	}
}

func TestPhaseRoutesIncludeCoreGameWorkflowPhases(t *testing.T) {
	contract := DefaultContract("recommended")

	for _, phase := range coregame.DefaultContract().PhaseIDs {
		route, ok := phaseRouteFor(contract.PhaseRoutes, string(phase))
		if !ok {
			t.Fatalf("missing phase route for %q", phase)
		}
		if route.Capability != CapabilityReasoningHeavy {
			t.Fatalf("recommended route for %q=%q want reasoning_heavy", phase, route.Capability)
		}
	}
}

func TestMinimalDoesNotBlockForImageOrAudioNotConfigured(t *testing.T) {
	contract := DefaultContract("minimal")

	if !contract.RoutingValidationResult.CanContinue {
		t.Fatalf("minimal routing should continue: %#v", contract.RoutingValidationResult)
	}
	if contract.RoutingValidationResult.Status == ValidationBlocker {
		t.Fatalf("minimal routing must not block: %#v", contract.RoutingValidationResult)
	}
	for _, id := range []CapabilityID{CapabilityImageGeneration, CapabilityImageReview, CapabilityAudioGeneration, CapabilityAudioReview} {
		capability, ok := capabilityByID(contract.Capabilities, id)
		if !ok {
			t.Fatalf("capability %q missing", id)
		}
		if capability.Selected || capability.ValidationStatus != ValidationNotSelected {
			t.Fatalf("minimal future capability %q must be not selected/non-blocking: %#v", id, capability)
		}
	}
}

func TestRecommendedRoutesProductionCapabilities(t *testing.T) {
	contract := DefaultContract("recommended")

	for _, phase := range []string{"game-concept", "game-pillars", "core-loop", "player-fantasy", "mechanics-brief", "narrative-brief", "tone-and-mood", "story-constraints", "gdd-slice", "change-brief", "repair-brief"} {
		assertRouteCapability(t, contract, phase, CapabilityReasoningHeavy)
	}
	assertRouteCapability(t, contract, "sdd-apply", CapabilityCodeApply)
	assertRouteCapability(t, contract, "godot-handoff", CapabilityCodeApply)
	assertRouteCapability(t, contract, "sdd-archive", CapabilityFastText)
	assertRouteCapability(t, contract, "archive-summary", CapabilityFastText)
	assertRouteCapability(t, contract, "qa-review", CapabilityQAReview)
}

func TestPhaseRoutesKeepOptionalReviewSeparateFromFutureMedia(t *testing.T) {
	contract := DefaultContract("full")

	assertRoute(t, contract, "archive-summary", CapabilityFastText, StatusNotConfigured, ValidationWarning)
	assertRoute(t, contract, "visual-review", CapabilityVisualReview, StatusNotConfigured, ValidationWarning)
	assertRoute(t, contract, "image-review", CapabilityImageReview, StatusDeferred, ValidationDeferred)
}

func TestFullDoesNotMarkImageOrAudioReady(t *testing.T) {
	contract := DefaultContract("full")

	for _, id := range []CapabilityID{CapabilityImageGeneration, CapabilityImageReview, CapabilityAudioGeneration, CapabilityAudioReview} {
		capability, ok := capabilityByID(contract.Capabilities, id)
		if !ok {
			t.Fatalf("capability %q missing", id)
		}
		if capability.ConfiguredStatus == StatusConfigured || capability.ValidationStatus == ValidationPass {
			t.Fatalf("full future capability %q must not be marked ready: %#v", id, capability)
		}
	}
}

func TestFullKeepsOptionalNonFutureCapabilitiesSeparate(t *testing.T) {
	contract := DefaultContract("full")

	tests := []struct {
		id          CapabilityID
		requirement string
	}{
		{id: CapabilityArchiveSummary, requirement: "optional"},
		{id: CapabilityVisualReview, requirement: "optional"},
		{id: CapabilityGodotTechnicalReview, requirement: "optional"},
		{id: CapabilityImageGeneration, requirement: "future"},
		{id: CapabilityImageReview, requirement: "future"},
		{id: CapabilityAudioGeneration, requirement: "future"},
		{id: CapabilityAudioReview, requirement: "future"},
	}

	for _, tt := range tests {
		t.Run(string(tt.id), func(t *testing.T) {
			capability, ok := capabilityByID(contract.Capabilities, tt.id)
			if !ok {
				t.Fatalf("capability %q missing", tt.id)
			}
			if capability.Requirement != tt.requirement {
				t.Fatalf("capability %q requirement=%q want %q", tt.id, capability.Requirement, tt.requirement)
			}
		})
	}
}

func TestFullRoutingValidationDefersOnlyFutureCapabilities(t *testing.T) {
	contract := DefaultContract("full")

	for _, id := range []CapabilityID{CapabilityArchiveSummary, CapabilityVisualReview, CapabilityGodotTechnicalReview} {
		assertValidationDeferredDoesNotContainCapability(t, contract.RoutingValidationResult, id)
	}
	for _, id := range []CapabilityID{CapabilityImageGeneration, CapabilityImageReview, CapabilityAudioGeneration, CapabilityAudioReview} {
		assertValidationDeferredContainsCapability(t, contract.RoutingValidationResult, id)
	}
}

func TestCustomOverridesAreMetadataOnly(t *testing.T) {
	contract := DefaultContract("custom")

	if contract.OverrideStatus != StatusNotConfigured {
		t.Fatalf("override status=%q want not_configured", contract.OverrideStatus)
	}
	if len(contract.CapabilityOverrides) != 0 || len(contract.PhaseOverrides) != 0 {
		t.Fatalf("custom overrides should be prepared but empty metadata: capability=%#v phase=%#v", contract.CapabilityOverrides, contract.PhaseOverrides)
	}
	for _, binding := range contract.ProviderModelBindings {
		if binding.BindingSource != BindingSourceManualOrRuntimeOwned || !binding.RequiresUserConfiguration {
			t.Fatalf("binding should remain manual/runtime-owned metadata: %#v", binding)
		}
	}
}

func TestNoExecutionClaimsArePresentAndFalse(t *testing.T) {
	contract := DefaultContract("recommended")

	for _, claim := range []string{"provider_api_calls", "model_downloads", "image_generation", "audio_generation", "sdd_execution", "godot_mutation", "comfyui_execution", "blender_execution", "auto_approval"} {
		value, ok := contract.NoExecutionClaims[claim]
		if !ok || value {
			t.Fatalf("no-execution claim %q missing or true: %#v", claim, contract.NoExecutionClaims)
		}
	}
}

func TestDefaultBindingsUseCanonicalManualRuntimeOwnedSource(t *testing.T) {
	contract := DefaultContract("recommended")

	if len(contract.ProviderModelBindings) == 0 {
		t.Fatal("expected default provider model bindings")
	}
	for _, binding := range contract.ProviderModelBindings {
		if binding.BindingSource != BindingSourceManualOrRuntimeOwned {
			t.Fatalf("binding %q source=%q want %q", binding.Capability, binding.BindingSource, BindingSourceManualOrRuntimeOwned)
		}
	}
	if BindingSourceManualOrRuntimeOwned == BindingSourceManualOrFutureDiscovery {
		t.Fatal("canonical and legacy binding sources must remain distinct values")
	}
}

func TestValidateAcceptsDeprecatedLegacyBindingSourceForHistoricalInputs(t *testing.T) {
	contract := DefaultContract("recommended")
	for idx := range contract.ProviderModelBindings {
		contract.ProviderModelBindings[idx].BindingSource = BindingSourceManualOrFutureDiscovery
	}
	if err := Validate(contract); err != nil {
		t.Fatalf("Validate must accept the deprecated legacy binding source for historical inputs: %v", err)
	}
}

func TestValidateRejectsUnknownBindingSource(t *testing.T) {
	contract := DefaultContract("recommended")
	contract.ProviderModelBindings[0].BindingSource = "manual_or_whatever"
	if err := Validate(contract); err == nil {
		t.Fatal("Validate must reject an unknown binding source")
	}
}

func phaseRouteFor(items []PhaseRoute, phaseID string) (PhaseRoute, bool) {
	for _, item := range items {
		if item.PhaseID == phaseID {
			return item, true
		}
	}
	return PhaseRoute{}, false
}

func removeCapability(items []Capability, id CapabilityID) []Capability {
	out := make([]Capability, 0, len(items))
	for _, item := range items {
		if item.ID != id {
			out = append(out, item)
		}
	}
	return out
}

func assertCapabilityIDsEqual(t *testing.T, got []CapabilityID, want []CapabilityID) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("capability IDs=%#v want %#v", got, want)
	}
	for idx := range want {
		if got[idx] != want[idx] {
			t.Fatalf("capability IDs=%#v want %#v", got, want)
		}
	}
}

func assertRouteCapability(t *testing.T, contract Contract, phaseID string, capability CapabilityID) {
	t.Helper()
	route, ok := phaseRouteFor(contract.PhaseRoutes, phaseID)
	if !ok {
		t.Fatalf("route %q missing", phaseID)
	}
	if route.Capability != capability {
		t.Fatalf("route %q capability=%q want %q", phaseID, route.Capability, capability)
	}
}

func assertRoute(t *testing.T, contract Contract, phaseID string, capability CapabilityID, status string, validationStatus string) {
	t.Helper()
	route, ok := phaseRouteFor(contract.PhaseRoutes, phaseID)
	if !ok {
		t.Fatalf("route %q missing", phaseID)
	}
	if route.Capability != capability || route.Status != status || route.ValidationStatus != validationStatus {
		t.Fatalf("route %q = %#v; want capability=%q status=%q validation=%q", phaseID, route, capability, status, validationStatus)
	}
}

func assertValidationDeferredContainsCapability(t *testing.T, result ValidationResult, id CapabilityID) {
	t.Helper()
	for _, item := range result.Deferred {
		if strings.Contains(item, string(id)) {
			return
		}
	}
	t.Fatalf("validation deferred missing %q: %#v", id, result.Deferred)
}

func assertValidationDeferredDoesNotContainCapability(t *testing.T, result ValidationResult, id CapabilityID) {
	t.Helper()
	for _, item := range result.Deferred {
		if strings.Contains(item, string(id)) {
			t.Fatalf("validation deferred contains optional non-future capability %q: %#v", id, result.Deferred)
		}
	}
}
