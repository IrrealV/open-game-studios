package commands

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"open-game-studios/internal/assets"
	"open-game-studios/internal/integrations"
	"open-game-studios/internal/opencode"
	"open-game-studios/internal/persistence"
	"open-game-studios/internal/piinstall"
	"open-game-studios/internal/routing"
	"open-game-studios/internal/templates"
	"open-game-studios/internal/toolcheck"
	"open-game-studios/internal/workdoc"
	"open-game-studios/internal/workflows/coregame"
	"open-game-studios/internal/workflows/modelrouting"
)

type WizardInput struct {
	Args                       []string
	Registry                   *templates.Registry
	Persistence                persistence.Placeholder
	EnvChecker                 EnvironmentChecker
	ToolRegistry               *toolcheck.Registry
	OptionalIntegrationCatalog integrations.Catalog

	// Installer overrides the runtime prerequisite backend. Nil uses the
	// production piinstall adapter.
	Installer WizardInstaller
	// Context bounds runtime Prepare/Execute and every write boundary. Nil makes
	// the wizard install an os.Interrupt-cancelled context so Ctrl-C stops the
	// run instead of leaving a false success.
	Context context.Context
	// Config overrides the runtime installer configuration for tests. The wizard
	// still forces WorkspaceDir to the physically validated workspace and derives
	// GodotRequired from the active workflow.
	Config *piinstall.Config
	// IsTerminal reports whether a file is an interactive terminal. Nil uses the
	// production charmbracelet/x/term check. Tests inject a deterministic seam
	// instead of swapping stdio.
	IsTerminal func(*os.File) bool
	// TuiProgram runs the interactive plan-only preview program. Nil uses the
	// production alt-screen Bubble Tea program.
	TuiProgram wizardTuiProgram
}

type wizardState struct {
	UseMode                string
	EnginePack             string
	SetupDepth             string
	PresetSource           string
	RoutingPresetSource    string
	Complexity             string
	StartingPoint          string
	ProfileName            string
	Connectors             []string
	Packs                  []string
	Tools                  []string
	Providers              []opencode.ConnectedProvider
	ProviderWarning        string
	EnvWarnings            []toolcheck.Result
	EnvBlockers            []toolcheck.Result
	ValidationScope        wizardValidationScopeArtifact
	PreflightResult        wizardPreflightResultArtifact
	DeepValidationRequired []string
	Snapshot               opencode.ModelSnapshot
	Policy                 routing.Policy
	MCPRequired            []string
	MCPOptional            []string
	OptionalIntegrations   []integrations.OptionalIntegrationSelection
	AcceptedConsents       []string
	RejectedConsents       []string
	DeferredConsents       []string
	AssetPipeline          wizardAssetPipelineState
	DeferredLanes          []wizardDeferredLane
	NextSteps              []string
	Confirmed              bool
	RuntimeSetup           *wizardRuntimeSetupState
	MemoryStatus           string
	MemoryDetail           string

	ProfileArtifactPath string
	FinalArtifactPath   string
	PackArtifacts       []string
	SmokePaths          outputPaths
}

type wizardFinalArtifact struct {
	Profile                   string                             `json:"profile"`
	UseMode                   string                             `json:"use_mode"`
	EnginePack                string                             `json:"engine_pack"`
	EnginePackStatus          string                             `json:"engine_pack_status"`
	GenerationDeferred        bool                               `json:"generation_deferred"`
	ActiveGenerationSupported bool                               `json:"active_generation_supported"`
	SetupDepth                string                             `json:"setup_depth"`
	Complexity                string                             `json:"complexity"`
	StartingPoint             string                             `json:"starting_point"`
	Connectors                []string                           `json:"connectors"`
	Packs                     []string                           `json:"packs"`
	Tools                     []string                           `json:"tools"`
	Providers                 []string                           `json:"providers"`
	ProviderNotice            string                             `json:"provider_notice,omitempty"`
	EnvWarnings               []string                           `json:"env_warnings,omitempty"`
	EnvBlockers               []string                           `json:"env_blockers,omitempty"`
	SelectedTools             []string                           `json:"selected_tools"`
	ValidatedTools            []string                           `json:"validated_tools"`
	SkippedTools              []string                           `json:"skipped_tools"`
	DeferredTools             []string                           `json:"deferred_tools"`
	ValidationScope           wizardValidationScopeArtifact      `json:"validation_scope"`
	PreflightResult           wizardPreflightResultArtifact      `json:"preflight_result"`
	GenerationReadiness       wizardGenerationReadinessArtifact  `json:"generation_readiness"`
	ProfileGenerationAllowed  bool                               `json:"profile_generation_allowed"`
	MetadataOnlyGeneration    bool                               `json:"metadata_only_generation"`
	EngineWorkflowReady       bool                               `json:"engine_workflow_ready"`
	BlockedWorkflows          []string                           `json:"blocked_workflows,omitempty"`
	BlockingReason            string                             `json:"blocking_reason,omitempty"`
	CanContinueMetadataOnly   bool                               `json:"can_continue_metadata_only"`
	DeepValidationRequired    []string                           `json:"deep_validation_required"`
	CapturedAt                string                             `json:"captured_at"`
	SetupPreset               string                             `json:"setup_preset"`
	PresetSource              string                             `json:"preset_source"`
	TaxonomyVersion           string                             `json:"taxonomy_version"`
	SelectedCore              []wizardTaxonomyItem               `json:"selected_core"`
	Preset                    string                             `json:"preset"`
	TierDefaults              map[string]string                  `json:"tier_defaults"`
	PhaseOverrides            map[string]string                  `json:"phase_overrides"`
	RoleOverrides             map[string]string                  `json:"role_overrides"`
	LegacyRouting             bool                               `json:"legacy_routing"`
	LegacyRoutingBoundary     string                             `json:"legacy_routing_boundary"`
	NoModelExecution          bool                               `json:"no_model_execution"`
	MCP                       map[string][]string                `json:"mcp"`
	OptionalIntegrations      wizardOptionalIntegrationsArtifact `json:"optional_integrations"`
	ConsentSensitive          []wizardConsentSensitiveArtifact   `json:"consent_sensitive_integrations"`
	AcceptedConsents          []string                           `json:"accepted_consents"`
	RejectedConsents          []string                           `json:"rejected_consents"`
	DeferredConsents          []string                           `json:"deferred_consents"`
	WorkflowAdapters          []wizardTaxonomyIntegration        `json:"workflow_adapters"`
	VisualWorkflowLanes       []wizardVisualWorkflowLane         `json:"visual_workflow_lanes"`
	AssetPipeline             wizardAssetPipelineArtifact        `json:"asset_pipeline"`
	VisualWorkflow            wizardVisualWorkflowArtifact       `json:"visual_workflow"`
	CoreGameWorkflow          wizardCoreGameWorkflowArtifact     `json:"core_game_workflow"`
	ModelRouting              wizardModelRoutingArtifact         `json:"model_routing"`
	ModelRoutingVersion       string                             `json:"model_routing_version"`
	RoutingPreset             string                             `json:"routing_preset"`
	RoutingPresetSource       string                             `json:"routing_preset_source"`
	RoutingValidationResult   modelrouting.ValidationResult      `json:"routing_validation_result"`
	DeferredLanes             []wizardDeferredLane               `json:"deferred_lanes"`
	NextSteps                 []string                           `json:"next_steps"`
	RuntimeSetup              *wizardRuntimeSetupArtifact        `json:"runtime_setup,omitempty"`
	MemoryOutcome             wizardMemoryOutcomeArtifact        `json:"memory_outcome"`
	GeneratedAt               string                             `json:"generated_at"`
}

type wizardValidationScopeArtifact struct {
	Preset       string                         `json:"preset"`
	Mode         string                         `json:"mode"`
	CanContinue  bool                           `json:"can_continue"`
	Tools        []wizardToolValidationArtifact `json:"tools"`
	SummaryLines []string                       `json:"summary_lines"`
}

type wizardToolValidationArtifact struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Severity string `json:"severity"`
	Reason   string `json:"reason,omitempty"`
}

type wizardPreflightResultArtifact struct {
	Status       string   `json:"status"`
	CanContinue  bool     `json:"can_continue"`
	Warnings     []string `json:"warnings,omitempty"`
	Blockers     []string `json:"blockers,omitempty"`
	Validated    []string `json:"validated"`
	NotValidated []string `json:"not_validated"`
}

type wizardGenerationReadinessArtifact struct {
	ProfileGenerationAllowed bool     `json:"profile_generation_allowed"`
	MetadataOnlyGeneration   bool     `json:"metadata_only_generation"`
	EngineWorkflowReady      bool     `json:"engine_workflow_ready"`
	BlockedWorkflows         []string `json:"blocked_workflows,omitempty"`
	BlockingReason           string   `json:"blocking_reason,omitempty"`
	CanContinueMetadataOnly  bool     `json:"can_continue_metadata_only"`
	SummaryLines             []string `json:"summary_lines"`
}

type wizardDeferredLane struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Note   string `json:"note"`
}

type wizardOptionalIntegrationsArtifact struct {
	Selected        []integrations.OptionalIntegrationSelection `json:"selected"`
	UnknownOptional []wizardTaxonomyIntegration                 `json:"unknown_optional,omitempty"`
}

type wizardTaxonomyItem struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Category string `json:"category"`
	Status   string `json:"status"`
	Note     string `json:"note,omitempty"`
}

type wizardTaxonomyIntegration struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Category string `json:"category"`
	Status   string `json:"status"`
	Boundary string `json:"boundary"`
}

type wizardEnginePackMetadata struct {
	ID                        string
	Label                     string
	Status                    string
	Note                      string
	GenerationDeferred        bool
	ActiveGenerationSupported bool
}

type wizardConsentSensitiveArtifact struct {
	ID           string   `json:"id"`
	Label        string   `json:"label"`
	Category     string   `json:"category"`
	Requirements []string `json:"requirements"`
	Status       string   `json:"status"`
	Boundary     string   `json:"boundary"`
}

type wizardVisualWorkflowLane struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Category string `json:"category"`
	Status   string `json:"status"`
	Boundary string `json:"boundary"`
}

const wizardTaxonomyVersion = "wizard-option-taxonomy/v1"

type wizardAssetPipelineState struct {
	Intent           string
	SelectedAdapters []assets.AdapterCapability
	DeferredRequests []string
}

type wizardAssetPipelineArtifact struct {
	Intent            string                     `json:"intent"`
	ContractVersion   string                     `json:"contract_version"`
	Boundary          string                     `json:"boundary"`
	SelectedAdapters  []assets.AdapterCapability `json:"selected_adapters"`
	FutureScope       []string                   `json:"future_scope"`
	DeferredRequests  []string                   `json:"deferred_requests,omitempty"`
	NoExecutionClaims map[string]bool            `json:"no_execution_claims"`
}

type wizardVisualWorkflowArtifact struct {
	Intent            string                         `json:"intent"`
	ContractVersion   string                         `json:"contract_version"`
	Boundary          string                         `json:"boundary"`
	Paths             assets.VisualWorkflowPaths     `json:"paths"`
	ArtBible          assets.VisualArtBibleContract  `json:"art_bible"`
	AssetSpec         assets.VisualAssetSpecContract `json:"asset_spec"`
	Readiness         assets.VisualReadinessContract `json:"asset_readiness"`
	Audit             assets.VisualAuditContract     `json:"asset_audit"`
	Gates             []assets.WorkflowGate          `json:"workflow_gates"`
	GuidanceRoutes    []string                       `json:"guidance_routes"`
	NoExecutionClaims map[string]bool                `json:"no_execution_claims"`
}

type wizardCoreGameWorkflowArtifact struct {
	ContractVersion       string                          `json:"contract_version"`
	Boundary              string                          `json:"boundary"`
	SupportedModes        []string                        `json:"supported_modes"`
	ModeContracts         []coregame.ModeContract         `json:"mode_contracts"`
	PhaseIDs              []coregame.PhaseID              `json:"phase_ids"`
	NarrativeModes        []coregame.NarrativeMode        `json:"narrative_modes"`
	RepairClassifications []coregame.RepairClassification `json:"repair_classifications"`
	ApprovalPolicy        coregame.ApprovalPolicy         `json:"approval_policy"`
	BriefContracts        []coregame.BriefContract        `json:"brief_contracts"`
	MarkdownTemplates     []coregame.MarkdownTemplate     `json:"markdown_templates"`
	RepairHandoffFlow     []string                        `json:"repair_change_handoff_flow"`
	DownstreamReferences  []coregame.DownstreamReference  `json:"downstream_references"`
	NoExecutionClaims     map[string]bool                 `json:"no_execution_claims"`
}

type wizardModelRoutingArtifact struct {
	Version                          string                            `json:"version"`
	Boundary                         string                            `json:"routing_boundary"`
	CoreWorkflowVersion              string                            `json:"core_game_workflow_version"`
	SetupPreset                      string                            `json:"setup_preset"`
	PresetSource                     string                            `json:"preset_source"`
	RoutingPreset                    string                            `json:"routing_preset"`
	RoutingPresetSource              string                            `json:"routing_preset_source"`
	Capabilities                     []modelrouting.Capability         `json:"capabilities"`
	PhaseRoutes                      []modelrouting.PhaseRoute         `json:"phase_routes"`
	ProviderModelBindings            []modelrouting.ProviderBinding    `json:"provider_model_bindings"`
	CapabilityOverrides              []modelrouting.CapabilityOverride `json:"capability_overrides"`
	PhaseOverrides                   []modelrouting.PhaseOverride      `json:"phase_overrides"`
	OverrideStatus                   string                            `json:"override_status"`
	ValidationResult                 string                            `json:"validation_result"`
	UnconfiguredRequiredCapabilities []modelrouting.CapabilityID       `json:"unconfigured_required_capabilities"`
	UnconfiguredOptionalCapabilities []modelrouting.CapabilityID       `json:"unconfigured_optional_capabilities"`
	OptionalCapabilities             []modelrouting.CapabilityID       `json:"optional_capabilities"`
	FutureCapabilities               []modelrouting.CapabilityID       `json:"future_capabilities"`
	DeferredCapabilities             []modelrouting.CapabilityID       `json:"deferred_capabilities"`
	RoutingValidationResult          modelrouting.ValidationResult     `json:"routing_validation_result"`
	NoExecutionClaims                map[string]bool                   `json:"no_execution_claims"`
}

const legacyRoutingBoundary = "compatibility metadata only; model-routing/v1 is authoritative"

const (
	stepWelcomeNum      = 0
	stepUseModeNum      = 1
	stepEnginePackNum   = 2
	stepSetupDepthNum   = 3
	stepProfileNameNum  = 4
	stepResourcesNum    = 5
	stepOptionalsNum    = 6
	stepVisualLaneNum   = 7
	stepSummaryNum      = 8
	stepConfirmationNum = 9
	stepGenerationNum   = 10
	stepSmokeNum        = 11
	stepNextStepsNum    = 12
	defaultFinalOutput  = ".game-studio/generated/wizard/final.artifact.json"

	// Deprecated deep-setup step numbers retained for direct unit tests of legacy helpers.
	stepProvidersNum     = stepVisualLaneNum
	stepBasePresetNum    = stepVisualLaneNum
	stepTierRoutingNum   = stepVisualLaneNum
	stepPhaseRoutingNum  = stepVisualLaneNum
	stepRoleOverridesNum = stepVisualLaneNum
	stepMCPsNum          = stepOptionalsNum
)

