package commands

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"open-game-studios/internal/integrations"
	"open-game-studios/internal/persistence"
	"open-game-studios/internal/templates"
	"open-game-studios/internal/toolcheck"
	"open-game-studios/internal/workdoc"
)

type fakeEnvChecker struct {
	results []toolcheck.Result
	seen    [][]string
	modes   []toolcheck.SelectionMode
}

func (f *fakeEnvChecker) Execute(_ context.Context, selection toolcheck.Selection) []toolcheck.Result {
	f.seen = append(f.seen, append([]string{}, selection.ToolIDs...))
	f.modes = append(f.modes, selection.Mode)
	return append([]toolcheck.Result{}, f.results...)
}

type commandTestCheck struct {
	meta toolcheck.Metadata
}

func (c commandTestCheck) Metadata() toolcheck.Metadata { return c.meta }
func (c commandTestCheck) Run(context.Context, toolcheck.CommandRunner) toolcheck.Result {
	return toolcheck.Result{ToolID: c.meta.ID, ToolName: c.meta.Name, Status: toolcheck.StatusSuccess}
}

func TestRunEnvCheck_SelectedToolAndBlockingOutput(t *testing.T) {
	fake := &fakeEnvChecker{results: []toolcheck.Result{{ToolID: "blender", Status: toolcheck.StatusFailure, Reason: "missing", Attempted: "blender --version", Remediation: "add Blender to PATH"}}}

	stdout := captureStdout(t, func() {
		err := RunEnvCheck(EnvCheckInput{Args: []string{"--tool", "blender"}, Checker: fake})
		if err == nil {
			t.Fatalf("expected blocking env-check error")
		}
		if !strings.Contains(err.Error(), "blender") {
			t.Fatalf("expected error to mention blender, got %v", err)
		}
	})

	if !reflect.DeepEqual(fake.seen, [][]string{{"blender"}}) {
		t.Fatalf("selection=%v, want only blender", fake.seen)
	}
	if !reflect.DeepEqual(fake.modes, []toolcheck.SelectionMode{toolcheck.SelectionModeSelected}) {
		t.Fatalf("selection modes=%v, want selected", fake.modes)
	}
	for _, token := range []string{"[env-check] blender: failure", "attempted: blender --version", "add Blender to PATH"} {
		if !strings.Contains(stdout, token) {
			t.Fatalf("expected output token %q, got %s", token, stdout)
		}
	}
}

func TestRunEnvCheck_DefaultSelectionIsExplicitAll(t *testing.T) {
	fake := &fakeEnvChecker{}
	if err := RunEnvCheck(EnvCheckInput{Checker: fake}); err != nil {
		t.Fatalf("RunEnvCheck returned error: %v", err)
	}
	if !reflect.DeepEqual(fake.modes, []toolcheck.SelectionMode{toolcheck.SelectionModeAll}) {
		t.Fatalf("selection modes=%v, want explicit all", fake.modes)
	}
}

func TestRunEnvCheck_DefaultSelectionUsesInitialSet(t *testing.T) {
	runner := toolcheck.NewRunner(toolcheck.NewDefaultRegistry(), &commandRunnerAllMissing{})
	stdout := captureStdout(t, func() {
		if err := RunEnvCheck(EnvCheckInput{Checker: runner}); err == nil {
			t.Fatalf("expected missing default tools to be blocking")
		}
	})
	if !strings.Contains(stdout, "[env-check] blender: failure") || !strings.Contains(stdout, "[env-check] godot: failure") {
		t.Fatalf("expected default Godot and Blender checks, got %s", stdout)
	}
}

type commandRunnerAllMissing struct{}

func (commandRunnerAllMissing) LookPath(name string) (string, error) { return "", os.ErrNotExist }
func (commandRunnerAllMissing) Run(context.Context, string, ...string) (toolcheck.CommandResult, error) {
	return toolcheck.CommandResult{}, os.ErrNotExist
}

func TestRunInit_PreflightUsesPrimaryEngineOnlyBeforeWrites(t *testing.T) {
	root := t.TempDir()
	docPath := filepath.Join(root, "GAME-STUDIO.md")
	writeWorkdocFixture(t, docPath, "godot")
	fake := &fakeEnvChecker{results: []toolcheck.Result{{ToolID: "godot", Status: toolcheck.StatusSuccess}}}

	stdout := captureStdout(t, func() {
		if err := RunInit(InitInput{Args: []string{"--root", root, "--doc", docPath}, Persistence: persistence.NewPlaceholder(), EnvChecker: fake}); err != nil {
			t.Fatalf("RunInit returned error: %v", err)
		}
	})

	if !reflect.DeepEqual(fake.seen, [][]string{{"godot"}}) {
		t.Fatalf("selection=%v, want only godot", fake.seen)
	}
	if strings.Contains(stdout, "blender") {
		t.Fatalf("init output should not mention unselected Blender, got %s", stdout)
	}
}

