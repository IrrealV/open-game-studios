package visual

import (
	"fmt"
	"path/filepath"
	"strings"
)

// AuditPaths names the explicit documents for one read-only audit. The audit
// never scans for an artifact root and never walks parent directories.
type AuditPaths struct {
	// ArtBible is the path to the Art Bible markdown document.
	ArtBible string
	// SpecsDir is the directory that holds Asset Spec markdown documents.
	SpecsDir string
}

// AuditGameRoot builds the conventional paths for one game fixture root:
// <root>/art-bible.md and <root>/assets/*.md.
func AuditGameRoot(root string) AuditPaths {
	return AuditPaths{
		ArtBible: filepath.Join(root, "art-bible.md"),
		SpecsDir: filepath.Join(root, "assets"),
	}
}

// Audit performs the read-only visual-workflow/v1 audit over one Art Bible and
// its Asset Specs. It proves document alignment only: it never executes
// ComfyUI or Blender, never generates an image or model, never mutates an
// engine or DCC file, and never covers audio/music/SFX. It does not approve the
// Art Bible.
//
// The returned result is always populated, including on a partial failure, so
// the caller can report exactly which checks ran. A structural parse failure
// returns a non-nil error.
func Audit(paths AuditPaths) (AuditResult, error) {
	result := AuditResult{
		Version:      Version,
		Boundary:     Boundary,
		ArtBiblePath: paths.ArtBible,
	}
	if strings.TrimSpace(paths.ArtBible) == "" {
		return result, fmt.Errorf("visual: audit requires an Art Bible path")
	}
	if strings.TrimSpace(paths.SpecsDir) == "" {
		return result, fmt.Errorf("visual: audit requires an Asset Spec directory")
	}

	bible, err := ParseArtBible(paths.ArtBible)
	if err != nil {
		return result, err
	}
	result.ArtBibleApproval = bible.ApprovalState

	specs, err := LoadAssetSpecs(paths.SpecsDir)
	if err != nil {
		return result, err
	}
	result.SpecCount = len(specs)

	checks := []AuditCheck{
		artBibleFieldsCheck(bible),
		specsPresentCheck(specs, paths.SpecsDir),
		artBibleLinkCheck(specs, paths.ArtBible),
		outputTargetsCheck(specs),
		promptImportCheck(specs),
		acceptanceCheck(specs),
		approvalCheck(bible),
		executionBoundaryCheck(),
	}
	result.Checks = checks

	blocking := []string{}
	for _, check := range checks {
		if check.Blocking && !check.Passed {
			blocking = append(blocking, fmt.Sprintf("%s: %s", check.Label, check.Detail))
		}
	}

	result.MetadataReady = len(blocking) == 0
	result.AdapterReady = result.MetadataReady && IsApproved(bible.ApprovalState)
	switch {
	case !result.MetadataReady:
		result.Status = StatusConcerns
	case result.AdapterReady:
		result.Status = StatusReadyForAdapter
	default:
		result.Status = StatusReadyForMetadata
		blocking = append(blocking, "Art Bible approval is pending; metadata-only adapter planning may proceed, but adapter execution remains blocked until a human approves the Art Bible")
	}
	result.BlockingNotes = blocking
	return result, nil
}

func artBibleFieldsCheck(bible ArtBible) AuditCheck {
	notes := ValidateArtBible(bible)
	return AuditCheck{
		ID:       "art_bible_fields",
		Label:    "Art Bible has all required contract fields",
		Passed:   len(notes) == 0,
		Blocking: true,
		Detail:   detail(len(notes) == 0, "pillars, style, palette, references, constraints, negative guidance and approval state present", strings.Join(notes, "; ")),
	}
}

func specsPresentCheck(specs []AssetSpec, dir string) AuditCheck {
	return AuditCheck{
		ID:       "asset_specs_present",
		Label:    "Asset Spec documents are present",
		Passed:   len(specs) > 0,
		Blocking: true,
		Detail:   detail(len(specs) > 0, fmt.Sprintf("%d Asset Spec document(s) found in %s", len(specs), dir), fmt.Sprintf("no Asset Spec markdown found in %s", dir)),
	}
}