func RunWizard(input WizardInput) error {
	fs := flag.NewFlagSet("wizard", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	defaultProfile := fs.String("profile", "Game-Studio", "initial profile name")
	defaultComplexity := fs.String("complexity", "", "deprecated: maps simple|advanced|expert to setup depth")
	defaultStart := fs.String("start", "", "deprecated: maps scratch|existing|import to use mode")
	defaultUseMode := fs.String("use-mode", "create_from_scratch", "initial use mode")
	defaultEnginePack := fs.String("engine-pack", "godot-core", "initial engine pack")
	defaultSetupDepth := fs.String("setup-depth", "recommended", "initial setup depth (minimal|recommended|full|custom)")
	outDir := fs.String("out-dir", "profiles/game-studio/generated", "pack artifact output directory")
	finalArtifactPath := fs.String("final-artifact", defaultFinalOutput, "wizard final artifact output path")
	nonInteractive := fs.Bool("non-interactive", false, "run wizard with defaults without prompts")
	planOnly := fs.Bool("plan-only", false, "render the read-only runtime prerequisite plan and fingerprint without executing it")
	approvePlan := fs.String("approve-plan", "", "non-interactive approval of the exact freshly prepared plan fingerprint")
	metadataOnly := fs.Bool("metadata-only", false, "run the legacy artifact-only flow without preparing or executing prerequisites")
	if err := fs.Parse(input.Args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return fmt.Errorf("wizard accepts only flags; unexpected positional arguments: %q", fs.Args())
	}
	explicitFlags := visitedFlags(fs)
	approveProvided := explicitFlags["approve-plan"]
	if *metadataOnly && (*planOnly || approveProvided) {
		return fmt.Errorf("--metadata-only cannot be combined with --plan-only or --approve-plan")
	}
	if *planOnly && approveProvided {
		return fmt.Errorf("--plan-only cannot be combined with --approve-plan")
	}
	if approveProvided && !*nonInteractive {
		return fmt.Errorf("--approve-plan requires --non-interactive")
	}

	// One run-scoped context drives preflight, Prepare, Execute, and every write
	// boundary. An injected context wins for deterministic tests; otherwise the
	// CLI wires os.Interrupt so Ctrl-C cancels the run instead of leaving a false
	// success behind.
	ctx := input.Context
	if ctx == nil {
		var stop context.CancelFunc
		ctx, stop = signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}
	workspaceRoot := detectWorkspaceRoot(cwd)

	state := wizardState{
		UseMode:       resolveUseModeDefault(*defaultUseMode, *defaultStart, explicitFlags["use-mode"]),
		EnginePack:    normalizeEnginePack(*defaultEnginePack),
		SetupDepth:    resolveSetupDepthDefault(*defaultSetupDepth, *defaultComplexity, explicitFlags["setup-depth"]),
		ProfileName:   sanitizeProfileName(*defaultProfile),
		MCPRequired:   []string{},
		MCPOptional:   []string{},
		AssetPipeline: wizardAssetPipelineState{Intent: "metadata-only visual asset preferences"},
	}
	state.StartingPoint = startingPointForUseMode(state.UseMode)
	state.Complexity = complexityForSetupDepth(state.SetupDepth)
	state.DeferredLanes = defaultDeferredLanes()
	state.NextSteps = defaultNextSteps(state)

	// A plan-only preview on an interactive terminal hands the read-only journey
	// to the styled TUI. Every other path, including non-interactive plan-only,
	// keeps the existing prompt/report behavior unchanged.
	isTerminal := wizardTuiTerminalCheck(input)
	if wizardTuiEnabled(*planOnly, *nonInteractive, isTerminal(os.Stdin), isTerminal(os.Stdout)) {
		return runWizardTuiPreview(ctx, input, wizardTuiOptions{
			workspaceRoot:     workspaceRoot,
			outDir:            *outDir,
			finalArtifactPath: *finalArtifactPath,
			state:             state,
		})
	}

	reader := bufio.NewReader(os.Stdin)

	stepWelcomeScreen()
	if err := stepChooseUseMode(reader, *nonInteractive, &state); err != nil {
		return err
	}
	if err := stepChooseEnginePack(reader, *nonInteractive, &state); err != nil {
		return err
	}
	if err := stepChooseSetupDepth(reader, *nonInteractive, &state); err != nil {
		return err
	}
	if err := stepProfileNaming(reader, *nonInteractive, &state); err != nil {
		return err
	}
	if err := stepResources(reader, *nonInteractive, &state); err != nil {
		return err
	}
	if err := stepOptionalIntegrations(reader, *nonInteractive, &state, input.OptionalIntegrationCatalog); err != nil {
		return err
	}
	stepVisualWorkflowLane(&state)
	if err := stepRuntimeContext(&state); err != nil {
		return err
	}

	var preflightResults []toolcheck.Result
	if input.EnvChecker != nil {
		selectedToolIDs := wizardRequiredToolIDs(state, input.ToolRegistry)
		results := input.EnvChecker.Execute(ctx, toolcheck.SelectTools(selectedToolIDs...))
		printEnvCheckResults("wizard preflight", results)
		state.EnvWarnings = nonBlockingWarnings(results)
		state.EnvBlockers = toolcheck.BlockingResults(results)

		optionalIDs := optionalDiagnosticToolIDs(input.ToolRegistry, state.OptionalIntegrations)
		if len(optionalIDs) > 0 {
			optionalResults := optionalWarningResults(input.EnvChecker.Execute(ctx, toolcheck.SelectTools(optionalIDs...)))
			printEnvCheckResults("wizard optional diagnostics", optionalResults)
			state.EnvWarnings = append(state.EnvWarnings, nonBlockingWarnings(optionalResults)...)
			results = append(results, optionalResults...)
		}
		preflightResults = results
		state.ValidationScope = wizardValidationScope(state, input.ToolRegistry, results)
		state.PreflightResult = wizardPreflightResult(state, results)
		state.DeepValidationRequired = deepValidationRequired(state)
	} else {
		state.ValidationScope = wizardValidationScope(state, input.ToolRegistry, nil)
		state.PreflightResult = wizardPreflightResult(state, nil)
		state.DeepValidationRequired = deepValidationRequired(state)
	}

	baseNextSteps := append([]string{}, state.NextSteps...)
	state.NextSteps = appendEnvironmentNextSteps(state, baseNextSteps)

	installer := input.Installer
	var (
		runtimeConfig piinstall.Config
		runtimePlan   piinstall.Plan
	)
	if !*metadataOnly {
		if err := wizardCanceled(ctx); err != nil {
			return err
		}
		if installer == nil {
			installer = DefaultWizardInstaller()
		}
		physicalWorkspace, err := resolvePhysicalPath(workspaceRoot)
		if err != nil {
			return fmt.Errorf("resolve workspace root: %w", err)
		}
		runtimeConfig = wizardRuntimeConfig(input, physicalWorkspace, state)
		_, runtimePlan, err = installer.Prepare(ctx, runtimeConfig)
		if err != nil {
			return fmt.Errorf("prepare runtime prerequisite plan: %w", err)
		}
		if err := wizardCanceled(ctx); err != nil {
			return err
		}
		printWizardRuntimePlan(runtimePlan)
		if *planOnly {
			printWizardPlanApprovalInstructions(runtimePlan, input.Args, *nonInteractive, workspaceRoot)
			return nil
		}
		if runtimePlan.Blocked() {
			return errWizardRuntimeBlocked
		}
	}

	stepSummaryScreen(state)
	preflightErr := toolcheck.BlockingError(state.EnvBlockers)

	if *metadataOnly {
		fmt.Println("[wizard] metadata-only generates local profile/pack artifacts, the workspace config, the final artifact, and attempts the Engram memory write-through; it is not a read-only run.")
		if err := stepConfirmationPrompt(reader, *nonInteractive, &state); err != nil {
			return err
		}
		if !state.Confirmed {
			fmt.Println("[wizard] cancelled before generation")
			return nil
		}
	}

	// The profile artifact is derived from the persisted-preset provenance, so
	// resolve that provenance before validating targets: the preflight and the
	// generation step must render byte-identical content.
	state.PresetSource = presetSourcePersistedWorkspace
	state.RoutingPresetSource = routingPresetSourceDerived

	targets, err := wizardValidateLocalTargets(workspaceRoot, *outDir, *finalArtifactPath, state, runtimeConfig.InstallRoot)
	if err != nil {
		return err
	}

	if !*metadataOnly {
		fmt.Println("[wizard] after a successful prerequisite install this run also writes local profile/pack artifacts, the workspace config, and attempts the Engram memory write-through.")
		if err := wizardCanceled(ctx); err != nil {
			return err
		}
		consent, err := resolveWizardRuntimeConsent(reader, *nonInteractive, *approvePlan, approveProvided, runtimePlan)
		if err != nil {
			return err
		}
		if !consent.Approved {
			fmt.Println("[wizard] prerequisite installation not approved; no files were written")
			return nil
		}

		report, execErr := installer.Execute(ctx, runtimeConfig, runtimePlan, consent)
		if execErr != nil {
			printWizardRuntimeReport(report)
			return fmt.Errorf("execute runtime prerequisite plan: %w", execErr)
		}
		if err := wizardCanceled(ctx); err != nil {
			state.RuntimeSetup = wizardRuntimeSetupFromReport(runtimePlan, report)
			printWizardRuntimeReport(report)
			return fmt.Errorf("%w: %v", errWizardRuntimeIncomplete, err)
		}
		if reason := wizardReportIncompleteReason(runtimePlan, report); reason != "" {
			state.RuntimeSetup = wizardRuntimeSetupFromReport(runtimePlan, report)
			printWizardRuntimeReport(report)
			fmt.Printf("[wizard] prerequisite installation incomplete: %s\n", reason)
			return errWizardRuntimeIncomplete
		}
		printWizardRuntimeReport(report)
		printWizardRuntimeLaunch(report)
		state.RuntimeSetup = wizardRuntimeSetupFromReport(runtimePlan, report)
		if input.EnvChecker != nil {
			reconcileRuntimeVerifiedGodot(&state, input.ToolRegistry, report, preflightResults, baseNextSteps)
		}
		// Route the real Engram write-through to the backend-verified binary
		// instead of assuming one on the ambient PATH. A complete report always
		// reported a nonempty Engram binary; an empty one is incomplete above.
		input.Persistence = input.Persistence.WithEngramExecutable(report.Launch.EngramBinary)
		state.NextSteps = append(state.NextSteps, wizardRuntimeNextSteps(report)...)
		preflightErr = toolcheck.BlockingError(state.EnvBlockers)
	}

	if err := wizardCanceled(ctx); err != nil {
		return err
	}
	config, err := newWorkspaceConfig(state.SetupDepth, time.Now())
	if err != nil {
		return fmt.Errorf("prepare confirmed workspace config: %w", err)
	}
	if err := writeWorkspaceConfigAtomic(workspaceRoot, config); err != nil {
		return fmt.Errorf("persist confirmed workspace config: %w", err)
	}

	if err := stepGenerationArtifacts(ctx, input, workspaceRoot, targets.outDir, targets.finalArtifactPath, &state); err != nil {
		return err
	}
	if err := stepSmokeValidation(&state); err != nil {
		return err
	}
	stepNextStepsScreen(state)
	if preflightErr != nil {
		return preflightErr
	}

	fmt.Println("[wizard] staged setup flow completed")
	return nil
}

// wizardResolvedTargets holds the pre-validated, workspace-contained output
// destinations. Resolving them performs no writes.
type wizardResolvedTargets struct {
	outDir            string
	finalArtifactPath string
}

// wizardValidateLocalTargets resolves every wizard destination and applies the
// G5a profile leaf policy plus the physical ancestor containment check before
// any Execute or write. It then validates the complete managed-destination
// inventory so distinct output roles cannot alias each other or protected
// namespaces. A refusal here leaves the workspace untouched.
func wizardValidateLocalTargets(workspaceRoot, outDir, finalArtifactPath string, state wizardState, installRoot string) (wizardResolvedTargets, error) {
	resolvedOutDir, resolvedFinalArtifactPath, err := resolveWizardOutputPaths(workspaceRoot, outDir, finalArtifactPath)
	if err != nil {
		return wizardResolvedTargets{}, err
	}
	profileArtifactTarget := wizardProfileArtifactRel(state.ProfileName)
	resolvedProfileArtifactTarget, err := resolveWizardProfileTarget(workspaceRoot, profileArtifactTarget)
	if err != nil {
		return wizardResolvedTargets{}, fmt.Errorf("validate profile artifact target: %w", err)
	}
	if err := preflightWizardProfileTarget(resolvedProfileArtifactTarget, renderWizardProfileMarkdown(state)); err != nil {
		return wizardResolvedTargets{}, err
	}
	managedConfigDirTarget := "openspec"
	if _, err := resolveWizardSafePath(workspaceRoot, &managedConfigDirTarget); err != nil {
		return wizardResolvedTargets{}, fmt.Errorf("validate openspec config target: %w", err)
	}
	if err := wizardValidateManagedDestinations(workspaceRoot, outDir, finalArtifactPath, profileArtifactTarget, installRoot); err != nil {
		return wizardResolvedTargets{}, err
	}
	return wizardResolvedTargets{outDir: resolvedOutDir, finalArtifactPath: resolvedFinalArtifactPath}, nil
}

func stepWelcomeScreen() {
	fmt.Printf("[wizard][step-%d] Welcome\n", stepWelcomeNum)
	fmt.Println("  Pi only · staged Game-Studio profile setup · Godot first · hybrid persistence")
	fmt.Println("  This wizard records intent, profile metadata, verification, and next steps. By default it plans missing Pi-only prerequisites and, after explicit fingerprint-bound approval, installs them. Use --metadata-only to write artifacts only; --plan-only previews without writing. It never executes future lanes.")
	fmt.Println("  Pi owns authentication and models; Gentle Shell owns orchestration. Future integrations remain explicit placeholders.")
}

func stepChooseUseMode(reader *bufio.Reader, nonInteractive bool, state *wizardState) error {
	fmt.Printf("[wizard][step-%d] Choose use mode\n", stepUseModeNum)
	fmt.Println("  create_from_scratch · existing_game · design_narrative_only · visual_artifacts_only · godot_sdd_handoff · repair_change_workflow")
	value, err := promptValue(reader, nonInteractive, "  Use mode", state.UseMode)
	if err != nil {
		return err
	}
	state.UseMode = normalizeUseMode(value)
	if state.UseMode == "" {
		return fmt.Errorf("invalid use mode: %q", value)
	}
	state.StartingPoint = startingPointForUseMode(state.UseMode)
	return nil
}

func stepChooseEnginePack(reader *bufio.Reader, nonInteractive bool, state *wizardState) error {
	fmt.Printf("[wizard][step-%d] Choose engine pack\n", stepEnginePackNum)
	fmt.Println("  Godot is active. Unity/UE5 remain explicit future placeholders.")
	value, err := promptValue(reader, nonInteractive, "  Engine pack", state.EnginePack)
	if err != nil {
		return err
	}
	state.EnginePack = normalizeEnginePack(value)
	if state.EnginePack == "" {
		return fmt.Errorf("invalid engine pack: %q", value)
	}
	return nil
}

func stepChooseSetupDepth(reader *bufio.Reader, nonInteractive bool, state *wizardState) error {
	fmt.Printf("[wizard][step-%d] Choose setup depth\n", stepSetupDepthNum)
	fmt.Println("  minimal · recommended · full · custom")
	value, err := promptValue(reader, nonInteractive, "  Setup depth", state.SetupDepth)
	if err != nil {
		return err
	}
	state.SetupDepth = normalizeSetupDepth(value)
	if state.SetupDepth == "" {
		return fmt.Errorf("invalid setup depth: %q", value)
	}
	state.Complexity = complexityForSetupDepth(state.SetupDepth)
	return nil
}

func stepProfileNaming(reader *bufio.Reader, nonInteractive bool, state *wizardState) error {
	fmt.Printf("[wizard][step-%d] Profile naming / renaming\n", stepProfileNameNum)
	value, err := promptValue(reader, nonInteractive, "  Profile name", state.ProfileName)
	if err != nil {
		return err
	}
	if strings.TrimSpace(value) != "" {
		state.ProfileName = sanitizeProfileName(value)
	} else {
		state.ProfileName = sanitizeProfileName(state.ProfileName)
	}
	return nil
}

func stepResources(reader *bufio.Reader, nonInteractive bool, state *wizardState) error {
	fmt.Printf("[wizard][step-%d] Essential selections\n", stepResourcesNum)
	state.Connectors, state.Packs, state.Tools = defaultsForSetup(state.EnginePack, state.SetupDepth, state.UseMode)
	fmt.Println("  Select only what this first profile needs. Deeper tool detection and preset taxonomy stay deferred.")

	connectors, err := promptValue(reader, nonInteractive, "  Connectors (csv)", strings.Join(state.Connectors, ","))
	if err != nil {
		return err
	}
	packs, err := promptValue(reader, nonInteractive, "  Packs (csv)", strings.Join(state.Packs, ","))
	if err != nil {
		return err
	}
	tools, err := promptValue(reader, nonInteractive, "  Tools (csv)", strings.Join(state.Tools, ","))
	if err != nil {
		return err
	}

	state.Connectors = splitCSV(connectors)
	state.Packs = splitCSV(packs)
	state.Tools = splitCSV(tools)

	fmt.Println("  Memory uses the Pi-native Engram companion; MCP adapters are optional and are not configured or inspected by this wizard.")
	return nil
}

const wizardProviderNotice = "Pi owns authentication and model configuration; OGS did not inspect providers or models. An empty inventory is not evidence that no provider exists"

const wizardMCPNotice = "memory uses the Pi-native Engram companion; MCP adapters are optional and are not configured, started, or inspected by this wizard"

func stepRuntimeContext(state *wizardState) error {
	fmt.Printf("[wizard][step-%d] Runtime ownership\n", stepProvidersNum)
	fmt.Println("  Pi owns authentication and model configuration; Gentle Shell owns orchestration and its native runtime.")
	fmt.Println("  This wizard does not inspect Pi or any other runtime's providers or models; provider/model inventory remains not-inspected.")
	fmt.Printf("  %s\n", wizardProviderNotice)
	fmt.Println("  Optional MCP adapters and future integrations are not configured, started, or verified here.")

	state.Providers = []opencode.ConnectedProvider{}
	state.Snapshot = opencode.ModelSnapshot{
		Profile:    state.ProfileName,
		CapturedAt: time.Now().UTC(),
		Providers:  []opencode.ProviderSnapshot{},
	}
	state.Policy = unconfiguredRoutingPolicy()
	state.ProviderWarning = wizardProviderNotice
	return nil
}

func stepVisualWorkflowLane(state *wizardState) {
	fmt.Printf("[wizard][step-%d] Visual workflow lane\n", stepVisualLaneNum)
	visualWorkflow := defaultVisualWorkflowProfileData()
	fmt.Printf("  %s recorded as metadata-only guidance; readiness=%s; approval=%s.\n", visualWorkflow.Contract.Version, visualWorkflow.Contract.Readiness.Status, visualWorkflow.Contract.ArtBible.ApprovalState)
	fmt.Println("  Source/production lanes, provider-native image generation, and ComfyUI/Blender execution remain deferred placeholders.")
	state.DeferredLanes = defaultDeferredLanes()
	state.NextSteps = defaultNextSteps(*state)
}

func stepTierRouting(reader *bufio.Reader, nonInteractive bool, state *wizardState) error {
	fmt.Printf("[wizard][step-%d] Tier routing\n", stepTierRoutingNum)
	fmt.Println("  Tier defaults from balanced preset:")
	for _, tier := range []routing.Tier{routing.TierFast, routing.TierBalanced, routing.TierDeep} {
		selection := state.Policy.TierDefaults[tier]
		fmt.Printf("  - %s => %s:%s\n", tier, selection.Provider, selection.Model)
	}

	if nonInteractive || len(state.Snapshot.Providers) == 0 {
		return nil
	}

	value, err := promptValue(reader, false, "  Optional tier override (tier=provider:model), blank to keep defaults", "")
	if err != nil {
		return err
	}
	if strings.TrimSpace(value) == "" {
		return nil
	}

	tier, selection, err := parseOverride(value)
	if err != nil {
		return err
	}
	normalizedTier := strings.ToLower(strings.TrimSpace(tier))
	if normalizedTier != string(routing.TierFast) && normalizedTier != string(routing.TierBalanced) && normalizedTier != string(routing.TierDeep) {
		return fmt.Errorf("invalid tier %q", tier)
	}
	if !state.Snapshot.HasModel(selection.Provider, selection.Model) {
		return fmt.Errorf("invalid tier override %s:%s", selection.Provider, selection.Model)
	}
	state.Policy.TierDefaults[routing.Tier(normalizedTier)] = routing.Selection{Provider: strings.ToLower(selection.Provider), Model: selection.Model}

	return nil
}

func stepSDDPhaseRouting(reader *bufio.Reader, nonInteractive bool, state *wizardState) error {
	fmt.Printf("[wizard][step-%d] SDD phase routing\n", stepPhaseRoutingNum)
	if state.Complexity != "expert" {
		fmt.Println("  Skipped: enabled only in expert mode.")
		return nil
	}
	if len(state.Snapshot.Providers) == 0 {
		fmt.Println("  Skipped: no connected providers/models available for phase overrides.")
		return nil
	}

	if nonInteractive {
		fmt.Println("  No phase overrides supplied (non-interactive).")
		return nil
	}

	for _, phase := range routing.DefaultPhases {
		value, err := promptValue(reader, false, fmt.Sprintf("  Phase override for %s (provider:model, blank to skip)", phase), "")
		if err != nil {
			return err
		}
		if strings.TrimSpace(value) == "" {
			continue
		}
		selection := splitProviderModel(value)
		if err := state.Policy.ApplyPhaseOverride(state.Snapshot, phase, selection); err != nil {
			return err
		}
	}

	return nil
}

func stepRoleOverrides(reader *bufio.Reader, nonInteractive bool, state *wizardState) error {
	fmt.Printf("[wizard][step-%d] Role overrides\n", stepRoleOverridesNum)
	if state.Complexity == "simple" {
		fmt.Println("  Skipped: enabled in advanced/expert modes only.")
		return nil
	}
	if len(state.Snapshot.Providers) == 0 {
		fmt.Println("  Skipped: no connected providers/models available for role overrides.")
		return nil
	}

	if nonInteractive {
		fmt.Println("  No role overrides supplied (non-interactive).")
		return nil
	}

	for _, role := range routing.DefaultRoles {
		value, err := promptValue(reader, false, fmt.Sprintf("  Role override for %s (provider:model, blank to skip)", role), "")
		if err != nil {
			return err
		}
		if strings.TrimSpace(value) == "" {
			continue
		}
		selection := splitProviderModel(value)
		if err := state.Policy.ApplyRoleOverride(state.Snapshot, role, selection); err != nil {
			return err
		}
	}

	return nil
}

func stepMCPSelection(reader *bufio.Reader, nonInteractive bool, state *wizardState) error {
	fmt.Printf("[wizard][step-%d] MCPs\n", stepMCPsNum)
	fmt.Println("  MCP adapters are optional and not configured by this wizard; memory uses the Pi-native Engram companion.")

	if nonInteractive {
		return nil
	}

	value, err := promptValue(reader, false, "  Optional MCP extras (csv)", "")
	if err != nil {
		return err
	}
	state.MCPOptional = splitCSV(value)
	return nil
}

func stepOptionalIntegrations(reader *bufio.Reader, nonInteractive bool, state *wizardState, catalog integrations.Catalog) error {
	fmt.Printf("[wizard][step-%d] Optional integrations\n", stepOptionalsNum)
	if catalog == nil {
		catalog = integrations.NewDefaultRegistry()
	}
	items := catalog.List()
	fallbackIDs := presetIntegrationDefaults(state.SetupDepth, items)
	if nonInteractive {
		state.OptionalIntegrations = []integrations.OptionalIntegrationSelection{}
		state.AssetPipeline.SelectedAdapters = []assets.AdapterCapability{}
		state.AssetPipeline.DeferredRequests = []string{}
		state.AcceptedConsents = []string{}
		state.RejectedConsents = []string{}
		state.DeferredConsents = deferredConsentIDs(items, nil)
		if len(fallbackIDs) == 0 {
			fmt.Println("  optional integrations: none (non-interactive default)")
			return nil
		}
		fmt.Printf("  setup_preset %s preselects safe metadata only: %s\n", state.SetupDepth, strings.Join(fallbackIDs, ", "))
		return applyOptionalIntegrationSelections(reader, true, state, catalog, fallbackIDs)
	}

	if normalizeSetupDepth(state.SetupDepth) == "custom" {
		fmt.Println("  Categories are separated: core / optional / consent-sensitive / workflow adapter / visual lane / future-deferred.")
	}
	for _, item := range items {
		fmt.Printf("  - [%s] %s: %s (%s)\n", taxonomyCategoryForIntegration(item), item.ID, item.Label, item.Category)
		for _, hint := range item.Hints {
			fmt.Printf("    hint: %s\n", hint)
		}
	}
	fallback := strings.Join(fallbackIDs, ",")
	value, err := promptValue(reader, false, "  Optional/workflow selections (csv stable IDs; consent prompts stay explicit)", fallback)
	if err != nil {
		return err
	}
	return applyOptionalIntegrationSelections(reader, false, state, catalog, splitCSV(value))
}

func applyOptionalIntegrationSelections(reader *bufio.Reader, nonInteractive bool, state *wizardState, catalog integrations.Catalog, ids []string) error {
	items := catalog.List()
	selectedIDs := map[string]bool{}
	selected := make([]integrations.OptionalIntegrationSelection, 0)
	adapters := make([]assets.AdapterCapability, 0)
	state.AcceptedConsents = []string{}
	state.RejectedConsents = []string{}
	for _, id := range ids {
		if reason, ok := assets.DeferredAdapterReason(id); ok {
			fmt.Printf("  warning: asset adapter %q is deferred future scope (%s); ignored\n", id, reason)
			state.AssetPipeline.DeferredRequests = append(state.AssetPipeline.DeferredRequests, fmt.Sprintf("%s: %s", id, reason))
			continue
		}
		item, ok := catalog.Get(id)
		if !ok {
			fmt.Printf("  warning: optional integration %q is not registered; ignored\n", id)
			continue
		}
		selectedIDs[item.ID] = true
		selection := integrations.OptionalIntegrationSelection{ID: item.ID}
		if len(item.Consent) > 0 {
			consent := map[string]bool{}
			allGranted := true
			for _, requirement := range item.Consent {
				answer, err := promptValue(reader, nonInteractive, fmt.Sprintf("  Consent for %s — %s (yes/no)", item.Label, requirement.Label), "no")
				if err != nil {
					return err
				}
				granted := isYes(answer)
				consent[requirement.ID] = granted
				if !granted {
					allGranted = false
					state.RejectedConsents = append(state.RejectedConsents, consentArtifactID(item.ID, requirement.ID))
				} else {
					state.AcceptedConsents = append(state.AcceptedConsents, consentArtifactID(item.ID, requirement.ID))
				}
			}
			if !allGranted {
				fmt.Printf("  %s omitted because required consent was not granted\n", item.ID)
				continue
			}
			selection.Consent = consent
		}
		if item.AssetAdapter != nil {
			adapters = append(adapters, *item.AssetAdapter)
			continue
		}
		selected = append(selected, selection)
	}
	state.OptionalIntegrations = selected
	state.AssetPipeline.SelectedAdapters = adapters
	state.DeferredConsents = deferredConsentIDs(items, selectedIDs)
	return nil
}

func stepSummaryScreen(state wizardState) {
	fmt.Printf("[wizard][step-%d] Summary\n", stepSummaryNum)
	fmt.Printf("  profile: %s\n", state.ProfileName)
	fmt.Printf("  setup_preset: %s\n", state.SetupDepth)
	fmt.Printf("  taxonomy_version: %s\n", wizardTaxonomyVersion)
	fmt.Printf("  use_mode: %s\n", state.UseMode)
	fmt.Printf("  engine_pack: %s\n", state.EnginePack)
	enginePack := enginePackMetadata(state.EnginePack)
	fmt.Printf("  engine_pack_status: %s\n", enginePack.Status)
	if enginePack.GenerationDeferred {
		fmt.Printf("  engine_pack_note: %s\n", enginePack.Note)
	}
	fmt.Printf("  setup_depth: %s\n", state.SetupDepth)
	fmt.Printf("  complexity: %s\n", state.Complexity)
	fmt.Printf("  starting_point: %s\n", state.StartingPoint)
	fmt.Printf("  connectors: %s\n", strings.Join(state.Connectors, ", "))
	fmt.Printf("  packs: %s\n", strings.Join(state.Packs, ", "))
	fmt.Printf("  tools: %s\n", strings.Join(state.Tools, ", "))
	printValidationScopeSummary(state)
	if state.ProviderWarning != "" {
		fmt.Printf("  provider_notice: %s\n", state.ProviderWarning)
	}
	if len(state.EnvBlockers) > 0 {
		fmt.Println("  env_blockers:")
		for _, blocker := range state.EnvBlockers {
			fmt.Printf("    - %s\n", blocker.String())
		}
	}
	readiness := wizardGenerationReadiness(state)
	for _, line := range readiness.SummaryLines {
		fmt.Printf("  %s\n", line)
	}
	if len(state.EnvWarnings) > 0 {
		fmt.Println("  env_warnings:")
		for _, warning := range state.EnvWarnings {
			fmt.Printf("    - %s\n", warning.String())
		}
	}
	if len(state.DeepValidationRequired) > 0 {
		fmt.Println("  deep_validation_required:")
		for _, item := range state.DeepValidationRequired {
			fmt.Printf("    - %s\n", item)
		}
	}
	fmt.Println("  providers: not-inspected (Pi owns authentication and model configuration)")
	fmt.Printf("  preset: %s\n", state.Policy.Preset)
	fmt.Printf("  phase_overrides: %d\n", len(state.Policy.PhaseOverrides))
	fmt.Printf("  role_overrides: %d\n", len(state.Policy.RoleOverrides))
	fmt.Printf("  mcp_required: %s\n", formatListOrNone(state.MCPRequired))
	fmt.Printf("  mcp_optional: %s\n", formatListOrNone(state.MCPOptional))
	fmt.Printf("  mcp_note: %s\n", wizardMCPNotice)
	if len(state.OptionalIntegrations) == 0 {
		fmt.Println("  optional_integrations: none")
	} else {
		fmt.Println("  optional_integrations:")
		for _, selection := range state.OptionalIntegrations {
			fmt.Printf("    - %s (metadata only; not installed or started)\n", selection.ID)
			for key, value := range selection.Consent {
				fmt.Printf("      consent.%s: %t\n", key, value)
			}
		}
	}
	if len(state.AcceptedConsents) == 0 && len(state.RejectedConsents) == 0 && len(state.DeferredConsents) == 0 {
		fmt.Println("  consent_sensitive: none")
	} else {
		fmt.Printf("  consent_sensitive accepted=%s rejected=%s deferred=%s\n", strings.Join(state.AcceptedConsents, ", "), strings.Join(state.RejectedConsents, ", "), strings.Join(state.DeferredConsents, ", "))
	}
	if len(state.AssetPipeline.SelectedAdapters) == 0 {
		fmt.Println("  workflow_adapters: no visual adapter selected (metadata contract still documented)")
	} else {
		fmt.Println("  workflow_adapters:")
		for _, adapter := range state.AssetPipeline.SelectedAdapters {
			fmt.Printf("    - %s (%s): metadata only; no assets generated; no adapter execution\n", adapter.ID, adapter.Label)
		}
		fmt.Println("  asset_pipeline: workflow adapter preferences recorded as metadata only; no assets generated")
	}
	visualWorkflow := defaultVisualWorkflowProfileData()
	fmt.Printf("  visual_workflow_lanes: %s; readiness=%s; approval=%s; metadata only; no image/audio/model generation or routing executed\n", visualWorkflow.Contract.Version, visualWorkflow.Contract.Readiness.Status, visualWorkflow.Contract.ArtBible.ApprovalState)
	coreWorkflow := defaultCoreGameWorkflowProfileData()
	fmt.Printf("  core_game_workflow: %s; game intent, GDD slice, narrative modes, repair/change handoff; human approval required; no auto-approval\n", coreWorkflow.Contract.Version)
	modelRouting := modelrouting.DefaultContract(state.SetupDepth)
	fmt.Printf("  model_routing: %s; preset=%s; required=%s; optional=%s; future=%s; validation=%s; metadata only; no provider/model execution\n", modelRouting.Version, modelRouting.RoutingPreset, formatCapabilityIDs(requiredCapabilityIDs(modelRouting.Capabilities)), formatCapabilityIDs(optionalCapabilityIDs(modelRouting.Capabilities)), formatCapabilityIDs(futureCapabilityIDs(modelRouting.Capabilities)), modelRouting.RoutingValidationResult.Status)
	fmt.Println("  engine_pack_boundary: Unity/UE5 are future placeholders. Godot is the only active engine pack for generated profiles today.")
	fmt.Println("  will_generate: profile markdown, Godot pack metadata, final machine-readable artifact, smoke verification")
	fmt.Println("  local_writes: profile/pack artifacts, workspace config, the final artifact, and an Engram memory write-through (metadata-only included); prerequisite installation is separate and requires explicit approval")
	fmt.Println("  will_not_execute: consent-sensitive integrations without consent, visual adapters, image generation, audio generation, model routing, GDD/artifact approval")
	if len(state.DeferredLanes) > 0 {
		fmt.Println("  deferred_lanes:")
		for _, lane := range state.DeferredLanes {
			fmt.Printf("    - %s: %s (%s)\n", lane.ID, lane.Status, lane.Note)
		}
	}
}

func stepConfirmationPrompt(reader *bufio.Reader, nonInteractive bool, state *wizardState) error {
	fmt.Printf("[wizard][step-%d] Confirmation\n", stepConfirmationNum)
	if nonInteractive {
		state.Confirmed = true
		fmt.Println("  Auto-confirmed (non-interactive)")
		return nil
	}

	value, err := promptValue(reader, false, "  Confirm generation? (yes/no)", "yes")
	if err != nil {
		return err
	}
	state.Confirmed = strings.EqualFold(strings.TrimSpace(value), "yes") || strings.EqualFold(strings.TrimSpace(value), "y")
	return nil
}

func stepGenerationArtifacts(runCtx context.Context, input WizardInput, workspaceRoot string, resolvedOutDir, resolvedFinalArtifactPath string, state *wizardState) error {
	fmt.Printf("[wizard][step-%d] Generation\n", stepGenerationNum)
	if err := wizardCanceled(runCtx); err != nil {
		return err
	}
	if _, _, err := resolveWizardOutputPaths(workspaceRoot, resolvedOutDir, resolvedFinalArtifactPath); err != nil {
		return err
	}

	profileArtifactRel := wizardProfileArtifactRel(state.ProfileName)
	resolvedProfileArtifactPath, err := resolveWizardProfileTarget(workspaceRoot, profileArtifactRel)
	if err != nil {
		return fmt.Errorf("validate profile artifact target: %w", err)
	}
	managedConfigDirRel := "openspec"
	if _, err := resolveWizardSafePath(workspaceRoot, &managedConfigDirRel); err != nil {
		return fmt.Errorf("validate openspec config target: %w", err)
	}

	doc := workdoc.Document{
		ProfileName:     state.ProfileName,
		PrimaryEngine:   string(templates.EngineGodot),
		Platform:        "Pi only",
		PersistenceMode: input.Persistence.Mode(),
	}

	selected := map[string]bool{
		artifactProfile:    true,
		artifactSummary:    true,
		artifactPattern:    true,
		artifactPackConfig: true,
	}

	preset := newResolvedPreset(state.SetupDepth, state.PresetSource)
	emitted, err := generatePackArtifactsWithAssetPipeline(GenerateInput{Registry: input.Registry, Persistence: input.Persistence}, doc, templates.EngineGodot, "", resolvedOutDir, "flat", selected, defaultAssetPipelineProfileData(state.AssetPipeline.SelectedAdapters), preset)
	if err != nil {
		return err
	}
	state.PackArtifacts = emitted

	paths, err := resolveOutputPaths(templates.EngineGodot, "", resolvedOutDir, "flat")
	if err != nil {
		return err
	}
	state.SmokePaths = paths

	state.ProfileArtifactPath = resolvedProfileArtifactPath
	if err := writeWizardProfileMarkdown(resolvedProfileArtifactPath, *state); err != nil {
		return err
	}

	// The first final artifact records the memory outcome as pending so it never
	// claims a completed whole-setup success before the Engram write-through has
	// run. A successful save updates the status; a failed or canceled save leaves
	// an honest non-success record.
	state.FinalArtifactPath = resolvedFinalArtifactPath
	state.MemoryStatus = wizardMemoryPending
	state.MemoryDetail = ""
	if err := writeWizardFinalArtifact(resolvedFinalArtifactPath, *state); err != nil {
		return err
	}

	if err := writeWizardManagedBlock(workspaceRoot, state.ProfileName, state.ProfileArtifactPath, state.FinalArtifactPath); err != nil {
		return err
	}

	emittedArtifacts := append([]string{}, state.PackArtifacts...)
	emittedArtifacts = append(emittedArtifacts, state.ProfileArtifactPath, state.FinalArtifactPath)
	if err := wizardCanceled(runCtx); err != nil {
		return err
	}
	writeThrough := persistence.GenerationWriteThrough{
		ProfileName:      state.ProfileName,
		Engine:           string(templates.EngineGodot),
		PersistenceMode:  input.Persistence.Mode(),
		SummaryArtifact:  state.FinalArtifactPath,
		GeneratedAt:      time.Now().UTC(),
		EmittedArtifacts: emittedArtifacts,
	}
	if err := wizardPersistGeneration(runCtx, input.Persistence, state, writeThrough); err != nil {
		// A canceled run must not be forced to write again just to persist the
		// outcome. When the context is still live, record the failed/disabled
		// outcome honestly; the local artifacts already written are preserved.
		if runCtx.Err() == nil {
			if writeErr := writeWizardFinalArtifact(resolvedFinalArtifactPath, *state); writeErr != nil {
				fmt.Printf("  warning: could not record the memory outcome in the final artifact: %v\n", writeErr)
			}
		}
		return err
	}
	if runCtx.Err() == nil {
		if err := writeWizardFinalArtifact(resolvedFinalArtifactPath, *state); err != nil {
			return err
		}
	}

	fmt.Printf("  generated profile: %s\n", filepath.Clean(state.ProfileArtifactPath))
	fmt.Printf("  generated final artifact: %s\n", filepath.Clean(state.FinalArtifactPath))

	return nil
}

func wizardProfileArtifactRel(profileName string) string {
	return filepath.Join(".game-studio", "profiles", fmt.Sprintf("%s.md", sanitizeProfileFileName(profileName)))
}

func stepSmokeValidation(state *wizardState) error {
	fmt.Printf("[wizard][step-%d] Smoke\n", stepSmokeNum)

	if err := runSmokeContentChecks(state.SmokePaths); err != nil {
		return err
	}
	if err := validateWizardProfileMarkdown(state.ProfileArtifactPath, state.ProfileName); err != nil {
		return err
	}

	fmt.Println("  smoke validation passed")
	switch {
	case len(state.EnvBlockers) > 0:
		fmt.Println("  metadata-only profile artifacts validated; engine workflows remain blocked by preflight")
	case wizardRuntimeSetupVerifiedComplete(state.RuntimeSetup):
		fmt.Println("  wizard runtime flow completed successfully")
	default:
		fmt.Println("  metadata profile artifacts validated; runtime readiness was not verified in this run")
	}
	return nil
}

func stepNextStepsScreen(state wizardState) {
	fmt.Printf("[wizard][step-%d] Next steps\n", stepNextStepsNum)
	for _, step := range state.NextSteps {
		fmt.Printf("  - %s\n", step)
	}
}

func unconfiguredRoutingPolicy() routing.Policy {
	fallback := routing.Selection{Provider: modelrouting.StatusNotConfigured, Model: modelrouting.StatusNotConfigured}
	return routing.Policy{
		Preset: routing.PresetBalanced,
		TierDefaults: map[routing.Tier]routing.Selection{
			routing.TierFast:     fallback,
			routing.TierBalanced: fallback,
			routing.TierDeep:     fallback,
		},
		RoleOverrides:  map[string]routing.Selection{},
		PhaseOverrides: map[string]routing.Selection{},
	}
}

func writeWizardProfileMarkdown(path string, state wizardState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create profile directory: %w", err)
	}
	return writeWizardProfileContent(path, renderWizardProfileMarkdown(state))
}

