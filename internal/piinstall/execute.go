package piinstall

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	ogsskills "open-game-studios/skills"
)

// Execute runs an approved plan against the host. It refuses any plan that is
// not explicitly approved with a matching fingerprint, refuses a plan with a
// blocked step, re-probes to catch a stale proposal, and stops on the first
// failure without rolling back completed steps. There is no implicit approval.
func Execute(ctx context.Context, cfg Config, plan Plan, consent Consent) (Report, error) {
	cfg = cfg.withDefaults()
	if err := cfg.validate(); err != nil {
		return Report{}, err
	}

	if !consent.Approved {
		report := Report{Consent: consent, Steps: plan.Steps}
		for _, step := range plan.Steps {
			report.Entries = append(report.Entries, Entry{
				Component: step.Component,
				Outcome:   OutcomeDeclined,
				Detail:    step.Reason,
			})
		}
		return report, ErrConsentRequired
	}
	if consent.PlanFingerprint != plan.Fingerprint() {
		return Report{}, ErrPlanDrift
	}
	// Bind execution to the config the human approved. Recompute only the
	// normalized binding against the current config while keeping the approved
	// plan's displayed fields (including the public GodotRequired flag and every
	// step). A changed install root, workspace, Godot requirement, explicit path,
	// or any tampered step field changes the digest and refuses here, before any
	// write or installer command.
	approved := plan
	approved.binding = cfg.planBinding()
	if consent.PlanFingerprint != fingerprintOf(approved) {
		return Report{}, fmt.Errorf("%w: execution configuration differs from the approved plan", ErrPlanDrift)
	}
	for _, step := range plan.Steps {
		if step.Action == ActionBlocked {
			return Report{}, fmt.Errorf("%w: %s: %s", ErrPlanBlocked, step.Component, step.Reason)
		}
	}

	exec := &executor{cfg: cfg, plan: plan, report: Report{Consent: consent, Steps: plan.Steps}}

	fresh, err := Detect(ctx, cfg)
	if err != nil {
		exec.report.Failed = true
		exec.report.Entries = append(exec.report.Entries, Entry{
			Component: ComponentOGSPayload,
			Outcome:   OutcomeUnverified,
			Detail:    "pre-execution re-probe failed: " + err.Error(),
		})
		return exec.report, nil
	}
	for _, step := range plan.Steps {
		if step.Observed.ProbeDeferred {
			continue
		}
		now, _ := fresh.State(step.Component)
		if drift(step.Observed, now) {
			exec.report.Failed = true
			exec.report.Entries = append(exec.report.Entries, Entry{
				Component: step.Component,
				Outcome:   OutcomeFailed,
				Detail:    "approved plan is stale: re-probe changed the observed state; refusing to guess",
				Path:      now.Path,
				Version:   now.Version,
			})
			return exec.report, nil
		}
	}

	if err := makeDirsSafe(cfg.InstallRoot); err != nil {
		exec.report.Failed = true
		exec.report.Entries = append(exec.report.Entries, Entry{
			Component: ComponentNode,
			Outcome:   OutcomeFailed,
			Detail:    "cannot prepare owned install root: " + err.Error(),
		})
		return exec.report, nil
	}
	// Revalidate the now-existing owned root before any managed probe or write.
	// A missing root was only validated up to its nearest existing ancestor.
	if err := validateOwnedRoot(cfg.InstallRoot); err != nil {
		exec.report.Failed = true
		exec.report.Entries = append(exec.report.Entries, Entry{
			Component: ComponentNode,
			Outcome:   OutcomeFailed,
			Detail:    "created install root failed ownership validation: " + err.Error(),
		})
		return exec.report, nil
	}

	for _, step := range plan.Steps {
		entry, stop := exec.runStep(ctx, step)
		exec.report.Entries = append(exec.report.Entries, entry)
		if stop {
			exec.report.Failed = true
			break
		}
	}
	return exec.report, nil
}

func drift(planned, now ComponentState) bool {
	return planned.Found != now.Found ||
		planned.Path != now.Path ||
		planned.Version != now.Version ||
		planned.Compatibility != now.Compatibility
}

