package assets

import "strings"

const (
	SchemaVersion         = "asset-manifest/v1"
	VisualWorkflowVersion = "visual-workflow/v1"

	KindSprite         = "sprite"
	KindTexture        = "texture"
	KindConcept        = "concept"
	KindReferenceImage = "reference-image"
	KindBlockout       = "blockout"
	KindModel          = "model"
	KindImportMetadata = "import-metadata"

	ValidationDraft         = "draft"
	ValidationValid         = "valid"
	ValidationInvalid       = "invalid"
	ValidationOutOfScope    = "out_of_scope"
	ValidationNeedsMetadata = "needs_metadata"

	ApprovalDraft         = "draft"
	ApprovalValidated     = "validated"
	ApprovalNeedsRevision = "needs_revision"
	ApprovalApproved      = "approved"
	ApprovalImported      = "imported"
	ApprovalRejected      = "rejected"

	ImportTargetGodot   = "godot"
	ImportTargetBlender = "blender"

	ExecutionMetadataOnly = "metadata-only"

	VisualStatusMissing         = "missing"
	VisualStatusDraft           = "draft"
	VisualStatusPendingApproval = "pending_approval"
	VisualStatusApproved        = "approved"
	VisualStatusConcerns        = "concerns"
	VisualStatusReadyForAdapter = "ready_for_adapter"
	VisualStatusAudited         = "audited"

	VisualArtBiblePath      = "design/art/art-bible.md"
	VisualAssetSpecPath     = "design/assets/specs/"
	VisualAssetManifestPath = "design/assets/asset-manifest.md"
	VisualAssetAuditPath    = "design/assets/audits/latest.md"
)

type VisualWorkflowContract struct {
	Version   string                  `json:"version"`
	Boundary  string                  `json:"boundary"`
	Paths     VisualWorkflowPaths     `json:"paths"`
	ArtBible  VisualArtBibleContract  `json:"art_bible"`
	AssetSpec VisualAssetSpecContract `json:"asset_spec"`
	Readiness VisualReadinessContract `json:"asset_readiness"`
	Audit     VisualAuditContract     `json:"asset_audit"`
	Gates     []WorkflowGate          `json:"workflow_gates"`
}

type VisualWorkflowPaths struct {
	ArtBible      string `json:"art_bible"`
	AssetSpecs    string `json:"asset_specs"`
	AssetManifest string `json:"asset_manifest"`
	AssetAudit    string `json:"asset_audit"`
}

type VisualArtBibleContract struct {
	Path             string   `json:"path"`
	Status           string   `json:"status"`
	ApprovalState    string   `json:"approval_state"`
	Pillars          []string `json:"pillars"`
	Style            string   `json:"style"`
	Palette          []string `json:"palette"`
	References       []string `json:"references"`
	Constraints      []string `json:"constraints"`
	NegativeGuidance []string `json:"negative_guidance"`
}

