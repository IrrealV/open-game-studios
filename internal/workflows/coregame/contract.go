package coregame

import "fmt"

const Version = "core-game-workflow/v1"

type PhaseID string

const (
	PhaseGameConcept      PhaseID = "game-concept"
	PhaseGamePillars      PhaseID = "game-pillars"
	PhaseCoreLoop         PhaseID = "core-loop"
	PhasePlayerFantasy    PhaseID = "player-fantasy"
	PhaseMechanicsBrief   PhaseID = "mechanics-brief"
	PhaseNarrativeBrief   PhaseID = "narrative-brief"
	PhaseToneAndMood      PhaseID = "tone-and-mood"
	PhaseStoryConstraints PhaseID = "story-constraints"
	PhaseGDDSlice         PhaseID = "gdd-slice"
	PhaseChangeBrief      PhaseID = "change-brief"
	PhaseRepairBrief      PhaseID = "repair-brief"
)

type NarrativeMode string

const (
	NarrativeExplicitStory        NarrativeMode = "explicit_story"
	NarrativeEnvironmentalStory   NarrativeMode = "environmental_story"
	NarrativeEmergentStory        NarrativeMode = "emergent_story"
	NarrativeMinimalContext       NarrativeMode = "minimal_context"
	NarrativeNoneOrMechanicsFirst NarrativeMode = "none_or_mechanics_first"
)

type RepairClassification string

const (
	ClassificationDesignChange                  RepairClassification = "design_change"
	ClassificationImplementationBug             RepairClassification = "implementation_bug"
	ClassificationTuningBalancingIssue          RepairClassification = "tuning_balancing_issue"
	ClassificationMovementControlsIssue         RepairClassification = "movement_controls_issue"
	ClassificationAnimationPoseIssue            RepairClassification = "animation_pose_issue"
	ClassificationAssetImportIssue              RepairClassification = "asset_import_issue"
	ClassificationTextureVisualIssue            RepairClassification = "texture_visual_issue"
	ClassificationProgrammingLogicBug           RepairClassification = "programming_logic_bug"
	ClassificationNarrativeContentInconsistency RepairClassification = "narrative_content_inconsistency"
	ClassificationMixedUnknown                  RepairClassification = "mixed_unknown"
)

type Contract struct {
	Version               string                 `json:"version"`
	Boundary              string                 `json:"boundary"`
	SupportedModes        []string               `json:"supported_modes"`
	ModeContracts         []ModeContract         `json:"mode_contracts"`
	PhaseIDs              []PhaseID              `json:"phase_ids"`
	NarrativeModes        []NarrativeMode        `json:"narrative_modes"`
	RepairClassifications []RepairClassification `json:"repair_classifications"`
	ApprovalPolicy        ApprovalPolicy         `json:"approval_policy"`
	BriefContracts        []BriefContract        `json:"brief_contracts"`
	RepairHandoffFlow     []string               `json:"repair_handoff_flow"`
	DownstreamReferences  []DownstreamReference  `json:"downstream_references"`
	NoExecutionClaims     map[string]bool        `json:"no_execution_claims"`
}

type ModeContract struct {
	ID                  string    `json:"id"`
	Intent              string    `json:"intent"`
	Sequence            []string  `json:"sequence"`
	RelevantArtifacts   []PhaseID `json:"relevant_artifacts"`
	ApprovalRequirement string    `json:"approval_requirement"`
	DownstreamBehavior  string    `json:"downstream_behavior"`
}

type ApprovalPolicy struct {
	DefaultStatus    string                    `json:"default_status"`
	ApprovalState    string                    `json:"approval_state"`
	AutoApproved     bool                      `json:"auto_approved"`
	DecisionBoundary string                    `json:"decision_boundary"`
	ArtifactDefaults []ArtifactApprovalDefault `json:"artifact_defaults"`
}

type ArtifactApprovalDefault struct {
	ArtifactID    PhaseID `json:"artifact_id"`
	Status        string  `json:"status"`
	ApprovalState string  `json:"approval_state"`
	AutoApproved  bool    `json:"auto_approved"`
}

