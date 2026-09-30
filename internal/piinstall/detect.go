package piinstall

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	ogsskills "open-game-studios/skills"
)

// minimumVersion is the lowest version of a component that the plan will reuse
// instead of proposing a managed install.
var minimumVersion = map[Component]string{
	ComponentNode:       "22.19.0", // Pi's documented Node requirement
	ComponentNpm:        "10.0.0",
	ComponentPi:         "0.87.1",
	ComponentEngramCore: "2.2.0",
	ComponentGodot:      "4.0.0",
}

var semverPattern = regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)
var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

// The git recognition policy is a single shared repository-path/ref contract
// applied to every recognized source form. It is a deliberate safe subset of
// Pi's public parser (dist/utils/git.js): the installer consumes a `pi list`
// record, so a source it cannot validate must fail closed as `unverified`
// rather than be treated as proof of absence. Being narrower than Pi is safe;
// over-accepting an unvalidated source is not. No form here performs a network
// lookup, and no new dependency (such as Pi's bundled hosted-git-info) is added.
//
// Processing order for every form:
//  1. Separate the repository path from URL userinfo and the SCP host.
//  2. Split the FIRST repository `@ref`; a slash inside the ref never counts
//     toward repository depth.
//  3. Reject unsafe raw or encoded forms (leading slash, backslash, NUL, `..`
//     segment, or an undecodable percent escape) BEFORE normalizing, so an
//     unsafe input is never cleaned into an accepted value.
//  4. Strip a terminal `.git` from the repository portion.
//  5. Require at least two meaningful namespace/repository segments, each
//     nonempty and not `.`, `..`, or `.git`.
// Both the raw and the final repository portion are validated.

// supportedGitSource reports whether a Pi git package source is one of the
// documented, validatable shapes: a hierarchical HTTPS/SSH URL, an scp-style
// `git@host:namespace/repo`, or a dotted-host/localhost shorthand. A bare,
// whitespace-only, malformed URL-like, or otherwise opaque source is refused so
// the listing fails closed as unverified. It performs no network lookup and
// invents no new Pi grammar.
func supportedGitSource(payload string) bool {
	if payload == "" || strings.TrimSpace(payload) != payload {
		return false
	}
	if strings.ContainsAny(payload, " \t\r\n") {
		return false
	}
	switch {
	case strings.Contains(payload, "://"):
		// Any scheme-looking payload is validated strictly as a URL and never
		// falls through to the shorthand/scp matcher, so a malformed URL like
		// "https:/" or an unsupported scheme is refused instead of being
		// accepted as a host/path shorthand.
		return supportedGitURL(payload)
	case strings.HasPrefix(payload, "git@"):
		return supportedGitSCP(payload)
	default:
		return supportedGitShorthand(payload)
	}
}

// supportedGitURL parses a URL-form source with net/url and requires an exactly
// recognized lowercase scheme (HTTPS or SSH), no opaque part, a conservative
// host, a valid optional port, and a validated repository path. net/url.Parse is
// not WHATWG `new URL`: it validates that a port is numeric but not that it is
// in range, and it accepts host/path forms (IPv6/zone literals, percent-encoded
// or numeric hosts, double-slash paths) whose producer acceptance is unproven.
// Those ambiguous forms are refused rather than emulated. Query and fragment
// suffixes are allowed because they do not change the host/path identity.
func supportedGitURL(payload string) bool {
	if !strings.HasPrefix(payload, "https://") && !strings.HasPrefix(payload, "ssh://") {
		// Rejects `http://`, `git://`, `ftp://`, `file://`, and any uppercase or
		// mixed-case scheme such as `HTTPS://` while keeping the documented
		// lowercase HTTPS/SSH subset.
		return false
	}
	parsed, err := url.Parse(payload)
	if err != nil {
		return false
	}
	if parsed.Opaque != "" {
		return false
	}
	if !gitHostSafe(parsed.Hostname()) {
		return false
	}
	if port := parsed.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return false
		}
	}
	// parsed.EscapedPath() keeps the encoded path (matching Pi's use of the WHATWG
	// `pathname`). Strip exactly one structural leading slash; a second leading
	// slash stays and is rejected as an unsafe leading slash.
	pathWithMaybeRef := strings.TrimPrefix(parsed.EscapedPath(), "/")
	repoPortion, _ := splitGitRef(pathWithMaybeRef)
	_, ok := validateGitRepositoryPath(repoPortion)
	return ok
}

