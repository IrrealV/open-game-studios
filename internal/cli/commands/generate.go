package commands

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"

	"open-game-studios/internal/assets"
	"open-game-studios/internal/persistence"
	"open-game-studios/internal/templates"
	"open-game-studios/internal/workdoc"
	"open-game-studios/internal/workflows/coregame"
	"open-game-studios/internal/workflows/modelrouting"
)

type GenerateInput struct {
	Args        []string
	Registry    *templates.Registry
	Persistence persistence.Placeholder
}

const (
	artifactProfile    = "profile"
	artifactSummary    = "summary"
	artifactPattern    = "pattern"
	artifactPackConfig = "pack-config"
)

func RunGenerate(input GenerateInput) error {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	docPath := fs.String("doc", "GAME-STUDIO.md", "path to Game-Studio working document")
	engine := fs.String("engine", string(templates.EngineGodot), "target engine (godot|unity|ue5)")
	outPath := fs.String("out", "", "explicit profile artifact path override")
	outDir := fs.String("out-dir", "profiles/game-studio/generated", "output base directory")
	layout := fs.String("layout", "flat", "artifact layout (flat|pack)")
	artifacts := fs.String("artifacts", "all", "artifacts to emit (all|profile|summary|pattern|pack-config|comma-separated)")
	setupDepth := fs.String("setup-depth", "", "invocation-only setup preset override (minimal|recommended|full|custom); otherwise use workspace config or recommended default")
	if err := fs.Parse(input.Args); err != nil {
		return err
	}
	explicitFlags := visitedFlags(fs)

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}
	workspaceRoot := detectWorkspaceRoot(cwd)
	preset, err := resolveGeneratePreset(workspaceRoot, *setupDepth, explicitFlags["setup-depth"])
	if err != nil {
		return fmt.Errorf("generate preset resolution failed: %w", err)
	}

	resolvedDocPath := resolveWorkspacePath(workspaceRoot, *docPath)
	doc, err := workdoc.LoadAndValidate(resolvedDocPath)
	if err != nil {
		return fmt.Errorf("generate validation failed: %w", err)
	}

	targetEngine := templates.Engine(strings.ToLower(strings.TrimSpace(*engine)))
	if targetEngine == "" {
		targetEngine = templates.EngineGodot
	}

	selected, err := resolveArtifacts(*artifacts)
	if err != nil {
		return err
	}

	resolvedOutPath := ""
	if strings.TrimSpace(*outPath) != "" {
		resolvedOutPath = resolveWorkspacePath(workspaceRoot, *outPath)
	}
	resolvedOutDir := resolveWorkspacePath(workspaceRoot, *outDir)
	emitted, err := generatePackArtifacts(input, doc, targetEngine, resolvedOutPath, resolvedOutDir, *layout, selected, preset)
	if err != nil {
		return err
	}

	fmt.Println("[generate] profile generation complete")
	fmt.Printf("[generate] templates registered: %d\n", input.Registry.Count())
	fmt.Printf("[generate] persistence mode: %s\n", input.Persistence.Mode())
	fmt.Printf("[generate] engine: %s\n", targetEngine)
	fmt.Printf("[generate] setup preset: %s (%s)\n", preset.SetupPreset, preset.PresetSource)
	fmt.Printf("[generate] artifacts emitted: %d\n", len(emitted))
	for _, artifact := range emitted {
		fmt.Printf("[generate] artifact: %s\n", artifact)
	}

	return nil
}

