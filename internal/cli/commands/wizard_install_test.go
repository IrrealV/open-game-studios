package commands

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"open-game-studios/internal/persistence"
	"open-game-studios/internal/piinstall"
	"open-game-studios/internal/templates"
	"open-game-studios/internal/toolcheck"
)

// fakeWizardInstaller is the deterministic wizard-owned installer seam. It lets
// the runtime tests assert call counts, the exact config, and the consent
// binding without touching the accepted backend or the host.
type fakeWizardInstaller struct {
	prepareCalls int
	executeCalls int

	plan       piinstall.Plan
	prepareErr error
	executeErr error
	report     piinstall.Report

	cancelOnExecute context.CancelFunc

	lastConfig  piinstall.Config
	lastConsent piinstall.Consent
}

func (f *fakeWizardInstaller) Prepare(_ context.Context, cfg piinstall.Config) (piinstall.Detection, piinstall.Plan, error) {
	f.prepareCalls++
	f.lastConfig = cfg
	if f.prepareErr != nil {
		return piinstall.Detection{}, piinstall.Plan{}, f.prepareErr
	}
	return piinstall.Detection{}, f.plan, nil
}

func (f *fakeWizardInstaller) Execute(_ context.Context, cfg piinstall.Config, plan piinstall.Plan, consent piinstall.Consent) (piinstall.Report, error) {
	f.executeCalls++
	f.lastConfig = cfg
	f.lastConsent = consent
	if f.cancelOnExecute != nil {
		f.cancelOnExecute()
	}
	return f.report, f.executeErr
}

func fakeWizardPlan() piinstall.Plan {
	return piinstall.Plan{
		GodotRequired: true,
		Steps: []piinstall.PlanStep{
			{
				Component: piinstall.ComponentNode,
				Action:    piinstall.ActionInstall,
				Reason:    "node is missing; install the pinned candidate",
				Command:   "download+verify+extract node",
				Outputs:   []string{"/owned/node/24.21.0"},
				Effects:   []string{"downloads over https and verifies the pinned SHA-256"},
				Observed:  piinstall.ComponentState{Component: piinstall.ComponentNode, Compatibility: piinstall.CompatAbsent},
			},
			{
				Component: piinstall.ComponentNpm,
				Action:    piinstall.ActionInstall,
				Reason:    "npm is provided by the managed Node install",
				Command:   "(provided by the managed Node archive)",
				Observed:  piinstall.ComponentState{Component: piinstall.ComponentNpm, Compatibility: piinstall.CompatAbsent},
			},
			{
				Component: piinstall.ComponentPi,
				Action:    piinstall.ActionInstall,
				Reason:    "Pi is missing; install the pinned package into an owned npm prefix",
				Command:   "npm install --global --prefix /owned/pi/0.87.1 --ignore-scripts @earendil-works/pi-coding-agent@0.87.1",
				Effects:   []string{"scopes the child PATH, HOME, and npm cache under the owned install root"},
				Observed:  piinstall.ComponentState{Component: piinstall.ComponentPi, Compatibility: piinstall.CompatAbsent},
			},
			{
				Component: piinstall.ComponentShell,
				Action:    piinstall.ActionEnsure,
				Reason:    "reuse an exact gentle-pi registration or install it after consent",
				Command:   "pi install npm:gentle-pi@3.7.0 --no-approve",
				Effects:   []string{"initializes Pi bootstrap settings as a side effect of the package command"},
				Observed:  piinstall.ComponentState{Component: piinstall.ComponentShell, Compatibility: piinstall.CompatUnknown, ProbeDeferred: true},
			},
			{
				Component: piinstall.ComponentEngramCore,
				Action:    piinstall.ActionInstall,
				Reason:    "engram core is missing; install the pinned candidate",
				Command:   "download+verify+extract engram core",
				Outputs:   []string{"/owned/engram/2.2.0"},
				Effects:   []string{"downloads the official Engram tar.gz over https and verifies its pinned SHA-256 and size"},
				Observed:  piinstall.ComponentState{Component: piinstall.ComponentEngramCore, Compatibility: piinstall.CompatAbsent},
			},
			{
				Component: piinstall.ComponentEngramCompanion,
				Action:    piinstall.ActionEnsure,
				Reason:    "reuse an exact gentle-engram registration or install it after consent",
				Command:   "pi install npm:gentle-engram@0.1.15 --no-approve",
				Effects:   []string{"requires the Engram core binary through PATH or ENGRAM_BIN"},
				Observed:  piinstall.ComponentState{Component: piinstall.ComponentEngramCompanion, Compatibility: piinstall.CompatUnknown, ProbeDeferred: true},
			},
			{
				Component: piinstall.ComponentOGSPayload,
				Action:    piinstall.ActionInstall,
				Reason:    "copy the embedded canonical OGS skills into the workspace",
				Command:   "copy embedded payload",
				Effects:   []string{"writes only the embedded OGS skill files under .pi/skills: ogs-godot-change/SKILL.md, ogs-godot-change/references/godot-setup.md, ogs-core/SKILL.md, ogs-core/references/handoff-contract.md"},
				Observed:  piinstall.ComponentState{Component: piinstall.ComponentOGSPayload, Compatibility: piinstall.CompatAbsent, Path: ".pi/skills"},
			},
			{
				Component: piinstall.ComponentGodot,
				Action:    piinstall.ActionInstall,
				Reason:    "Godot 4.7.2 is missing; install the pinned candidate",
				Command:   "download+verify+extract Godot 4.7.2",
				Outputs:   []string{"/owned/godot/4.7.2"},
				Effects:   []string{"downloads the official Godot zip over https and verifies its pinned SHA-256 and size"},
				Observed:  piinstall.ComponentState{Component: piinstall.ComponentGodot, Compatibility: piinstall.CompatAbsent},
			},
		},
	}
}

func blockedWizardPlan() piinstall.Plan {
	return piinstall.Plan{
		Steps: []piinstall.PlanStep{
			{Component: piinstall.ComponentEngramCore, Action: piinstall.ActionBlocked, Reason: "fail closed: engram output could not be parsed as a version"},
		},
	}
}

func completedWizardReport() piinstall.Report {
	return piinstall.Report{
		Entries: []piinstall.Entry{
			{Component: piinstall.ComponentNode, Outcome: piinstall.OutcomeInstalled, Path: "/owned/node/24.21.0/bin/node", Version: "24.21.0"},
			{Component: piinstall.ComponentNpm, Outcome: piinstall.OutcomeInstalled, Path: "/owned/node/24.21.0/bin/npm", Version: "11.19.0"},
			{Component: piinstall.ComponentPi, Outcome: piinstall.OutcomeReused, Path: "/owned/pi/bin/pi", Version: "0.87.1"},
			{Component: piinstall.ComponentShell, Outcome: piinstall.OutcomeInstalled, Detail: "exact pinned package is already registered"},
			{Component: piinstall.ComponentEngramCore, Outcome: piinstall.OutcomeInstalled, Path: "/owned/engram/bin/engram", Version: "2.2.0"},
			{Component: piinstall.ComponentEngramCompanion, Outcome: piinstall.OutcomeInstalled, Detail: "exact pinned package is already registered"},
			{Component: piinstall.ComponentOGSPayload, Outcome: piinstall.OutcomeInstalled, Path: ".pi/skills"},
			{Component: piinstall.ComponentGodot, Outcome: piinstall.OutcomeInstalled, Path: "/owned/godot/4.7.2/Godot_v4.7.2-stable_linux.x86_64", Version: "4.7.2"},
		},
		Launch: piinstall.LaunchInfo{
			PiExecutable:  "/owned/pi/bin/pi",
			PiPrefix:      "/owned/pi/0.87.1",
			NodeBinDir:    "/owned/node/24.21.0/bin",
			NpmExecutable: "/owned/node/24.21.0/bin/npm",
			EngramBinary:  "/owned/engram/bin/engram",
			GodotBinary:   "/owned/godot/4.7.2/Godot_v4.7.2-stable_linux.x86_64",
			PathAdditions: []string{"/owned/engram/bin"},
		},
	}
}

func assertNoWizardWrites(t *testing.T, workspace string) {
	t.Helper()
	assertPathNotExist(t, filepath.Join(workspace, "out"))
	assertPathNotExist(t, filepath.Join(workspace, ".game-studio", "profiles"))
	assertPathNotExist(t, filepath.Join(workspace, ".game-studio", "workspace.config.json"))
	assertPathNotExist(t, filepath.Join(workspace, ".pi"))
	configRaw, err := os.ReadFile(filepath.Join(workspace, "openspec", "config.yaml"))
	if err != nil {
		t.Fatalf("read openspec config: %v", err)
	}
	if strings.Contains(string(configRaw), "# BEGIN GAME-STUDIO WIZARD (managed)") {
		t.Fatalf("openspec config must not carry a managed block after a stopped wizard: %s", configRaw)
	}
}