// supportedGitSCP handles the scp-style `git@host:namespace/repo[.git][@ref]`
// form. The host is everything before the first colon (Pi's `^git@([^:]+):`),
// and a slash-only `git@host/path` is refused as the ambiguous `user@host/slash`
// shape. A leading slash in the repository portion is an absolute path and is
// rejected by the shared validator.
func supportedGitSCP(payload string) bool {
	rest := strings.TrimPrefix(payload, "git@")
	colon := strings.IndexByte(rest, ':')
	if colon <= 0 {
		return false
	}
	if !gitHostSafe(rest[:colon]) {
		return false
	}
	repoPortion, _ := splitGitRef(rest[colon+1:])
	_, ok := validateGitRepositoryPath(repoPortion)
	return ok
}

// supportedGitShorthand handles the host/path shorthand such as
// `github.com/example/pi-tools@v1`. The host is everything before the first
// slash and must contain a dot or be `localhost` (Pi's generic-shorthand rule).
// A host carrying `@` or `:` is refused: `git@host/slash` and
// `host:path` are not the documented shorthand and must not be guessed into one.
func supportedGitShorthand(payload string) bool {
	slash := strings.IndexByte(payload, '/')
	if slash <= 0 {
		return false
	}
	host := payload[:slash]
	if strings.ContainsAny(host, "@:") {
		return false
	}
	if !strings.Contains(host, ".") && host != "localhost" {
		return false
	}
	if !gitHostSafe(host) {
		return false
	}
	repoPortion, _ := splitGitRef(payload[slash+1:])
	_, ok := validateGitRepositoryPath(repoPortion)
	return ok
}

// splitGitRef separates the first repository `@ref` from the repository path,
// mirroring Pi's splitRef: the split is at the first `@`, and both sides must be
// nonempty for it to be a ref. A slash after the `@` belongs to the ref and
// never contributes repository depth. When the split is not a valid ref, the
// whole input is returned as the repository path.
func splitGitRef(pathWithMaybeRef string) (string, string) {
	at := strings.IndexByte(pathWithMaybeRef, '@')
	if at < 0 {
		return pathWithMaybeRef, ""
	}
	repoPath := pathWithMaybeRef[:at]
	ref := pathWithMaybeRef[at+1:]
	if repoPath == "" || ref == "" {
		return pathWithMaybeRef, ""
	}
	return repoPath, ref
}

// validateGitRepositoryPath validates the repository portion (host/userinfo and
// ref already removed) and returns the normalized path. It rejects unsafe raw
// and normalized forms, strips a terminal `.git`, and requires at least two
// meaningful segments.
func validateGitRepositoryPath(repoPortion string) (string, bool) {
	if repoPortion == "" {
		return "", false
	}
	if gitComponentUnsafe(repoPortion, true) {
		return "", false
	}
	normalized := strings.TrimSuffix(repoPortion, ".git")
	if gitComponentUnsafe(normalized, true) {
		return "", false
	}
	segments := strings.Split(normalized, "/")
	if len(segments) < 2 {
		return "", false
	}
	for _, segment := range segments {
		if !meaningfulGitSegment(segment) {
			return "", false
		}
	}
	return normalized, true
}

// meaningfulGitSegment reports whether a repository path segment names a real
// namespace or repository component. Empty segments and the special `.`, `..`,
// and `.git` names do not.
func meaningfulGitSegment(segment string) bool {
	switch segment {
	case "", ".", "..", ".git":
		return false
	default:
		return true
	}
}

