package coregame

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
)

// EntryKind records how a draft entered the core-game workflow. It maps onto
// the existing ModeContract IDs plus the phase that separates a change brief
// from a repair brief.
type EntryKind string

const (
	EntryCreation    EntryKind = "creation"
	EntryDirectPhase EntryKind = "direct-phase"
	EntryChange      EntryKind = "change"
	EntryRepair      EntryKind = "repair"
)

// Mode IDs reused verbatim from the core-game-workflow/v1 contract.
const (
	modeCreation     = "zero-to-one creation"
	modeDirectPhase  = "direct phase invocation"
	modeRepairChange = "repair/change handoff"
)

// DecisionState is the recorded human decision state a downstream handoff must
// carry. Only DecisionApproved permits consistency validation. This package
// never records a decision and never captures consent.
type DecisionState string

const (
	DecisionApproved DecisionState = "approved"
	DecisionPending  DecisionState = "pending"
	DecisionDeclined DecisionState = "declined"
)

// ValidationLimitation is attached to every downstream validation result. It is
// the explicit trust boundary: this package checks a draft against a decision
// reference supplied by a trusted coordinator. It does not authenticate a
// human, capture consent, or grant execution authority.
const ValidationLimitation = "consistency validation only: verifies a draft against a coordinator-supplied recorded human decision; it does not authenticate a human, capture consent, or grant execution authority"

// SectionContent is one body of caller-supplied text for a named document
// section. Section text is data only and is never execution authority. It is
// always rendered as literal fenced content, never as raw Markdown headings or
// executable HTML.
type SectionContent struct {
	Section string
	Body    string
}

// ArtifactReference identifies an existing workflow artifact by phase, an
// explicit caller-root-relative locator, and the expected SHA256 of its actual
// bytes. BuildDraft verifies the revision against bytes read through the
// caller-owned read seam, never against a caller-supplied string alone. Phase
// is a label, not an identity: a direct phase revision may legitimately
// reference prior bytes of the same phase at a declared locator.
type ArtifactReference struct {
	Phase    PhaseID
	Locator  string
	Revision string
}

// DraftRequest is a deterministic, structured request to build a draft. There
// is no natural-language inference: mode, phase, classification, target,
// section contents and artifact references are all explicit. Contents and
// References are copied by BuildDraft, so later caller mutation never affects a
// built draft.
type DraftRequest struct {
	Mode           string
	Phase          PhaseID
	Request        string
	Classification RepairClassification
	Target         string
	Contents       []SectionContent
	References     []ArtifactReference
}

// Draft is a concrete, still-unapproved draft artifact. Status, ApprovalState
// and AutoApproved come from the contract approval policy; AutoApproved is
// always false and no supplied text or field can change that. Revision is a
// deterministic digest over every semantic field, the rendered Markdown and the
// verified references.
//
// A draft may be incomplete at construction time: it is pending and no
// downstream handoff follows from it. Downstream completeness is enforced
// separately by ValidateDownstream.
type Draft struct {
	Version        string
	Mode           string
	Kind           EntryKind
	Phase          PhaseID
	Classification RepairClassification
	Request        string
	Target         string
	Contents       []SectionContent
	References     []ArtifactReference
	Markdown       string
	Status         string
	ApprovalState  string
	AutoApproved   bool
	Revision       string
}

// HumanDecision is an out-of-band decision record supplied by a trusted
// coordinator. It must name a nonempty recorded decision reference, carry an
// approved state, and be bound to the exact draft revision and intended
// downstream target. Document text or draft fields can never substitute for it.
type HumanDecision struct {
	Reference     string
	State         DecisionState
	Target        string
	DraftRevision string
}

// DownstreamValidation reports the outcome of consistency validation. Accepted
// means the draft and the supplied decision agree. It is never consent and
// never execution authority; see ValidationLimitation.
type DownstreamValidation struct {
	Accepted          bool
	Phase             PhaseID
	Target            string
	DraftRevision     string
	DecisionReference string
	DecisionState     DecisionState
	Limitation        string
}

// briefPlan is the normalized, copied intent behind a draft.
type briefPlan struct {
	mode           string
	kind           EntryKind
	phase          PhaseID
	classification RepairClassification
	request        string
	target         string
	contents       []SectionContent
	references     []ArtifactReference
}

