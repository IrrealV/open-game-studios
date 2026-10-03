package visual

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"open-game-studios/internal/assets"
)

// maxDocumentBytes bounds every document the evaluator reads. Art Bible and
// Asset Spec files are explicit, caller-owned markdown documents; the
// evaluator never scans for an artifact root and never walks parent
// directories.
const maxDocumentBytes = 1 << 20

// ParseArtBible reads and parses one Art Bible markdown document. It reads a
// single explicit path and never mutates it.
func ParseArtBible(path string) (ArtBible, error) {
	raw, err := readDocument(path)
	if err != nil {
		return ArtBible{}, err
	}
	frontmatter, body := splitFrontmatter(raw)
	meta := parseFrontmatter(frontmatter)
	sections := parseSections(body)

	bible := ArtBible{
		Path:             path,
		Version:          meta["version"],
		ApprovalState:    firstNonEmpty(meta["approval_state"], meta["approval"]),
		Pillars:          bullets(sections[SectionPillars]),
		Style:            firstParagraph(sections[SectionStyle]),
		Palette:          bullets(sections[SectionPalette]),
		References:       bullets(sections[SectionReferences]),
		Constraints:      bullets(sections[SectionConstraints]),
		NegativeGuidance: bullets(sections[SectionNegativeGuidance]),
	}
	if strings.TrimSpace(bible.Version) == "" {
		return ArtBible{}, fmt.Errorf("visual: %q has no version in its YAML frontmatter", path)
	}
	return bible, nil
}

// ParseAssetSpec reads and parses one Asset Spec markdown document. It reads a
// single explicit path and never mutates it.
func ParseAssetSpec(path string) (AssetSpec, error) {
	raw, err := readDocument(path)
	if err != nil {
		return AssetSpec{}, err
	}
	frontmatter, body := splitFrontmatter(raw)
	meta := parseFrontmatter(frontmatter)
	sections := parseSections(body)

	spec := AssetSpec{
		Path:               path,
		Version:            meta["version"],
		Asset:              firstNonEmpty(meta["asset"], meta["asset_name"]),
		ArtBibleLink:       firstNonEmpty(meta["art_bible"], meta["art-bible"]),
		ApprovalState:      firstNonEmpty(meta["approval_state"], meta["approval"]),
		Intent:             firstParagraph(sections[SectionIntent]),
		GameplayUse:        firstParagraph(sections[SectionGameplayUse]),
		NarrativeUse:       firstParagraph(sections[SectionNarrativeUse]),
		ArtBibleLinks:      bullets(sections[SectionArtBibleLinks]),
		OutputTargets:      bullets(sections[SectionOutputTargets]),
		PromptMetadata:     bullets(sections[SectionPromptMetadata]),
		ImportHints:        bullets(sections[SectionImportHints]),
		AcceptanceCriteria: bullets(sections[SectionAcceptance]),
	}
	if strings.TrimSpace(spec.Version) == "" {
		return AssetSpec{}, fmt.Errorf("visual: %q has no version in its YAML frontmatter", path)
	}
	if strings.TrimSpace(spec.Asset) == "" {
		return AssetSpec{}, fmt.Errorf("visual: %q has no asset name in its YAML frontmatter", path)
	}
	return spec, nil
}

// LoadAssetSpecs reads every markdown document in dir, sorted by file name so
// an audit report is deterministic.
func LoadAssetSpecs(dir string) ([]AssetSpec, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("visual: read asset spec directory %q: %w", dir, err)
	}
	specs := make([]AssetSpec, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			continue
		}
		spec, err := ParseAssetSpec(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		specs = append(specs, spec)
	}
	sort.Slice(specs, func(i, j int) bool { return specs[i].Path < specs[j].Path })
	return specs, nil
}