type executor struct {
	cfg    Config
	plan   Plan
	report Report

	nodeBinDir    string
	npmBinDir     string
	npmExecutable string
	piExecutable  string
	piPrefix      string
}

func (e *executor) runStep(ctx context.Context, step PlanStep) (Entry, bool) {
	switch step.Action {
	case ActionReuse:
		e.recordReuse(step.Component, step.Observed.Path, step.Observed.Version)
		return Entry{
			Component: step.Component,
			Outcome:   OutcomeReused,
			Detail:    step.Reason,
			Path:      step.Observed.Path,
			Version:   step.Observed.Version,
		}, false
	case ActionEnsure:
		return e.ensurePackage(ctx, step)
	case ActionInstall:
		return e.installStep(ctx, step)
	default:
		return Entry{Component: step.Component, Outcome: OutcomeBlocked, Detail: step.Reason}, true
	}
}

func (e *executor) installStep(ctx context.Context, step PlanStep) (Entry, bool) {
	switch step.Component {
	case ComponentNode:
		return e.installNode(ctx)
	case ComponentNpm:
		return e.provideNpm(ctx)
	case ComponentPi:
		return e.installPi(ctx)
	case ComponentEngramCore:
		candidate, _ := CandidateFor(ComponentEngramCore)
		return e.installArchivedBinary(ctx, ComponentEngramCore, candidate, "engram", "engram", 0, extractTarGz)
	case ComponentGodot:
		candidate, _ := CandidateFor(ComponentGodot)
		return e.installArchivedBinary(ctx, ComponentGodot, candidate, "Godot_v4.7.2-stable_linux.x86_64", "Godot_v4.7.2-stable_linux.x86_64", 0, extractZip)
	case ComponentOGSPayload:
		return e.installPayloadStep()
	default:
		return Entry{Component: step.Component, Outcome: OutcomeFailed, Detail: "no installer is implemented for this component"}, true
	}
}

func (e *executor) installNode(ctx context.Context) (Entry, bool) {
	entry, stop := e.ensureNodeBundle(ctx)
	if !stop && entry.Path != "" {
		e.selectNodeRuntime(filepath.Dir(entry.Path))
	}
	return entry, stop
}

// ensureNodeBundle acquires the pinned Node archive into the owned root and
// returns the verified `bin/node` entry. It deliberately performs no runtime
// selection, so a caller can obtain npm from the bundle without displacing an
// already-selected Node. This is the only place the Node archive is acquired.
func (e *executor) ensureNodeBundle(ctx context.Context) (Entry, bool) {
	candidate, _ := CandidateFor(ComponentNode)
	return e.ensureArchivedBinary(ctx, ComponentNode, candidate, "node", "bin/node", 1, extractTarGz)
}

// selectNodeRuntime records the Node bin directory used by later managed
// commands, along with the npm that ships beside it.
func (e *executor) selectNodeRuntime(binDir string) {
	e.nodeBinDir = binDir
	e.report.Launch.NodeBinDir = binDir
	e.npmExecutable = filepath.Join(binDir, "npm")
	e.npmBinDir = binDir
	e.report.Launch.NpmExecutable = e.npmExecutable
}