func TestRunInit_PreflightWithNoRelevantToolsDoesNotRunAll(t *testing.T) {
	root := t.TempDir()
	docPath := filepath.Join(root, "GAME-STUDIO.md")
	writeWorkdocFixture(t, docPath, "unity")
	fake := &fakeEnvChecker{}

	stdout := captureStdout(t, func() {
		if err := RunInit(InitInput{Args: []string{"--root", root, "--doc", docPath}, Persistence: persistence.NewPlaceholder(), EnvChecker: fake}); err != nil {
			t.Fatalf("RunInit returned error: %v", err)
		}
	})

	if !reflect.DeepEqual(fake.modes, []toolcheck.SelectionMode{toolcheck.SelectionModeSelected}) || !reflect.DeepEqual(fake.seen, [][]string{{}}) {
		t.Fatalf("selection modes=%v ids=%v, want explicit empty selected", fake.modes, fake.seen)
	}
	if strings.Contains(stdout, "[init preflight] blender") || strings.Contains(stdout, "[init preflight] godot") {
		t.Fatalf("init should not run all checks when no relevant tools are selected, got %s", stdout)
	}
}

func TestRequiredToolSelectionUsesRegistryForFutureTools(t *testing.T) {
	registry := toolcheck.NewDefaultRegistry()
	if err := registry.Register(commandTestCheck{meta: toolcheck.Metadata{ID: "maya", Name: "Maya", Required: true}}); err != nil {
		t.Fatalf("register future tool: %v", err)
	}
	if err := registry.Register(commandTestCheck{meta: toolcheck.Metadata{ID: "go", Name: "Go", Required: true}}); err != nil {
		t.Fatalf("register short future tool: %v", err)
	}

	tests := []struct {
		name string
		got  []string
		want []string
	}{
		{
			name: "init primary engine can select future registered tool",
			got:  initRequiredToolIDs(workdocFixtureWithEngine("maya"), registry),
			want: []string{"maya"},
		},
		{
			name: "wizard resource labels can select future registered tool",
			got:  wizardRequiredToolIDs(wizardState{Packs: []string{"maya-core"}, Connectors: []string{"godot-docs"}, Tools: []string{"formatter"}}, registry),
			want: []string{"godot", "maya"},
		},
		{
			name: "wizard unselected registered tool is not forced",
			got:  wizardRequiredToolIDs(wizardState{Packs: []string{"godot-core"}, Tools: []string{"formatter"}}, registry),
			want: []string{"godot"},
		},
		{
			name: "short future id does not match inside another tool id",
			got:  wizardRequiredToolIDs(wizardState{Packs: []string{"godot-core"}}, registry),
			want: []string{"godot"},
		},
		{
			name: "godot4 alias selects canonical godot",
			got:  wizardRequiredToolIDs(wizardState{Tools: []string{"godot4"}}, registry),
			want: []string{"godot"},
		},
		{
			name: "alias text does not create substring false positive",
			got:  wizardRequiredToolIDs(wizardState{Tools: []string{"godot40"}}, registry),
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !reflect.DeepEqual(tt.got, tt.want) {
				t.Fatalf("tool ids=%v, want %v", tt.got, tt.want)
			}
		})
	}
}

func TestRunWizard_PreflightWithNoRelevantToolsDoesNotRunAll(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
	fake := &fakeEnvChecker{}
	input := strings.Join([]string{
		"create_from_scratch",
		"godot-core",
		"minimal",
		"No Tools Profile",
		"none",
		"none",
		"none",
		"",
		"yes",
	}, "\n") + "\n"

	stdout := captureStdout(t, func() {
		err := withStdinText(t, input, func() error {
			return RunWizard(WizardInput{Args: []string{"--metadata-only", "--final-artifact", finalPath}, Registry: templates.NewRegistry(), Persistence: persistence.NewPlaceholder(), EnvChecker: fake})
		})
		if err != nil {
			t.Fatalf("RunWizard returned error: %v", err)
		}
	})

	if !reflect.DeepEqual(fake.modes, []toolcheck.SelectionMode{toolcheck.SelectionModeSelected}) || !reflect.DeepEqual(fake.seen, [][]string{{}}) {
		t.Fatalf("selection modes=%v ids=%v, want explicit empty selected", fake.modes, fake.seen)
	}
	if strings.Contains(stdout, "[wizard preflight] godot") || strings.Contains(stdout, "[wizard preflight] blender") {
		t.Fatalf("wizard should not run all checks when no relevant tools are selected, got %s", stdout)
	}
	if !strings.Contains(stdout, "Blender not selected: skipped") || !strings.Contains(stdout, "ComfyUI lane deferred: not validated") {
		t.Fatalf("wizard summary should explain skipped/deferred validation scope, got %s", stdout)
	}
}