// gitComponentUnsafe mirrors Pi's hasUnsafeGitInstallPart: a component whose
// percent-encoding cannot be decoded, or that contains a NUL, a backslash, a
// leading slash, or a `..` segment (raw or decoded), is unsafe. When allowSlash
// is false (a host), a slash is unsafe as well. Both the raw and the decoded
// candidate are checked so an encoded payload is never normalized into an
// accepted value. The decoded string must also be valid UTF-8: Go's
// url.PathUnescape can emit invalid bytes for escapes such as `%FF` or the
// surrogate `%ED%A0%80`, whereas Pi's decodeURIComponent throws and rejects them.
func gitComponentUnsafe(raw string, allowSlash bool) bool {
	decoded, err := url.PathUnescape(raw)
	if err != nil || !utf8.ValidString(decoded) {
		return true
	}
	for _, candidate := range []string{raw, decoded} {
		if strings.ContainsRune(candidate, '\x00') || strings.ContainsRune(candidate, '\\') || strings.HasPrefix(candidate, "/") {
			return true
		}
		if !allowSlash && strings.ContainsRune(candidate, '/') {
			return true
		}
		for _, segment := range strings.Split(candidate, "/") {
			if segment == ".." {
				return true
			}
		}
	}
	return false
}

// gitHostSafe is the conservative host subset. It requires a nonempty component
// that is not `.`, applies the same unsafe-component rules as Pi's host check,
// and further refuses bracketed/IPv6 (`:`/`[`/`]`) and percent-encoded or zone
// (`%`) hosts. It also refuses any host whose final label is all digits or
// begins with `0x`/`0X` (see hostHasAmbiguousNumericLabel). Go's net/url accepts
// those forms, but WHATWG HTTP URL parsing resolves them numerically and their
// producer acceptance is unproven, so they stay unverified.
func gitHostSafe(host string) bool {
	if host == "" || host == "." {
		return false
	}
	if gitComponentUnsafe(host, false) {
		return false
	}
	if strings.ContainsAny(host, "%:[]") {
		return false
	}
	return !hostHasAmbiguousNumericLabel(host)
}

// hostHasAmbiguousNumericLabel reports whether the host's final label is an
// all-digit sequence or begins with `0x`/`0X`. WHATWG HTTP URL parsing resolves
// such final labels as numbers and refuses out-of-range values; this installer
// does not emulate that algorithm, so it conservatively leaves any such host
// unverified. A single trailing host dot (for example `github.com.`) is ignored
// when identifying the final label. This common subset intentionally also
// excludes valid dotted-decimal IPv4 (`192.168.0.1`) and hexadecimal/numeric-
// ended names; ordinary DNS names, `localhost`, and single-label SSH aliases
// such as `build01` remain recognized.
func hostHasAmbiguousNumericLabel(host string) bool {
	label := strings.TrimSuffix(host, ".")
	if dot := strings.LastIndexByte(label, '.'); dot >= 0 {
		label = label[dot+1:]
	}
	if label == "" {
		return false
	}
	if strings.HasPrefix(label, "0x") || strings.HasPrefix(label, "0X") {
		return true
	}
	for i := 0; i < len(label); i++ {
		if label[i] < '0' || label[i] > '9' {
			return false
		}
	}
	return true
}

// Detect performs the read-only prerequisite scan. It never runs a mutating
// command and never lists Pi packages, because `pi list` initializes bootstrap
// settings; package presence is deferred to post-consent probing.
func Detect(ctx context.Context, cfg Config) (Detection, error) {
	cfg = cfg.withDefaults()
	if err := cfg.validate(); err != nil {
		return Detection{}, err
	}

	detection := Detection{}
	node := cfg.probe(ctx, ComponentNode, cfg.NodePath, "node", "--version")
	classifyMinimum(&node, minimumVersion[ComponentNode])

	npm := cfg.probe(ctx, ComponentNpm, cfg.NpmPath, "npm", "--version")
	classifyMinimum(&npm, minimumVersion[ComponentNpm])

	pi := cfg.probe(ctx, ComponentPi, cfg.PiPath, "pi", "--version")
	classifyMinimum(&pi, minimumVersion[ComponentPi])

	engram := cfg.probe(ctx, ComponentEngramCore, cfg.EngramPath, "engram", "--version")
	classifyMinimum(&engram, minimumVersion[ComponentEngramCore])

	detection.States = append(detection.States, node, npm, pi, engram)

	detection.States = append(detection.States,
		deferredPackageState(ComponentShell, "gentle-pi"),
		deferredPackageState(ComponentEngramCompanion, "gentle-engram"),
	)

	if cfg.GodotRequired {
		godot := cfg.probeGodot(ctx)
		detection.States = append(detection.States, godot)
	}

	detection.States = append(detection.States, detectPayload(cfg.WorkspaceDir))
	return detection, nil
}

