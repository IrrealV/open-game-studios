package commands

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"

	"open-game-studios/internal/piinstall"
	"open-game-studios/internal/toolcheck"
)

// wizardTuiStage is one page of the read-only preview journey.
type wizardTuiStage int

const (
	wizardTuiStageWelcome wizardTuiStage = iota
	wizardTuiStageUseMode
	wizardTuiStageEngine
	wizardTuiStageSetupDepth
	wizardTuiStageProfile
	wizardTuiStageDetection
	wizardTuiStagePlan
	wizardTuiStageFinish
)

func (s wizardTuiStage) title() string {
	switch s {
	case wizardTuiStageWelcome:
		return "Start"
	case wizardTuiStageUseMode:
		return "Workflow"
	case wizardTuiStageEngine:
		return "Engine context"
	case wizardTuiStageSetupDepth:
		return "Setup depth"
	case wizardTuiStageProfile:
		return "Profile name"
	case wizardTuiStageDetection:
		return "Detection"
	case wizardTuiStagePlan:
		return "Prerequisite plan"
	case wizardTuiStageFinish:
		return "Preview complete"
	default:
		return ""
	}
}

// wizardTuiStatus is the detection/plan preparation state. Error and blocked are
// distinct so a failed probe never renders as ready.
type wizardTuiStatus int

const (
	wizardTuiStatusIdle wizardTuiStatus = iota
	wizardTuiStatusLoading
	wizardTuiStatusReady
	wizardTuiStatusError
	wizardTuiStatusBlocked
)

// wizardTuiPrepareMsg carries one asynchronous plan preparation result. The
// generation ties it to the snapshot that requested it.
type wizardTuiPrepareMsg struct {
	generation int
	preflight  []string
	plan       piinstall.Plan
	err        error
	blocked    bool
}

// wizardTuiProgram runs the preview and returns the final model. The production
// implementation owns an alt-screen Bubble Tea program; tests inject a
// deterministic driver without swapping stdio.
type wizardTuiProgram func(ctx context.Context, model wizardTuiModel) (wizardTuiModel, error)

// wizardTuiOptions carries the run-scoped inputs the preview needs without
// reaching back into package globals.
type wizardTuiOptions struct {
	workspaceRoot     string
	outDir            string
	finalArtifactPath string
	state             wizardState
}

// wizardTuiModel is the whole preview state. Update returns a copy, so no
// shared mutable state exists between runs or tests.
type wizardTuiModel struct {
	ctx     context.Context
	input   WizardInput
	options wizardTuiOptions

	stage    wizardTuiStage
	status   wizardTuiStatus
	cursor   int
	quit     bool
	canceled bool

	width  int
	height int

	useModeChoices []wizardTuiChoice
	engineChoices  []wizardTuiChoice
	depthChoices   []wizardTuiChoice

	state      wizardState
	profile    textinput.Model
	spinner    spinner.Model
	viewport   viewport.Model
	inputError string

	snapshot   wizardTuiSnapshot
	generation int
	cancel     context.CancelFunc
	plan       piinstall.Plan
	preflight  []string
	prepareErr error
	details    bool
}

func newWizardTuiModel(ctx context.Context, input WizardInput, options wizardTuiOptions) wizardTuiModel {
	profile := textinput.New()
	profile.Placeholder = "Game-Studio"
	profile.CharLimit = 64
	profile.Width = 40
	profile.SetValue(sanitizeProfileName(options.state.ProfileName))
	profile.CursorEnd()
	profile.Focus()

	spin := spinner.New()
	spin.Spinner = spinner.Dot

	model := wizardTuiModel{
		ctx:            ctx,
		input:          input,
		options:        options,
		stage:          wizardTuiStageWelcome,
		status:         wizardTuiStatusIdle,
		profile:        profile,
		spinner:        spin,
		useModeChoices: wizardTuiUseModeChoices(),
		engineChoices:  wizardTuiEngineChoices(),
		depthChoices:   wizardTuiDepthChoices(),
		width:          80,
		height:         24,
	}
	model.state = options.state
	model.state.UseMode = normalizeUseMode(options.state.UseMode)
	model.state.EnginePack = normalizeEnginePack(options.state.EnginePack)
	model.state.SetupDepth = normalizeSetupDepth(options.state.SetupDepth)
	model.state.ProfileName = sanitizeProfileName(options.state.ProfileName)
	model.cursor = 0
	model.resize(model.width, model.height)
	return model
}