// machineSections are document sections rendered only from typed draft fields
// or the contract. Caller text may never supply or override them.
var machineSections = map[string]bool{
	"metadata":                  true,
	"classification":            true,
	"relevant artifacts":        true,
	"human decision":            true,
	"approval state":            true,
	"auto_approved: false":      true,
	"downstream handoff target": true,
	"downstream references":     true,
}

// draftMinimumSections are caller-owned sections required merely to construct a
// pending draft. Deliberately smaller than the downstream set: a pending draft
// may be incomplete because it is not approved and nothing is handed off from
// it. Direct phases without a template only require at least one supplied
// block. See validateDownstreamCompleteness for the stricter downstream rule.
var draftMinimumSections = map[PhaseID][]string{
	PhaseGDDSlice:    {"game concept", "core loop"},
	PhaseChangeBrief: {"requested change", "intended outcome", "acceptance criteria", "verification plan"},
	PhaseRepairBrief: {"observed behavior", "expected behavior", "acceptance criteria", "verification plan"},
}

// BuildDraft turns a structured request into a concrete pending draft. It
// validates the request, resolves the entry kind and phase against the existing
// contract, verifies artifact references through refs, renders Markdown from
// the existing templates, and computes the revision digest. It returns an
// actionable error for an empty request, an unknown or mismatched mode/phase,
// an invalid classification, a missing draft-minimum section, or a missing,
// unreadable, malformed, duplicate, unknown-phase or stale reference.
//
// BuildDraft enforces draft-minimum completeness only. Downstream completeness
// is enforced by ValidateDownstream, so an incomplete pending draft can exist
// without being mistaken for an approved handoff.
//
// refs is the caller-owned read seam: BuildDraft performs no filesystem,
// network or discovery access of its own beyond reading the exact locators the
// caller declared. A nil refs is only valid when no references are declared.
func BuildDraft(request DraftRequest, refs fs.FS) (Draft, error) {
	contract := DefaultContract()
	plan, err := planDraft(request, contract)
	if err != nil {
		return Draft{}, err
	}
	return buildFromPlan(plan, contract, refs)
}

// ValidateDownstream first proves the draft is canonically coherent, then
// rechecks current reference bytes, then confirms the coordinator-supplied
// decision agrees with the draft revision and downstream target.
//
// Structural coherence is independent of any supplied decision: the draft's
// Version, Kind, Phase, semantic fields and rendered Markdown must match a
// canonical reconstruction of the same request, and its revision must be that
// reconstruction's revision. A freshly recomputed digest over tampered fields
// and a matching synthetic decision therefore cannot rescue a malformed or
// incoherent draft.
//
// Downstream completeness is stricter than BuildDraft's draft minimum: a
// templated downstream draft must carry every applicable caller-owned canonical
// section, and a change or repair handoff must carry at least one byte-verified
// relevant artifact reference. An absent, pending, declined, malformed or
// mismatched decision blocks, as does a true AutoApproved or non-pending
// approval state.
//
// This is consistency validation of an already-observed human decision. It is
// not authentication, not consent capture, and not execution authority.
func ValidateDownstream(draft Draft, refs fs.FS, decision HumanDecision) (DownstreamValidation, error) {
	contract := DefaultContract()
	result := DownstreamValidation{
		Phase:             draft.Phase,
		Target:            draft.Target,
		DecisionReference: decision.Reference,
		DecisionState:     decision.State,
		Limitation:        ValidationLimitation,
	}
	canonical, plan, err := canonicalDraft(draft, contract, refs)
	if err != nil {
		return result, err
	}
	if err := draftMatchesCanonical(draft, canonical); err != nil {
		return result, err
	}
	if err := validateDownstreamCompleteness(plan, contract); err != nil {
		return result, err
	}
	if draft.AutoApproved {
		return result, fmt.Errorf("draft auto approval is never valid")
	}
	result.DraftRevision = draft.Revision
	if decision.DraftRevision != draft.Revision {
		return result, fmt.Errorf("decision is not bound to the current draft revision")
	}
	if strings.TrimSpace(decision.Reference) == "" {
		return result, fmt.Errorf("decision must name a recorded human-decision reference")
	}
	switch decision.State {
	case DecisionApproved:
	case DecisionPending:
		return result, fmt.Errorf("decision is still pending human approval")
	case DecisionDeclined:
		return result, fmt.Errorf("decision was declined")
	default:
		return result, fmt.Errorf("malformed decision state %q", decision.State)
	}
	if decision.Target == "" || decision.Target != draft.Target {
		return result, fmt.Errorf("decision target %q does not match draft target %q", decision.Target, draft.Target)
	}
	result.Accepted = true
	return result, nil
}

