package commands

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"open-game-studios/internal/workflows/coregame"
)

func briefSHA256(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func briefWriteJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal fixture %s: %v", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create fixture dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", path, err)
	}
}

// briefWriteRef writes a synthetic reference artifact under root and returns the
// matching coregame.ArtifactReference with the real content digest.
func briefWriteRef(t *testing.T, root, locator, content string) coregame.ArtifactReference {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(locator))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("create reference dir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write reference %s: %v", locator, err)
	}
	return coregame.ArtifactReference{Phase: coregame.PhaseGDDSlice, Locator: locator, Revision: briefSHA256(content)}
}

func runBriefForTest(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var stdout bytes.Buffer
	err := RunBrief(BriefInput{Args: args, Stdout: &stdout})
	return stdout.String(), err
}

func readDraftArtifact(t *testing.T, path string) coregame.Draft {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read draft artifact %s: %v", path, err)
	}
	var draft coregame.Draft
	if err := json.Unmarshal(data, &draft); err != nil {
		t.Fatalf("decode draft artifact %s: %v", path, err)
	}
	return draft
}

func listFiles(t *testing.T, root string) []string {
	t.Helper()
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return paths
}

func briefGDDContents() []coregame.SectionContent {
	return []coregame.SectionContent{
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

func briefChangeContents() []coregame.SectionContent {
	return []coregame.SectionContent{
		{Section: "requested change", Body: "Shift the synthetic loop."},
		{Section: "intended outcome", Body: "A clearer synthetic loop."},
		{Section: "affected systems/files", Body: "unknown at this stage."},
		{Section: "proposed options", Body: "Option A only."},
		{Section: "risks", Body: "Synthetic risk."},
		{Section: "acceptance criteria", Body: "The loop is clear."},
		{Section: "verification plan", Body: "Run focused synthetic checks."},
	}
}

func briefRepairContents() []coregame.SectionContent {
	return []coregame.SectionContent{
		{Section: "observed behavior", Body: "The synthetic loop stalls."},
		{Section: "expected behavior", Body: "The synthetic loop advances."},
		{Section: "affected systems/files", Body: "unknown at this stage."},
		{Section: "proposed fix options", Body: "Option A only."},
		{Section: "risks", Body: "Synthetic risk."},
		{Section: "acceptance criteria", Body: "The loop advances."},
		{Section: "verification plan", Body: "Run focused synthetic checks."},
	}
}

func briefChangeRequest(ref coregame.ArtifactReference) coregame.DraftRequest {
	return coregame.DraftRequest{
		Mode:           "repair/change handoff",
		Phase:          coregame.PhaseChangeBrief,
		Request:        "Adjust the synthetic loop.",
		Classification: coregame.ClassificationDesignChange,
		Target:         "Godot handoff",
		Contents:       briefChangeContents(),
		References:     []coregame.ArtifactReference{ref},
	}
}

func briefWithBody(contents []coregame.SectionContent, section, body string) []coregame.SectionContent {
	out := make([]coregame.SectionContent, len(contents))
	copy(out, contents)
	for i := range out {
		if out[i].Section == section {
			out[i].Body = body
		}
	}
	return out
}

// briefFailingWriter always fails, so report-delivery error paths are testable.
type briefFailingWriter struct{ err error }

func (w briefFailingWriter) Write([]byte) (int, error) { return 0, w.err }

// briefBuildDraft builds a draft fixture through the accepted library using a
// plain directory read seam. The CLI path is tested separately through os.Root.
func briefBuildDraft(t *testing.T, root string, request coregame.DraftRequest) coregame.Draft {
	t.Helper()
	draft, err := coregame.BuildDraft(request, os.DirFS(root))
	if err != nil {
		t.Fatalf("build draft fixture: %v", err)
	}
	return draft
}

func briefApprovedDecision(draft coregame.Draft) coregame.HumanDecision {
	return coregame.HumanDecision{
		Reference:     "synthetic recorded decision",
		State:         coregame.DecisionApproved,
		Target:        draft.Target,
		DraftRevision: draft.Revision,
	}
}

func TestRunBriefDraft_CreatesPendingArtifactForEveryEntryIntent(t *testing.T) {
	cases := []struct {
		name      string
		request   func(root string) coregame.DraftRequest
		wantKind  coregame.EntryKind
		wantPhase coregame.PhaseID
	}{
		{
			name: "creation gdd slice",
			request: func(string) coregame.DraftRequest {
				return coregame.DraftRequest{
					Mode:     "zero-to-one creation",
					Request:  "Draft a synthetic slice.",
					Target:   "Godot handoff",
					Contents: briefGDDContents(),
				}
			},
			wantKind:  coregame.EntryCreation,
			wantPhase: coregame.PhaseGDDSlice,
		},
		{
			name: "direct phase without template",
			request: func(string) coregame.DraftRequest {
				return coregame.DraftRequest{
					Mode:     "direct phase invocation",
					Phase:    coregame.PhaseCoreLoop,
					Request:  "Revise the synthetic loop only.",
					Contents: []coregame.SectionContent{{Section: "loop notes", Body: "Tighten the synthetic loop."}},
				}
			},
			wantKind:  coregame.EntryDirectPhase,
			wantPhase: coregame.PhaseCoreLoop,
		},
		{
			name: "change brief",
			request: func(root string) coregame.DraftRequest {
				return briefChangeRequest(briefWriteRef(t, root, "artifacts/gdd-slice.md", "approved synthetic slice"))
			},
			wantKind:  coregame.EntryChange,
			wantPhase: coregame.PhaseChangeBrief,
		},
		{
			name: "repair brief",
			request: func(root string) coregame.DraftRequest {
				ref := briefWriteRef(t, root, "artifacts/core-loop.md", "approved synthetic loop")
				ref.Phase = coregame.PhaseCoreLoop
				return coregame.DraftRequest{
					Mode:           "repair/change handoff",
					Phase:          coregame.PhaseRepairBrief,
					Request:        "Investigate the synthetic stall.",
					Classification: coregame.ClassificationImplementationBug,
					Target:         "Godot handoff",
					Contents:       briefRepairContents(),
					References:     []coregame.ArtifactReference{ref},
				}
			},
			wantKind:  coregame.EntryRepair,
			wantPhase: coregame.PhaseRepairBrief,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			request := tc.request(root)
			requestPath := filepath.Join(root, "request.json")
			briefWriteJSON(t, requestPath, request)

			stdout, err := runBriefForTest(t, "draft", "--input", requestPath, "--root", root, "--out", "draft.json")
			if err != nil {
				t.Fatalf("RunBrief draft: %v", err)
			}

			outPath := filepath.Join(root, "draft.json")
			draft := readDraftArtifact(t, outPath)
			if draft.Kind != tc.wantKind || draft.Phase != tc.wantPhase {
				t.Fatalf("kind/phase = %q/%q, want %q/%q", draft.Kind, draft.Phase, tc.wantKind, tc.wantPhase)
			}
			if draft.Status != "draft" || draft.ApprovalState != "pending_human_approval" || draft.AutoApproved {
				t.Fatalf("artifact is not pending: status=%q approval=%q auto=%t", draft.Status, draft.ApprovalState, draft.AutoApproved)
			}
			if draft.Revision == "" || draft.Markdown == "" {
				t.Fatalf("artifact is missing canonical markdown or revision: revision=%q markdown=%d bytes", draft.Revision, len(draft.Markdown))
			}
			for _, token := range []string{"status: draft", "approval_state: pending_human_approval", "auto_approved: false", "revision: " + draft.Revision} {
				if !strings.Contains(stdout, token) {
					t.Fatalf("success report missing %q, got:\n%s", token, stdout)
				}
			}
		})
	}
}

func TestRunBriefDraft_PersistsCanonicalRevisionDeterministically(t *testing.T) {
	root := t.TempDir()
	ref := briefWriteRef(t, root, "artifacts/gdd-slice.md", "approved synthetic slice")
	requestPath := filepath.Join(root, "request.json")
	briefWriteJSON(t, requestPath, briefChangeRequest(ref))

	for _, out := range []string{"first.json", "second.json"} {
		if _, err := runBriefForTest(t, "draft", "--input", requestPath, "--root", root, "--out", out); err != nil {
			t.Fatalf("RunBrief draft %s: %v", out, err)
		}
	}
	first := readDraftArtifact(t, filepath.Join(root, "first.json"))
	second := readDraftArtifact(t, filepath.Join(root, "second.json"))
	if first.Revision != second.Revision || first.Markdown != second.Markdown {
		t.Fatalf("identical requests produced different artifacts: %q vs %q", first.Revision, second.Revision)
	}
}

func TestRunBriefDraft_RefusesExistingOutputWithoutOverwrite(t *testing.T) {
	request := coregame.DraftRequest{
		Mode:     "zero-to-one creation",
		Request:  "Draft a synthetic slice.",
		Target:   "Godot handoff",
		Contents: briefGDDContents(),
	}

	t.Run("existing regular file preserved", func(t *testing.T) {
		root := t.TempDir()
		requestPath := filepath.Join(root, "request.json")
		briefWriteJSON(t, requestPath, request)
		outPath := filepath.Join(root, "draft.json")
		if err := os.WriteFile(outPath, []byte("original bytes"), 0o644); err != nil {
			t.Fatalf("seed existing output: %v", err)
		}

		_, err := runBriefForTest(t, "draft", "--input", requestPath, "--root", root, "--out", "draft.json")
		if err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
			t.Fatalf("expected a refusal to overwrite, got %v", err)
		}
		if data, readErr := os.ReadFile(outPath); readErr != nil || string(data) != "original bytes" {
			t.Fatalf("existing output changed: data=%q err=%v", data, readErr)
		}
	})

	t.Run("dangling symlink preserved", func(t *testing.T) {
		root := t.TempDir()
		requestPath := filepath.Join(root, "request.json")
		briefWriteJSON(t, requestPath, request)
		linkPath := filepath.Join(root, "draft.json")
		if err := os.Symlink("missing-target.json", linkPath); err != nil {
			t.Fatalf("seed dangling symlink: %v", err)
		}

		_, err := runBriefForTest(t, "draft", "--input", requestPath, "--root", root, "--out", "draft.json")
		if err == nil {
			t.Fatalf("expected a refusal to write through a dangling symlink")
		}
		info, lstatErr := os.Lstat(linkPath)
		if lstatErr != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("dangling symlink was removed or replaced: info=%v err=%v", info, lstatErr)
		}
	})

	t.Run("missing output directory is not created", func(t *testing.T) {
		root := t.TempDir()
		requestPath := filepath.Join(root, "request.json")
		briefWriteJSON(t, requestPath, request)

		_, err := runBriefForTest(t, "draft", "--input", requestPath, "--root", root, "--out", "nested/draft.json")
		if err == nil || !strings.Contains(err.Error(), "does not exist") {
			t.Fatalf("expected a missing-directory error, got %v", err)
		}
		if _, statErr := os.Stat(filepath.Join(root, "nested")); !os.IsNotExist(statErr) {
			t.Fatalf("nested output directory must not be created, stat err=%v", statErr)
		}
	})
}