func artBibleLinkCheck(specs []AssetSpec, artBiblePath string) AuditCheck {
	notes := []string{}
	linked := 0
	for _, spec := range specs {
		if strings.TrimSpace(spec.ArtBibleLink) == "" {
			notes = append(notes, fmt.Sprintf("%s has no Art Bible link", specLabel(spec)))
			continue
		}
		if !SameDocument(resolveLink(spec.Path, spec.ArtBibleLink), artBiblePath) {
			notes = append(notes, fmt.Sprintf("%s links %q, which does not resolve to %q", specLabel(spec), spec.ArtBibleLink, artBiblePath))
			continue
		}
		if len(spec.ArtBibleLinks) == 0 {
			notes = append(notes, fmt.Sprintf("%s links the Art Bible in frontmatter but names no Art Bible links in its body", specLabel(spec)))
			continue
		}
		linked++
	}
	passed := len(specs) > 0 && linked == len(specs)
	return AuditCheck{
		ID:       "art_bible_links",
		Label:    "Every Asset Spec links the Art Bible",
		Passed:   passed,
		Blocking: true,
		Detail:   detail(passed, fmt.Sprintf("%d of %d Asset Spec(s) link %s", linked, len(specs), artBiblePath), strings.Join(notes, "; ")),
	}
}

func outputTargetsCheck(specs []AssetSpec) AuditCheck {
	notes := []string{}
	for _, spec := range specs {
		if len(spec.OutputTargets) == 0 {
			notes = append(notes, fmt.Sprintf("%s declares no output targets", specLabel(spec)))
		}
	}
	passed := len(specs) > 0 && len(notes) == 0
	return AuditCheck{
		ID:       "output_targets",
		Label:    "Every Asset Spec declares output targets",
		Passed:   passed,
		Blocking: true,
		Detail:   detail(passed, "metadata output targets declared for every Asset Spec", strings.Join(notes, "; ")),
	}
}

func promptImportCheck(specs []AssetSpec) AuditCheck {
	notes := []string{}
	for _, spec := range specs {
		if len(spec.PromptMetadata) == 0 {
			notes = append(notes, fmt.Sprintf("%s has no prompt-ready metadata", specLabel(spec)))
		}
		if len(spec.ImportHints) == 0 {
			notes = append(notes, fmt.Sprintf("%s has no import hints", specLabel(spec)))
		}
	}
	passed := len(specs) > 0 && len(notes) == 0
	return AuditCheck{
		ID:       "prompt_import_metadata",
		Label:    "Every Asset Spec carries prompt and import metadata",
		Passed:   passed,
		Blocking: true,
		Detail:   detail(passed, "prompt-ready metadata and import hints present for every Asset Spec", strings.Join(notes, "; ")),
	}
}

func acceptanceCheck(specs []AssetSpec) AuditCheck {
	notes := []string{}
	for _, spec := range specs {
		if len(spec.AcceptanceCriteria) == 0 {
			notes = append(notes, fmt.Sprintf("%s has no acceptance criteria", specLabel(spec)))
			continue
		}
		for _, criterion := range spec.AcceptanceCriteria {
			if !looksTestable(criterion) {
				notes = append(notes, fmt.Sprintf("%s acceptance criterion is not testable: %q", specLabel(spec), criterion))
			}
		}
	}
	passed := len(specs) > 0 && len(notes) == 0
	return AuditCheck{
		ID:       "acceptance_criteria",
		Label:    "Every Asset Spec has testable acceptance criteria",
		Passed:   passed,
		Blocking: true,
		Detail:   detail(passed, "testable acceptance criteria present for every Asset Spec", strings.Join(notes, "; ")),
	}
}

func approvalCheck(bible ArtBible) AuditCheck {
	approved := IsApproved(bible.ApprovalState)
	return AuditCheck{
		ID:       "art_bible_approval",
		Label:    "Art Bible human approval recorded",
		Passed:   approved,
		Blocking: false,
		Detail:   detail(approved, "Art Bible approval is recorded", "Art Bible approval is pending; the CLI cannot self-approve it"),
	}
}

func executionBoundaryCheck() AuditCheck {
	return AuditCheck{
		ID:       "execution_boundary",
		Label:    "Metadata-only boundary respected",
		Passed:   true,
		Blocking: true,
		Detail:   "read-only audit; no ComfyUI/Blender execution, no image/model generation, no Godot/DCC mutation, no audio/music/SFX claim",
	}
}

func detail(passed bool, ok, problem string) string {
	if passed {
		return ok
	}
	if strings.TrimSpace(problem) == "" {
		return "check failed"
	}
	return problem
}
