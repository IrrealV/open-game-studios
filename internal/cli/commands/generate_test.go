package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"open-game-studios/internal/assets"
	"open-game-studios/internal/persistence"
	"open-game-studios/internal/templates"
	"open-game-studios/internal/workdoc"
	"open-game-studios/internal/workflows/coregame"
	"open-game-studios/internal/workflows/modelrouting"
)

func TestBuildProfileTemplateDataIncludesDefaultAssetPipelineMetadata(t *testing.T) {
	pack, ok := templates.NewRegistry().GetPack(templates.EngineGodot)
	if !ok {
		t.Fatal("expected godot pack metadata")
	}
	data := buildProfileTemplateData(workdoc.Document{ProfileName: "Game-Studio", PrimaryEngine: "godot", PersistenceMode: "hybrid"}, templates.EngineGodot, pack, outputPaths{}, newResolvedPreset("recommended", presetSourceDefault))

	if data.AssetPipeline.ContractVersion != assets.SchemaVersion {
		t.Fatalf("contract version=%q want %q", data.AssetPipeline.ContractVersion, assets.SchemaVersion)
	}
	if len(data.AssetPipeline.AllowedAdapters) != 2 || data.AssetPipeline.AllowedAdapters[0].ID != assets.AdapterComfyUIWorkflows || data.AssetPipeline.AllowedAdapters[1].ID != assets.AdapterBlenderReferenceModels {
		t.Fatalf("allowed adapters=%#v", data.AssetPipeline.AllowedAdapters)
	}
	if len(data.AssetPipeline.Selected) != 0 {
		t.Fatalf("default selected adapters=%#v want empty", data.AssetPipeline.Selected)
	}
	if !strings.Contains(data.AssetPipeline.Boundary, "metadata only") || !strings.Contains(data.AssetPipeline.Boundary, "no install") {
		t.Fatalf("metadata-only boundary missing: %q", data.AssetPipeline.Boundary)
	}
	if data.VisualWorkflow.Contract.Version != assets.VisualWorkflowVersion {
		t.Fatalf("visual workflow version=%q want %q", data.VisualWorkflow.Contract.Version, assets.VisualWorkflowVersion)
	}
	if data.VisualWorkflow.Contract.Paths.ArtBible != assets.VisualArtBiblePath || data.VisualWorkflow.Contract.Paths.AssetManifest != assets.VisualAssetManifestPath {
		t.Fatalf("visual workflow paths missing: %#v", data.VisualWorkflow.Contract.Paths)
	}
	if data.VisualWorkflow.Contract.ArtBible.ApprovalState != assets.VisualStatusPendingApproval || data.VisualWorkflow.Contract.Readiness.Status == assets.VisualStatusReadyForAdapter {
		t.Fatalf("visual workflow defaults must not imply approval/readiness: %#v", data.VisualWorkflow.Contract)
	}
	if data.CoreGameWorkflow.Contract.Version != coregame.Version {
		t.Fatalf("core game workflow version=%q want %q", data.CoreGameWorkflow.Contract.Version, coregame.Version)
	}
	if len(data.CoreGameWorkflow.Templates) != 3 {
		t.Fatalf("expected core game markdown templates, got %#v", data.CoreGameWorkflow.Templates)
	}
	if data.ModelRouting.Contract.Version != modelrouting.Version {
		t.Fatalf("model routing version=%q want %q", data.ModelRouting.Contract.Version, modelrouting.Version)
	}
}

