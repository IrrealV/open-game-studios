package commands

import (
	"strings"

	"open-game-studios/internal/piinstall"
)

// wizardTuiChoice is one selectable value on a preview page. Deferred marks a
// value the preview can select but the active generation path cannot yet honor.
type wizardTuiChoice struct {
	Value    string
	Label    string
	Note     string
	Deferred bool
}

// wizardTuiUseModeChoices are the workflow values the preview edits. Values are
// the canonical use-mode identifiers so the flag semantics are preserved.
func wizardTuiUseModeChoices() []wizardTuiChoice {
	return []wizardTuiChoice{
		{Value: "create_from_scratch", Label: "Create from scratch", Note: "new Godot project skeleton"},
		{Value: "existing_game", Label: "Existing game", Note: "adopt an existing Godot workspace"},
		{Value: "design_narrative_only", Label: "Design & narrative only", Note: "no engine prerequisite"},
		{Value: "visual_artifacts_only", Label: "Visual artifacts only", Note: "no engine prerequisite"},
		{Value: "godot_sdd_handoff", Label: "Godot SDD handoff", Note: "spec-driven flow in Godot"},
		{Value: "repair_change_workflow", Label: "Repair / change workflow", Note: "fix an existing Godot project"},
	}
}

// wizardTuiEngineChoices expose the active engine pack plus the deferred packs.
// Deferred packs stay reachable and honestly labeled because they are valid CLI
// selections that only defer active generation.
func wizardTuiEngineChoices() []wizardTuiChoice {
	godot := enginePackMetadata("godot-core")
	unity := enginePackMetadata("unity")
	ue5 := enginePackMetadata("ue5")
	return []wizardTuiChoice{
		{Value: "godot-core", Label: godot.Label, Note: godot.Note},
		{Value: "unity", Label: "Unity engine pack", Note: unity.Note, Deferred: unity.GenerationDeferred},
		{Value: "ue5", Label: "Unreal Engine 5 pack", Note: ue5.Note, Deferred: ue5.GenerationDeferred},
	}
}

// wizardTuiDepthChoices mirror the setup-depth presets and their default
// resource semantics. The preview never edits resources directly.
func wizardTuiDepthChoices() []wizardTuiChoice {
	return []wizardTuiChoice{
		{Value: "minimal", Label: "Minimal", Note: "Godot docs connector and formatter tool"},
		{Value: "recommended", Label: "Recommended", Note: "balanced defaults; keeps Godot docs"},
		{Value: "full", Label: "Full", Note: "adds asset-store connector and linter tool"},
		{Value: "custom", Label: "Custom", Note: "adds asset-store connector; tune later on the existing path"},
	}
}

// wizardTuiSnapshot is the immutable capture of the edited project essentials.
// It is the single source for both the displayed preflight and the injected
// installer call, so a stale selection can never leak into plan preparation.
type wizardTuiSnapshot struct {
	UseMode     string
	EnginePack  string
	SetupDepth  string
	ProfileName string
	Connectors  []string
	Packs       []string
	Tools       []string
}

// newWizardTuiSnapshot normalizes the live model state and derives the resource
// defaults with the same shared helpers the staged prompt flow uses.
func newWizardTuiSnapshot(state wizardState) wizardTuiSnapshot {
	useMode := normalizeUseMode(state.UseMode)
	engine := normalizeEnginePack(state.EnginePack)
	depth := normalizeSetupDepth(state.SetupDepth)
	connectors, packs, tools := defaultsForSetup(engine, depth, useMode)
	return wizardTuiSnapshot{
		UseMode:     useMode,
		EnginePack:  engine,
		SetupDepth:  depth,
		ProfileName: sanitizeProfileName(state.ProfileName),
		Connectors:  connectors,
		Packs:       packs,
		Tools:       tools,
	}
}