type DownstreamReference struct {
	Target              string   `json:"target"`
	Consumes            []string `json:"consumes"`
	Verifies            []string `json:"verifies,omitempty"`
	ApprovalRequirement string   `json:"approval_requirement"`
}

type BriefContract struct {
	ID      PhaseID  `json:"id"`
	Purpose string   `json:"purpose"`
	Fields  []string `json:"fields"`
}

type MarkdownTemplate struct {
	ID              PhaseID                 `json:"id"`
	Filename        string                  `json:"filename"`
	Purpose         string                  `json:"purpose"`
	Sections        []string                `json:"sections"`
	ApprovalDefault ArtifactApprovalDefault `json:"approval_default"`
}

func DefaultContract() Contract {
	briefFields := []string{
		"observed behavior or requested change",
		"expected behavior / intended outcome according to approved artifacts",
		"classification",
		"relevant artifact references",
		"affected systems/files if known",
		"proposed fix options",
		"risks",
		"human decision / approval state",
		"acceptance criteria",
		"verification plan",
		"downstream handoff target",
	}

	return Contract{
		Version:  Version,
		Boundary: "Pi-only studio planning contract; defines phase IDs, artifact metadata, brief structure, and handoff markers without executing generation, debugging, playtesting, image, audio, or engine mutation workflows",
		SupportedModes: []string{
			"zero-to-one creation",
			"direct phase invocation",
			"repair/change handoff",
		},
		ModeContracts: []ModeContract{
			{
				ID:     "zero-to-one creation",
				Intent: "guide a new or existing game idea into an approved game-intent slice before downstream production planning",
				Sequence: []string{
					"idea/existing game",
					"concept interview",
					"game concept",
					"game pillars",
					"core loop",
					"player fantasy",
					"mechanics scope",
					"narrative mode",
					"narrative brief/minimal narrative contract",
					"tone and mood",
					"story constraints",
					"gdd slice",
					"human approval gate",
					"downstream",
				},
				RelevantArtifacts:   []PhaseID{PhaseGameConcept, PhaseGamePillars, PhaseCoreLoop, PhasePlayerFantasy, PhaseMechanicsBrief, PhaseNarrativeBrief, PhaseToneAndMood, PhaseStoryConstraints, PhaseGDDSlice},
				ApprovalRequirement: "human approval required before downstream handoff; no creative or design decision is auto-approved",
				DownstreamBehavior:  "approved GDD slice and referenced intent become metadata inputs for downstream workflows",
			},
			{
				ID:                  "direct phase invocation",
				Intent:              "work on one concrete phase without restarting the complete zero-to-one flow",
				Sequence:            []string{"select phase", "load relevant artifacts", "draft phase output", "human approval gate", "update downstream references"},
				RelevantArtifacts:   []PhaseID{PhaseGameConcept, PhaseGamePillars, PhaseCoreLoop, PhasePlayerFantasy, PhaseMechanicsBrief, PhaseNarrativeBrief, PhaseToneAndMood, PhaseStoryConstraints, PhaseGDDSlice},
				ApprovalRequirement: "human approval required for the invoked phase before it changes downstream references",
				DownstreamBehavior:  "only approved phase output may be referenced downstream; the wider flow remains intact",
			},
			{
				ID:                  "repair/change handoff",
				Intent:              "preserve triage for reported issues or requested changes before implementation handoff",
				Sequence:            []string{"reported issue", "triage", "classification", "relevant artifacts", "repair/change brief", "human approval", "downstream handoff", "verification"},
				RelevantArtifacts:   []PhaseID{PhaseChangeBrief, PhaseRepairBrief},
				ApprovalRequirement: "human approval required before repair or change work is handed downstream",
				DownstreamBehavior:  "approved brief, acceptance criteria, and verification plan become downstream handoff metadata",
			},
		},
		PhaseIDs: []PhaseID{
			PhaseGameConcept,
			PhaseGamePillars,
			PhaseCoreLoop,
			PhasePlayerFantasy,
			PhaseMechanicsBrief,
			PhaseNarrativeBrief,
			PhaseToneAndMood,
			PhaseStoryConstraints,
			PhaseGDDSlice,
			PhaseChangeBrief,
			PhaseRepairBrief,
		},
		NarrativeModes: []NarrativeMode{
			NarrativeExplicitStory,
			NarrativeEnvironmentalStory,
			NarrativeEmergentStory,
			NarrativeMinimalContext,
			NarrativeNoneOrMechanicsFirst,
		},
		RepairClassifications: []RepairClassification{
			ClassificationDesignChange,
			ClassificationImplementationBug,
			ClassificationTuningBalancingIssue,
			ClassificationMovementControlsIssue,
			ClassificationAnimationPoseIssue,
			ClassificationAssetImportIssue,
			ClassificationTextureVisualIssue,
			ClassificationProgrammingLogicBug,
			ClassificationNarrativeContentInconsistency,
			ClassificationMixedUnknown,
		},
		ApprovalPolicy: ApprovalPolicy{
			DefaultStatus:    "draft",
			ApprovalState:    "pending_human_approval",
			AutoApproved:     false,
			DecisionBoundary: "creative decisions, design decisions, narrative decisions, approval gates, and downstream mutations require explicit human approval and are never auto-approved",
			ArtifactDefaults: []ArtifactApprovalDefault{
				{ArtifactID: PhaseGameConcept, Status: "draft", ApprovalState: "pending_human_approval", AutoApproved: false},
				{ArtifactID: PhaseGamePillars, Status: "draft", ApprovalState: "pending_human_approval", AutoApproved: false},
				{ArtifactID: PhaseCoreLoop, Status: "draft", ApprovalState: "pending_human_approval", AutoApproved: false},
				{ArtifactID: PhasePlayerFantasy, Status: "draft", ApprovalState: "pending_human_approval", AutoApproved: false},
				{ArtifactID: PhaseMechanicsBrief, Status: "draft", ApprovalState: "pending_human_approval", AutoApproved: false},
				{ArtifactID: PhaseNarrativeBrief, Status: "draft", ApprovalState: "pending_human_approval", AutoApproved: false},
				{ArtifactID: PhaseToneAndMood, Status: "draft", ApprovalState: "pending_human_approval", AutoApproved: false},
				{ArtifactID: PhaseStoryConstraints, Status: "draft", ApprovalState: "pending_human_approval", AutoApproved: false},
				{ArtifactID: PhaseGDDSlice, Status: "draft", ApprovalState: "pending_human_approval", AutoApproved: false},
				{ArtifactID: PhaseChangeBrief, Status: "draft", ApprovalState: "pending_human_approval", AutoApproved: false},
				{ArtifactID: PhaseRepairBrief, Status: "draft", ApprovalState: "pending_human_approval", AutoApproved: false},
			},
		},
		BriefContracts: []BriefContract{
			{ID: PhaseChangeBrief, Purpose: "intentional changes to approved game design or narrative artifacts", Fields: briefFields},
			{ID: PhaseRepairBrief, Purpose: "bugs, problems, regressions, or mismatches against approved artifacts", Fields: briefFields},
		},
		RepairHandoffFlow: []string{"reported issue", "triage", "classification", "relevant artifacts", "repair/change brief", "human approval", "downstream handoff", "verification"},
		DownstreamReferences: []DownstreamReference{
			{Target: "Art Bible", Consumes: []string{"approved intent", "tone and mood", "gdd slice"}, ApprovalRequirement: "requires approved game intent before visual direction is treated as authoritative"},
			{Target: "Visual Identity Anchor", Consumes: []string{"approved intent", "tone and mood", "gdd slice"}, ApprovalRequirement: "requires approved game intent before visual identity constraints are anchored"},
			{Target: "Asset Spec", Consumes: []string{"approved intent", "gdd slice", "mechanics brief", "Art Bible"}, ApprovalRequirement: "requires approved game intent, approved GDD/mechanics, and approved or explicitly draft Art Bible references"},
			{Target: "Asset Manifest", Consumes: []string{"Asset Spec", "provenance", "status"}, ApprovalRequirement: "records approval/status metadata; does not imply generated assets are approved"},
			{Target: "Godot handoff", Consumes: []string{"gdd slice", "mechanics brief", "change brief", "repair brief"}, ApprovalRequirement: "requires approved gameplay intent or approved change/repair brief before engine handoff"},
			{Target: "SDD handoff", Consumes: []string{"change brief", "repair brief", "acceptance criteria"}, ApprovalRequirement: "requires human-approved brief and acceptance criteria before implementation planning"},
			{Target: "QA/review checklist", Consumes: []string{"approved intent", "acceptance criteria", "verification plan"}, Verifies: []string{"approved intent", "acceptance criteria", "verification plan"}, ApprovalRequirement: "verifies against approved artifacts; it does not approve them automatically"},
			{Target: "future image/audio workflows", Consumes: []string{"Art Bible", "Asset Spec", "GDD constraints"}, ApprovalRequirement: "metadata-only placeholder; future workflows must consume constraints without being implemented here"},
			{Target: "future model routing by phase/capability", Consumes: []string{"phase IDs", "capabilities"}, ApprovalRequirement: "metadata-only placeholder for future routing; no model command execution is promised"},
		},
		NoExecutionClaims: map[string]bool{
			"debugging_executed":              false,
			"image_generated":                 false,
			"audio_generated":                 false,
			"playtesting_run":                 false,
			"engine_files_mutated":            false,
			"collaboration_workflow_executed": false,
			"claude_native_mechanics_enabled": false,
		},
	}
}

