package coregame

import (
	"strings"
	"testing"
)

func TestDefaultContractBoundaryIsPiStudioNeutral(t *testing.T) {
	boundary := DefaultContract().Boundary
	if strings.Contains(boundary, "OpenCode") {
		t.Fatalf("core game workflow boundary must not claim an OpenCode-native runtime: %q", boundary)
	}
	if !strings.Contains(boundary, "Pi-only") {
		t.Fatalf("core game workflow boundary must describe the Pi-only studio: %q", boundary)
	}
}

func TestDefaultContractDefinesCoreGameWorkflowMarkers(t *testing.T) {
	contract := DefaultContract()

	if err := Validate(contract); err != nil {
		t.Fatalf("default contract should validate: %v", err)
	}

	for _, phase := range []PhaseID{PhaseGameConcept, PhaseGamePillars, PhaseCoreLoop, PhasePlayerFantasy, PhaseMechanicsBrief, PhaseNarrativeBrief, PhaseToneAndMood, PhaseStoryConstraints, PhaseGDDSlice, PhaseChangeBrief, PhaseRepairBrief} {
		if !containsPhase(contract.PhaseIDs, phase) {
			t.Fatalf("phase %q missing from contract: %#v", phase, contract.PhaseIDs)
		}
	}

	for _, mode := range []NarrativeMode{NarrativeExplicitStory, NarrativeEnvironmentalStory, NarrativeEmergentStory, NarrativeMinimalContext, NarrativeNoneOrMechanicsFirst} {
		if !containsNarrativeMode(contract.NarrativeModes, mode) {
			t.Fatalf("narrative mode %q missing from contract: %#v", mode, contract.NarrativeModes)
		}
	}

	for _, claim := range []string{"debugging_executed", "image_generated", "audio_generated", "playtesting_run", "engine_files_mutated", "collaboration_workflow_executed", "claude_native_mechanics_enabled"} {
		value, ok := contract.NoExecutionClaims[claim]
		if !ok || value {
			t.Fatalf("no-execution claim %q missing or true: %#v", claim, contract.NoExecutionClaims)
		}
	}

	wantFlow := []string{"reported issue", "triage", "classification", "relevant artifacts", "repair/change brief", "human approval", "downstream handoff", "verification"}
	for idx, step := range wantFlow {
		if contract.RepairHandoffFlow[idx] != step {
			t.Fatalf("handoff step %d=%q want %q", idx, contract.RepairHandoffFlow[idx], step)
		}
	}
}

func TestDefaultContractRequiresHumanApprovalAndNoAutoApproval(t *testing.T) {
	contract := DefaultContract()

	if contract.ApprovalPolicy.DefaultStatus != "draft" {
		t.Fatalf("default status=%q want draft", contract.ApprovalPolicy.DefaultStatus)
	}
	if contract.ApprovalPolicy.ApprovalState != "pending_human_approval" {
		t.Fatalf("approval state=%q want pending_human_approval", contract.ApprovalPolicy.ApprovalState)
	}
	if contract.ApprovalPolicy.AutoApproved {
		t.Fatal("approval policy must not auto-approve artifacts")
	}

	for _, phase := range contract.PhaseIDs {
		approval, ok := approvalDefaultFor(contract.ApprovalPolicy.ArtifactDefaults, phase)
		if !ok {
			t.Fatalf("approval default missing for %q", phase)
		}
		if approval.Status != "draft" || approval.ApprovalState != "pending_human_approval" || approval.AutoApproved {
			t.Fatalf("invalid approval default for %q: %#v", phase, approval)
		}
	}

	invalid := contract
	invalid.ApprovalPolicy.ArtifactDefaults[0].AutoApproved = true
	if err := Validate(invalid); err == nil {
		t.Fatal("Validate should reject auto-approved core game artifacts")
	}
}

func TestDefaultContractDefinesModeContracts(t *testing.T) {
	contract := DefaultContract()

	zeroToOne, ok := modeContractFor(contract.ModeContracts, "zero-to-one creation")
	if !ok {
		t.Fatal("zero-to-one mode contract missing")
	}
	for _, step := range []string{"idea/existing game", "concept interview", "game concept", "game pillars", "core loop", "player fantasy", "mechanics scope", "narrative mode", "narrative brief/minimal narrative contract", "tone and mood", "story constraints", "gdd slice", "human approval gate", "downstream"} {
		if !containsString(zeroToOne.Sequence, step) {
			t.Fatalf("zero-to-one sequence missing %q: %#v", step, zeroToOne.Sequence)
		}
	}

	direct, ok := modeContractFor(contract.ModeContracts, "direct phase invocation")
	if !ok || direct.Intent == "" || !containsString(direct.Sequence, "load relevant artifacts") || !containsString(direct.Sequence, "human approval gate") {
		t.Fatalf("direct phase invocation contract incomplete: %#v", direct)
	}
	repair, ok := modeContractFor(contract.ModeContracts, "repair/change handoff")
	if !ok || !containsString(repair.Sequence, "triage") || !containsString(repair.Sequence, "verification") {
		t.Fatalf("repair/change handoff contract incomplete: %#v", repair)
	}
}