// renderWizardProfileMarkdown renders the deterministic wizard profile content.
// It carries no run timestamp so an unchanged rerun is byte-identical; truthful
// run metadata stays in the separate final artifact.
func renderWizardProfileMarkdown(state wizardState) string {
	visualWorkflow := assets.DefaultVisualWorkflow()
	coreWorkflow := coregame.DefaultContract()

	return fmt.Sprintf(`# %s

generated_by: installer-wizard
engine: godot
persistence: hybrid

## Starting Point
- use_mode: %s
- engine_pack: %s
- setup_depth: %s
- setup_preset: %s
- preset_source: %s
- mode: %s

## Resource Selection
- connectors: %s
- packs: %s
- tools: %s

## Providers
- ownership: %s
- inventory: not-inspected

## Model Routing
- contract_version: %s
- boundary: %s
- routing_preset: %s
- routing_preset_source: %s
- required_capabilities: %s
- optional_capabilities: %s
- future_capabilities: %s
- unconfigured_required: %s
- deferred_capabilities: %s
- validation: %s (can_continue=%t)
- tier.fast: %s
- tier.balanced: %s
- tier.deep: %s
- legacy_routing: true (%s)
- no_execution: no provider API calls, model downloads, image/audio generation, SDD execution, Godot mutation, ComfyUI/Blender execution, or auto-approval

## Overrides
- per_phase: %d
- per_role: %d

## MCP Runtime Assistants
- required: %s
- optional: %s
- note: %s

## Optional Integrations
%s

## Workflow Adapters
%s

## Visual Workflow Readiness
- contract_version: %s
- art_bible: %s (%s; approval=%s)
- asset_spec: %s (%s)
- asset_audit: %s (%s; read-only)
- routes: /art-bible, /asset-spec, /asset-audit
- boundary: metadata-only guidance; no art generated, no approval granted, no ComfyUI/Blender execution, no Godot/DCC mutation, no audio/music/SFX scope.

## Core Game Workflow
- contract_version: %s
- use_mode: %s
- phase_ids: %s
- narrative_modes: %s
- repair_change_handoff: reported issue -> triage -> classification -> relevant artifacts -> repair/change brief -> human approval -> downstream handoff -> verification
- approval: human approval required; artifacts remain draft/pending; auto_approved=false
- boundary: markers and artifact contracts only; no debugging execution, image/audio generation, automatic playtesting, or engine mutation.

## Deferred Lanes
%s

## Next Steps
%s
`, state.ProfileName, state.UseMode, state.EnginePack, state.SetupDepth, state.SetupDepth, state.PresetSource, state.StartingPoint, strings.Join(state.Connectors, ", "), strings.Join(state.Packs, ", "), strings.Join(state.Tools, ", "), wizardProviderNotice, modelrouting.Version, modelrouting.DefaultContract(state.SetupDepth).Boundary, state.SetupDepth, state.RoutingPresetSource, formatCapabilityIDs(requiredCapabilityIDs(modelrouting.DefaultContract(state.SetupDepth).Capabilities)), formatCapabilityIDs(optionalCapabilityIDs(modelrouting.DefaultContract(state.SetupDepth).Capabilities)), formatCapabilityIDs(futureCapabilityIDs(modelrouting.DefaultContract(state.SetupDepth).Capabilities)), formatCapabilityIDs(modelrouting.DefaultContract(state.SetupDepth).UnconfiguredRequiredCapabilities), formatCapabilityIDs(modelrouting.DefaultContract(state.SetupDepth).DeferredCapabilities), modelrouting.DefaultContract(state.SetupDepth).RoutingValidationResult.Status, modelrouting.DefaultContract(state.SetupDepth).RoutingValidationResult.CanContinue, formatSelection(state.Policy.TierDefaults[routing.TierFast]), formatSelection(state.Policy.TierDefaults[routing.TierBalanced]), formatSelection(state.Policy.TierDefaults[routing.TierDeep]), legacyRoutingBoundary, len(state.Policy.PhaseOverrides), len(state.Policy.RoleOverrides), formatListOrNone(state.MCPRequired), formatListOrNone(state.MCPOptional), wizardMCPNotice, formatOptionalIntegrationsMarkdown(state.OptionalIntegrations), formatWorkflowAdaptersMarkdown(state.AssetPipeline.SelectedAdapters), visualWorkflow.Version, visualWorkflow.ArtBible.Path, visualWorkflow.ArtBible.Status, visualWorkflow.ArtBible.ApprovalState, visualWorkflow.AssetSpec.PathPrefix, visualWorkflow.AssetSpec.Status, visualWorkflow.Audit.Path, visualWorkflow.Audit.Status, coreWorkflow.Version, state.UseMode, joinPhaseIDs(coreWorkflow.PhaseIDs), joinNarrativeModes(coreWorkflow.NarrativeModes), formatDeferredLanesMarkdown(state.DeferredLanes), formatNextStepsMarkdown(state.NextSteps))

}