func TestGeneratePackArtifactsRendersAssetPipelineMetadataOnly(t *testing.T) {
	outDir := t.TempDir()
	registry := templates.NewRegistry()
	doc := workdoc.Document{ProfileName: "Game-Studio", PrimaryEngine: "godot", PersistenceMode: "hybrid"}
	selectedAdapter, ok := assets.GetDefaultAdapter(assets.AdapterComfyUIWorkflows)
	if !ok {
		t.Fatal("expected comfyui adapter fixture")
	}

	_, err := generatePackArtifactsWithAssetPipeline(GenerateInput{Registry: registry, Persistence: persistence.NewPlaceholder()}, doc, templates.EngineGodot, "", outDir, "flat", map[string]bool{artifactProfile: true, artifactSummary: true, artifactPattern: true, artifactPackConfig: true}, defaultAssetPipelineProfileData([]assets.AdapterCapability{selectedAdapter}), newResolvedPreset("recommended", presetSourceDefault))
	if err != nil {
		t.Fatalf("generatePackArtifactsWithAssetPipeline returned error: %v", err)
	}

	profileRaw, err := os.ReadFile(filepath.Join(outDir, "studio-profile.godot.md"))
	if err != nil {
		t.Fatalf("read profile artifact: %v", err)
	}
	profile := string(profileRaw)
	for _, token := range []string{"Visual Asset Pipeline Contract", "Visual Workflow Contract", "Core Game Workflow Contract", "Model Routing Contract", coregame.Version, modelrouting.Version, "default_text", "reasoning_heavy", "code_apply", "qa_review", modelrouting.BindingSourceManualOrRuntimeOwned, "game-concept", "game-pillars", "core-loop", "explicit_story", "none_or_mechanics_first", "reported issue → triage → classification → relevant artifacts → repair/change brief → human approval → downstream handoff → verification", "/art-bible", "/asset-spec", "/asset-audit", assets.VisualWorkflowVersion, assets.AdapterComfyUIWorkflows, assets.AdapterBlenderReferenceModels, "metadata only", "no Godot project files", "Blender scene files"} {
		if !strings.Contains(profile, token) {
			t.Fatalf("expected profile to contain %q: %s", token, profile)
		}
	}
	for _, forbidden := range []string{"tools installed", "models downloaded", "APIs called", "assets generated"} {
		if strings.Contains(profile, forbidden+": true") {
			t.Fatalf("profile must not claim %q: %s", forbidden, profile)
		}
	}

	summaryRaw, err := os.ReadFile(filepath.Join(outDir, "studio-profile.godot.summary.md"))
	if err != nil {
		t.Fatalf("read summary artifact: %v", err)
	}
	summary := string(summaryRaw)
	for _, token := range []string{"optional_capabilities: archive_summary, visual_review, godot_technical_review", "future_capabilities: image_generation, image_review, audio_generation, audio_review"} {
		if !strings.Contains(summary, token) {
			t.Fatalf("expected summary to contain %q: %s", token, summary)
		}
	}
	legacyMixedField := "optional_" + "future_" + "capabilities"
	if strings.Contains(summary, legacyMixedField) {
		t.Fatalf("summary must not mix optional non-future capabilities into %s: %s", legacyMixedField, summary)
	}

	var config map[string]any
	configRaw, err := os.ReadFile(filepath.Join(outDir, "pack.godot.config.json"))
	if err != nil {
		t.Fatalf("read pack config: %v", err)
	}
	if err := json.Unmarshal(configRaw, &config); err != nil {
		t.Fatalf("pack config must remain valid JSON: %v", err)
	}
	assetPipeline, ok := config["asset_pipeline"].(map[string]any)
	if !ok {
		t.Fatalf("asset_pipeline metadata missing from pack config: %#v", config)
	}
	if !strings.Contains(assetPipeline["boundary"].(string), "metadata only") {
		t.Fatalf("unexpected asset pipeline boundary: %#v", assetPipeline["boundary"])
	}
	visualWorkflow, ok := config["visual_workflow"].(map[string]any)
	if !ok {
		t.Fatalf("visual_workflow metadata missing from pack config: %#v", config)
	}
	if visualWorkflow["contract_version"] != assets.VisualWorkflowVersion {
		t.Fatalf("visual_workflow contract=%#v", visualWorkflow["contract_version"])
	}
	if visualWorkflow["separate_from_asset_manifest"] != true {
		t.Fatalf("visual workflow must remain separate from asset manifest: %#v", visualWorkflow)
	}
	coreWorkflow, ok := config["core_game_workflow"].(map[string]any)
	if !ok {
		t.Fatalf("core_game_workflow metadata missing from pack config: %#v", config)
	}
	if coreWorkflow["contract_version"] != coregame.Version {
		t.Fatalf("core_game_workflow contract=%#v", coreWorkflow["contract_version"])
	}
	approvalPolicy, ok := coreWorkflow["approval_policy"].(map[string]any)
	if !ok || approvalPolicy["auto_approved"] != false || approvalPolicy["approval_state"] != "pending_human_approval" {
		t.Fatalf("core_game_workflow approval policy invalid: %#v", coreWorkflow["approval_policy"])
	}
	if defaults, ok := approvalPolicy["artifact_defaults"].([]any); !ok || len(defaults) != len(coregame.DefaultContract().PhaseIDs) {
		t.Fatalf("core_game_workflow artifact defaults invalid: %#v", approvalPolicy["artifact_defaults"])
	}
	if modeContracts, ok := coreWorkflow["mode_contracts"].([]any); !ok || len(modeContracts) != 3 {
		t.Fatalf("core_game_workflow mode contracts invalid: %#v", coreWorkflow["mode_contracts"])
	}
	if downstream, ok := coreWorkflow["downstream_references"].([]any); !ok || len(downstream) == 0 {
		t.Fatalf("core_game_workflow downstream relationships invalid: %#v", coreWorkflow["downstream_references"])
	} else if _, ok := downstream[0].(map[string]any); !ok {
		t.Fatalf("core_game_workflow downstream references must be structured objects: %#v", downstream[0])
	}
	modelRouting, ok := config["model_routing"].(map[string]any)
	if !ok {
		t.Fatalf("model_routing metadata missing from pack config: %#v", config)
	}
	assertModelRoutingArtifact(t, modelRouting)
	assertCapabilityCatalogFields(t, modelRouting)

	var hybrid map[string]any
	hybridRaw, err := os.ReadFile(filepath.Join(outDir, "pack.godot.hybrid-map.json"))
	if err != nil {
		t.Fatalf("read hybrid map: %v", err)
	}
	if err := json.Unmarshal(hybridRaw, &hybrid); err != nil {
		t.Fatalf("hybrid map must remain valid JSON: %v", err)
	}
	if _, ok := hybrid["visual_workflow"].(map[string]any); !ok {
		t.Fatalf("visual_workflow gates missing from hybrid map: %#v", hybrid)
	}
	if _, ok := hybrid["core_game_workflow"].(map[string]any); !ok {
		t.Fatalf("core_game_workflow missing from hybrid map: %#v", hybrid)
	} else {
		hybridWorkflow := hybrid["core_game_workflow"].(map[string]any)
		hybridApproval, ok := hybridWorkflow["approval_policy"].(map[string]any)
		if !ok {
			t.Fatalf("core_game_workflow approval policy missing from hybrid map: %#v", hybridWorkflow)
		}
		if defaults, ok := hybridApproval["artifact_defaults"].([]any); !ok || len(defaults) != len(coregame.DefaultContract().PhaseIDs) {
			t.Fatalf("core_game_workflow artifact defaults missing from hybrid map: %#v", hybridApproval)
		}
		assertCoreWorkflowNoExecutionClaims(t, hybridWorkflow["no_execution_claims"])
	}
	if modelRouting, ok := hybrid["model_routing"].(map[string]any); !ok {
		t.Fatalf("model_routing missing from hybrid map: %#v", hybrid)
	} else {
		assertModelRoutingArtifact(t, modelRouting)
		assertCapabilityCatalogFields(t, modelRouting)
	}
	assertCoreWorkflowNoExecutionClaims(t, coreWorkflow["no_execution_claims"])
}