func TestRunBriefDraft_RejectsUnsafeOutputLocators(t *testing.T) {
	request := coregame.DraftRequest{
		Mode:     "zero-to-one creation",
		Request:  "Draft a synthetic slice.",
		Target:   "Godot handoff",
		Contents: briefGDDContents(),
	}
	for _, locator := range []string{"/abs.json", "../escape.json", "a/../../escape.json", "a//b.json", "./a.json", "a/", ".", "..", `sub\win.json`} {
		t.Run(locator, func(t *testing.T) {
			parent := t.TempDir()
			root := filepath.Join(parent, "root")
			if err := os.MkdirAll(root, 0o755); err != nil {
				t.Fatalf("create root: %v", err)
			}
			requestPath := filepath.Join(root, "request.json")
			briefWriteJSON(t, requestPath, request)

			_, err := runBriefForTest(t, "draft", "--input", requestPath, "--root", root, "--out", locator)
			if err == nil {
				t.Fatalf("output locator %q was accepted", locator)
			}
			if !strings.Contains(err.Error(), "output locator") {
				t.Fatalf("error for %q does not name the output locator: %v", locator, err)
			}
			if _, statErr := os.Stat(filepath.Join(parent, "escape.json")); !os.IsNotExist(statErr) {
				t.Fatalf("unsafe locator %q wrote outside the root", locator)
			}
		})
	}
}