// inspectWizardProfileLeaf applies the profile leaf policy to a path without
// following a symlink at that leaf. It reports whether a reusable regular-file
// leaf already exists. A symlink, dangling symlink, or any non-regular entry is
// refused; a missing leaf is reported as absent; any other stat error is
// surfaced so the caller aborts before writing.
func inspectWizardProfileLeaf(path string) (bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect wizard profile target %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("refusing to write wizard profile %s: target is a symlink", path)
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("refusing to write wizard profile %s: target is not a regular file", path)
	}
	return true, nil
}

// preflightWizardProfileTarget refuses to proceed when the resolved profile
// target already holds content this run did not generate. It performs no
// writes, so a refusal happens before the workspace config or any other
// artifact is emitted.
func preflightWizardProfileTarget(path, expected string) error {
	exists, err := inspectWizardProfileLeaf(path)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	existing, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read existing wizard profile %s: %w", path, err)
	}
	if !bytes.Equal(existing, []byte(expected)) {
		return fmt.Errorf("refusing to overwrite existing wizard profile %s: content differs from this run's generated profile; move or rename it, or choose a different --profile", path)
	}
	return nil
}

// writeWizardProfileContent creates a new profile exclusively, reuses an
// existing byte-identical profile, and refuses any other pre-existing target.
// Exclusive creation prevents a late-created leaf from being overwritten.
func writeWizardProfileContent(path, content string) error {
	expected := []byte(content)

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return preflightWizardProfileTarget(path, content)
		}
		return fmt.Errorf("write wizard profile markdown: %w", err)
	}

	if _, err := file.Write(expected); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return fmt.Errorf("write wizard profile markdown: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return fmt.Errorf("sync wizard profile markdown: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close wizard profile markdown: %w", err)
	}
	return nil
}