// ValidateArtBible returns one note per missing required contract field. An
// empty slice means every field the visual-workflow/v1 Art Bible contract
// requires is present. It never approves the document.
func ValidateArtBible(bible ArtBible) []string {
	notes := []string{}
	if strings.TrimSpace(bible.Version) == "" {
		notes = append(notes, "Art Bible is missing version")
	}
	if len(bible.Pillars) == 0 {
		notes = append(notes, "Art Bible is missing visual pillars")
	}
	if strings.TrimSpace(bible.Style) == "" {
		notes = append(notes, "Art Bible is missing style")
	}
	if len(bible.Palette) == 0 {
		notes = append(notes, "Art Bible is missing palette")
	}
	if len(bible.References) == 0 {
		notes = append(notes, "Art Bible is missing references")
	}
	if len(bible.Constraints) == 0 {
		notes = append(notes, "Art Bible is missing constraints")
	}
	if len(bible.NegativeGuidance) == 0 {
		notes = append(notes, "Art Bible is missing negative guidance")
	}
	if strings.TrimSpace(bible.ApprovalState) == "" {
		notes = append(notes, "Art Bible is missing approval state")
	}
	return notes
}

// ValidateAssetSpec returns one note per missing required contract field, plus
// any Art Bible linkage that does not resolve to artBiblePath. An empty slice
// means the spec satisfies the visual-workflow/v1 Asset Spec contract.
func ValidateAssetSpec(spec AssetSpec, artBiblePath string) []string {
	notes := []string{}
	label := specLabel(spec)
	if strings.TrimSpace(spec.Version) == "" {
		notes = append(notes, fmt.Sprintf("%s is missing version", label))
	}
	if strings.TrimSpace(spec.Intent) == "" {
		notes = append(notes, fmt.Sprintf("%s is missing asset intent", label))
	}
	if strings.TrimSpace(spec.GameplayUse) == "" {
		notes = append(notes, fmt.Sprintf("%s is missing gameplay use", label))
	}
	if strings.TrimSpace(spec.NarrativeUse) == "" {
		notes = append(notes, fmt.Sprintf("%s is missing narrative use", label))
	}
	if len(spec.ArtBibleLinks) == 0 {
		notes = append(notes, fmt.Sprintf("%s is missing Art Bible links", label))
	}
	if strings.TrimSpace(spec.ArtBibleLink) == "" {
		notes = append(notes, fmt.Sprintf("%s is missing an Art Bible link in its frontmatter", label))
	} else if artBiblePath != "" && !SameDocument(resolveLink(spec.Path, spec.ArtBibleLink), artBiblePath) {
		notes = append(notes, fmt.Sprintf("%s Art Bible link %q does not resolve to %q", label, spec.ArtBibleLink, artBiblePath))
	}
	if len(spec.OutputTargets) == 0 {
		notes = append(notes, fmt.Sprintf("%s is missing output targets", label))
	}
	if len(spec.PromptMetadata) == 0 {
		notes = append(notes, fmt.Sprintf("%s is missing prompt-ready metadata", label))
	}
	if len(spec.ImportHints) == 0 {
		notes = append(notes, fmt.Sprintf("%s is missing import hints", label))
	}
	if len(spec.AcceptanceCriteria) == 0 {
		notes = append(notes, fmt.Sprintf("%s is missing acceptance criteria", label))
	}
	for _, criterion := range spec.AcceptanceCriteria {
		if !looksTestable(criterion) {
			notes = append(notes, fmt.Sprintf("%s acceptance criterion is not testable: %q", label, criterion))
		}
	}
	return notes
}

