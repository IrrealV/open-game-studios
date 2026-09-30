package coregame

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// gddContents supplies every caller-owned gdd-slice template section, so it is
// complete enough for a downstream handoff as well as for construction.
func gddContents() []SectionContent {
	return []SectionContent{
		{Section: "game concept", Body: "Synthetic concept."},
		{Section: "game pillars", Body: "Synthetic pillars."},
		{Section: "core loop", Body: "Synthetic loop."},
		{Section: "player fantasy", Body: "Synthetic fantasy."},
		{Section: "mechanics brief", Body: "Synthetic mechanics."},
		{Section: "narrative brief", Body: "No narrative; mechanics-first synthetic slice."},
		{Section: "tone and mood", Body: "Synthetic tone."},
		{Section: "story constraints", Body: "Synthetic constraints."},
	}
}

// gddMinimalContents meets only the draft minimum, not downstream completeness.
func gddMinimalContents() []SectionContent {
	return []SectionContent{
		{Section: "game concept", Body: "Synthetic concept."},
		{Section: "core loop", Body: "Synthetic loop."},
	}
}

func changeContents() []SectionContent {
	return []SectionContent{
		{Section: "requested change", Body: "Shift the synthetic loop."},
		{Section: "intended outcome", Body: "A clearer synthetic loop."},
		{Section: "affected systems/files", Body: "unknown at this stage."},
		{Section: "proposed options", Body: "Option A only."},
		{Section: "risks", Body: "Synthetic risk."},
		{Section: "acceptance criteria", Body: "The loop is clear."},
		{Section: "verification plan", Body: "Run focused synthetic checks."},
	}
}

func repairContents() []SectionContent {
	return []SectionContent{
		{Section: "observed behavior", Body: "The synthetic loop stalls."},
		{Section: "expected behavior", Body: "The synthetic loop advances."},
		{Section: "affected systems/files", Body: "unknown at this stage."},
		{Section: "proposed fix options", Body: "Option A only."},
		{Section: "risks", Body: "Synthetic risk."},
		{Section: "acceptance criteria", Body: "The loop advances."},
		{Section: "verification plan", Body: "Run focused synthetic checks."},
	}
}

func withBody(contents []SectionContent, section, body string) []SectionContent {
	out := make([]SectionContent, len(contents))
	copy(out, contents)
	for i := range out {
		if out[i].Section == section {
			out[i].Body = body
		}
	}
	return out
}

// syntheticRefFS builds a caller-owned read seam with one synthetic gdd-slice artifact.
func syntheticRefFS(t *testing.T, content string) (fs.FS, ArtifactReference) {
	t.Helper()
	return refFSFor(t, PhaseGDDSlice, "artifacts/gdd-slice.md", content)
}

func refFSFor(t *testing.T, phase PhaseID, locator, content string) (fs.FS, ArtifactReference) {
	t.Helper()
	data := []byte(content)
	return fstest.MapFS{locator: &fstest.MapFile{Data: data}},
		ArtifactReference{Phase: phase, Locator: locator, Revision: sha256Hex(data)}
}

func validChangeRequest(ref ArtifactReference) DraftRequest {
	return DraftRequest{
		Mode:           modeRepairChange,
		Phase:          PhaseChangeBrief,
		Request:        "Adjust the synthetic loop.",
		Classification: ClassificationDesignChange,
		Target:         "Godot handoff",
		Contents:       changeContents(),
		References:     []ArtifactReference{ref},
	}
}

func validGDDRequest() DraftRequest {
	return DraftRequest{
		Mode:     modeCreation,
		Request:  "Draft a synthetic slice.",
		Target:   "Godot handoff",
		Contents: gddContents(),
	}
}

func validDecision(draft Draft) HumanDecision {
	return HumanDecision{
		Reference:     "synthetic review reference",
		State:         DecisionApproved,
		Target:        draft.Target,
		DraftRevision: draft.Revision,
	}
}

func reseal(t *testing.T, draft Draft) Draft {
	t.Helper()
	revision, err := revisionOf(draft)
	if err != nil {
		t.Fatalf("revisionOf: %v", err)
	}
	draft.Revision = revision
	return draft
}

// fencedBlocks extracts the content of every fenced code block. It relies on
// the renderer guaranteeing a closing fence at least as long as the opening one.
func fencedBlocks(markdown string) []string {
	lines := strings.Split(markdown, "\n")
	var blocks []string
	for i := 0; i < len(lines); i++ {
		open, ok := allBacktickLine(lines[i])
		if !ok {
			continue
		}
		end := i + 1
		for ; end < len(lines); end++ {
			if closeLen, ok := allBacktickLine(lines[end]); ok && closeLen >= open {
				break
			}
		}
		blocks = append(blocks, strings.Join(lines[i+1:end], "\n"))
		i = end
	}
	return blocks
}