func TestRunBriefDraft_RejectsMalformedInput(t *testing.T) {
	valid := `{"Mode":"zero-to-one creation","Request":"x","Contents":[{"Section":"game concept","Body":"c"},{"Section":"core loop","Body":"l"}]}`
	cases := []struct {
		name string
		data []byte
	}{
		{"invalid json", []byte("not json at all")},
		{"unknown field", []byte(`{"Mode":"zero-to-one creation","Bogus":true}`)},
		{"trailing json value", []byte(valid + "\n" + `{"second":1}`)},
		{"trailing garbage", []byte(valid + "\nnot json")},
		{"empty file", []byte("")},
		{"oversized", bytes.Repeat([]byte(" "), briefMaxInputBytes+1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			requestPath := filepath.Join(root, "request.json")
			if err := os.WriteFile(requestPath, tc.data, 0o644); err != nil {
				t.Fatalf("write malformed input: %v", err)
			}

			_, err := runBriefForTest(t, "draft", "--input", requestPath, "--root", root, "--out", "draft.json")
			if err == nil {
				t.Fatalf("malformed input %q was accepted", tc.name)
			}
			if _, statErr := os.Stat(filepath.Join(root, "draft.json")); !os.IsNotExist(statErr) {
				t.Fatalf("malformed input %q produced an artifact", tc.name)
			}
		})
	}
}

