package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"open-game-studios/internal/workflows/coregame"
)

// briefMaxInputBytes bounds every JSON document the brief CLI reads. Inputs are
// explicit caller-owned files; the CLI never scans directories and never
// searches for an artifact root.
const briefMaxInputBytes = 1 << 20

// BriefInput is the narrowed input seam for `game-studio brief`. Stdout is
// injectable for tests; a nil writer falls back to os.Stdout.
type BriefInput struct {
	Args   []string
	Stdout io.Writer
}

// RunBrief dispatches the explicit brief subcommands. It is a thin adapter over
// internal/workflows/coregame: it decodes caller-supplied JSON, resolves an
// explicit artifact root, and delegates all draft construction and downstream
// validation to the accepted library. It never generates game content and never
// captures consent.
func RunBrief(input BriefInput) error {
	stdout := input.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	if len(input.Args) == 0 {
		return errors.New(briefUsage())
	}
	switch input.Args[0] {
	case "draft":
		return runBriefDraft(input.Args[1:], stdout)
	case "check":
		return runBriefCheck(input.Args[1:], stdout)
	default:
		return fmt.Errorf("unknown brief subcommand %q\n\n%s", input.Args[0], briefUsage())
	}
}

// runBriefDraft builds one pending draft through the accepted library and
// writes it as a single JSON artifact. The request, the artifact root and the
// output locator are all explicit: a missing artifact root, a missing output
// directory or an existing output file are errors, never silent fallbacks.
func runBriefDraft(args []string, stdout io.Writer) error {
	set := flag.NewFlagSet("brief draft", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	inputPath := set.String("input", "", "path to a coregame.DraftRequest JSON file (required)")
	rootPath := set.String("root", "", "explicit artifact root directory (required)")
	outLocator := set.String("out", "", "root-relative output path for the draft JSON (required)")
	if err := set.Parse(args); err != nil {
		return fmt.Errorf("brief draft: %w\n\n%s", err, briefUsage())
	}
	if set.NArg() != 0 {
		return fmt.Errorf("brief draft: unexpected positional arguments %v", set.Args())
	}
	for _, required := range []struct{ name, value string }{
		{"input", *inputPath},
		{"root", *rootPath},
		{"out", *outLocator},
	} {
		if strings.TrimSpace(required.value) == "" {
			return fmt.Errorf("brief draft: --%s is required and must not be empty", required.name)
		}
	}
	if err := validateRootRelativeOutput(*outLocator); err != nil {
		return err
	}

	var request coregame.DraftRequest
	if err := readStrictJSON(*inputPath, &request); err != nil {
		return err
	}

	root, err := os.OpenRoot(*rootPath)
	if err != nil {
		return fmt.Errorf("brief draft: open artifact root %q: %w", *rootPath, err)
	}
	defer root.Close()

	draft, err := coregame.BuildDraft(request, root.FS())
	if err != nil {
		return fmt.Errorf("brief draft: %w", err)
	}

	artifact, err := json.MarshalIndent(draft, "", "  ")
	if err != nil {
		return fmt.Errorf("brief draft: encode draft: %w", err)
	}
	artifact = append(artifact, '\n')

	// The shared input/output bound keeps a draft round-trippable through
	// `brief check`, which reads the same bounded JSON. Reject an oversized
	// encoded draft before any output is created, and name the fix rather than
	// silently truncating the artifact.
	if len(artifact) > briefMaxInputBytes {
		return fmt.Errorf("brief draft: encoded draft is %d bytes, over the %d-byte artifact limit; reduce the amount of supplied content", len(artifact), briefMaxInputBytes)
	}

	if err := writeExclusiveRootFile(root, *outLocator, artifact); err != nil {
		return err
	}

	if err := printDraftReport(stdout, *outLocator, draft); err != nil {
		return fmt.Errorf("brief draft: the draft was written but its report could not be delivered; the artifact remains at %q: %w", *outLocator, err)
	}
	return nil
}

// runBriefCheck decodes a draft and a separate coordinator-supplied decision,
// then runs the library's downstream consistency validation against the current
// reference bytes under the explicit artifact root. It writes no artifact and
// performs no production action.
func runBriefCheck(args []string, stdout io.Writer) error {
	set := flag.NewFlagSet("brief check", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	inputPath := set.String("input", "", "path to a coregame.Draft JSON file (required)")
	rootPath := set.String("root", "", "explicit artifact root directory for reference bytes (required)")
	decisionPath := set.String("decision", "", "path to a coregame.HumanDecision JSON file (required)")
	if err := set.Parse(args); err != nil {
		return fmt.Errorf("brief check: %w\n\n%s", err, briefUsage())
	}
	if set.NArg() != 0 {
		return fmt.Errorf("brief check: unexpected positional arguments %v", set.Args())
	}
	for _, required := range []struct{ name, value string }{
		{"input", *inputPath},
		{"root", *rootPath},
		{"decision", *decisionPath},
	} {
		if strings.TrimSpace(required.value) == "" {
			return fmt.Errorf("brief check: --%s is required and must not be empty", required.name)
		}
	}

	var draft coregame.Draft
	if err := readStrictJSON(*inputPath, &draft); err != nil {
		return err
	}
	var decision coregame.HumanDecision
	if err := readStrictJSON(*decisionPath, &decision); err != nil {
		return err
	}

	root, err := os.OpenRoot(*rootPath)
	if err != nil {
		return fmt.Errorf("brief check: open artifact root %q: %w", *rootPath, err)
	}
	defer root.Close()

	result, err := coregame.ValidateDownstream(draft, root.FS(), decision)
	if err != nil {
		// The library guarantees a limitation string on every result, including
		// rejection. Surface it so the caller never mistakes an error for
		// authorization.
		return fmt.Errorf("brief check: downstream consistency not established: %w (limitation: %s)", err, result.Limitation)
	}
	if !result.Accepted {
		// Defensive: the accepted library only reports Accepted with a nil
		// error, but the CLI must never print acceptance for a non-accepted
		// result under any future library change.
		return fmt.Errorf("brief check: downstream consistency not established (limitation: %s)", result.Limitation)
	}

	if err := printCheckReport(stdout, result); err != nil {
		// The consistency check succeeded; only report delivery failed. Do not
		// imply a failed domain validation and grant no authority.
		return fmt.Errorf("brief check: consistency was validated but its report could not be delivered: %w (limitation: %s)", err, result.Limitation)
	}
	return nil
}

// writeExclusiveRootFile creates the output inside the root without ever
// overwriting existing state. O_EXCL refuses a regular file and a dangling
// symlink alike, and a missing parent directory is an error rather than an
// implicit MkdirAll.
func writeExclusiveRootFile(root *os.Root, locator string, data []byte) error {
	file, err := root.OpenFile(locator, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		switch {
		case errors.Is(err, fs.ErrExist):
			return fmt.Errorf("brief draft: refusing to overwrite existing output %q", locator)
		case errors.Is(err, fs.ErrNotExist):
			return fmt.Errorf("brief draft: output directory for %q does not exist; the CLI does not create it", locator)
		default:
			return fmt.Errorf("brief draft: create %q: %w", locator, err)
		}
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return fmt.Errorf("brief draft: wrote an incomplete artifact at %q; write failed: %w", locator, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("brief draft: wrote an incomplete artifact at %q; close failed: %w", locator, err)
	}
	return nil
}

// readStrictJSON reads one bounded JSON value from an explicit path and rejects
// unknown fields, a second value and trailing data. The file is only read;
// its bytes are never modified.
func readStrictJSON(path string, target any) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("brief: open %q: %w", path, err)
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, briefMaxInputBytes+1))
	if err != nil {
		return fmt.Errorf("brief: read %q: %w", path, err)
	}
	if len(data) > briefMaxInputBytes {
		return fmt.Errorf("brief: %q exceeds the %d-byte input limit", path, briefMaxInputBytes)
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("brief: decode %q: %w", path, err)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("brief: %q contains more than one JSON value", path)
		}
		return fmt.Errorf("brief: %q has trailing data after the JSON value: %w", path, err)
	}
	return nil
}