func TestRunWizard_SkippedPreflightDoesNotClaimEngineReadiness(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")

	if err := RunWizard(WizardInput{Args: []string{"--metadata-only", "--non-interactive", "--setup-depth", "minimal", "--engine-pack", "godot-core", "--final-artifact", finalPath}, Registry: templates.NewRegistry(), Persistence: persistence.NewPlaceholder()}); err != nil {
		t.Fatalf("RunWizard returned error: %v", err)
	}

	artifact := readWizardArtifactFixture(t, finalPath)
	if artifact.PreflightResult.Status != "skipped" {
		t.Fatalf("preflight status=%q, want skipped", artifact.PreflightResult.Status)
	}
	if !artifact.ProfileGenerationAllowed || !artifact.MetadataOnlyGeneration || !artifact.CanContinueMetadataOnly || artifact.EngineWorkflowReady {
		t.Fatalf("expected metadata-only generation with unknown engine readiness, got %#v", artifact.GenerationReadiness)
	}
	if len(artifact.EnvBlockers) != 0 || len(artifact.BlockedWorkflows) != 0 || artifact.BlockingReason != "" {
		t.Fatalf("skipped preflight must not invent blockers, blockers=%#v workflows=%#v reason=%q", artifact.EnvBlockers, artifact.BlockedWorkflows, artifact.BlockingReason)
	}
	readinessSummary := strings.Join(artifact.GenerationReadiness.SummaryLines, "\n")
	if !strings.Contains(readinessSummary, "engine_workflow_ready: false") || !strings.Contains(readinessSummary, "readiness is unknown") {
		t.Fatalf("expected unknown readiness summary, got %q", readinessSummary)
	}
	if strings.Contains(readinessSummary, "engine_workflow_ready: true") || strings.Contains(readinessSummary, "preflight has no blockers") {
		t.Fatalf("skipped preflight must not be summarized as passed, got %q", readinessSummary)
	}
}

func TestRunWizard_PreflightBlocksRuntimeButPersistsMetadataArtifact(t *testing.T) {
	for _, tt := range []struct {
		name       string
		setupDepth string
	}{
		{name: "minimal", setupDepth: "minimal"},
		{name: "recommended", setupDepth: "recommended"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			workspace := wizardWorkspaceFixture(t)
			setWorkingDirectory(t, workspace)
			t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))
			finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
			fake := &fakeEnvChecker{results: []toolcheck.Result{{ToolID: "godot", Status: toolcheck.StatusFailure, Reason: `godot lookup: exec: "godot": executable file not found in $PATH`, Attempted: "godot --version", Remediation: "install Godot and update PATH"}}}

			stdout := captureStdout(t, func() {
				err := RunWizard(WizardInput{Args: []string{"--metadata-only", "--non-interactive", "--setup-depth", tt.setupDepth, "--engine-pack", "godot-core", "--final-artifact", finalPath}, Registry: templates.NewRegistry(), Persistence: persistence.NewPlaceholder(), EnvChecker: fake})
				if err == nil {
					t.Fatalf("expected wizard preflight failure")
				}
				if !strings.Contains(err.Error(), "godot") {
					t.Fatalf("expected error to mention godot, got %v", err)
				}
			})

			if !reflect.DeepEqual(fake.seen, [][]string{{"godot"}}) {
				t.Fatalf("selection=%v, want only godot", fake.seen)
			}
			if !strings.Contains(stdout, "install Godot") {
				t.Fatalf("expected remediation output, got %s", stdout)
			}
			for _, token := range []string{"Godot selected: executable missing or not found in PATH.", `godot lookup: exec: "godot": executable file not found in $PATH`, "install Godot and update PATH", "Profile metadata can still be generated.", "Godot workflows are not ready", "metadata-only profile artifacts validated"} {
				if !strings.Contains(stdout, token) {
					t.Fatalf("expected metadata-only blocker token %q, got %s", token, stdout)
				}
			}
			artifact := readWizardArtifactFixture(t, finalPath)
			if len(artifact.EnvBlockers) != 1 || !strings.Contains(artifact.EnvBlockers[0], "godot") {
				t.Fatalf("expected persisted godot env blocker, got %#v", artifact.EnvBlockers)
			}
			if artifact.PreflightResult.Status != "blocker" || artifact.PreflightResult.CanContinue {
				t.Fatalf("expected blocking preflight result, got %#v", artifact.PreflightResult)
			}
			if !artifact.ProfileGenerationAllowed || !artifact.MetadataOnlyGeneration || !artifact.CanContinueMetadataOnly || artifact.EngineWorkflowReady {
				t.Fatalf("expected metadata-only generation allowed and engine workflow blocked, got %#v", artifact.GenerationReadiness)
			}
			if !containsString(artifact.BlockedWorkflows, "godot-runtime-workflows") || !strings.Contains(strings.ToLower(artifact.BlockingReason), "not found") || !strings.Contains(artifact.BlockingReason, "install Godot and update PATH") {
				t.Fatalf("expected godot runtime workflow blocker, workflows=%#v reason=%q", artifact.BlockedWorkflows, artifact.BlockingReason)
			}
			if !containsString(artifact.SelectedTools, "godot") || !containsString(artifact.ValidatedTools, "godot") || containsString(artifact.SelectedTools, "blender") || containsString(artifact.ValidatedTools, "blender") {
				t.Fatalf("expected only selected godot validation, selected=%#v validated=%#v", artifact.SelectedTools, artifact.ValidatedTools)
			}
			nextSteps := strings.Join(artifact.NextSteps, "\n")
			if len(artifact.NextSteps) == 0 || !strings.Contains(nextSteps, "Godot executable lookup/PATH failure") || !strings.Contains(nextSteps, "install Godot and update PATH") {
				t.Fatalf("expected persisted godot next steps, got %#v", artifact.NextSteps)
			}
		})

	}
}