func TestRunWizard_PlanOnlyRendersPlanWithoutWrites(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
	fake := &fakeWizardInstaller{plan: fakeWizardPlan()}

	stdout := captureStdout(t, func() {
		if err := RunWizard(WizardInput{
			Args:        []string{"--non-interactive", "--plan-only", "--final-artifact", finalPath},
			Registry:    templates.NewRegistry(),
			Persistence: persistence.NewPlaceholder(),
			Installer:   fake,
		}); err != nil {
			t.Fatalf("RunWizard returned error: %v", err)
		}
	})

	if fake.prepareCalls != 1 || fake.executeCalls != 0 {
		t.Fatalf("prepare=%d execute=%d, want 1/0", fake.prepareCalls, fake.executeCalls)
	}
	for _, token := range []string{
		"[wizard] runtime prerequisite plan",
		fake.plan.Fingerprint(),
		"node [install]",
		"download+verify+extract node",
		"/owned/node/24.21.0",
		"downloads over https",
		"plan-only preview complete",
		"approve this exact plan: game-studio wizard",
	} {
		if !strings.Contains(stdout, token) {
			t.Fatalf("expected plan-only output to contain %q, got:\n%s", token, stdout)
		}
	}
	if !filepath.IsAbs(fake.lastConfig.WorkspaceDir) {
		t.Fatalf("expected an absolute WorkspaceDir, got %q", fake.lastConfig.WorkspaceDir)
	}
	if !fake.lastConfig.GodotRequired {
		t.Fatalf("create_from_scratch with godot-core must require Godot")
	}
	assertNoWizardWrites(t, workspace)
}

func TestRunWizard_NonInteractiveDefaultsRequireExplicitApproval(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
	fake := &fakeWizardInstaller{plan: fakeWizardPlan()}

	err := RunWizard(WizardInput{
		Args:        []string{"--non-interactive", "--final-artifact", finalPath},
		Registry:    templates.NewRegistry(),
		Persistence: persistence.NewPlaceholder(),
		Installer:   fake,
	})
	if err == nil || !strings.Contains(err.Error(), "requires --plan-only or --approve-plan") {
		t.Fatalf("expected explicit approval-required error, got %v", err)
	}
	if fake.executeCalls != 0 {
		t.Fatalf("execute calls=%d, want 0", fake.executeCalls)
	}
	assertNoWizardWrites(t, workspace)
}

func TestRunWizard_ApprovePlanMismatchWritesNothing(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
	fake := &fakeWizardInstaller{plan: fakeWizardPlan()}

	err := RunWizard(WizardInput{
		Args:        []string{"--non-interactive", "--approve-plan", "deadbeef", "--final-artifact", finalPath},
		Registry:    templates.NewRegistry(),
		Persistence: persistence.NewPlaceholder(),
		Installer:   fake,
	})
	if err == nil || !strings.Contains(err.Error(), "does not match the freshly prepared plan fingerprint") {
		t.Fatalf("expected approval mismatch error, got %v", err)
	}
	if fake.executeCalls != 0 {
		t.Fatalf("execute calls=%d, want 0", fake.executeCalls)
	}
	assertNoWizardWrites(t, workspace)
}

func newFakeEngram(t *testing.T, script string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "engram")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake Engram executable: %v", err)
	}
	return path
}

// completedWizardReportWithEngram returns a truthful complete report whose
// reported Engram binary is a real fake executable, so the configured
// write-through genuinely succeeds.
func completedWizardReportWithEngram(t *testing.T) piinstall.Report {
	t.Helper()
	report := completedWizardReport()
	report.Launch.EngramBinary = newFakeEngram(t, "#!/bin/sh\nexit 0\n")
	return report
}

func TestRunWizard_MatchingApprovePlanExecutesOnce(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
	fake := &fakeWizardInstaller{plan: fakeWizardPlan(), report: completedWizardReportWithEngram(t)}

	err := RunWizard(WizardInput{
		Args:        []string{"--non-interactive", "--approve-plan", fake.plan.Fingerprint(), "--final-artifact", finalPath},
		Registry:    templates.NewRegistry(),
		Persistence: persistence.NewPlaceholder(),
		Installer:   fake,
	})
	if err != nil {
		t.Fatalf("RunWizard returned error: %v", err)
	}
	if fake.executeCalls != 1 {
		t.Fatalf("execute calls=%d, want 1", fake.executeCalls)
	}
	if !fake.lastConsent.Approved || fake.lastConsent.PlanFingerprint != fake.plan.Fingerprint() {
		t.Fatalf("consent was not bound to the prepared fingerprint: %#v", fake.lastConsent)
	}
	if !fake.lastConfig.GodotRequired {
		t.Fatalf("expected GodotRequired for the active Godot workflow mode")
	}
	if _, err := os.Stat(finalPath); err != nil {
		t.Fatalf("expected final artifact after approved setup: %v", err)
	}
}

func TestRunWizard_InteractiveDeclineWritesNothing(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
	fake := &fakeWizardInstaller{plan: fakeWizardPlan()}

	input := strings.Join([]string{
		"create_from_scratch",
		"godot-core",
		"minimal",
		"Decline Profile",
		"godot-docs",
		"godot-core",
		"formatter",
		"",
		"no",
	}, "\n") + "\n"

	stdout := captureStdout(t, func() {
		err := withStdinText(t, input, func() error {
			return RunWizard(WizardInput{
				Args:        []string{"--final-artifact", finalPath},
				Registry:    templates.NewRegistry(),
				Persistence: persistence.NewPlaceholder(),
				Installer:   fake,
			})
		})
		if err != nil {
			t.Fatalf("RunWizard returned error: %v", err)
		}
	})

	if fake.executeCalls != 0 {
		t.Fatalf("execute calls=%d, want 0 after decline", fake.executeCalls)
	}
	if !strings.Contains(stdout, "not approved") {
		t.Fatalf("expected explicit decline output, got:\n%s", stdout)
	}
	assertNoWizardWrites(t, workspace)
}

func TestRunWizard_BlockedPlanStopsBeforeExecute(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
	plan := blockedWizardPlan()
	fake := &fakeWizardInstaller{plan: plan}

	err := RunWizard(WizardInput{
		Args:        []string{"--non-interactive", "--approve-plan", plan.Fingerprint(), "--final-artifact", finalPath},
		Registry:    templates.NewRegistry(),
		Persistence: persistence.NewPlaceholder(),
		Installer:   fake,
	})
	if !errors.Is(err, errWizardRuntimeBlocked) {
		t.Fatalf("expected blocked plan error, got %v", err)
	}
	if fake.executeCalls != 0 {
		t.Fatalf("execute calls=%d, want 0", fake.executeCalls)
	}
	assertNoWizardWrites(t, workspace)
}