func writeWizardFinalArtifact(path string, state wizardState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create wizard artifact directory: %w", err)
	}

	readiness := wizardGenerationReadiness(state)
	payload := wizardFinalArtifact{
		Profile:                   state.ProfileName,
		UseMode:                   state.UseMode,
		EnginePack:                state.EnginePack,
		EnginePackStatus:          enginePackMetadata(state.EnginePack).Status,
		GenerationDeferred:        enginePackMetadata(state.EnginePack).GenerationDeferred,
		ActiveGenerationSupported: enginePackMetadata(state.EnginePack).ActiveGenerationSupported,
		SetupDepth:                state.SetupDepth,
		Complexity:                state.Complexity,
		StartingPoint:             state.StartingPoint,
		Connectors:                state.Connectors,
		Packs:                     state.Packs,
		Tools:                     state.Tools,
		Providers:                 snapshotProviders(state.Snapshot),
		ProviderNotice:            state.ProviderWarning,
		EnvWarnings:               resultStrings(state.EnvWarnings),
		EnvBlockers:               resultStrings(state.EnvBlockers),
		SelectedTools:             selectedToolIDsForScope(state.ValidationScope),
		ValidatedTools:            validatedToolIDsForScope(state.ValidationScope),
		SkippedTools:              skippedToolIDsForScope(state.ValidationScope),
		DeferredTools:             deferredToolIDsForScope(state.ValidationScope),
		ValidationScope:           state.ValidationScope,
		PreflightResult:           state.PreflightResult,
		GenerationReadiness:       readiness,
		ProfileGenerationAllowed:  readiness.ProfileGenerationAllowed,
		MetadataOnlyGeneration:    readiness.MetadataOnlyGeneration,
		EngineWorkflowReady:       readiness.EngineWorkflowReady,
		BlockedWorkflows:          readiness.BlockedWorkflows,
		BlockingReason:            readiness.BlockingReason,
		CanContinueMetadataOnly:   readiness.CanContinueMetadataOnly,
		DeepValidationRequired:    append([]string{}, state.DeepValidationRequired...),
		CapturedAt:                state.Snapshot.CapturedAt.Format(time.RFC3339),
		SetupPreset:               state.SetupDepth,
		PresetSource:              state.PresetSource,
		TaxonomyVersion:           wizardTaxonomyVersion,
		SelectedCore:              selectedCoreForArtifact(state),
		Preset:                    state.Policy.Preset,
		TierDefaults:              tierDefaultsForArtifact(state.Policy),
		PhaseOverrides:            selectionMapForArtifact(state.Policy.PhaseOverrides),
		RoleOverrides:             selectionMapForArtifact(state.Policy.RoleOverrides),
		LegacyRouting:             true,
		LegacyRoutingBoundary:     legacyRoutingBoundary,
		NoModelExecution:          true,
		MCP: map[string][]string{
			"required": state.MCPRequired,
			"optional": state.MCPOptional,
		},
		OptionalIntegrations:    wizardOptionalIntegrationsPayload(state.OptionalIntegrations, integrations.NewDefaultRegistry()),
		ConsentSensitive:        consentSensitivePayload(integrations.NewDefaultRegistry().List(), state),
		AcceptedConsents:        append([]string{}, state.AcceptedConsents...),
		RejectedConsents:        append([]string{}, state.RejectedConsents...),
		DeferredConsents:        append([]string{}, state.DeferredConsents...),
		WorkflowAdapters:        workflowAdaptersPayload(state.AssetPipeline.SelectedAdapters),
		VisualWorkflowLanes:     visualWorkflowLanesPayload(state),
		AssetPipeline:           wizardAssetPipelinePayload(state.AssetPipeline),
		VisualWorkflow:          wizardVisualWorkflowPayload(),
		CoreGameWorkflow:        wizardCoreGameWorkflowPayload(),
		ModelRouting:            wizardModelRoutingPayload(newResolvedPreset(state.SetupDepth, state.PresetSource)),
		ModelRoutingVersion:     modelrouting.Version,
		RoutingPreset:           modelrouting.DefaultContract(state.SetupDepth).RoutingPreset,
		RoutingPresetSource:     state.RoutingPresetSource,
		RoutingValidationResult: modelrouting.DefaultContract(state.SetupDepth).RoutingValidationResult,
		DeferredLanes:           append([]wizardDeferredLane{}, state.DeferredLanes...),
		NextSteps:               append([]string{}, state.NextSteps...),
		RuntimeSetup:            wizardRuntimeSetupPayload(state.RuntimeSetup),
		MemoryOutcome:           wizardMemoryOutcomePayload(state),
		GeneratedAt:             time.Now().UTC().Format(time.RFC3339),
	}

	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal wizard final artifact: %w", err)
	}

	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return fmt.Errorf("write wizard final artifact: %w", err)
	}

	return nil
}

func wizardRequiredToolIDs(state wizardState, registry *toolcheck.Registry) []string {
	resources := make([]string, 0, len(state.Packs)+len(state.Connectors)+len(state.Tools))
	resources = append(resources, state.Packs...)
	resources = append(resources, state.Connectors...)
	resources = append(resources, state.Tools...)
	return requiredToolIDsFromResources(registry, resources...)
}

func nonBlockingWarnings(results []toolcheck.Result) []toolcheck.Result {
	out := make([]toolcheck.Result, 0)
	for _, result := range results {
		if result.Status == toolcheck.StatusWarning {
			out = append(out, result)
			continue
		}
		if result.Status == toolcheck.StatusFailure && !result.Blocking() {
			result.Status = toolcheck.StatusWarning
			out = append(out, result)
		}
	}
	return out
}

func resultStrings(results []toolcheck.Result) []string {
	out := make([]string, 0, len(results))
	for _, result := range results {
		out = append(out, result.String())
	}
	return out
}

func wizardValidationScope(state wizardState, registry *toolcheck.Registry, results []toolcheck.Result) wizardValidationScopeArtifact {
	if registry == nil {
		registry = toolcheck.NewDefaultRegistry()
	}
	selected := stringSet(wizardRequiredToolIDs(state, registry))
	resultByTool := map[string]toolcheck.Result{}
	for _, result := range results {
		id := registry.CanonicalID(result.ToolID)
		if id == "" {
			id = strings.ToLower(strings.TrimSpace(result.ToolID))
		}
		if id != "" {
			resultByTool[id] = result
		}
	}

	tools := make([]wizardToolValidationArtifact, 0)
	for _, meta := range registry.List() {
		if selected[meta.ID] {
			if result, ok := resultByTool[meta.ID]; ok {
				tools = append(tools, validationArtifactFromResult(result))
				continue
			}
			tools = append(tools, wizardToolValidationArtifact{ID: meta.ID, Status: "skipped", Severity: "skipped", Reason: "selected but preflight checker was not available in this run"})
			continue
		}
		tools = append(tools, wizardToolValidationArtifact{ID: meta.ID, Status: "not_selected", Severity: "not_selected", Reason: fmt.Sprintf("%s not selected: skipped", meta.Name)})
	}
	tools = append(tools, deferredValidationTools(state)...)
	sort.Slice(tools, func(i, j int) bool { return tools[i].ID < tools[j].ID })

	return wizardValidationScopeArtifact{
		Preset:       state.SetupDepth,
		Mode:         "selected_preflight_only",
		CanContinue:  len(toolcheck.BlockingResults(results)) == 0,
		Tools:        tools,
		SummaryLines: validationSummaryLines(tools),
	}
}

func validationArtifactFromResult(result toolcheck.Result) wizardToolValidationArtifact {
	status := "pass"
	severity := "pass"
	switch result.Status {
	case toolcheck.StatusWarning:
		status = "warning"
		severity = "warning"
	case toolcheck.StatusFailure:
		if result.Blocking() {
			status = "blocker"
			severity = "blocker"
		} else {
			status = "warning"
			severity = "warning"
		}
	}
	return wizardToolValidationArtifact{ID: result.ToolID, Status: status, Severity: severity, Reason: result.String()}
}

func deferredValidationTools(state wizardState) []wizardToolValidationArtifact {
	items := []wizardToolValidationArtifact{
		{ID: "comfyui-workflows", Status: "deferred", Severity: "deferred", Reason: "ComfyUI lane deferred: not validated"},
		{ID: "provider-native-image-generation", Status: "deferred", Severity: "deferred", Reason: "Provider-native image generation deferred: not validated"},
		{ID: "audio-generation", Status: "deferred", Severity: "deferred", Reason: "Audio lane future scope: not validated"},
	}
	if normalizeSetupDepth(state.SetupDepth) == "minimal" {
		items = append(items, wizardToolValidationArtifact{ID: "visual-workflow-lanes", Status: "deferred", Severity: "deferred", Reason: "Visual workflow lanes deferred for minimal setup: not validated"})
	}
	return items
}

func wizardPreflightResult(state wizardState, results []toolcheck.Result) wizardPreflightResultArtifact {
	warnings := resultStrings(nonBlockingWarnings(results))
	blockers := resultStrings(toolcheck.BlockingResults(results))
	validated := validatedToolIDsForScope(state.ValidationScope)
	notValidated := append(skippedToolIDsForScope(state.ValidationScope), deferredToolIDsForScope(state.ValidationScope)...)
	status := "pass"
	if len(results) == 0 {
		status = "skipped"
	}
	if len(warnings) > 0 {
		status = "warning"
	}
	if len(blockers) > 0 {
		status = "blocker"
	}
	return wizardPreflightResultArtifact{Status: status, CanContinue: len(blockers) == 0, Warnings: warnings, Blockers: blockers, Validated: validated, NotValidated: notValidated}
}

// wizardRuntimeSetupVerifiedComplete reports whether this run produced actual
// complete executed runtime evidence: the runtime setup executed, was not
// marked failed, and reached the completed report status. A nil setup
// (metadata-only or preview) is not evidence, and neither is an ambient
// preflight alone.
func wizardRuntimeSetupVerifiedComplete(setup *wizardRuntimeSetupState) bool {
	if setup == nil || !setup.Executed || setup.Failed {
		return false
	}
	return setup.ReportStatus == "completed"
}

func wizardGenerationReadiness(state wizardState) wizardGenerationReadinessArtifact {
	blockers := toolcheck.BlockingResults(state.EnvBlockers)
	if len(blockers) > 0 {
		reason := blockingReason(blockers)
		blockedWorkflows := blockedWorkflowsForBlockers(blockers)
		summaryLines := []string{
			"Selected required tool validation failed.",
			"Profile metadata can still be generated.",
		}
		for _, blocker := range blockers {
			summaryLines = append(summaryLines, blockerReadinessGuidanceFor(blocker).Summary)
		}
		summaryLines = append(summaryLines, blockedWorkflowReadinessLines(blockers)...)
		summaryLines = append(summaryLines,
			"profile_generation_allowed: true (metadata-only generation)",
			fmt.Sprintf("engine_workflow_ready: false (%s blocked)", strings.Join(blockedWorkflows, ", ")),
		)
		return wizardGenerationReadinessArtifact{
			ProfileGenerationAllowed: true,
			MetadataOnlyGeneration:   true,
			EngineWorkflowReady:      false,
			BlockedWorkflows:         blockedWorkflows,
			BlockingReason:           reason,
			CanContinueMetadataOnly:  true,
			SummaryLines:             summaryLines,
		}
	}

	// Engine workflow readiness requires actual complete executed runtime
	// evidence. A passing ambient preflight, or a metadata-only run that skipped
	// prerequisite execution, is not enough.
	if wizardRuntimeSetupVerifiedComplete(state.RuntimeSetup) {
		return wizardGenerationReadinessArtifact{
			ProfileGenerationAllowed: true,
			MetadataOnlyGeneration:   true,
			EngineWorkflowReady:      true,
			CanContinueMetadataOnly:  true,
			SummaryLines: []string{
				"profile_generation_allowed: true (metadata-only profile artifacts can be generated)",
				"engine_workflow_ready: true (prerequisite runtime verified complete in this run)",
			},
		}
	}

	if state.PreflightResult.Status == "skipped" {
		return wizardGenerationReadinessArtifact{
			ProfileGenerationAllowed: true,
			MetadataOnlyGeneration:   true,
			EngineWorkflowReady:      false,
			CanContinueMetadataOnly:  true,
			SummaryLines: []string{
				"profile_generation_allowed: true (metadata-only profile artifacts can be generated)",
				"engine_workflow_ready: false (preflight validation was not run; readiness is unknown)",
			},
		}
	}

	return wizardGenerationReadinessArtifact{
		ProfileGenerationAllowed: true,
		MetadataOnlyGeneration:   true,
		EngineWorkflowReady:      false,
		CanContinueMetadataOnly:  true,
		SummaryLines: []string{
			"profile_generation_allowed: true (metadata-only profile artifacts can be generated)",
			"engine_workflow_ready: false (runtime readiness was not verified in this run)",
		},
	}
}

