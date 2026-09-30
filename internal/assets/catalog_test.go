package assets

import "testing"

func TestDefaultAdapterCatalogIncludesOnlyInitialAdapters(t *testing.T) {
	catalog := DefaultAdapterCatalog()
	if len(catalog) != 2 {
		t.Fatalf("default adapter catalog length=%d want 2", len(catalog))
	}
	got := []string{catalog[0].ID, catalog[1].ID}
	want := []string{AdapterComfyUIWorkflows, AdapterBlenderReferenceModels}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("catalog ids=%v want %v", got, want)
		}
	}
}

func TestDefaultAdapterCatalogMetadataOnlyCapabilities(t *testing.T) {
	tests := []struct {
		id              string
		wantWorkflow    bool
		wantBlenderHint bool
	}{
		{id: AdapterComfyUIWorkflows, wantWorkflow: true},
		{id: AdapterBlenderReferenceModels, wantBlenderHint: true},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			adapter, ok := GetDefaultAdapter(tt.id)
			if !ok {
				t.Fatalf("expected adapter %q", tt.id)
			}
			if adapter.ExecutionOwnership != ExecutionMetadataOnly {
				t.Fatalf("ExecutionOwnership=%q want %q", adapter.ExecutionOwnership, ExecutionMetadataOnly)
			}
			if len(adapter.SupportedKinds) == 0 || len(adapter.InputTypes) == 0 || len(adapter.OutputMetadataTypes) == 0 || len(adapter.Limitations) == 0 {
				t.Fatalf("capability metadata incomplete: %#v", adapter)
			}
			if (len(adapter.WorkflowHints) > 0) != tt.wantWorkflow {
				t.Fatalf("workflow hints=%#v", adapter.WorkflowHints)
			}
			if (adapter.BlenderReferenceHints != nil) != tt.wantBlenderHint {
				t.Fatalf("blender reference hints=%#v", adapter.BlenderReferenceHints)
			}
		})
	}
}

func TestDeferredStandaloneAdaptersAreFutureScope(t *testing.T) {
	for _, id := range []string{"Hunyuan3D", "TripoSR", "Stable-Fast-3D", "TRELLIS.2", "ComfyUI-3D-Pack"} {
		t.Run(id, func(t *testing.T) {
			if _, ok := GetDefaultAdapter(id); ok {
				t.Fatalf("%q must not be an initial default adapter", id)
			}
			if reason, ok := DeferredAdapterReason(id); !ok || reason == "" {
				t.Fatalf("%q must be marked deferred future scope, reason=%q ok=%v", id, reason, ok)
			}
		})
	}
}

func TestDefaultAdapterCatalogReturnsDefensiveCopies(t *testing.T) {
	catalog := DefaultAdapterCatalog()
	catalog[0].SupportedKinds[0] = "mutated"
	catalog[0].VisualPrerequisites.RequiredGates[0] = "mutated"
	again := DefaultAdapterCatalog()
	if again[0].SupportedKinds[0] == "mutated" {
		t.Fatal("expected default adapter catalog to return defensive copies")
	}
	if again[0].VisualPrerequisites.RequiredGates[0] == "mutated" {
		t.Fatal("expected visual prerequisites to return defensive copies")
	}
}

func TestVisualAdapterPrerequisitesBlockReadinessUntilWorkflowApproved(t *testing.T) {
	for _, id := range []string{AdapterComfyUIWorkflows, AdapterBlenderReferenceModels} {
		t.Run(id, func(t *testing.T) {
			adapter, ok := GetDefaultAdapter(id)
			if !ok {
				t.Fatalf("expected adapter %q", id)
			}
			prereq := adapter.VisualPrerequisites
			if prereq == nil {
				t.Fatalf("visual workflow prerequisites missing: %#v", adapter)
			}
			if prereq.Version != VisualWorkflowVersion {
				t.Fatalf("prerequisite version=%q want %q", prereq.Version, VisualWorkflowVersion)
			}
			if !prereq.BlocksReadiness || prereq.ReadinessStatus == VisualStatusReadyForAdapter {
				t.Fatalf("adapter readiness must remain blocked by default: %#v", prereq)
			}
			for _, gate := range []string{"approved_art_bible", "asset_spec_with_art_bible_link", "read_only_asset_audit"} {
				if !containsString(prereq.RequiredGates, gate) {
					t.Fatalf("required gate %q missing: %#v", gate, prereq.RequiredGates)
				}
			}
		})
	}
}
