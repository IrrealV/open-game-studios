package assets

import "testing"

func TestDefaultManifestCapturesRequiredMetadataBoundaries(t *testing.T) {
	manifest := DefaultManifest(KindSprite)
	manifest.AssetID = "asset-hero-sprite"
	manifest.Provenance = AssetProvenance{Prompt: "hero idle", License: "studio-owned"}
	manifest.RequestedOutputs = []RequestedOutput{{ID: "out-1", Kind: KindSprite}}
	manifest.GeneratedOutputs = []GeneratedOutput{{ID: "gen-1", Ref: "metadata://hero.png", Kind: KindSprite}}
	manifest.AdapterSelection = AdapterSelection{AdapterID: AdapterComfyUIWorkflows, WorkflowFamily: "visual-image-generation", WorkflowName: "hero-sprite"}
	manifest.Validation = ValidationState{Status: ValidationValid}
	manifest.Approval = ApprovalState{State: ApprovalDraft}
	manifest.ImportTargets = []ImportTarget{{Target: ImportTargetGodot, Path: "res://art/hero.png", Boundary: "metadata only"}}

	if manifest.SchemaVersion == "" || manifest.AssetID == "" || manifest.Kind == "" {
		t.Fatalf("manifest identity fields missing: %#v", manifest)
	}
	if manifest.Provenance.Prompt == "" || manifest.Provenance.License == "" || len(manifest.RequestedOutputs) == 0 || len(manifest.GeneratedOutputs) == 0 {
		t.Fatalf("manifest provenance/output fields missing: %#v", manifest)
	}
	if manifest.Validation.Status == manifest.Approval.State {
		t.Fatalf("validation and approval states must remain separate fields: %#v", manifest)
	}
	if len(manifest.ImportTargets) != 1 || manifest.ImportTargets[0].Boundary == "" {
		t.Fatalf("import target boundary metadata missing: %#v", manifest.ImportTargets)
	}
}

func TestVisualKindAcceptance(t *testing.T) {
	tests := []struct {
		kind string
		want bool
	}{
		{KindSprite, true},
		{KindTexture, true},
		{KindConcept, true},
		{KindReferenceImage, true},
		{KindBlockout, true},
		{KindModel, true},
		{KindImportMetadata, true},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			if got := IsVisualKind(tt.kind); got != tt.want {
				t.Fatalf("IsVisualKind(%q)=%v want %v", tt.kind, got, tt.want)
			}
		})
	}
}

func TestValidationAndApprovalSeparation(t *testing.T) {
	manifest := DefaultManifest(KindModel)
	manifest.Validation = ValidationState{Status: ValidationValid}
	manifest.Approval = ApprovalState{State: ApprovalNeedsRevision}

	if manifest.Validation.Status != ValidationValid {
		t.Fatalf("validation changed unexpectedly: %#v", manifest.Validation)
	}
	if manifest.Approval.State != ApprovalNeedsRevision {
		t.Fatalf("approval changed unexpectedly: %#v", manifest.Approval)
	}
}

func TestDefaultVisualWorkflowCapturesMetadataOnlyReadiness(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "version", got: DefaultVisualWorkflow().Version, want: VisualWorkflowVersion},
		{name: "art bible path", got: DefaultVisualWorkflow().Paths.ArtBible, want: VisualArtBiblePath},
		{name: "asset spec path", got: DefaultVisualWorkflow().Paths.AssetSpecs, want: VisualAssetSpecPath},
		{name: "manifest path", got: DefaultVisualWorkflow().Paths.AssetManifest, want: VisualAssetManifestPath},
		{name: "audit path", got: DefaultVisualWorkflow().Paths.AssetAudit, want: VisualAssetAuditPath},
		{name: "art bible status", got: DefaultVisualWorkflow().ArtBible.Status, want: VisualStatusMissing},
		{name: "approval pending", got: DefaultVisualWorkflow().ArtBible.ApprovalState, want: VisualStatusPendingApproval},
		{name: "readiness blocked", got: DefaultVisualWorkflow().Readiness.Status, want: VisualStatusConcerns},
		{name: "audit read-only", got: DefaultVisualWorkflow().Audit.Mode, want: "read-only"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("got %q want %q", tt.got, tt.want)
			}
		})
	}

	workflow := DefaultVisualWorkflow()
	if workflow.Version == SchemaVersion {
		t.Fatalf("visual workflow version must remain separate from asset manifest version %q", SchemaVersion)
	}
	if len(workflow.ArtBible.Pillars) != 0 || len(workflow.ArtBible.Palette) != 0 || len(workflow.ArtBible.References) != 0 {
		t.Fatalf("default art bible must not imply generated or approved creative content: %#v", workflow.ArtBible)
	}
	if len(workflow.Gates) == 0 || len(workflow.Readiness.RequiredGates) == 0 {
		t.Fatalf("workflow readiness gates missing: %#v", workflow)
	}
	for _, forbidden := range []string{"ComfyUI execution", "Blender execution", "Godot/DCC mutation", "audio/music/SFX"} {
		if !containsString(workflow.Audit.Excludes, forbidden) {
			t.Fatalf("audit excludes missing %q: %#v", forbidden, workflow.Audit.Excludes)
		}
	}
}