// BuildDetection exposes the same read-only scan for callers that want to
// inspect before planning.
func BuildDetection(ctx context.Context, cfg Config) (Detection, error) {
	return Detect(ctx, cfg)
}

func (c Config) resolveBinary(explicit, name string) (string, bool) {
	if strings.TrimSpace(explicit) != "" {
		return strings.TrimSpace(explicit), true
	}
	path, err := c.commands().LookPath(name)
	if err != nil || strings.TrimSpace(path) == "" {
		return "", false
	}
	return path, true
}

func (c Config) probe(ctx context.Context, component Component, explicit, name string, args ...string) ComponentState {
	state := ComponentState{
		Component:     component,
		Compatibility: CompatAbsent,
		Detail:        fmt.Sprintf("%s was not found; pass an explicit path if it is installed elsewhere on this machine", name),
	}
	path, ok := c.resolveBinary(explicit, name)
	if !ok {
		return state
	}
	state.Found = true
	state.Path = path

	result := c.commands().Run(ctx, CommandSpec{Name: path, Args: args})
	if result.Err != nil || result.ExitCode != 0 {
		state.Compatibility = CompatUnknown
		state.Detail = fmt.Sprintf("%s probe failed: %s", name, boundedDetail(result))
		return state
	}
	raw := strings.TrimSpace(result.Stdout)
	if raw == "" {
		raw = strings.TrimSpace(result.Stderr)
	}
	version, ok := parseVersion(raw)
	if !ok {
		state.Compatibility = CompatUnknown
		state.Detail = fmt.Sprintf("%s output could not be parsed as a version; refusing to guess", name)
		return state
	}
	state.Version = version
	return state
}

func (c Config) probeGodot(ctx context.Context) ComponentState {
	if strings.TrimSpace(c.GodotPath) != "" {
		state := c.probe(ctx, ComponentGodot, c.GodotPath, "godot", godotProbeArgs()...)
		classifyGodot(&state)
		return state
	}
	for _, name := range []string{"godot", "godot4"} {
		state := c.probe(ctx, ComponentGodot, "", name, godotProbeArgs()...)
		if state.Found {
			classifyGodot(&state)
			return state
		}
	}
	return ComponentState{
		Component:     ComponentGodot,
		Compatibility: CompatAbsent,
		Detail:        "Godot 4 was not found; pass an explicit executable path if it is installed elsewhere",
	}
}

func godotProbeArgs() []string {
	return []string{"--headless", "--version"}
}

func classifyGodot(state *ComponentState) {
	if state.Compatibility != CompatUnknown && state.Compatibility != CompatAbsent && state.Version != "" {
		if versionAtLeast(state.Version, minimumVersion[ComponentGodot]) && strings.HasPrefix(state.Version, "4.") {
			state.Compatibility = CompatCompatible
		} else {
			state.Compatibility = CompatIncompatible
		}
	}
}

func classifyMinimum(state *ComponentState, minimum string) {
	if !state.Found || state.Compatibility == CompatUnknown || state.Version == "" {
		return
	}
	if versionAtLeast(state.Version, minimum) {
		state.Compatibility = CompatCompatible
		return
	}
	state.Compatibility = CompatIncompatible
	state.Detail = fmt.Sprintf("version %s is below the required %s; refusing to replace it silently", state.Version, minimum)
}

func deferredPackageState(component Component, packageName string) ComponentState {
	return ComponentState{
		Component:     component,
		Compatibility: CompatUnknown,
		ProbeDeferred: true,
		Detail:        fmt.Sprintf("%s presence requires a post-consent `pi list` probe; preview must not initialize bootstrap settings", packageName),
	}
}

