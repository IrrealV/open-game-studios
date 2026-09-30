package commands

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"open-game-studios/internal/persistence"
	"open-game-studios/internal/piinstall"
	"open-game-studios/internal/templates"
	"open-game-studios/internal/toolcheck"
)

// WizardInstaller is the wizard-owned seam over the accepted piinstall backend.
// It mirrors only Prepare and Execute so a deterministic fake can be injected in
// tests without widening or re-implementing the backend API.
type WizardInstaller interface {
	Prepare(ctx context.Context, cfg piinstall.Config) (piinstall.Detection, piinstall.Plan, error)
	Execute(ctx context.Context, cfg piinstall.Config, plan piinstall.Plan, consent piinstall.Consent) (piinstall.Report, error)
}

// defaultWizardInstaller is the production adapter. It performs no writes in
// Prepare and delegates consent, drift, blocked, and re-probe enforcement to the
// backend Execute.
type defaultWizardInstaller struct{}

func (defaultWizardInstaller) Prepare(ctx context.Context, cfg piinstall.Config) (piinstall.Detection, piinstall.Plan, error) {
	return piinstall.Prepare(ctx, cfg)
}

func (defaultWizardInstaller) Execute(ctx context.Context, cfg piinstall.Config, plan piinstall.Plan, consent piinstall.Consent) (piinstall.Report, error) {
	return piinstall.Execute(ctx, cfg, plan, consent)
}

// DefaultWizardInstaller returns the production prerequisite installer adapter.
func DefaultWizardInstaller() WizardInstaller {
	return defaultWizardInstaller{}
}

var (
	// errWizardRuntimeBlocked stops before consent when the prepared plan
	// contains a fail-closed blocked component.
	errWizardRuntimeBlocked = errors.New("wizard: runtime prerequisite plan contains a blocked component; resolve it before installing")
	// errWizardRuntimeIncomplete stops normal generation when Execute ran but
	// reported a failed or unverified step (Report.Failed, even with a nil error).
	errWizardRuntimeIncomplete = errors.New("wizard: prerequisite installation did not complete; setup artifacts were not generated")
	// errWizardMemoryFailed stops a non-success return when the configured Engram
	// write-through fails. Local artifacts remain, but the setup is not complete.
	errWizardMemoryFailed = errors.New("wizard: the Engram memory write-through did not complete; local artifacts remain but setup is not complete")
)

// Memory write-through outcome statuses recorded in the final artifact.
const (
	wizardMemoryPending  = "pending"
	wizardMemorySaved    = "saved"
	wizardMemoryFailed   = "failed"
	wizardMemoryCanceled = "canceled"
	wizardMemoryDisabled = "disabled"
)

// wizardCanceled reports the run-scoped context cancellation as an explicit
// error so the wizard never proceeds to a write boundary or a success path.
func wizardCanceled(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("wizard canceled: %w", err)
	}
	return nil
}

func wizardRuntimeConfig(input WizardInput, physicalWorkspace string, state wizardState) piinstall.Config {
	var cfg piinstall.Config
	if input.Config != nil {
		cfg = *input.Config
	}
	cfg.WorkspaceDir = physicalWorkspace
	cfg.GodotRequired = wizardGodotRequired(state)
	if strings.TrimSpace(cfg.InstallRoot) == "" {
		cfg.InstallRoot = wizardDefaultInstallRoot()
	}
	return cfg
}

// wizardGodotRequired is true only for the active Godot workflow modes. The
// design/narrative-only and visual-artifacts-only modes, and every deferred
// future engine pack, never mark Godot as required.
func wizardGodotRequired(state wizardState) bool {
	if normalizeEnginePack(state.EnginePack) != "godot-core" {
		return false
	}
	switch normalizeUseMode(state.UseMode) {
	case "create_from_scratch", "existing_game", "godot_sdd_handoff", "repair_change_workflow":
		return true
	default:
		return false
	}
}

func wizardDefaultInstallRoot() string {
	home := strings.TrimSpace(os.Getenv("HOME"))
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".local", "share", "open-game-studios")
}

func wizardPlanStatus(plan piinstall.Plan) string {
	switch {
	case plan.Blocked():
		return "blocked"
	case plan.NeedsConsent():
		return "approval_required"
	default:
		return "no_changes"
	}
}