func generatePackArtifacts(input GenerateInput, doc workdoc.Document, targetEngine templates.Engine, profileOutPath, outDir, layout string, selected map[string]bool, preset resolvedPreset) ([]string, error) {
	if err := input.Registry.ValidatePackLoadable(targetEngine); err != nil {
		return nil, err
	}

	pack, _ := input.Registry.GetPack(targetEngine)

	paths, err := resolveOutputPaths(targetEngine, profileOutPath, outDir, layout)
	if err != nil {
		return nil, err
	}
	data := buildProfileTemplateData(doc, targetEngine, pack, paths, preset)

	emitted := []string{}
	if selected[artifactProfile] {
		profileArtifact, renderErr := renderTemplateToFile(input.Registry, pack.ProfileTemplateID, data, paths.profile)
		if renderErr != nil {
			return nil, renderErr
		}
		emitted = append(emitted, profileArtifact)
	}

	if selected[artifactSummary] {
		summaryArtifact, renderErr := renderTemplateToFile(input.Registry, pack.SummaryTemplateID, data, paths.summary)
		if renderErr != nil {
			return nil, renderErr
		}
		emitted = append(emitted, summaryArtifact)
	}

	if selected[artifactPattern] {
		patternArtifact, renderErr := renderTemplateToFile(input.Registry, pack.PatternTemplateID, data, paths.pattern)
		if renderErr != nil {
			return nil, renderErr
		}
		emitted = append(emitted, patternArtifact)
	}

	if selected[artifactPackConfig] {
		configArtifact, renderErr := renderTemplateToFile(input.Registry, pack.ConfigTemplateID, data, paths.config)
		if renderErr != nil {
			return nil, renderErr
		}
		emitted = append(emitted, configArtifact)
	}

	if len(emitted) == 0 {
		return nil, fmt.Errorf("no artifacts selected")
	}

	return emitted, nil
}

type outputPaths struct {
	profile string
	summary string
	pattern string
	config  string
}

func resolveOutputPaths(targetEngine templates.Engine, profileOutPath, outDir, layout string) (outputPaths, error) {
	switch strings.ToLower(strings.TrimSpace(layout)) {
	case "flat":
		profile := filepath.Join(outDir, fmt.Sprintf("studio-profile.%s.md", targetEngine))
		if strings.TrimSpace(profileOutPath) != "" {
			profile = profileOutPath
		}
		return outputPaths{
			profile: profile,
			summary: filepath.Join(outDir, fmt.Sprintf("studio-profile.%s.summary.md", targetEngine)),
			pattern: filepath.Join(outDir, fmt.Sprintf("pack.%s.hybrid-map.json", targetEngine)),
			config:  filepath.Join(outDir, fmt.Sprintf("pack.%s.config.json", targetEngine)),
		}, nil
	case "pack":
		base := filepath.Join(outDir, "packs", string(targetEngine))
		profile := filepath.Join(base, "profile.md")
		if strings.TrimSpace(profileOutPath) != "" {
			profile = profileOutPath
		}
		return outputPaths{
			profile: profile,
			summary: filepath.Join(base, "summary.md"),
			pattern: filepath.Join(base, "hybrid-map.json"),
			config:  filepath.Join(base, "pack.config.json"),
		}, nil
	default:
		return outputPaths{}, fmt.Errorf("invalid layout %q: use flat or pack", layout)
	}
}

func resolveArtifacts(raw string) (map[string]bool, error) {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		value = "all"
	}

	if value == "all" {
		return map[string]bool{
			artifactProfile:    true,
			artifactSummary:    true,
			artifactPattern:    true,
			artifactPackConfig: true,
		}, nil
	}

	selected := map[string]bool{}
	for _, item := range strings.Split(value, ",") {
		normalized := strings.TrimSpace(item)
		switch normalized {
		case artifactProfile, artifactSummary, artifactPattern, artifactPackConfig:
			selected[normalized] = true
		case "":
			continue
		default:
			return nil, fmt.Errorf("invalid artifact %q", normalized)
		}
	}

	if len(selected) == 0 {
		return nil, fmt.Errorf("no valid artifacts selected")
	}

	return selected, nil
}

func renderTemplateToFile(registry *templates.Registry, templateID string, data profileTemplateData, outPath string) (string, error) {
	tplMeta, ok := registry.Get(templateID)
	if !ok {
		return "", fmt.Errorf("template not found: %q", templateID)
	}

	source, err := templates.FS.ReadFile(tplMeta.Path)
	if err != nil {
		return "", fmt.Errorf("read template %s: %w", tplMeta.Path, err)
	}

	tpl, err := template.New(tplMeta.ID).Parse(string(source))
	if err != nil {
		return "", fmt.Errorf("parse template %s: %w", tplMeta.ID, err)
	}

	var rendered bytes.Buffer
	if err := tpl.Execute(&rendered, data); err != nil {
		return "", fmt.Errorf("render template %s: %w", tplMeta.ID, err)
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return "", fmt.Errorf("create output directory: %w", err)
	}

	if err := os.WriteFile(outPath, rendered.Bytes(), 0o644); err != nil {
		return "", fmt.Errorf("write output artifact: %w", err)
	}

	return outPath, nil
}