func TestRunBriefDraft_RejectsUnsafeOrStaleReferences(t *testing.T) {
	external := t.TempDir()
	externalContent := "external bytes outside the artifact root"
	externalPath := filepath.Join(external, "outside.md")
	if err := os.WriteFile(externalPath, []byte(externalContent), 0o644); err != nil {
		t.Fatalf("write external fixture: %v", err)
	}

	cases := []struct {
		name  string
		build func(t *testing.T, root string) coregame.DraftRequest
	}{
		{
			name: "traversal locator",
			build: func(t *testing.T, root string) coregame.DraftRequest {
				return briefChangeRequest(coregame.ArtifactReference{Phase: coregame.PhaseGDDSlice, Locator: "../outside.md", Revision: briefSHA256(externalContent)})
			},
		},
		{
			name: "symlink escaping the root",
			build: func(t *testing.T, root string) coregame.DraftRequest {
				if err := os.MkdirAll(filepath.Join(root, "artifacts"), 0o755); err != nil {
					t.Fatalf("create artifacts dir: %v", err)
				}
				if err := os.Symlink(externalPath, filepath.Join(root, "artifacts", "gdd-slice.md")); err != nil {
					t.Fatalf("create escaping symlink: %v", err)
				}
				return briefChangeRequest(coregame.ArtifactReference{Phase: coregame.PhaseGDDSlice, Locator: "artifacts/gdd-slice.md", Revision: briefSHA256(externalContent)})
			},
		},
		{
			name: "stale revision",
			build: func(t *testing.T, root string) coregame.DraftRequest {
				if err := os.MkdirAll(filepath.Join(root, "artifacts"), 0o755); err != nil {
					t.Fatalf("create artifacts dir: %v", err)
				}
				if err := os.WriteFile(filepath.Join(root, "artifacts", "gdd-slice.md"), []byte("current bytes"), 0o644); err != nil {
					t.Fatalf("write reference: %v", err)
				}
				return briefChangeRequest(coregame.ArtifactReference{Phase: coregame.PhaseGDDSlice, Locator: "artifacts/gdd-slice.md", Revision: briefSHA256("different bytes")})
			},
		},
		{
			name: "missing reference",
			build: func(t *testing.T, root string) coregame.DraftRequest {
				return briefChangeRequest(coregame.ArtifactReference{Phase: coregame.PhaseGDDSlice, Locator: "artifacts/missing.md", Revision: briefSHA256("anything")})
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			requestPath := filepath.Join(root, "request.json")
			briefWriteJSON(t, requestPath, tc.build(t, root))

			_, err := runBriefForTest(t, "draft", "--input", requestPath, "--root", root, "--out", "draft.json")
			if err == nil {
				t.Fatalf("unsafe or stale reference %q was accepted", tc.name)
			}
			if _, statErr := os.Stat(filepath.Join(root, "draft.json")); !os.IsNotExist(statErr) {
				t.Fatalf("reference failure %q still produced an artifact", tc.name)
			}
		})
	}
}

