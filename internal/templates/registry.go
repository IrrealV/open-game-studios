package templates

import "fmt"

type Engine string

const (
	EngineGodot Engine = "godot"
	EngineUnity Engine = "unity"
	EngineUE5   Engine = "ue5"
)

type CCGSLayer string

const (
	LayerOrchestrator CCGSLayer = "orchestrator"
	LayerWorkflow     CCGSLayer = "workflow"
	LayerDomainPack   CCGSLayer = "domain-pack"
	LayerSDDHandoff   CCGSLayer = "sdd-handoff"
)

type Template struct {
	ID        string
	Path      string
	Engine    Engine
	Layer     CCGSLayer
	Profile   string
	Supported bool
}

type PackMetadata struct {
	ID                string
	Engine            Engine
	Version           string
	Status            string
	ProfileTemplateID string
	SummaryTemplateID string
	PatternTemplateID string
	ConfigTemplateID  string
	LayerOrder        []CCGSLayer
	SkillAgentMap     []SkillAgentBinding
	Supported         bool
}

type SkillAgentBinding struct {
	Skill string
	Agent string
	Role  string
}

type Registry struct {
	items map[string]Template
	packs map[Engine]PackMetadata
}

func NewRegistry() *Registry {
	items := map[string]Template{}

	// Godot: initial supported pack.
	items["game-studio/godot/profile"] = Template{
		ID:        "game-studio/godot/profile",
		Path:      "assets/profile/game-studio/godot/profile.md.tmpl",
		Engine:    EngineGodot,
		Layer:     LayerDomainPack,
		Profile:   "Game-Studio",
		Supported: true,
	}
	items["game-studio/godot/profile-summary"] = Template{
		ID:        "game-studio/godot/profile-summary",
		Path:      "assets/profile/game-studio/godot/profile.summary.md.tmpl",
		Engine:    EngineGodot,
		Layer:     LayerOrchestrator,
		Profile:   "Game-Studio",
		Supported: true,
	}
	items["game-studio/godot/pattern-hybrid-map"] = Template{
		ID:        "game-studio/godot/pattern-hybrid-map",
		Path:      "assets/profile/game-studio/godot/pattern.hybrid-map.json.tmpl",
		Engine:    EngineGodot,
		Layer:     LayerWorkflow,
		Profile:   "Game-Studio",
		Supported: true,
	}
	items["game-studio/godot/pack-config"] = Template{
		ID:        "game-studio/godot/pack-config",
		Path:      "assets/profile/game-studio/godot/pack.config.json.tmpl",
		Engine:    EngineGodot,
		Layer:     LayerDomainPack,
		Profile:   "Game-Studio",
		Supported: true,
	}

	// Future packs are represented now to keep registry contract stable.
	items["game-studio/unity/profile"] = Template{
		ID:        "game-studio/unity/profile",
		Path:      "assets/profile/game-studio/unity/profile.md.tmpl",
		Engine:    EngineUnity,
		Layer:     LayerDomainPack,
		Profile:   "Game-Studio",
		Supported: false,
	}
	items["game-studio/ue5/profile"] = Template{
		ID:        "game-studio/ue5/profile",
		Path:      "assets/profile/game-studio/ue5/profile.md.tmpl",
		Engine:    EngineUE5,
		Layer:     LayerDomainPack,
		Profile:   "Game-Studio",
		Supported: false,
	}

	packs := map[Engine]PackMetadata{}
	packs[EngineGodot] = PackMetadata{
		ID:                "game-studio/godot",
		Engine:            EngineGodot,
		Version:           "0.1.0",
		Status:            "active",
		ProfileTemplateID: "game-studio/godot/profile",
		SummaryTemplateID: "game-studio/godot/profile-summary",
		PatternTemplateID: "game-studio/godot/pattern-hybrid-map",
		ConfigTemplateID:  "game-studio/godot/pack-config",
		LayerOrder: []CCGSLayer{
			LayerOrchestrator,
			LayerWorkflow,
			LayerDomainPack,
			LayerSDDHandoff,
		},
		SkillAgentMap: []SkillAgentBinding{
			{Skill: "art-bible", Agent: "visual-workflow-agent", Role: "art-direction-guidance"},
			{Skill: "asset-spec", Agent: "visual-workflow-agent", Role: "asset-specification-guidance"},
			{Skill: "asset-audit", Agent: "visual-workflow-agent", Role: "read-only-readiness-audit"},
			{Skill: "godot-specialist", Agent: "godot-pack-agent", Role: "engine-domain"},
			{Skill: "sdd-propose", Agent: "sdd-propose-agent", Role: "spec-planning"},
			{Skill: "sdd-apply", Agent: "sdd-apply-agent", Role: "implementation"},
			{Skill: "sdd-verify", Agent: "sdd-verify-agent", Role: "validation"},
		},
		Supported: true,
	}
	packs[EngineUnity] = PackMetadata{
		ID:                "game-studio/unity",
		Engine:            EngineUnity,
		Version:           "0.0.0",
		Status:            "placeholder",
		ProfileTemplateID: "game-studio/unity/profile",
		SummaryTemplateID: "",
		PatternTemplateID: "",
		ConfigTemplateID:  "",
		LayerOrder: []CCGSLayer{
			LayerOrchestrator,
			LayerWorkflow,
			LayerDomainPack,
			LayerSDDHandoff,
		},
		SkillAgentMap: []SkillAgentBinding{},
		Supported:     false,
	}
	packs[EngineUE5] = PackMetadata{
		ID:                "game-studio/ue5",
		Engine:            EngineUE5,
		Version:           "0.0.0",
		Status:            "placeholder",
		ProfileTemplateID: "game-studio/ue5/profile",
		SummaryTemplateID: "",
		PatternTemplateID: "",
		ConfigTemplateID:  "",
		LayerOrder: []CCGSLayer{
			LayerOrchestrator,
			LayerWorkflow,
			LayerDomainPack,
			LayerSDDHandoff,
		},
		SkillAgentMap: []SkillAgentBinding{},
		Supported:     false,
	}

	return &Registry{items: items, packs: packs}
}