func buildProfileTemplateData(doc workdoc.Document, targetEngine templates.Engine, pack templates.PackMetadata, paths outputPaths, preset resolvedPreset) profileTemplateData {
	layers := make([]string, 0, len(pack.LayerOrder))
	for _, layer := range pack.LayerOrder {
		layers = append(layers, string(layer))
	}

	return profileTemplateData{
		ProfileName:      doc.ProfileName,
		TargetEngine:     string(targetEngine),
		PersistenceMode:  doc.PersistenceMode,
		PrimaryEngineDoc: doc.PrimaryEngine,
		GeneratedAt:      time.Now().UTC().Format(time.RFC3339),
		PackID:           pack.ID,
		PackVersion:      pack.Version,
		PackStatus:       pack.Status,
		CCGSLayers:       layers,
		SkillAgentMap:    stableMapping(pack.SkillAgentMap),
		MCPPlaceholders: map[string]mcpPlaceholder{
			// MCP adapters are optional and never required. Memory uses the
			// Pi-native Engram companion; Context7 and other adapters are not
			// implicitly enabled or installed by the generator.
			"engram":   {Enabled: false, Mode: "placeholder", Required: false},
			"context7": {Enabled: false, Mode: "placeholder", Required: false},
		},
		ProfileArtifactPath: paths.profile,
		SummaryArtifactPath: paths.summary,
		PatternArtifactPath: paths.pattern,
		ConfigArtifactPath:  paths.config,
		SetupPreset:         preset.SetupPreset,
		PresetSource:        preset.PresetSource,
		RoutingPreset:       preset.RoutingPreset,
		RoutingPresetSource: preset.RoutingPresetSource,
		AssetPipeline:       defaultAssetPipelineProfileData(nil),
		VisualWorkflow:      defaultVisualWorkflowProfileData(),
		CoreGameWorkflow:    defaultCoreGameWorkflowProfileData(),
		ModelRouting:        defaultModelRoutingProfileData(preset.RoutingPreset),
	}
}

func generatePackArtifactsWithAssetPipeline(input GenerateInput, doc workdoc.Document, targetEngine templates.Engine, profileOutPath, outDir, layout string, selected map[string]bool, assetPipeline assetPipelineProfileData, preset resolvedPreset) ([]string, error) {
	if err := input.Registry.ValidatePackLoadable(targetEngine); err != nil {
		return nil, err
	}
	pack, _ := input.Registry.GetPack(targetEngine)
	paths, err := resolveOutputPaths(targetEngine, profileOutPath, outDir, layout)
	if err != nil {
		return nil, err
	}
	data := buildProfileTemplateData(doc, targetEngine, pack, paths, preset)
	data.AssetPipeline = assetPipeline

	emitted := []string{}
	if selected[artifactProfile] {
		profileArtifact, renderErr := renderTemplateToFile(input.Registry, pack.ProfileTemplateID, data, paths.profile)
		if renderErr != nil {
			return nil, renderErr
		}
		emitted = append(emitted, profileArtifact)
	}
	if selected[artifactSummary] {
		summaryArtifact, renderErr := renderTemplateToFile(input.Registry, pack.SummaryTemplateID, data, paths.summary)
		if renderErr != nil {
			return nil, renderErr
		}
		emitted = append(emitted, summaryArtifact)
	}
	if selected[artifactPattern] {
		patternArtifact, renderErr := renderTemplateToFile(input.Registry, pack.PatternTemplateID, data, paths.pattern)
		if renderErr != nil {
			return nil, renderErr
		}
		emitted = append(emitted, patternArtifact)
	}
	if selected[artifactPackConfig] {
		configArtifact, renderErr := renderTemplateToFile(input.Registry, pack.ConfigTemplateID, data, paths.config)
		if renderErr != nil {
			return nil, renderErr
		}
		emitted = append(emitted, configArtifact)
	}
	if len(emitted) == 0 {
		return nil, fmt.Errorf("no artifacts selected")
	}
	return emitted, nil
}