func TestRunWizard_MultipleRequiredToolBlockersPersistAllReasons(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
	fake := &fakeEnvChecker{results: []toolcheck.Result{
		{ToolID: "godot", ToolName: "Godot", Status: toolcheck.StatusFailure, Severity: toolcheck.SeverityRequired, Required: true, Reason: `godot lookup: exec: "godot": executable file not found in $PATH`, Attempted: "godot --version", Remediation: "install Godot and update PATH"},
		{ToolID: "blender", ToolName: "Blender", Status: toolcheck.StatusFailure, Severity: toolcheck.SeverityRequired, Required: true, Reason: "missing", Attempted: "blender --version", Remediation: "add Blender to PATH"},
	}}
	input := strings.Join([]string{
		"create_from_scratch",
		"godot-core",
		"custom",
		"Multiple Blockers Profile",
		"none",
		"godot-core",
		"blender",
		"none",
		"yes",
	}, "\n") + "\n"

	stdout := captureStdout(t, func() {
		err := withStdinText(t, input, func() error {
			return RunWizard(WizardInput{Args: []string{"--metadata-only", "--final-artifact", finalPath}, Registry: templates.NewRegistry(), Persistence: persistence.NewPlaceholder(), EnvChecker: fake})
		})
		if err == nil {
			t.Fatalf("expected wizard preflight failure")
		}
		if !strings.Contains(err.Error(), "godot") || !strings.Contains(err.Error(), "blender") {
			t.Fatalf("expected error to mention both blockers, got %v", err)
		}
	})

	if !reflect.DeepEqual(fake.seen, [][]string{{"blender", "godot"}}) {
		t.Fatalf("selection=%v, want godot and blender", fake.seen)
	}
	for _, token := range []string{"Godot workflows are not ready", "Blender selected: validation failed.", "Required tool workflows are not ready until blender passes preflight validation.", "add Blender to PATH"} {
		if !strings.Contains(stdout, token) {
			t.Fatalf("expected multiple-blocker output token %q, got %s", token, stdout)
		}
	}

	artifact := readWizardArtifactFixture(t, finalPath)
	if len(artifact.EnvBlockers) != 2 || !strings.Contains(strings.Join(artifact.EnvBlockers, "\n"), "godot") || !strings.Contains(strings.Join(artifact.EnvBlockers, "\n"), "blender") {
		t.Fatalf("expected persisted godot and blender blockers, got %#v", artifact.EnvBlockers)
	}
	if !containsString(artifact.BlockedWorkflows, "godot-runtime-workflows") || !containsString(artifact.BlockedWorkflows, "blender-workflows") {
		t.Fatalf("expected both blocked workflows, got %#v", artifact.BlockedWorkflows)
	}
	if !strings.Contains(strings.ToLower(artifact.BlockingReason), "godot") || !strings.Contains(strings.ToLower(artifact.BlockingReason), "missing") || !strings.Contains(artifact.BlockingReason, "blender") || !strings.Contains(artifact.BlockingReason, "add Blender to PATH") {
		t.Fatalf("expected blocking reason to include both blockers, got %q", artifact.BlockingReason)
	}
	nextSteps := strings.Join(artifact.NextSteps, "\n")
	if !strings.Contains(nextSteps, "Godot executable lookup/PATH failure") || !strings.Contains(nextSteps, "install Godot and update PATH") || !strings.Contains(nextSteps, "blender") || !strings.Contains(nextSteps, "add Blender to PATH") {
		t.Fatalf("expected next steps to include both blocker remediations, got %#v", artifact.NextSteps)
	}
	if artifact.PreflightResult.Status != "blocker" || artifact.PreflightResult.CanContinue || !artifact.ProfileGenerationAllowed || !artifact.MetadataOnlyGeneration || !artifact.CanContinueMetadataOnly || artifact.EngineWorkflowReady {
		t.Fatalf("expected persisted metadata-only blocker artifact, got preflight=%#v readiness=%#v", artifact.PreflightResult, artifact.GenerationReadiness)
	}
}