// wizardTuiEnabled is the pure routing gate: an explicit plan-only preview on an
// interactive terminal. Non-interactive, metadata, and non-TTY runs keep the
// existing prompt/report behavior untouched.
func wizardTuiEnabled(planOnly, nonInteractive, stdinTTY, stdoutTTY bool) bool {
	return planOnly && !nonInteractive && stdinTTY && stdoutTTY
}

// wizardTuiTerminalCheck resolves the per-input terminal-detection seam.
func wizardTuiTerminalCheck(input WizardInput) func(*os.File) bool {
	if input.IsTerminal != nil {
		return input.IsTerminal
	}
	return func(file *os.File) bool {
		return term.IsTerminal(file.Fd())
	}
}

// runWizardTuiPreview owns the preview program lifecycle. It never approves or
// executes a plan, and it only prints guidance after a graceful completion.
func runWizardTuiPreview(ctx context.Context, input WizardInput, options wizardTuiOptions) error {
	model := newWizardTuiModel(ctx, input, options)
	program := input.TuiProgram
	if program == nil {
		program = defaultWizardTuiProgram
	}
	final, err := program(ctx, model)
	if err != nil {
		return err
	}
	if final.canceled {
		return nil
	}
	if final.stage == wizardTuiStageFinish {
		printWizardTuiPreviewSummary(final)
		if final.plan.Blocked() {
			return errWizardRuntimeBlocked
		}
		return nil
	}
	if final.prepareErr != nil {
		return final.prepareErr
	}
	if final.plan.Blocked() {
		return errWizardRuntimeBlocked
	}
	return nil
}

func defaultWizardTuiProgram(_ context.Context, model wizardTuiModel) (wizardTuiModel, error) {
	program := tea.NewProgram(model, tea.WithAltScreen())
	final, err := program.Run()
	if err != nil {
		return model, err
	}
	if finalized, ok := final.(wizardTuiModel); ok {
		return finalized, nil
	}
	return model, nil
}

// printWizardTuiPreviewSummary renders the truthful completion summary from the
// final choices. It prints a plan-only replay only; it never prints an approval
// or execute command.
func printWizardTuiPreviewSummary(model wizardTuiModel) {
	snapshot := model.snapshot
	fmt.Println("[wizard] preview complete; no runtime, config, artifact, or memory writes were made")
	fmt.Printf("  fingerprint: %s\n", model.plan.Fingerprint())
	fmt.Printf("  status: %s\n", wizardPlanStatus(model.plan))
	fmt.Printf("  run this read-only preview from the workspace root: %s\n", model.options.workspaceRoot)
	args := wizardTuiReplayArgs(snapshot, model.options.outDir, model.options.finalArtifactPath)
	fmt.Printf("  reproducible plan-only preview: %s\n", wizardReplayCommand(args, "--plan-only"))
	fmt.Println("  installation is not available from this preview; it never approves or executes a plan.")
}

func (m wizardTuiModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

func (m wizardTuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		m.resize(typed.Width, typed.Height)
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(typed)
		return m, cmd
	case wizardTuiPrepareMsg:
		return m.handlePrepare(typed)
	case tea.KeyMsg:
		return m.handleKey(typed)
	default:
		return m, nil
	}
}

func (m wizardTuiModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "ctrl+c" {
		m.canceled = true
		return m.shutdown()
	}
	// The profile field owns every other key, including 'q', so a typed name is
	// never mistaken for a quit shortcut.
	if m.stage == wizardTuiStageProfile {
		return m.handleProfileKey(msg)
	}
	if key == "q" {
		return m.shutdown()
	}
	switch m.stage {
	case wizardTuiStageWelcome:
		return m.handleWelcomeKey(key)
	case wizardTuiStageUseMode, wizardTuiStageEngine, wizardTuiStageSetupDepth:
		return m.handleChoiceKey(key)
	case wizardTuiStageDetection:
		return m.handleDetectionKey(key)
	case wizardTuiStagePlan:
		return m.handlePlanKey(key)
	case wizardTuiStageFinish:
		return m.handleFinishKey(key)
	}
	return m, nil
}