// provideNpm resolves and verifies the npm used for managed installs. When the
// Node step already installed the pinned bundle, its npm is probed before it is
// reported; a missing or non-runnable npm is a failure, never a success. When a
// compatible Node was reused but npm is absent, it installs the same pinned
// official Node bundle into the owned root purely as an npm provider; the
// selected Node runtime (and Launch.NodeBinDir) stays that existing Node, so the
// provider's bin directory never wins Node lookup. The existing Node is never
// replaced, reconfigured, or modified.
func (e *executor) provideNpm(ctx context.Context) (Entry, bool) {
	if e.npmExecutable != "" {
		version, ok := e.probeNpm(ctx, e.npmExecutable)
		if !ok {
			return e.fail(ComponentNpm, "the resolved npm did not run under the managed Node environment; refusing to report success", e.npmExecutable), true
		}
		return Entry{Component: ComponentNpm, Outcome: OutcomeReused, Detail: fmt.Sprintf("verified npm %s provided by the resolved Node install", version), Path: e.npmExecutable, Version: version}, false
	}
	entry, stop := e.ensureNodeBundle(ctx)
	if stop {
		return Entry{Component: ComponentNpm, Outcome: entry.Outcome, Detail: "installing the pinned Node bundle to provide npm: " + entry.Detail, Path: entry.Path}, true
	}
	providerNpm := filepath.Join(filepath.Dir(entry.Path), "npm")
	version, ok := e.probeNpm(ctx, providerNpm)
	if !ok {
		return e.fail(ComponentNpm, "the installed Node bundle did not provide a runnable npm; state is partial", providerNpm), true
	}
	e.npmExecutable = providerNpm
	e.npmBinDir = filepath.Dir(providerNpm)
	e.report.Launch.NpmExecutable = providerNpm
	detail := "installed the pinned official Node bundle to provide npm; the existing Node on PATH was not modified"
	if entry.Outcome == OutcomeReused {
		detail = "reused the managed Node bundle's npm; the existing Node on PATH was not modified"
	}
	return Entry{Component: ComponentNpm, Outcome: entry.Outcome, Detail: detail, Path: providerNpm, Version: version}, false
}

// probeNpm runs npm --version with the managed Node bin directory on the child
// PATH so npm's Node-backed entry point resolves the managed runtime. It accepts
// a symlinked npm (Node's normal in-root layout) and requires the reported
// version to meet the npm minimum.
func (e *executor) probeNpm(ctx context.Context, npmPath string) (string, bool) {
	if npmPath == "" {
		return "", false
	}
	if _, err := os.Lstat(npmPath); err != nil {
		return "", false
	}
	result := e.cfg.commands().Run(ctx, CommandSpec{Name: npmPath, Args: []string{"--version"}, Env: e.runtimeEnv()})
	if result.Err != nil || result.ExitCode != 0 {
		return "", false
	}
	version, ok := parseVersion(result.Stdout + "\n" + result.Stderr)
	if !ok {
		return "", false
	}
	minimum, ok := minimumVersion[ComponentNpm]
	if !ok || !versionAtLeast(version, minimum) {
		return "", false
	}
	return version, true
}

// installArchivedBinary ensures one archived binary is present and records the
// resulting launch state for Engram and Godot. Node is handled separately via
// ensureNodeBundle/selectNodeRuntime so obtaining the bundle never forces Node
// runtime selection.
func (e *executor) installArchivedBinary(ctx context.Context, component Component, candidate Candidate, binaryName, binaryRel string, strip int, extract func(dst, archivePath string, opts ExtractOptions) error) (Entry, bool) {
	entry, stop := e.ensureArchivedBinary(ctx, component, candidate, binaryName, binaryRel, strip, extract)
	if !stop && entry.Path != "" {
		e.recordReuse(component, entry.Path, entry.Version)
	}
	return entry, stop
}