type VisualAssetSpecContract struct {
	PathPrefix         string   `json:"path_prefix"`
	Status             string   `json:"status"`
	AssetIntent        string   `json:"asset_intent"`
	GameplayUse        string   `json:"gameplay_use"`
	NarrativeUse       string   `json:"narrative_use"`
	ArtBibleLink       string   `json:"art_bible_link"`
	OutputTargets      []string `json:"output_targets"`
	PromptReadyFields  []string `json:"prompt_ready_fields"`
	ImportHints        []string `json:"import_hints"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
}

type VisualReadinessContract struct {
	Status        string   `json:"status"`
	RequiredGates []string `json:"required_gates"`
	BlockingNotes []string `json:"blocking_notes"`
}

type VisualAuditContract struct {
	Path     string   `json:"path"`
	Status   string   `json:"status"`
	Mode     string   `json:"mode"`
	Checks   []string `json:"checks"`
	Excludes []string `json:"excludes"`
}

type WorkflowGate struct {
	ID          string   `json:"id"`
	Status      string   `json:"status"`
	Requirement string   `json:"requirement"`
	Blocks      []string `json:"blocks,omitempty"`
}

type VisualReadinessEvaluation struct {
	Ready         bool                   `json:"ready"`
	Status        string                 `json:"status"`
	BlockingNotes []string               `json:"blocking_notes,omitempty"`
	Workflow      VisualWorkflowContract `json:"workflow"`
}

type VisualWorkflowPrerequisites struct {
	Version         string   `json:"version"`
	RequiredGates   []string `json:"required_gates"`
	ReadinessStatus string   `json:"readiness_status"`
	BlocksReadiness bool     `json:"blocks_readiness"`
	Boundary        string   `json:"boundary"`
}

type AssetManifest struct {
	SchemaVersion    string            `json:"schema_version"`
	AssetID          string            `json:"asset_id"`
	Kind             string            `json:"kind"`
	Provenance       AssetProvenance   `json:"provenance"`
	RequestedOutputs []RequestedOutput `json:"requested_outputs"`
	GeneratedOutputs []GeneratedOutput `json:"generated_outputs,omitempty"`
	AdapterSelection AdapterSelection  `json:"adapter_selection"`
	Validation       ValidationState   `json:"validation"`
	Approval         ApprovalState     `json:"approval"`
	ImportTargets    []ImportTarget    `json:"import_targets"`
}

type AssetProvenance struct {
	Prompt          string   `json:"prompt,omitempty"`
	ReferenceAssets []string `json:"reference_assets,omitempty"`
	SourceNotes     string   `json:"source_notes,omitempty"`
	License         string   `json:"license,omitempty"`
}

type RequestedOutput struct {
	ID          string            `json:"id"`
	Kind        string            `json:"kind"`
	Description string            `json:"description,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type GeneratedOutput struct {
	ID       string            `json:"id"`
	Ref      string            `json:"ref"`
	Kind     string            `json:"kind"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type AdapterSelection struct {
	AdapterID      string            `json:"adapter_id"`
	WorkflowFamily string            `json:"workflow_family,omitempty"`
	WorkflowName   string            `json:"workflow_name,omitempty"`
	ModelHint      string            `json:"model_hint,omitempty"`
	CheckpointHint string            `json:"checkpoint_hint,omitempty"`
	Loras          []string          `json:"loras,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

type ValidationState struct {
	Status string   `json:"status"`
	Notes  []string `json:"notes,omitempty"`
}

type ApprovalState struct {
	State    string   `json:"state"`
	Reviewer string   `json:"reviewer,omitempty"`
	Notes    []string `json:"notes,omitempty"`
}

type ImportTarget struct {
	Target   string            `json:"target"`
	Path     string            `json:"path,omitempty"`
	Settings map[string]string `json:"settings,omitempty"`
	Hints    []string          `json:"hints,omitempty"`
	Boundary string            `json:"boundary"`
}

type AdapterCapability struct {
	ID                    string                       `json:"id"`
	Label                 string                       `json:"label"`
	Category              string                       `json:"category"`
	SupportedKinds        []string                     `json:"supported_kinds"`
	InputTypes            []string                     `json:"input_types"`
	OutputMetadataTypes   []string                     `json:"output_metadata_types"`
	ExecutionOwnership    string                       `json:"execution_ownership"`
	WorkflowHints         []WorkflowHint               `json:"workflow_hints,omitempty"`
	ModelHints            []ModelHint                  `json:"model_hints,omitempty"`
	BlenderReferenceHints *BlenderReference            `json:"blender_reference_hints,omitempty"`
	VisualPrerequisites   *VisualWorkflowPrerequisites `json:"visual_workflow_prerequisites,omitempty"`
	Limitations           []string                     `json:"limitations"`
	FutureScope           bool                         `json:"future_scope,omitempty"`
	Metadata              map[string]string            `json:"metadata,omitempty"`
}

type WorkflowHint struct {
	Family         string   `json:"family"`
	Name           string   `json:"name"`
	RequiredInputs []string `json:"required_inputs"`
}

type ModelHint struct {
	Model      string   `json:"model"`
	Checkpoint string   `json:"checkpoint,omitempty"`
	Loras      []string `json:"loras,omitempty"`
}

type BlenderReference struct {
	ReferenceImage    string   `json:"reference_image"`
	ScaleUnits        string   `json:"scale_units"`
	StyleConstraints  []string `json:"style_constraints"`
	PolyBudgetHint    string   `json:"poly_budget_hint"`
	AllowedOperations []string `json:"allowed_operations"`
	ExportImportHints []string `json:"export_import_hints"`
}

func DefaultManifest(kind string) AssetManifest {
	return AssetManifest{
		SchemaVersion: SchemaVersion,
		Kind:          normalize(kind),
		Validation:    ValidationState{Status: ValidationDraft},
		Approval:      ApprovalState{State: ApprovalDraft},
	}
}

func DefaultVisualWorkflow() VisualWorkflowContract {
	boundary := "metadata-only visual direction and readiness; does not generate, approve, execute ComfyUI/Blender, mutate Godot/DCC files, or cover audio/music/SFX"
	return VisualWorkflowContract{
		Version:  VisualWorkflowVersion,
		Boundary: boundary,
		Paths: VisualWorkflowPaths{
			ArtBible:      VisualArtBiblePath,
			AssetSpecs:    VisualAssetSpecPath,
			AssetManifest: VisualAssetManifestPath,
			AssetAudit:    VisualAssetAuditPath,
		},
		ArtBible: VisualArtBibleContract{
			Path:             VisualArtBiblePath,
			Status:           VisualStatusMissing,
			ApprovalState:    VisualStatusPendingApproval,
			Pillars:          []string{},
			Style:            "draft human-authored direction required",
			Palette:          []string{},
			References:       []string{},
			Constraints:      []string{},
			NegativeGuidance: []string{},
		},
		AssetSpec: VisualAssetSpecContract{
			PathPrefix:         VisualAssetSpecPath,
			Status:             VisualStatusMissing,
			AssetIntent:        "pending human-authored asset intent",
			GameplayUse:        "pending gameplay use",
			NarrativeUse:       "pending narrative use",
			ArtBibleLink:       VisualArtBiblePath,
			OutputTargets:      []string{"sprite", "texture", "concept/reference image", "blockout", "model", "import metadata"},
			PromptReadyFields:  []string{"intent", "style guidance", "references", "constraints", "negative guidance", "acceptance criteria"},
			ImportHints:        []string{"Godot/Blender metadata hints only; adapters own future processing"},
			AcceptanceCriteria: []string{"aligned with approved Art Bible", "passes read-only asset audit"},
		},
		Readiness: VisualReadinessContract{
			Status:        VisualStatusConcerns,
			RequiredGates: []string{"approved_art_bible", "asset_spec_with_art_bible_link", "read_only_asset_audit"},
			BlockingNotes: []string{"Art Bible approval is pending", "Asset Spec audit has not passed"},
		},
		Audit: VisualAuditContract{
			Path:     VisualAssetAuditPath,
			Status:   VisualStatusMissing,
			Mode:     "read-only",
			Checks:   []string{"Art Bible approval", "Asset Spec linkage", "adapter-readiness metadata"},
			Excludes: []string{"ComfyUI execution", "Blender execution", "Godot/DCC mutation", "audio/music/SFX"},
		},
		Gates: []WorkflowGate{
			{ID: "approved_art_bible", Status: VisualStatusPendingApproval, Requirement: "Human approval required before adapter readiness", Blocks: []string{"adapter_ready"}},
			{ID: "asset_spec_audit", Status: VisualStatusMissing, Requirement: "Asset Spec must link intent, use, targets, prompts, import hints, and acceptance criteria", Blocks: []string{"adapter_ready"}},
			{ID: "read_only_asset_audit", Status: VisualStatusMissing, Requirement: "Read-only audit must pass without execution or file mutation", Blocks: []string{"adapter_ready"}},
		},
	}
}

func EvaluateVisualReadiness(workflow VisualWorkflowContract) VisualReadinessEvaluation {
	notes := []string{}

	if strings.TrimSpace(workflow.ArtBible.Path) == "" {
		notes = append(notes, "Art Bible path is missing")
	}
	if workflow.ArtBible.ApprovalState != VisualStatusApproved {
		notes = append(notes, "Art Bible approval is pending")
	}
	if strings.TrimSpace(workflow.AssetSpec.ArtBibleLink) == "" {
		notes = append(notes, "Asset Spec is missing Art Bible linkage")
	} else if strings.TrimSpace(workflow.ArtBible.Path) != "" && workflow.AssetSpec.ArtBibleLink != workflow.ArtBible.Path {
		notes = append(notes, "Asset Spec Art Bible link does not match the workflow Art Bible path")
	}
	if workflow.AssetSpec.Status != VisualStatusAudited && workflow.AssetSpec.Status != VisualStatusReadyForAdapter {
		notes = append(notes, "Asset Spec audit has not passed")
	}
	if workflow.Audit.Mode != "read-only" {
		notes = append(notes, "Asset audit must remain read-only")
	}
	if workflow.Audit.Status != VisualStatusAudited {
		notes = append(notes, "Read-only asset audit has not completed")
	}

	status := VisualStatusReadyForAdapter
	if len(notes) > 0 {
		status = VisualStatusConcerns
	}
	workflow.Readiness.Status = status
	workflow.Readiness.BlockingNotes = append([]string{}, notes...)

	return VisualReadinessEvaluation{
		Ready:         len(notes) == 0,
		Status:        status,
		BlockingNotes: append([]string{}, notes...),
		Workflow:      workflow,
	}
}

func IsVisualKind(kind string) bool {
	switch normalize(kind) {
	case KindSprite, KindTexture, KindConcept, KindReferenceImage, KindBlockout, KindModel, KindImportMetadata:
		return true
	default:
		return false
	}
}

func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