func TestBlockingReasonForGodotBlockerPreservesCheckerDiagnosis(t *testing.T) {
	tests := []struct {
		name            string
		blocker         toolcheck.Result
		wantContains    []string
		wantNotContains []string
	}{
		{
			name: "missing executable keeps missing and path guidance",
			blocker: toolcheck.Result{
				ToolID:      "godot",
				Status:      toolcheck.StatusFailure,
				Reason:      `godot lookup: exec: "godot": executable file not found in $PATH`,
				Attempted:   "godot --version",
				Remediation: "Install Godot 4 and ensure godot or godot4 is available on PATH.",
			},
			wantContains: []string{"executable file not found", "PATH", "Install Godot 4"},
		},
		{
			name: "invalid version is not reported as missing",
			blocker: toolcheck.Result{
				ToolID:      "godot",
				Status:      toolcheck.StatusFailure,
				Reason:      "godot version: Godot 4 is required; detected Godot Engine v3.5.3",
				Attempted:   "godot --version (/usr/bin/godot)",
				Remediation: "Install Godot 4 and ensure godot or godot4 is available on PATH.",
			},
			wantContains:    []string{"Godot 4 is required", "detected Godot Engine v3.5.3", "Install Godot 4"},
			wantNotContains: []string{"Godot executable not found"},
		},
		{
			name: "unparseable version is not reported as missing",
			blocker: toolcheck.Result{
				ToolID:      "godot",
				Status:      toolcheck.StatusFailure,
				Reason:      "godot version: Godot 4 is required; could not parse version output: custom engine",
				Attempted:   "godot --version (/usr/bin/godot)",
				Remediation: "Install Godot 4 and ensure godot or godot4 is available on PATH.",
			},
			wantContains:    []string{"could not parse version output", "custom engine", "Install Godot 4"},
			wantNotContains: []string{"Godot executable not found"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := blockingReasonForBlocker(tt.blocker)
			for _, token := range tt.wantContains {
				if !strings.Contains(got, token) {
					t.Fatalf("expected blocking reason to contain %q, got %q", token, got)
				}
			}
			for _, token := range tt.wantNotContains {
				if strings.Contains(got, token) {
					t.Fatalf("expected blocking reason not to contain %q, got %q", token, got)
				}
			}
		})
	}
}

func TestRunWizard_GodotBlockingReasonUsesCheckerDiagnosis(t *testing.T) {
	for _, tt := range []struct {
		name                       string
		reason                     string
		remediation                string
		wantSummaryContains        []string
		wantNextStepsContains      []string
		wantBlockingReasonContains []string
		wantNotContains            []string
	}{
		{
			name:                       "invalid version",
			reason:                     "godot version: Godot 4 is required; detected Godot Engine v3.5.3",
			remediation:                "Select a compatible Godot 4 release.",
			wantSummaryContains:        []string{"incompatible or invalid version", "Godot 4 is required", "detected Godot Engine v3.5.3", "Select a compatible Godot 4 release."},
			wantNextStepsContains:      []string{"Resolve the incompatible or invalid Godot version", "Select a compatible Godot 4 release"},
			wantBlockingReasonContains: []string{"Godot 4 is required", "detected Godot Engine v3.5.3", "Select a compatible Godot 4 release."},
			wantNotContains:            []string{"missing", "not found"},
		},
		{
			name:                       "unparseable version output",
			reason:                     "godot version: Godot 4 is required; could not parse version output: custom engine",
			remediation:                "Verify that the selected Godot binary prints a standard version string.",
			wantSummaryContains:        []string{"parse/verification failed", "could not parse version output", "custom engine", "Verify that the selected Godot binary prints a standard version string."},
			wantNextStepsContains:      []string{"Resolve the Godot version parse/verification failure", "Verify that the selected Godot binary prints a standard version string"},
			wantBlockingReasonContains: []string{"could not parse version output", "custom engine", "Verify that the selected Godot binary prints a standard version string."},
			wantNotContains:            []string{"missing", "not found"},
		},
		{
			name:                       "permission error",
			reason:                     "godot run: permission denied",
			remediation:                "Restore execute permission for /usr/bin/godot.",
			wantSummaryContains:        []string{"Godot selected: validation failed", "permission denied", "Restore execute permission for /usr/bin/godot."},
			wantNextStepsContains:      []string{"Resolve the reported Godot validation failure", "Restore execute permission for /usr/bin/godot"},
			wantBlockingReasonContains: []string{"permission denied", "Restore execute permission for /usr/bin/godot."},
			wantNotContains:            []string{"missing", "not found", "incompatible or invalid"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			workspace := wizardWorkspaceFixture(t)
			setWorkingDirectory(t, workspace)
			t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))
			finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
			fake := &fakeEnvChecker{results: []toolcheck.Result{{ToolID: "godot", ToolName: "Godot", Status: toolcheck.StatusFailure, Severity: toolcheck.SeverityRequired, Required: true, Reason: tt.reason, Attempted: "godot --version (/usr/bin/godot)", Remediation: tt.remediation}}}

			err := RunWizard(WizardInput{Args: []string{"--metadata-only", "--non-interactive", "--setup-depth", "minimal", "--engine-pack", "godot-core", "--final-artifact", finalPath}, Registry: templates.NewRegistry(), Persistence: persistence.NewPlaceholder(), EnvChecker: fake})
			if err == nil {
				t.Fatalf("expected wizard preflight failure")
			}

			artifact := readWizardArtifactFixture(t, finalPath)
			readinessSummary := strings.Join(artifact.GenerationReadiness.SummaryLines, "\n")
			nextSteps := strings.Join(artifact.NextSteps, "\n")
			for _, token := range tt.wantSummaryContains {
				if !strings.Contains(readinessSummary, token) {
					t.Fatalf("expected readiness summary to contain %q, got %q", token, readinessSummary)
				}
			}
			for _, token := range tt.wantNextStepsContains {
				if !strings.Contains(nextSteps, token) {
					t.Fatalf("expected next steps to contain %q, got %q", token, nextSteps)
				}
			}
			for _, token := range tt.wantBlockingReasonContains {
				if !strings.Contains(artifact.BlockingReason, token) {
					t.Fatalf("expected blocking reason to contain %q, got %q", token, artifact.BlockingReason)
				}
			}
			for _, token := range tt.wantNotContains {
				guidance := strings.ToLower(readinessSummary + "\n" + nextSteps + "\n" + artifact.BlockingReason)
				if strings.Contains(guidance, strings.ToLower(token)) {
					t.Fatalf("expected readiness guidance not to contain %q, summary=%q next_steps=%q blocking_reason=%q", token, readinessSummary, nextSteps, artifact.BlockingReason)
				}
			}
			if artifact.PreflightResult.Status != "blocker" || artifact.PreflightResult.CanContinue || !artifact.MetadataOnlyGeneration || !artifact.CanContinueMetadataOnly || artifact.EngineWorkflowReady {
				t.Fatalf("expected persisted metadata-only blocker artifact, got preflight=%#v readiness=%#v", artifact.PreflightResult, artifact.GenerationReadiness)
			}
			if len(artifact.EnvBlockers) == 0 || len(artifact.NextSteps) == 0 || !containsString(artifact.BlockedWorkflows, "godot-runtime-workflows") {
				t.Fatalf("expected env blockers, next steps, and godot blocked workflow, artifact=%#v", artifact)
			}
		})
	}
}