// validateRootRelativeOutput rejects an absolute locator, any traversal, and
// any non-canonical path before the output is touched.
func validateRootRelativeOutput(locator string) error {
	if locator == "" {
		return fmt.Errorf("brief draft: output locator must not be empty")
	}
	if strings.ContainsRune(locator, 0) {
		return fmt.Errorf("brief draft: output locator contains a NUL byte")
	}
	if path.IsAbs(locator) || filepath.IsAbs(locator) || strings.HasPrefix(locator, "/") || strings.HasPrefix(locator, `\`) {
		return fmt.Errorf("brief draft: output locator %q must be root-relative, not absolute", locator)
	}
	if strings.ContainsRune(locator, '\\') {
		return fmt.Errorf("brief draft: output locator %q must use forward slashes", locator)
	}
	if path.Clean(locator) != locator || !fs.ValidPath(locator) || locator == "." {
		return fmt.Errorf("brief draft: output locator %q does not name a file inside the artifact root", locator)
	}
	return nil
}

// writeReport writes every report line in order and stops at the first sink
// failure, so a broken writer can never produce a partially reported success.
// Untrusted report fields are quoted with %q by the caller so they cannot
// inject extra `[brief]` report lines.
func writeReport(stdout io.Writer, lines []string) error {
	for _, line := range lines {
		if _, err := io.WriteString(stdout, line); err != nil {
			return err
		}
	}
	return nil
}

func printDraftReport(stdout io.Writer, locator string, draft coregame.Draft) error {
	return writeReport(stdout, []string{
		fmt.Sprintf("[brief] draft written: %q\n", locator),
		fmt.Sprintf("[brief] workflow: %s\n", draft.Version),
		fmt.Sprintf("[brief] mode: %s\n", draft.Mode),
		fmt.Sprintf("[brief] entry: %s\n", draft.Kind),
		fmt.Sprintf("[brief] phase: %s\n", draft.Phase),
		fmt.Sprintf("[brief] status: %s\n", draft.Status),
		fmt.Sprintf("[brief] approval_state: %s\n", draft.ApprovalState),
		fmt.Sprintf("[brief] auto_approved: %t\n", draft.AutoApproved),
		fmt.Sprintf("[brief] revision: %s\n", draft.Revision),
		"[brief] note: pending draft; no approval, consent, or downstream action is implied.\n",
	})
}

func printCheckReport(stdout io.Writer, result coregame.DownstreamValidation) error {
	return writeReport(stdout, []string{
		"[brief] downstream consistency: accepted\n",
		fmt.Sprintf("[brief] phase: %s\n", result.Phase),
		fmt.Sprintf("[brief] target: %s\n", result.Target),
		fmt.Sprintf("[brief] draft_revision: %s\n", result.DraftRevision),
		fmt.Sprintf("[brief] decision_reference: %q\n", result.DecisionReference),
		fmt.Sprintf("[brief] decision_state: %s\n", result.DecisionState),
		fmt.Sprintf("[brief] limitation: %s\n", result.Limitation),
		"[brief] note: check mode verifies only a claimed external recorded decision; it does not authenticate a human, capture consent, or grant authority.\n",
	})
}

func briefUsage() string {
	return `brief: build a pending core-game draft, or check one against a recorded decision

Usage:
  game-studio brief draft --input <request.json> --root <artifact-root> --out <relative-draft.json>
  game-studio brief check --input <draft.json> --root <artifact-root> --decision <decision.json>

Draft:
  Decodes a coregame.DraftRequest, builds a pending coregame.Draft through the
  accepted core-game library, and writes it as one JSON artifact. The artifact
  is always pending: no approval, consent or downstream action is implied.
  --out must be a safe root-relative path; existing output is never overwritten
  and missing output directories are never created implicitly.

Check:
  Decodes a coregame.Draft and a separate coregame.HumanDecision, then runs the
  library's downstream consistency validation against the current reference
  bytes under --root. It writes no artifact and performs no production action.
  An absent, pending, declined, malformed, stale or mismatched input fails at
  the app boundary and is never reported as accepted.
  This verifies only a claimed external recorded decision: it cannot
  authenticate authors, capture consent, issue permission or grant authority.
  A real coordinator must observe the human decision independently before using
  such data for production.`
}