// SameDocument reports whether two paths name the same document. Comparison is
// best-effort absolute-path equality; a relative link is resolved against the
// document that contains it before this call.
func SameDocument(a, b string) bool {
	if strings.TrimSpace(a) == "" || strings.TrimSpace(b) == "" {
		return false
	}
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA == nil && errB == nil {
		return filepath.Clean(absA) == filepath.Clean(absB)
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

func specLabel(spec AssetSpec) string {
	if strings.TrimSpace(spec.Asset) != "" {
		return "Asset Spec " + strconv.Quote(spec.Asset)
	}
	return "Asset Spec " + strconv.Quote(filepath.Base(spec.Path))
}

// resolveLink resolves a spec's declared Art Bible link against the spec's
// directory. Absolute links are returned cleaned.
func resolveLink(specPath, link string) string {
	link = strings.TrimSpace(link)
	if filepath.IsAbs(link) {
		return filepath.Clean(link)
	}
	return filepath.Join(filepath.Dir(specPath), link)
}

func readDocument(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("visual: stat %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("visual: %q is not a regular file", path)
	}
	if info.Size() > maxDocumentBytes {
		return "", fmt.Errorf("visual: %q exceeds the %d-byte document limit", path, maxDocumentBytes)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("visual: read %q: %w", path, err)
	}
	return string(data), nil
}

// splitFrontmatter separates a leading YAML frontmatter block delimited by
// lines of exactly `---` from the markdown body. A document without a leading
// frontmatter block yields an empty frontmatter and the whole input as body.
func splitFrontmatter(raw string) (string, string) {
	lines := splitLines(raw)
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", raw
	}
	for idx := 1; idx < len(lines); idx++ {
		if strings.TrimSpace(lines[idx]) == "---" {
			return strings.Join(lines[1:idx], "\n"), strings.Join(lines[idx+1:], "\n")
		}
	}
	// Unterminated frontmatter: treat the whole document as body so a missing
	// closing delimiter surfaces as missing fields rather than silent success.
	return "", raw
}

// parseFrontmatter parses flat `key: value` lines. Nested YAML is out of scope:
// the contract stores rich content in body sections, and an unrecognised line
// is ignored rather than guessed at.
func parseFrontmatter(frontmatter string) map[string]string {
	values := map[string]string{}
	for _, line := range splitLines(frontmatter) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		key, value, found := strings.Cut(trimmed, ":")
		if !found {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		value = strings.TrimPrefix(value, "\"")
		value = strings.TrimSuffix(value, "\"")
		value = strings.TrimPrefix(value, "'")
		value = strings.TrimSuffix(value, "'")
		values[key] = value
	}
	return values
}

// parseSections maps each lower-case `## heading` to the raw lines that follow
// it up to the next level-two heading.
func parseSections(body string) map[string][]string {
	sections := map[string][]string{}
	current := ""
	for _, line := range splitLines(body) {
		if strings.HasPrefix(strings.TrimSpace(line), "## ") {
			current = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "## ")))
			if _, ok := sections[current]; !ok {
				sections[current] = []string{}
			}
			continue
		}
		if current == "" {
			continue
		}
		sections[current] = append(sections[current], line)
	}
	return sections
}

// bullets extracts markdown list items (`- `, `* `, `N. `) from section lines.
func bullets(lines []string) []string {
	items := []string{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "- "), strings.HasPrefix(trimmed, "* "):
			item := strings.TrimSpace(trimmed[2:])
			if item != "" {
				items = append(items, item)
			}
		default:
			if item, ok := numberedItem(trimmed); ok {
				items = append(items, item)
			}
		}
	}
	return items
}

func numberedItem(trimmed string) (string, bool) {
	dot := strings.Index(trimmed, ". ")
	if dot <= 0 {
		return "", false
	}
	if _, err := strconv.Atoi(trimmed[:dot]); err != nil {
		return "", false
	}
	item := strings.TrimSpace(trimmed[dot+2:])
	if item == "" {
		return "", false
	}
	return item, true
}

// firstParagraph returns the first blank-line-delimited paragraph of a
// section, with bullet list items excluded.
func firstParagraph(lines []string) string {
	parts := []string{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if len(parts) > 0 {
				break
			}
			continue
		}
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			continue
		}
		if _, ok := numberedItem(trimmed); ok {
			continue
		}
		parts = append(parts, trimmed)
	}
	return strings.Join(parts, " ")
}