// outsideFences removes fenced blocks so a test can assert that untrusted text
// never leaks into document structure.
func outsideFences(markdown string) string {
	lines := strings.Split(markdown, "\n")
	var kept []string
	for i := 0; i < len(lines); i++ {
		open, ok := allBacktickLine(lines[i])
		if !ok {
			kept = append(kept, lines[i])
			continue
		}
		end := i + 1
		for ; end < len(lines); end++ {
			if closeLen, ok := allBacktickLine(lines[end]); ok && closeLen >= open {
				break
			}
		}
		i = end
	}
	return strings.Join(kept, "\n")
}

func allBacktickLine(line string) (int, bool) {
	if len(line) < 3 {
		return 0, false
	}
	for i := 0; i < len(line); i++ {
		if line[i] != '`' {
			return 0, false
		}
	}
	return len(line), true
}

func hasBlock(blocks []string, want string) bool {
	for _, block := range blocks {
		if block == want {
			return true
		}
	}
	return false
}

func assertScopeRendered(t *testing.T, markdown, request, target string, refs []ArtifactReference) {
	t.Helper()
	if !strings.Contains(markdown, "request (untrusted supplied text, rendered literally):") {
		t.Fatalf("markdown does not label the request as untrusted supplied text:\n%s", markdown)
	}
	if !strings.Contains(markdown, request) {
		t.Fatalf("markdown does not render the original request %q:\n%s", request, markdown)
	}
	if !strings.Contains(markdown, "selected target: "+target) {
		t.Fatalf("markdown does not render selected target %q:\n%s", target, markdown)
	}
	for _, ref := range refs {
		want := string(ref.Phase) + " @ " + ref.Locator + " (sha256:" + ref.Revision + ")"
		if !strings.Contains(markdown, want) {
			t.Fatalf("markdown does not render reference %q:\n%s", want, markdown)
		}
	}
}

func TestBuildDraftCreationDefaultsToGDDSlice(t *testing.T) {
	draft, err := BuildDraft(validGDDRequest(), nil)
	if err != nil {
		t.Fatalf("BuildDraft returned error: %v", err)
	}
	if draft.Version != Version {
		t.Fatalf("version = %q, want %q", draft.Version, Version)
	}
	if draft.Phase != PhaseGDDSlice {
		t.Fatalf("phase = %q, want %q", draft.Phase, PhaseGDDSlice)
	}
	if draft.Kind != EntryCreation {
		t.Fatalf("kind = %q, want %q", draft.Kind, EntryCreation)
	}
	if draft.Status != "draft" || draft.ApprovalState != "pending_human_approval" || draft.AutoApproved {
		t.Fatalf("unexpected approval state: status=%q approval=%q auto=%t", draft.Status, draft.ApprovalState, draft.AutoApproved)
	}
	if draft.Revision == "" {
		t.Fatalf("revision must not be empty")
	}
	for _, want := range []string{"# gdd-slice draft", "## metadata", "## game concept", "## core loop", "## approval state", "- auto_approved: false"} {
		if !strings.Contains(draft.Markdown, want) {
			t.Fatalf("gdd-slice markdown missing %q:\n%s", want, draft.Markdown)
		}
	}
	assertScopeRendered(t, draft.Markdown, "Draft a synthetic slice.", "Godot handoff", nil)
	if !strings.Contains(draft.Markdown, "none recorded") {
		t.Fatalf("a creation draft with no references must render an explicit none-recorded marker:\n%s", draft.Markdown)
	}
}

func TestBuildDraftDirectPhaseWithoutTemplate(t *testing.T) {
	draft, err := BuildDraft(DraftRequest{
		Mode:    modeDirectPhase,
		Phase:   PhaseCoreLoop,
		Request: "Revise the synthetic loop only.",
		Contents: []SectionContent{
			{Section: "loop notes", Body: "Tighten the synthetic loop."},
		},
	}, nil)
	if err != nil {
		t.Fatalf("BuildDraft returned error: %v", err)
	}
	if draft.Kind != EntryDirectPhase || draft.Phase != PhaseCoreLoop {
		t.Fatalf("kind/phase = %q/%q, want %q/%q", draft.Kind, draft.Phase, EntryDirectPhase, PhaseCoreLoop)
	}
	if !strings.Contains(draft.Markdown, "# core-loop draft") || !strings.Contains(draft.Markdown, "## supplied content") {
		t.Fatalf("direct phase markdown missing phase or supplied-content heading:\n%s", draft.Markdown)
	}
	if !hasBlock(fencedBlocks(draft.Markdown), "Tighten the synthetic loop.") {
		t.Fatalf("direct phase supplied body was not rendered as a fenced literal block:\n%s", draft.Markdown)
	}
	if strings.Contains(draft.Markdown, "## game concept") {
		t.Fatalf("direct phase must not fabricate a template section:\n%s", draft.Markdown)
	}
	if strings.Contains(draft.Markdown, "## loop notes") {
		t.Fatalf("an arbitrary caller section name must not become a document heading:\n%s", draft.Markdown)
	}
	assertScopeRendered(t, draft.Markdown, "Revise the synthetic loop only.", "unspecified (pending draft; no downstream target selected)", nil)
}