func TestRunBriefDraft_ValidationFailureLeavesInputsUnchanged(t *testing.T) {
	root := t.TempDir()
	refPath := filepath.Join(root, "artifacts", "gdd-slice.md")
	if err := os.MkdirAll(filepath.Dir(refPath), 0o755); err != nil {
		t.Fatalf("create artifacts dir: %v", err)
	}
	if err := os.WriteFile(refPath, []byte("current bytes"), 0o644); err != nil {
		t.Fatalf("write reference: %v", err)
	}
	request := briefChangeRequest(coregame.ArtifactReference{Phase: coregame.PhaseGDDSlice, Locator: "artifacts/gdd-slice.md", Revision: briefSHA256("stale bytes")})
	requestPath := filepath.Join(root, "request.json")
	briefWriteJSON(t, requestPath, request)

	before := map[string]string{}
	for _, path := range []string{requestPath, refPath} {
		before[path] = briefFileHash(t, path)
	}

	if _, err := runBriefForTest(t, "draft", "--input", requestPath, "--root", root, "--out", "draft.json"); err == nil {
		t.Fatalf("expected stale reference to fail the draft")
	}
	if _, statErr := os.Stat(filepath.Join(root, "draft.json")); !os.IsNotExist(statErr) {
		t.Fatalf("a failed draft must not produce an artifact")
	}
	for path, want := range before {
		if got := briefFileHash(t, path); got != want {
			t.Fatalf("input %s changed: %s -> %s", path, want, got)
		}
	}
}

func briefFileHash(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestRunBriefCheck_AcceptsConsistentSyntheticDecision(t *testing.T) {
	root := t.TempDir()
	ref := briefWriteRef(t, root, "artifacts/gdd-slice.md", "approved synthetic slice")
	draft := briefBuildDraft(t, root, briefChangeRequest(ref))
	draftPath := filepath.Join(root, "draft.json")
	decisionPath := filepath.Join(root, "decision.json")
	briefWriteJSON(t, draftPath, draft)
	briefWriteJSON(t, decisionPath, briefApprovedDecision(draft))

	before := listFiles(t, root)
	stdout, err := runBriefForTest(t, "check", "--input", draftPath, "--root", root, "--decision", decisionPath)
	if err != nil {
		t.Fatalf("RunBrief check: %v", err)
	}
	for _, token := range []string{
		"downstream consistency: accepted",
		"target: " + draft.Target,
		"draft_revision: " + draft.Revision,
		"decision_reference: \"synthetic recorded decision\"",
		"does not authenticate",
	} {
		if !strings.Contains(stdout, token) {
			t.Fatalf("check report missing %q, got:\n%s", token, stdout)
		}
	}
	if after := listFiles(t, root); len(after) != len(before) {
		t.Fatalf("check mode created or removed files: before=%v after=%v", before, after)
	}
}

func TestRunBriefCheck_RejectsInvalidOrMismatchedDecisions(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*coregame.HumanDecision)
	}{
		{"pending", func(d *coregame.HumanDecision) { d.State = coregame.DecisionPending }},
		{"declined", func(d *coregame.HumanDecision) { d.State = coregame.DecisionDeclined }},
		{"absent state", func(d *coregame.HumanDecision) { d.State = "" }},
		{"malformed state", func(d *coregame.HumanDecision) { d.State = coregame.DecisionState("maybe") }},
		{"absent reference", func(d *coregame.HumanDecision) { d.Reference = "" }},
		{"revision mismatch", func(d *coregame.HumanDecision) { d.DraftRevision = strings.Repeat("0", 64) }},
		{"target mismatch", func(d *coregame.HumanDecision) { d.Target = "SDD handoff" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			ref := briefWriteRef(t, root, "artifacts/gdd-slice.md", "approved synthetic slice")
			draft := briefBuildDraft(t, root, briefChangeRequest(ref))
			draftPath := filepath.Join(root, "draft.json")
			decisionPath := filepath.Join(root, "decision.json")
			briefWriteJSON(t, draftPath, draft)
			decision := briefApprovedDecision(draft)
			tc.mutate(&decision)
			briefWriteJSON(t, decisionPath, decision)

			stdout, err := runBriefForTest(t, "check", "--input", draftPath, "--root", root, "--decision", decisionPath)
			if err == nil {
				t.Fatalf("decision %q was accepted", tc.name)
			}
			if strings.Contains(stdout, "accepted") {
				t.Fatalf("rejected decision %q reported acceptance:\n%s", tc.name, stdout)
			}
			if !strings.Contains(err.Error(), coregame.ValidationLimitation) {
				t.Fatalf("rejection %q must surface the validation limitation, got %v", tc.name, err)
			}
		})
	}

	t.Run("missing decision file", func(t *testing.T) {
		root := t.TempDir()
		ref := briefWriteRef(t, root, "artifacts/gdd-slice.md", "approved synthetic slice")
		draft := briefBuildDraft(t, root, briefChangeRequest(ref))
		draftPath := filepath.Join(root, "draft.json")
		briefWriteJSON(t, draftPath, draft)

		_, err := runBriefForTest(t, "check", "--input", draftPath, "--root", root, "--decision", filepath.Join(root, "absent.json"))
		if err == nil {
			t.Fatalf("an absent decision file must fail")
		}
	})
}