func printWizardRuntimePlan(plan piinstall.Plan) {
	fmt.Println("[wizard] runtime prerequisite plan")
	fmt.Printf("  fingerprint: %s\n", plan.Fingerprint())
	fmt.Printf("  status: %s\n", wizardPlanStatus(plan))
	fmt.Printf("  godot_required: %t\n", plan.GodotRequired)
	if len(plan.Steps) == 0 {
		fmt.Println("  steps: none")
		return
	}
	fmt.Println("  steps:")
	for _, step := range plan.Steps {
		fmt.Printf("    - %s [%s]: %s\n", step.Component, step.Action, step.Reason)
		if step.Command != "" {
			fmt.Printf("        command: %s\n", step.Command)
		}
		if len(step.Outputs) > 0 {
			fmt.Printf("        destination: %s\n", strings.Join(step.Outputs, ", "))
		}
		for _, effect := range step.Effects {
			fmt.Printf("        effect: %s\n", effect)
		}
		fmt.Printf("        observed: %s\n", describeComponentState(step.Observed))
	}
}

func describeComponentState(state piinstall.ComponentState) string {
	parts := []string{string(state.Compatibility)}
	if state.Path != "" {
		parts = append(parts, "path="+state.Path)
	}
	if state.Version != "" {
		parts = append(parts, "version="+state.Version)
	}
	if state.ProbeDeferred {
		parts = append(parts, "probe_deferred=true")
	}
	if state.Detail != "" {
		parts = append(parts, "detail="+state.Detail)
	}
	return strings.Join(parts, "; ")
}

// printWizardPlanApprovalInstructions renders the stable fingerprint plus
// replay commands that preserve the caller's original wizard flags. It never
// evaluates or executes a shell fragment, and it does not claim an interactive
// run can be reproduced from CLI flags.
func printWizardPlanApprovalInstructions(plan piinstall.Plan, originalArgs []string, nonInteractive bool, workspaceRoot string) {
	fingerprint := plan.Fingerprint()
	fmt.Println("[wizard] plan-only preview complete; no runtime, config, artifact, or memory writes were made")
	fmt.Printf("  fingerprint: %s\n", fingerprint)
	fmt.Printf("  run any printed preview/approval command from the workspace root: %s\n", workspaceRoot)
	if !nonInteractive {
		fmt.Println("  the staged answers entered interactively are not representable as CLI flags, so no printed command can reproduce this exact plan.")
		fmt.Println("  to reproduce it, rerun `game-studio wizard`, choose the same options, and confirm the same fingerprint shown above.")
		return
	}
	fmt.Printf("  reproducible preview: %s\n", wizardReplayCommand(originalArgs, "--plan-only"))
	fmt.Printf("  approve this exact plan: %s\n", wizardReplayCommand(originalArgs, "--approve-plan", fingerprint))
}

// wizardReplayArgs reproduces the caller's original wizard arguments, dropping
// only the wizard-mode flags the printed command replaces (--plan-only,
// --metadata-only, --approve-plan). Every other option, including non-default
// profile/engine/workflow/depth and out-dir/final-artifact, is preserved
// verbatim so the replay cannot silently fall back to defaults.
func wizardReplayArgs(original []string, appendArgs ...string) []string {
	out := make([]string, 0, len(original)+len(appendArgs))
	for i := 0; i < len(original); i++ {
		arg := original[i]
		if arg == "--" {
			// RunWizard rejects positional arguments. Do not leave a terminal
			// delimiter in front of the approval option appended below.
			break
		}
		name, _, inlineValue := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		isOption := strings.HasPrefix(arg, "-")
		replaced := isOption && (name == "plan-only" || name == "metadata-only" || name == "approve-plan")
		if !replaced {
			out = append(out, arg)
		}
		// Consume a string option's value as data, even when it looks like a
		// mode flag. Keep this list aligned with RunWizard's string flags.
		if isOption && !inlineValue {
			switch name {
			case "profile", "complexity", "start", "use-mode", "engine-pack", "setup-depth", "out-dir", "final-artifact", "approve-plan":
				if i+1 < len(original) {
					i++
					if !replaced {
						out = append(out, original[i])
					}
				}
			}
		}
	}
	return append(out, appendArgs...)
}

func wizardReplayCommand(original []string, appendArgs ...string) string {
	args := wizardReplayArgs(original, appendArgs...)
	parts := make([]string, 0, len(args)+2)
	parts = append(parts, "game-studio", "wizard")
	for _, arg := range args {
		parts = append(parts, wizardShellArg(arg))
	}
	return strings.Join(parts, " ")
}

// wizardShellArg quotes an argument only when it contains characters a POSIX
// shell would interpret, keeping the command readable without opening an
// injection surface.
func wizardShellArg(arg string) string {
	if arg != "" && isShellSafeArg(arg) {
		return arg
	}
	return wizardShellQuote(arg)
}

