package modelrouting

import (
	"fmt"
	"strings"

	"open-game-studios/internal/workflows/coregame"
)

const Version = "model-routing/v1"

type CapabilityID string

const (
	CapabilityDefaultText          CapabilityID = "default_text"
	CapabilityReasoningHeavy       CapabilityID = "reasoning_heavy"
	CapabilityFastText             CapabilityID = "fast_text"
	CapabilityCodeApply            CapabilityID = "code_apply"
	CapabilityQAReview             CapabilityID = "qa_review"
	CapabilityImageGeneration      CapabilityID = "image_generation"
	CapabilityImageReview          CapabilityID = "image_review"
	CapabilityAudioGeneration      CapabilityID = "audio_generation"
	CapabilityAudioReview          CapabilityID = "audio_review"
	CapabilityArchiveSummary       CapabilityID = "archive_summary"
	CapabilityVisualReview         CapabilityID = "visual_review"
	CapabilityGodotTechnicalReview CapabilityID = "godot_technical_review"
)

const (
	StatusConfigured    = "configured"
	StatusNotConfigured = "not_configured"
	StatusDeferred      = "deferred"

	ValidationPass        = "pass"
	ValidationWarning     = "warning"
	ValidationBlocker     = "blocker"
	ValidationSkipped     = "skipped"
	ValidationDeferred    = "deferred"
	ValidationNotSelected = "not_selected"

	// BindingSourceManualOrRuntimeOwned is the canonical default binding source
	// for model-routing/v1 metadata. Bindings are manual user preferences; the
	// runtime owns actual model selection and authentication. No provider
	// inventory, provider API call, or model download occurs.
	BindingSourceManualOrRuntimeOwned = "manual_or_runtime_owned"

	// BindingSourceManualOrFutureDiscovery is a deprecated legacy binding source
	// retained only so historical serialized model-routing/v1 inputs remain
	// accepted during validation. New default contracts, templates, and smoke
	// checks emit BindingSourceManualOrRuntimeOwned instead. The symbol is not an
	// alias: it keeps the old string and does not make older serialized clients
	// compatible with the new canonical source.
	BindingSourceManualOrFutureDiscovery = "manual_or_future_opencode_discovery"
)

type Contract struct {
	Version                          string               `json:"version"`
	Boundary                         string               `json:"boundary"`
	CoreWorkflowVersion              string               `json:"core_game_workflow_version"`
	RoutingPreset                    string               `json:"routing_preset"`
	Capabilities                     []Capability         `json:"capabilities"`
	PhaseRoutes                      []PhaseRoute         `json:"phase_routes"`
	ProviderModelBindings            []ProviderBinding    `json:"provider_model_bindings"`
	CapabilityOverrides              []CapabilityOverride `json:"capability_overrides"`
	PhaseOverrides                   []PhaseOverride      `json:"phase_overrides"`
	OverrideStatus                   string               `json:"override_status"`
	ValidationResult                 string               `json:"validation_result"`
	UnconfiguredRequiredCapabilities []CapabilityID       `json:"unconfigured_required_capabilities"`
	UnconfiguredOptionalCapabilities []CapabilityID       `json:"unconfigured_optional_capabilities"`
	OptionalCapabilities             []CapabilityID       `json:"optional_capabilities"`
	FutureCapabilities               []CapabilityID       `json:"future_capabilities"`
	DeferredCapabilities             []CapabilityID       `json:"deferred_capabilities"`
	RoutingValidationResult          ValidationResult     `json:"routing_validation_result"`
	NoExecutionClaims                map[string]bool      `json:"no_execution_claims"`
}

type Capability struct {
	ID               CapabilityID `json:"id"`
	Label            string       `json:"label"`
	Category         string       `json:"category"`
	Requirement      string       `json:"requirement"`
	Selected         bool         `json:"selected"`
	ConfiguredStatus string       `json:"configured_status"`
	Provider         string       `json:"provider"`
	Model            string       `json:"model"`
	Fallback         CapabilityID `json:"fallback,omitempty"`
	ValidationStatus string       `json:"validation_status"`
	Notes            string       `json:"notes"`
	Boundary         string       `json:"boundary"`
}

type PhaseRoute struct {
	PhaseID          string       `json:"phase_id"`
	Capability       CapabilityID `json:"capability"`
	Status           string       `json:"status"`
	ValidationStatus string       `json:"validation_status"`
	Boundary         string       `json:"boundary"`
}