func assertModelRoutingArtifact(t *testing.T, raw map[string]any) {
	t.Helper()
	if raw["version"] != modelrouting.Version {
		t.Fatalf("model routing version invalid: %#v", raw["version"])
	}
	capabilities, ok := raw["capabilities"].([]any)
	if !ok || len(capabilities) == 0 {
		t.Fatalf("model routing capabilities missing: %#v", raw)
	}
	for _, id := range []string{"default_text", "reasoning_heavy", "fast_text", "code_apply", "qa_review", "image_generation", "image_review", "audio_generation", "audio_review"} {
		if !containsObjectString(capabilities, "id", id) {
			t.Fatalf("model routing capability %q missing: %#v", id, capabilities)
		}
	}
	bindings, ok := raw["provider_model_bindings"].([]any)
	if !ok || len(bindings) == 0 {
		t.Fatalf("model routing bindings missing: %#v", raw)
	}
	for _, bindingRaw := range bindings {
		binding := bindingRaw.(map[string]any)
		if binding["provider"] != "not_configured" || binding["model"] != "not_configured" || binding["binding_source"] != modelrouting.BindingSourceManualOrRuntimeOwned {
			t.Fatalf("model routing binding must be unconfigured metadata: %#v", binding)
		}
	}
	claims, ok := raw["no_execution_claims"].(map[string]any)
	if !ok {
		t.Fatalf("model routing no-execution claims missing: %#v", raw)
	}
	for _, claim := range []string{"provider_api_calls", "model_downloads", "image_generation", "audio_generation", "sdd_execution", "godot_mutation", "comfyui_execution", "blender_execution", "auto_approval"} {
		if claims[claim] != false {
			t.Fatalf("model routing no-execution claim %q invalid: %#v", claim, claims)
		}
	}
}