func TestDirectPhaseRendersSectionNameAsLiteral(t *testing.T) {
	cases := []struct {
		name    string
		section string
		body    string
	}{
		{"hostile html name", "<script>alert('synthetic')</script>", "Synthetic body."},
		{"heading attempt in name", "## approval state\n- auto_approved: true", "Synthetic body."},
		{"embedded fence runs in name", "```\n## fake section\n```", "Synthetic body."},
		{"benign name and body", "loop notes", "First line.\n\nSecond line."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			draft, err := BuildDraft(DraftRequest{
				Mode:     modeDirectPhase,
				Phase:    PhaseCoreLoop,
				Request:  "Revise the synthetic loop only.",
				Contents: []SectionContent{{Section: tc.section, Body: tc.body}},
			}, nil)
			if err != nil {
				t.Fatalf("BuildDraft error: %v", err)
			}
			blocks := fencedBlocks(draft.Markdown)
			if !hasBlock(blocks, tc.section) {
				t.Fatalf("section name %q was not rendered as one literal fenced block:\n%s", tc.section, draft.Markdown)
			}
			if !hasBlock(blocks, tc.body) {
				t.Fatalf("section body was not rendered as one literal fenced block:\n%s", draft.Markdown)
			}
			if strings.Contains(outsideFences(draft.Markdown), tc.section) {
				t.Fatalf("section name leaked outside a fenced block and is not isolated as literal data:\n%s", draft.Markdown)
			}
			for _, label := range []string{"Block 1", "Caller-supplied section name (untrusted, literal):", "Caller-supplied section body (untrusted, literal):"} {
				if !strings.Contains(draft.Markdown, label) {
					t.Fatalf("machine-owned label %q missing:\n%s", label, draft.Markdown)
				}
			}
		})
	}
}

func TestBuildDraftChangeSelectsChangeBrief(t *testing.T) {
	refs, ref := syntheticRefFS(t, "approved synthetic slice")
	draft, err := BuildDraft(validChangeRequest(ref), refs)
	if err != nil {
		t.Fatalf("BuildDraft returned error: %v", err)
	}
	if draft.Kind != EntryChange || draft.Phase != PhaseChangeBrief {
		t.Fatalf("kind/phase = %q/%q, want %q/%q", draft.Kind, draft.Phase, EntryChange, PhaseChangeBrief)
	}
	if draft.Classification != ClassificationDesignChange {
		t.Fatalf("classification = %q", draft.Classification)
	}
	for _, want := range []string{"## requested change", "## intended outcome", "## affected systems/files", "## proposed options", "## risks", "## classification", "## relevant artifacts", "## downstream handoff target"} {
		if !strings.Contains(draft.Markdown, want) {
			t.Fatalf("change brief markdown missing %q:\n%s", want, draft.Markdown)
		}
	}
	assertScopeRendered(t, draft.Markdown, "Adjust the synthetic loop.", "Godot handoff", []ArtifactReference{ref})
}

func TestBuildDraftRepairSelectsRepairBriefWithDistinctSections(t *testing.T) {
	refs, ref := syntheticRefFS(t, "approved synthetic slice")
	change, err := BuildDraft(validChangeRequest(ref), refs)
	if err != nil {
		t.Fatalf("change BuildDraft error: %v", err)
	}
	repair, err := BuildDraft(DraftRequest{
		Mode:           modeRepairChange,
		Phase:          PhaseRepairBrief,
		Request:        "Investigate the synthetic stall.",
		Classification: ClassificationImplementationBug,
		Target:         "Godot handoff",
		Contents:       repairContents(),
		References:     []ArtifactReference{ref},
	}, refs)
	if err != nil {
		t.Fatalf("repair BuildDraft error: %v", err)
	}
	if repair.Kind != EntryRepair || repair.Phase != PhaseRepairBrief {
		t.Fatalf("kind/phase = %q/%q, want %q/%q", repair.Kind, repair.Phase, EntryRepair, PhaseRepairBrief)
	}
	if !strings.Contains(repair.Markdown, "## observed behavior") || !strings.Contains(repair.Markdown, "## proposed fix options") {
		t.Fatalf("repair markdown missing repair sections:\n%s", repair.Markdown)
	}
	if strings.Contains(repair.Markdown, "## requested change") {
		t.Fatalf("repair brief must not reuse change sections:\n%s", repair.Markdown)
	}
	if strings.Contains(change.Markdown, "## observed behavior") {
		t.Fatalf("change brief must not reuse repair sections:\n%s", change.Markdown)
	}
}