type ProviderBinding struct {
	Capability                CapabilityID `json:"capability"`
	Provider                  string       `json:"provider"`
	Model                     string       `json:"model"`
	ConfiguredStatus          string       `json:"configured_status"`
	BindingSource             string       `json:"binding_source"`
	RequiresUserConfiguration bool         `json:"requires_user_configuration"`
	Boundary                  string       `json:"boundary"`
}

type CapabilityOverride struct {
	Capability       CapabilityID `json:"capability"`
	Provider         string       `json:"provider,omitempty"`
	Model            string       `json:"model,omitempty"`
	OverrideStatus   string       `json:"override_status"`
	ValidationResult string       `json:"validation_result"`
}

type PhaseOverride struct {
	PhaseID          string       `json:"phase_id"`
	Capability       CapabilityID `json:"capability,omitempty"`
	OverrideStatus   string       `json:"override_status"`
	ValidationResult string       `json:"validation_result"`
}

type ValidationResult struct {
	Status      string   `json:"status"`
	CanContinue bool     `json:"can_continue"`
	Warnings    []string `json:"warnings,omitempty"`
	Blockers    []string `json:"blockers,omitempty"`
	Deferred    []string `json:"deferred,omitempty"`
}

func DefaultContract(preset string) Contract {
	normalizedPreset := normalizePreset(preset)
	capabilities := defaultCapabilities(normalizedPreset)
	contract := Contract{
		Version:              Version,
		Boundary:             "metadata-only routing contract for manual user preferences; actual models and authentication are Pi-owned and orchestration is Gentle Shell-owned; no provider API calls, model downloads, image/audio generation, SDD execution, Godot mutation, ComfyUI/Blender execution, or auto-approval",
		CoreWorkflowVersion:  coregame.Version,
		RoutingPreset:        normalizedPreset,
		Capabilities:         capabilities,
		PhaseRoutes:          defaultPhaseRoutes(normalizedPreset),
		CapabilityOverrides:  []CapabilityOverride{},
		PhaseOverrides:       []PhaseOverride{},
		OverrideStatus:       StatusNotConfigured,
		ValidationResult:     ValidationWarning,
		OptionalCapabilities: optionalCapabilityCatalogIDs(),
		FutureCapabilities:   futureCapabilityCatalogIDs(),
		NoExecutionClaims: map[string]bool{
			"provider_api_calls": false,
			"model_downloads":    false,
			"image_generation":   false,
			"audio_generation":   false,
			"sdd_execution":      false,
			"godot_mutation":     false,
			"comfyui_execution":  false,
			"blender_execution":  false,
			"auto_approval":      false,
		},
	}
	contract.ProviderModelBindings = defaultBindings(capabilities)
	contract.RoutingValidationResult = validateContract(contract)
	contract.ValidationResult = contract.RoutingValidationResult.Status
	for _, capability := range contract.Capabilities {
		if capability.Selected && capability.ConfiguredStatus != StatusConfigured {
			switch capability.Requirement {
			case "required":
				contract.UnconfiguredRequiredCapabilities = append(contract.UnconfiguredRequiredCapabilities, capability.ID)
			case "optional":
				contract.UnconfiguredOptionalCapabilities = append(contract.UnconfiguredOptionalCapabilities, capability.ID)
			case "future":
				contract.DeferredCapabilities = append(contract.DeferredCapabilities, capability.ID)
			}
		}
	}
	return contract
}

func Validate(contract Contract) error {
	if contract.Version != Version {
		return fmt.Errorf("invalid model routing version %q", contract.Version)
	}
	if contract.CoreWorkflowVersion != coregame.Version {
		return fmt.Errorf("invalid core workflow version %q", contract.CoreWorkflowVersion)
	}
	if len(contract.Capabilities) == 0 || len(contract.PhaseRoutes) == 0 || len(contract.ProviderModelBindings) == 0 {
		return fmt.Errorf("model routing contract incomplete")
	}
	for _, id := range contractCapabilityCatalogIDs() {
		if _, ok := capabilityByID(contract.Capabilities, id); !ok {
			return fmt.Errorf("capability %q missing", id)
		}
	}
	for _, phase := range coregame.DefaultContract().PhaseIDs {
		if !hasPhaseRoute(contract.PhaseRoutes, string(phase)) {
			return fmt.Errorf("phase route %q missing", phase)
		}
	}
	for claim, value := range contract.NoExecutionClaims {
		if value {
			return fmt.Errorf("no-execution claim %q must be false", claim)
		}
	}
	for _, binding := range contract.ProviderModelBindings {
		if !isSupportedBindingSource(binding.BindingSource) {
			return fmt.Errorf("invalid provider model binding source %q for capability %q", binding.BindingSource, binding.Capability)
		}
	}
	return nil
}