func blockedWorkflowReadinessLines(blockers []toolcheck.Result) []string {
	lines := make([]string, 0, 2)
	seen := map[string]bool{}
	for _, blocker := range blockers {
		if !strings.EqualFold(strings.TrimSpace(blocker.ToolID), toolcheck.GodotToolID) {
			continue
		}
		line := blockerReadinessGuidanceFor(blocker).Workflow
		if line != "" && !seen[line] {
			seen[line] = true
			lines = append(lines, line)
		}
	}
	if ids := blockerToolIDsExcept(blockers, toolcheck.GodotToolID); len(ids) > 0 {
		lines = append(lines, fmt.Sprintf("Required tool workflows are not ready until %s passes preflight validation.", strings.Join(ids, ", ")))
	}
	if len(lines) == 0 {
		lines = append(lines, fmt.Sprintf("Required tool workflows are not ready until %s passes preflight validation.", strings.Join(blockerToolIDs(blockers), ", ")))
	}
	return lines
}

func blockedWorkflowsForBlockers(blockers []toolcheck.Result) []string {
	workflows := make([]string, 0, len(blockers))
	seen := map[string]bool{}
	for _, blocker := range blockers {
		workflow := strings.ToLower(strings.TrimSpace(blocker.ToolID)) + "-workflows"
		if strings.EqualFold(strings.TrimSpace(blocker.ToolID), toolcheck.GodotToolID) {
			workflow = "godot-runtime-workflows"
		}
		if workflow == "-workflows" || seen[workflow] {
			continue
		}
		seen[workflow] = true
		workflows = append(workflows, workflow)
	}
	sort.Strings(workflows)
	return workflows
}

func blockingReason(blockers []toolcheck.Result) string {
	if len(blockers) == 0 {
		return ""
	}
	reasons := make([]string, 0, len(blockers))
	seen := map[string]bool{}
	for _, blocker := range blockers {
		reason := blockingReasonForBlocker(blocker)
		if reason == "" || seen[reason] {
			continue
		}
		seen[reason] = true
		reasons = append(reasons, reason)
	}
	return strings.Join(reasons, "; ")
}

func blockingReasonForBlocker(blocker toolcheck.Result) string {
	if strings.EqualFold(strings.TrimSpace(blocker.ToolID), toolcheck.GodotToolID) {
		if strings.TrimSpace(blocker.Reason) != "" {
			return blocker.String()
		}
		if godotBlockerLooksLikeExecutableMissing(blocker) {
			withReason := blocker
			withReason.Reason = "Godot executable not found"
			return withReason.String()
		}
	}
	return blocker.String()
}

func godotBlockerLooksLikeExecutableMissing(blocker toolcheck.Result) bool {
	reason := strings.ToLower(strings.TrimSpace(blocker.Reason))
	if reason != "" {
		if reason == "missing" || reason == "not found" {
			return true
		}
		if strings.Contains(reason, " run:") || strings.Contains(reason, " version:") || strings.Contains(reason, "could not run") {
			return false
		}
		for _, token := range []string{" lookup:", "not found on path", "executable file not found", "executable missing", "no such file", "lookpath", "path missing", "missing from path"} {
			if strings.Contains(reason, token) {
				return true
			}
		}
		return false
	}

	text := strings.ToLower(strings.Join([]string{blocker.Attempted, blocker.Remediation}, " "))
	for _, token := range []string{"not found", "no such file", "executable missing", "lookup", "lookpath", "path missing", "missing from path"} {
		if strings.Contains(text, token) {
			return true
		}
	}
	return false
}

type blockerReadinessGuidance struct {
	Summary     string
	Workflow    string
	NextStep    string
	Remediation string
}

func blockerReadinessGuidanceFor(blocker toolcheck.Result) blockerReadinessGuidance {
	toolID := strings.TrimSpace(blocker.ToolID)
	if toolID == "" {
		toolID = "selected required tool"
	}
	toolName := strings.TrimSpace(blocker.ToolName)
	if toolName == "" {
		toolName = toolLabel(toolID)
	}
	if toolName == "" {
		toolName = toolID
	}

	guidance := blockerReadinessGuidance{
		Summary:  fmt.Sprintf("%s selected: validation failed.", toolName),
		Workflow: fmt.Sprintf("Required tool workflows are not ready until %s passes preflight validation.", toolID),
		NextStep: fmt.Sprintf("Resolve required tool %s", toolID),
	}
	if strings.EqualFold(toolID, toolcheck.GodotToolID) {
		guidance.Workflow = "Godot workflows are not ready until the reported preflight failure is resolved."
		guidance.NextStep = "Resolve the reported Godot validation failure"
		reason := strings.ToLower(strings.TrimSpace(blocker.Reason))
		switch {
		case godotBlockerLooksLikeExecutableMissing(blocker):
			guidance.Summary = "Godot selected: executable missing or not found in PATH."
			guidance.Workflow = "Godot workflows are not ready until the executable lookup/PATH failure is resolved."
			guidance.NextStep = "Resolve the Godot executable lookup/PATH failure"
		case godotBlockerLooksLikeUnparseableVersion(reason):
			guidance.Summary = "Godot selected: version output parse/verification failed."
			guidance.Workflow = "Godot workflows are not ready until version output can be parsed and verified."
			guidance.NextStep = "Resolve the Godot version parse/verification failure"
		case godotBlockerLooksLikeInvalidVersion(reason):
			guidance.Summary = "Godot selected: incompatible or invalid version."
			guidance.Workflow = "Godot workflows are not ready until the incompatible or invalid version is resolved."
			guidance.NextStep = "Resolve the incompatible or invalid Godot version"
		}
	}

	if reason := strings.TrimSpace(blocker.Reason); reason != "" {
		guidance.Summary += " Checker diagnosis: " + reason
	}
	guidance.Remediation = strings.TrimSpace(blocker.Remediation)
	if guidance.Remediation == "" {
		guidance.Remediation = "Resolve the reported preflight failure"
	}
	guidance.Summary += " Checker remediation: " + guidance.Remediation
	return guidance
}

func godotBlockerLooksLikeUnparseableVersion(reason string) bool {
	for _, token := range []string{"could not parse", "cannot parse", "unparseable", "parse version output", "verify version output"} {
		if strings.Contains(reason, token) {
			return true
		}
	}
	return false
}

func godotBlockerLooksLikeInvalidVersion(reason string) bool {
	for _, token := range []string{"godot 4 is required", "incompatible version", "invalid version", "unsupported version"} {
		if strings.Contains(reason, token) {
			return true
		}
	}
	return false
}

func appendEnvironmentNextSteps(state wizardState, steps []string) []string {
	out := append([]string{}, steps...)
	blockers := toolcheck.BlockingResults(state.EnvBlockers)
	if len(blockers) == 0 {
		return out
	}
	for _, blocker := range blockers {
		guidance := blockerReadinessGuidanceFor(blocker)
		out = append(out, fmt.Sprintf("%s: %s, then rerun env-check or wizard validation.", guidance.NextStep, strings.TrimRight(guidance.Remediation, ".,;:!?")))
	}
	out = append(out, fmt.Sprintf("Use the generated metadata-only profile artifacts now, but do not run %s workflows until preflight passes.", strings.Join(blockerToolIDs(blockers), ", ")))
	return out
}

func blockerToolIDs(blockers []toolcheck.Result) []string {
	ids := make([]string, 0, len(blockers))
	seen := map[string]bool{}
	for _, blocker := range blockers {
		id := strings.ToLower(strings.TrimSpace(blocker.ToolID))
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return []string{"selected required tool"}
	}
	return ids
}

func blockerToolIDsExcept(blockers []toolcheck.Result, excludedToolID string) []string {
	filtered := make([]toolcheck.Result, 0, len(blockers))
	for _, blocker := range blockers {
		if strings.EqualFold(strings.TrimSpace(blocker.ToolID), excludedToolID) {
			continue
		}
		filtered = append(filtered, blocker)
	}
	if len(filtered) == 0 {
		return nil
	}
	return blockerToolIDs(filtered)
}

func deepValidationRequired(state wizardState) []string {
	items := []string{
		"Smoke verification after generation remains the workflow-level validation boundary.",
		"End-to-end clean Windows/macOS/Linux validation remains required before public MVP release.",
	}
	if normalizeSetupDepth(state.SetupDepth) != "minimal" {
		items = append(items, "Visual workflow metadata requires later source/production lane validation; no image/audio workflow is executed now.")
	}
	return items
}

func selectedToolIDsForScope(scope wizardValidationScopeArtifact) []string {
	return toolIDsByStatus(scope, map[string]bool{"pass": true, "warning": true, "blocker": true, "skipped": true})
}

func validatedToolIDsForScope(scope wizardValidationScopeArtifact) []string {
	return toolIDsByStatus(scope, map[string]bool{"pass": true, "warning": true, "blocker": true})
}

func skippedToolIDsForScope(scope wizardValidationScopeArtifact) []string {
	return toolIDsByStatus(scope, map[string]bool{"skipped": true, "not_selected": true})
}

func deferredToolIDsForScope(scope wizardValidationScopeArtifact) []string {
	return toolIDsByStatus(scope, map[string]bool{"deferred": true})
}

func toolIDsByStatus(scope wizardValidationScopeArtifact, statuses map[string]bool) []string {
	out := make([]string, 0)
	for _, tool := range scope.Tools {
		if statuses[tool.Status] {
			out = append(out, tool.ID)
		}
	}
	sort.Strings(out)
	return out
}

func validationSummaryLines(tools []wizardToolValidationArtifact) []string {
	lines := make([]string, 0, len(tools))
	for _, tool := range tools {
		switch tool.Status {
		case "pass", "warning", "blocker":
			lines = append(lines, fmt.Sprintf("%s selected: validating %s.", toolLabel(tool.ID), toolLabel(tool.ID)))
		case "not_selected", "skipped", "deferred":
			if tool.Reason != "" {
				lines = append(lines, tool.Reason)
			}
		}
	}
	return lines
}

func printValidationScopeSummary(state wizardState) {
	if len(state.ValidationScope.Tools) == 0 {
		return
	}
	fmt.Println("  validation_scope:")
	fmt.Printf("    mode: %s\n", state.ValidationScope.Mode)
	fmt.Printf("    can_continue: %t\n", state.ValidationScope.CanContinue)
	for _, line := range state.ValidationScope.SummaryLines {
		fmt.Printf("    - %s\n", line)
	}
}

func toolLabel(id string) string {
	switch id {
	case "godot":
		return "Godot"
	case "blender":
		return "Blender"
	case "comfyui-workflows":
		return "ComfyUI"
	default:
		return id
	}
}

func writeWizardManagedBlock(workspaceRoot, profileName, profileArtifact, finalArtifact string) error {
	const (
		begin = "# BEGIN GAME-STUDIO WIZARD (managed)"
		end   = "# END GAME-STUDIO WIZARD (managed)"
	)

	configDirRel := "openspec"
	configDir, err := resolveWizardSafePath(workspaceRoot, &configDirRel)
	if err != nil {
		return fmt.Errorf("resolve managed config target: %w", err)
	}
	path := filepath.Join(configDir, "config.yaml")
	if _, err := resolveWizardSafePath(workspaceRoot, &path); err != nil {
		return fmt.Errorf("resolve managed config target: %w", err)
	}
	if err := ensureOpenspecConfig(path); err != nil {
		return fmt.Errorf("prepare openspec config: %w", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read openspec config: %w", err)
	}

	profileArtifactForConfig, err := metadataPathValue(profileArtifact, workspaceRoot)
	if err != nil {
		return fmt.Errorf("resolve managed profile artifact path: %w", err)
	}
	finalArtifactForConfig, err := metadataPathValue(finalArtifact, workspaceRoot)
	if err != nil {
		return fmt.Errorf("resolve managed final artifact path: %w", err)
	}

	managed := fmt.Sprintf(`%s
wizard:
  profile_name: %s
  profile_artifact: %s
  final_artifact: %s
  sync_refresh_placeholder:
    status: deferred
    command_hint: "deferred placeholder: sync/refresh command is not implemented in this build"
%s`, begin, yamlQuoted(profileName), yamlQuoted(profileArtifactForConfig), yamlQuoted(finalArtifactForConfig), end)

	content := string(raw)
	start := strings.Index(content, begin)
	stop := strings.Index(content, end)
	if start != -1 && stop != -1 && stop > start {
		stop += len(end)
		prefix := strings.TrimRight(content[:start], "\n")
		suffix := strings.TrimLeft(content[stop:], "\n")
		if suffix == "" {
			content = prefix + "\n\n" + managed + "\n"
		} else {
			content = prefix + "\n\n" + managed + "\n\n" + suffix
		}
	} else {
		content = strings.TrimRight(content, "\n") + "\n\n" + managed + "\n"
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write openspec config managed block: %w", err)
	}

	return nil
}

func ensureOpenspecConfig(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create openspec directory: %w", err)
	}

	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat openspec config: %w", err)
	}

	seed := "schema: spec-driven\nstrict_tdd: false\n"
	if err := os.WriteFile(path, []byte(seed), 0o644); err != nil {
		return fmt.Errorf("bootstrap openspec config: %w", err)
	}

	return nil
}

func validateWizardProfileMarkdown(path, profile string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read generated wizard profile: %w", err)
	}
	text := string(raw)
	for _, token := range []string{fmt.Sprintf("# %s", profile), "## Providers", wizardProviderNotice, "not-inspected", "## MCP Runtime Assistants", wizardMCPNotice} {
		if !strings.Contains(text, token) {
			return fmt.Errorf("wizard profile validation failed: missing %q", token)
		}
	}
	return nil
}

func normalizeComplexity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "simple":
		return "simple"
	case "advanced", "":
		return "advanced"
	case "expert":
		return "expert"
	default:
		return ""
	}
}

func normalizeUseMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "create_from_scratch", "scratch", "from-scratch", "from scratch", "new", "":
		return "create_from_scratch"
	case "existing_game", "existing", "existing-game":
		return "existing_game"
	case "design_narrative_only", "design-narrative-only", "narrative", "narrative-only":
		return "design_narrative_only"
	case "visual_artifacts_only", "visual-artifacts-only", "visual", "artifacts-only":
		return "visual_artifacts_only"
	case "godot_sdd_handoff", "godot-sdd-handoff", "sdd", "handoff":
		return "godot_sdd_handoff"
	case "repair_change_workflow", "repair-change-workflow", "repair", "change", "repair/change":
		return "repair_change_workflow"
	default:
		return ""
	}
}

func normalizeEnginePack(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "godot-core", "godot", "godot_core", "":
		return "godot-core"
	case "unity", "unity-core", "unity_core":
		return "unity"
	case "ue5", "unreal", "unreal-engine-5", "ue5-core":
		return "ue5"
	default:
		return ""
	}
}

func enginePackMetadata(enginePack string) wizardEnginePackMetadata {
	switch normalizeEnginePack(enginePack) {
	case "unity":
		return wizardEnginePackMetadata{
			ID:                        "unity",
			Label:                     "Unity engine pack placeholder",
			Status:                    "deferred",
			Note:                      "future placeholder; active profile generation is not supported for Unity yet",
			GenerationDeferred:        true,
			ActiveGenerationSupported: false,
		}
	case "ue5":
		return wizardEnginePackMetadata{
			ID:                        "ue5",
			Label:                     "Unreal Engine 5 engine pack placeholder",
			Status:                    "deferred",
			Note:                      "future placeholder; active profile generation is not supported for Unreal Engine 5 yet",
			GenerationDeferred:        true,
			ActiveGenerationSupported: false,
		}
	default:
		return wizardEnginePackMetadata{
			ID:                        "godot-core",
			Label:                     "Godot core engine pack",
			Status:                    "active",
			Note:                      "only active engine pack for generated profiles today",
			GenerationDeferred:        false,
			ActiveGenerationSupported: true,
		}
	}
}