func draftAsRequest(draft Draft) DraftRequest {
	return DraftRequest{
		Mode:           draft.Mode,
		Phase:          draft.Phase,
		Request:        draft.Request,
		Classification: draft.Classification,
		Target:         draft.Target,
		Contents:       draft.Contents,
		References:     draft.References,
	}
}

// canonicalDraft reconstructs the only coherent draft for the request and
// verifies the reference bytes. It is the authority ValidateDownstream compares
// against, so a matching decision cannot legitimize a structurally incoherent
// draft.
func canonicalDraft(draft Draft, contract Contract, refs fs.FS) (Draft, briefPlan, error) {
	plan, err := planDraft(draftAsRequest(draft), contract)
	if err != nil {
		return Draft{}, briefPlan{}, fmt.Errorf("draft is not structurally complete: %w", err)
	}
	canonical, err := buildFromPlan(plan, contract, refs)
	if err != nil {
		return Draft{}, briefPlan{}, err
	}
	return canonical, plan, nil
}

func buildFromPlan(plan briefPlan, contract Contract, refs fs.FS) (Draft, error) {
	if err := resolveReferences(refs, plan.references, contract); err != nil {
		return Draft{}, err
	}
	markdown, err := renderDraft(plan, contract, DefaultMarkdownTemplates())
	if err != nil {
		return Draft{}, err
	}
	draft := newDraft(plan, contract, markdown)
	revision, err := revisionOf(draft)
	if err != nil {
		return Draft{}, err
	}
	draft.Revision = revision
	return draft, nil
}

func newDraft(plan briefPlan, contract Contract, markdown string) Draft {
	return Draft{
		Version:        contract.Version,
		Mode:           plan.mode,
		Kind:           plan.kind,
		Phase:          plan.phase,
		Classification: plan.classification,
		Request:        plan.request,
		Target:         plan.target,
		Contents:       plan.contents,
		References:     plan.references,
		Markdown:       markdown,
		Status:         contract.ApprovalPolicy.DefaultStatus,
		ApprovalState:  contract.ApprovalPolicy.ApprovalState,
		AutoApproved:   contract.ApprovalPolicy.AutoApproved,
	}
}

func draftMatchesCanonical(draft, canonical Draft) error {
	switch {
	case draft.Version != canonical.Version:
		return fmt.Errorf("draft version %q is not the canonical %q", draft.Version, canonical.Version)
	case draft.Mode != canonical.Mode:
		return fmt.Errorf("draft mode %q is not the canonical %q", draft.Mode, canonical.Mode)
	case draft.Kind != canonical.Kind:
		return fmt.Errorf("draft kind %q is not the canonical %q", draft.Kind, canonical.Kind)
	case draft.Phase != canonical.Phase:
		return fmt.Errorf("draft phase %q is not the canonical %q", draft.Phase, canonical.Phase)
	case draft.Classification != canonical.Classification:
		return fmt.Errorf("draft classification %q is not the canonical %q", draft.Classification, canonical.Classification)
	case draft.Request != canonical.Request:
		return fmt.Errorf("draft request text is not the canonical request")
	case draft.Target != canonical.Target:
		return fmt.Errorf("draft target %q is not the canonical %q", draft.Target, canonical.Target)
	case !sameSections(draft.Contents, canonical.Contents):
		return fmt.Errorf("draft contents are not the canonical supplied sections")
	case !sameReferences(draft.References, canonical.References):
		return fmt.Errorf("draft references are not the canonical verified references")
	case draft.Markdown != canonical.Markdown:
		return fmt.Errorf("draft markdown is not the canonical rendering for phase %q", canonical.Phase)
	case draft.Status != canonical.Status:
		return fmt.Errorf("draft status %q is not the pending contract default", draft.Status)
	case draft.ApprovalState != canonical.ApprovalState:
		return fmt.Errorf("draft approval state %q is not the pending contract default", draft.ApprovalState)
	case draft.AutoApproved != canonical.AutoApproved:
		return fmt.Errorf("draft auto approval flag is not the contract default")
	case draft.Revision != canonical.Revision:
		return fmt.Errorf("draft revision does not match its canonical reconstruction")
	}
	return nil
}