// isSupportedBindingSource accepts the canonical default and the deprecated
// legacy source for historical inputs, and rejects anything else.
func isSupportedBindingSource(source string) bool {
	switch source {
	case BindingSourceManualOrRuntimeOwned, BindingSourceManualOrFutureDiscovery:
		return true
	default:
		return false
	}
}

func optionalCapabilityCatalogIDs() []CapabilityID {
	return []CapabilityID{
		CapabilityArchiveSummary,
		CapabilityVisualReview,
		CapabilityGodotTechnicalReview,
	}
}

func futureCapabilityCatalogIDs() []CapabilityID {
	return []CapabilityID{
		CapabilityImageGeneration,
		CapabilityImageReview,
		CapabilityAudioGeneration,
		CapabilityAudioReview,
	}
}

func contractCapabilityCatalogIDs() []CapabilityID {
	ids := []CapabilityID{
		CapabilityDefaultText,
		CapabilityReasoningHeavy,
		CapabilityFastText,
		CapabilityCodeApply,
		CapabilityQAReview,
	}
	ids = append(ids, optionalCapabilityCatalogIDs()...)
	ids = append(ids, futureCapabilityCatalogIDs()...)
	return ids
}

func defaultCapabilities(preset string) []Capability {
	selected := map[CapabilityID]bool{
		CapabilityDefaultText:    true,
		CapabilityReasoningHeavy: true,
		CapabilityFastText:       true,
		CapabilityCodeApply:      true,
		CapabilityQAReview:       true,
	}
	if preset == "full" {
		selected[CapabilityImageGeneration] = true
		selected[CapabilityImageReview] = true
		selected[CapabilityAudioGeneration] = true
		selected[CapabilityAudioReview] = true
		selected[CapabilityArchiveSummary] = true
		selected[CapabilityVisualReview] = true
		selected[CapabilityGodotTechnicalReview] = true
	}
	items := []Capability{
		capability(CapabilityDefaultText, "Default text", "core", "required", selected[CapabilityDefaultText], "General text work; can cover minimal core routing."),
		capability(CapabilityReasoningHeavy, "Reasoning-heavy design", "core", "required", selected[CapabilityReasoningHeavy], "Game design, narrative, GDD, change, and repair planning."),
		capability(CapabilityFastText, "Fast text", "core", "required", selected[CapabilityFastText], "Summaries, archive metadata, and lightweight profile text."),
		capability(CapabilityCodeApply, "Code apply", "core", "required", selected[CapabilityCodeApply], "Implementation handoff metadata only; does not execute SDD apply."),
		capability(CapabilityQAReview, "QA review", "core", "required", selected[CapabilityQAReview], "Review and release-gate metadata only."),
		capability(CapabilityImageGeneration, "Image generation", "future", "future", selected[CapabilityImageGeneration], "Future lane; not configured and not executed."),
		capability(CapabilityImageReview, "Image review", "future", "future", selected[CapabilityImageReview], "Future visual review lane; not configured and not executed."),
		capability(CapabilityAudioGeneration, "Audio generation", "future", "future", selected[CapabilityAudioGeneration], "Future audio lane; not configured and not executed."),
		capability(CapabilityAudioReview, "Audio review", "future", "future", selected[CapabilityAudioReview], "Future audio review lane; not configured and not executed."),
		capability(CapabilityArchiveSummary, "Archive summary", "optional", "optional", selected[CapabilityArchiveSummary], "Optional summary specialization; may fall back to fast_text."),
		capability(CapabilityVisualReview, "Visual review", "optional", "optional", selected[CapabilityVisualReview], "Optional visual review specialization; metadata only."),
		capability(CapabilityGodotTechnicalReview, "Godot technical review", "optional", "optional", selected[CapabilityGodotTechnicalReview], "Optional Godot review specialization; no Godot mutation."),
	}
	for idx := range items {
		if items[idx].Requirement == "future" && items[idx].Selected {
			items[idx].ValidationStatus = ValidationDeferred
		}
		if !items[idx].Selected {
			items[idx].ValidationStatus = ValidationNotSelected
		}
	}
	return items
}

func capability(id CapabilityID, label, category, requirement string, selected bool, notes string) Capability {
	return Capability{ID: id, Label: label, Category: category, Requirement: requirement, Selected: selected, ConfiguredStatus: StatusNotConfigured, Provider: StatusNotConfigured, Model: StatusNotConfigured, ValidationStatus: ValidationWarning, Notes: notes, Boundary: "metadata only; requires user configuration before any future execution owner can use it"}
}