func detectPayload(workspace string) ComponentState {
	destination := payloadDestination(workspace)
	state := ComponentState{
		Component:     ComponentOGSPayload,
		Path:          destination,
		Compatibility: CompatAbsent,
	}
	total, identical, modified, missing := 0, 0, 0, 0
	for _, group := range ogsskills.Groups() {
		for _, rel := range group.Files {
			total++
			target := filepath.Join(destination, group.Dir, filepath.FromSlash(rel))
			if err := rejectSymlinkedPath(workspace, target); err != nil {
				state.Found = true
				state.Compatibility = CompatUnknown
				state.Detail = fmt.Sprintf("refusing to follow a symlinked payload path for %s/%s: %v", group.Dir, rel, err)
				return state
			}
			existing, err := os.ReadFile(target)
			if err != nil {
				if os.IsNotExist(err) {
					missing++
					continue
				}
				state.Compatibility = CompatUnknown
				state.Detail = fmt.Sprintf("cannot read existing payload file %s/%s: %v", group.Dir, rel, err)
				return state
			}
			embedded, err := ogsskills.ReadGroupFile(group.Dir, rel)
			if err != nil {
				state.Compatibility = CompatUnknown
				state.Detail = err.Error()
				return state
			}
			if bytes.Equal(existing, embedded) {
				identical++
				continue
			}
			modified++
		}
	}
	switch {
	case missing == total:
		state.Detail = "embedded OGS payload is not installed in the workspace"
	case modified == 0 && missing == 0:
		state.Found = true
		state.Compatibility = CompatCompatible
		state.Detail = "identical embedded OGS payload is already present"
	default:
		state.Found = true
		state.Compatibility = CompatModified
		state.Detail = fmt.Sprintf("payload present but %d file(s) differ; existing files are preserved, missing files are added", modified)
	}
	return state
}

func payloadDestination(workspace string) string {
	return filepath.Join(workspace, filepath.FromSlash(ogsskills.SkillsRoot))
}

func boundedDetail(result CommandResult) string {
	text := strings.TrimSpace(result.Stderr)
	if text == "" {
		text = strings.TrimSpace(result.Stdout)
	}
	if text == "" && result.Err != nil {
		text = result.Err.Error()
	}
	if len(text) > 400 {
		text = text[:400] + "…"
	}
	return text
}

// versionTuple is a simple major/minor/patch triple.
type versionTuple struct {
	major int
	minor int
	patch int
}

func parseVersionTuple(raw string) (versionTuple, bool) {
	match := semverPattern.FindStringSubmatch(raw)
	if match == nil {
		return versionTuple{}, false
	}
	major, _ := strconv.Atoi(match[1])
	minor, _ := strconv.Atoi(match[2])
	patch, _ := strconv.Atoi(match[3])
	return versionTuple{major: major, minor: minor, patch: patch}, true
}

func parseVersion(raw string) (string, bool) {
	tuple, ok := parseVersionTuple(raw)
	if !ok {
		return "", false
	}
	return fmt.Sprintf("%d.%d.%d", tuple.major, tuple.minor, tuple.patch), true
}

func versionAtLeast(raw, minimum string) bool {
	current, ok := parseVersionTuple(raw)
	if !ok {
		return false
	}
	floor, ok := parseVersionTuple(minimum)
	if !ok {
		return false
	}
	switch {
	case current.major != floor.major:
		return current.major > floor.major
	case current.minor != floor.minor:
		return current.minor > floor.minor
	default:
		return current.patch >= floor.patch
	}
}

type packageListClass string

const (
	listPresent    packageListClass = "present"
	listAbsent     packageListClass = "absent"
	listConflict   packageListClass = "conflict"
	listUnverified packageListClass = "unverified"
)

// piListRecord is one recognized package registration row.
type piListRecord struct {
	name     string
	version  string
	scope    string
	filtered bool
}

// recordIndent and detailIndent mirror the public Pi listing layout: a record
// source row is printed with two leading spaces and, beneath it, the installed
// path with four.
const (
	recordIndent = 2
	detailIndent = 4
)