func DefaultMarkdownTemplates() []MarkdownTemplate {
	approval := ArtifactApprovalDefault{Status: "draft", ApprovalState: "pending_human_approval", AutoApproved: false}
	return []MarkdownTemplate{
		{
			ID:              PhaseGDDSlice,
			Filename:        "gdd-slice.md",
			Purpose:         "human-readable game design slice linking concept, pillars, loop, fantasy, mechanics, narrative, tone, and constraints",
			Sections:        []string{"metadata", "game concept", "game pillars", "core loop", "player fantasy", "mechanics brief", "narrative brief", "tone and mood", "story constraints", "downstream references", "approval state", "auto_approved: false"},
			ApprovalDefault: artifactApprovalDefault(approval, PhaseGDDSlice),
		},
		{
			ID:              PhaseChangeBrief,
			Filename:        "change-brief.md",
			Purpose:         "human-readable approval brief for intentional design or narrative changes",
			Sections:        []string{"metadata", "requested change", "intended outcome", "classification", "relevant artifacts", "affected systems/files", "proposed options", "risks", "human decision", "approval state", "auto_approved: false", "acceptance criteria", "verification plan", "downstream handoff target"},
			ApprovalDefault: artifactApprovalDefault(approval, PhaseChangeBrief),
		},
		{
			ID:              PhaseRepairBrief,
			Filename:        "repair-brief.md",
			Purpose:         "human-readable approval brief for bugs, regressions, and artifact mismatches",
			Sections:        []string{"metadata", "observed behavior", "expected behavior", "classification", "relevant artifacts", "affected systems/files", "proposed fix options", "risks", "human decision", "approval state", "auto_approved: false", "acceptance criteria", "verification plan", "downstream handoff target"},
			ApprovalDefault: artifactApprovalDefault(approval, PhaseRepairBrief),
		},
	}
}

