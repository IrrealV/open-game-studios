package assets

const (
	AdapterComfyUIWorkflows       = "comfyui-workflows"
	AdapterBlenderReferenceModels = "blender-reference-modeling"
)

var deferredAdapterIDs = map[string]string{
	"hunyuan3d":                  "future standalone 3D generation adapter",
	"triposr":                    "future standalone 3D reconstruction adapter",
	"stable-fast-3d":             "future standalone 3D generation adapter",
	"trellis.2":                  "future standalone 3D generation adapter",
	"comfyui-3d-pack":            "future ComfyUI pack metadata, not an initial standalone adapter",
	"comfyui-3d-pack-standalone": "future ComfyUI pack metadata, not an initial standalone adapter",
}

func DefaultAdapterCatalog() []AdapterCapability {
	return cloneCapabilities([]AdapterCapability{
		{
			ID:                  AdapterComfyUIWorkflows,
			Label:               "ComfyUI Workflows",
			Category:            "asset-adapter",
			SupportedKinds:      []string{KindSprite, KindTexture, KindConcept, KindReferenceImage},
			InputTypes:          []string{"prompt", "reference-image", "workflow-json"},
			OutputMetadataTypes: []string{"image-ref", "workflow-ref", "model-preference", "import-hints"},
			ExecutionOwnership:  ExecutionMetadataOnly,
			WorkflowHints: []WorkflowHint{{
				Family:         "visual-image-generation",
				Name:           "studio-reference-or-texture",
				RequiredInputs: []string{"prompt", "workflow_name", "model_hint"},
			}},
			ModelHints:          []ModelHint{{Model: "sdxl-compatible", Checkpoint: "studio-selected-checkpoint", Loras: []string{"optional-style-lora"}}},
			VisualPrerequisites: defaultVisualAdapterPrerequisites(),
			Limitations: []string{
				"Metadata only: does not install, clone, build, run, call APIs, or manage ComfyUI services.",
				"Adapter readiness is blocked until visual-workflow/v1 has an approved Art Bible, linked Asset Spec, and passing read-only audit.",
				"3D model generation packs are future scope and are not standalone initial adapters.",
			},
		},
		{
			ID:                  AdapterBlenderReferenceModels,
			Label:               "Blender Reference Modeling",
			Category:            "asset-adapter",
			SupportedKinds:      []string{KindReferenceImage, KindBlockout, KindModel, KindImportMetadata},
			InputTypes:          []string{"reference-image", "scale-units", "style-constraints", "poly-budget-hint"},
			OutputMetadataTypes: []string{"blend-ref", "mesh-import-hints", "godot-import-metadata", "blender-import-metadata"},
			ExecutionOwnership:  ExecutionMetadataOnly,
			BlenderReferenceHints: &BlenderReference{
				ReferenceImage:    "required metadata reference; no file mutation performed",
				ScaleUnits:        "meters",
				StyleConstraints:  []string{"blockout-first", "human-reviewed"},
				PolyBudgetHint:    "studio-defined",
				AllowedOperations: []string{"reference modeling", "blockout planning", "import metadata capture"},
				ExportImportHints: []string{"Godot import target metadata only", "Blender scene/file ownership remains external"},
			},
			VisualPrerequisites: defaultVisualAdapterPrerequisites(),
			Limitations: []string{
				"Metadata only: does not launch Blender, mutate .blend files, convert assets, or write Godot project files.",
				"Adapter readiness is blocked until visual-workflow/v1 has an approved Art Bible, linked Asset Spec, and passing read-only audit.",
				"Automated 3D reconstruction/generation adapters are future scope.",
			},
		},
	})
}

func defaultVisualAdapterPrerequisites() *VisualWorkflowPrerequisites {
	workflow := DefaultVisualWorkflow()
	return &VisualWorkflowPrerequisites{
		Version:         workflow.Version,
		RequiredGates:   append([]string{}, workflow.Readiness.RequiredGates...),
		ReadinessStatus: VisualStatusConcerns,
		BlocksReadiness: true,
		Boundary:        workflow.Boundary,
	}
}

func GetDefaultAdapter(id string) (AdapterCapability, bool) {
	for _, capability := range DefaultAdapterCatalog() {
		if capability.ID == normalize(id) {
			return capability, true
		}
	}
	return AdapterCapability{}, false
}

func DeferredAdapterReason(id string) (string, bool) {
	reason, ok := deferredAdapterIDs[normalize(id)]
	return reason, ok
}

func cloneCapabilities(items []AdapterCapability) []AdapterCapability {
	out := make([]AdapterCapability, len(items))
	for i, item := range items {
		out[i] = cloneCapability(item)
	}
	return out
}

func cloneCapability(item AdapterCapability) AdapterCapability {
	item.SupportedKinds = append([]string{}, item.SupportedKinds...)
	item.InputTypes = append([]string{}, item.InputTypes...)
	item.OutputMetadataTypes = append([]string{}, item.OutputMetadataTypes...)
	item.WorkflowHints = append([]WorkflowHint{}, item.WorkflowHints...)
	for i := range item.WorkflowHints {
		item.WorkflowHints[i].RequiredInputs = append([]string{}, item.WorkflowHints[i].RequiredInputs...)
	}
	item.ModelHints = append([]ModelHint{}, item.ModelHints...)
	for i := range item.ModelHints {
		item.ModelHints[i].Loras = append([]string{}, item.ModelHints[i].Loras...)
	}
	if item.BlenderReferenceHints != nil {
		ref := *item.BlenderReferenceHints
		ref.StyleConstraints = append([]string{}, ref.StyleConstraints...)
		ref.AllowedOperations = append([]string{}, ref.AllowedOperations...)
		ref.ExportImportHints = append([]string{}, ref.ExportImportHints...)
		item.BlenderReferenceHints = &ref
	}
	if item.VisualPrerequisites != nil {
		prereq := *item.VisualPrerequisites
		prereq.RequiredGates = append([]string{}, prereq.RequiredGates...)
		item.VisualPrerequisites = &prereq
	}
	item.Limitations = append([]string{}, item.Limitations...)
	if item.Metadata != nil {
		item.Metadata = map[string]string{}
		for key, value := range item.Metadata {
			item.Metadata[key] = value
		}
	}
	return item
}