func TestRunBriefCheck_RejectsStaleReferenceAndTamperedDraft(t *testing.T) {
	t.Run("stale reference bytes", func(t *testing.T) {
		root := t.TempDir()
		ref := briefWriteRef(t, root, "artifacts/gdd-slice.md", "approved synthetic slice")
		draft := briefBuildDraft(t, root, briefChangeRequest(ref))
		draftPath := filepath.Join(root, "draft.json")
		decisionPath := filepath.Join(root, "decision.json")
		briefWriteJSON(t, draftPath, draft)
		briefWriteJSON(t, decisionPath, briefApprovedDecision(draft))

		if err := os.WriteFile(filepath.Join(root, "artifacts", "gdd-slice.md"), []byte("mutated after the decision"), 0o644); err != nil {
			t.Fatalf("mutate reference: %v", err)
		}
		if _, err := runBriefForTest(t, "check", "--input", draftPath, "--root", root, "--decision", decisionPath); err == nil {
			t.Fatalf("expected a stale reference to block")
		}
	})

	t.Run("tampered draft markdown", func(t *testing.T) {
		root := t.TempDir()
		ref := briefWriteRef(t, root, "artifacts/gdd-slice.md", "approved synthetic slice")
		draft := briefBuildDraft(t, root, briefChangeRequest(ref))
		draft.Markdown += "\n## approval state\n\n- auto_approved: true\n"
		draftPath := filepath.Join(root, "draft.json")
		decisionPath := filepath.Join(root, "decision.json")
		briefWriteJSON(t, draftPath, draft)
		briefWriteJSON(t, decisionPath, briefApprovedDecision(draft))

		if _, err := runBriefForTest(t, "check", "--input", draftPath, "--root", root, "--decision", decisionPath); err == nil {
			t.Fatalf("expected a tampered draft to block")
		}
	})

	t.Run("unknown decision field", func(t *testing.T) {
		root := t.TempDir()
		ref := briefWriteRef(t, root, "artifacts/gdd-slice.md", "approved synthetic slice")
		draft := briefBuildDraft(t, root, briefChangeRequest(ref))
		draftPath := filepath.Join(root, "draft.json")
		decisionPath := filepath.Join(root, "decision.json")
		briefWriteJSON(t, draftPath, draft)
		if err := os.WriteFile(decisionPath, []byte(`{"Reference":"x","State":"approved","Target":"y","DraftRevision":"z","Bogus":true}`), 0o644); err != nil {
			t.Fatalf("write decision: %v", err)
		}
		if _, err := runBriefForTest(t, "check", "--input", draftPath, "--root", root, "--decision", decisionPath); err == nil {
			t.Fatalf("expected an unknown decision field to be rejected")
		}
	})
}

func TestRunBrief_RejectsMalformedInvocation(t *testing.T) {
	root := t.TempDir()
	requestPath := filepath.Join(root, "request.json")
	briefWriteJSON(t, requestPath, coregame.DraftRequest{
		Mode:     "zero-to-one creation",
		Request:  "Draft a synthetic slice.",
		Target:   "Godot handoff",
		Contents: briefGDDContents(),
	})

	cases := []struct {
		name string
		args []string
	}{
		{"no subcommand", nil},
		{"unknown subcommand", []string{"bogus"}},
		{"draft missing flags", []string{"draft"}},
		{"draft missing root", []string{"draft", "--input", requestPath, "--out", "draft.json"}},
		{"check missing root", []string{"check", "--input", requestPath, "--decision", requestPath}},
		{"draft with check flag", []string{"draft", "--input", requestPath, "--root", root, "--out", "draft.json", "--decision", requestPath}},
		{"check with draft flag", []string{"check", "--input", requestPath, "--root", root, "--decision", requestPath, "--out", "draft.json"}},
		{"draft with extra positional", []string{"draft", "--input", requestPath, "--root", root, "--out", "draft.json", "extra"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := runBriefForTest(t, tc.args...); err == nil {
				t.Fatalf("malformed invocation %q was accepted", tc.name)
			}
		})
	}

	if _, err := runBriefForTest(t, "bogus"); err == nil || !strings.Contains(err.Error(), "unknown brief subcommand") {
		t.Fatalf("expected an unknown-subcommand error, got %v", err)
	}
}