func assertCapabilityCatalogFields(t *testing.T, raw map[string]any) {
	t.Helper()
	assertStringArrayField(t, raw, "optional_capabilities", []string{"archive_summary", "visual_review", "godot_technical_review"})
	assertStringArrayField(t, raw, "future_capabilities", []string{"image_generation", "image_review", "audio_generation", "audio_review"})

	optional := raw["optional_capabilities"].([]any)
	for _, id := range []string{"image_generation", "image_review", "audio_generation", "audio_review"} {
		if containsStringValue(optional, id) {
			t.Fatalf("optional_capabilities must not contain future capability %q: %#v", id, optional)
		}
	}

	future := raw["future_capabilities"].([]any)
	for _, id := range []string{"archive_summary", "visual_review", "godot_technical_review"} {
		if containsStringValue(future, id) {
			t.Fatalf("future_capabilities must not contain optional capability %q: %#v", id, future)
		}
	}
}

func assertStringArrayField(t *testing.T, raw map[string]any, field string, want []string) {
	t.Helper()
	items, ok := raw[field].([]any)
	if !ok {
		t.Fatalf("%s missing or not an array: %#v", field, raw[field])
	}
	for _, id := range want {
		if !containsStringValue(items, id) {
			t.Fatalf("%s missing %q: %#v", field, id, items)
		}
	}
}

func containsStringValue(items []any, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func containsObjectString(items []any, key, value string) bool {
	for _, item := range items {
		object, ok := item.(map[string]any)
		if ok && object[key] == value {
			return true
		}
	}
	return false
}

func assertCoreWorkflowNoExecutionClaims(t *testing.T, raw any) {
	t.Helper()
	claims, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("core_game_workflow no_execution_claims missing: %#v", raw)
	}
	for _, claim := range []string{"debugging_executed", "image_generated", "audio_generated", "playtesting_run", "engine_files_mutated", "collaboration_workflow_executed", "claude_native_mechanics_enabled"} {
		value, ok := claims[claim].(bool)
		if !ok || value {
			t.Fatalf("core_game_workflow no_execution_claim %q invalid: %#v", claim, claims)
		}
	}
}