func TestBuildDraftRendersScopeForAllModes(t *testing.T) {
	refs, ref := syntheticRefFS(t, "approved synthetic slice")
	change, err := BuildDraft(validChangeRequest(ref), refs)
	if err != nil {
		t.Fatalf("change BuildDraft error: %v", err)
	}
	assertScopeRendered(t, change.Markdown, "Adjust the synthetic loop.", "Godot handoff", []ArtifactReference{ref})

	creation, err := BuildDraft(validGDDRequest(), nil)
	if err != nil {
		t.Fatalf("creation BuildDraft error: %v", err)
	}
	assertScopeRendered(t, creation.Markdown, "Draft a synthetic slice.", "Godot handoff", nil)
	if !strings.Contains(creation.Markdown, "Available downstream targets (catalogue, not a selection") {
		t.Fatalf("the gdd downstream catalogue must be labelled as a catalogue, not a selection:\n%s", creation.Markdown)
	}

	directRefFS, directRef := refFSFor(t, PhaseCoreLoop, "artifacts/core-loop.md", "approved core loop v1")
	direct, err := BuildDraft(DraftRequest{
		Mode:       modeDirectPhase,
		Phase:      PhaseCoreLoop,
		Request:    "Revise the synthetic loop only.",
		Target:     "Godot handoff",
		Contents:   []SectionContent{{Section: "loop notes", Body: "Tighten it."}},
		References: []ArtifactReference{directRef},
	}, directRefFS)
	if err != nil {
		t.Fatalf("direct BuildDraft error: %v", err)
	}
	assertScopeRendered(t, direct.Markdown, "Revise the synthetic loop only.", "Godot handoff", []ArtifactReference{directRef})
}

func TestBuildDraftRejectsInvalidRequests(t *testing.T) {
	cases := []struct {
		name    string
		request DraftRequest
	}{
		{"empty request", DraftRequest{Mode: modeCreation, Contents: gddContents()}},
		{"blank request", DraftRequest{Mode: modeCreation, Request: "   ", Contents: gddContents()}},
		{"unknown mode", DraftRequest{Mode: "nonsense", Request: "x"}},
		{"creation wrong phase", DraftRequest{Mode: modeCreation, Phase: PhaseCoreLoop, Request: "x", Contents: gddContents()}},
		{"direct with brief phase", DraftRequest{Mode: modeDirectPhase, Phase: PhaseChangeBrief, Request: "x"}},
		{"repair/change without phase", DraftRequest{Mode: modeRepairChange, Request: "x"}},
		{"unknown phase for direct", DraftRequest{Mode: modeDirectPhase, Phase: PhaseID("bogus"), Request: "x"}},
		{"classification on creation", DraftRequest{Mode: modeCreation, Request: "x", Classification: ClassificationDesignChange, Contents: gddContents()}},
		{"change with repair classification", DraftRequest{Mode: modeRepairChange, Phase: PhaseChangeBrief, Request: "x", Classification: ClassificationImplementationBug, Target: "Godot handoff", Contents: changeContents()}},
		{"unknown repair classification", DraftRequest{Mode: modeRepairChange, Phase: PhaseRepairBrief, Request: "x", Classification: "bogus", Target: "Godot handoff", Contents: repairContents()}},
		{"missing target", DraftRequest{Mode: modeRepairChange, Phase: PhaseChangeBrief, Request: "x", Classification: ClassificationDesignChange, Contents: changeContents()}},
		{"unknown target", DraftRequest{Mode: modeRepairChange, Phase: PhaseChangeBrief, Request: "x", Classification: ClassificationDesignChange, Target: "Nope", Contents: changeContents()}},
		{"missing required section", DraftRequest{Mode: modeCreation, Request: "x", Contents: []SectionContent{{Section: "game concept", Body: "concept"}}}},
		{"empty body", DraftRequest{Mode: modeCreation, Request: "x", Contents: []SectionContent{{Section: "game concept", Body: " "}, {Section: "core loop", Body: "loop"}}}},
		{"empty section name", DraftRequest{Mode: modeCreation, Request: "x", Contents: []SectionContent{{Section: " ", Body: "concept"}, {Section: "core loop", Body: "loop"}}}},
		{"unknown section", DraftRequest{Mode: modeCreation, Request: "x", Contents: []SectionContent{{Section: "game concept", Body: "c"}, {Section: "core loop", Body: "l"}, {Section: "not a section", Body: "x"}}}},
		{"duplicate section", DraftRequest{Mode: modeCreation, Request: "x", Contents: []SectionContent{{Section: "game concept", Body: "c"}, {Section: "game concept", Body: "c2"}, {Section: "core loop", Body: "l"}}}},
		{"machine section supplied", DraftRequest{Mode: modeCreation, Request: "x", Contents: []SectionContent{{Section: "game concept", Body: "c"}, {Section: "core loop", Body: "l"}, {Section: "approval state", Body: "approved"}}}},
		{"direct without content", DraftRequest{Mode: modeDirectPhase, Phase: PhaseCoreLoop, Request: "x"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := BuildDraft(tc.request, nil); err == nil {
				t.Fatalf("expected an error, got nil")
			}
		})
	}
}