// ensureArchivedBinary downloads, verifies, extracts, and re-probes one archived
// binary at its planned destination. It never records runtime selection, so a
// caller that only wants the bytes does not overwrite an existing selection.
func (e *executor) ensureArchivedBinary(ctx context.Context, component Component, candidate Candidate, binaryName, binaryRel string, strip int, extract func(dst, archivePath string, opts ExtractOptions) error) (Entry, bool) {
	finalDir := filepath.Join(e.cfg.InstallRoot, componentDir(component), candidate.Version)
	expectedRel := filepath.FromSlash(binaryRel)
	finalBinary := filepath.Join(finalDir, expectedRel)
	if version := e.reuseBinary(ctx, finalBinary, component); version != "" {
		return Entry{Component: component, Outcome: OutcomeReused, Detail: "existing managed binary at the planned destination", Path: finalBinary, Version: version}, false
	}
	if anyExists(finalDir) {
		return e.fail(component, "destination already exists with an unusable binary; refusing to overwrite", finalDir), true
	}

	staging, err := os.MkdirTemp(e.cfg.InstallRoot, ".ogs-"+string(component)+"-")
	if err != nil {
		return e.fail(component, "create owned staging directory: "+err.Error(), ""), true
	}
	// Cleanup only the staging directory this call created.
	defer os.RemoveAll(staging)

	archivePath := filepath.Join(staging, "download")
	if err := e.download(ctx, candidate, archivePath); err != nil {
		return e.fail(component, err.Error(), ""), true
	}
	digest, size := e.expectedDigest(candidate)
	if err := verifyArchive(archivePath, digest, size); err != nil {
		return e.fail(component, err.Error(), ""), true
	}
	tree := filepath.Join(staging, "tree")
	if err := extract(tree, archivePath, ExtractOptions{StripComponents: strip}); err != nil {
		return e.fail(component, "extract archive: "+err.Error(), ""), true
	}
	binaryPath, err := findRegularFile(tree, binaryName, 4)
	if err != nil {
		return e.fail(component, "locate extracted binary: "+err.Error(), ""), true
	}
	if err := os.Chmod(binaryPath, 0o700); err != nil {
		return e.fail(component, "mark extracted binary executable: "+err.Error(), ""), true
	}
	relative, err := filepath.Rel(tree, binaryPath)
	if err != nil {
		return e.fail(component, "resolve extracted binary path: "+err.Error(), ""), true
	}
	if relative != expectedRel {
		return e.fail(component, fmt.Sprintf("extracted binary at %s does not match the expected layout %s; refusing to move into place", relative, expectedRel), ""), true
	}
	if err := makeDirsSafe(filepath.Dir(finalDir)); err != nil {
		return e.fail(component, "prepare destination parent: "+err.Error(), ""), true
	}
	if anyExists(finalDir) {
		return e.fail(component, "destination appeared during install; refusing to overwrite", finalDir), true
	}
	if err := os.Rename(tree, finalDir); err != nil {
		return e.fail(component, "move verified tree into place: "+err.Error(), ""), true
	}

	installedBinary := finalBinary
	version := e.reuseBinary(ctx, installedBinary, component)
	if version == "" {
		return e.fail(component, "installed binary failed read-only verification; state is partial", installedBinary), true
	}
	return Entry{Component: component, Outcome: OutcomeInstalled, Detail: "verified against the pinned SHA-256 and re-probed", Path: installedBinary, Version: version}, false
}

func (e *executor) installPi(ctx context.Context) (Entry, bool) {
	candidate, _ := CandidateFor(ComponentPi)
	prefix := filepath.Join(e.cfg.InstallRoot, componentDir(ComponentPi), candidate.Version)
	piBinary := filepath.Join(prefix, "bin", "pi")
	if version := e.reuseBinary(ctx, piBinary, ComponentPi); version != "" {
		e.piExecutable = piBinary
		e.piPrefix = prefix
		e.recordReuse(ComponentPi, piBinary, version)
		return Entry{Component: ComponentPi, Outcome: OutcomeReused, Detail: "existing managed Pi at the planned prefix", Path: piBinary, Version: version}, false
	}
	if anyExists(prefix) {
		return e.fail(ComponentPi, "npm prefix already exists without a usable Pi; refusing to overwrite", prefix), true
	}
	npm := e.npmExecutable
	if npm == "" {
		return e.fail(ComponentPi, "no usable npm was resolved; cannot install Pi", ""), true
	}
	home := filepath.Join(e.cfg.InstallRoot, "home")
	if err := makeDirsSafe(home); err != nil {
		return e.fail(ComponentPi, "prepare scoped npm HOME: "+err.Error(), ""), true
	}
	cache := filepath.Join(e.cfg.InstallRoot, "npm-cache")
	if err := makeDirsSafe(cache); err != nil {
		return e.fail(ComponentPi, "prepare scoped npm cache: "+err.Error(), ""), true
	}

	spec := CommandSpec{
		Name: npm,
		Args: []string{"install", "--global", "--prefix", prefix, "--ignore-scripts", PiNpmPackage},
		Env:  e.npmEnv(prefix, home, cache),
	}
	result := e.cfg.commands().Run(ctx, spec)
	if result.Err != nil || result.ExitCode != 0 {
		return e.fail(ComponentPi, "npm install failed: "+boundedDetail(result), ""), true
	}

	version := e.reuseBinary(ctx, piBinary, ComponentPi)
	if version == "" {
		return e.fail(ComponentPi, "npm reported success but the managed Pi binary did not verify; state is partial", piBinary), true
	}
	e.piExecutable = piBinary
	e.piPrefix = prefix
	e.report.Launch.PiExecutable = piBinary
	e.report.Launch.PiPrefix = prefix
	return Entry{Component: ComponentPi, Outcome: OutcomeInstalled, Detail: "installed with --ignore-scripts and re-probed", Path: piBinary, Version: version}, false
}