// validateDownstreamCompleteness enforces the stricter downstream rule: every
// applicable caller-owned canonical section must be supplied for a templated
// phase, and change/repair handoffs need at least one verified reference. It is
// intentionally separate from the draft minimum so an incomplete pending draft
// can exist without being handoff-ready. Direct phases without a template stay
// bounded phase output and are not expanded into an invented questionnaire.
func validateDownstreamCompleteness(plan briefPlan, contract Contract) error {
	template, templated := lookupTemplate(DefaultMarkdownTemplates(), plan.phase)
	if templated {
		have := map[string]bool{}
		for _, item := range plan.contents {
			have[item.Section] = true
		}
		for _, section := range template.Sections {
			if machineSections[section] {
				continue
			}
			if !have[section] {
				return fmt.Errorf("downstream %s draft is missing applicable section %q", plan.phase, section)
			}
		}
	}
	if plan.kind == EntryChange || plan.kind == EntryRepair {
		if len(plan.references) == 0 {
			return fmt.Errorf("downstream %s draft requires at least one verified relevant artifact reference", plan.kind)
		}
	}
	return nil
}

func planDraft(request DraftRequest, contract Contract) (briefPlan, error) {
	plan := briefPlan{
		mode:           request.Mode,
		classification: request.Classification,
		request:        request.Request,
		target:         request.Target,
		contents:       cloneSections(request.Contents),
		references:     cloneReferences(request.References),
	}
	if strings.TrimSpace(plan.request) == "" {
		return briefPlan{}, fmt.Errorf("draft request text must not be empty")
	}
	kind, phase, err := resolveEntry(plan.mode, request.Phase)
	if err != nil {
		return briefPlan{}, err
	}
	plan.kind, plan.phase = kind, phase
	if err := validateClassification(plan.kind, plan.classification, contract); err != nil {
		return briefPlan{}, err
	}
	if err := validateTarget(plan.kind, plan.target, contract); err != nil {
		return briefPlan{}, err
	}
	template, templated := lookupTemplate(DefaultMarkdownTemplates(), plan.phase)
	if err := validateSections(plan, template, templated); err != nil {
		return briefPlan{}, err
	}
	return plan, nil
}

func resolveEntry(mode string, phase PhaseID) (EntryKind, PhaseID, error) {
	switch mode {
	case modeCreation:
		if phase == "" {
			phase = PhaseGDDSlice
		}
		if phase != PhaseGDDSlice {
			return "", "", fmt.Errorf("mode %q targets %q only, got %q", modeCreation, PhaseGDDSlice, phase)
		}
		return EntryCreation, phase, nil
	case modeDirectPhase:
		if !isDesignPhase(phase) {
			return "", "", fmt.Errorf("mode %q requires one design phase, got %q", modeDirectPhase, phase)
		}
		return EntryDirectPhase, phase, nil
	case modeRepairChange:
		switch phase {
		case PhaseChangeBrief:
			return EntryChange, phase, nil
		case PhaseRepairBrief:
			return EntryRepair, phase, nil
		default:
			return "", "", fmt.Errorf("mode %q requires phase %q or %q, got %q", modeRepairChange, PhaseChangeBrief, PhaseRepairBrief, phase)
		}
	default:
		return "", "", fmt.Errorf("unknown workflow mode %q", mode)
	}
}

func isDesignPhase(phase PhaseID) bool {
	switch phase {
	case PhaseGameConcept, PhaseGamePillars, PhaseCoreLoop, PhasePlayerFantasy,
		PhaseMechanicsBrief, PhaseNarrativeBrief, PhaseToneAndMood, PhaseStoryConstraints, PhaseGDDSlice:
		return true
	default:
		return false
	}
}

