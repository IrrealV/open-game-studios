package commands

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"open-game-studios/internal/workflows/visual"
)

// VisualInput is the narrowed input seam for `game-studio visual`. Stdout is
// injectable for tests; a nil writer falls back to os.Stdout.
type VisualInput struct {
	Args   []string
	Stdout io.Writer
}

// RunVisual dispatches the explicit visual subcommands. It is a thin adapter
// over internal/workflows/visual: it reads or writes exactly the path the
// caller names, validates the visual-workflow/v1 documents, and runs the
// read-only audit. It never executes ComfyUI or Blender, never generates an
// image or model, never mutates an engine/DCC file, and never approves an Art
// Bible on its own.
func RunVisual(input VisualInput) error {
	stdout := input.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	if len(input.Args) == 0 {
		return errors.New(visualUsage())
	}
	switch input.Args[0] {
	case "art-bible":
		return runVisualArtBible(input.Args[1:], stdout)
	case "asset-spec":
		return runVisualAssetSpec(input.Args[1:], stdout)
	case "audit":
		return runVisualAudit(input.Args[1:], stdout)
	case "approve-art-bible":
		return runVisualApproveArtBible(input.Args[1:], stdout)
	default:
		return fmt.Errorf("unknown visual subcommand %q\n\n%s", input.Args[0], visualUsage())
	}
}

// runVisualArtBible validates an existing Art Bible, or writes a pending
// template when the path does not exist yet. The template is always
// approval_state: pending; this command never records approval.
func runVisualArtBible(args []string, stdout io.Writer) error {
	set := flag.NewFlagSet("visual art-bible", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	if err := set.Parse(args); err != nil {
		return fmt.Errorf("visual art-bible: %w\n\n%s", err, visualUsage())
	}
	if set.NArg() != 1 {
		return fmt.Errorf("visual art-bible: expected exactly one path, got %v\n\n%s", set.Args(), visualUsage())
	}
	path := set.Arg(0)

	bible, err := visual.ParseArtBible(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			if writeErr := writeVisualTemplate(path, artBibleTemplate()); writeErr != nil {
				return writeErr
			}
			return writeReport(stdout, []string{
				fmt.Sprintf("[visual] art bible template written: %q\n", path),
				"[visual] approval_state: pending\n",
				"[visual] note: replace every placeholder, then have a human approve it; this command never approves.\n",
			})
		}
		return err
	}

	notes := visual.ValidateArtBible(bible)
	if len(notes) > 0 {
		return fmt.Errorf("visual art-bible: %q is not contract-complete: %s", path, strings.Join(notes, "; "))
	}
	return writeReport(stdout, []string{
		fmt.Sprintf("[visual] art bible valid: %q\n", path),
		fmt.Sprintf("[visual] version: %s\n", bible.Version),
		fmt.Sprintf("[visual] approval_state: %s\n", bible.ApprovalState),
		fmt.Sprintf("[visual] pillars: %d, palette: %d, references: %d, constraints: %d, negative_guidance: %d\n",
			len(bible.Pillars), len(bible.Palette), len(bible.References), len(bible.Constraints), len(bible.NegativeGuidance)),
		fmt.Sprintf("[visual] style: %s\n", bible.Style),
		"[visual] note: validation only; no approval is recorded or implied.\n",
	})
}

// runVisualAssetSpec validates an existing Asset Spec, or writes a pending
// template under the named directory when the spec does not exist yet.
func runVisualAssetSpec(args []string, stdout io.Writer) error {
	set := flag.NewFlagSet("visual asset-spec", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	if err := set.Parse(args); err != nil {
		return fmt.Errorf("visual asset-spec: %w\n\n%s", err, visualUsage())
	}
	if set.NArg() != 2 {
		return fmt.Errorf("visual asset-spec: expected <assets-dir> <asset-name>, got %v\n\n%s", set.Args(), visualUsage())
	}
	dir := set.Arg(0)
	name := set.Arg(1)
	if err := validateAssetName(name); err != nil {
		return err
	}
	path := filepath.Join(dir, name+".md")

	spec, err := visual.ParseAssetSpec(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			if writeErr := writeVisualTemplate(path, assetSpecTemplate(name)); writeErr != nil {
				return writeErr
			}
			return writeReport(stdout, []string{
				fmt.Sprintf("[visual] asset spec template written: %q\n", path),
				fmt.Sprintf("[visual] asset: %s\n", name),
				"[visual] approval_state: pending\n",
				"[visual] note: link the Art Bible, declare output targets and testable acceptance criteria; this command never approves.\n",
			})
		}
		return err
	}

	// The Art Bible path is unknown to this command, so linkage is required but
	// its resolution is checked by `visual audit`, which knows the Art Bible.
	notes := visual.ValidateAssetSpec(spec, "")
	if len(notes) > 0 {
		return fmt.Errorf("visual asset-spec: %q is not contract-complete: %s", path, strings.Join(notes, "; "))
	}
	return writeReport(stdout, []string{
		fmt.Sprintf("[visual] asset spec valid: %q\n", path),
		fmt.Sprintf("[visual] asset: %s\n", spec.Asset),
		fmt.Sprintf("[visual] art_bible_link: %s\n", spec.ArtBibleLink),
		fmt.Sprintf("[visual] output_targets: %d, import_hints: %d, acceptance_criteria: %d\n",
			len(spec.OutputTargets), len(spec.ImportHints), len(spec.AcceptanceCriteria)),
		"[visual] note: validation only; Art Bible linkage resolution is proven by `game-studio visual audit`.\n",
	})
}