// handleFinishKey lets the completion screen scroll through long wrapped
// profile, replay, and fingerprint content while keeping the exit keys
// discoverable.
func (m wizardTuiModel) handleFinishKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "enter", "esc":
		return m.shutdown()
	case "up", "k":
		m.viewport.LineUp(1)
	case "down", "j":
		m.viewport.LineDown(1)
	case "pgup":
		m.viewport.HalfViewUp()
	case "pgdown":
		m.viewport.HalfViewDown()
	case "home", "g":
		m.viewport.GotoTop()
	case "end", "G":
		m.viewport.GotoBottom()
	}
	return m, nil
}

func (m wizardTuiModel) shutdown() (tea.Model, tea.Cmd) {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.quit = true
	return m, tea.Quit
}

func (m wizardTuiModel) handleWelcomeKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k", "down", "j", "tab":
		m.cursor = 1 - m.cursor
	case "left":
		m.cursor = 0
	case "right":
		m.cursor = 1
	case "enter", " ", "space":
		if m.cursor == 1 {
			return m.shutdown()
		}
		m.stage = wizardTuiStageUseMode
		m.cursor = wizardTuiIndexOf(m.useModeChoices, m.state.UseMode)
	case "esc":
		return m.shutdown()
	default:
		return m, nil
	}
	m.refreshBody()
	return m, nil
}

func (m wizardTuiModel) handleChoiceKey(key string) (tea.Model, tea.Cmd) {
	choices := m.currentChoices()
	if len(choices) == 0 {
		return m, nil
	}
	switch key {
	case "up", "k":
		m.cursor = (m.cursor - 1 + len(choices)) % len(choices)
	case "down", "j":
		m.cursor = (m.cursor + 1) % len(choices)
	case "home":
		m.cursor = 0
	case "end":
		m.cursor = len(choices) - 1
	case "enter", " ", "space":
		m.applyChoice(choices[m.cursor].Value)
		m.advance()
	case "esc", "left", "backspace":
		m.goBack()
	default:
		return m, nil
	}
	m.refreshBody()
	return m, nil
}

func (m wizardTuiModel) handleProfileKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		raw := strings.TrimSpace(m.profile.Value())
		if raw == "" {
			m.inputError = "Enter a profile name before preparing the plan."
			m.refreshBody()
			return m, nil
		}
		m.inputError = ""
		m.state.ProfileName = sanitizeProfileName(raw)
		m.profile.SetValue(m.state.ProfileName)
		cmd := m.beginDetection()
		return m, cmd
	case "esc":
		m.inputError = ""
		m.stage = wizardTuiStageSetupDepth
		m.cursor = wizardTuiIndexOf(m.depthChoices, m.state.SetupDepth)
		m.profile.Blur()
		m.refreshBody()
		return m, nil
	}
	m.inputError = ""
	var cmd tea.Cmd
	m.profile, cmd = m.profile.Update(msg)
	m.refreshBody()
	return m, cmd
}

func (m wizardTuiModel) handleDetectionKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "left", "backspace":
		m.invalidateDetection()
		m.stage = wizardTuiStageProfile
		cmd := m.profile.Focus()
		m.refreshBody()
		return m, cmd
	case "r":
		if m.status == wizardTuiStatusError || m.status == wizardTuiStatusBlocked {
			m.invalidateDetection()
			m.status = wizardTuiStatusLoading
			m.refreshBody()
			cmd := m.detectionCommand(m.generation)
			return m, cmd
		}
	}
	return m, nil
}