func (e *executor) ensurePackage(ctx context.Context, step PlanStep) (Entry, bool) {
	candidate, _ := CandidateFor(step.Component)
	packageName, _, _ := splitNpmSource(candidate.Source)
	if e.piExecutable == "" {
		return e.fail(step.Component, "no usable Pi executable is available; cannot manage packages", ""), true
	}

	classification, detail := e.classifyPackages(ctx, candidate.Source, packageName, candidate.Version)
	switch classification {
	case listPresent:
		return Entry{Component: step.Component, Outcome: OutcomeReused, Detail: detail}, false
	case listConflict:
		return e.fail(step.Component, "conflicting package registration: "+detail, ""), true
	case listUnverified:
		return Entry{Component: step.Component, Outcome: OutcomeUnverified, Detail: detail}, true
	}

	spec := CommandSpec{
		Name: e.piExecutable,
		Args: []string{"install", candidate.Source, "--no-approve"},
		Env:  e.piEnv(),
	}
	result := e.cfg.commands().Run(ctx, spec)
	if result.Err != nil || result.ExitCode != 0 {
		return e.fail(step.Component, "pi install failed: "+boundedDetail(result), ""), true
	}
	post, postDetail := e.classifyPackages(ctx, candidate.Source, packageName, candidate.Version)
	if post == listPresent {
		return Entry{Component: step.Component, Outcome: OutcomeInstalled, Detail: postDetail}, false
	}
	return Entry{Component: step.Component, Outcome: OutcomeUnverified, Detail: "package install could not be verified: " + postDetail}, true
}

func (e *executor) classifyPackages(ctx context.Context, source, packageName, version string) (packageListClass, string) {
	spec := CommandSpec{
		Name: e.piExecutable,
		Args: []string{"list", "--no-approve"},
		Env:  e.piEnv(),
	}
	result := e.cfg.commands().Run(ctx, spec)
	if result.Err != nil || result.ExitCode != 0 {
		return listUnverified, "pi list failed: " + boundedDetail(result)
	}
	return classifyPiPackageList(result.Stdout+"\n"+result.Stderr, packageName, version)
}

func (e *executor) installPayloadStep() (Entry, bool) {
	report, err := installPayload(e.cfg.WorkspaceDir)
	if err != nil {
		return e.fail(ComponentOGSPayload, "copy embedded payload: "+err.Error(), ""), true
	}
	e.report.Payload = report
	installed, preserved, identical := 0, 0, 0
	for _, file := range report.Files {
		switch file.Status {
		case PayloadInstalled:
			installed++
		case PayloadPreserved:
			preserved++
		case PayloadIdentical:
			identical++
		}
	}
	e.report.Notes = append(e.report.Notes,
		"Pi must be restarted, or /reload run in an active session, to pick up the skills under "+ogsskills.SkillsRoot+"; project trust must be granted for project-local skills. This installer does not live-load the skills.")
	switch {
	case installed == 0 && preserved == 0:
		return Entry{Component: ComponentOGSPayload, Outcome: OutcomeReused, Detail: "identical embedded payload already present", Path: report.Destination}, false
	case preserved == 0:
		return Entry{Component: ComponentOGSPayload, Outcome: OutcomeInstalled, Detail: "embedded payload written without overwriting modified files", Path: report.Destination}, false
	case installed == 0:
		// No embedded file was written but at least one existing file differs.
		// This is not a reuse: the managed payload is unverified. Other existing
		// files may still be byte-identical, so the diagnostic counts them
		// instead of claiming every embedded file differs.
		return Entry{Component: ComponentOGSPayload, Outcome: OutcomeUnverified, Detail: fmt.Sprintf("%d modified payload file(s) differ from the embedded copies and were preserved unchanged; %d identical file(s) were left unchanged; nothing was installed and the managed payload is unverified", preserved, identical), Path: report.Destination}, true
	default:
		// Some files were installed, but any differing preserved file keeps the
		// aggregate payload unverified and stops the plan. The successful
		// file-level writes stay in place and are reported accurately.
		return Entry{Component: ComponentOGSPayload, Outcome: OutcomeUnverified, Detail: fmt.Sprintf("%d embedded payload file(s) were installed; %d modified file(s) differ from the embedded copies and were preserved unchanged; the managed payload is unverified", installed, preserved), Path: report.Destination}, true
	}
}