// wizardState materializes the snapshot into the shape the shared config and
// Godot-required helpers expect. It never mutates the live model state.
func (s wizardTuiSnapshot) wizardState() wizardState {
	state := wizardState{
		UseMode:     s.UseMode,
		EnginePack:  s.EnginePack,
		SetupDepth:  s.SetupDepth,
		ProfileName: s.ProfileName,
		Connectors:  append([]string{}, s.Connectors...),
		Packs:       append([]string{}, s.Packs...),
		Tools:       append([]string{}, s.Tools...),
	}
	state.StartingPoint = startingPointForUseMode(state.UseMode)
	state.Complexity = complexityForSetupDepth(state.SetupDepth)
	return state
}

// wizardTuiReplayArgs rebuilds a non-interactive, plan-only preview command from
// the FINAL user choices rather than the original argument list, so the printed
// replay cannot silently fall back to stale flags.
func wizardTuiReplayArgs(snapshot wizardTuiSnapshot, outDir, finalArtifactPath string) []string {
	args := []string{
		"--non-interactive",
		"--plan-only",
		"--profile", snapshot.ProfileName,
		"--use-mode", snapshot.UseMode,
		"--engine-pack", snapshot.EnginePack,
		"--setup-depth", snapshot.SetupDepth,
	}
	if trimmed := strings.TrimSpace(outDir); trimmed != "" {
		args = append(args, "--out-dir", trimmed)
	}
	if trimmed := strings.TrimSpace(finalArtifactPath); trimmed != "" {
		args = append(args, "--final-artifact", trimmed)
	}
	return args
}

// wizardTuiPlanGroup is a friendly grouping of plan steps. Every step keeps its
// real fields; grouping only reorders presentation.
type wizardTuiPlanGroup struct {
	Key   string
	Title string
	Note  string
	Steps []piinstall.PlanStep
}

// wizardTuiPlanGroups buckets the real plan steps by action. Empty groups are
// dropped so the review stays compact.
func wizardTuiPlanGroups(plan piinstall.Plan) []wizardTuiPlanGroup {
	groups := []wizardTuiPlanGroup{
		{Key: "install", Title: "Install", Note: "new managed components written to owned destinations"},
		{Key: "conditional", Title: "Conditional", Note: "reuse an exact pinned registration, or install after consent"},
		{Key: "reuse", Title: "Reuse", Note: "already compatible; left untouched"},
		{Key: "blocked", Title: "Blocked", Note: "fail closed until a human resolves them"},
	}
	index := make(map[string]int, len(groups))
	for i := range groups {
		index[groups[i].Key] = i
	}
	for _, step := range plan.Steps {
		if i, ok := index[wizardTuiStepGroupKey(step)]; ok {
			groups[i].Steps = append(groups[i].Steps, step)
		}
	}
	out := make([]wizardTuiPlanGroup, 0, len(groups))
	for _, group := range groups {
		if len(group.Steps) == 0 {
			continue
		}
		out = append(out, group)
	}
	return out
}

func wizardTuiStepGroupKey(step piinstall.PlanStep) string {
	switch step.Action {
	case piinstall.ActionReuse:
		return "reuse"
	case piinstall.ActionEnsure:
		return "conditional"
	case piinstall.ActionBlocked:
		return "blocked"
	default:
		return "install"
	}
}

func wizardTuiBlockedSteps(plan piinstall.Plan) []piinstall.PlanStep {
	out := make([]piinstall.PlanStep, 0)
	for _, step := range plan.Steps {
		if step.Action == piinstall.ActionBlocked {
			out = append(out, step)
		}
	}
	return out
}

func wizardTuiChoiceLabel(choices []wizardTuiChoice, value string) string {
	for _, choice := range choices {
		if choice.Value == value {
			return choice.Label
		}
	}
	return value
}

func wizardTuiIndexOf(choices []wizardTuiChoice, value string) int {
	for i, choice := range choices {
		if choice.Value == value {
			return i
		}
	}
	return 0
}