func isShellSafeArg(arg string) bool {
	for _, r := range arg {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case strings.ContainsRune("-._/=:@+,", r):
		default:
			return false
		}
	}
	return true
}

// resolveWizardRuntimeConsent binds approval to the freshly prepared fingerprint.
// There is no boolean shortcut: a non-interactive run without a matching
// --approve-plan value never authorizes execution.
func resolveWizardRuntimeConsent(reader *bufio.Reader, nonInteractive bool, approvePlan string, approveProvided bool, plan piinstall.Plan) (piinstall.Consent, error) {
	fingerprint := plan.Fingerprint()
	if approveProvided {
		provided := strings.TrimSpace(approvePlan)
		if provided != fingerprint {
			return piinstall.Consent{}, fmt.Errorf("--approve-plan %q does not match the freshly prepared plan fingerprint %s; no files were written", provided, fingerprint)
		}
		return piinstall.Consent{Approved: true, PlanFingerprint: fingerprint}, nil
	}
	if nonInteractive {
		return piinstall.Consent{}, fmt.Errorf("non-interactive run requires --plan-only or --approve-plan %s; --non-interactive alone never authorizes installation", fingerprint)
	}
	fmt.Printf("[wizard][step-%d] Prerequisite approval\n", stepConfirmationNum)
	value, err := promptValue(reader, false, fmt.Sprintf("  Approve prerequisite plan %s? (yes/no)", fingerprint), "no")
	if err != nil {
		return piinstall.Consent{}, err
	}
	if !isYes(value) {
		return piinstall.Consent{PlanFingerprint: fingerprint}, nil
	}
	return piinstall.Consent{Approved: true, PlanFingerprint: fingerprint}, nil
}

func printWizardRuntimeReport(report piinstall.Report) {
	fmt.Println("[wizard] runtime prerequisite report")
	fmt.Printf("  failed: %t\n", report.Failed)
	if len(report.Entries) == 0 {
		fmt.Println("  entries: none")
	}
	for _, entry := range report.Entries {
		line := fmt.Sprintf("  - %s: %s", entry.Component, entry.Outcome)
		if entry.Version != "" {
			line += " version=" + entry.Version
		}
		if entry.Path != "" {
			line += " path=" + entry.Path
		}
		if entry.Detail != "" {
			line += " (" + entry.Detail + ")"
		}
		fmt.Println(line)
	}
	if len(report.Payload.Files) > 0 {
		fmt.Printf("  payload destination: %s\n", report.Payload.Destination)
		for _, file := range report.Payload.Files {
			fmt.Printf("    - %s: %s\n", file.RelativePath, file.Status)
		}
	}
	for _, note := range report.Notes {
		fmt.Printf("  note: %s\n", note)
	}
}

// printWizardRuntimeLaunch prints the usable launch command only for a complete,
// verified runtime setup. It is never called for a failed, partial, unverified,
// empty, or canceled report.
func printWizardRuntimeLaunch(report piinstall.Report) {
	if command := wizardLaunchCommand(report.Launch); command != "" {
		fmt.Printf("  launch: %s\n", command)
	}
}

// wizardShellQuote renders a POSIX single-quoted literal so a reported path can
// never break out of the printed command.
func wizardShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// wizardLaunchCommand reconstructs a usable direct POSIX command from the
// backend LaunchInfo. NodeBinDir is prepended first, then the Pi and npm
// directories, then any deduplicated PathAdditions; the user's PATH is preserved
// at the end. Engram/Godot binaries are exported as ENGRAM_BIN/GODOT_BIN. It
// returns "" when no Pi executable was reported, and it never mutates the
// process environment.
func wizardLaunchCommand(launch piinstall.LaunchInfo) string {
	piExecutable := strings.TrimSpace(launch.PiExecutable)
	if piExecutable == "" {
		return ""
	}

	dirs := make([]string, 0, 4+len(launch.PathAdditions))
	addDir := func(dir string) {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			return
		}
		for _, existing := range dirs {
			if existing == dir {
				return
			}
		}
		dirs = append(dirs, dir)
	}
	addDir(launch.NodeBinDir)
	addDir(filepath.Dir(piExecutable))
	if strings.TrimSpace(launch.NpmExecutable) != "" {
		addDir(filepath.Dir(launch.NpmExecutable))
	}
	for _, dir := range launch.PathAdditions {
		addDir(dir)
	}

	assignments := make([]string, 0, 3)
	if len(dirs) > 0 {
		assignments = append(assignments, "PATH="+wizardShellQuote(strings.Join(dirs, ":"))+`:"$PATH"`)
	}
	if strings.TrimSpace(launch.EngramBinary) != "" {
		assignments = append(assignments, "ENGRAM_BIN="+wizardShellQuote(launch.EngramBinary))
	}
	if strings.TrimSpace(launch.GodotBinary) != "" {
		assignments = append(assignments, "GODOT_BIN="+wizardShellQuote(launch.GodotBinary))
	}
	command := wizardShellQuote(piExecutable)
	if len(assignments) == 0 {
		return command
	}
	return strings.Join(assignments, " ") + " " + command
}