func validateClassification(kind EntryKind, classification RepairClassification, contract Contract) error {
	switch kind {
	case EntryCreation, EntryDirectPhase:
		if classification != "" {
			return fmt.Errorf("classification %q is only valid for change or repair drafts", classification)
		}
		return nil
	case EntryChange:
		if classification != ClassificationDesignChange {
			return fmt.Errorf("change brief classification must be %q, got %q", ClassificationDesignChange, classification)
		}
		return nil
	case EntryRepair:
		if !contractHasClassification(contract, classification) {
			return fmt.Errorf("unknown repair classification %q", classification)
		}
		return nil
	default:
		return fmt.Errorf("unsupported draft kind %q", kind)
	}
}

func validateTarget(kind EntryKind, target string, contract Contract) error {
	if target == "" {
		if kind == EntryChange || kind == EntryRepair {
			return fmt.Errorf("%s draft requires a downstream target", kind)
		}
		return nil
	}
	if !contractHasTarget(contract, target) {
		return fmt.Errorf("unknown downstream target %q", target)
	}
	return nil
}

func validateSections(plan briefPlan, template MarkdownTemplate, templated bool) error {
	allowed := map[string]bool{}
	if templated {
		for _, section := range template.Sections {
			if !machineSections[section] {
				allowed[section] = true
			}
		}
	}
	seen := map[string]bool{}
	for _, item := range plan.contents {
		if strings.TrimSpace(item.Section) == "" {
			return fmt.Errorf("supplied section name must not be empty")
		}
		if machineSections[item.Section] {
			return fmt.Errorf("section %q is machine-owned and cannot be supplied", item.Section)
		}
		if templated && !allowed[item.Section] {
			return fmt.Errorf("section %q is not part of the %s template", item.Section, plan.phase)
		}
		if seen[item.Section] {
			return fmt.Errorf("duplicate supplied section %q", item.Section)
		}
		seen[item.Section] = true
		if strings.TrimSpace(item.Body) == "" {
			return fmt.Errorf("supplied section %q has an empty body", item.Section)
		}
	}
	if !templated && len(plan.contents) == 0 {
		return fmt.Errorf("direct phase %q requires at least one supplied content block", plan.phase)
	}
	for _, required := range draftMinimumSections[plan.phase] {
		if !seen[required] {
			return fmt.Errorf("draft is missing required supplied section %q", required)
		}
	}
	return nil
}

func resolveReferences(refs fs.FS, references []ArtifactReference, contract Contract) error {
	if len(references) == 0 {
		return nil
	}
	if refs == nil {
		return fmt.Errorf("artifact references require a caller-owned read source")
	}
	seen := map[string]bool{}
	for _, ref := range references {
		if !fs.ValidPath(ref.Locator) {
			return fmt.Errorf("artifact reference %q has an invalid locator %q", ref.Phase, ref.Locator)
		}
		if !isSHA256Hex(ref.Revision) {
			return fmt.Errorf("artifact reference %q revision %q is not a SHA256 hex digest", ref.Locator, ref.Revision)
		}
		if !contractHasPhase(contract, ref.Phase) {
			return fmt.Errorf("artifact reference %q has unknown phase %q", ref.Locator, ref.Phase)
		}
		if seen[ref.Locator] {
			return fmt.Errorf("duplicate artifact reference locator %q", ref.Locator)
		}
		seen[ref.Locator] = true
		data, err := fs.ReadFile(refs, ref.Locator)
		if err != nil {
			return fmt.Errorf("artifact reference %q (%s): %w", ref.Locator, ref.Phase, err)
		}
		sum := sha256.Sum256(data)
		if actual := hex.EncodeToString(sum[:]); actual != ref.Revision {
			return fmt.Errorf("artifact reference %q is stale: expected %s, found %s", ref.Locator, ref.Revision, actual)
		}
	}
	return nil
}

