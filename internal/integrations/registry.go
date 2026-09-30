package integrations

import (
	"sort"
	"strings"

	"open-game-studios/internal/assets"
)

const (
	EngramMonitorID            = "engram-monitor"
	MetronousID                = "metronous"
	ComfyUIWorkflowsID         = assets.AdapterComfyUIWorkflows
	BlenderReferenceModelingID = assets.AdapterBlenderReferenceModels

	TelemetryPrivacyConsentID = "telemetry_privacy"
)

type Registry struct {
	items map[string]Integration
}

func NewRegistry(items ...Integration) *Registry {
	registry := &Registry{items: map[string]Integration{}}
	for _, item := range items {
		if id := normalizeID(item.ID); id != "" {
			item.ID = id
			registry.items[id] = cloneIntegration(item)
		}
	}
	return registry
}

func NewDefaultRegistry() *Registry {
	comfy, _ := assets.GetDefaultAdapter(assets.AdapterComfyUIWorkflows)
	blender, _ := assets.GetDefaultAdapter(assets.AdapterBlenderReferenceModels)
	return NewRegistry(
		Integration{
			ID:              EngramMonitorID,
			Label:           "Engram Monitor",
			Category:        "observability",
			Description:     "Optional local visibility metadata for Engram-backed workflows.",
			DefaultSelected: false,
			DiagnosticToolIDs: []string{
				"engram-monitor",
			},
			Hints: []string{
				"Selection records intent only; install or service startup is not performed.",
				"Use existing Engram tooling/runtime if your studio already manages it.",
			},
		},
		Integration{
			ID:              MetronousID,
			Label:           "Metronous",
			Category:        "telemetry",
			Description:     "Optional telemetry integration metadata for future studio analytics.",
			DefaultSelected: false,
			DiagnosticToolIDs: []string{
				"metronous",
			},
			Hints: []string{
				"Requires explicit telemetry/privacy consent before it can be selected.",
				"Selection is metadata only; no clone, build, install, or service management is performed.",
			},
			Consent: []ConsentRequirement{
				{
					ID:           TelemetryPrivacyConsentID,
					Label:        "Telemetry/privacy consent",
					RequiredText: "I explicitly consent to Metronous telemetry/privacy metadata being recorded for this profile.",
				},
			},
		},
		Integration{
			ID:              ComfyUIWorkflowsID,
			Label:           "ComfyUI Workflows",
			Category:        "asset-adapter",
			Description:     "Selection metadata for visual ComfyUI workflow/model preferences.",
			DefaultSelected: false,
			Hints: []string{
				"Selection records workflow/model preference metadata only.",
				"No install, clone, build, run, API call, download, or service management is performed.",
			},
			AssetAdapter: &comfy,
		},
		Integration{
			ID:              BlenderReferenceModelingID,
			Label:           "Blender Reference Modeling",
			Category:        "asset-adapter",
			Description:     "Selection metadata for Blender reference modeling/import boundaries.",
			DefaultSelected: false,
			Hints: []string{
				"Selection records reference modeling and import metadata only.",
				"No Blender launch, file mutation, conversion, or Godot project write is performed.",
			},
			AssetAdapter: &blender,
		},
	)
}

func (r *Registry) List() []Integration {
	if r == nil {
		r = NewDefaultRegistry()
	}
	out := make([]Integration, 0, len(r.items))
	for _, item := range r.items {
		out = append(out, cloneIntegration(item))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (r *Registry) Get(id string) (Integration, bool) {
	if r == nil {
		r = NewDefaultRegistry()
	}
	item, ok := r.items[normalizeID(id)]
	if !ok {
		return Integration{}, false
	}
	return cloneIntegration(item), true
}

func normalizeID(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}

func cloneIntegration(item Integration) Integration {
	item.DiagnosticToolIDs = append([]string{}, item.DiagnosticToolIDs...)
	item.Hints = append([]string{}, item.Hints...)
	item.Consent = append([]ConsentRequirement{}, item.Consent...)
	if item.AssetAdapter != nil {
		adapter := *item.AssetAdapter
		adapter.SupportedKinds = append([]string{}, adapter.SupportedKinds...)
		adapter.InputTypes = append([]string{}, adapter.InputTypes...)
		adapter.OutputMetadataTypes = append([]string{}, adapter.OutputMetadataTypes...)
		adapter.WorkflowHints = append([]assets.WorkflowHint{}, adapter.WorkflowHints...)
		for i := range adapter.WorkflowHints {
			adapter.WorkflowHints[i].RequiredInputs = append([]string{}, adapter.WorkflowHints[i].RequiredInputs...)
		}
		adapter.ModelHints = append([]assets.ModelHint{}, adapter.ModelHints...)
		for i := range adapter.ModelHints {
			adapter.ModelHints[i].Loras = append([]string{}, adapter.ModelHints[i].Loras...)
		}
		if adapter.BlenderReferenceHints != nil {
			ref := *adapter.BlenderReferenceHints
			ref.StyleConstraints = append([]string{}, ref.StyleConstraints...)
			ref.AllowedOperations = append([]string{}, ref.AllowedOperations...)
			ref.ExportImportHints = append([]string{}, ref.ExportImportHints...)
			adapter.BlenderReferenceHints = &ref
		}
		adapter.Limitations = append([]string{}, adapter.Limitations...)
		if adapter.Metadata != nil {
			adapter.Metadata = map[string]string{}
			for key, value := range item.AssetAdapter.Metadata {
				adapter.Metadata[key] = value
			}
		}
		item.AssetAdapter = &adapter
	}
	return item
}