func TestRunWizard_ReportFailedStopsGeneration(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
	fake := &fakeWizardInstaller{
		plan: fakeWizardPlan(),
		report: piinstall.Report{
			Failed: true,
			Entries: []piinstall.Entry{
				{Component: piinstall.ComponentNode, Outcome: piinstall.OutcomeFailed, Detail: "extract archive: corrupted"},
			},
		},
	}

	stdout := captureStdout(t, func() {
		err := RunWizard(WizardInput{
			Args:        []string{"--non-interactive", "--approve-plan", fake.plan.Fingerprint(), "--final-artifact", finalPath},
			Registry:    templates.NewRegistry(),
			Persistence: persistence.NewPlaceholder(),
			Installer:   fake,
		})
		if !errors.Is(err, errWizardRuntimeIncomplete) {
			t.Fatalf("expected incomplete-runtime error, got %v", err)
		}
	})

	if !strings.Contains(stdout, "failed: true") || !strings.Contains(stdout, "extract archive: corrupted") {
		t.Fatalf("expected truthful partial report, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "[wizard] staged setup flow completed") {
		t.Fatalf("must not print the success marker after a failed report: %s", stdout)
	}
	assertNoWizardWrites(t, workspace)
}

func TestRunWizard_ExecuteErrorStopsGeneration(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
	fake := &fakeWizardInstaller{plan: fakeWizardPlan(), executeErr: errors.New("plan drift")}

	err := RunWizard(WizardInput{
		Args:        []string{"--non-interactive", "--approve-plan", fake.plan.Fingerprint(), "--final-artifact", finalPath},
		Registry:    templates.NewRegistry(),
		Persistence: persistence.NewPlaceholder(),
		Installer:   fake,
	})
	if err == nil || !strings.Contains(err.Error(), "execute runtime prerequisite plan") {
		t.Fatalf("expected execute error, got %v", err)
	}
	assertNoWizardWrites(t, workspace)
}

func TestRunWizard_ContextCancellationDoesNotReportSuccess(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	fake := &fakeWizardInstaller{
		plan: fakeWizardPlan(),
		report: piinstall.Report{
			Failed:  true,
			Entries: []piinstall.Entry{{Component: piinstall.ComponentNode, Outcome: piinstall.OutcomeFailed, Detail: "context canceled"}},
		},
	}

	stdout := captureStdout(t, func() {
		err := RunWizard(WizardInput{
			Args:        []string{"--non-interactive", "--approve-plan", fake.plan.Fingerprint(), "--final-artifact", finalPath},
			Registry:    templates.NewRegistry(),
			Persistence: persistence.NewPlaceholder(),
			Installer:   fake,
			Context:     canceled,
		})
		if err == nil {
			t.Fatalf("canceled execution must not report success")
		}
	})

	if strings.Contains(stdout, "[wizard] staged setup flow completed") {
		t.Fatalf("must not print success marker after cancellation: %s", stdout)
	}
	assertNoWizardWrites(t, workspace)
}

func TestRunWizard_ProfileCollisionStopsBeforeExecute(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
	profilePath := filepath.Join(workspace, ".game-studio", "profiles", "game-studio.md")
	if err := os.MkdirAll(filepath.Dir(profilePath), 0o755); err != nil {
		t.Fatalf("create profile dir: %v", err)
	}
	if err := os.WriteFile(profilePath, []byte("# hand-authored\n"), 0o644); err != nil {
		t.Fatalf("seed differing profile: %v", err)
	}
	fake := &fakeWizardInstaller{plan: fakeWizardPlan()}

	err := RunWizard(WizardInput{
		Args:        []string{"--non-interactive", "--approve-plan", fake.plan.Fingerprint(), "--final-artifact", finalPath},
		Registry:    templates.NewRegistry(),
		Persistence: persistence.NewPlaceholder(),
		Installer:   fake,
	})
	if err == nil || !strings.Contains(err.Error(), "refusing to overwrite existing wizard profile") {
		t.Fatalf("expected profile collision error, got %v", err)
	}
	if fake.executeCalls != 0 {
		t.Fatalf("collision must be refused before Execute; execute calls=%d", fake.executeCalls)
	}
}

func TestRunWizard_ProfileOutsideAncestorStopsBeforeExecute(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	outsideRoot := t.TempDir()
	if err := os.Symlink(outsideRoot, filepath.Join(workspace, ".game-studio")); err != nil {
		t.Skipf("symlink not supported in this environment: %v", err)
	}
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
	fake := &fakeWizardInstaller{plan: fakeWizardPlan()}

	err := RunWizard(WizardInput{
		Args:        []string{"--non-interactive", "--approve-plan", fake.plan.Fingerprint(), "--final-artifact", finalPath},
		Registry:    templates.NewRegistry(),
		Persistence: persistence.NewPlaceholder(),
		Installer:   fake,
	})
	if err == nil || !strings.Contains(err.Error(), "outside workspace root") {
		t.Fatalf("expected workspace escape error, got %v", err)
	}
	if fake.executeCalls != 0 {
		t.Fatalf("escape must be refused before Execute; execute calls=%d", fake.executeCalls)
	}
}

func TestWizardGodotRequiredOnlyForActiveWorkflowModes(t *testing.T) {
	tests := []struct {
		engine string
		mode   string
		want   bool
	}{
		{engine: "godot-core", mode: "create_from_scratch", want: true},
		{engine: "godot-core", mode: "existing_game", want: true},
		{engine: "godot-core", mode: "godot_sdd_handoff", want: true},
		{engine: "godot-core", mode: "repair_change_workflow", want: true},
		{engine: "godot-core", mode: "design_narrative_only", want: false},
		{engine: "godot-core", mode: "visual_artifacts_only", want: false},
		{engine: "unity", mode: "create_from_scratch", want: false},
		{engine: "ue5", mode: "existing_game", want: false},
	}
	for _, tt := range tests {
		state := wizardState{EnginePack: tt.engine, UseMode: tt.mode}
		if got := wizardGodotRequired(state); got != tt.want {
			t.Fatalf("wizardGodotRequired(%s,%s)=%t want %t", tt.engine, tt.mode, got, tt.want)
		}
		cfg := wizardRuntimeConfig(WizardInput{}, "/physical/workspace", state)
		if cfg.WorkspaceDir != "/physical/workspace" {
			t.Fatalf("WorkspaceDir=%q, want the forced workspace", cfg.WorkspaceDir)
		}
		if cfg.GodotRequired != tt.want {
			t.Fatalf("config GodotRequired=%t want %t", cfg.GodotRequired, tt.want)
		}
	}
}

func TestWizardLaunchCommand_OrderQuotingAndEnvPreserved(t *testing.T) {
	previousPATH := os.Getenv("PATH")
	previousEngram := os.Getenv("ENGRAM_BIN")
	previousGodot := os.Getenv("GODOT_BIN")

	launch := piinstall.LaunchInfo{
		PiExecutable:  "/opt/pi/bin/pi",
		NodeBinDir:    "/opt/node/bin",
		NpmExecutable: "/opt/npm/bin/npm",
		EngramBinary:  "/opt/engram/bin/engram",
		GodotBinary:   "/opt/godot/Godot",
		PathAdditions: []string{"/opt/node/bin", "/opt/extra"},
	}
	command := wizardLaunchCommand(launch)
	want := `PATH='/opt/node/bin:/opt/pi/bin:/opt/npm/bin:/opt/extra':"$PATH" ENGRAM_BIN='/opt/engram/bin/engram' GODOT_BIN='/opt/godot/Godot' '/opt/pi/bin/pi'`
	if command != want {
		t.Fatalf("launch command mismatch:\n got %q\nwant %q", command, want)
	}

	if os.Getenv("PATH") != previousPATH || os.Getenv("ENGRAM_BIN") != previousEngram || os.Getenv("GODOT_BIN") != previousGodot {
		t.Fatal("wizardLaunchCommand must not mutate the process environment")
	}

	quoted := wizardLaunchCommand(piinstall.LaunchInfo{
		PiExecutable: "/weird/it's pi",
		NodeBinDir:   "/opt/my node/bin",
	})
	if !strings.Contains(quoted, `'/weird/it'\''s pi'`) {
		t.Fatalf("expected POSIX quoting for a single quote, got %q", quoted)
	}
	if !strings.Contains(quoted, `'/opt/my node/bin:/weird'`) {
		t.Fatalf("expected POSIX quoting for a path with a space, got %q", quoted)
	}
	if got := wizardLaunchCommand(piinstall.LaunchInfo{}); got != "" {
		t.Fatalf("expected empty command without a Pi executable, got %q", got)
	}
}

func TestRunWizard_LeafSymlinkProfileStopsBeforeExecute(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	profilesDir := filepath.Join(workspace, ".game-studio", "profiles")
	if err := os.MkdirAll(profilesDir, 0o755); err != nil {
		t.Fatalf("create profiles dir: %v", err)
	}
	target := filepath.Join(profilesDir, "game-studio-real.md")
	if err := os.WriteFile(target, []byte("# unrelated\n"), 0o644); err != nil {
		t.Fatalf("seed symlink target: %v", err)
	}
	if err := os.Symlink(filepath.Base(target), filepath.Join(profilesDir, "game-studio.md")); err != nil {
		t.Skipf("symlink not supported in this environment: %v", err)
	}
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
	fake := &fakeWizardInstaller{plan: fakeWizardPlan()}

	err := RunWizard(WizardInput{
		Args:        []string{"--non-interactive", "--approve-plan", fake.plan.Fingerprint(), "--final-artifact", finalPath},
		Registry:    templates.NewRegistry(),
		Persistence: persistence.NewPlaceholder(),
		Installer:   fake,
	})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected leaf-symlink refusal, got %v", err)
	}
	if fake.executeCalls != 0 {
		t.Fatalf("leaf symlink must be refused before Execute; execute calls=%d", fake.executeCalls)
	}
	raw, readErr := os.ReadFile(target)
	if readErr != nil || string(raw) != "# unrelated\n" {
		t.Fatalf("symlink target must be preserved, content=%q err=%v", raw, readErr)
	}
}