func normalizeSetupDepth(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "minimal", "simple":
		return "minimal"
	case "recommended", "advanced", "":
		return "recommended"
	case "full", "expert":
		return "full"
	case "custom":
		return "custom"
	default:
		return ""
	}
}

func visitedFlags(fs *flag.FlagSet) map[string]bool {
	visited := map[string]bool{}
	fs.Visit(func(flag *flag.Flag) {
		visited[flag.Name] = true
	})
	return visited
}

func resolveUseModeDefault(useMode, start string, useModeExplicit bool) string {
	if useModeExplicit {
		return useMode
	}
	if mapped := useModeForDeprecatedStart(start); mapped != "" {
		return mapped
	}
	return useMode
}

func resolveSetupDepthDefault(setupDepth, complexity string, setupDepthExplicit bool) string {
	if setupDepthExplicit {
		return setupDepth
	}
	if strings.TrimSpace(complexity) != "" && normalizeSetupDepth(complexity) != "" {
		return complexity
	}
	return setupDepth
}

func useModeForDeprecatedStart(start string) string {
	switch strings.ToLower(strings.TrimSpace(start)) {
	case "scratch":
		return "create_from_scratch"
	case "existing":
		return "existing_game"
	case "import":
		return "godot_sdd_handoff"
	default:
		return ""
	}
}

func startingPointForUseMode(useMode string) string {
	switch normalizeUseMode(useMode) {
	case "existing_game", "repair_change_workflow":
		return "existing"
	case "godot_sdd_handoff":
		return "import"
	default:
		return "scratch"
	}
}

func complexityForSetupDepth(setupDepth string) string {
	switch normalizeSetupDepth(setupDepth) {
	case "minimal":
		return "simple"
	case "full", "custom":
		return "expert"
	default:
		return "advanced"
	}
}

func defaultsForSetup(enginePack, setupDepth, useMode string) ([]string, []string, []string) {
	pack := normalizeEnginePack(enginePack)
	if pack == "" || !enginePackMetadata(pack).ActiveGenerationSupported {
		pack = "godot-core"
	}

	connectors := []string{"godot-docs"}
	packs := []string{pack}
	tools := []string{"formatter"}

	switch normalizeSetupDepth(setupDepth) {
	case "minimal":
		return connectors, packs, tools
	case "full":
		connectors = append(connectors, "asset-store")
		tools = append(tools, "linter")
	case "custom":
		connectors = append(connectors, "asset-store")
	}

	if normalizeUseMode(useMode) == "visual_artifacts_only" {
		connectors = append(connectors, "visual-workflow")
	}

	return connectors, packs, tools
}

func normalizeStart(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "scratch", "from-scratch", "from scratch", "":
		return "scratch"
	case "existing":
		return "existing"
	case "import", "import-later", "import later":
		return "import"
	default:
		return ""
	}
}

func isYes(value string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	return trimmed == "yes" || trimmed == "y"
}

func formatOptionalIntegrationsMarkdown(selections []integrations.OptionalIntegrationSelection) string {
	if len(selections) == 0 {
		return "- selected: none\n- note: metadata only; no install, clone, build, run, start, or stop actions are performed."
	}
	registry := integrations.NewDefaultRegistry()
	lines := []string{"- note: metadata only; no install, clone, build, run, start, or stop actions are performed."}
	for _, selection := range selections {
		item, ok := registry.Get(selection.ID)
		label := selection.ID
		if ok {
			label = item.Label
		}
		lines = append(lines, fmt.Sprintf("- %s (%s): selected", selection.ID, label))
		for _, key := range sortedBoolKeys(selection.Consent) {
			lines = append(lines, fmt.Sprintf("  - consent.%s: %t", key, selection.Consent[key]))
		}
		if ok {
			for _, hint := range item.Hints {
				lines = append(lines, fmt.Sprintf("  - hint: %s", hint))
			}
		}
	}
	return strings.Join(lines, "\n")
}

func formatWorkflowAdaptersMarkdown(adapters []assets.AdapterCapability) string {
	if len(adapters) == 0 {
		return "- selected: none\n- note: visual workflow adapters are separate from generic optionals; no execution is performed."
	}
	lines := []string{"- note: metadata only; no install, clone, build, run, API call, model download, asset generation, or DCC/Godot mutation is performed."}
	for _, adapter := range adapters {
		lines = append(lines, fmt.Sprintf("- %s (%s): workflow_adapter selected", adapter.ID, adapter.Label))
		for _, limitation := range adapter.Limitations {
			lines = append(lines, fmt.Sprintf("  - limitation: %s", limitation))
		}
	}
	return strings.Join(lines, "\n")
}

func defaultDeferredLanes() []wizardDeferredLane {
	return []wizardDeferredLane{
		{ID: "tool-detection-ux", Status: "deferred", Note: "#29 owns deeper detection/remediation UX; this wizard validates selected essentials only"},
		{ID: "visual-source-production-lanes", Status: "deferred", Note: "#32 owns full source/production lane separation"},
		{ID: "provider-native-image-generation", Status: "deferred", Note: "#30 owns provider-native image generation; no images are generated here"},
	}
}

func presetIntegrationDefaults(setupDepth string, items []integrations.Integration) []string {
	if normalizeSetupDepth(setupDepth) != "full" {
		return []string{}
	}
	selected := make([]string, 0)
	for _, item := range items {
		if len(item.Consent) > 0 {
			continue
		}
		category := taxonomyCategoryForIntegration(item)
		if category == "optional" || category == "workflow_adapter" {
			selected = append(selected, item.ID)
		}
	}
	sort.Strings(selected)
	return selected
}

func taxonomyCategoryForIntegration(item integrations.Integration) string {
	if len(item.Consent) > 0 {
		return "consent_sensitive"
	}
	if item.AssetAdapter != nil {
		return "workflow_adapter"
	}
	category := strings.TrimSpace(item.Category)
	if category == "" || category == "asset-adapter" || category == "workflow-adapter" {
		return "unknown_optional"
	}
	return "optional"
}

func consentArtifactID(integrationID, requirementID string) string {
	return integrationID + ":" + requirementID
}

func deferredConsentIDs(items []integrations.Integration, selected map[string]bool) []string {
	deferred := make([]string, 0)
	for _, item := range items {
		if len(item.Consent) == 0 || selected[item.ID] {
			continue
		}
		for _, requirement := range item.Consent {
			deferred = append(deferred, consentArtifactID(item.ID, requirement.ID))
		}
	}
	sort.Strings(deferred)
	return deferred
}

func selectedCoreForArtifact(state wizardState) []wizardTaxonomyItem {
	enginePack := enginePackMetadata(state.EnginePack)
	return []wizardTaxonomyItem{
		{ID: "game-studio-profile", Label: state.ProfileName, Category: "core", Status: "selected", Note: "primary staged Game-Studio profile metadata (Pi only)"},
		{ID: enginePack.ID, Label: enginePack.Label, Category: "core", Status: enginePack.Status, Note: enginePack.Note},
		{ID: "core-game-workflow/v1", Label: "Core game workflow", Category: "core", Status: "selected", Note: "draft/pending human approval only"},
		{ID: "model-routing/v1", Label: "Model routing contract", Category: "core", Status: "selected", Note: "capability routing metadata only; no provider execution"},
		{ID: "sdd-handoff", Label: "SDD handoff metadata", Category: "core", Status: "selected"},
		{ID: "smoke-verify", Label: "Smoke/verify checks", Category: "core", Status: "selected"},
	}
}

func wizardOptionalIntegrationsPayload(selections []integrations.OptionalIntegrationSelection, catalog integrations.Catalog) wizardOptionalIntegrationsArtifact {
	selected := make([]integrations.OptionalIntegrationSelection, 0, len(selections))
	unknown := make([]wizardTaxonomyIntegration, 0)
	for _, selection := range cloneOptionalSelections(selections) {
		item, ok := catalog.Get(selection.ID)
		if ok && taxonomyCategoryForIntegration(item) == "unknown_optional" || !ok {
			unknown = append(unknown, wizardTaxonomyIntegration{ID: selection.ID, Label: selection.ID, Category: "unknown_optional", Status: "deferred_review", Boundary: "insufficient metadata to classify safely"})
			continue
		}
		selected = append(selected, selection)
	}
	return wizardOptionalIntegrationsArtifact{Selected: selected, UnknownOptional: unknown}
}

func consentSensitivePayload(items []integrations.Integration, state wizardState) []wizardConsentSensitiveArtifact {
	accepted := stringSet(state.AcceptedConsents)
	rejected := stringSet(state.RejectedConsents)
	out := make([]wizardConsentSensitiveArtifact, 0)
	for _, item := range items {
		if len(item.Consent) == 0 {
			continue
		}
		requirements := make([]string, 0, len(item.Consent))
		status := "deferred_consent"
		for _, requirement := range item.Consent {
			id := consentArtifactID(item.ID, requirement.ID)
			requirements = append(requirements, requirement.ID)
			if accepted[id] {
				status = "accepted"
			}
			if rejected[id] {
				status = "rejected"
			}
		}
		out = append(out, wizardConsentSensitiveArtifact{ID: item.ID, Label: item.Label, Category: "consent_sensitive", Requirements: requirements, Status: status, Boundary: "requires explicit human consent; never auto-accepted by presets"})
	}
	return out
}

func workflowAdaptersPayload(adapters []assets.AdapterCapability) []wizardTaxonomyIntegration {
	out := make([]wizardTaxonomyIntegration, 0, len(adapters))
	for _, adapter := range adapters {
		out = append(out, wizardTaxonomyIntegration{ID: adapter.ID, Label: adapter.Label, Category: "workflow_adapter", Status: "metadata_selected", Boundary: "metadata only; no install, model download, API call, asset generation, or DCC/Godot mutation"})
	}
	return out
}

func visualWorkflowLanesPayload(state wizardState) []wizardVisualWorkflowLane {
	status := "deferred"
	if normalizeSetupDepth(state.SetupDepth) != "minimal" {
		status = "metadata_selected"
	}
	lanes := []wizardVisualWorkflowLane{
		{ID: "art-bible", Label: "Art Bible metadata", Category: "visual_workflow_lane", Status: status, Boundary: "human-authored metadata; pending approval; no generation"},
		{ID: "asset-spec", Label: "Asset Spec metadata", Category: "visual_workflow_lane", Status: status, Boundary: "specification metadata; no image/audio/model generation"},
		{ID: "visual-readiness-audit", Label: "Visual readiness/audit", Category: "visual_workflow_lane", Status: status, Boundary: "read-only guidance; no tool execution"},
	}
	for _, adapter := range state.AssetPipeline.SelectedAdapters {
		lanes = append(lanes, wizardVisualWorkflowLane{ID: adapter.ID, Label: adapter.Label, Category: "workflow_adapter", Status: "metadata_selected", Boundary: "adapter preference only; execution deferred"})
	}
	return lanes
}

func stringSet(items []string) map[string]bool {
	out := map[string]bool{}
	for _, item := range items {
		out[item] = true
	}
	return out
}

func defaultNextSteps(state wizardState) []string {
	steps := []string{
		"Open the generated profile artifact and confirm the staged setup metadata.",
		"Use core-game-workflow/v1 to draft game intent or a repair/change brief; keep approval pending until a human approves it.",
		"Run smoke validation after generation when changing profile artifacts.",
	}
	if normalizeUseMode(state.UseMode) == "visual_artifacts_only" {
		steps = append(steps, "Start with Art Bible / Asset Spec guidance before any future image or adapter workflow.")
	}
	if normalizeUseMode(state.UseMode) == "repair_change_workflow" {
		steps = append(steps, "Create a repair/change brief with acceptance criteria before SDD/Godot handoff.")
	}
	return steps
}

func formatDeferredLanesMarkdown(lanes []wizardDeferredLane) string {
	if len(lanes) == 0 {
		return "- none"
	}
	lines := make([]string, 0, len(lanes))
	for _, lane := range lanes {
		lines = append(lines, fmt.Sprintf("- %s: %s — %s", lane.ID, lane.Status, lane.Note))
	}
	return strings.Join(lines, "\n")
}

func formatNextStepsMarkdown(steps []string) string {
	if len(steps) == 0 {
		return "- Review generated profile artifact."
	}
	lines := make([]string, 0, len(steps))
	for _, step := range steps {
		lines = append(lines, "- "+step)
	}
	return strings.Join(lines, "\n")
}

func selectedAssetAdapters(selections []integrations.OptionalIntegrationSelection, catalog integrations.Catalog) []assets.AdapterCapability {
	if catalog == nil {
		catalog = integrations.NewDefaultRegistry()
	}
	out := make([]assets.AdapterCapability, 0)
	for _, selection := range selections {
		item, ok := catalog.Get(selection.ID)
		if !ok || item.AssetAdapter == nil {
			continue
		}
		out = append(out, *item.AssetAdapter)
	}
	return out
}

func wizardAssetPipelinePayload(state wizardAssetPipelineState) wizardAssetPipelineArtifact {
	data := defaultAssetPipelineProfileData(state.SelectedAdapters)
	intent := strings.TrimSpace(state.Intent)
	if intent == "" {
		intent = "metadata-only visual asset preferences"
	}
	return wizardAssetPipelineArtifact{
		Intent:           intent,
		ContractVersion:  data.ContractVersion,
		Boundary:         data.Boundary,
		SelectedAdapters: data.Selected,
		FutureScope:      data.FutureScope,
		DeferredRequests: append([]string{}, state.DeferredRequests...),
		NoExecutionClaims: map[string]bool{
			"tools_installed":       false,
			"models_downloaded":     false,
			"apis_called":           false,
			"assets_generated":      false,
			"godot_files_mutated":   false,
			"blender_files_mutated": false,
		},
	}
}

func wizardVisualWorkflowPayload() wizardVisualWorkflowArtifact {
	data := defaultVisualWorkflowProfileData()
	contract := data.Contract
	return wizardVisualWorkflowArtifact{
		Intent:            "metadata-only art direction, asset specification, and readiness guidance",
		ContractVersion:   contract.Version,
		Boundary:          contract.Boundary,
		Paths:             contract.Paths,
		ArtBible:          contract.ArtBible,
		AssetSpec:         contract.AssetSpec,
		Readiness:         contract.Readiness,
		Audit:             contract.Audit,
		Gates:             append([]assets.WorkflowGate{}, contract.Gates...),
		GuidanceRoutes:    append([]string{}, data.GuidanceRoutes...),
		NoExecutionClaims: cloneBoolMap(data.NoClaims),
	}
}

func wizardCoreGameWorkflowPayload() wizardCoreGameWorkflowArtifact {
	data := defaultCoreGameWorkflowProfileData()
	contract := data.Contract
	return wizardCoreGameWorkflowArtifact{
		ContractVersion:       contract.Version,
		Boundary:              contract.Boundary,
		SupportedModes:        append([]string{}, contract.SupportedModes...),
		ModeContracts:         append([]coregame.ModeContract{}, contract.ModeContracts...),
		PhaseIDs:              append([]coregame.PhaseID{}, contract.PhaseIDs...),
		NarrativeModes:        append([]coregame.NarrativeMode{}, contract.NarrativeModes...),
		RepairClassifications: append([]coregame.RepairClassification{}, contract.RepairClassifications...),
		ApprovalPolicy:        contract.ApprovalPolicy,
		BriefContracts:        append([]coregame.BriefContract{}, contract.BriefContracts...),
		MarkdownTemplates:     append([]coregame.MarkdownTemplate{}, data.Templates...),
		RepairHandoffFlow:     append([]string{}, contract.RepairHandoffFlow...),
		DownstreamReferences:  append([]coregame.DownstreamReference{}, contract.DownstreamReferences...),
		NoExecutionClaims:     cloneBoolMap(contract.NoExecutionClaims),
	}
}

