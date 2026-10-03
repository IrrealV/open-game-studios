// Package visual implements the metadata-only visual-workflow/v1 contracts
// described in openspec/specs/visual-workflow/spec.md.
//
// It reads human-authored Art Bible and Asset Spec markdown, validates the
// fields the contract requires, and produces a read-only readiness audit. It
// never executes ComfyUI or Blender, never generates images or models, never
// mutates engine/DCC files, and never covers audio/music/SFX. The exported
// types re-use the existing asset-manifest/v1 vocabulary in internal/assets
// rather than introducing a second schema.
package visual

import (
	"strings"

	"open-game-studios/internal/assets"
)

// Version is the visual workflow schema version shared with the asset
// contract vocabulary.
const Version = assets.VisualWorkflowVersion

// Boundary is the single human-readable statement of what this package does
// not do. It is emitted in every audit so a report can never imply execution.
const Boundary = "metadata-only visual direction and readiness; does not generate assets, approve documents, execute ComfyUI/Blender, mutate Godot/DCC files, or cover audio/music/SFX"

// Approval states re-use the asset contract vocabulary. A document is never
// approved by this package: approval is a human decision recorded separately.
const (
	ApprovalPending  = assets.VisualStatusPendingApproval
	ApprovalApproved = assets.VisualStatusApproved
)

// Statuses reported by the read-only audit.
const (
	// StatusConcerns means at least one required structural check failed.
	StatusConcerns = assets.VisualStatusConcerns
	// StatusReadyForMetadata means the documents are structurally complete but
	// the Art Bible is not yet human-approved, so only metadata-only adapter
	// planning may proceed.
	StatusReadyForMetadata = "ready_for_metadata_planning"
	// StatusReadyForAdapter means the documents are complete and the Art Bible
	// is human-approved, so metadata-only adapter planning is unblocked.
	StatusReadyForAdapter = assets.VisualStatusReadyForAdapter
)

// Canonical body section headings. Headings are normalised to lower-case and
// trimmed before lookup, so authoring style (Title Case, trailing spaces) does
// not change the parse.
const (
	SectionPillars          = "visual pillars"
	SectionStyle            = "style"
	SectionPalette          = "palette"
	SectionReferences       = "references"
	SectionConstraints      = "constraints"
	SectionNegativeGuidance = "negative guidance"

	SectionIntent         = "intent"
	SectionGameplayUse    = "gameplay use"
	SectionNarrativeUse   = "narrative use"
	SectionArtBibleLinks  = "art bible links"
	SectionOutputTargets  = "output targets"
	SectionPromptMetadata = "prompt metadata"
	SectionImportHints    = "import hints"
	SectionAcceptance     = "acceptance criteria"
)

// ArtBible is the parsed, human-authored visual direction document.
//
// The structural fields map one-to-one onto assets.VisualArtBibleContract.
type ArtBible struct {
	Path             string
	Version          string
	ApprovalState    string
	Pillars          []string
	Style            string
	Palette          []string
	References       []string
	Constraints      []string
	NegativeGuidance []string
}

// AssetSpec is the parsed per-asset specification.
//
// The structural fields map onto assets.VisualAssetSpecContract.
type AssetSpec struct {
	Path               string
	Version            string
	Asset              string
	ArtBibleLink       string
	ApprovalState      string
	Intent             string
	GameplayUse        string
	NarrativeUse       string
	ArtBibleLinks      []string
	OutputTargets      []string
	PromptMetadata     []string
	ImportHints        []string
	AcceptanceCriteria []string
}

// AuditCheck is one pass/fail line in the read-only readiness audit. A
// blocking check that fails prevents metadata readiness; a non-blocking check
// (including Art Bible approval) is reported but does not by itself make the
// documents structurally incomplete.
type AuditCheck struct {
	ID       string
	Label    string
	Passed   bool
	Blocking bool
	Detail   string
}

// AuditResult is the outcome of a read-only audit over one Art Bible and its
// Asset Specs. It carries no execution claim and never approves a document.
type AuditResult struct {
	Version          string
	Boundary         string
	Status           string
	MetadataReady    bool
	AdapterReady     bool
	ArtBiblePath     string
	ArtBibleApproval string
	SpecCount        int
	Checks           []AuditCheck
	BlockingNotes    []string
}

// IsApproved reports whether a recorded approval state means approved.
func IsApproved(state string) bool {
	return strings.EqualFold(strings.TrimSpace(state), ApprovalApproved)
}