func artifactApprovalDefault(base ArtifactApprovalDefault, id PhaseID) ArtifactApprovalDefault {
	base.ArtifactID = id
	return base
}

func Validate(contract Contract) error {
	if contract.Version != Version {
		return fmt.Errorf("unexpected core game workflow version %q", contract.Version)
	}
	if !containsStrings(contract.SupportedModes, []string{"zero-to-one creation", "direct phase invocation", "repair/change handoff"}) {
		return fmt.Errorf("core game workflow supported modes incomplete")
	}
	if !validModeContracts(contract.ModeContracts) {
		return fmt.Errorf("core game workflow mode contracts incomplete")
	}
	if !containsPhases(contract.PhaseIDs, []PhaseID{PhaseGameConcept, PhaseGamePillars, PhaseCoreLoop, PhasePlayerFantasy, PhaseMechanicsBrief, PhaseNarrativeBrief, PhaseToneAndMood, PhaseStoryConstraints, PhaseGDDSlice, PhaseChangeBrief, PhaseRepairBrief}) {
		return fmt.Errorf("core game workflow phase ids incomplete")
	}
	if !containsNarrativeModes(contract.NarrativeModes, []NarrativeMode{NarrativeExplicitStory, NarrativeEnvironmentalStory, NarrativeEmergentStory, NarrativeMinimalContext, NarrativeNoneOrMechanicsFirst}) {
		return fmt.Errorf("core game workflow narrative modes incomplete")
	}
	if !containsRepairClassifications(contract.RepairClassifications, []RepairClassification{ClassificationDesignChange, ClassificationImplementationBug, ClassificationTuningBalancingIssue, ClassificationMovementControlsIssue, ClassificationAnimationPoseIssue, ClassificationAssetImportIssue, ClassificationTextureVisualIssue, ClassificationProgrammingLogicBug, ClassificationNarrativeContentInconsistency, ClassificationMixedUnknown}) {
		return fmt.Errorf("core game workflow repair classifications incomplete")
	}
	if !validApprovalPolicy(contract.ApprovalPolicy) {
		return fmt.Errorf("core game workflow approval policy incomplete")
	}
	if !validBriefContracts(contract.BriefContracts) {
		return fmt.Errorf("core game workflow brief contracts incomplete")
	}
	if !sameStrings(contract.RepairHandoffFlow, []string{"reported issue", "triage", "classification", "relevant artifacts", "repair/change brief", "human approval", "downstream handoff", "verification"}) {
		return fmt.Errorf("core game workflow repair handoff flow incomplete")
	}
	if !validDownstreamReferences(contract.DownstreamReferences) {
		return fmt.Errorf("core game workflow downstream references incomplete")
	}
	for _, claim := range []string{"debugging_executed", "image_generated", "audio_generated", "playtesting_run", "engine_files_mutated", "collaboration_workflow_executed", "claude_native_mechanics_enabled"} {
		if contract.NoExecutionClaims[claim] {
			return fmt.Errorf("core game workflow execution claim %q must be false", claim)
		}
		if _, ok := contract.NoExecutionClaims[claim]; !ok {
			return fmt.Errorf("core game workflow execution claim %q missing", claim)
		}
	}
	return nil
}