func TestRunWizard_RecordsRuntimeSetupInFinalArtifact(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
	fake := &fakeWizardInstaller{plan: fakeWizardPlan(), report: completedWizardReportWithEngram(t)}

	if err := RunWizard(WizardInput{
		Args:        []string{"--non-interactive", "--approve-plan", fake.plan.Fingerprint(), "--final-artifact", finalPath},
		Registry:    templates.NewRegistry(),
		Persistence: persistence.NewPlaceholder(),
		Installer:   fake,
	}); err != nil {
		t.Fatalf("RunWizard returned error: %v", err)
	}

	artifact := readWizardArtifactFixture(t, finalPath)
	if artifact.RuntimeSetup == nil {
		t.Fatal("expected runtime_setup in the final artifact after a completed setup")
	}
	if artifact.RuntimeSetup.PlanFingerprint != fake.plan.Fingerprint() || artifact.RuntimeSetup.PlanStatus != "approval_required" {
		t.Fatalf("runtime_setup plan binding mismatch: %#v", artifact.RuntimeSetup)
	}
	if !artifact.RuntimeSetup.Executed || artifact.RuntimeSetup.Failed || artifact.RuntimeSetup.ReportStatus != "completed" {
		t.Fatalf("runtime_setup report status mismatch: %#v", artifact.RuntimeSetup)
	}
	if len(artifact.RuntimeSetup.Entries) != 8 {
		t.Fatalf("expected 8 runtime entries, got %#v", artifact.RuntimeSetup.Entries)
	}
	if artifact.RuntimeSetup.Launch.NodeBinDir != "/owned/node/24.21.0/bin" || artifact.RuntimeSetup.Launch.EngramBinary != fake.report.Launch.EngramBinary {
		t.Fatalf("launch info not recorded: %#v", artifact.RuntimeSetup.Launch)
	}
	if artifact.MemoryOutcome.Status != wizardMemorySaved {
		t.Fatalf("expected the memory outcome to record a completed save, got %#v", artifact.MemoryOutcome)
	}
}

func TestRunWizard_MetadataOnlySkipsRuntimeSetup(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
	fake := &fakeWizardInstaller{plan: fakeWizardPlan()}

	stdout := captureStdout(t, func() {
		if err := RunWizard(WizardInput{
			Args:        []string{"--metadata-only", "--non-interactive", "--final-artifact", finalPath},
			Registry:    templates.NewRegistry(),
			Persistence: persistence.NewPlaceholder(),
			Installer:   fake,
		}); err != nil {
			t.Fatalf("RunWizard returned error: %v", err)
		}
	})

	if fake.prepareCalls != 0 || fake.executeCalls != 0 {
		t.Fatalf("metadata-only must not touch the installer: prepare=%d execute=%d", fake.prepareCalls, fake.executeCalls)
	}
	if !strings.Contains(stdout, "not a read-only run") {
		t.Fatalf("metadata-only must disclose the local artifacts and memory write-through: %s", stdout)
	}
	artifact := readWizardArtifactFixture(t, finalPath)
	if artifact.RuntimeSetup == nil || artifact.RuntimeSetup.PlanStatus != "skipped_metadata_only" || artifact.RuntimeSetup.Executed || artifact.RuntimeSetup.ReportStatus != "skipped" {
		t.Fatalf("metadata-only must persist an explicit skipped runtime_setup, got %#v", artifact.RuntimeSetup)
	}
	if len(artifact.Providers) != 0 {
		t.Fatalf("metadata-only must keep not-inspected providers, got %#v", artifact.Providers)
	}
	if _, err := os.Stat(filepath.Join(workspace, ".pi")); !os.IsNotExist(err) {
		t.Fatalf("metadata-only must not install the runtime payload, stat err=%v", err)
	}
	if artifact.EngineWorkflowReady || artifact.GenerationReadiness.EngineWorkflowReady {
		t.Fatalf("metadata-only must not claim engine readiness without executed runtime evidence: %#v", artifact.GenerationReadiness)
	}
	if strings.Contains(stdout, "wizard runtime flow completed successfully") || strings.Contains(stdout, "engine_workflow_ready: true") {
		t.Fatalf("metadata-only must not claim a completed runtime flow: %s", stdout)
	}
}

// TestRunWizard_MetadataOnlyNoBlockerDoesNotClaimEngineReadiness guards F2: a
// metadata-only run with a passing no-blocker preflight must still report
// engine_workflow_ready false (top-level and nested) because it executed no
// runtime prerequisites. It must not touch Prepare/Execute and must persist an
// explicit skipped runtime_setup.
func TestRunWizard_MetadataOnlyNoBlockerDoesNotClaimEngineReadiness(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
	fake := &fakeWizardInstaller{plan: fakeWizardPlan()}
	env := &fakeEnvChecker{results: []toolcheck.Result{{ToolID: toolcheck.GodotToolID, ToolName: "Godot", Status: toolcheck.StatusSuccess, Reason: "detected Godot 4"}}}

	stdout := captureStdout(t, func() {
		if err := RunWizard(WizardInput{
			Args:        []string{"--metadata-only", "--non-interactive", "--setup-depth", "minimal", "--final-artifact", finalPath},
			Registry:    templates.NewRegistry(),
			Persistence: persistence.NewPlaceholder().WithEngramEnabled(false),
			Installer:   fake,
			EnvChecker:  env,
		}); err != nil {
			t.Fatalf("RunWizard returned error: %v", err)
		}
	})

	if fake.prepareCalls != 0 || fake.executeCalls != 0 {
		t.Fatalf("metadata-only must not touch the installer: prepare=%d execute=%d", fake.prepareCalls, fake.executeCalls)
	}
	if strings.Contains(stdout, "wizard runtime flow completed successfully") || strings.Contains(stdout, "engine_workflow_ready: true") {
		t.Fatalf("metadata-only must not claim completed runtime or engine readiness: %s", stdout)
	}

	artifact := readWizardArtifactFixture(t, finalPath)
	if artifact.PreflightResult.Status != "pass" {
		t.Fatalf("expected a passing no-blocker preflight, got %#v", artifact.PreflightResult)
	}
	if artifact.RuntimeSetup == nil || artifact.RuntimeSetup.PlanStatus != "skipped_metadata_only" || artifact.RuntimeSetup.Executed {
		t.Fatalf("metadata-only must persist a skipped, unexecuted runtime_setup, got %#v", artifact.RuntimeSetup)
	}
	if artifact.EngineWorkflowReady || artifact.GenerationReadiness.EngineWorkflowReady {
		t.Fatalf("passing preflight without executed runtime evidence must not be engine ready: %#v", artifact.GenerationReadiness)
	}
}

func TestRunWizard_ConflictFlagCombinationsAreRejected(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
	fake := &fakeWizardInstaller{plan: fakeWizardPlan()}

	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "metadata-only plus plan-only", args: []string{"--metadata-only", "--plan-only"}, want: "cannot be combined"},
		{name: "metadata-only plus approve-plan", args: []string{"--metadata-only", "--non-interactive", "--approve-plan", "x"}, want: "cannot be combined"},
		{name: "plan-only plus approve-plan", args: []string{"--plan-only", "--non-interactive", "--approve-plan", "x"}, want: "cannot be combined"},
		{name: "approve-plan without non-interactive", args: []string{"--approve-plan", "x"}, want: "requires --non-interactive"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append(append([]string{}, tt.args...), "--final-artifact", finalPath)
			err := RunWizard(WizardInput{
				Args:        args,
				Registry:    templates.NewRegistry(),
				Persistence: persistence.NewPlaceholder(),
				Installer:   fake,
			})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("want error containing %q, got %v", tt.want, err)
			}
			if fake.prepareCalls != 0 || fake.executeCalls != 0 {
				t.Fatalf("conflicting flags must not reach the installer: prepare=%d execute=%d", fake.prepareCalls, fake.executeCalls)
			}
		})
	}
	assertNoWizardWrites(t, workspace)
}