func renderDraft(plan briefPlan, contract Contract, templates []MarkdownTemplate) (string, error) {
	template, templated := lookupTemplate(templates, plan.phase)
	var b strings.Builder
	if templated {
		fmt.Fprintf(&b, "# %s draft\n\n", plan.phase)
		fmt.Fprintf(&b, "%s\n\n", template.Purpose)
		for _, section := range template.Sections {
			body, ok := sectionBody(section, plan, contract)
			if !ok {
				continue
			}
			fmt.Fprintf(&b, "## %s\n\n%s\n\n", section, body)
		}
		return strings.TrimRight(b.String(), "\n") + "\n", nil
	}
	fmt.Fprintf(&b, "# %s draft\n\n", plan.phase)
	fmt.Fprintf(&b, "Direct phase invocation for %s. No markdown template is defined for this phase, so supplied content is rendered literally as untrusted caller data alongside machine-owned metadata.\n\n", plan.phase)
	b.WriteString("## metadata\n\n")
	b.WriteString(metadataBody(plan, contract))
	b.WriteString("\n\n## supplied content\n\n")
	for i, item := range plan.contents {
		fmt.Fprintf(&b, "Block %d\n\n", i+1)
		fmt.Fprintf(&b, "Caller-supplied section name (untrusted, literal):\n\n%s\n\n", literalBlock(item.Section))
		fmt.Fprintf(&b, "Caller-supplied section body (untrusted, literal):\n\n%s\n\n", literalBlock(item.Body))
	}
	b.WriteString("## approval state\n\n")
	b.WriteString(approvalBody(contract))
	b.WriteString("\n\n## auto_approved: false\n\n- auto_approved: false\n")
	return strings.TrimRight(b.String(), "\n") + "\n", nil
}

func sectionBody(section string, plan briefPlan, contract Contract) (string, bool) {
	if body, ok := machineSectionBody(section, plan, contract); ok {
		return body, true
	}
	for _, item := range plan.contents {
		if item.Section == section {
			return literalBlock(item.Body), true
		}
	}
	return "", false
}

func machineSectionBody(section string, plan briefPlan, contract Contract) (string, bool) {
	switch section {
	case "metadata":
		return metadataBody(plan, contract), true
	case "classification":
		if plan.classification == "" {
			return "", false
		}
		return "- classification: " + string(plan.classification), true
	case "relevant artifacts":
		return literalBlock(referencesLiteral(plan.references)), true
	case "human decision":
		return "- human decision / approval state: pending_human_approval\n- recorded decision reference: none", true
	case "approval state":
		return approvalBody(contract), true
	case "auto_approved: false":
		return "- auto_approved: false", true
	case "downstream handoff target":
		return "- selected target: " + selectedTarget(plan), true
	case "downstream references":
		return targetCatalogue(contract), true
	default:
		return "", false
	}
}

// metadataBody renders machine-owned scope for every mode: the original
// request, the selected target (or an explicit unspecified marker for a pending
// draft), and every declared reference's phase, locator and SHA. The request
// and reference text are caller-supplied, so they are rendered literally.
func metadataBody(plan briefPlan, contract Contract) string {
	policy := contract.ApprovalPolicy
	var b strings.Builder
	fmt.Fprintf(&b, "- workflow: %s\n", contract.Version)
	fmt.Fprintf(&b, "- phase: %s\n", plan.phase)
	fmt.Fprintf(&b, "- mode: %s\n", plan.mode)
	fmt.Fprintf(&b, "- entry: %s\n", plan.kind)
	fmt.Fprintf(&b, "- status: %s\n", policy.DefaultStatus)
	fmt.Fprintf(&b, "- approval_state: %s\n", policy.ApprovalState)
	fmt.Fprintf(&b, "- auto_approved: %t\n", policy.AutoApproved)
	fmt.Fprintf(&b, "- selected target: %s\n", selectedTarget(plan))
	fmt.Fprintf(&b, "\nrequest (untrusted supplied text, rendered literally):\n\n%s\n", literalBlock(plan.request))
	fmt.Fprintf(&b, "\nreferences (phase @ locator @ sha256, caller-declared):\n\n%s", literalBlock(referencesLiteral(plan.references)))
	return b.String()
}

func approvalBody(contract Contract) string {
	policy := contract.ApprovalPolicy
	return fmt.Sprintf("- approval_state: %s\n- auto_approved: %t", policy.ApprovalState, policy.AutoApproved)
}