// classifyPiPackageList parses the human-readable `pi list --no-approve` output.
// It is deliberately conservative: any line that does not match the verified
// public Pi 0.87.1 listing grammar fails closed as unverified, and conflicting,
// duplicated, filtered, or cross-scope registrations fail closed rather than
// being treated as absent or present.
func classifyPiPackageList(output, packageName, version string) (packageListClass, string) {
	cleaned := ansiPattern.ReplaceAllString(output, "")
	lines := strings.Split(cleaned, "\n")
	var records []piListRecord
	scope := ""
	sawContent := false
	sawHeading := false
	sawRecord := false
	headingRecords := 0
	emptyHeading := false
	noPackages := false
	prevWasRecord := false
	finalizeHeading := func() {
		if sawHeading && headingRecords == 0 {
			emptyHeading = true
		}
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		sawContent = true
		switch trimmed {
		case "User packages:", "Project packages:":
			finalizeHeading()
			sawHeading = true
			headingRecords = 0
			if trimmed == "User packages:" {
				scope = "user"
			} else {
				scope = "project"
			}
			prevWasRecord = false
			continue
		case "No packages installed.":
			noPackages = true
			prevWasRecord = false
			continue
		}
		indent, indented := leadingIndent(line)
		if !indented {
			return listUnverified, "unrecognized pi list line: " + boundedListingLine(trimmed)
		}
		if indent == detailIndent {
			// Pi prints the installed path at four spaces directly beneath its record.
			// Accept it only as that immediate producer detail line, and never scan
			// the path for the target.
			if prevWasRecord && looksLikePathDetail(trimmed) {
				prevWasRecord = false
				continue
			}
			return listUnverified, "unrecognized pi list line: " + boundedListingLine(trimmed)
		}
		if indent != recordIndent {
			return listUnverified, "unrecognized pi list line: " + boundedListingLine(trimmed)
		}
		prevWasRecord = false
		if strings.HasPrefix(trimmed, "git:") {
			if scope == "" {
				return listUnverified, "package entry appeared before a recognized section header"
			}
			entry := strings.TrimSuffix(trimmed, " (filtered)")
			payload := strings.TrimSpace(strings.TrimPrefix(entry, "git:"))
			if !supportedGitSource(payload) {
				// A bare, whitespace-only, unsupported-scheme, or opaque git/local
				// source is not a shape Pi documents; refuse to certify absence from it.
				return listUnverified, "unsupported or malformed git package source: " + boundedListingLine(trimmed)
			}
			sawRecord = true
			headingRecords++
			prevWasRecord = true
			continue
		}
		if strings.HasPrefix(trimmed, "npm:") {
			if scope == "" {
				return listUnverified, "package entry appeared before a recognized section header"
			}
			entry := strings.TrimSuffix(trimmed, " (filtered)")
			filtered := entry != trimmed
			name, sourceVersion, ok := splitNpmSource(entry)
			if !ok {
				return listUnverified, "unrecognized npm package record: " + boundedListingLine(trimmed)
			}
			records = append(records, piListRecord{name: name, version: sourceVersion, scope: scope, filtered: filtered})
			sawRecord = true
			headingRecords++
			prevWasRecord = true
			continue
		}
		// Any other two-space row is a configured source this classifier does not
		// recognize, such as an opaque local path. It is not a producer detail line
		// and it never proves the target absent.
		return listUnverified, "unrecognized package source row: " + boundedListingLine(trimmed)
	}
	finalizeHeading()
	if !sawContent {
		return listUnverified, "package listing was empty"
	}
	if emptyHeading {
		return listUnverified, "a package section header was present without any records; the listing may be truncated or hiding entries"
	}
	if noPackages {
		if sawHeading {
			return listUnverified, "package listing reported no packages alongside package section headers; ambiguous listing"
		}
		if len(records) > 0 {
			return listUnverified, "package listing reported no packages but also listed records"
		}
		return listAbsent, "no packages are registered"
	}
	if !sawRecord {
		return listUnverified, "package listing contained no recognized package records; refusing to certify absence"
	}

	var matches []piListRecord
	for _, record := range records {
		if record.name == packageName {
			matches = append(matches, record)
		}
	}
	if len(matches) == 0 {
		return listAbsent, "pinned package is not registered"
	}
	for _, match := range matches {
		if match.filtered {
			return listUnverified, "the registered package is marked filtered/hidden; refusing to treat it as an active registration"
		}
	}
	scopes := map[string]bool{}
	for _, match := range matches {
		scopes[match.scope] = true
	}
	if len(scopes) > 1 {
		return listConflict, "package is registered in more than one scope; ambiguous registration"
	}
	versions := map[string]bool{}
	for _, match := range matches {
		versions[match.version] = true
	}
	if len(versions) > 1 {
		return listConflict, fmt.Sprintf("registered %s versions conflict: %s", packageName, strings.Join(sortedKeys(versions), ", "))
	}
	if len(matches) > 1 {
		return listUnverified, "package is registered more than once in the same scope; ambiguous registration"
	}
	only := matches[0]
	if only.scope != "user" {
		return listConflict, "package is registered only in project scope; a personal install would duplicate it"
	}
	if only.version != version {
		return listConflict, fmt.Sprintf("registered %s@%s conflicts with pinned %s", only.name, only.version, version)
	}
	return listPresent, "exact pinned package is already registered"
}