func TestRunWizard_RoutesEngramWriteThroughToReportedBinary(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")

	spyDir := t.TempDir()
	spyLog := filepath.Join(spyDir, "engram-spy.log")
	spyPath := filepath.Join(spyDir, "engram")
	spyScript := "#!/bin/sh\n" +
		"printf 'title=%s\\n' \"$2\" >> \"$ENGRAM_SPY_LOG\"\n" +
		"exit 0\n"
	if err := os.WriteFile(spyPath, []byte(spyScript), 0o755); err != nil {
		t.Fatalf("write fake Engram executable: %v", err)
	}
	t.Setenv("ENGRAM_SPY_LOG", spyLog)

	report := completedWizardReport()
	report.Launch.EngramBinary = spyPath
	fake := &fakeWizardInstaller{plan: fakeWizardPlan(), report: report}

	stdout := captureStdout(t, func() {
		if err := RunWizard(WizardInput{
			Args:        []string{"--non-interactive", "--approve-plan", fake.plan.Fingerprint(), "--final-artifact", finalPath},
			Registry:    templates.NewRegistry(),
			Persistence: persistence.NewPlaceholder(),
			Installer:   fake,
		}); err != nil {
			t.Fatalf("RunWizard returned error: %v", err)
		}
	})

	raw, err := os.ReadFile(spyLog)
	if err != nil {
		t.Fatalf("expected the reported Engram binary to receive the write-through: %v", err)
	}
	if !strings.Contains(string(raw), "title=installer-wizard/final-artifact/godot") {
		t.Fatalf("unexpected Engram write-through payload: %s", raw)
	}
	if strings.Contains(stdout, "warning: engram write-through skipped") {
		t.Fatalf("write-through must not be skipped: %s", stdout)
	}
	artifact := readWizardArtifactFixture(t, finalPath)
	if artifact.MemoryOutcome.Status != wizardMemorySaved {
		t.Fatalf("expected the memory outcome to record a completed save, got %#v", artifact.MemoryOutcome)
	}
}

// unexpectedDownloader fails the test if the default backend ever downloads.
type unexpectedDownloader struct {
	mu    sync.Mutex
	calls int
}

func (d *unexpectedDownloader) Download(context.Context, string, string, int64) error {
	d.mu.Lock()
	d.calls++
	d.mu.Unlock()
	return errors.New("unexpected download in the integration fixture")
}

// fakePiCommandRunner implements piinstall.CommandRunner entirely in-process so
// the real default adapter can run without a network, a host probe, or a real
// package install.
type fakePiCommandRunner struct {
	mu        sync.Mutex
	fakeEnram string
	installed map[string]string
}

func newFakePiCommandRunner(engramPath string) *fakePiCommandRunner {
	return &fakePiCommandRunner{fakeEnram: engramPath, installed: map[string]string{}}
}

func (r *fakePiCommandRunner) LookPath(name string) (string, error) {
	switch name {
	case "node", "npm", "pi":
		return filepath.Join("/fake/bin", name), nil
	case "engram":
		return r.fakeEnram, nil
	default:
		return "", errors.New("unexpected lookup: " + name)
	}
}

func (r *fakePiCommandRunner) Run(_ context.Context, spec piinstall.CommandSpec) piinstall.CommandResult {
	switch {
	case strings.HasSuffix(spec.Name, "/node"):
		return piinstall.CommandResult{Stdout: "v24.21.0\n"}
	case strings.HasSuffix(spec.Name, "/npm"):
		return piinstall.CommandResult{Stdout: "11.19.0\n"}
	case spec.Name == r.fakeEnram:
		return piinstall.CommandResult{Stdout: "engram 2.2.0\n"}
	case strings.HasSuffix(spec.Name, "/pi"):
		switch {
		case len(spec.Args) > 0 && spec.Args[0] == "list":
			return piinstall.CommandResult{Stdout: r.listing()}
		case len(spec.Args) > 1 && spec.Args[0] == "install":
			name, version := parseFakeNpmSpec(spec.Args[1])
			r.mu.Lock()
			r.installed[name] = version
			r.mu.Unlock()
			return piinstall.CommandResult{}
		default:
			return piinstall.CommandResult{Stdout: "0.87.1\n"}
		}
	}
	return piinstall.CommandResult{ExitCode: 1, Err: errors.New("unexpected command: " + spec.Name)}
}

func (r *fakePiCommandRunner) listing() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.installed) == 0 {
		return "No packages installed.\n"
	}
	names := make([]string, 0, len(r.installed))
	for name := range r.installed {
		names = append(names, name)
	}
	sort.Strings(names)
	lines := []string{"User packages:"}
	for _, name := range names {
		lines = append(lines, "  npm:"+name+"@"+r.installed[name], "    /fake/path")
	}
	return strings.Join(lines, "\n") + "\n"
}

func parseFakeNpmSpec(source string) (string, string) {
	spec := strings.TrimPrefix(source, "npm:")
	at := strings.LastIndexByte(spec, '@')
	if at < 0 {
		return spec, ""
	}
	return spec[:at], spec[at+1:]
}

func TestDefaultWizardInstallerAdapter_Integration(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")

	installRoot := t.TempDir()
	engramDir := t.TempDir()
	fakeEngram := filepath.Join(engramDir, "engram")
	if err := os.WriteFile(fakeEngram, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake Engram executable: %v", err)
	}

	runner := newFakePiCommandRunner(fakeEngram)
	downloader := &unexpectedDownloader{}
	installer := DefaultWizardInstaller()
	baseConfig := &piinstall.Config{InstallRoot: installRoot, Commands: runner, Downloader: downloader}

	stdout := captureStdout(t, func() {
		if err := RunWizard(WizardInput{
			Args:        []string{"--non-interactive", "--plan-only", "--use-mode", "design_narrative_only", "--final-artifact", finalPath},
			Registry:    templates.NewRegistry(),
			Persistence: persistence.NewPlaceholder(),
			Installer:   installer,
			Config:      baseConfig,
		}); err != nil {
			t.Fatalf("plan-only RunWizard returned error: %v", err)
		}
	})
	fingerprint := parsePrintedPlanFingerprint(t, stdout)
	if downloader.calls != 0 {
		t.Fatalf("plan-only must not download; calls=%d", downloader.calls)
	}

	runStdout := captureStdout(t, func() {
		if err := RunWizard(WizardInput{
			Args:        []string{"--non-interactive", "--approve-plan", fingerprint, "--use-mode", "design_narrative_only", "--final-artifact", finalPath},
			Registry:    templates.NewRegistry(),
			Persistence: persistence.NewPlaceholder(),
			Installer:   installer,
			Config:      baseConfig,
		}); err != nil {
			t.Fatalf("approved RunWizard returned error: %v", err)
		}
	})
	if downloader.calls != 0 {
		t.Fatalf("the reused-component fixture must never download; calls=%d", downloader.calls)
	}
	if !strings.Contains(runStdout, "godot_required: false") {
		t.Fatalf("design_narrative_only must not require Godot:\n%s", runStdout)
	}
	if _, err := os.Stat(filepath.Join(workspace, ".pi", "skills")); err != nil {
		t.Fatalf("expected the canonical payload copied into the workspace: %v", err)
	}
	artifact := readWizardArtifactFixture(t, finalPath)
	if artifact.RuntimeSetup == nil || artifact.RuntimeSetup.ReportStatus != "completed" {
		t.Fatalf("expected a completed runtime setup, got %#v", artifact.RuntimeSetup)
	}
	var piReused, engramReused bool
	for _, entry := range artifact.RuntimeSetup.Entries {
		if entry.Component == string(piinstall.ComponentPi) && entry.Outcome == string(piinstall.OutcomeReused) {
			piReused = true
		}
		if entry.Component == string(piinstall.ComponentEngramCore) && entry.Outcome == string(piinstall.OutcomeReused) {
			engramReused = true
		}
	}
	if !piReused || !engramReused {
		t.Fatalf("expected backend-verified Pi and Engram reuse, got %#v", artifact.RuntimeSetup.Entries)
	}
}

func parsePrintedPlanFingerprint(t *testing.T, stdout string) string {
	t.Helper()
	const marker = "  fingerprint: "
	for _, line := range strings.Split(stdout, "\n") {
		if strings.HasPrefix(line, marker) {
			return strings.TrimSpace(strings.TrimPrefix(line, marker))
		}
	}
	t.Fatalf("no plan fingerprint found in output:\n%s", stdout)
	return ""
}

func godotAmbientBlocker() *fakeEnvChecker {
	return &fakeEnvChecker{results: []toolcheck.Result{{
		ToolID:      toolcheck.GodotToolID,
		ToolName:    "Godot",
		Status:      toolcheck.StatusFailure,
		Severity:    toolcheck.SeverityRequired,
		Required:    true,
		Reason:      "godot lookup: executable file not found in $PATH",
		Attempted:   "godot --version",
		Remediation: "Install Godot 4 and ensure godot or godot4 is available on PATH.",
	}}}
}