func TestRunBrief_RequiresExplicitRootForReferences(t *testing.T) {
	// A request with references cannot be satisfied without an explicit root;
	// the CLI must never scan the working directory for a candidate root.
	request := briefChangeRequest(coregame.ArtifactReference{Phase: coregame.PhaseGDDSlice, Locator: "artifacts/gdd-slice.md", Revision: briefSHA256("approved synthetic slice")})
	requestPath := filepath.Join(t.TempDir(), "request.json")
	briefWriteJSON(t, requestPath, request)

	_, err := runBriefForTest(t, "draft", "--input", requestPath, "--root", "", "--out", "draft.json")
	if err == nil {
		t.Fatalf("an empty explicit root must be rejected")
	}
}

func TestRunBriefDraft_RejectsEncodedArtifactOverLimit(t *testing.T) {
	root := t.TempDir()
	ref := briefWriteRef(t, root, "artifacts/gdd-slice.md", "approved synthetic slice")
	request := briefChangeRequest(ref)
	request.Contents = briefWithBody(request.Contents, "risks", strings.Repeat("a", 700000))
	requestPath := filepath.Join(root, "request.json")
	briefWriteJSON(t, requestPath, request)
	refPath := filepath.Join(root, "artifacts", "gdd-slice.md")
	beforeRequest := briefFileHash(t, requestPath)
	beforeRef := briefFileHash(t, refPath)

	_, err := runBriefForTest(t, "draft", "--input", requestPath, "--root", root, "--out", "draft.json")
	if err == nil {
		t.Fatalf("an oversized encoded draft must fail")
	}
	for _, token := range []string{"artifact limit", "reduce the amount of supplied content"} {
		if !strings.Contains(err.Error(), token) {
			t.Fatalf("oversize error missing %q, got %v", token, err)
		}
	}
	if _, statErr := os.Stat(filepath.Join(root, "draft.json")); !os.IsNotExist(statErr) {
		t.Fatalf("an oversized draft must not produce an output artifact")
	}
	if got := briefFileHash(t, requestPath); got != beforeRequest {
		t.Fatalf("request bytes changed: %s -> %s", beforeRequest, got)
	}
	if got := briefFileHash(t, refPath); got != beforeRef {
		t.Fatalf("reference bytes changed: %s -> %s", beforeRef, got)
	}
}

func TestRunBriefDraft_CheckRoundTripWithLargeContent(t *testing.T) {
	root := t.TempDir()
	ref := briefWriteRef(t, root, "artifacts/gdd-slice.md", "approved synthetic slice")
	request := briefChangeRequest(ref)
	request.Contents = briefWithBody(request.Contents, "risks", strings.Repeat("a", 350000))
	requestPath := filepath.Join(root, "request.json")
	briefWriteJSON(t, requestPath, request)

	if _, err := runBriefForTest(t, "draft", "--input", requestPath, "--root", root, "--out", "draft.json"); err != nil {
		t.Fatalf("a large but under-limit draft must build: %v", err)
	}
	draftPath := filepath.Join(root, "draft.json")
	draft := readDraftArtifact(t, draftPath)
	if draft.Revision == "" || len(draft.Markdown) == 0 {
		t.Fatalf("round-trip draft is incomplete")
	}
	decisionPath := filepath.Join(root, "decision.json")
	briefWriteJSON(t, decisionPath, briefApprovedDecision(draft))

	stdout, err := runBriefForTest(t, "check", "--input", draftPath, "--root", root, "--decision", decisionPath)
	if err != nil {
		t.Fatalf("a large under-limit draft must round-trip through check: %v", err)
	}
	if !strings.Contains(stdout, "downstream consistency: accepted") {
		t.Fatalf("round-trip check did not report acceptance:\n%s", stdout)
	}
}