func (m wizardTuiModel) handlePlanKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "left", "backspace":
		m.invalidateDetection()
		m.stage = wizardTuiStageProfile
		cmd := m.profile.Focus()
		m.refreshBody()
		return m, cmd
	case "enter":
		m.stage = wizardTuiStageFinish
		m.viewport.GotoTop()
		m.refreshBody()
		return m, nil
	case "r":
		// A blocked plan may be retried in place; retry re-runs detection and can
		// never approve or execute anything.
		if m.status == wizardTuiStatusBlocked {
			m.invalidateDetection()
			m.stage = wizardTuiStageDetection
			m.status = wizardTuiStatusLoading
			m.refreshBody()
			cmd := m.detectionCommand(m.generation)
			return m, cmd
		}
		return m, nil
	case "d":
		// Disclosure is display-only: it toggles presentation and resets the
		// scroll position to real content, never recomputing or approving the
		// plan. Collapsing a deeply scrolled expanded plan must not land on a
		// nearly empty page.
		m.details = !m.details
		m.refreshBody()
		m.viewport.GotoTop()
		return m, nil
	case "up", "k":
		m.viewport.LineUp(1)
	case "down", "j":
		m.viewport.LineDown(1)
	case "pgup":
		m.viewport.HalfViewUp()
	case "pgdown":
		m.viewport.HalfViewDown()
	case "home", "g":
		m.viewport.GotoTop()
	case "end", "G":
		m.viewport.GotoBottom()
	default:
		return m, nil
	}
	return m, nil
}

func (m wizardTuiModel) currentChoices() []wizardTuiChoice {
	switch m.stage {
	case wizardTuiStageUseMode:
		return m.useModeChoices
	case wizardTuiStageEngine:
		return m.engineChoices
	case wizardTuiStageSetupDepth:
		return m.depthChoices
	default:
		return nil
	}
}

func (m *wizardTuiModel) applyChoice(value string) {
	switch m.stage {
	case wizardTuiStageUseMode:
		m.state.UseMode = value
		m.state.StartingPoint = startingPointForUseMode(value)
	case wizardTuiStageEngine:
		m.state.EnginePack = value
	case wizardTuiStageSetupDepth:
		m.state.SetupDepth = value
		m.state.Complexity = complexityForSetupDepth(value)
	}
}

func (m *wizardTuiModel) advance() {
	switch m.stage {
	case wizardTuiStageUseMode:
		m.stage = wizardTuiStageEngine
		m.cursor = wizardTuiIndexOf(m.engineChoices, m.state.EnginePack)
	case wizardTuiStageEngine:
		m.stage = wizardTuiStageSetupDepth
		m.cursor = wizardTuiIndexOf(m.depthChoices, m.state.SetupDepth)
	case wizardTuiStageSetupDepth:
		m.stage = wizardTuiStageProfile
		m.profile.Focus()
	}
}

func (m *wizardTuiModel) goBack() {
	switch m.stage {
	case wizardTuiStageUseMode:
		m.stage = wizardTuiStageWelcome
		m.cursor = 0
	case wizardTuiStageEngine:
		m.stage = wizardTuiStageUseMode
		m.cursor = wizardTuiIndexOf(m.useModeChoices, m.state.UseMode)
	case wizardTuiStageSetupDepth:
		m.stage = wizardTuiStageEngine
		m.cursor = wizardTuiIndexOf(m.engineChoices, m.state.EnginePack)
	}
}

// beginDetection snapshots the edited choices and starts an asynchronous
// preparation. The returned command performs no work until it is executed.
func (m *wizardTuiModel) beginDetection() tea.Cmd {
	m.invalidateDetection()
	m.snapshot = newWizardTuiSnapshot(m.state)
	m.stage = wizardTuiStageDetection
	m.status = wizardTuiStatusLoading
	m.profile.Blur()
	m.refreshBody()
	return m.detectionCommand(m.generation)
}

// invalidateDetection cancels any in-flight preparation, bumps the generation so
// late results are dropped, and clears the derived plan.
func (m *wizardTuiModel) invalidateDetection() {
	m.generation++
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.plan = piinstall.Plan{}
	m.preflight = nil
	m.prepareErr = nil
	m.details = false
	m.status = wizardTuiStatusIdle
}