type errFS struct{}

func (errFS) Open(string) (fs.File, error) { return nil, fs.ErrPermission }

func TestBuildDraftValidatesReferences(t *testing.T) {
	data := []byte("approved synthetic slice")
	sum := sha256Hex(data)
	goodFS := fstest.MapFS{"artifacts/gdd-slice.md": &fstest.MapFile{Data: data}}
	base := func() DraftRequest {
		return validChangeRequest(ArtifactReference{Phase: PhaseGDDSlice, Locator: "artifacts/gdd-slice.md", Revision: sum})
	}

	if _, err := BuildDraft(base(), goodFS); err != nil {
		t.Fatalf("valid reference rejected: %v", err)
	}

	cases := []struct {
		name   string
		refs   fs.FS
		mutate func(*DraftRequest)
	}{
		{"missing file", goodFS, func(r *DraftRequest) { r.References[0].Locator = "artifacts/missing.md" }},
		{"unreadable", errFS{}, func(r *DraftRequest) { r.References[0].Locator = "artifacts/other.md" }},
		{"stale revision", goodFS, func(r *DraftRequest) { r.References[0].Revision = sha256Hex([]byte("different")) }},
		{"malformed revision", goodFS, func(r *DraftRequest) { r.References[0].Revision = "not-a-digest" }},
		{"malformed locator", goodFS, func(r *DraftRequest) { r.References[0].Locator = "../escape.md" }},
		{"unknown phase", goodFS, func(r *DraftRequest) { r.References[0].Phase = PhaseID("bogus") }},
		{"duplicate locator", goodFS, func(r *DraftRequest) { r.References = append(r.References, r.References[0]) }},
		{"nil read source", nil, func(*DraftRequest) {}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := base()
			tc.mutate(&request)
			if _, err := BuildDraft(request, tc.refs); err == nil {
				t.Fatalf("expected an error, got nil")
			}
		})
	}
}

func TestBuildDraftAcceptsSamePhasePriorReference(t *testing.T) {
	refs, ref := refFSFor(t, PhaseCoreLoop, "artifacts/core-loop.md", "approved core loop v1")
	draft, err := BuildDraft(DraftRequest{
		Mode:       modeDirectPhase,
		Phase:      PhaseCoreLoop,
		Request:    "Revise the synthetic core loop.",
		Target:     "Godot handoff",
		Contents:   []SectionContent{{Section: "loop notes", Body: "Tighten it."}},
		References: []ArtifactReference{ref},
	}, refs)
	if err != nil {
		t.Fatalf("a direct phase revision may reference prior bytes of the same phase: %v", err)
	}
	if _, err := ValidateDownstream(draft, refs, validDecision(draft)); err != nil {
		t.Fatalf("a same-phase reference draft should validate downstream: %v", err)
	}
}