func (e *executor) download(ctx context.Context, candidate Candidate, destPath string) error {
	maxBytes := candidate.Size
	if override, ok := e.cfg.archiveDigests[candidate.Component]; ok {
		maxBytes = override.size
	}
	if maxBytes <= 0 {
		// Size not pinned: use a generous but bounded cap.
		maxBytes = int64(2) << 30
	}
	return e.cfg.downloader().Download(ctx, candidate.Source, destPath, maxBytes)
}

func (e *executor) expectedDigest(candidate Candidate) (string, int64) {
	if override, ok := e.cfg.archiveDigests[candidate.Component]; ok {
		return override.sha, override.size
	}
	return candidate.SHA256, candidate.Size
}

func (e *executor) reuseBinary(ctx context.Context, path string, component Component) string {
	if !fileExists(path) {
		return ""
	}
	// Run with the managed runtime environment so a Node-backed launcher (the
	// Pi binary, or npm's JS entry point) resolves the managed Node bin dir even
	// when it is not on the parent PATH.
	result := e.cfg.commands().Run(ctx, CommandSpec{Name: path, Args: versionArgs(component), Env: e.runtimeEnv()})
	if result.Err != nil || result.ExitCode != 0 {
		return ""
	}
	version, ok := parseVersion(result.Stdout + "\n" + result.Stderr)
	if !ok {
		return ""
	}
	minimum, ok := minimumVersion[component]
	if !ok || !versionAtLeast(version, minimum) {
		return ""
	}
	return version
}

func (e *executor) recordReuse(component Component, path, version string) {
	if path == "" {
		return
	}
	switch component {
	case ComponentNode:
		e.nodeBinDir = filepath.Dir(path)
		e.report.Launch.NodeBinDir = e.nodeBinDir
	case ComponentNpm:
		e.npmExecutable = path
		e.npmBinDir = filepath.Dir(path)
		e.report.Launch.NpmExecutable = path
	case ComponentPi:
		e.piExecutable = path
		e.report.Launch.PiExecutable = path
	}
	e.recordLaunch(component, path)
}

func (e *executor) recordLaunch(component Component, path string) {
	switch component {
	case ComponentEngramCore:
		e.report.Launch.EngramBinary = path
		e.report.Launch.PathAdditions = appendUnique(e.report.Launch.PathAdditions, filepath.Dir(path))
	case ComponentGodot:
		e.report.Launch.GodotBinary = path
	}
}

func (e *executor) fail(component Component, detail, path string) Entry {
	return Entry{Component: component, Outcome: OutcomeFailed, Detail: detail, Path: path}
}

// runtimeEnv builds the environment for a managed child process. The managed
// Node bin directory is always prepended when known so Node-backed launchers do
// not depend on the host PATH. extraDirs are additional tool directories.
func (e *executor) runtimeEnv(extraDirs ...string) []string {
	dirs := make([]string, 0, len(extraDirs)+1)
	if e.nodeBinDir != "" {
		dirs = append(dirs, e.nodeBinDir)
	}
	dirs = append(dirs, extraDirs...)
	env := []string{"PATH=" + joinToolPath(os.Getenv("PATH"), dirs...)}
	if home := os.Getenv("HOME"); home != "" {
		env = append(env, "HOME="+home)
	}
	return env
}