func (m *wizardTuiModel) detectionCommand(generation int) tea.Cmd {
	ctx, cancel := context.WithCancel(m.ctx)
	m.cancel = cancel

	input := m.input
	installer := input.Installer
	if installer == nil {
		installer = DefaultWizardInstaller()
	}
	envChecker := input.EnvChecker
	toolRegistry := input.ToolRegistry
	workspaceRoot := m.options.workspaceRoot
	snapshotState := m.snapshot.wizardState()

	return func() tea.Msg {
		var preflight []string
		if envChecker != nil {
			selected := wizardRequiredToolIDs(snapshotState, toolRegistry)
			preflight = append(preflight, resultStrings(envChecker.Execute(ctx, toolcheck.SelectTools(selected...)))...)
		}
		physicalWorkspace, err := resolvePhysicalPath(workspaceRoot)
		if err != nil {
			return wizardTuiPrepareMsg{generation: generation, preflight: preflight, err: fmt.Errorf("resolve workspace root: %w", err)}
		}
		cfg := wizardRuntimeConfig(input, physicalWorkspace, snapshotState)
		_, plan, err := installer.Prepare(ctx, cfg)
		if err != nil {
			return wizardTuiPrepareMsg{generation: generation, preflight: preflight, err: fmt.Errorf("prepare runtime prerequisite plan: %w", err)}
		}
		if err := ctx.Err(); err != nil {
			return wizardTuiPrepareMsg{generation: generation, preflight: preflight, err: err}
		}
		return wizardTuiPrepareMsg{generation: generation, preflight: preflight, plan: plan, blocked: plan.Blocked()}
	}
}

func (m wizardTuiModel) handlePrepare(msg wizardTuiPrepareMsg) (tea.Model, tea.Cmd) {
	if msg.generation != m.generation {
		// A late result from a superseded snapshot must never replace the plan
		// prepared from the current choices.
		return m, nil
	}
	m.preflight = msg.preflight
	switch {
	case msg.err != nil:
		m.status = wizardTuiStatusError
		m.prepareErr = msg.err
		m.plan = piinstall.Plan{}
	case msg.blocked:
		// A blocked plan is still a valid READ-ONLY plan, so it opens the full
		// Plan review with the blocked state prominent instead of hiding every
		// other component's paths and effects.
		m.status = wizardTuiStatusBlocked
		m.prepareErr = nil
		m.plan = msg.plan
		m.stage = wizardTuiStagePlan
		m.details = false
		m.viewport.GotoTop()
	default:
		m.status = wizardTuiStatusReady
		m.prepareErr = nil
		m.plan = msg.plan
		m.stage = wizardTuiStagePlan
		m.details = false
		m.viewport.GotoTop()
	}
	m.refreshBody()
	return m, nil
}

func (m *wizardTuiModel) resize(width, height int) {
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}
	m.width = width
	m.height = height
	bodyWidth, _ := m.layout()
	m.profile.Width = bodyWidth - 6
	if m.profile.Width < 10 {
		m.profile.Width = 10
	}
	m.refreshBody()
}

// refreshBody rebuilds the scrollable body from the current stage and status,
// and re-derives the viewport dimensions from the ACTUAL rendered chrome so a
// wrapped header or help line can never push the frame past the terminal.
func (m *wizardTuiModel) refreshBody() {
	bodyWidth, bodyHeight := m.layout()
	m.viewport.Width = bodyWidth
	m.viewport.Height = bodyHeight
	pinned := m.viewport.YOffset
	m.viewport.SetContent(m.bodyContent(bodyWidth))
	switch m.stage {
	case wizardTuiStagePlan, wizardTuiStageFinish:
		// SetContent can leave an offset past the new content (a collapsed plan
		// is much shorter); SetYOffset clamps it so the visible page stays on
		// real content instead of trailing blanks.
		m.viewport.SetYOffset(pinned)
	default:
		m.viewport.GotoTop()
	}
}

func (m wizardTuiModel) frameWidth() int {
	width := m.width
	if width <= 0 {
		width = 80
	}
	if width > wizardTuiMaxWidth {
		width = wizardTuiMaxWidth
	}
	width -= 2
	if width < wizardTuiMinWidth-2 {
		width = wizardTuiMinWidth - 2
	}
	return width
}

func (m wizardTuiModel) layout() (int, int) {
	bodyWidth := m.frameWidth() - 2
	if bodyWidth < 16 {
		bodyWidth = 16
	}
	bodyHeight := m.height - m.wizardTuiChromeHeight()
	if bodyHeight < 3 {
		bodyHeight = 3
	}
	return bodyWidth, bodyHeight
}