func wizardReportStatus(report piinstall.Report) string {
	if report.Failed {
		return "failed"
	}
	for _, entry := range report.Entries {
		switch entry.Outcome {
		case piinstall.OutcomeUnverified, piinstall.OutcomeFailed, piinstall.OutcomeBlocked, piinstall.OutcomeDeclined:
			return "partial"
		}
	}
	return "completed"
}

// wizardReportIncompleteReason returns "" only when the report is a complete,
// fully verified success for every component the plan requires. A failed,
// unverified, blocked, declined, or missing entry, or a missing required launch
// path, is incomplete. It never treats an empty or facade success as coverage.
func wizardReportIncompleteReason(plan piinstall.Plan, report piinstall.Report) string {
	if report.Failed {
		return "the backend reported a failed step"
	}

	entries := map[piinstall.Component]piinstall.Entry{}
	for _, entry := range report.Entries {
		if _, exists := entries[entry.Component]; !exists {
			entries[entry.Component] = entry
		}
	}

	problems := make([]string, 0)
	for _, entry := range report.Entries {
		switch entry.Outcome {
		case piinstall.OutcomeInstalled, piinstall.OutcomeReused:
		default:
			problems = append(problems, fmt.Sprintf("%s is %s", entry.Component, entry.Outcome))
		}
	}
	for _, step := range plan.Steps {
		if step.Action == piinstall.ActionBlocked {
			problems = append(problems, fmt.Sprintf("%s is blocked", step.Component))
			continue
		}
		entry, ok := entries[step.Component]
		if !ok {
			problems = append(problems, fmt.Sprintf("%s has no report entry", step.Component))
			continue
		}
		switch entry.Outcome {
		case piinstall.OutcomeInstalled, piinstall.OutcomeReused:
		default:
			problems = append(problems, fmt.Sprintf("%s is %s", step.Component, entry.Outcome))
		}
	}
	if strings.TrimSpace(report.Launch.PiExecutable) == "" {
		problems = append(problems, "the Pi executable was not reported")
	}
	if strings.TrimSpace(report.Launch.NodeBinDir) == "" {
		problems = append(problems, "the Node bin directory was not reported")
	}
	if strings.TrimSpace(report.Launch.NpmExecutable) == "" {
		problems = append(problems, "the npm executable was not reported")
	}
	if strings.TrimSpace(report.Launch.EngramBinary) == "" {
		problems = append(problems, "the Engram binary was not reported")
	}
	if planHasComponent(plan, piinstall.ComponentGodot) && strings.TrimSpace(report.Launch.GodotBinary) == "" {
		problems = append(problems, "the Godot binary was not reported")
	}
	if len(problems) > 0 {
		return "prerequisite report is incomplete: " + strings.Join(problems, "; ")
	}
	return ""
}

func planHasComponent(plan piinstall.Plan, component piinstall.Component) bool {
	for _, step := range plan.Steps {
		if step.Component == component {
			return true
		}
	}
	return false
}

func canonicalToolID(registry *toolcheck.Registry, id string) string {
	if registry != nil {
		if canonical := registry.CanonicalID(id); canonical != "" {
			return canonical
		}
	}
	return strings.ToLower(strings.TrimSpace(id))
}

func withoutCanonicalTool(results []toolcheck.Result, registry *toolcheck.Registry, toolID string) []toolcheck.Result {
	out := make([]toolcheck.Result, 0, len(results))
	for _, result := range results {
		if canonicalToolID(registry, result.ToolID) == toolID {
			continue
		}
		out = append(out, result)
	}
	return out
}

func hasCanonicalTool(results []toolcheck.Result, registry *toolcheck.Registry, toolID string) bool {
	for _, result := range results {
		if canonicalToolID(registry, result.ToolID) == toolID {
			return true
		}
	}
	return false
}