func validModeContracts(items []ModeContract) bool {
	required := map[string][]string{
		"zero-to-one creation":    {"idea/existing game", "concept interview", "game concept", "game pillars", "core loop", "player fantasy", "mechanics scope", "narrative mode", "narrative brief/minimal narrative contract", "tone and mood", "story constraints", "gdd slice", "human approval gate", "downstream"},
		"direct phase invocation": {"select phase", "load relevant artifacts", "draft phase output", "human approval gate", "update downstream references"},
		"repair/change handoff":   {"reported issue", "triage", "classification", "relevant artifacts", "repair/change brief", "human approval", "downstream handoff", "verification"},
	}
	seen := map[string]ModeContract{}
	for _, item := range items {
		seen[item.ID] = item
	}
	for id, sequence := range required {
		item, ok := seen[id]
		if !ok || !sameStrings(item.Sequence, sequence) || len(item.RelevantArtifacts) == 0 || item.ApprovalRequirement == "" || item.DownstreamBehavior == "" {
			return false
		}
	}
	return true
}

func validApprovalPolicy(policy ApprovalPolicy) bool {
	if policy.DefaultStatus != "draft" || policy.ApprovalState != "pending_human_approval" || policy.AutoApproved || policy.DecisionBoundary == "" {
		return false
	}
	seen := map[PhaseID]ArtifactApprovalDefault{}
	required := map[PhaseID]struct{}{
		PhaseGameConcept:      {},
		PhaseGamePillars:      {},
		PhaseCoreLoop:         {},
		PhasePlayerFantasy:    {},
		PhaseMechanicsBrief:   {},
		PhaseNarrativeBrief:   {},
		PhaseToneAndMood:      {},
		PhaseStoryConstraints: {},
		PhaseGDDSlice:         {},
		PhaseChangeBrief:      {},
		PhaseRepairBrief:      {},
	}
	for _, item := range policy.ArtifactDefaults {
		if item.Status != "draft" || item.ApprovalState != "pending_human_approval" || item.AutoApproved {
			return false
		}
		if _, ok := required[item.ArtifactID]; !ok {
			return false
		}
		if _, ok := seen[item.ArtifactID]; ok {
			return false
		}
		seen[item.ArtifactID] = item
	}
	for phase := range required {
		item, ok := seen[phase]
		if !ok || item.Status != "draft" || item.ApprovalState != "pending_human_approval" || item.AutoApproved {
			return false
		}
	}
	return true
}