func godotAndBlenderAmbientBlockers() *fakeEnvChecker {
	return &fakeEnvChecker{results: []toolcheck.Result{
		{ToolID: toolcheck.GodotToolID, ToolName: "Godot", Status: toolcheck.StatusFailure, Severity: toolcheck.SeverityRequired, Required: true, Reason: "godot lookup: executable file not found in $PATH", Attempted: "godot --version", Remediation: "Install Godot 4 and ensure godot or godot4 is available on PATH."},
		{ToolID: "blender", ToolName: "Blender", Status: toolcheck.StatusFailure, Severity: toolcheck.SeverityRequired, Required: true, Reason: "blender lookup: executable file not found in $PATH", Attempted: "blender --version", Remediation: "add Blender to PATH."},
	}}
}

func withoutGodotPlan() piinstall.Plan {
	plan := fakeWizardPlan()
	plan.GodotRequired = false
	steps := make([]piinstall.PlanStep, 0, len(plan.Steps))
	for _, step := range plan.Steps {
		if step.Component == piinstall.ComponentGodot {
			continue
		}
		steps = append(steps, step)
	}
	plan.Steps = steps
	return plan
}

func reportWithoutGodot() piinstall.Report {
	report := completedWizardReport()
	report.Launch.GodotBinary = ""
	entries := make([]piinstall.Entry, 0, len(report.Entries))
	for _, entry := range report.Entries {
		if entry.Component == piinstall.ComponentGodot {
			continue
		}
		entries = append(entries, entry)
	}
	report.Entries = entries
	return report
}

func TestRunWizard_AmbientGodotMissingManagedGodotSuccessReturnsZero(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
	fake := &fakeWizardInstaller{plan: fakeWizardPlan(), report: completedWizardReportWithEngram(t)}

	if err := RunWizard(WizardInput{
		Args:        []string{"--non-interactive", "--approve-plan", fake.plan.Fingerprint(), "--final-artifact", finalPath},
		Registry:    templates.NewRegistry(),
		Persistence: persistence.NewPlaceholder(),
		EnvChecker:  godotAmbientBlocker(),
		Installer:   fake,
	}); err != nil {
		t.Fatalf("verified managed Godot must reconcile the ambient preflight, got %v", err)
	}

	artifact := readWizardArtifactFixture(t, finalPath)
	if len(artifact.EnvBlockers) != 0 {
		t.Fatalf("expected the reconciled Godot blocker to be cleared, got %#v", artifact.EnvBlockers)
	}
	if !artifact.EngineWorkflowReady || artifact.PreflightResult.Status != "pass" {
		t.Fatalf("expected reconciled pass readiness, preflight=%#v readiness=%#v", artifact.PreflightResult, artifact.GenerationReadiness)
	}
	if !containsString(artifact.ValidatedTools, toolcheck.GodotToolID) {
		t.Fatalf("expected godot recorded as validated, got %#v", artifact.ValidatedTools)
	}
	nextSteps := strings.Join(artifact.NextSteps, "\n")
	if !strings.Contains(nextSteps, fake.report.Launch.GodotBinary) {
		t.Fatalf("expected the reported managed Godot path in next steps, got %#v", artifact.NextSteps)
	}
	if strings.Contains(nextSteps, "Install Godot 4") {
		t.Fatalf("stale Godot install-on-PATH advice must be removed: %#v", artifact.NextSteps)
	}
}

func TestRunWizard_UnrelatedBlockerRemainsAfterGodotReconcile(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
	fake := &fakeWizardInstaller{plan: fakeWizardPlan(), report: completedWizardReportWithEngram(t)}

	err := RunWizard(WizardInput{
		Args:        []string{"--non-interactive", "--approve-plan", fake.plan.Fingerprint(), "--final-artifact", finalPath},
		Registry:    templates.NewRegistry(),
		Persistence: persistence.NewPlaceholder(),
		EnvChecker:  godotAndBlenderAmbientBlockers(),
		Installer:   fake,
	})
	if err == nil || !strings.Contains(err.Error(), "blender") {
		t.Fatalf("expected the unrelated blender blocker to remain, got %v", err)
	}
	if strings.Contains(strings.ToLower(err.Error()), "godot") {
		t.Fatalf("reconciled Godot must not remain a blocker: %v", err)
	}
	artifact := readWizardArtifactFixture(t, finalPath)
	if len(artifact.EnvBlockers) != 1 || !strings.Contains(artifact.EnvBlockers[0], "blender") {
		t.Fatalf("expected only the blender blocker to remain, got %#v", artifact.EnvBlockers)
	}
	if !containsString(artifact.ValidatedTools, toolcheck.GodotToolID) {
		t.Fatalf("expected godot recorded as validated, got %#v", artifact.ValidatedTools)
	}
}

func TestRunWizard_UnverifiedGodotEvidenceDoesNotClear(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
	report := completedWizardReport()
	for i := range report.Entries {
		if report.Entries[i].Component == piinstall.ComponentGodot {
			report.Entries[i].Outcome = piinstall.OutcomeUnverified
		}
	}
	report.Launch.GodotBinary = ""
	fake := &fakeWizardInstaller{plan: fakeWizardPlan(), report: report}

	err := RunWizard(WizardInput{
		Args:        []string{"--non-interactive", "--approve-plan", fake.plan.Fingerprint(), "--final-artifact", finalPath},
		Registry:    templates.NewRegistry(),
		Persistence: persistence.NewPlaceholder(),
		EnvChecker:  godotAmbientBlocker(),
		Installer:   fake,
	})
	if !errors.Is(err, errWizardRuntimeIncomplete) {
		t.Fatalf("unverified Godot evidence must be incomplete, got %v", err)
	}
	assertNoWizardWrites(t, workspace)
}

func TestRunWizard_UnverifiedPayloadReportDoesNotCompleteSetup(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
	report := completedWizardReport()
	for i := range report.Entries {
		if report.Entries[i].Component == piinstall.ComponentOGSPayload {
			report.Entries[i].Outcome = piinstall.OutcomeUnverified
			report.Entries[i].Detail = "1 modified payload file was preserved unchanged and the managed payload is unverified"
		}
	}
	fake := &fakeWizardInstaller{plan: fakeWizardPlan(), report: report}

	stdout := captureStdout(t, func() {
		err := RunWizard(WizardInput{
			Args:        []string{"--non-interactive", "--approve-plan", fake.plan.Fingerprint(), "--final-artifact", finalPath},
			Registry:    templates.NewRegistry(),
			Persistence: persistence.NewPlaceholder(),
			Installer:   fake,
		})
		if !errors.Is(err, errWizardRuntimeIncomplete) {
			t.Fatalf("an unverified payload report must not complete setup, got %v", err)
		}
	})
	if strings.Contains(stdout, "[wizard] staged setup flow completed") {
		t.Fatalf("must not print the success marker after an unverified payload report: %s", stdout)
	}
	if !strings.Contains(stdout, string(piinstall.OutcomeUnverified)) {
		t.Fatalf("expected the unverified payload outcome in the report, got:\n%s", stdout)
	}
	assertNoWizardWrites(t, workspace)
}

func TestRunWizard_DesignModeDoesNotFakeGodotReadiness(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
	plan := withoutGodotPlan()
	report := reportWithoutGodot()
	report.Launch.EngramBinary = newFakeEngram(t, "#!/bin/sh\nexit 0\n")
	fake := &fakeWizardInstaller{plan: plan, report: report}

	err := RunWizard(WizardInput{
		Args:        []string{"--non-interactive", "--approve-plan", plan.Fingerprint(), "--use-mode", "design_narrative_only", "--final-artifact", finalPath},
		Registry:    templates.NewRegistry(),
		Persistence: persistence.NewPlaceholder(),
		EnvChecker:  godotAmbientBlocker(),
		Installer:   fake,
	})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "godot") {
		t.Fatalf("design mode must not clear the ambient Godot blocker without managed evidence, got %v", err)
	}
	artifact := readWizardArtifactFixture(t, finalPath)
	if artifact.EngineWorkflowReady {
		t.Fatalf("design mode must not claim Godot readiness: %#v", artifact.GenerationReadiness)
	}
}

func TestRunWizard_FakeExecuteCancelsContextDoesNotWrite(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fake := &fakeWizardInstaller{plan: fakeWizardPlan(), report: completedWizardReport(), cancelOnExecute: cancel}

	stdout := captureStdout(t, func() {
		err := RunWizard(WizardInput{
			Args:        []string{"--non-interactive", "--approve-plan", fake.plan.Fingerprint(), "--final-artifact", finalPath},
			Registry:    templates.NewRegistry(),
			Persistence: persistence.NewPlaceholder(),
			Installer:   fake,
			Context:     ctx,
		})
		if err == nil {
			t.Fatal("a canceled Execute must not report success")
		}
	})
	if strings.Contains(stdout, "launch:") || strings.Contains(stdout, "[wizard] staged setup flow completed") {
		t.Fatalf("canceled run must not print launch or success: %s", stdout)
	}
	assertNoWizardWrites(t, workspace)
}