// npmEnv scopes npm's HOME, prefix, and cache to the owned root. PATH carries
// the selected Node runtime so npm's Node-backed entry point resolves it; the
// npm provider's own bin directory is intentionally omitted so an npm-provider
// bundle can never displace the selected Node.
func (e *executor) npmEnv(prefix, home, cache string) []string {
	dirs := make([]string, 0, 1)
	if e.nodeBinDir != "" {
		dirs = append(dirs, e.nodeBinDir)
	}
	return []string{
		"PATH=" + joinToolPath(os.Getenv("PATH"), dirs...),
		"HOME=" + home,
		"npm_config_prefix=" + prefix,
		"npm_config_cache=" + cache,
		"npm_config_userconfig=" + filepath.Join(e.cfg.InstallRoot, "npmrc"),
		"npm_config_fund=false",
		"npm_config_audit=false",
		"npm_config_update_notifier=false",
	}
}

func (e *executor) piEnv() []string {
	dirs := make([]string, 0, 2)
	if e.piExecutable != "" {
		dirs = append(dirs, filepath.Dir(e.piExecutable))
	}
	// Pi's package manager spawns `npm` by PATH when it installs a package. The
	// npm provider's bin directory must therefore be reachable, but it must come
	// after the selected Node bin (runtimeEnv always prepends nodeBinDir first)
	// so the chosen Node still wins `node` lookup. Without this, a later
	// `pi install npm:...` cannot find npm at all.
	if e.npmBinDir != "" {
		dirs = append(dirs, e.npmBinDir)
	}
	env := e.runtimeEnv(dirs...)
	if engram := e.report.Launch.EngramBinary; engram != "" {
		env = append(env, "ENGRAM_BIN="+engram)
	}
	return env
}

func versionArgs(component Component) []string {
	if component == ComponentGodot {
		return godotProbeArgs()
	}
	return []string{"--version"}
}

func componentDir(component Component) string {
	switch component {
	case ComponentNode:
		return "node"
	case ComponentEngramCore:
		return "engram"
	case ComponentGodot:
		return "godot"
	default:
		return string(component)
	}
}

func appendUnique(items []string, value string) []string {
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}

// installPayload copies the embedded canonical skill into the workspace,
// preserving any modified or unowned existing files.
func installPayload(workspace string) (PayloadReport, error) {
	destination := payloadDestination(workspace)
	report := PayloadReport{Destination: destination}
	for _, group := range ogsskills.Groups() {
		for _, rel := range group.Files {
			relativePath := group.Dir + "/" + rel
			embedded, err := ogsskills.ReadGroupFile(group.Dir, rel)
			if err != nil {
				return report, err
			}
			target := filepath.Join(destination, group.Dir, filepath.FromSlash(rel))
			if err := rejectSymlinkedPath(workspace, target); err != nil {
				return report, err
			}
			existing, err := os.ReadFile(target)
			if err == nil {
				if bytes.Equal(existing, embedded) {
					report.Files = append(report.Files, PayloadFileResult{RelativePath: relativePath, Status: PayloadIdentical})
				} else {
					report.Files = append(report.Files, PayloadFileResult{RelativePath: relativePath, Status: PayloadPreserved})
				}
				continue
			}
			if !os.IsNotExist(err) {
				return report, err
			}
			if err := makeDirsSafe(filepath.Dir(target)); err != nil {
				return report, err
			}
			if err := checkDestinationLeaf(target); err != nil {
				return report, err
			}
			file, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
			if err != nil {
				return report, err
			}
			if _, err := file.Write(embedded); err != nil {
				file.Close()
				return report, err
			}
			if err := file.Close(); err != nil {
				return report, err
			}
			report.Files = append(report.Files, PayloadFileResult{RelativePath: relativePath, Status: PayloadInstalled})
		}
	}
	return report, nil
}