func TestVisualWorkflowCapturesRequiredArtBibleAndAssetSpecFields(t *testing.T) {
	workflow := DefaultVisualWorkflow()
	workflow.ArtBible.Pillars = []string{"readable silhouettes", "cozy sci-fi"}
	workflow.ArtBible.Style = "low-poly painterly"
	workflow.ArtBible.Palette = []string{"warm amber", "deep teal"}
	workflow.ArtBible.References = []string{"studio://refs/hero-board"}
	workflow.ArtBible.Constraints = []string{"64px tile readability"}
	workflow.ArtBible.NegativeGuidance = []string{"no photoreal gore"}
	workflow.ArtBible.ApprovalState = VisualStatusPendingApproval
	workflow.AssetSpec.AssetIntent = "hero idle sprite"
	workflow.AssetSpec.GameplayUse = "player avatar state readability"
	workflow.AssetSpec.NarrativeUse = "communicates rookie engineer personality"
	workflow.AssetSpec.ArtBibleLink = workflow.ArtBible.Path
	workflow.AssetSpec.OutputTargets = []string{KindSprite, KindImportMetadata}
	workflow.AssetSpec.PromptReadyFields = []string{"intent", "style guidance", "references", "constraints", "negative guidance", "acceptance criteria"}
	workflow.AssetSpec.ImportHints = []string{"res://art/hero/idle.png metadata target only"}
	workflow.AssetSpec.AcceptanceCriteria = []string{"silhouette matches Art Bible", "import metadata has owner path"}

	if len(workflow.ArtBible.Pillars) == 0 || workflow.ArtBible.Style == "" || len(workflow.ArtBible.Palette) == 0 || len(workflow.ArtBible.References) == 0 || len(workflow.ArtBible.Constraints) == 0 || len(workflow.ArtBible.NegativeGuidance) == 0 || workflow.ArtBible.ApprovalState == "" {
		t.Fatalf("Art Bible required fields not captured: %#v", workflow.ArtBible)
	}
	if workflow.AssetSpec.AssetIntent == "" || workflow.AssetSpec.GameplayUse == "" || workflow.AssetSpec.NarrativeUse == "" || workflow.AssetSpec.ArtBibleLink == "" || len(workflow.AssetSpec.OutputTargets) == 0 || len(workflow.AssetSpec.PromptReadyFields) == 0 || len(workflow.AssetSpec.ImportHints) == 0 || len(workflow.AssetSpec.AcceptanceCriteria) == 0 {
		t.Fatalf("Asset Spec required fields not captured: %#v", workflow.AssetSpec)
	}
	for _, field := range []string{"intent", "style guidance", "references", "constraints", "negative guidance", "acceptance criteria"} {
		if !containsString(workflow.AssetSpec.PromptReadyFields, field) {
			t.Fatalf("Asset Spec prompt-ready metadata missing %q: %#v", field, workflow.AssetSpec.PromptReadyFields)
		}
	}
}

func TestEvaluateVisualReadiness(t *testing.T) {
	readyWorkflow := DefaultVisualWorkflow()
	readyWorkflow.ArtBible.Status = VisualStatusApproved
	readyWorkflow.ArtBible.ApprovalState = VisualStatusApproved
	readyWorkflow.AssetSpec.Status = VisualStatusAudited
	readyWorkflow.AssetSpec.ArtBibleLink = readyWorkflow.ArtBible.Path
	readyWorkflow.Audit.Status = VisualStatusAudited

	tests := []struct {
		name         string
		mutate       func(*VisualWorkflowContract)
		wantReady    bool
		wantStatus   string
		wantBlocking string
	}{
		{
			name:       "approved linked and audited workflow is ready for adapter metadata",
			wantReady:  true,
			wantStatus: VisualStatusReadyForAdapter,
		},
		{
			name: "missing art bible link is not ready",
			mutate: func(workflow *VisualWorkflowContract) {
				workflow.AssetSpec.ArtBibleLink = ""
			},
			wantReady:    false,
			wantStatus:   VisualStatusConcerns,
			wantBlocking: "Asset Spec is missing Art Bible linkage",
		},
		{
			name: "pending art bible approval is not ready",
			mutate: func(workflow *VisualWorkflowContract) {
				workflow.ArtBible.ApprovalState = VisualStatusPendingApproval
			},
			wantReady:    false,
			wantStatus:   VisualStatusConcerns,
			wantBlocking: "Art Bible approval is pending",
		},
		{
			name: "non read only audit is not ready",
			mutate: func(workflow *VisualWorkflowContract) {
				workflow.Audit.Mode = "executes-tools"
			},
			wantReady:    false,
			wantStatus:   VisualStatusConcerns,
			wantBlocking: "Asset audit must remain read-only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workflow := readyWorkflow
			if tt.mutate != nil {
				tt.mutate(&workflow)
			}

			got := EvaluateVisualReadiness(workflow)
			if got.Ready != tt.wantReady || got.Status != tt.wantStatus || got.Workflow.Readiness.Status != tt.wantStatus {
				t.Fatalf("readiness=%#v want ready=%v status=%q", got, tt.wantReady, tt.wantStatus)
			}
			if tt.wantBlocking != "" && !containsString(got.BlockingNotes, tt.wantBlocking) {
				t.Fatalf("blocking note %q missing: %#v", tt.wantBlocking, got.BlockingNotes)
			}
		})
	}
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