func TestRunWizard_NonGodotRequiredToolBlockerUsesToolSpecificReadiness(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
	fake := &fakeEnvChecker{results: []toolcheck.Result{{ToolID: "blender", ToolName: "Blender", Status: toolcheck.StatusFailure, Severity: toolcheck.SeverityRequired, Required: true, Reason: "missing", Attempted: "blender --version", Remediation: "add Blender to PATH."}}}
	input := strings.Join([]string{
		"create_from_scratch",
		"godot-core",
		"custom",
		"Blender Blocked Profile",
		"none",
		"none",
		"blender",
		"none",
		"yes",
	}, "\n") + "\n"

	stdout := captureStdout(t, func() {
		err := withStdinText(t, input, func() error {
			return RunWizard(WizardInput{Args: []string{"--metadata-only", "--final-artifact", finalPath}, Registry: templates.NewRegistry(), Persistence: persistence.NewPlaceholder(), EnvChecker: fake})
		})
		if err == nil {
			t.Fatalf("expected wizard preflight failure")
		}
		if !strings.Contains(err.Error(), "blender") {
			t.Fatalf("expected error to mention blender, got %v", err)
		}
	})

	if !reflect.DeepEqual(fake.seen, [][]string{{"blender"}}) {
		t.Fatalf("selection=%v, want only blender", fake.seen)
	}
	if strings.Contains(stdout, "Godot workflows are not ready") || strings.Contains(stdout, "Install Godot 4.x") {
		t.Fatalf("non-godot blocker should not emit Godot-specific readiness, got %s", stdout)
	}
	for _, token := range []string{"Selected required tool validation failed.", "Required tool workflows are not ready until blender passes preflight validation.", "add Blender to PATH"} {
		if !strings.Contains(stdout, token) {
			t.Fatalf("expected non-godot blocker token %q, got %s", token, stdout)
		}
	}

	artifact := readWizardArtifactFixture(t, finalPath)
	if len(artifact.EnvBlockers) != 1 || !strings.Contains(artifact.EnvBlockers[0], "blender") || !strings.Contains(artifact.EnvBlockers[0], "add Blender to PATH") {
		t.Fatalf("expected persisted blender blocker with remediation, got %#v", artifact.EnvBlockers)
	}
	if !containsString(artifact.BlockedWorkflows, "blender-workflows") || !strings.Contains(artifact.BlockingReason, "blender") {
		t.Fatalf("expected blender workflow blocker, workflows=%#v reason=%q", artifact.BlockedWorkflows, artifact.BlockingReason)
	}
	nextSteps := strings.Join(artifact.NextSteps, "\n")
	if strings.Contains(nextSteps, "Godot") || !strings.Contains(nextSteps, "blender") || !strings.Contains(nextSteps, "add Blender to PATH") {
		t.Fatalf("expected blender-specific next steps without Godot wording, got %#v", artifact.NextSteps)
	}
	if strings.Contains(nextSteps, "PATH., then rerun") || !strings.Contains(nextSteps, "PATH, then rerun env-check or wizard validation.") {
		t.Fatalf("expected exactly one punctuation separator before rerun guidance, got %#v", artifact.NextSteps)
	}
	if artifact.PreflightResult.Status != "blocker" || artifact.PreflightResult.CanContinue || !artifact.ProfileGenerationAllowed || !artifact.MetadataOnlyGeneration || !artifact.CanContinueMetadataOnly || artifact.EngineWorkflowReady {
		t.Fatalf("expected persisted metadata-only blocker artifact, got preflight=%#v readiness=%#v", artifact.PreflightResult, artifact.GenerationReadiness)
	}
}