func selectedTarget(plan briefPlan) string {
	if plan.target == "" {
		return "unspecified (pending draft; no downstream target selected)"
	}
	return plan.target
}

func referencesLiteral(references []ArtifactReference) string {
	if len(references) == 0 {
		return "none recorded"
	}
	var b strings.Builder
	for i, ref := range references {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "%s @ %s (sha256:%s)", ref.Phase, ref.Locator, ref.Revision)
	}
	return b.String()
}

// targetCatalogue renders the contract's available downstream targets. It is
// machine-generated from the contract and is explicitly labelled as a
// catalogue, not as the draft's selected target.
func targetCatalogue(contract Contract) string {
	var b strings.Builder
	b.WriteString("Available downstream targets (catalogue, not a selection; machine-generated from the workflow contract):")
	for _, ref := range contract.DownstreamReferences {
		fmt.Fprintf(&b, "\n- %s: %s", ref.Target, ref.ApprovalRequirement)
	}
	return b.String()
}

// literalBlock renders untrusted text as a fenced literal block whose fence is
// longer than any backtick run in the content. That keeps caller text from
// masquerading as Markdown headings, machine approval metadata or raw HTML.
func literalBlock(content string) string {
	fence := strings.Repeat("`", longestBacktickRun(content)+1)
	if len(fence) < 3 {
		fence = "```"
	}
	return fence + "\n" + content + "\n" + fence
}

func longestBacktickRun(content string) int {
	longest := 0
	current := 0
	for i := 0; i < len(content); i++ {
		if content[i] == '`' {
			current++
			if current > longest {
				longest = current
			}
			continue
		}
		current = 0
	}
	return longest
}

func lookupTemplate(templates []MarkdownTemplate, phase PhaseID) (MarkdownTemplate, bool) {
	for _, template := range templates {
		if template.ID == phase {
			return template, true
		}
	}
	return MarkdownTemplate{}, false
}

func contractHasPhase(contract Contract, phase PhaseID) bool {
	for _, candidate := range contract.PhaseIDs {
		if candidate == phase {
			return true
		}
	}
	return false
}

func contractHasClassification(contract Contract, classification RepairClassification) bool {
	for _, candidate := range contract.RepairClassifications {
		if candidate == classification {
			return true
		}
	}
	return false
}

func contractHasTarget(contract Contract, target string) bool {
	for _, candidate := range contract.DownstreamReferences {
		if candidate.Target == target {
			return true
		}
	}
	return false
}

func isSHA256Hex(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func sameSections(a, b []SectionContent) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sameReferences(a, b []ArtifactReference) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func cloneSections(in []SectionContent) []SectionContent {
	if in == nil {
		return nil
	}
	out := make([]SectionContent, len(in))
	copy(out, in)
	return out
}

func cloneReferences(in []ArtifactReference) []ArtifactReference {
	if in == nil {
		return nil
	}
	out := make([]ArtifactReference, len(in))
	copy(out, in)
	return out
}

type revisionPayload struct {
	Version        string               `json:"version"`
	Mode           string               `json:"mode"`
	Kind           EntryKind            `json:"kind"`
	Phase          PhaseID              `json:"phase"`
	Classification RepairClassification `json:"classification"`
	Request        string               `json:"request"`
	Target         string               `json:"target"`
	Contents       []SectionContent     `json:"contents"`
	References     []ArtifactReference  `json:"references"`
	Markdown       string               `json:"markdown"`
	Status         string               `json:"status"`
	ApprovalState  string               `json:"approval_state"`
	AutoApproved   bool                 `json:"auto_approved"`
}

func revisionOf(draft Draft) (string, error) {
	payload, err := json.Marshal(revisionPayload{
		Version:        draft.Version,
		Mode:           draft.Mode,
		Kind:           draft.Kind,
		Phase:          draft.Phase,
		Classification: draft.Classification,
		Request:        draft.Request,
		Target:         draft.Target,
		Contents:       draft.Contents,
		References:     draft.References,
		Markdown:       draft.Markdown,
		Status:         draft.Status,
		ApprovalState:  draft.ApprovalState,
		AutoApproved:   draft.AutoApproved,
	})
	if err != nil {
		return "", fmt.Errorf("encode draft revision: %w", err)
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}