func managedGodotResult(godotPath string) toolcheck.Result {
	return toolcheck.Result{
		ToolID:    toolcheck.GodotToolID,
		ToolName:  "Godot",
		Status:    toolcheck.StatusSuccess,
		Severity:  toolcheck.SeverityRequired,
		Required:  true,
		Reason:    "managed Godot verified at " + godotPath,
		Attempted: godotPath,
	}
}

// reconcileRuntimeVerifiedGodot replaces only the ambient Godot preflight
// availability with the backend-verified managed Godot when Execute reported
// Godot installed/reused and a nonempty Godot binary. Every unrelated blocker
// and warning is preserved, and a design/visual mode that never planned Godot
// is left untouched.
func reconcileRuntimeVerifiedGodot(state *wizardState, registry *toolcheck.Registry, report piinstall.Report, results []toolcheck.Result, baseNextSteps []string) bool {
	if !wizardReportComponentVerified(report, piinstall.ComponentGodot) {
		return false
	}
	godotPath := strings.TrimSpace(report.Launch.GodotBinary)
	if godotPath == "" {
		return false
	}
	// Only replace an existing ambient Godot result; never invent a Godot
	// preflight entry that was not part of the prior evidence.
	if !hasCanonicalTool(results, registry, toolcheck.GodotToolID) {
		return false
	}

	reconciled := withoutCanonicalTool(results, registry, toolcheck.GodotToolID)
	reconciled = append(reconciled, managedGodotResult(godotPath))

	state.EnvWarnings = withoutCanonicalTool(state.EnvWarnings, registry, toolcheck.GodotToolID)
	state.EnvBlockers = withoutCanonicalTool(state.EnvBlockers, registry, toolcheck.GodotToolID)
	state.ValidationScope = wizardValidationScope(*state, registry, reconciled)
	state.PreflightResult = wizardPreflightResult(*state, reconciled)
	state.DeepValidationRequired = deepValidationRequired(*state)
	state.NextSteps = appendEnvironmentNextSteps(*state, append([]string{}, baseNextSteps...))
	return true
}

// wizardReportComponentVerified reports whether the backend observed a component
// as installed or reused. Only these outcomes may be treated as available.
func wizardReportComponentVerified(report piinstall.Report, component piinstall.Component) bool {
	for _, entry := range report.Entries {
		if entry.Component != component {
			continue
		}
		if entry.Outcome == piinstall.OutcomeInstalled || entry.Outcome == piinstall.OutcomeReused {
			return true
		}
	}
	return false
}

func wizardRuntimeNextSteps(report piinstall.Report) []string {
	steps := make([]string, 0, 3)
	if wizardReportComponentVerified(report, piinstall.ComponentEngramCore) && strings.TrimSpace(report.Launch.EngramBinary) != "" {
		steps = append(steps, "Engram core is available through ENGRAM_BIN="+report.Launch.EngramBinary+" for memory writes; the ambient PATH was not modified.")
	}
	if wizardReportComponentVerified(report, piinstall.ComponentGodot) && strings.TrimSpace(report.Launch.GodotBinary) != "" {
		steps = append(steps, "Godot is available at "+report.Launch.GodotBinary+" via GODOT_BIN; no gameplay was executed.")
	}
	if wizardReportComponentVerified(report, piinstall.ComponentPi) {
		steps = append(steps, "Relaunch Pi from the reported command so the managed Node/npm/Pi directories stay ahead of the ambient PATH.")
	}
	return steps
}

// wizardRuntimeSetupState is the wizard-owned distillation of the backend plan
// and report. It records only observed values so the final artifact never
// over-claims readiness.
type wizardRuntimeSetupState struct {
	PlanFingerprint string
	PlanStatus      string
	Executed        bool
	ReportStatus    string
	Failed          bool
	Entries         []wizardRuntimeEntry
	Launch          wizardLaunch
	Notes           []string
	PayloadFiles    []wizardRuntimePayloadFile
}

type wizardRuntimeEntry struct {
	Component string
	Outcome   string
	Detail    string
	Path      string
	Version   string
}

type wizardRuntimePayloadFile struct {
	RelativePath string
	Status       string
}

type wizardLaunch struct {
	PiExecutable  string
	PiPrefix      string
	NodeBinDir    string
	NpmExecutable string
	EngramBinary  string
	GodotBinary   string
	PathAdditions []string
}