func TestRunWizard_PreflightWarningsRemainVisible(t *testing.T) {
	t.Run("selected warning proceeds and is persisted", func(t *testing.T) {
		workspace := wizardWorkspaceFixture(t)
		setWorkingDirectory(t, workspace)
		t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))
		finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
		fake := &fakeEnvChecker{results: []toolcheck.Result{{ToolID: "godot", Status: toolcheck.StatusWarning, Reason: "incomplete version", Attempted: "godot --version", Remediation: "verify PATH"}}}

		stdout := captureStdout(t, func() {
			if err := RunWizard(WizardInput{Args: []string{"--metadata-only", "--non-interactive", "--final-artifact", finalPath}, Registry: templates.NewRegistry(), Persistence: persistence.NewPlaceholder(), EnvChecker: fake}); err != nil {
				t.Fatalf("RunWizard returned error: %v", err)
			}
		})

		for _, token := range []string{"[wizard][step-8] Summary", "  validation_scope:", "    - Godot selected: validating Godot.", "    - Blender not selected: skipped", "    - ComfyUI lane deferred: not validated", "  env_warnings:", "    - godot: warning", "incomplete version", "verify PATH"} {
			if !strings.Contains(stdout, token) {
				t.Fatalf("expected wizard summary warning token %q, got %s", token, stdout)
			}
		}
		summaryIndex := strings.Index(stdout, "[wizard][step-8] Summary")
		warningIndex := strings.Index(stdout, "  env_warnings:")
		confirmationIndex := strings.Index(stdout, "[wizard][step-9] Confirmation")
		if summaryIndex < 0 || warningIndex < 0 || confirmationIndex < 0 || !(summaryIndex < warningIndex && warningIndex < confirmationIndex) {
			t.Fatalf("expected env warnings inside summary before confirmation, got %s", stdout)
		}
		artifact := readWizardArtifactFixture(t, finalPath)
		if len(artifact.EnvWarnings) != 1 || !strings.Contains(artifact.EnvWarnings[0], "incomplete version") {
			t.Fatalf("expected persisted env warning, got %#v", artifact.EnvWarnings)
		}
		if !sameStrings(artifact.SelectedTools, []string{"godot"}) || !sameStrings(artifact.ValidatedTools, []string{"godot"}) {
			t.Fatalf("expected scoped selected/validated tools to be godot only, selected=%#v validated=%#v", artifact.SelectedTools, artifact.ValidatedTools)
		}
		if !containsString(artifact.SkippedTools, "blender") || !containsString(artifact.DeferredTools, "comfyui-workflows") || !containsString(artifact.DeferredTools, "provider-native-image-generation") || !containsString(artifact.DeferredTools, "audio-generation") {
			t.Fatalf("expected skipped/deferred tools in artifact, skipped=%#v deferred=%#v", artifact.SkippedTools, artifact.DeferredTools)
		}
		if artifact.PreflightResult.Status != "warning" || !artifact.PreflightResult.CanContinue || len(artifact.ValidationScope.Tools) == 0 || len(artifact.DeepValidationRequired) == 0 {
			t.Fatalf("expected machine-readable validation scope/preflight info, scope=%#v preflight=%#v deep=%#v", artifact.ValidationScope, artifact.PreflightResult, artifact.DeepValidationRequired)
		}
	})
}

func TestRunWizard_MinimalPreflightValidatesOnlySelectedCoreTool(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
	fake := &fakeEnvChecker{results: []toolcheck.Result{{ToolID: "godot", Status: toolcheck.StatusSuccess, Reason: "detected Godot 4"}}}

	stdout := captureStdout(t, func() {
		if err := RunWizard(WizardInput{Args: []string{"--metadata-only", "--non-interactive", "--setup-depth", "minimal", "--final-artifact", finalPath}, Registry: templates.NewRegistry(), Persistence: persistence.NewPlaceholder(), EnvChecker: fake}); err != nil {
			t.Fatalf("RunWizard returned error: %v", err)
		}
	})

	if !reflect.DeepEqual(fake.seen, [][]string{{"godot"}}) {
		t.Fatalf("minimal preflight selected tools=%#v, want godot only", fake.seen)
	}
	for _, token := range []string{"Godot selected: validating Godot.", "Blender not selected: skipped", "ComfyUI lane deferred: not validated", "Audio lane future scope: not validated"} {
		if !strings.Contains(stdout, token) {
			t.Fatalf("expected scoped validation summary token %q, got %s", token, stdout)
		}
	}
	artifact := readWizardArtifactFixture(t, finalPath)
	// This run is --metadata-only, so a passing preflight alone must not claim
	// engine readiness: engine_workflow_ready requires complete executed runtime evidence.
	if artifact.PreflightResult.Status != "pass" || artifact.EngineWorkflowReady || !sameStrings(artifact.SelectedTools, []string{"godot"}) || !sameStrings(artifact.ValidatedTools, []string{"godot"}) {
		t.Fatalf("minimal scoped preflight mismatch: selected=%#v validated=%#v preflight=%#v", artifact.SelectedTools, artifact.ValidatedTools, artifact.PreflightResult)
	}
}