func wizardModelRoutingPayload(preset resolvedPreset) wizardModelRoutingArtifact {
	contract := modelrouting.DefaultContract(preset.RoutingPreset)
	return wizardModelRoutingArtifact{
		Version:                          contract.Version,
		Boundary:                         contract.Boundary,
		CoreWorkflowVersion:              contract.CoreWorkflowVersion,
		SetupPreset:                      preset.SetupPreset,
		PresetSource:                     preset.PresetSource,
		RoutingPreset:                    contract.RoutingPreset,
		RoutingPresetSource:              preset.RoutingPresetSource,
		Capabilities:                     append([]modelrouting.Capability{}, contract.Capabilities...),
		PhaseRoutes:                      append([]modelrouting.PhaseRoute{}, contract.PhaseRoutes...),
		ProviderModelBindings:            append([]modelrouting.ProviderBinding{}, contract.ProviderModelBindings...),
		CapabilityOverrides:              append([]modelrouting.CapabilityOverride{}, contract.CapabilityOverrides...),
		PhaseOverrides:                   append([]modelrouting.PhaseOverride{}, contract.PhaseOverrides...),
		OverrideStatus:                   contract.OverrideStatus,
		ValidationResult:                 contract.ValidationResult,
		UnconfiguredRequiredCapabilities: append([]modelrouting.CapabilityID{}, contract.UnconfiguredRequiredCapabilities...),
		UnconfiguredOptionalCapabilities: append([]modelrouting.CapabilityID{}, contract.UnconfiguredOptionalCapabilities...),
		OptionalCapabilities:             append([]modelrouting.CapabilityID{}, contract.OptionalCapabilities...),
		FutureCapabilities:               append([]modelrouting.CapabilityID{}, contract.FutureCapabilities...),
		DeferredCapabilities:             append([]modelrouting.CapabilityID{}, contract.DeferredCapabilities...),
		RoutingValidationResult:          contract.RoutingValidationResult,
		NoExecutionClaims:                cloneBoolMap(contract.NoExecutionClaims),
	}
}

func requiredCapabilityIDs(items []modelrouting.Capability) []modelrouting.CapabilityID {
	out := make([]modelrouting.CapabilityID, 0)
	for _, item := range items {
		if item.Requirement == "required" && item.Selected {
			out = append(out, item.ID)
		}
	}
	return out
}

func optionalCapabilityIDs(items []modelrouting.Capability) []modelrouting.CapabilityID {
	out := make([]modelrouting.CapabilityID, 0)
	for _, item := range items {
		if item.Requirement == "optional" {
			out = append(out, item.ID)
		}
	}
	return out
}

func futureCapabilityIDs(items []modelrouting.Capability) []modelrouting.CapabilityID {
	out := make([]modelrouting.CapabilityID, 0)
	for _, item := range items {
		if item.Requirement == "future" {
			out = append(out, item.ID)
		}
	}
	return out
}

func formatListOrNone(items []string) string {
	if len(items) == 0 {
		return "none"
	}
	return strings.Join(items, ", ")
}

func formatCapabilityIDs(items []modelrouting.CapabilityID) string {
	if len(items) == 0 {
		return "none"
	}
	values := make([]string, 0, len(items))
	for _, item := range items {
		values = append(values, string(item))
	}
	return strings.Join(values, ", ")
}

func joinPhaseIDs(items []coregame.PhaseID) string {
	values := make([]string, 0, len(items))
	for _, item := range items {
		values = append(values, string(item))
	}
	return strings.Join(values, ", ")
}

func joinNarrativeModes(items []coregame.NarrativeMode) string {
	values := make([]string, 0, len(items))
	for _, item := range items {
		values = append(values, string(item))
	}
	return strings.Join(values, ", ")
}

func cloneBoolMap(items map[string]bool) map[string]bool {
	out := map[string]bool{}
	for key, value := range items {
		out[key] = value
	}
	return out
}

func cloneOptionalSelections(selections []integrations.OptionalIntegrationSelection) []integrations.OptionalIntegrationSelection {
	out := make([]integrations.OptionalIntegrationSelection, 0, len(selections))
	for _, selection := range selections {
		cloned := integrations.OptionalIntegrationSelection{ID: selection.ID}
		if len(selection.Consent) > 0 {
			cloned.Consent = map[string]bool{}
			for key, value := range selection.Consent {
				cloned.Consent[key] = value
			}
		}
		out = append(out, cloned)
	}
	return out
}

func sortedBoolKeys(items map[string]bool) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func promptValue(reader *bufio.Reader, nonInteractive bool, label, fallback string) (string, error) {
	if nonInteractive {
		return fallback, nil
	}

	fmt.Printf("%s [%s]: ", label, fallback)
	raw, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("read wizard input: %w", err)
	}
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback, nil
	}
	return value, nil
}

func splitCSV(value string) []string {
	parts := strings.Split(strings.TrimSpace(value), ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		normalized := strings.TrimSpace(part)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func resolveWizardOutputPaths(workspaceRoot string, outDir, finalArtifactPath string) (string, string, error) {
	resolvedOutDir, err := resolveWizardSafePath(workspaceRoot, &outDir)
	if err != nil {
		return "", "", fmt.Errorf("validate --out-dir: %w", err)
	}

	resolvedFinalArtifactPath, err := resolveWizardSafePath(workspaceRoot, &finalArtifactPath)
	if err != nil {
		return "", "", fmt.Errorf("validate --final-artifact: %w", err)
	}

	return resolvedOutDir, resolvedFinalArtifactPath, nil
}

func validateWizardOutputPaths(workspaceRoot string, outDir *string, finalArtifactPath *string) error {
	_, _, err := resolveWizardOutputPaths(workspaceRoot, *outDir, *finalArtifactPath)
	return err
}

func resolveWizardSafePath(workspaceRoot string, path *string) (string, error) {
	if path == nil || strings.TrimSpace(*path) == "" {
		return "", fmt.Errorf("path is required")
	}

	resolved := resolveWorkspacePath(workspaceRoot, *path)
	clean := filepath.Clean(resolved)
	canonical, err := resolveWithSymlinkBoundary(workspaceRoot, clean)
	if err != nil {
		return "", fmt.Errorf("resolve workspace-relative path: %w", err)
	}
	if clean == "." {
		return "", fmt.Errorf("path resolves to workspace root")
	}

	physicalRoot, err := resolvePhysicalPath(filepath.Clean(workspaceRoot))
	if err != nil {
		return "", fmt.Errorf("resolve workspace root: %w", err)
	}
	rel, err := filepath.Rel(physicalRoot, canonical)
	if err != nil {
		return "", fmt.Errorf("resolve workspace-relative path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q is outside workspace root", *path)
	}

	return canonical, nil
}

// resolveWizardProfileTarget applies the profile leaf policy to the original
// lexical path before resolving symlinks, then returns the physically resolved,
// workspace-contained profile path. Checking the lexical leaf first is required:
// resolving a leaf symlink first would report its target as a regular file and
// erase the symlink identity the writer must refuse. Ancestor directory
// symlinks remain supported through the containment check.
func resolveWizardProfileTarget(workspaceRoot, profileRel string) (string, error) {
	lexical := filepath.Clean(resolveWorkspacePath(workspaceRoot, profileRel))
	if _, err := inspectWizardProfileLeaf(lexical); err != nil {
		return "", err
	}

	target := profileRel
	physical, err := resolveWizardSafePath(workspaceRoot, &target)
	if err != nil {
		return "", err
	}
	return physical, nil
}

// resolveWithSymlinkBoundary resolves every existing symlink in a path's
// ancestor chain, then appends the still-missing suffix verbatim. It compares
// the physical workspace root against the physical destination so a symlinked
// ancestor that points outside the workspace is rejected even when the deeper
// target already exists on disk.
//
// The check reflects the filesystem state at resolution time; it does not make
// a later write race-free, and it never normalizes an escaping path into a safe
// one.
func resolveWithSymlinkBoundary(workspaceRoot, rawPath string) (string, error) {
	clean := filepath.Clean(rawPath)
	physicalRoot, err := resolvePhysicalPath(filepath.Clean(workspaceRoot))
	if err != nil {
		return "", fmt.Errorf("resolve workspace root: %w", err)
	}

	physicalPath, err := resolvePhysicalPath(clean)
	if err != nil {
		return "", err
	}

	rel, err := filepath.Rel(physicalRoot, physicalPath)
	if err != nil {
		return "", fmt.Errorf("resolve workspace-relative path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q is outside workspace root", rawPath)
	}

	return physicalPath, nil
}

// resolvePhysicalPath returns the physical absolute path for path, following
// symlinks in every existing ancestor, and re-appends any suffix that does not
// exist yet. A dangling or looping symlink in the chain is refused rather than
// silently treated as a regular missing path.
func resolvePhysicalPath(path string) (string, error) {
	clean := filepath.Clean(path)
	var missingTail []string

	base := clean
	for {
		resolved, err := filepath.EvalSymlinks(base)
		if err == nil {
			full := resolved
			for i := len(missingTail) - 1; i >= 0; i-- {
				full = filepath.Join(full, missingTail[i])
			}
			return filepath.Clean(full), nil
		}

		info, lstatErr := os.Lstat(base)
		switch {
		case lstatErr == nil && info.Mode()&os.ModeSymlink != 0:
			return "", fmt.Errorf("resolve symlink target: %w", err)
		case lstatErr == nil && !info.IsDir() && len(missingTail) > 0:
			return "", fmt.Errorf("path %q cannot be traversed through non-directory %q", path, base)
		case lstatErr == nil:
			return "", fmt.Errorf("resolve path %q: %w", path, err)
		case !os.IsNotExist(lstatErr):
			return "", fmt.Errorf("stat path during symlink-safe resolution: %w", lstatErr)
		}

		parent := filepath.Dir(base)
		if parent == base {
			return "", fmt.Errorf("unable to resolve path %q", path)
		}
		missingTail = append(missingTail, filepath.Base(base))
		base = parent
	}
}

func sanitizeProfileName(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "Game-Studio"
	}

	filtered := make([]rune, 0, len(trimmed))
	for _, r := range trimmed {
		switch {
		case r == '\\':
			filtered = append(filtered, '-')
		case r == '"':
			filtered = append(filtered, '\'')
		case r == '\n' || r == '\r' || r == '\t':
			filtered = append(filtered, ' ')
		case r < 0x20 || r == 0x7f:
			continue
		default:
			filtered = append(filtered, r)
		}
	}

	normalized := strings.Join(strings.Fields(string(filtered)), " ")
	if normalized == "" {
		return "Game-Studio"
	}

	return normalized
}

func splitProviderModel(value string) routing.Selection {
	parts := strings.SplitN(strings.TrimSpace(value), ":", 2)
	if len(parts) != 2 {
		return routing.Selection{}
	}
	return routing.Selection{Provider: strings.TrimSpace(parts[0]), Model: strings.TrimSpace(parts[1])}
}

func parseOverride(raw string) (string, routing.Selection, error) {
	keyValue := strings.SplitN(strings.TrimSpace(raw), "=", 2)
	if len(keyValue) != 2 {
		return "", routing.Selection{}, fmt.Errorf("expected key=provider:model")
	}
	selection := splitProviderModel(keyValue[1])
	if strings.TrimSpace(selection.Provider) == "" || strings.TrimSpace(selection.Model) == "" {
		return "", routing.Selection{}, fmt.Errorf("provider/model cannot be empty")
	}
	return strings.TrimSpace(keyValue[0]), selection, nil
}

func sanitizeProfileFileName(profile string) string {
	normalized := strings.ToLower(strings.TrimSpace(profile))
	var builder strings.Builder
	lastWasSeparator := false
	for _, r := range normalized {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
			lastWasSeparator = false
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastWasSeparator = false
		case r == ' ' || r == '_' || r == '-' || r == '.':
			if builder.Len() == 0 || lastWasSeparator {
				continue
			}
			builder.WriteRune('-')
			lastWasSeparator = true
		}
	}

	clean := builder.String()
	clean = strings.Trim(clean, "-")

	if clean == "" {
		return "game-studio"
	}

	for strings.Contains(clean, "--") {
		clean = strings.ReplaceAll(clean, "--", "-")
	}

	return clean
}

func snapshotProviders(snapshot opencode.ModelSnapshot) []string {
	out := make([]string, 0, len(snapshot.Providers))
	for _, provider := range snapshot.Providers {
		out = append(out, provider.Provider)
	}
	return out
}

func detectWorkspaceRoot(start string) string {
	current := filepath.Clean(start)
	for {
		markers := []string{
			filepath.Join(current, ".game-studio", "workspace.manifest.json"),
			filepath.Join(current, filepath.FromSlash(workspaceConfigRelativePath)),
			filepath.Join(current, "openspec", "config.yaml"),
		}
		for _, marker := range markers {
			if _, err := os.Stat(marker); err == nil {
				return current
			}
		}

		parent := filepath.Dir(current)
		if parent == current {
			return filepath.Clean(start)
		}
		current = parent
	}
}

func resolveWorkspacePath(workspaceRoot, value string) string {
	clean := filepath.Clean(strings.TrimSpace(value))
	if clean == "" {
		return workspaceRoot
	}
	if filepath.IsAbs(clean) {
		return clean
	}
	return filepath.Join(workspaceRoot, clean)
}

func metadataPathValue(value, workspaceRoot string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("metadata path is empty")
	}

	clean := filepath.Clean(trimmed)
	if clean == "" {
		return "", fmt.Errorf("metadata path is empty")
	}
	if clean == "." {
		return "", fmt.Errorf("metadata path resolves to workspace root")
	}

	if filepath.IsAbs(clean) {
		rel, err := filepath.Rel(workspaceRoot, clean)
		if err != nil {
			return "", fmt.Errorf("resolve workspace-relative path: %w", err)
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("metadata path %q is outside workspace root", clean)
		}
		clean = rel
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("metadata path %q is outside workspace root", clean)
	}

	return filepath.ToSlash(clean), nil
}

func yamlQuoted(value string) string {
	sanitized := strings.ReplaceAll(value, "\r", " ")
	sanitized = strings.ReplaceAll(sanitized, "\n", " ")
	return strconv.Quote(sanitized)
}

func tierDefaultsForArtifact(policy routing.Policy) map[string]string {
	out := map[string]string{}
	for tier, selection := range policy.TierDefaults {
		out[string(tier)] = formatSelection(selection)
	}
	return out
}

func selectionMapForArtifact(items map[string]routing.Selection) map[string]string {
	out := map[string]string{}
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		out[key] = formatSelection(items[key])
	}
	return out
}

func formatSelection(selection routing.Selection) string {
	return fmt.Sprintf("%s:%s", selection.Provider, selection.Model)
}