func wizardRuntimeSetupFromReport(plan piinstall.Plan, report piinstall.Report) *wizardRuntimeSetupState {
	setup := &wizardRuntimeSetupState{
		PlanFingerprint: plan.Fingerprint(),
		PlanStatus:      wizardPlanStatus(plan),
		Executed:        true,
		ReportStatus:    wizardReportStatus(report),
		Failed:          report.Failed,
		Notes:           append([]string{}, report.Notes...),
		Launch: wizardLaunch{
			PiExecutable:  report.Launch.PiExecutable,
			PiPrefix:      report.Launch.PiPrefix,
			NodeBinDir:    report.Launch.NodeBinDir,
			NpmExecutable: report.Launch.NpmExecutable,
			EngramBinary:  report.Launch.EngramBinary,
			GodotBinary:   report.Launch.GodotBinary,
			PathAdditions: append([]string{}, report.Launch.PathAdditions...),
		},
	}
	for _, entry := range report.Entries {
		setup.Entries = append(setup.Entries, wizardRuntimeEntry{
			Component: string(entry.Component),
			Outcome:   string(entry.Outcome),
			Detail:    entry.Detail,
			Path:      entry.Path,
			Version:   entry.Version,
		})
	}
	for _, file := range report.Payload.Files {
		setup.PayloadFiles = append(setup.PayloadFiles, wizardRuntimePayloadFile{RelativePath: file.RelativePath, Status: file.Status})
	}
	return setup
}

// wizardRuntimeSetupArtifact is the persisted form of wizardRuntimeSetupState.
// Metadata-only runs persist an explicit skipped shape (plan_status
// "skipped_metadata_only", executed false) instead of omitting the field, so the
// artifact never looks like a runtime readiness claim.
type wizardRuntimeSetupArtifact struct {
	PlanFingerprint string                             `json:"plan_fingerprint"`
	PlanStatus      string                             `json:"plan_status"`
	Executed        bool                               `json:"executed"`
	ReportStatus    string                             `json:"report_status"`
	Failed          bool                               `json:"failed"`
	Entries         []wizardRuntimeEntryArtifact       `json:"entries"`
	Launch          wizardLaunchArtifact               `json:"launch"`
	Notes           []string                           `json:"notes,omitempty"`
	PayloadFiles    []wizardRuntimePayloadFileArtifact `json:"payload_files,omitempty"`
}

type wizardRuntimeEntryArtifact struct {
	Component string `json:"component"`
	Outcome   string `json:"outcome"`
	Detail    string `json:"detail,omitempty"`
	Path      string `json:"path,omitempty"`
	Version   string `json:"version,omitempty"`
}

type wizardRuntimePayloadFileArtifact struct {
	RelativePath string `json:"relative_path"`
	Status       string `json:"status"`
}

type wizardLaunchArtifact struct {
	PiExecutable  string   `json:"pi_executable,omitempty"`
	PiPrefix      string   `json:"pi_prefix,omitempty"`
	NodeBinDir    string   `json:"node_bin_dir,omitempty"`
	NpmExecutable string   `json:"npm_executable,omitempty"`
	EngramBinary  string   `json:"engram_binary,omitempty"`
	GodotBinary   string   `json:"godot_binary,omitempty"`
	PathAdditions []string `json:"path_additions,omitempty"`
}

func wizardRuntimeSetupPayload(setup *wizardRuntimeSetupState) *wizardRuntimeSetupArtifact {
	if setup == nil {
		return &wizardRuntimeSetupArtifact{
			PlanStatus:   "skipped_metadata_only",
			Executed:     false,
			ReportStatus: "skipped",
		}
	}
	payload := &wizardRuntimeSetupArtifact{
		PlanFingerprint: setup.PlanFingerprint,
		PlanStatus:      setup.PlanStatus,
		Executed:        setup.Executed,
		ReportStatus:    setup.ReportStatus,
		Failed:          setup.Failed,
		Notes:           append([]string{}, setup.Notes...),
		Launch: wizardLaunchArtifact{
			PiExecutable:  setup.Launch.PiExecutable,
			PiPrefix:      setup.Launch.PiPrefix,
			NodeBinDir:    setup.Launch.NodeBinDir,
			NpmExecutable: setup.Launch.NpmExecutable,
			EngramBinary:  setup.Launch.EngramBinary,
			GodotBinary:   setup.Launch.GodotBinary,
			PathAdditions: append([]string{}, setup.Launch.PathAdditions...),
		},
	}
	for _, entry := range setup.Entries {
		payload.Entries = append(payload.Entries, wizardRuntimeEntryArtifact{
			Component: entry.Component,
			Outcome:   entry.Outcome,
			Detail:    entry.Detail,
			Path:      entry.Path,
			Version:   entry.Version,
		})
	}
	for _, file := range setup.PayloadFiles {
		payload.PayloadFiles = append(payload.PayloadFiles, wizardRuntimePayloadFileArtifact{RelativePath: file.RelativePath, Status: file.Status})
	}
	return payload
}