func TestWizardReplayCommand_PreservesNonDefaultFlagsAndQuotes(t *testing.T) {
	original := []string{
		"--non-interactive", "--plan-only",
		"--engine-pack", "unity",
		"--use-mode", "godot_sdd_handoff",
		"--profile", "O'Brien Studio",
		"--final-artifact", "/tmp/out with space/final.json",
	}
	approve := wizardReplayCommand(original, "--approve-plan", "abc123")
	for _, token := range []string{
		"--non-interactive",
		"--engine-pack unity",
		"--use-mode godot_sdd_handoff",
		`--profile 'O'\''Brien Studio'`,
		`--final-artifact '/tmp/out with space/final.json'`,
		"--approve-plan abc123",
	} {
		if !strings.Contains(approve, token) {
			t.Fatalf("expected replay command to contain %q, got %q", token, approve)
		}
	}
	if strings.Contains(approve, "--plan-only") || strings.Contains(approve, "--metadata-only") {
		t.Fatalf("approval command must not keep preview-only flags: %q", approve)
	}
	if strings.Contains(approve, "godot-core") || strings.Contains(approve, "create_from_scratch") {
		t.Fatalf("replay command must not revert caller defaults: %q", approve)
	}
	preview := wizardReplayCommand(original, "--plan-only")
	if !strings.Contains(preview, "--plan-only") || strings.Contains(preview, "--approve-plan") {
		t.Fatalf("preview command mismatch: %q", preview)
	}
}

func TestWizardReplayArgs_PreservesFlagShapedStringValues(t *testing.T) {
	for _, name := range []string{"profile", "complexity", "start", "use-mode", "engine-pack", "setup-depth", "out-dir", "final-artifact"} {
		for _, prefix := range []string{"-", "--"} {
			for _, value := range []string{"--plan-only", "--metadata-only", "--approve-plan", "--", "-plan-only"} {
				t.Run(prefix+name+"/"+value, func(t *testing.T) {
					option := prefix + name
					original := []string{"--non-interactive", "--plan-only", option, value, "--"}
					want := []string{"--non-interactive", option, value, "--approve-plan", "digest"}
					if got := wizardReplayArgs(original, "--approve-plan", "digest"); !slices.Equal(got, want) {
						t.Fatalf("replay args = %q, want %q", got, want)
					}
				})
			}
		}
	}
	original := []string{"--profile=--plan-only", "-approve-plan=old", "-plan-only=false", "--non-interactive"}
	want := []string{"--profile=--plan-only", "--non-interactive", "--approve-plan", "new"}
	if got := wizardReplayArgs(original, "--approve-plan", "new"); !slices.Equal(got, want) {
		t.Fatalf("inline replay = %q, want %q", got, want)
	}
}

func TestRunWizard_RejectsPositionalsBeforePrepare(t *testing.T) {
	fake := &fakeWizardInstaller{plan: fakeWizardPlan()}
	err := RunWizard(WizardInput{Args: []string{"--non-interactive", "--plan-only", "unexpected"}, Installer: fake})
	if err == nil || !strings.Contains(err.Error(), "unexpected positional arguments") {
		t.Fatalf("want positional argument error, got %v", err)
	}
	if fake.prepareCalls != 0 || fake.executeCalls != 0 {
		t.Fatal("invalid arguments must not reach installer")
	}
}

func TestPrintWizardPlanApprovalInstructions_InteractiveHonestGuidance(t *testing.T) {
	plan := fakeWizardPlan()
	stdout := captureStdout(t, func() {
		printWizardPlanApprovalInstructions(plan, []string{"--plan-only"}, false, "/workspace")
	})
	if !strings.Contains(stdout, plan.Fingerprint()) {
		t.Fatalf("expected the fingerprint, got %s", stdout)
	}
	if !strings.Contains(stdout, "not representable as CLI flags") {
		t.Fatalf("expected honest interactive guidance, got %s", stdout)
	}
	if strings.Contains(stdout, "approve this exact plan:") || strings.Contains(stdout, "reproducible preview:") {
		t.Fatalf("interactive runs must not claim a reprinted command reproduces the plan: %s", stdout)
	}
}

func TestRunWizard_ManagedDestinationAliasesRejected(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		config  func(workspace string) *piinstall.Config
		setup   func(t *testing.T, workspace string)
		wantErr string
	}{
		{
			name:    "reserved workspace config",
			args:    []string{"--final-artifact", ".game-studio/workspace.config.json"},
			wantErr: "resolve to the same path",
		},
		{
			name:    "reserved openspec config",
			args:    []string{"--final-artifact", "openspec/config.yaml"},
			wantErr: "resolve to the same path",
		},
		{
			name:    "wizard profile alias",
			args:    []string{"--final-artifact", ".game-studio/profiles/game-studio.md"},
			wantErr: "resolve to the same path",
		},
		{
			name:    "generated pack file alias",
			args:    []string{"--out-dir", "out/generated", "--final-artifact", "out/generated/studio-profile.godot.md"},
			wantErr: "resolve to the same path",
		},
		{
			name:    "out dir and final artifact alias",
			args:    []string{"--out-dir", "out/generated", "--final-artifact", "out/generated"},
			wantErr: "resolve to the same path",
		},
		{
			name: "physical alias through internal symlink ancestor",
			setup: func(t *testing.T, workspace string) {
				t.Helper()
				internal := filepath.Join(workspace, "internal-game-studio")
				if err := os.MkdirAll(internal, 0o755); err != nil {
					t.Fatalf("create internal dir: %v", err)
				}
				if err := os.Symlink(internal, filepath.Join(workspace, ".game-studio")); err != nil {
					t.Skipf("symlink not supported in this environment: %v", err)
				}
			},
			args:    []string{"--final-artifact", ".game-studio/workspace.config.json"},
			wantErr: "resolve to the same path",
		},
		{
			name:    "protected pi namespace",
			args:    []string{"--final-artifact", ".pi/skills/evil.md"},
			wantErr: "protected control namespace",
		},
		{
			name:    "protected git namespace",
			args:    []string{"--final-artifact", ".git/config"},
			wantErr: "protected control namespace",
		},
		{
			name: "managed install root inside workspace",
			config: func(workspace string) *piinstall.Config {
				return &piinstall.Config{InstallRoot: filepath.Join(workspace, "managed-root")}
			},
			args:    []string{"--final-artifact", "managed-root/final.json"},
			wantErr: "overlaps the managed install root",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			workspace := wizardWorkspaceFixture(t)
			setWorkingDirectory(t, workspace)
			if tt.setup != nil {
				tt.setup(t, workspace)
			}
			var config *piinstall.Config
			if tt.config != nil {
				config = tt.config(workspace)
			}
			fake := &fakeWizardInstaller{plan: fakeWizardPlan(), report: completedWizardReport()}
			args := append([]string{"--non-interactive", "--approve-plan", fake.plan.Fingerprint()}, tt.args...)

			err := RunWizard(WizardInput{
				Args:        args,
				Registry:    templates.NewRegistry(),
				Persistence: persistence.NewPlaceholder(),
				Installer:   fake,
				Config:      config,
			})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("want error containing %q, got %v", tt.wantErr, err)
			}
			if fake.executeCalls != 0 {
				t.Fatalf("managed-destination refusal must stop before Execute; execute calls=%d", fake.executeCalls)
			}
			assertNoWizardWrites(t, workspace)
		})
	}
}