func TestRunBriefDraft_ReportDeliveryFailureRetainsArtifact(t *testing.T) {
	root := t.TempDir()
	requestPath := filepath.Join(root, "request.json")
	briefWriteJSON(t, requestPath, coregame.DraftRequest{
		Mode:     "zero-to-one creation",
		Request:  "Draft a synthetic slice.",
		Target:   "Godot handoff",
		Contents: briefGDDContents(),
	})

	sink := briefFailingWriter{err: errors.New("sink unavailable")}
	err := RunBrief(BriefInput{Args: []string{"draft", "--input", requestPath, "--root", root, "--out", "draft.json"}, Stdout: sink})
	if err == nil {
		t.Fatalf("a failing report sink must return an error")
	}
	if !strings.Contains(err.Error(), "artifact remains at") || !strings.Contains(err.Error(), strconv.Quote("draft.json")) {
		t.Fatalf("draft report failure must name the retained artifact, got %v", err)
	}
	draft := readDraftArtifact(t, filepath.Join(root, "draft.json"))
	if draft.Status != "draft" || draft.ApprovalState != "pending_human_approval" || draft.AutoApproved {
		t.Fatalf("retained artifact is not a valid pending draft: status=%q approval=%q auto=%t", draft.Status, draft.ApprovalState, draft.AutoApproved)
	}
}

func TestRunBriefCheck_ReportDeliveryFailureReturnsError(t *testing.T) {
	root := t.TempDir()
	ref := briefWriteRef(t, root, "artifacts/gdd-slice.md", "approved synthetic slice")
	draft := briefBuildDraft(t, root, briefChangeRequest(ref))
	draftPath := filepath.Join(root, "draft.json")
	decisionPath := filepath.Join(root, "decision.json")
	briefWriteJSON(t, draftPath, draft)
	briefWriteJSON(t, decisionPath, briefApprovedDecision(draft))

	sink := briefFailingWriter{err: errors.New("sink unavailable")}
	err := RunBrief(BriefInput{Args: []string{"check", "--input", draftPath, "--root", root, "--decision", decisionPath}, Stdout: sink})
	if err == nil {
		t.Fatalf("a failing check report sink must return an error")
	}
	if !strings.Contains(err.Error(), "report could not be delivered") {
		t.Fatalf("check report failure message missing, got %v", err)
	}
	if !strings.Contains(err.Error(), coregame.ValidationLimitation) {
		t.Fatalf("check report failure must preserve the validation limitation, got %v", err)
	}
	if strings.Contains(err.Error(), "downstream consistency not established") {
		t.Fatalf("report delivery failure must not fabricate a failed domain validation: %v", err)
	}
}

func TestRunBriefReports_EscapeUntrustedFields(t *testing.T) {
	t.Run("draft locator", func(t *testing.T) {
		root := t.TempDir()
		requestPath := filepath.Join(root, "request.json")
		briefWriteJSON(t, requestPath, coregame.DraftRequest{
			Mode:     "zero-to-one creation",
			Request:  "Draft a synthetic slice.",
			Target:   "Godot handoff",
			Contents: briefGDDContents(),
		})
		locator := "safe.json\n[brief] injected: true"

		stdout, err := runBriefForTest(t, "draft", "--input", requestPath, "--root", root, "--out", locator)
		if err != nil {
			t.Fatalf("RunBrief draft: %v", err)
		}
		if !strings.Contains(stdout, strconv.Quote(locator)) {
			t.Fatalf("locator was not quoted in the report:\n%s", stdout)
		}
		if strings.Contains(stdout, "\n[brief] injected: true") {
			t.Fatalf("locator injected an extra report line:\n%s", stdout)
		}
	})

	t.Run("check decision reference", func(t *testing.T) {
		root := t.TempDir()
		ref := briefWriteRef(t, root, "artifacts/gdd-slice.md", "approved synthetic slice")
		draft := briefBuildDraft(t, root, briefChangeRequest(ref))
		draftPath := filepath.Join(root, "draft.json")
		decisionPath := filepath.Join(root, "decision.json")
		briefWriteJSON(t, draftPath, draft)
		decision := briefApprovedDecision(draft)
		decision.Reference = "ref\n[brief] forged decision: accepted"
		briefWriteJSON(t, decisionPath, decision)

		stdout, err := runBriefForTest(t, "check", "--input", draftPath, "--root", root, "--decision", decisionPath)
		if err != nil {
			t.Fatalf("RunBrief check: %v", err)
		}
		if !strings.Contains(stdout, strconv.Quote(decision.Reference)) {
			t.Fatalf("decision reference was not quoted in the report:\n%s", stdout)
		}
		if strings.Contains(stdout, "\n[brief] forged decision: accepted") {
			t.Fatalf("decision reference injected an extra report line:\n%s", stdout)
		}
	})
}