func validBriefContracts(items []BriefContract) bool {
	requiredFields := []string{"human decision / approval state", "acceptance criteria", "verification plan", "downstream handoff target"}
	seen := map[PhaseID]BriefContract{}
	for _, item := range items {
		seen[item.ID] = item
	}
	for _, id := range []PhaseID{PhaseChangeBrief, PhaseRepairBrief} {
		item, ok := seen[id]
		if !ok || item.Purpose == "" || !containsStrings(item.Fields, requiredFields) {
			return false
		}
	}
	return true
}

func validDownstreamReferences(items []DownstreamReference) bool {
	required := map[string][]string{
		"Art Bible":                    {"approved intent", "tone and mood", "gdd slice"},
		"Visual Identity Anchor":       {"approved intent", "tone and mood", "gdd slice"},
		"Asset Spec":                   {"approved intent", "gdd slice", "mechanics brief", "Art Bible"},
		"Asset Manifest":               {"Asset Spec", "provenance", "status"},
		"Godot handoff":                {"gdd slice", "mechanics brief", "change brief", "repair brief"},
		"SDD handoff":                  {"change brief", "repair brief", "acceptance criteria"},
		"QA/review checklist":          {"approved intent", "acceptance criteria", "verification plan"},
		"future image/audio workflows": {"Art Bible", "Asset Spec", "GDD constraints"},
		"future model routing by phase/capability": {"phase IDs", "capabilities"},
	}
	seen := map[string]DownstreamReference{}
	for _, item := range items {
		seen[item.Target] = item
	}
	for target, consumes := range required {
		item, ok := seen[target]
		if !ok || !containsStrings(item.Consumes, consumes) || item.ApprovalRequirement == "" {
			return false
		}
		if target == "QA/review checklist" && !containsStrings(item.Verifies, []string{"approved intent", "acceptance criteria", "verification plan"}) {
			return false
		}
	}
	return true
}

func containsPhases(items []PhaseID, required []PhaseID) bool {
	seen := map[PhaseID]struct{}{}
	for _, item := range items {
		seen[item] = struct{}{}
	}
	for _, item := range required {
		if _, ok := seen[item]; !ok {
			return false
		}
	}
	return true
}

func containsNarrativeModes(items []NarrativeMode, required []NarrativeMode) bool {
	seen := map[NarrativeMode]struct{}{}
	for _, item := range items {
		seen[item] = struct{}{}
	}
	for _, item := range required {
		if _, ok := seen[item]; !ok {
			return false
		}
	}
	return true
}

func containsRepairClassifications(items []RepairClassification, required []RepairClassification) bool {
	seen := map[RepairClassification]struct{}{}
	for _, item := range items {
		seen[item] = struct{}{}
	}
	for _, item := range required {
		if _, ok := seen[item]; !ok {
			return false
		}
	}
	return true
}

func containsStrings(items []string, required []string) bool {
	seen := map[string]struct{}{}
	for _, item := range items {
		seen[item] = struct{}{}
	}
	for _, item := range required {
		if _, ok := seen[item]; !ok {
			return false
		}
	}
	return true
}

func sameStrings(items []string, required []string) bool {
	if len(items) != len(required) {
		return false
	}
	for idx, item := range items {
		if item != required[idx] {
			return false
		}
	}
	return true
}