// runVisualAudit runs the read-only audit over <game-root>/art-bible.md and
// <game-root>/assets/*.md and prints every check. It never executes an
// adapter and never writes a file.
func runVisualAudit(args []string, stdout io.Writer) error {
	set := flag.NewFlagSet("visual audit", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	if err := set.Parse(args); err != nil {
		return fmt.Errorf("visual audit: %w\n\n%s", err, visualUsage())
	}
	if set.NArg() != 1 {
		return fmt.Errorf("visual audit: expected exactly one game-root path, got %v\n\n%s", set.Args(), visualUsage())
	}
	root := set.Arg(0)

	result, err := visual.Audit(visual.AuditGameRoot(root))
	if err != nil {
		return fmt.Errorf("visual audit: %w", err)
	}

	lines := []string{
		fmt.Sprintf("[visual] audit: %q\n", root),
		"[visual] mode: read-only\n",
		fmt.Sprintf("[visual] art_bible: %q (approval: %s)\n", result.ArtBiblePath, result.ArtBibleApproval),
		fmt.Sprintf("[visual] boundary: %s\n", result.Boundary),
	}
	for _, check := range result.Checks {
		state := "pass"
		if !check.Passed {
			state = "concern"
		}
		lines = append(lines, fmt.Sprintf("[visual] check %s: %s — %s\n", check.ID, state, check.Detail))
	}
	lines = append(lines,
		fmt.Sprintf("[visual] asset_specs: %d\n", result.SpecCount),
		fmt.Sprintf("[visual] metadata_ready: %t\n", result.MetadataReady),
		fmt.Sprintf("[visual] adapter_ready: %t\n", result.AdapterReady),
		fmt.Sprintf("[visual] status: %s\n", result.Status),
	)
	for _, note := range result.BlockingNotes {
		lines = append(lines, fmt.Sprintf("[visual] note: %s\n", note))
	}
	lines = append(lines, "[visual] boundary note: no ComfyUI/Blender execution, no image/model generation, no Godot/DCC mutation, no audio scope.\n")
	if err := writeReport(stdout, lines); err != nil {
		return fmt.Errorf("visual audit: report could not be delivered: %w", err)
	}

	if result.Status == visual.StatusConcerns {
		return fmt.Errorf("visual audit: documents are not ready: %s", strings.Join(result.BlockingNotes, "; "))
	}
	return nil
}

// runVisualApproveArtBible records an explicit human approval. It refuses to
// run without --confirm and a nonempty --reviewer so the CLI can never
// self-approve an Art Bible. It records a claimed decision only; it does not
// authenticate the reviewer.
func runVisualApproveArtBible(args []string, stdout io.Writer) error {
	path, confirm, reviewer, err := parseApprovalArgs(args)
	if err != nil {
		return err
	}
	if !confirm {
		return fmt.Errorf("visual approve-art-bible: refusing to approve without --confirm; the CLI cannot self-approve an Art Bible")
	}
	if strings.TrimSpace(reviewer) == "" {
		return fmt.Errorf("visual approve-art-bible: --reviewer must name the human recording the approval")
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("visual approve-art-bible: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("visual approve-art-bible: %q is not a regular file", path)
	}
	updated, err := visual.ApprovalRewrite(path, reviewer)
	if err != nil {
		return fmt.Errorf("visual approve-art-bible: %w", err)
	}
	if err := os.WriteFile(path, []byte(updated), info.Mode().Perm()); err != nil {
		return fmt.Errorf("visual approve-art-bible: write %q: %w", path, err)
	}
	return writeReport(stdout, []string{
		fmt.Sprintf("[visual] art bible approval recorded: %q\n", path),
		fmt.Sprintf("[visual] approval_state: %s\n", visual.ApprovalApproved),
		fmt.Sprintf("[visual] approved_by: %q\n", reviewer),
		"[visual] note: this records a claimed human decision; it does not authenticate the reviewer or grant execution authority.\n",
	})
}

// parseApprovalArgs accepts the Art Bible path and the approval flags in any
// order, because the standard flag package stops at the first positional. It
// rejects an unknown flag rather than silently ignoring it.
func parseApprovalArgs(args []string) (path string, confirm bool, reviewer string, err error) {
	positionals := []string{}
	for idx := 0; idx < len(args); idx++ {
		arg := args[idx]
		switch {
		case arg == "--confirm" || arg == "-confirm":
			confirm = true
		case arg == "--reviewer" || arg == "-reviewer":
			if idx+1 >= len(args) {
				return "", false, "", fmt.Errorf("visual approve-art-bible: %s requires a value\n\n%s", arg, visualUsage())
			}
			idx++
			reviewer = args[idx]
		case strings.HasPrefix(arg, "--reviewer=") || strings.HasPrefix(arg, "-reviewer="):
			_, value, _ := strings.Cut(arg, "=")
			reviewer = value
		case arg == "--help" || arg == "-h":
			return "", false, "", errors.New(visualUsage())
		case strings.HasPrefix(arg, "-") && arg != "-":
			return "", false, "", fmt.Errorf("visual approve-art-bible: unknown flag %q\n\n%s", arg, visualUsage())
		default:
			positionals = append(positionals, arg)
		}
	}
	if len(positionals) != 1 {
		return "", false, "", fmt.Errorf("visual approve-art-bible: expected exactly one path, got %v\n\n%s", positionals, visualUsage())
	}
	return positionals[0], confirm, reviewer, nil
}

func validateAssetName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("visual asset-spec: asset name must not be empty")
	}
	if strings.ContainsAny(name, `/\`) || name == "." || name == ".." || strings.Contains(name, "..") {
		return fmt.Errorf("visual asset-spec: asset name %q must be a plain file name without path separators", name)
	}
	return nil
}

// writeVisualTemplate creates a pending template at an explicit path and never
// overwrites existing state.
func writeVisualTemplate(path, content string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		switch {
		case errors.Is(err, fs.ErrExist):
			return fmt.Errorf("visual: refusing to overwrite existing %q", path)
		case errors.Is(err, fs.ErrNotExist):
			return fmt.Errorf("visual: parent directory for %q does not exist; the CLI does not create it", path)
		default:
			return fmt.Errorf("visual: create %q: %w", path, err)
		}
	}
	if _, err := file.WriteString(content); err != nil {
		_ = file.Close()
		return fmt.Errorf("visual: wrote an incomplete template at %q: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("visual: wrote an incomplete template at %q: %w", path, err)
	}
	return nil
}

func artBibleTemplate() string {
	return `---
version: 1
approval_state: pending
---

# Art Bible — <game name>

> Human-authored visual direction. Replace every placeholder, then have a human
> approve it. This template is deliberately pending and must not be treated as
> approval.

## Visual Pillars

- <pillar one>
- <pillar two>

## Style

<one short paragraph on shape language, materials, and rendering style.>

## Palette

- <role>: <name> (<#RRGGBB>)

## References

- <reference used as direction, not as a copied asset.>

## Constraints

- <scope limit, engine limit, or budget.>

## Negative Guidance

- <what must not appear.>
`
}

func assetSpecTemplate(name string) string {
	return fmt.Sprintf(`---
version: 1
asset: %s
art_bible: ../art-bible.md
approval_state: pending
---

# Asset Spec — %s

## Intent

<what this asset represents.>

## Gameplay Use

<how the game uses it.>

## Narrative Use

<narrative role, or "None — non-narrative asset".>

## Art Bible Links

- <pillar or constraint from the Art Bible this asset follows.>

## Output Targets

- <target output, e.g. an engine primitive or a future exported asset.>

## Prompt Metadata

- <prompt-ready metadata for a future adapter; metadata only, not executed.>

## Import Hints

- <future import hint; no engine mutation is performed here.>

## Acceptance Criteria

- <testable criterion with a measurable value.>
`, name, name)
}

func visualUsage() string {
	return `visual: metadata-only Art Bible and Asset Spec direction plus a read-only audit

Usage:
  game-studio visual art-bible <art-bible.md>
  game-studio visual asset-spec <assets-dir> <asset-name>
  game-studio visual audit <game-root>
  game-studio visual approve-art-bible <art-bible.md> --confirm --reviewer <name>

Subcommands:
  art-bible            Validate an existing Art Bible, or write a pending
                       template when the path does not exist yet. It never
                       records approval.
  asset-spec           Validate an existing Asset Spec, or write a pending
                       template when the file does not exist yet. Art Bible
                       linkage is resolved by the audit.
  audit                Read-only audit of <game-root>/art-bible.md and
                       <game-root>/assets/*.md. It executes no adapter.
  approve-art-bible    Record an explicit human approval. Requires --confirm
                       and --reviewer; the CLI cannot self-approve.

Boundary:
  Metadata only. No ComfyUI or Blender execution, no image/model generation,
  no Godot/DCC mutation, and no audio/music/SFX. A recorded approval is a
  claimed human decision, not authentication or execution authority.`
}