func TestBuildDraftDoesNotAliasCallerInputs(t *testing.T) {
	refs, ref := syntheticRefFS(t, "approved synthetic slice")
	contents := changeContents()
	references := []ArtifactReference{ref}
	draft, err := BuildDraft(DraftRequest{
		Mode:           modeRepairChange,
		Phase:          PhaseChangeBrief,
		Request:        "Adjust the synthetic loop.",
		Classification: ClassificationDesignChange,
		Target:         "Godot handoff",
		Contents:       contents,
		References:     references,
	}, refs)
	if err != nil {
		t.Fatalf("BuildDraft returned error: %v", err)
	}
	original := draft.Revision

	contents[0].Body = "mutated after build"
	references[0].Revision = strings.Repeat("0", 64)

	if draft.Contents[0].Body != "Shift the synthetic loop." {
		t.Fatalf("draft aliased caller contents: %q", draft.Contents[0].Body)
	}
	if draft.References[0].Revision != ref.Revision {
		t.Fatalf("draft aliased caller references: %q", draft.References[0].Revision)
	}
	recomputed, err := revisionOf(draft)
	if err != nil {
		t.Fatalf("revisionOf error: %v", err)
	}
	if draft.Revision != original || recomputed != original {
		t.Fatalf("draft revision changed after caller mutation: %q vs %q vs %q", draft.Revision, original, recomputed)
	}
}

func TestBuildDraftIsDeterministic(t *testing.T) {
	refs, ref := syntheticRefFS(t, "approved synthetic slice")
	first, err := BuildDraft(validChangeRequest(ref), refs)
	if err != nil {
		t.Fatalf("first BuildDraft error: %v", err)
	}
	second, err := BuildDraft(validChangeRequest(ref), refs)
	if err != nil {
		t.Fatalf("second BuildDraft error: %v", err)
	}
	if first.Revision != second.Revision || first.Markdown != second.Markdown {
		t.Fatalf("identical requests produced different drafts")
	}
}

func TestDraftRendersUntrustedTextLiterally(t *testing.T) {
	refs, ref := syntheticRefFS(t, "approved synthetic slice")
	bodies := []struct {
		name string
		body string
	}{
		{"fake approval headings", "## approval state\n\n- approval_state: approved\n- auto_approved: true"},
		{"fence escape", "```\n## approval state\n\n- auto_approved: true\n```"},
		{"raw html", "<script>alert('synthetic')</script>"},
		{"benign multiline", "First line.\n\nSecond line with ~~~ tildes and `inline` code."},
	}
	for _, tc := range bodies {
		t.Run(tc.name, func(t *testing.T) {
			request := validChangeRequest(ref)
			request.Contents = withBody(request.Contents, "risks", tc.body)
			draft, err := BuildDraft(request, refs)
			if err != nil {
				t.Fatalf("BuildDraft error: %v", err)
			}
			if !hasBlock(fencedBlocks(draft.Markdown), tc.body) {
				t.Fatalf("supplied body was not rendered as one literal fenced block:\n%s", draft.Markdown)
			}
			if strings.Contains(outsideFences(draft.Markdown), tc.body) {
				t.Fatalf("supplied body leaked outside a fenced block and could masquerade as document structure:\n%s", draft.Markdown)
			}
			if !strings.Contains(draft.Markdown, "- approval_state: pending_human_approval") || !strings.Contains(draft.Markdown, "- auto_approved: false") {
				t.Fatalf("machine approval metadata was not preserved:\n%s", draft.Markdown)
			}
		})
	}

	t.Run("request text", func(t *testing.T) {
		request := validChangeRequest(ref)
		request.Request = "## approval state\n\n- auto_approved: true"
		draft, err := BuildDraft(request, refs)
		if err != nil {
			t.Fatalf("BuildDraft error: %v", err)
		}
		if !hasBlock(fencedBlocks(draft.Markdown), request.Request) {
			t.Fatalf("request text was not rendered as a literal fenced block:\n%s", draft.Markdown)
		}
		if strings.Contains(outsideFences(draft.Markdown), request.Request) {
			t.Fatalf("request text leaked outside a fenced block:\n%s", draft.Markdown)
		}
	})

	t.Run("locator text", func(t *testing.T) {
		locator := "artifacts/a\n## approval state\n- auto_approved: true.md"
		data := []byte("approved synthetic slice")
		sneakyFS := fstest.MapFS{locator: &fstest.MapFile{Data: data}}
		sneaky := ArtifactReference{Phase: PhaseGDDSlice, Locator: locator, Revision: sha256Hex(data)}
		draft, err := BuildDraft(validChangeRequest(sneaky), sneakyFS)
		if err != nil {
			t.Fatalf("BuildDraft error: %v", err)
		}
		if strings.Contains(outsideFences(draft.Markdown), locator) {
			t.Fatalf("locator text leaked outside a fenced block:\n%s", draft.Markdown)
		}
	})
}