func (r *Registry) Count() int {
	return len(r.items)
}

func (r *Registry) Get(id string) (Template, bool) {
	t, ok := r.items[id]
	return t, ok
}

func (r *Registry) GetPack(engine Engine) (PackMetadata, bool) {
	p, ok := r.packs[engine]
	return p, ok
}

func (r *Registry) ValidatePackLoadable(engine Engine) error {
	pack, ok := r.GetPack(engine)
	if !ok {
		return fmt.Errorf("pack metadata not found for engine %q", engine)
	}

	if !pack.Supported {
		return fmt.Errorf("engine %q is placeholder only", engine)
	}

	if pack.Engine != engine {
		return fmt.Errorf("pack engine mismatch: metadata=%q request=%q", pack.Engine, engine)
	}

	if pack.ID == "" || pack.Version == "" || pack.Status == "" {
		return fmt.Errorf("pack metadata incomplete for engine %q", engine)
	}
	if pack.Status != "active" {
		return fmt.Errorf("supported pack %q must be active, got %q", engine, pack.Status)
	}

	if pack.ProfileTemplateID == "" || pack.SummaryTemplateID == "" || pack.PatternTemplateID == "" || pack.ConfigTemplateID == "" {
		return fmt.Errorf("pack template mapping incomplete for engine %q", engine)
	}

	if len(pack.LayerOrder) == 0 {
		return fmt.Errorf("pack layer order missing for engine %q", engine)
	}

	if engine == EngineGodot && len(pack.SkillAgentMap) == 0 {
		return fmt.Errorf("godot skill-agent mapping missing")
	}

	requiredTemplates := []string{pack.ProfileTemplateID, pack.SummaryTemplateID, pack.PatternTemplateID, pack.ConfigTemplateID}
	for _, templateID := range requiredTemplates {
		tpl, found := r.Get(templateID)
		if !found {
			return fmt.Errorf("required template %q not registered", templateID)
		}
		if !tpl.Supported {
			return fmt.Errorf("required template %q not supported", templateID)
		}
		if tpl.Engine != engine {
			return fmt.Errorf("template %q engine mismatch: expected %q got %q", templateID, engine, tpl.Engine)
		}
	}

	if hasDuplicateSkills(pack.SkillAgentMap) {
		return fmt.Errorf("duplicate skill entries in pack skill-agent map")
	}

	for _, binding := range pack.SkillAgentMap {
		if binding.Skill == "" || binding.Agent == "" || binding.Role == "" {
			return fmt.Errorf("invalid skill-agent binding: empty field")
		}
	}

	return nil
}

func hasDuplicateSkills(items []SkillAgentBinding) bool {
	seen := map[string]struct{}{}
	for _, item := range items {
		if _, ok := seen[item.Skill]; ok {
			return true
		}
		seen[item.Skill] = struct{}{}
	}
	return false
}