func TestRunWizard_RejectsLexicalManagedFileSymlinks(t *testing.T) {
	pack, err := resolveOutputPaths(templates.EngineGodot, "", "out/generated", "flat")
	if err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{"out/final.json", pack.profile, pack.summary, pack.pattern, pack.config, workspaceConfigRelativePath, "openspec/config.yaml"} {
		t.Run(relative, func(t *testing.T) {
			workspace := wizardWorkspaceFixture(t)
			setWorkingDirectory(t, workspace)
			target := filepath.Join(workspace, "important-unmanaged.txt")
			const sentinel = "existing user data must survive"
			if err := os.WriteFile(target, []byte(sentinel), 0o600); err != nil {
				t.Fatal(err)
			}
			link := filepath.Join(workspace, relative)
			if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, link); err != nil {
				t.Skipf("symlink unavailable: %v", err)
			}
			fake := &fakeWizardInstaller{plan: fakeWizardPlan(), report: completedWizardReport()}
			err := RunWizard(WizardInput{
				Args:     []string{"--non-interactive", "--approve-plan", fake.plan.Fingerprint(), "--out-dir", "out/generated", "--final-artifact", "out/final.json"},
				Registry: templates.NewRegistry(), Persistence: persistence.NewPlaceholder(), Installer: fake,
			})
			if err == nil || !strings.Contains(err.Error(), "symlink") {
				t.Fatalf("want leaf-symlink refusal, got %v", err)
			}
			if fake.executeCalls != 0 {
				t.Fatal("leaf refusal must precede Execute")
			}
			data, err := os.ReadFile(target)
			if err != nil || string(data) != sentinel {
				t.Fatalf("target changed: %q, %v", data, err)
			}
			info, err := os.Lstat(link)
			if err != nil || info.Mode()&os.ModeSymlink == 0 {
				t.Fatalf("original link was not preserved: %v", err)
			}
			assertPathNotExist(t, filepath.Join(workspace, wizardProfileArtifactRel("Game-Studio")))
			if relative != workspaceConfigRelativePath {
				assertPathNotExist(t, filepath.Join(workspace, workspaceConfigRelativePath))
			}
		})
	}
}

func TestRunWizard_RejectsPhysicalControlNamespaceAliases(t *testing.T) {
	for _, control := range []string{".git", ".pi"} {
		t.Run(control, func(t *testing.T) {
			workspace := wizardWorkspaceFixture(t)
			setWorkingDirectory(t, workspace)
			storage := filepath.Join(workspace, "control-storage")
			if err := os.MkdirAll(storage, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(storage, filepath.Join(workspace, control)); err != nil {
				t.Skipf("symlink unavailable: %v", err)
			}
			fake := &fakeWizardInstaller{plan: fakeWizardPlan()}
			err := RunWizard(WizardInput{
				Args:     []string{"--non-interactive", "--approve-plan", fake.plan.Fingerprint(), "--final-artifact", "control-storage/final.json"},
				Registry: templates.NewRegistry(), Persistence: persistence.NewPlaceholder(), Installer: fake,
			})
			if err == nil || !strings.Contains(err.Error(), "protected control namespace") {
				t.Fatalf("want protected namespace refusal, got %v", err)
			}
			if fake.executeCalls != 0 {
				t.Fatal("namespace refusal must precede Execute")
			}
			assertPathNotExist(t, filepath.Join(storage, "final.json"))
			assertPathNotExist(t, filepath.Join(workspace, workspaceConfigRelativePath))
		})
	}
}

func TestRunWizard_ManagedDestinationPositives(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "default destinations", args: []string{"--final-artifact", ".game-studio/generated/wizard/final.artifact.json"}},
		{name: "custom safe outdir", args: []string{"--out-dir", "out/custom-generated", "--final-artifact", "out/wizard/custom-final.json"}},
		{name: "final artifact inside outdir distinct name", args: []string{"--out-dir", "out/generated", "--final-artifact", "out/generated/final.artifact.json"}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			workspace := wizardWorkspaceFixture(t)
			setWorkingDirectory(t, workspace)
			fake := &fakeWizardInstaller{plan: fakeWizardPlan(), report: completedWizardReportWithEngram(t)}
			args := append([]string{"--non-interactive", "--approve-plan", fake.plan.Fingerprint()}, tt.args...)
			if err := RunWizard(WizardInput{
				Args:        args,
				Registry:    templates.NewRegistry(),
				Persistence: persistence.NewPlaceholder(),
				Installer:   fake,
			}); err != nil {
				t.Fatalf("expected managed destinations to be accepted, got %v", err)
			}
		})
	}
}

func TestRunWizard_MissingReportedEngramBinaryFails(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
	report := completedWizardReport() // EngramBinary points at a path that does not exist
	fake := &fakeWizardInstaller{plan: fakeWizardPlan(), report: report}

	stdout := captureStdout(t, func() {
		err := RunWizard(WizardInput{
			Args:        []string{"--non-interactive", "--approve-plan", fake.plan.Fingerprint(), "--final-artifact", finalPath},
			Registry:    templates.NewRegistry(),
			Persistence: persistence.NewPlaceholder(),
			Installer:   fake,
		})
		if !errors.Is(err, errWizardMemoryFailed) {
			t.Fatalf("missing reported Engram binary must fail the memory write-through, got %v", err)
		}
	})
	if strings.Contains(stdout, "[wizard] staged setup flow completed") {
		t.Fatalf("a failed memory write must not complete the staged flow: %s", stdout)
	}
	if _, err := os.Stat(finalPath); err != nil {
		t.Fatalf("local artifacts must remain after a memory failure: %v", err)
	}
	artifact := readWizardArtifactFixture(t, finalPath)
	if artifact.MemoryOutcome.Status != wizardMemoryFailed {
		t.Fatalf("expected an honest failed memory outcome, got %#v", artifact.MemoryOutcome)
	}
}

func TestRunWizard_FakeEngramNonZeroFails(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
	report := completedWizardReport()
	report.Launch.EngramBinary = newFakeEngram(t, "#!/bin/sh\nexit 3\n")
	fake := &fakeWizardInstaller{plan: fakeWizardPlan(), report: report}

	stdout := captureStdout(t, func() {
		err := RunWizard(WizardInput{
			Args:        []string{"--non-interactive", "--approve-plan", fake.plan.Fingerprint(), "--final-artifact", finalPath},
			Registry:    templates.NewRegistry(),
			Persistence: persistence.NewPlaceholder(),
			Installer:   fake,
		})
		if !errors.Is(err, errWizardMemoryFailed) {
			t.Fatalf("a non-zero Engram save must fail the setup, got %v", err)
		}
	})
	if strings.Contains(stdout, "[wizard] staged setup flow completed") {
		t.Fatalf("a non-zero Engram save must not complete the staged flow: %s", stdout)
	}
	artifact := readWizardArtifactFixture(t, finalPath)
	if artifact.MemoryOutcome.Status != wizardMemoryFailed {
		t.Fatalf("expected an honest failed memory outcome, got %#v", artifact.MemoryOutcome)
	}
}

func TestRunWizard_MetadataOnlyEngramFailureFails(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")

	stdout := captureStdout(t, func() {
		err := RunWizard(WizardInput{
			Args:        []string{"--metadata-only", "--non-interactive", "--final-artifact", finalPath},
			Registry:    templates.NewRegistry(),
			Persistence: persistence.NewPlaceholder().WithEngramExecutable(filepath.Join(workspace, "missing-engram")),
		})
		if !errors.Is(err, errWizardMemoryFailed) {
			t.Fatalf("metadata-only memory failure must be a non-success, got %v", err)
		}
	})
	if strings.Contains(stdout, "[wizard] staged setup flow completed") {
		t.Fatalf("metadata-only must not claim completion after a memory failure: %s", stdout)
	}
	artifact := readWizardArtifactFixture(t, finalPath)
	if artifact.MemoryOutcome.Status != wizardMemoryFailed {
		t.Fatalf("expected an honest failed metadata-only memory outcome, got %#v", artifact.MemoryOutcome)
	}
}

func TestRunWizard_DisabledEngramRecordsDisabled(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")

	if err := RunWizard(WizardInput{
		Args:        []string{"--metadata-only", "--non-interactive", "--final-artifact", finalPath},
		Registry:    templates.NewRegistry(),
		Persistence: persistence.NewPlaceholder().WithEngramEnabled(false),
	}); err != nil {
		t.Fatalf("disabled Engram persistence must not fail the wizard, got %v", err)
	}
	artifact := readWizardArtifactFixture(t, finalPath)
	if artifact.MemoryOutcome.Status != wizardMemoryDisabled {
		t.Fatalf("disabled persistence must be recorded as explicitly skipped, got %#v", artifact.MemoryOutcome)
	}
}

func TestWizardPersistGeneration_CanceledContextRecordsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	state := &wizardState{}
	err := wizardPersistGeneration(ctx, persistence.NewPlaceholder().WithEngramExecutable(newFakeEngram(t, "#!/bin/sh\nexit 0\n")), state, persistence.GenerationWriteThrough{
		ProfileName:      "Canceled Profile",
		Engine:           "godot",
		PersistenceMode:  "hybrid",
		SummaryArtifact:  "out/wizard/final.artifact.json",
		GeneratedAt:      time.Now().UTC(),
		EmittedArtifacts: []string{"out/wizard/final.artifact.json"},
	})
	if err == nil {
		t.Fatal("a canceled context must fail the memory write-through")
	}
	if state.MemoryStatus != wizardMemoryCanceled {
		t.Fatalf("expected a canceled memory outcome, got %q", state.MemoryStatus)
	}
}