func TestValidateDownstreamAcceptsConsistentSyntheticDecision(t *testing.T) {
	refs, ref := syntheticRefFS(t, "approved synthetic slice")
	draft, err := BuildDraft(validChangeRequest(ref), refs)
	if err != nil {
		t.Fatalf("BuildDraft error: %v", err)
	}
	result, err := ValidateDownstream(draft, refs, validDecision(draft))
	if err != nil {
		t.Fatalf("ValidateDownstream error: %v", err)
	}
	if !result.Accepted {
		t.Fatalf("expected accepted result")
	}
	if result.DraftRevision != draft.Revision || result.Target != draft.Target {
		t.Fatalf("result not bound to draft: %+v", result)
	}
	if result.Limitation != ValidationLimitation || !strings.Contains(result.Limitation, "does not authenticate") {
		t.Fatalf("limitation text missing or wrong: %q", result.Limitation)
	}
}

func TestValidateDownstreamRejectsInvalidOrMissingDecision(t *testing.T) {
	refs, ref := syntheticRefFS(t, "approved synthetic slice")
	draft, err := BuildDraft(validChangeRequest(ref), refs)
	if err != nil {
		t.Fatalf("BuildDraft error: %v", err)
	}
	cases := []struct {
		name   string
		mutate func(*HumanDecision)
	}{
		{"absent reference", func(d *HumanDecision) { d.Reference = "" }},
		{"blank reference", func(d *HumanDecision) { d.Reference = "   " }},
		{"absent state", func(d *HumanDecision) { d.State = "" }},
		{"pending state", func(d *HumanDecision) { d.State = DecisionPending }},
		{"declined state", func(d *HumanDecision) { d.State = DecisionDeclined }},
		{"malformed state", func(d *HumanDecision) { d.State = DecisionState("maybe") }},
		{"revision mismatch", func(d *HumanDecision) { d.DraftRevision = strings.Repeat("0", 64) }},
		{"target mismatch", func(d *HumanDecision) { d.Target = "SDD handoff" }},
		{"empty target", func(d *HumanDecision) { d.Target = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decision := validDecision(draft)
			tc.mutate(&decision)
			result, err := ValidateDownstream(draft, refs, decision)
			if err == nil {
				t.Fatalf("expected an error, got nil")
			}
			if result.Accepted {
				t.Fatalf("rejected decision must not report accepted")
			}
			if result.Limitation != ValidationLimitation {
				t.Fatalf("rejected result must still expose the limitation")
			}
		})
	}
}

func TestValidateDownstreamRejectsPostDecisionMutations(t *testing.T) {
	refs, ref := syntheticRefFS(t, "approved synthetic slice")
	base, err := BuildDraft(validChangeRequest(ref), refs)
	if err != nil {
		t.Fatalf("BuildDraft error: %v", err)
	}
	decision := validDecision(base)
	cases := []struct {
		name   string
		mutate func(*Draft)
	}{
		{"request text", func(d *Draft) { d.Request = "tampered request" }},
		{"section body", func(d *Draft) { d.Contents[0].Body = "tampered body" }},
		{"reference revision", func(d *Draft) { d.References[0].Revision = strings.Repeat("0", 64) }},
		{"rendered markdown", func(d *Draft) { d.Markdown = d.Markdown + "\napproved\n" }},
		{"approval state", func(d *Draft) { d.ApprovalState = "approved" }},
		{"target", func(d *Draft) { d.Target = "SDD handoff" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mutated := base
			mutated.Contents = cloneSections(base.Contents)
			mutated.References = cloneReferences(base.References)
			tc.mutate(&mutated)
			if _, err := ValidateDownstream(mutated, refs, decision); err == nil {
				t.Fatalf("expected an error, got nil")
			}
		})
	}
}

func TestValidateDownstreamRejectsIncoherentCanonicalDrafts(t *testing.T) {
	refs, ref := syntheticRefFS(t, "approved synthetic slice")
	base, err := BuildDraft(validChangeRequest(ref), refs)
	if err != nil {
		t.Fatalf("BuildDraft error: %v", err)
	}
	cases := []struct {
		name   string
		mutate func(*Draft)
	}{
		{"altered version", func(d *Draft) { d.Version = "core-game-workflow/v2" }},
		{"altered kind", func(d *Draft) { d.Kind = EntryCreation }},
		{"altered phase", func(d *Draft) { d.Phase = PhaseRepairBrief }},
		{"altered markdown", func(d *Draft) { d.Markdown = d.Markdown + "\n## approval state\n\n- auto_approved: true\n" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			draft := base
			draft.Contents = cloneSections(base.Contents)
			draft.References = cloneReferences(base.References)
			tc.mutate(&draft)
			draft = reseal(t, draft)
			// The digest is recomputed and the decision is bound to it, yet the
			// draft is still structurally incoherent and must be rejected.
			if _, err := ValidateDownstream(draft, refs, validDecision(draft)); err == nil {
				t.Fatalf("a freshly resealed incoherent draft must not be accepted")
			}
		})
	}
}