// wizardMemoryOutcomeArtifact is the small explicit record of the Engram memory
// outcome. It never carries raw CLI output and never claims a save that did not
// happen.
type wizardMemoryOutcomeArtifact struct {
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

func wizardMemoryOutcomePayload(state wizardState) wizardMemoryOutcomeArtifact {
	status := strings.TrimSpace(state.MemoryStatus)
	if status == "" {
		status = wizardMemoryPending
	}
	return wizardMemoryOutcomeArtifact{Status: status, Detail: state.MemoryDetail}
}

// wizardPersistGeneration performs the configured Engram write-through and
// records the outcome. A disabled wiring is recorded as explicitly skipped. A
// failure is recorded as failed (or canceled when the caller context is gone)
// and returned so the wizard never reports a completed success.
func wizardPersistGeneration(ctx context.Context, placeholder persistence.Placeholder, state *wizardState, writeThrough persistence.GenerationWriteThrough) error {
	if !placeholder.Wiring().Engram.Enabled {
		state.MemoryStatus = wizardMemoryDisabled
		state.MemoryDetail = "Engram persistence is disabled; no memory save was attempted"
		return nil
	}
	state.MemoryStatus = wizardMemoryPending
	state.MemoryDetail = ""
	if err := placeholder.WriteGenerationContext(ctx, writeThrough); err != nil {
		if ctx != nil && ctx.Err() != nil {
			state.MemoryStatus = wizardMemoryCanceled
			state.MemoryDetail = "the Engram memory write-through was canceled"
		} else {
			state.MemoryStatus = wizardMemoryFailed
			state.MemoryDetail = "the Engram memory write-through failed; local artifacts remain but the memory outcome is not complete"
		}
		return fmt.Errorf("%w (memory status %s)", errWizardMemoryFailed, state.MemoryStatus)
	}
	state.MemoryStatus = wizardMemorySaved
	state.MemoryDetail = ""
	return nil
}

// wizardManagedDestination is one wizard-owned output role in the
// managed-destination inventory.
type wizardManagedDestination struct {
	role string
	path string
	dir  bool
}

// wizardValidateManagedDestinations builds the complete managed-destination
// inventory (pack profile/summary/pattern/config, wizard profile, final
// artifact, workspace config, openspec config) and refuses distinct-role
// aliasing, file/ancestor collisions, existing symlink or non-regular leaves,
// and writes into the protected .git/.pi control namespaces or a managed
// install root nested in the workspace. It performs no writes.
func wizardValidateManagedDestinations(workspaceRoot, outDir, finalArtifactPath, profileArtifactTarget, installRoot string) error {
	physicalWorkspace, err := resolvePhysicalPath(workspaceRoot)
	if err != nil {
		return fmt.Errorf("resolve workspace root: %w", err)
	}

	packPaths, err := resolveOutputPaths(templates.EngineGodot, "", outDir, "flat")
	if err != nil {
		return fmt.Errorf("resolve pack artifact destinations: %w", err)
	}

	// Preserve lexical leaf identity for EVERY file role, not just profiles.
	// Physical resolution is for containment and aliases, never type policy.
	destinations := []wizardManagedDestination{
		{role: "out-dir", path: outDir, dir: true},
		{role: "pack-profile", path: packPaths.profile},
		{role: "pack-summary", path: packPaths.summary},
		{role: "pack-pattern", path: packPaths.pattern},
		{role: "pack-config", path: packPaths.config},
		{role: "wizard-profile", path: profileArtifactTarget},
		{role: "final-artifact", path: finalArtifactPath},
		{role: "workspace-config", path: workspaceConfigRelativePath},
		{role: "openspec-config", path: filepath.Join("openspec", "config.yaml")},
	}
	for i := range destinations {
		lexical := destinations[i].path
		if !filepath.IsAbs(lexical) {
			lexical = filepath.Join(workspaceRoot, lexical)
		}
		if !destinations[i].dir {
			if isPathWithin(lexical, filepath.Join(workspaceRoot, ".git")) || isPathWithin(lexical, filepath.Join(workspaceRoot, ".pi")) {
				return fmt.Errorf("refusing managed artifact %s (%s): it is inside a protected control namespace", lexical, destinations[i].role)
			}
			leaf := wizardManagedDestination{role: destinations[i].role, path: lexical}
			if err := wizardRejectManagedLeafTypes([]wizardManagedDestination{leaf}); err != nil {
				return err
			}
		}
		physical, err := resolveWizardSafePath(workspaceRoot, &lexical)
		if err != nil {
			return fmt.Errorf("validate managed destination %s: %w", destinations[i].role, err)
		}
		destinations[i].path = physical
	}

	if err := wizardRejectManagedLeafTypes(destinations); err != nil {
		return err
	}
	if err := wizardRejectManagedRoleCollisions(destinations); err != nil {
		return err
	}

	gitDir, err := resolvePhysicalPath(filepath.Join(physicalWorkspace, ".git"))
	if err != nil {
		return fmt.Errorf("resolve protected Git namespace: %w", err)
	}
	piDir, err := resolvePhysicalPath(filepath.Join(physicalWorkspace, ".pi"))
	if err != nil {
		return fmt.Errorf("resolve protected Pi namespace: %w", err)
	}
	for _, destination := range destinations {
		if destination.dir {
			continue
		}
		if isPathWithin(destination.path, gitDir) || isPathWithin(destination.path, piDir) {
			return fmt.Errorf("refusing managed artifact %s (%s): it is inside a protected control namespace", destination.path, destination.role)
		}
	}

	if strings.TrimSpace(installRoot) != "" {
		physicalInstallRoot, err := resolvePhysicalPath(installRoot)
		if err != nil {
			return fmt.Errorf("resolve managed install root: %w", err)
		}
		if isPathWithin(physicalInstallRoot, physicalWorkspace) {
			for _, destination := range destinations {
				if isPathWithin(destination.path, physicalInstallRoot) {
					return fmt.Errorf("refusing managed artifact %s (%s): it overlaps the managed install root %s", destination.path, destination.role, physicalInstallRoot)
				}
				if !destination.dir && isPathAncestor(destination.path, physicalInstallRoot) {
					return fmt.Errorf("refusing managed file %s (%s): it is an ancestor of the managed install root %s", destination.path, destination.role, physicalInstallRoot)
				}
			}
		}
	}

	return nil
}

func wizardRejectManagedLeafTypes(destinations []wizardManagedDestination) error {
	for _, destination := range destinations {
		info, err := os.Lstat(destination.path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("inspect managed destination %s: %w", destination.path, err)
		}
		if destination.dir {
			if !info.IsDir() {
				return fmt.Errorf("refusing managed directory destination %s (%s): existing path is not a directory", destination.path, destination.role)
			}
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing managed artifact %s (%s): target is a symlink", destination.path, destination.role)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("refusing managed artifact %s (%s): target is not a regular file", destination.path, destination.role)
		}
	}
	return nil
}

func wizardRejectManagedRoleCollisions(destinations []wizardManagedDestination) error {
	for i := 0; i < len(destinations); i++ {
		for j := i + 1; j < len(destinations); j++ {
			a, b := destinations[i], destinations[j]
			if a.path == b.path {
				return fmt.Errorf("managed destinations %s and %s resolve to the same path %s", a.role, b.role, a.path)
			}
			if !a.dir && isPathAncestor(a.path, b.path) {
				return fmt.Errorf("managed file destination %s (%s) is an ancestor of %s (%s)", a.path, a.role, b.path, b.role)
			}
			if !b.dir && isPathAncestor(b.path, a.path) {
				return fmt.Errorf("managed file destination %s (%s) is an ancestor of %s (%s)", b.path, b.role, a.path, a.role)
			}
		}
	}

	type existingFile struct {
		role string
		info os.FileInfo
	}
	seen := make([]existingFile, 0, len(destinations))
	for _, destination := range destinations {
		if destination.dir {
			continue
		}
		info, err := os.Stat(destination.path)
		if err != nil {
			continue
		}
		for _, existing := range seen {
			if os.SameFile(existing.info, info) {
				return fmt.Errorf("managed destinations %s and %s resolve to the same file", existing.role, destination.role)
			}
		}
		seen = append(seen, existingFile{role: destination.role, info: info})
	}
	return nil
}

func isPathAncestor(ancestor, candidate string) bool {
	return strings.HasPrefix(candidate, ancestor+string(filepath.Separator))
}

func isPathWithin(candidate, root string) bool {
	return candidate == root || isPathAncestor(root, candidate)
}