func TestDefaultContractDefinesStructuredDownstreamRelationships(t *testing.T) {
	contract := DefaultContract()

	tests := []struct {
		name     string
		consumes []string
		verifies []string
	}{
		{name: "Art Bible", consumes: []string{"approved intent", "tone and mood", "gdd slice"}},
		{name: "Visual Identity Anchor", consumes: []string{"approved intent", "tone and mood", "gdd slice"}},
		{name: "Asset Spec", consumes: []string{"approved intent", "gdd slice", "mechanics brief", "Art Bible"}},
		{name: "Asset Manifest", consumes: []string{"Asset Spec", "provenance", "status"}},
		{name: "Godot handoff", consumes: []string{"gdd slice", "mechanics brief", "change brief", "repair brief"}},
		{name: "SDD handoff", consumes: []string{"change brief", "repair brief", "acceptance criteria"}},
		{name: "QA/review checklist", consumes: []string{"approved intent", "acceptance criteria", "verification plan"}, verifies: []string{"approved intent", "acceptance criteria", "verification plan"}},
		{name: "future image/audio workflows", consumes: []string{"Art Bible", "Asset Spec", "GDD constraints"}},
		{name: "future model routing by phase/capability", consumes: []string{"phase IDs", "capabilities"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reference, ok := downstreamReferenceFor(contract.DownstreamReferences, tt.name)
			if !ok {
				t.Fatalf("downstream reference %q missing", tt.name)
			}
			for _, consume := range tt.consumes {
				if !containsString(reference.Consumes, consume) {
					t.Fatalf("%q consumes missing %q: %#v", tt.name, consume, reference.Consumes)
				}
			}
			for _, verify := range tt.verifies {
				if !containsString(reference.Verifies, verify) {
					t.Fatalf("%q verifies missing %q: %#v", tt.name, verify, reference.Verifies)
				}
			}
			if reference.ApprovalRequirement == "" {
				t.Fatalf("%q approval requirement missing", tt.name)
			}
		})
	}
}

func TestValidateRejectsMissingStructuredContracts(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Contract)
	}{
		{name: "mode contracts", mutate: func(contract *Contract) { contract.ModeContracts = nil }},
		{name: "approval policy", mutate: func(contract *Contract) { contract.ApprovalPolicy = ApprovalPolicy{} }},
		{name: "downstream relationships", mutate: func(contract *Contract) { contract.DownstreamReferences = nil }},
		{name: "no execution claims", mutate: func(contract *Contract) { contract.NoExecutionClaims["debugging_executed"] = true }},
		{name: "missing collaboration claim", mutate: func(contract *Contract) { delete(contract.NoExecutionClaims, "collaboration_workflow_executed") }},
		{name: "claude native claim enabled", mutate: func(contract *Contract) { contract.NoExecutionClaims["claude_native_mechanics_enabled"] = true }},
		{name: "unordered zero-to-one sequence", mutate: func(contract *Contract) {
			contract.ModeContracts[0].Sequence[0], contract.ModeContracts[0].Sequence[1] = contract.ModeContracts[0].Sequence[1], contract.ModeContracts[0].Sequence[0]
		}},
		{name: "duplicate artifact default", mutate: func(contract *Contract) {
			contract.ApprovalPolicy.ArtifactDefaults = append(contract.ApprovalPolicy.ArtifactDefaults, contract.ApprovalPolicy.ArtifactDefaults[0])
		}},
		{name: "unknown artifact default", mutate: func(contract *Contract) {
			contract.ApprovalPolicy.ArtifactDefaults = append(contract.ApprovalPolicy.ArtifactDefaults, ArtifactApprovalDefault{ArtifactID: PhaseID("unknown"), Status: "draft", ApprovalState: "pending_human_approval", AutoApproved: false})
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contract := DefaultContract()
			tt.mutate(&contract)
			if err := Validate(contract); err == nil {
				t.Fatalf("Validate should reject missing %s", tt.name)
			}
		})
	}
}

func TestDefaultMarkdownTemplatesCoverHumanBriefArtifacts(t *testing.T) {
	templates := DefaultMarkdownTemplates()

	for _, id := range []PhaseID{PhaseGDDSlice, PhaseChangeBrief, PhaseRepairBrief} {
		template, ok := templateFor(templates, id)
		if !ok {
			t.Fatalf("template %q missing: %#v", id, templates)
		}
		if template.ApprovalDefault.Status != "draft" || template.ApprovalDefault.ApprovalState != "pending_human_approval" || template.ApprovalDefault.AutoApproved {
			t.Fatalf("template %q approval default invalid: %#v", id, template.ApprovalDefault)
		}
	}
}

func containsPhase(items []PhaseID, want PhaseID) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func containsNarrativeMode(items []NarrativeMode, want NarrativeMode) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func templateFor(items []MarkdownTemplate, want PhaseID) (MarkdownTemplate, bool) {
	for _, item := range items {
		if item.ID == want {
			return item, true
		}
	}
	return MarkdownTemplate{}, false
}

func approvalDefaultFor(items []ArtifactApprovalDefault, want PhaseID) (ArtifactApprovalDefault, bool) {
	for _, item := range items {
		if item.ArtifactID == want {
			return item, true
		}
	}
	return ArtifactApprovalDefault{}, false
}

func modeContractFor(items []ModeContract, want string) (ModeContract, bool) {
	for _, item := range items {
		if item.ID == want {
			return item, true
		}
	}
	return ModeContract{}, false
}

func downstreamReferenceFor(items []DownstreamReference, want string) (DownstreamReference, bool) {
	for _, item := range items {
		if item.Target == want {
			return item, true
		}
	}
	return DownstreamReference{}, false
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
