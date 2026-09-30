package integrations

import (
	"testing"

	"open-game-studios/internal/assets"
)

func TestDefaultRegistryCatalog(t *testing.T) {
	registry := NewDefaultRegistry()

	items := registry.List()
	if len(items) != 4 {
		t.Fatalf("expected four default integrations, got %d", len(items))
	}
	if items[0].ID != BlenderReferenceModelingID || items[1].ID != ComfyUIWorkflowsID || items[2].ID != EngramMonitorID || items[3].ID != MetronousID {
		t.Fatalf("default integrations ids=%q,%q,%q,%q", items[0].ID, items[1].ID, items[2].ID, items[3].ID)
	}

	tests := []struct {
		id              string
		label           string
		category        string
		defaultSelected bool
		wantDiagnostic  bool
		wantHints       bool
		wantAsset       bool
	}{
		{id: BlenderReferenceModelingID, label: "Blender Reference Modeling", category: "asset-adapter", wantHints: true, wantAsset: true},
		{id: ComfyUIWorkflowsID, label: "ComfyUI Workflows", category: "asset-adapter", wantHints: true, wantAsset: true},
		{id: EngramMonitorID, label: "Engram Monitor", category: "observability", wantDiagnostic: true, wantHints: true},
		{id: MetronousID, label: "Metronous", category: "telemetry", wantDiagnostic: true, wantHints: true},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			item, ok := registry.Get(tt.id)
			if !ok {
				t.Fatalf("expected %q to be registered", tt.id)
			}
			if item.Label != tt.label || item.Category != tt.category {
				t.Fatalf("metadata=%#v", item)
			}
			if item.DefaultSelected != tt.defaultSelected {
				t.Fatalf("DefaultSelected=%v want %v", item.DefaultSelected, tt.defaultSelected)
			}
			if (len(item.DiagnosticToolIDs) > 0) != tt.wantDiagnostic {
				t.Fatalf("diagnostics=%#v", item.DiagnosticToolIDs)
			}
			if (len(item.Hints) > 0) != tt.wantHints {
				t.Fatalf("hints=%#v", item.Hints)
			}
			if (item.AssetAdapter != nil) != tt.wantAsset {
				t.Fatalf("asset adapter metadata=%#v", item.AssetAdapter)
			}
			if item.AssetAdapter != nil && item.AssetAdapter.ExecutionOwnership != assets.ExecutionMetadataOnly {
				t.Fatalf("asset adapter must be metadata only: %#v", item.AssetAdapter)
			}
		})
	}
}

func TestDefaultRegistryLookupIsStableAndDefensive(t *testing.T) {
	registry := NewDefaultRegistry()

	item, ok := registry.Get("  METRONOUS  ")
	if !ok || item.ID != MetronousID {
		t.Fatalf("expected normalized Metronous lookup, got %#v ok=%v", item, ok)
	}
	item.Hints[0] = "mutated"

	again, ok := registry.Get(MetronousID)
	if !ok {
		t.Fatal("expected metronous lookup")
	}
	if again.Hints[0] == "mutated" {
		t.Fatal("expected registry lookup to return defensive copies")
	}
}

func TestDefaultRegistryExcludesDeferredStandalone3DAdapters(t *testing.T) {
	registry := NewDefaultRegistry()
	for _, id := range []string{"hunyuan3d", "triposr", "stable-fast-3d", "trellis.2", "comfyui-3d-pack"} {
		t.Run(id, func(t *testing.T) {
			if _, ok := registry.Get(id); ok {
				t.Fatalf("%q must not be selectable in initial registry", id)
			}
		})
	}
}

func TestDefaultRegistryAssetAdapterCopiesAreDefensive(t *testing.T) {
	registry := NewDefaultRegistry()
	item, ok := registry.Get(ComfyUIWorkflowsID)
	if !ok || item.AssetAdapter == nil {
		t.Fatalf("expected comfyui asset adapter metadata")
	}
	item.AssetAdapter.SupportedKinds[0] = "mutated"
	item.AssetAdapter.WorkflowHints[0].RequiredInputs[0] = "mutated"
	item.AssetAdapter.ModelHints[0].Loras[0] = "mutated"
	again, ok := registry.Get(ComfyUIWorkflowsID)
	if !ok || again.AssetAdapter == nil {
		t.Fatalf("expected comfyui asset adapter metadata on second lookup")
	}
	if again.AssetAdapter.SupportedKinds[0] == "mutated" {
		t.Fatal("expected asset adapter metadata to be defensively copied")
	}
	if again.AssetAdapter.WorkflowHints[0].RequiredInputs[0] == "mutated" {
		t.Fatal("expected workflow required inputs to be defensively copied")
	}
	if again.AssetAdapter.ModelHints[0].Loras[0] == "mutated" {
		t.Fatal("expected model hint loras to be defensively copied")
	}

	blender, ok := registry.Get(BlenderReferenceModelingID)
	if !ok || blender.AssetAdapter == nil || blender.AssetAdapter.BlenderReferenceHints == nil {
		t.Fatalf("expected blender asset adapter metadata")
	}
	blender.AssetAdapter.BlenderReferenceHints.StyleConstraints[0] = "mutated"
	blender.AssetAdapter.BlenderReferenceHints.AllowedOperations[0] = "mutated"
	blender.AssetAdapter.BlenderReferenceHints.ExportImportHints[0] = "mutated"
	blenderAgain, ok := registry.Get(BlenderReferenceModelingID)
	if !ok || blenderAgain.AssetAdapter == nil || blenderAgain.AssetAdapter.BlenderReferenceHints == nil {
		t.Fatalf("expected blender asset adapter metadata on second lookup")
	}
	if blenderAgain.AssetAdapter.BlenderReferenceHints.StyleConstraints[0] == "mutated" {
		t.Fatal("expected blender style constraints to be defensively copied")
	}
	if blenderAgain.AssetAdapter.BlenderReferenceHints.AllowedOperations[0] == "mutated" {
		t.Fatal("expected blender allowed operations to be defensively copied")
	}
	if blenderAgain.AssetAdapter.BlenderReferenceHints.ExportImportHints[0] == "mutated" {
		t.Fatal("expected blender export/import hints to be defensively copied")
	}
}

func TestMetronousConsentMetadata(t *testing.T) {
	item, ok := NewDefaultRegistry().Get(MetronousID)
	if !ok {
		t.Fatal("expected metronous integration")
	}
	if len(item.Consent) != 1 {
		t.Fatalf("consent=%#v, want one requirement", item.Consent)
	}
	consent := item.Consent[0]
	if consent.ID != TelemetryPrivacyConsentID || consent.Label == "" || consent.RequiredText == "" {
		t.Fatalf("unexpected consent metadata: %#v", consent)
	}
}