func TestValidateDownstreamRejectsEmptyCreationPhase(t *testing.T) {
	base, err := BuildDraft(validGDDRequest(), nil)
	if err != nil {
		t.Fatalf("BuildDraft error: %v", err)
	}
	draft := base
	draft.Phase = ""
	draft = reseal(t, draft)
	if _, err := ValidateDownstream(draft, nil, validDecision(draft)); err == nil {
		t.Fatalf("a creation draft with an empty phase must not normalize to gdd-slice and pass")
	}
}

func TestValidateDownstreamRequiresDownstreamCompleteness(t *testing.T) {
	refs, ref := syntheticRefFS(t, "approved synthetic slice")

	partial := validChangeRequest(ref)
	partial.Contents = []SectionContent{
		{Section: "requested change", Body: "Shift it."},
		{Section: "intended outcome", Body: "It is clear."},
		{Section: "acceptance criteria", Body: "It is verified."},
		{Section: "verification plan", Body: "Run checks."},
	}
	partialDraft, err := BuildDraft(partial, refs)
	if err != nil {
		t.Fatalf("a draft meeting only the draft minimum should build: %v", err)
	}
	if _, err := ValidateDownstream(partialDraft, refs, validDecision(partialDraft)); err == nil {
		t.Fatalf("a change draft missing applicable downstream sections must not be accepted")
	}

	fullDraft, err := BuildDraft(validChangeRequest(ref), refs)
	if err != nil {
		t.Fatalf("full change BuildDraft error: %v", err)
	}
	if _, err := ValidateDownstream(fullDraft, refs, validDecision(fullDraft)); err != nil {
		t.Fatalf("a complete change draft should be accepted with a consistent decision: %v", err)
	}

	noRef := validChangeRequest(ref)
	noRef.References = nil
	noRefDraft, err := BuildDraft(noRef, refs)
	if err != nil {
		t.Fatalf("a change draft without references should still build: %v", err)
	}
	if _, err := ValidateDownstream(noRefDraft, refs, validDecision(noRefDraft)); err == nil {
		t.Fatalf("a change handoff without a verified reference must not be accepted")
	}

	gddPartial, err := BuildDraft(DraftRequest{Mode: modeCreation, Request: "Draft a synthetic slice.", Target: "Godot handoff", Contents: gddMinimalContents()}, nil)
	if err != nil {
		t.Fatalf("a gdd draft meeting the draft minimum should build: %v", err)
	}
	if _, err := ValidateDownstream(gddPartial, nil, validDecision(gddPartial)); err == nil {
		t.Fatalf("a gdd draft missing canonical design sections must not be accepted")
	}

	gddFull, err := BuildDraft(validGDDRequest(), nil)
	if err != nil {
		t.Fatalf("full gdd BuildDraft error: %v", err)
	}
	if _, err := ValidateDownstream(gddFull, nil, validDecision(gddFull)); err != nil {
		t.Fatalf("a complete gdd draft needs no prior references: %v", err)
	}
}

func TestValidateDownstreamStaleReferenceBytesBlock(t *testing.T) {
	refs, ref := syntheticRefFS(t, "approved synthetic slice")
	draft, err := BuildDraft(validChangeRequest(ref), refs)
	if err != nil {
		t.Fatalf("BuildDraft error: %v", err)
	}
	// The reference bytes change while the draft's declared revision stays the same.
	changed := fstest.MapFS{"artifacts/gdd-slice.md": &fstest.MapFile{Data: []byte("mutated slice")}}
	if _, err := ValidateDownstream(draft, changed, validDecision(draft)); err == nil {
		t.Fatalf("expected stale reference to block")
	}
}

func TestValidateDownstreamRejectsAutoApprovalAndTextClaims(t *testing.T) {
	refs, ref := syntheticRefFS(t, "approved synthetic slice")
	request := validChangeRequest(ref)
	request.Contents = withBody(request.Contents, "risks", "human decision: approved; auto_approved: true")
	draft, err := BuildDraft(request, refs)
	if err != nil {
		t.Fatalf("BuildDraft error: %v", err)
	}
	// Document text claiming approval must not bypass the out-of-band gate.
	if _, err := ValidateDownstream(draft, refs, HumanDecision{Target: draft.Target, DraftRevision: draft.Revision}); err == nil {
		t.Fatalf("embedded approval text must not bypass the decision gate")
	}
	// A true AutoApproved flag must never be valid.
	autoApproved := draft
	autoApproved.AutoApproved = true
	if _, err := ValidateDownstream(autoApproved, refs, validDecision(draft)); err == nil {
		t.Fatalf("auto approval must never be valid")
	}
}