func stableMapping(items []templates.SkillAgentBinding) []templates.SkillAgentBinding {
	out := make([]templates.SkillAgentBinding, len(items))
	copy(out, items)
	sort.Slice(out, func(i, j int) bool {
		return out[i].Skill < out[j].Skill
	})
	return out
}

type profileTemplateData struct {
	ProfileName         string
	TargetEngine        string
	PersistenceMode     string
	PrimaryEngineDoc    string
	GeneratedAt         string
	PackID              string
	PackVersion         string
	PackStatus          string
	CCGSLayers          []string
	SkillAgentMap       []templates.SkillAgentBinding
	MCPPlaceholders     map[string]mcpPlaceholder
	ProfileArtifactPath string
	SummaryArtifactPath string
	PatternArtifactPath string
	ConfigArtifactPath  string
	SetupPreset         string
	PresetSource        string
	RoutingPreset       string
	RoutingPresetSource string
	AssetPipeline       assetPipelineProfileData
	VisualWorkflow      visualWorkflowProfileData
	CoreGameWorkflow    coreGameWorkflowProfileData
	ModelRouting        modelRoutingProfileData
}

type mcpPlaceholder struct {
	Enabled  bool
	Mode     string
	Required bool
}

type assetPipelineProfileData struct {
	ContractVersion  string
	Boundary         string
	AllowedAdapters  []assets.AdapterCapability
	Selected         []assets.AdapterCapability
	ValidationStates []string
	ApprovalStates   []string
	ImportTargets    []string
	FutureScope      []string
}

type visualWorkflowProfileData struct {
	Contract       assets.VisualWorkflowContract
	NoClaims       map[string]bool
	GuidanceRoutes []string
}

type coreGameWorkflowProfileData struct {
	Contract  coregame.Contract
	Templates []coregame.MarkdownTemplate
}

type modelRoutingProfileData struct {
	Contract modelrouting.Contract
}

func defaultAssetPipelineProfileData(selected []assets.AdapterCapability) assetPipelineProfileData {
	return assetPipelineProfileData{
		ContractVersion:  assets.SchemaVersion,
		Boundary:         "metadata only: no install, clone, build, run, API call, service management, conversion, asset generation, or Godot/Blender file mutation",
		AllowedAdapters:  assets.DefaultAdapterCatalog(),
		Selected:         selected,
		ValidationStates: []string{assets.ValidationDraft, assets.ValidationValid, assets.ValidationInvalid, assets.ValidationOutOfScope, assets.ValidationNeedsMetadata},
		ApprovalStates:   []string{assets.ApprovalDraft, assets.ApprovalValidated, assets.ApprovalNeedsRevision, assets.ApprovalApproved, assets.ApprovalImported, assets.ApprovalRejected},
		ImportTargets:    []string{assets.ImportTargetGodot, assets.ImportTargetBlender},
		FutureScope:      []string{"Hunyuan3D standalone adapter", "TripoSR standalone adapter", "Stable Fast 3D standalone adapter", "TRELLIS.2 standalone adapter", "ComfyUI-3D-Pack standalone entry"},
	}
}

func defaultVisualWorkflowProfileData() visualWorkflowProfileData {
	return visualWorkflowProfileData{
		Contract: assets.DefaultVisualWorkflow(),
		NoClaims: map[string]bool{
			"art_generated":         false,
			"art_approved":          false,
			"comfyui_executed":      false,
			"blender_executed":      false,
			"godot_files_mutated":   false,
			"dcc_files_mutated":     false,
			"audio_music_sfx_scope": false,
		},
		GuidanceRoutes: []string{"/art-bible", "/asset-spec", "/asset-audit"},
	}
}

func defaultCoreGameWorkflowProfileData() coreGameWorkflowProfileData {
	return coreGameWorkflowProfileData{
		Contract:  coregame.DefaultContract(),
		Templates: coregame.DefaultMarkdownTemplates(),
	}
}

func defaultModelRoutingProfileData(preset string) modelRoutingProfileData {
	return modelRoutingProfileData{Contract: modelrouting.DefaultContract(preset)}
}