func TestValidateCoreGameWorkflowRejectsIncompleteStructuredMetadata(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{name: "mode contract without sequence", mutate: func(workflow map[string]any) {
			modes := workflow["mode_contracts"].([]any)
			modes[0].(map[string]any)["sequence"] = []any{}
		}},
		{name: "mode contract sequence out of order", mutate: func(workflow map[string]any) {
			modes := workflow["mode_contracts"].([]any)
			sequence := modes[0].(map[string]any)["sequence"].([]any)
			sequence[0], sequence[1] = sequence[1], sequence[0]
		}},
		{name: "missing artifact defaults", mutate: func(workflow map[string]any) {
			workflow["approval_policy"].(map[string]any)["artifact_defaults"] = []any{}
		}},
		{name: "duplicate artifact default", mutate: func(workflow map[string]any) {
			defaults := workflow["approval_policy"].(map[string]any)["artifact_defaults"].([]any)
			workflow["approval_policy"].(map[string]any)["artifact_defaults"] = append(defaults, defaults[0])
		}},
		{name: "unknown auto-approved artifact default", mutate: func(workflow map[string]any) {
			defaults := workflow["approval_policy"].(map[string]any)["artifact_defaults"].([]any)
			workflow["approval_policy"].(map[string]any)["artifact_defaults"] = append(defaults, map[string]any{"artifact_id": "unknown", "status": "draft", "approval_state": "pending_human_approval", "auto_approved": true})
		}},
		{name: "downstream reference without consumes", mutate: func(workflow map[string]any) {
			downstream := workflow["downstream_references"].([]any)
			downstream[0].(map[string]any)["consumes"] = []any{}
		}},
		{name: "asset spec without approved intent", mutate: func(workflow map[string]any) {
			downstream := workflow["downstream_references"].([]any)
			for _, item := range downstream {
				entry := item.(map[string]any)
				if entry["target"] == "Asset Spec" {
					entry["consumes"] = []any{"gdd slice", "mechanics brief", "Art Bible"}
				}
			}
		}},
		{name: "QA checklist without verifies", mutate: func(workflow map[string]any) {
			downstream := workflow["downstream_references"].([]any)
			for _, item := range downstream {
				entry := item.(map[string]any)
				if entry["target"] == "QA/review checklist" {
					entry["verifies"] = []any{}
				}
			}
		}},
		{name: "collaboration claim true", mutate: func(workflow map[string]any) {
			workflow["no_execution_claims"].(map[string]any)["collaboration_workflow_executed"] = true
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := map[string]any{"core_game_workflow": coreGameWorkflowFixture()}
			workflow := payload["core_game_workflow"].(map[string]any)
			tt.mutate(workflow)
			if err := validateCoreGameWorkflow(payload, "fixture.json"); err == nil {
				t.Fatalf("validateCoreGameWorkflow should reject %s", tt.name)
			}
		})
	}
}

func TestGeneratePackArtifactsDoesNotMutateGodotOrBlenderProjectFiles(t *testing.T) {
	root := t.TempDir()
	projectDir := filepath.Join(root, "game-project")
	blenderDir := filepath.Join(root, "blender")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("create godot fixture: %v", err)
	}
	if err := os.MkdirAll(blenderDir, 0o755); err != nil {
		t.Fatalf("create blender fixture: %v", err)
	}

	fixtures := map[string]string{
		filepath.Join(projectDir, "project.godot"): "[application]\nconfig/name=Fixture\n",
		filepath.Join(projectDir, "asset.import"):  "[remap]\nimporter=texture\n",
		filepath.Join(blenderDir, "scene.blend"):   "fake-blender-scene-bytes",
	}
	for path, contents := range fixtures {
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatalf("write fixture %s: %v", path, err)
		}
	}

	comfy, ok := assets.GetDefaultAdapter(assets.AdapterComfyUIWorkflows)
	if !ok {
		t.Fatal("expected comfyui adapter fixture")
	}
	blender, ok := assets.GetDefaultAdapter(assets.AdapterBlenderReferenceModels)
	if !ok {
		t.Fatal("expected blender adapter fixture")
	}

	_, err := generatePackArtifactsWithAssetPipeline(
		GenerateInput{Registry: templates.NewRegistry(), Persistence: persistence.NewPlaceholder()},
		workdoc.Document{ProfileName: "Game-Studio", PrimaryEngine: "godot", PersistenceMode: "hybrid"},
		templates.EngineGodot,
		"",
		filepath.Join(root, "generated"),
		"flat",
		map[string]bool{artifactProfile: true, artifactSummary: true, artifactPackConfig: true},
		defaultAssetPipelineProfileData([]assets.AdapterCapability{comfy, blender}),
		newResolvedPreset("recommended", presetSourceDefault),
	)
	if err != nil {
		t.Fatalf("generatePackArtifactsWithAssetPipeline returned error: %v", err)
	}

	for path, want := range fixtures {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read fixture %s after generation: %v", path, err)
		}
		if string(got) != want {
			t.Fatalf("generation mutated fixture %s: got %q want %q", path, got, want)
		}
	}
}