// looksTestable reports whether an acceptance criterion carries a checkable
// signal. The heuristic is deliberately permissive: it exists to catch prose
// that cannot be verified, not to replace human review.
func looksTestable(criterion string) bool {
	text := strings.ToLower(strings.TrimSpace(criterion))
	if text == "" {
		return false
	}
	for _, token := range []string{"#", "<=", ">=", "≤", "≥", "=", "within", "at most", "at least", "distance", "exists", "visible", "trigger"} {
		if strings.Contains(text, token) {
			return true
		}
	}
	for _, r := range text {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func splitLines(raw string) []string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	return strings.Split(raw, "\n")
}

// AsWorkflowContract maps parsed documents onto the existing
// assets.VisualWorkflowContract so adapter readiness consumes the accepted
// asset-pipeline contract instead of a parallel schema. Multi-spec fields are
// aggregated; the Art Bible approval state and audit status are carried
// through unchanged. It never derives an approval this package did not read.
func AsWorkflowContract(bible ArtBible, specs []AssetSpec, auditStatus string) assets.VisualWorkflowContract {
	workflow := assets.DefaultVisualWorkflow()
	workflow.ArtBible.Path = bible.Path
	workflow.ArtBible.Status = auditStatus
	workflow.ArtBible.ApprovalState = bible.ApprovalState
	workflow.ArtBible.Pillars = append([]string{}, bible.Pillars...)
	workflow.ArtBible.Style = bible.Style
	workflow.ArtBible.Palette = append([]string{}, bible.Palette...)
	workflow.ArtBible.References = append([]string{}, bible.References...)
	workflow.ArtBible.Constraints = append([]string{}, bible.Constraints...)
	workflow.ArtBible.NegativeGuidance = append([]string{}, bible.NegativeGuidance...)

	workflow.AssetSpec.Status = auditStatus
	if len(specs) > 0 {
		workflow.AssetSpec.PathPrefix = filepath.Dir(specs[0].Path)
		workflow.AssetSpec.ArtBibleLink = specs[0].ArtBibleLink
	}
	for _, spec := range specs {
		workflow.AssetSpec.OutputTargets = appendUnique(workflow.AssetSpec.OutputTargets, spec.OutputTargets...)
		workflow.AssetSpec.PromptReadyFields = appendUnique(workflow.AssetSpec.PromptReadyFields, spec.PromptMetadata...)
		workflow.AssetSpec.ImportHints = appendUnique(workflow.AssetSpec.ImportHints, spec.ImportHints...)
		workflow.AssetSpec.AcceptanceCriteria = appendUnique(workflow.AssetSpec.AcceptanceCriteria, spec.AcceptanceCriteria...)
		if workflow.AssetSpec.AssetIntent == "" {
			workflow.AssetSpec.AssetIntent = spec.Intent
		}
	}
	workflow.Audit.Status = auditStatus
	return workflow
}

func appendUnique(base []string, values ...string) []string {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		duplicate := false
		for _, existing := range base {
			if existing == value {
				duplicate = true
				break
			}
		}
		if !duplicate {
			base = append(base, value)
		}
	}
	return base
}

// ReadDocumentForApproval reads a document and returns its raw text. It is the
// read half of the explicit human-approval rewrite; it never mutates the file.
func ReadDocumentForApproval(path string) (string, error) {
	return readDocument(path)
}

// ApprovalRewrite returns the document text with approval_state set to
// approved and approved_by set to reviewer. It performs no write: the caller
// owns when and where the updated text is persisted. It never authenticates
// the reviewer; it records a claimed human decision only.
func ApprovalRewrite(path, reviewer string) (string, error) {
	raw, err := readDocument(path)
	if err != nil {
		return "", err
	}
	frontmatter, body := splitFrontmatter(raw)
	if strings.TrimSpace(frontmatter) == "" {
		return "", fmt.Errorf("visual: %q has no YAML frontmatter to update", path)
	}

	lines := splitLines(frontmatter)
	updated := make([]string, 0, len(lines)+2)
	seenState := false
	seenReviewer := false
	for _, line := range lines {
		switch frontmatterKey(line) {
		case "approval_state", "approval":
			updated = append(updated, "approval_state: "+ApprovalApproved)
			seenState = true
		case "approved_by":
			updated = append(updated, "approved_by: "+strconv.Quote(strings.TrimSpace(reviewer)))
			seenReviewer = true
		default:
			updated = append(updated, line)
		}
	}
	if !seenState {
		updated = append(updated, "approval_state: "+ApprovalApproved)
	}
	if !seenReviewer {
		updated = append(updated, "approved_by: "+strconv.Quote(strings.TrimSpace(reviewer)))
	}

	trimmedBody := strings.Trim(body, "\n")
	rendered := "---\n" + strings.Join(updated, "\n") + "\n---\n"
	if trimmedBody != "" {
		rendered += "\n" + trimmedBody + "\n"
	}
	return rendered, nil
}

// frontmatterKey returns the lower-case key of a flat `key: value` line, or an
// empty string when the line is not a key/value pair.
func frontmatterKey(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return ""
	}
	key, _, found := strings.Cut(trimmed, ":")
	if !found {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(key))
}