func TestRunWizard_SelectedOptionalToolFailureWarnsNotBlocks(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(workspace, "missing-auth.json"))
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
	registry := toolcheck.NewDefaultRegistry()
	if err := registry.Register(commandTestCheck{meta: toolcheck.Metadata{ID: "aseprite", Name: "Aseprite", Severity: toolcheck.SeverityOptional, Required: false}}); err != nil {
		t.Fatalf("register optional check: %v", err)
	}
	fake := &fakeEnvChecker{results: []toolcheck.Result{{ToolID: "aseprite", Status: toolcheck.StatusFailure, Severity: toolcheck.SeverityOptional, Required: false, Reason: "missing", Remediation: "install Aseprite or remove the optional tool"}}}
	input := strings.Join([]string{
		"create_from_scratch",
		"godot-core",
		"custom",
		"Optional Tool Profile",
		"none",
		"none",
		"aseprite",
		"",
		"yes",
	}, "\n") + "\n"

	stdout := captureStdout(t, func() {
		err := withStdinText(t, input, func() error {
			return RunWizard(WizardInput{Args: []string{"--metadata-only", "--final-artifact", finalPath}, Registry: templates.NewRegistry(), Persistence: persistence.NewPlaceholder(), EnvChecker: fake, ToolRegistry: registry})
		})
		if err != nil {
			t.Fatalf("RunWizard returned error: %v", err)
		}
	})

	if !strings.Contains(stdout, "aseprite selected: validating aseprite") || !strings.Contains(stdout, "env_warnings") {
		t.Fatalf("expected optional missing tool warning summary, got %s", stdout)
	}
	artifact := readWizardArtifactFixture(t, finalPath)
	if artifact.PreflightResult.Status != "warning" || !artifact.PreflightResult.CanContinue || len(artifact.EnvBlockers) != 0 || len(artifact.EnvWarnings) == 0 {
		t.Fatalf("optional selected missing tool should warn without blocking: preflight=%#v warnings=%#v blockers=%#v", artifact.PreflightResult, artifact.EnvWarnings, artifact.EnvBlockers)
	}
}

func TestOptionalDiagnosticResolutionIsSeparateAndNonBlocking(t *testing.T) {
	registry := toolcheck.NewDefaultRegistry()
	if err := registry.Register(commandTestCheck{meta: toolcheck.Metadata{ID: "engram-monitor", Name: "Engram Monitor", Required: true, Severity: toolcheck.SeverityRequired}}); err != nil {
		t.Fatalf("register optional diagnostic: %v", err)
	}

	ids := optionalDiagnosticToolIDs(registry, []integrations.OptionalIntegrationSelection{{ID: integrations.EngramMonitorID}, {ID: "missing-integration"}})
	if !reflect.DeepEqual(ids, []string{"engram-monitor"}) {
		t.Fatalf("optional diagnostics=%v, want engram-monitor only", ids)
	}

	warnings := optionalWarningResults([]toolcheck.Result{{ToolID: "engram-monitor", Status: toolcheck.StatusFailure, Severity: toolcheck.SeverityRequired, Required: true}})
	if len(warnings) != 1 || warnings[0].Status != toolcheck.StatusWarning || warnings[0].Severity != toolcheck.SeverityOptional || warnings[0].Required {
		t.Fatalf("optional result not downgraded to warning-only: %#v", warnings)
	}
	if err := toolcheck.BlockingError(warnings); err != nil {
		t.Fatalf("optional diagnostic must not block: %v", err)
	}
}

func writeWorkdocFixture(t *testing.T, path, engine string) {
	t.Helper()
	content := `# Game Studio

## 1) Vision

## 2) Product Definition
- User-facing profile count: ` + "`Game-Studio`" + `
- Primary engine target (phase 1): ` + engine + `
- Platform: Pi only
- Persistence mode: hybrid

## 3) Non-Negotiable Constraints

## 4) Architecture Direction (Working)

## 6) CCGS Preservation Notes
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write workdoc fixture: %v", err)
	}
}

func workdocFixtureWithEngine(engine string) workdoc.Document {
	return workdoc.Document{PrimaryEngine: engine}
}