// leadingIndent returns the count of leading spaces in line and whether the
// indentation is a plain run of spaces. A tab or mixed prefix is ambiguous for
// the public two-space/four-space layout and reported as not-ok so the listing
// fails closed.
func leadingIndent(line string) (int, bool) {
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case ' ':
			continue
		case '\t':
			return 0, false
		default:
			return i, true
		}
	}
	return len(line), true
}

// looksLikePathDetail reports whether a four-space row is the installed-path
// detail Pi prints beneath a recognized record. The producer emits an absolute
// path; anything else at that depth is ambiguous and stays unverified.
func looksLikePathDetail(trimmed string) bool {
	return filepath.IsAbs(trimmed)
}

func boundedListingLine(line string) string {
	if len(line) > 120 {
		return line[:120] + "…"
	}
	return line
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// npmNameSegmentPattern and npmVersionPattern are the small validated subset
// this classifier accepts for an npm record. They are deliberately narrow: a
// registry name is a lowercase unscoped segment or an @scope/segment pair, and a
// version is a plain semver triple (optionally with prerelease/build text).
// Aliases, ranges, and other npm specs stay unverified; this is not a general
// npm-spec parser and does not claim any spec it rejects is invalid for npm.
var npmNameSegmentPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._-]*[a-z0-9])?$`)
var npmVersionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)

// splitNpmSource recognizes "npm:<name>@<version>" only when the name passes the
// conservative registry-name grammar and the version is a plain semver. A bare
// "@" prefix, an empty scope, a missing version, a range, and any other shape are
// refused instead of counting as a supported registration.
func splitNpmSource(source string) (name, version string, ok bool) {
	if !strings.HasPrefix(source, "npm:") {
		return "", "", false
	}
	spec := strings.TrimPrefix(source, "npm:")
	at := strings.LastIndexByte(spec, '@')
	if at < 0 {
		return "", "", false
	}
	name = spec[:at]
	version = spec[at+1:]
	if !validNpmPackageName(name) || !npmVersionPattern.MatchString(version) {
		return "", "", false
	}
	return name, version, true
}

// validNpmPackageName accepts an unscoped registry name or an @scope/name pair.
// It requires lowercase alphanumerics bounded by alphanumerics, allows the
// normal `-`, `_`, and `.` separators internally, rejects a bare/empty scope, and
// enforces npm's 214-character ceiling.
func validNpmPackageName(name string) bool {
	if name == "" || len(name) > 214 {
		return false
	}
	if strings.HasPrefix(name, "@") {
		slash := strings.IndexByte(name, '/')
		if slash < 0 {
			return false
		}
		scope := name[1:slash]
		rest := name[slash+1:]
		if rest == "" || strings.ContainsRune(rest, '/') {
			return false
		}
		return npmNameSegmentPattern.MatchString(scope) && npmNameSegmentPattern.MatchString(rest)
	}
	if strings.ContainsRune(name, '/') {
		return false
	}
	return npmNameSegmentPattern.MatchString(name)
}