func defaultPhaseRoutes(preset string) []PhaseRoute {
	capability := CapabilityReasoningHeavy
	if preset == "minimal" {
		capability = CapabilityDefaultText
	}
	routes := make([]PhaseRoute, 0)
	for _, phase := range coregame.DefaultContract().PhaseIDs {
		routes = append(routes, PhaseRoute{PhaseID: string(phase), Capability: capability, Status: StatusNotConfigured, ValidationStatus: ValidationWarning, Boundary: "phase assignment only; no phase execution"})
	}
	routes = append(routes,
		PhaseRoute{PhaseID: "godot-handoff", Capability: CapabilityCodeApply, Status: StatusNotConfigured, ValidationStatus: ValidationWarning, Boundary: "future Godot handoff metadata only; no Godot mutation"},
		PhaseRoute{PhaseID: "sdd-apply", Capability: CapabilityCodeApply, Status: StatusNotConfigured, ValidationStatus: ValidationWarning, Boundary: "metadata only; does not execute SDD apply"},
		PhaseRoute{PhaseID: "sdd-archive", Capability: CapabilityFastText, Status: StatusNotConfigured, ValidationStatus: ValidationWarning, Boundary: "metadata only; no archive execution"},
		PhaseRoute{PhaseID: "archive-summary", Capability: CapabilityFastText, Status: StatusNotConfigured, ValidationStatus: ValidationWarning, Boundary: "metadata only"},
		PhaseRoute{PhaseID: "qa-review", Capability: CapabilityQAReview, Status: StatusNotConfigured, ValidationStatus: ValidationWarning, Boundary: "review gate metadata only"},
		PhaseRoute{PhaseID: "image-generation", Capability: CapabilityImageGeneration, Status: StatusDeferred, ValidationStatus: ValidationDeferred, Boundary: "future lane; no image generation"},
		PhaseRoute{PhaseID: "image-review", Capability: CapabilityImageReview, Status: StatusDeferred, ValidationStatus: ValidationDeferred, Boundary: "future lane; no image review execution"},
		PhaseRoute{PhaseID: "visual-review", Capability: CapabilityVisualReview, Status: StatusNotConfigured, ValidationStatus: ValidationWarning, Boundary: "optional visual review metadata only; no image review execution"},
		PhaseRoute{PhaseID: "audio-generation", Capability: CapabilityAudioGeneration, Status: StatusDeferred, ValidationStatus: ValidationDeferred, Boundary: "future lane; no audio generation"},
		PhaseRoute{PhaseID: "audio-review", Capability: CapabilityAudioReview, Status: StatusDeferred, ValidationStatus: ValidationDeferred, Boundary: "future lane; no audio review execution"},
	)
	return routes
}

func defaultBindings(capabilities []Capability) []ProviderBinding {
	bindings := make([]ProviderBinding, 0, len(capabilities))
	for _, item := range capabilities {
		bindings = append(bindings, ProviderBinding{Capability: item.ID, Provider: StatusNotConfigured, Model: StatusNotConfigured, ConfiguredStatus: StatusNotConfigured, BindingSource: BindingSourceManualOrRuntimeOwned, RequiresUserConfiguration: true, Boundary: "no provider API calls or model downloads; binding is declarative user preference metadata; Pi owns actual model selection and authentication"})
	}
	return bindings
}

func validateContract(contract Contract) ValidationResult {
	result := ValidationResult{Status: ValidationPass, CanContinue: true}
	for _, capability := range contract.Capabilities {
		if !capability.Selected {
			continue
		}
		if capability.ConfiguredStatus == StatusConfigured {
			continue
		}
		switch capability.Requirement {
		case "required":
			result.Status = ValidationWarning
			result.Warnings = append(result.Warnings, fmt.Sprintf("required capability %s is not_configured; user configuration required before execution owner can use it", capability.ID))
		case "future":
			result.Deferred = append(result.Deferred, fmt.Sprintf("%s is %s and not_configured; non-blocking", capability.ID, capability.Requirement))
		}
	}
	return result
}

func normalizePreset(preset string) string {
	switch strings.ToLower(strings.TrimSpace(preset)) {
	case "minimal", "recommended", "full", "custom":
		return strings.ToLower(strings.TrimSpace(preset))
	case "":
		return "recommended"
	default:
		return "custom"
	}
}

func capabilityByID(items []Capability, id CapabilityID) (Capability, bool) {
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
	}
	return Capability{}, false
}

func hasPhaseRoute(items []PhaseRoute, phaseID string) bool {
	for _, item := range items {
		if item.PhaseID == phaseID {
			return true
		}
	}
	return false
}
