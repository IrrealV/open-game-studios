package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"open-game-studios/internal/persistence"
	"open-game-studios/internal/piinstall"
	"open-game-studios/internal/templates"
)

func wizardTuiKeyEnter() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyEnter}
}

func wizardTuiKeyEsc() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyEsc}
}

func wizardTuiKeyRunes(value string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
}

func wizardTuiKeyType(kind tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: kind}
}

func wizardTuiUpdate(t *testing.T, model wizardTuiModel, msg tea.Msg) wizardTuiModel {
	t.Helper()
	updated, _ := model.Update(msg)
	result, ok := updated.(wizardTuiModel)
	if !ok {
		t.Fatalf("Update returned %T, want wizardTuiModel", updated)
	}
	return result
}

func wizardTuiUpdateCmd(t *testing.T, model wizardTuiModel, msg tea.Msg) (wizardTuiModel, tea.Cmd) {
	t.Helper()
	updated, cmd := model.Update(msg)
	result, ok := updated.(wizardTuiModel)
	if !ok {
		t.Fatalf("Update returned %T, want wizardTuiModel", updated)
	}
	return result, cmd
}

func defaultWizardTuiState() wizardState {
	return wizardState{
		UseMode:     "create_from_scratch",
		EnginePack:  "godot-core",
		SetupDepth:  "recommended",
		ProfileName: "Game-Studio",
	}
}

func newWizardTuiTestModel(t *testing.T, state wizardState, installer WizardInstaller) wizardTuiModel {
	t.Helper()
	return newWizardTuiModel(context.Background(), WizardInput{Installer: installer}, wizardTuiOptions{
		workspaceRoot:     t.TempDir(),
		outDir:            "profiles/game-studio/generated",
		finalArtifactPath: ".game-studio/generated/wizard/final.artifact.json",
		state:             state,
	})
}

// wizardTuiReachProfile walks the essentials pages down to the profile field.
func wizardTuiReachProfile(t *testing.T, model wizardTuiModel) wizardTuiModel {
	t.Helper()
	model = wizardTuiUpdate(t, model, wizardTuiKeyEnter()) // welcome -> use mode
	model = wizardTuiUpdate(t, model, wizardTuiKeyEnter()) // use mode -> engine
	model = wizardTuiUpdate(t, model, wizardTuiKeyEnter()) // engine -> depth
	return wizardTuiUpdate(t, model, wizardTuiKeyEnter())  // depth -> profile
}

// wizardTuiDetect walks the essentials pages and begins detection, returning the
// model at the detection page and the pending preparation command.
func wizardTuiDetect(t *testing.T, model wizardTuiModel) (wizardTuiModel, tea.Cmd) {
	t.Helper()
	model = wizardTuiReachProfile(t, model)
	return wizardTuiUpdateCmd(t, model, wizardTuiKeyEnter()) // profile -> detection
}

func TestWizardTuiEnabledRouting(t *testing.T) {
	cases := []struct {
		planOnly       bool
		nonInteractive bool
		stdinTTY       bool
		stdoutTTY      bool
		want           bool
	}{
		{planOnly: true, stdinTTY: true, stdoutTTY: true, want: true},
		{planOnly: true, stdinTTY: false, stdoutTTY: true, want: false},
		{planOnly: true, stdinTTY: true, stdoutTTY: false, want: false},
		{planOnly: true, nonInteractive: true, stdinTTY: true, stdoutTTY: true, want: false},
		{planOnly: false, stdinTTY: true, stdoutTTY: true, want: false},
		{planOnly: false, nonInteractive: true, stdinTTY: true, stdoutTTY: true, want: false},
	}
	for _, tc := range cases {
		got := wizardTuiEnabled(tc.planOnly, tc.nonInteractive, tc.stdinTTY, tc.stdoutTTY)
		if got != tc.want {
			t.Fatalf("wizardTuiEnabled(planOnly=%v nonInteractive=%v stdin=%v stdout=%v)=%v, want %v",
				tc.planOnly, tc.nonInteractive, tc.stdinTTY, tc.stdoutTTY, got, tc.want)
		}
	}
}

// The terminal and program seams must not leak into production defaults while
// still letting tests force the TTY route deterministically.
func TestWizardTuiTerminalCheckSeam(t *testing.T) {
	custom := func(*os.File) bool { return true }
	resolved := wizardTuiTerminalCheck(WizardInput{IsTerminal: custom})
	if !resolved(os.Stdin) {
		t.Fatal("injected terminal seam was ignored")
	}
	if wizardTuiTerminalCheck(WizardInput{}) == nil {
		t.Fatal("default terminal check must be non-nil")
	}
}

func TestWizardTuiViewLayoutBounds(t *testing.T) {
	model := newWizardTuiTestModel(t, defaultWizardTuiState(), nil)

	model = wizardTuiUpdate(t, model, tea.WindowSizeMsg{Width: 80, Height: 24})
	view := model.View()
	for _, want := range []string{"OPEN GAME STUDIOS", "Start", "Exit", "> "} {
		if !strings.Contains(view, want) {
			t.Fatalf("80x24 view missing %q:\n%s", want, view)
		}
	}
	if h := lipgloss.Height(view); h > 24 {
		t.Fatalf("80x24 view is %d lines tall:\n%s", h, view)
	}
	if w := lipgloss.Width(view); w > 80 {
		t.Fatalf("80x24 view is %d columns wide", w)
	}

	model = wizardTuiUpdate(t, model, tea.WindowSizeMsg{Width: 60, Height: 20})
	view = model.View()
	if h := lipgloss.Height(view); h > 20 {
		t.Fatalf("60x20 view is %d lines tall:\n%s", h, view)
	}
	if w := lipgloss.Width(view); w > 60 {
		t.Fatalf("60x20 view is %d columns wide", w)
	}

	model = wizardTuiUpdate(t, model, tea.WindowSizeMsg{Width: 40, Height: 12})
	view = model.View()
	if !strings.Contains(view, "too small") {
		t.Fatalf("too-small terminal must render an honest fallback, got:\n%s", view)
	}
	if h := lipgloss.Height(view); h > 12 {
		t.Fatalf("too-small view is %d lines tall:\n%s", h, view)
	}
}

func TestWizardTuiPreservesFlagsAndProfileEditing(t *testing.T) {
	state := wizardState{
		UseMode:     "existing_game",
		EnginePack:  "unity",
		SetupDepth:  "full",
		ProfileName: "My Game",
	}
	model := newWizardTuiTestModel(t, state, nil)

	model = wizardTuiUpdate(t, model, wizardTuiKeyEnter())
	if model.stage != wizardTuiStageUseMode {
		t.Fatalf("stage=%v, want use mode", model.stage)
	}
	if got := model.currentChoices()[model.cursor].Value; got != "existing_game" {
		t.Fatalf("initial use mode cursor=%q, want existing_game", got)
	}

	model = wizardTuiUpdate(t, model, wizardTuiKeyEnter())
	if got := model.currentChoices()[model.cursor].Value; got != "unity" {
		t.Fatalf("initial engine cursor=%q, want unity", got)
	}

	model = wizardTuiUpdate(t, model, wizardTuiKeyEnter())
	if got := model.currentChoices()[model.cursor].Value; got != "full" {
		t.Fatalf("initial depth cursor=%q, want full", got)
	}

	model = wizardTuiUpdate(t, model, wizardTuiKeyEnter())
	if model.stage != wizardTuiStageProfile {
		t.Fatalf("stage=%v, want profile", model.stage)
	}
	if got := model.profile.Value(); got != "My Game" {
		t.Fatalf("profile value=%q, want %q", got, "My Game")
	}

	// 'q' is a printable character while editing the profile, never a quit.
	model = wizardTuiUpdate(t, model, wizardTuiKeyRunes("q"))
	if model.quit {
		t.Fatal("q must not quit while editing the profile text field")
	}
	if got := model.profile.Value(); !strings.HasSuffix(got, "q") {
		t.Fatalf("typed q was not appended to the profile: %q", got)
	}

	// An empty profile is actionable, not a silent unrelated fallback.
	model.profile.SetValue("")
	model = wizardTuiUpdate(t, model, wizardTuiKeyEnter())
	if model.stage != wizardTuiStageProfile {
		t.Fatalf("empty profile must not advance; stage=%v", model.stage)
	}
	if model.inputError == "" {
		t.Fatal("empty profile must surface an actionable message")
	}
	if model.generation != 0 {
		t.Fatalf("empty profile started detection (generation=%d)", model.generation)
	}

	// Esc back preserves the chosen depth.
	model = wizardTuiUpdate(t, model, wizardTuiKeyEsc())
	if model.stage != wizardTuiStageSetupDepth {
		t.Fatalf("stage=%v, want setup depth", model.stage)
	}
	if model.state.SetupDepth != "full" || model.currentChoices()[model.cursor].Value != "full" {
		t.Fatalf("depth selection was not preserved: state=%q cursor=%q", model.state.SetupDepth, model.currentChoices()[model.cursor].Value)
	}
}

func TestWizardTuiBackPreservesSelectionsAndCtrlCCancels(t *testing.T) {
	model := newWizardTuiTestModel(t, defaultWizardTuiState(), nil)

	model = wizardTuiUpdate(t, model, wizardTuiKeyEnter())           // welcome -> use mode
	model = wizardTuiUpdate(t, model, wizardTuiKeyType(tea.KeyDown)) // existing_game
	model = wizardTuiUpdate(t, model, wizardTuiKeyEnter())           // -> engine
	model = wizardTuiUpdate(t, model, wizardTuiKeyType(tea.KeyDown)) // unity
	model = wizardTuiUpdate(t, model, wizardTuiKeyEnter())           // -> depth
	model = wizardTuiUpdate(t, model, wizardTuiKeyType(tea.KeyDown)) // full
	model = wizardTuiUpdate(t, model, wizardTuiKeyEnter())           // -> profile

	if model.state.UseMode != "existing_game" || model.state.EnginePack != "unity" || model.state.SetupDepth != "full" {
		t.Fatalf("unexpected selections: %#v", model.state)
	}

	model = wizardTuiUpdate(t, model, wizardTuiKeyEsc())
	if model.stage != wizardTuiStageSetupDepth || model.currentChoices()[model.cursor].Value != "full" {
		t.Fatalf("back to depth lost selection: stage=%v cursor=%q", model.stage, model.currentChoices()[model.cursor].Value)
	}
	model = wizardTuiUpdate(t, model, wizardTuiKeyEsc())
	if model.stage != wizardTuiStageEngine || model.currentChoices()[model.cursor].Value != "unity" {
		t.Fatalf("back to engine lost selection: stage=%v", model.stage)
	}
	model = wizardTuiUpdate(t, model, wizardTuiKeyEsc())
	if model.stage != wizardTuiStageUseMode || model.currentChoices()[model.cursor].Value != "existing_game" {
		t.Fatalf("back to use mode lost selection: stage=%v", model.stage)
	}
	model = wizardTuiUpdate(t, model, wizardTuiKeyEsc())
	if model.stage != wizardTuiStageWelcome {
		t.Fatalf("stage=%v, want welcome", model.stage)
	}

	model = wizardTuiUpdate(t, model, wizardTuiKeyType(tea.KeyCtrlC))
	if !model.canceled || !model.quit {
		t.Fatalf("ctrl+c must cancel and quit: canceled=%v quit=%v", model.canceled, model.quit)
	}
}

func TestWizardTuiNoPrepareBeforeDetection(t *testing.T) {
	fake := &fakeWizardInstaller{plan: fakeWizardPlan()}
	model := newWizardTuiTestModel(t, defaultWizardTuiState(), fake)

	model = wizardTuiUpdate(t, model, wizardTuiKeyEnter()) // welcome
	model = wizardTuiUpdate(t, model, wizardTuiKeyEnter()) // use mode
	model = wizardTuiUpdate(t, model, wizardTuiKeyEnter()) // engine
	model = wizardTuiUpdate(t, model, wizardTuiKeyEnter()) // depth -> profile

	if model.stage != wizardTuiStageProfile {
		t.Fatalf("stage=%v, want profile", model.stage)
	}
	if fake.prepareCalls != 0 || fake.executeCalls != 0 {
		t.Fatalf("no checks may run before detection: prepare=%d execute=%d", fake.prepareCalls, fake.executeCalls)
	}
}

func TestWizardTuiDetectionStaleResultRejected(t *testing.T) {
	fake := &fakeWizardInstaller{plan: fakeWizardPlan()}
	model, _ := wizardTuiDetect(t, newWizardTuiTestModel(t, defaultWizardTuiState(), fake))
	staleGeneration := model.generation

	// Edit the profile: going back invalidates the pending request.
	model = wizardTuiUpdate(t, model, wizardTuiKeyEsc())
	model, cmd := wizardTuiUpdateCmd(t, model, wizardTuiKeyEnter())
	if cmd == nil {
		t.Fatal("expected a fresh detection command after editing")
	}
	if model.generation == staleGeneration {
		t.Fatalf("generation did not advance: %d", model.generation)
	}

	model = wizardTuiUpdate(t, model, wizardTuiPrepareMsg{
		generation: staleGeneration,
		plan:       fakeWizardPlan(),
	})
	if len(model.plan.Steps) != 0 {
		t.Fatal("a stale plan result replaced the current plan")
	}
	if model.stage != wizardTuiStageDetection || model.status != wizardTuiStatusLoading {
		t.Fatalf("stale result changed the model: stage=%v status=%v", model.stage, model.status)
	}
}

// blockingWizardInstaller blocks inside Prepare until the request context is
// canceled, which makes per-request cancellation observable.
type blockingWizardInstaller struct {
	started chan struct{}
}

func (b *blockingWizardInstaller) Prepare(ctx context.Context, _ piinstall.Config) (piinstall.Detection, piinstall.Plan, error) {
	select {
	case b.started <- struct{}{}:
	default:
	}
	<-ctx.Done()
	return piinstall.Detection{}, piinstall.Plan{}, ctx.Err()
}

func (b *blockingWizardInstaller) Execute(context.Context, piinstall.Config, piinstall.Plan, piinstall.Consent) (piinstall.Report, error) {
	return piinstall.Report{}, errors.New("Execute must never run from the preview")
}

func TestWizardTuiCancelsInFlightDetection(t *testing.T) {
	installer := &blockingWizardInstaller{started: make(chan struct{}, 1)}
	model, cmd := wizardTuiDetect(t, newWizardTuiTestModel(t, defaultWizardTuiState(), installer))
	if cmd == nil {
		t.Fatal("expected a detection command")
	}

	result := make(chan wizardTuiPrepareMsg, 1)
	go func() {
		msg, _ := cmd().(wizardTuiPrepareMsg)
		result <- msg
	}()

	select {
	case <-installer.started:
	case <-time.After(2 * time.Second):
		t.Fatal("Prepare was not called")
	}

	model = wizardTuiUpdate(t, model, wizardTuiKeyEsc())
	if model.stage != wizardTuiStageProfile {
		t.Fatalf("stage=%v, want profile after back", model.stage)
	}

	var msg wizardTuiPrepareMsg
	select {
	case msg = <-result:
	case <-time.After(2 * time.Second):
		t.Fatal("canceled Prepare did not return")
	}
	if !errors.Is(msg.err, context.Canceled) {
		t.Fatalf("expected context.Canceled from the canceled request, got %v", msg.err)
	}

	model = wizardTuiUpdate(t, model, msg)
	if model.stage != wizardTuiStageProfile || len(model.plan.Steps) != 0 {
		t.Fatalf("canceled result must not change the model: stage=%v steps=%d", model.stage, len(model.plan.Steps))
	}
}

func TestWizardTuiDetectionSurfacesErrorAndBlocked(t *testing.T) {
	model, _ := wizardTuiDetect(t, newWizardTuiTestModel(t, defaultWizardTuiState(), nil))
	model = wizardTuiUpdate(t, model, wizardTuiPrepareMsg{generation: model.generation, err: errors.New("probe failed")})
	if model.status != wizardTuiStatusError || model.stage != wizardTuiStageDetection {
		t.Fatalf("error state not distinct: stage=%v status=%v", model.stage, model.status)
	}
	if view := model.View(); !strings.Contains(view, "failed") {
		t.Fatalf("error view is not actionable:\n%s", view)
	}

	before := model.generation
	model = wizardTuiUpdate(t, model, wizardTuiKeyRunes("r"))
	if model.status != wizardTuiStatusLoading || model.generation == before {
		t.Fatalf("retry did not restart detection: status=%v generation=%d", model.status, model.generation)
	}

	blocked, _ := wizardTuiDetect(t, newWizardTuiTestModel(t, defaultWizardTuiState(), nil))
	blocked = wizardTuiUpdate(t, blocked, wizardTuiPrepareMsg{
		generation: blocked.generation,
		plan:       blockedWizardPlan(),
		blocked:    true,
	})
	if blocked.status != wizardTuiStatusBlocked {
		t.Fatalf("status=%v, want blocked", blocked.status)
	}
	// A blocked plan is still a valid read-only plan: it must open the full
	// Plan review, not a reduced blocked-only detection screen.
	if blocked.stage != wizardTuiStagePlan {
		t.Fatalf("stage=%v, want plan for a blocked plan", blocked.stage)
	}
	if view := blocked.View(); !strings.Contains(view, "blocked") {
		t.Fatalf("blocked view must say blocked:\n%s", view)
	}
}

func TestWizardTuiEditInvalidatesDetectionAndPlan(t *testing.T) {
	fake := &fakeWizardInstaller{plan: fakeWizardPlan()}
	model, cmd := wizardTuiDetect(t, newWizardTuiTestModel(t, defaultWizardTuiState(), fake))
	if cmd == nil {
		t.Fatal("expected a detection command")
	}
	model = wizardTuiUpdate(t, model, cmd())
	if model.stage != wizardTuiStagePlan || len(model.plan.Steps) == 0 {
		t.Fatalf("stage=%v steps=%d, want a ready plan", model.stage, len(model.plan.Steps))
	}

	model = wizardTuiUpdate(t, model, wizardTuiKeyEsc())
	if model.stage != wizardTuiStageProfile {
		t.Fatalf("stage=%v, want profile", model.stage)
	}
	if len(model.plan.Steps) != 0 || model.status != wizardTuiStatusIdle {
		t.Fatalf("editing must invalidate the plan: steps=%d status=%v", len(model.plan.Steps), model.status)
	}

	model, cmd = wizardTuiUpdateCmd(t, model, wizardTuiKeyEnter())
	if cmd == nil {
		t.Fatal("expected a re-preparation command")
	}
	model = wizardTuiUpdate(t, model, cmd())
	if model.stage != wizardTuiStagePlan {
		t.Fatalf("stage=%v, want plan", model.stage)
	}
	if fake.prepareCalls != 2 {
		t.Fatalf("prepare calls=%d, want 2", fake.prepareCalls)
	}
}

func TestWizardTuiPlanReviewKeepsMaterialAndScrolls(t *testing.T) {
	plan := longWizardTuiPlan()
	fake := &fakeWizardInstaller{plan: plan}
	model, cmd := wizardTuiDetect(t, newWizardTuiTestModel(t, defaultWizardTuiState(), fake))
	if cmd == nil {
		t.Fatal("expected a detection command")
	}
	model = wizardTuiUpdate(t, model, cmd())
	if model.stage != wizardTuiStagePlan {
		t.Fatalf("stage=%v, want plan", model.stage)
	}

	bodyWidth, _ := model.layout()
	content := model.bodyContent(bodyWidth)
	// Compact default: component/action rows, group counts, and the disclosure
	// affordance, but not the description/destination/effect wall.
	for _, want := range []string{"node [install]", "shell [ensure]", "1 install", "1 conditional", "press d for exact details"} {
		if !strings.Contains(content, want) {
			t.Fatalf("compact plan dropped %q:\n%s", want, content)
		}
	}
	for _, hidden := range []string{longWizardTuiPath, longWizardTuiEffect, "→ ", "observed:", plan.Fingerprint()} {
		if strings.Contains(content, hidden) {
			t.Fatalf("compact plan leaked verbose material %q:\n%s", hidden, content)
		}
	}
	if strings.Contains(content, "$ download+verify") {
		t.Fatal("long command must stay behind Details by default")
	}

	unwrapped := strings.Join(renderWizardTuiPlanStep(plan.Steps[0], true), "\n")
	for _, want := range []string{longWizardTuiPath, longWizardTuiEffect, "→ ", "observed:"} {
		if !strings.Contains(unwrapped, want) {
			t.Fatalf("expanded step dropped material %q:\n%s", want, unwrapped)
		}
	}

	model = wizardTuiUpdate(t, model, wizardTuiKeyRunes("d"))
	detailed := model.bodyContent(bodyWidth)
	detailedFlat := wizardTuiStripWhitespace(detailed)
	for _, want := range []string{"$ download+verify", longWizardTuiPath, longWizardTuiEffect, "tail-marker-zeta", "initializes Pi bootstrap settings", plan.Fingerprint()} {
		if !strings.Contains(detailedFlat, wizardTuiStripWhitespace(want)) {
			t.Fatalf("Details must reveal %q:\n%s", want, detailed)
		}
	}
	for _, line := range strings.Split(detailed, "\n") {
		if lipgloss.Width(line) > bodyWidth {
			t.Fatalf("plan line exceeds body width %d: %q", bodyWidth, line)
		}
	}

	if model.viewport.TotalLineCount() <= model.viewport.Height {
		t.Fatalf("fixture must be scrollable: lines=%d height=%d", model.viewport.TotalLineCount(), model.viewport.Height)
	}
	model = wizardTuiUpdate(t, model, wizardTuiKeyRunes("G"))
	if model.viewport.YOffset == 0 {
		t.Fatal("expected the plan body to scroll")
	}
	if !strings.Contains(model.viewport.View(), "tail-marker-zeta") {
		t.Fatalf("scrolling did not reach the plan tail:\n%s", model.viewport.View())
	}
	model = wizardTuiUpdate(t, model, wizardTuiKeyRunes("g"))
	if model.viewport.YOffset != 0 {
		t.Fatalf("scroll-to-top failed: offset=%d", model.viewport.YOffset)
	}
}

// The ordinary eight-component plan must render materially fewer lines in the
// compact default than in the expanded disclosure, at both target sizes.
func TestWizardTuiPlanCompactReducesRenderedLines(t *testing.T) {
	plan := fakeWizardPlan()
	if len(plan.Steps) != 8 {
		t.Fatalf("fixture must carry the ordinary eight-component plan, got %d", len(plan.Steps))
	}
	sizes := []struct{ width, height int }{{80, 24}, {60, 20}}
	for _, size := range sizes {
		t.Run(fmt.Sprintf("%dx%d", size.width, size.height), func(t *testing.T) {
			fake := &fakeWizardInstaller{plan: plan}
			model, cmd := wizardTuiDetect(t, newWizardTuiTestModel(t, defaultWizardTuiState(), fake))
			if cmd == nil {
				t.Fatal("expected a detection command")
			}
			model = wizardTuiUpdate(t, model, cmd())
			model = wizardTuiUpdate(t, model, tea.WindowSizeMsg{Width: size.width, Height: size.height})
			if model.details {
				t.Fatal("the plan review must open compact")
			}
			compactLines := model.viewport.TotalLineCount()
			if strings.TrimSpace(model.viewport.View()) == "" {
				t.Fatal("compact plan rendered nothing")
			}

			expanded := wizardTuiUpdate(t, model, wizardTuiKeyRunes("d"))
			expandedLines := expanded.viewport.TotalLineCount()
			// Compact must stay under two thirds of the expanded disclosure.
			if compactLines*3 > expandedLines*2 {
				t.Fatalf("compact plan is not materially smaller: compact=%d expanded=%d", compactLines, expandedLines)
			}
			if fake.executeCalls != 0 {
				t.Fatalf("viewing the plan executed it: execute=%d", fake.executeCalls)
			}
		})
	}
}

// Every real component/action row must stay reachable in the compact default,
// and no verbose field may leak without the explicit disclosure.
func TestWizardTuiPlanCompactListsEveryComponent(t *testing.T) {
	plan := fakeWizardPlan()
	fake := &fakeWizardInstaller{plan: plan}
	model, cmd := wizardTuiDetect(t, newWizardTuiTestModel(t, defaultWizardTuiState(), fake))
	if cmd == nil {
		t.Fatal("expected a detection command")
	}
	model = wizardTuiUpdate(t, model, cmd())
	if model.details {
		t.Fatal("the plan review must open compact")
	}

	_, frames := wizardTuiScrollViewport(t, model)
	haystack := wizardTuiStripWhitespace(frames)
	for _, step := range plan.Steps {
		want := fmt.Sprintf("%s [%s]", step.Component, step.Action)
		if !strings.Contains(haystack, wizardTuiStripWhitespace(want)) {
			t.Fatalf("compact plan never listed %q:\n%s", want, frames)
		}
	}
	for _, hidden := range []string{plan.Fingerprint(), "observed:"} {
		if strings.Contains(haystack, wizardTuiStripWhitespace(hidden)) {
			t.Fatalf("compact plan leaked %q without Details:\n%s", hidden, frames)
		}
	}
	for _, step := range plan.Steps {
		for _, effect := range step.Effects {
			if strings.Contains(haystack, wizardTuiStripWhitespace(effect)) {
				t.Fatalf("compact plan leaked effect %q without Details:\n%s", effect, frames)
			}
		}
	}
	if fake.prepareCalls != 1 || fake.executeCalls != 0 {
		t.Fatalf("compact review ran checks: prepare=%d execute=%d", fake.prepareCalls, fake.executeCalls)
	}
}

// d is display-only: it must be reversible, keep the plan immutable, advertise
// its own state, and never recompute or execute anything.
func TestWizardTuiPlanDetailsToggleIsReversibleDisplayOnly(t *testing.T) {
	plan := fakeWizardPlan()
	fake := &fakeWizardInstaller{plan: plan}
	model, cmd := wizardTuiDetect(t, newWizardTuiTestModel(t, defaultWizardTuiState(), fake))
	if cmd == nil {
		t.Fatal("expected a detection command")
	}
	model = wizardTuiUpdate(t, model, cmd())
	if model.stage != wizardTuiStagePlan {
		t.Fatalf("stage=%v, want plan", model.stage)
	}
	compactLines := model.viewport.TotalLineCount()
	fingerprint := model.plan.Fingerprint()
	generation := model.generation

	if !strings.Contains(model.helpText(), "show details") {
		t.Fatalf("compact help must advertise the disclosure: %q", model.helpText())
	}
	expanded := wizardTuiUpdate(t, model, wizardTuiKeyRunes("d"))
	if !expanded.details {
		t.Fatal("d must expand the details")
	}
	if !strings.Contains(expanded.helpText(), "collapse") {
		t.Fatalf("expanded help must offer collapse: %q", expanded.helpText())
	}
	if expanded.viewport.TotalLineCount() <= compactLines {
		t.Fatalf("Details did not add disclosure lines: compact=%d expanded=%d", compactLines, expanded.viewport.TotalLineCount())
	}

	collapsed := wizardTuiUpdate(t, expanded, wizardTuiKeyRunes("d"))
	if collapsed.details {
		t.Fatal("a second d must collapse the details")
	}
	if !strings.Contains(collapsed.helpText(), "show details") {
		t.Fatalf("collapsed help must return to show details: %q", collapsed.helpText())
	}
	if collapsed.viewport.TotalLineCount() != compactLines {
		t.Fatalf("collapse did not restore the compact layout: got=%d want=%d", collapsed.viewport.TotalLineCount(), compactLines)
	}
	if collapsed.plan.Fingerprint() != fingerprint {
		t.Fatalf("toggling details changed the plan fingerprint: %q -> %q", fingerprint, collapsed.plan.Fingerprint())
	}
	if len(collapsed.plan.Steps) != len(plan.Steps) {
		t.Fatalf("toggling details changed the plan steps: got=%d want=%d", len(collapsed.plan.Steps), len(plan.Steps))
	}
	if collapsed.generation != generation {
		t.Fatalf("toggling details recomputed detection: generation=%d want=%d", collapsed.generation, generation)
	}
	if fake.prepareCalls != 1 || fake.executeCalls != 0 {
		t.Fatalf("toggling details ran checks: prepare=%d execute=%d", fake.prepareCalls, fake.executeCalls)
	}
}

// Collapsing an expanded plan that was scrolled to the bottom must reset to
// real content instead of leaving an almost empty, deeply offset page.
func TestWizardTuiPlanCollapseAfterDeepScrollShowsContent(t *testing.T) {
	fake := &fakeWizardInstaller{plan: fakeWizardPlan()}
	model, cmd := wizardTuiDetect(t, newWizardTuiTestModel(t, defaultWizardTuiState(), fake))
	if cmd == nil {
		t.Fatal("expected a detection command")
	}
	model = wizardTuiUpdate(t, model, cmd())
	model = wizardTuiUpdate(t, model, tea.WindowSizeMsg{Width: 60, Height: 20})
	model = wizardTuiUpdate(t, model, wizardTuiKeyRunes("d")) // expand
	model = wizardTuiUpdate(t, model, wizardTuiKeyRunes("G")) // deep scroll
	if model.viewport.YOffset == 0 {
		t.Fatal("expanded fixture must be scrollable")
	}
	expandedLines := model.viewport.TotalLineCount()

	collapsed := wizardTuiUpdate(t, model, wizardTuiKeyRunes("d"))
	if collapsed.details {
		t.Fatal("d must collapse")
	}
	if collapsed.viewport.TotalLineCount() >= expandedLines {
		t.Fatalf("collapse did not shrink the content: compact=%d expanded=%d", collapsed.viewport.TotalLineCount(), expandedLines)
	}
	if collapsed.viewport.YOffset != 0 {
		t.Fatalf("collapse must reset to useful content: offset=%d", collapsed.viewport.YOffset)
	}
	view := collapsed.viewport.View()
	if strings.TrimSpace(view) == "" {
		t.Fatal("collapsed viewport is empty")
	}
	if !strings.Contains(wizardTuiStripWhitespace(view), wizardTuiStripWhitespace("node [install]")) {
		t.Fatalf("collapsed viewport did not return to the plan rows:\n%s", view)
	}
	if fake.executeCalls != 0 {
		t.Fatalf("collapse executed the plan: execute=%d", fake.executeCalls)
	}
}

func TestWizardTuiFinishReplayUsesFinalChoices(t *testing.T) {
	fake := &fakeWizardInstaller{plan: fakeWizardPlan()}
	state := wizardState{UseMode: "existing_game", EnginePack: "godot-core", SetupDepth: "full", ProfileName: "Original"}
	model := wizardTuiReachProfile(t, newWizardTuiTestModel(t, state, fake))
	model.profile.SetValue("Chosen Studio")
	model, cmd := wizardTuiUpdateCmd(t, model, wizardTuiKeyEnter())
	if cmd == nil {
		t.Fatal("expected a detection command")
	}
	model = wizardTuiUpdate(t, model, cmd())
	model = wizardTuiUpdate(t, model, wizardTuiKeyEnter())
	if model.stage != wizardTuiStageFinish {
		t.Fatalf("stage=%v, want finish", model.stage)
	}

	bodyWidth, _ := model.layout()
	body := model.bodyContent(bodyWidth)
	if !strings.Contains(body, "Chosen Studio") || !strings.Contains(body, "existing_game") {
		t.Fatalf("finish preview must reflect final choices:\n%s", body)
	}
	replay := wizardReplayCommand(wizardTuiReplayArgs(model.snapshot, model.options.outDir, model.options.finalArtifactPath), "--plan-only")
	for _, want := range []string{"--non-interactive", "--plan-only", "Chosen Studio", "existing_game", "--setup-depth full"} {
		if !strings.Contains(replay, want) {
			t.Fatalf("replay %q missing %q", replay, want)
		}
	}
	if strings.Contains(replay, "--approve-plan") {
		t.Fatalf("preview printed an approval command: %s", replay)
	}
}

func TestWizardTuiReplayArgsUseFinalChoices(t *testing.T) {
	snapshot := wizardTuiSnapshot{
		UseMode:     "existing_game",
		EnginePack:  "godot-core",
		SetupDepth:  "full",
		ProfileName: "Chosen Studio",
	}
	args := wizardTuiReplayArgs(snapshot, "profiles/x", ".game-studio/f.json")
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"--non-interactive",
		"--plan-only",
		"--profile Chosen Studio",
		"--use-mode existing_game",
		"--engine-pack godot-core",
		"--setup-depth full",
		"--out-dir profiles/x",
		"--final-artifact .game-studio/f.json",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("replay args %q missing %q", joined, want)
		}
	}
	if strings.Contains(joined, "--approve-plan") {
		t.Fatalf("replay args must not grant approval: %q", joined)
	}
}

func TestWizardTuiRunWizardIntegrationPlanOnly(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
	fake := &fakeWizardInstaller{plan: fakeWizardPlan()}

	program := func(_ context.Context, model wizardTuiModel) (wizardTuiModel, error) {
		model = wizardTuiUpdate(t, model, wizardTuiKeyEnter()) // welcome -> use mode
		if model.state.UseMode != "create_from_scratch" {
			t.Fatalf("initial use mode=%q", model.state.UseMode)
		}
		model = wizardTuiUpdate(t, model, wizardTuiKeyEnter()) // -> engine
		model = wizardTuiUpdate(t, model, wizardTuiKeyEnter()) // -> depth
		model = wizardTuiUpdate(t, model, wizardTuiKeyEnter()) // -> profile
		model.profile.SetValue("Chosen Studio")
		model, cmd := wizardTuiUpdateCmd(t, model, wizardTuiKeyEnter())
		if cmd == nil {
			t.Fatal("expected a detection command")
		}
		model = wizardTuiUpdate(t, model, cmd())
		if model.stage != wizardTuiStagePlan {
			t.Fatalf("stage=%v, want plan", model.stage)
		}
		model = wizardTuiUpdate(t, model, wizardTuiKeyEnter()) // -> finish
		if model.stage != wizardTuiStageFinish {
			t.Fatalf("stage=%v, want finish", model.stage)
		}
		return model, nil
	}

	var runErr error
	stdout := captureStdout(t, func() {
		runErr = RunWizard(WizardInput{
			Args:        []string{"--plan-only", "--profile", "Stale Flag", "--use-mode", "create_from_scratch", "--final-artifact", finalPath},
			Registry:    templates.NewRegistry(),
			Persistence: persistence.NewPlaceholder(),
			Installer:   fake,
			IsTerminal:  func(*os.File) bool { return true },
			TuiProgram:  program,
		})
	})
	if runErr != nil {
		t.Fatalf("RunWizard returned error: %v", runErr)
	}
	if fake.prepareCalls != 1 || fake.executeCalls != 0 {
		t.Fatalf("prepare=%d execute=%d, want 1/0", fake.prepareCalls, fake.executeCalls)
	}
	if !filepath.IsAbs(fake.lastConfig.WorkspaceDir) {
		t.Fatalf("expected an absolute WorkspaceDir, got %q", fake.lastConfig.WorkspaceDir)
	}
	if !fake.lastConfig.GodotRequired {
		t.Fatal("create_from_scratch with godot-core must require Godot")
	}
	assertNoWizardWrites(t, workspace)

	for _, want := range []string{"preview complete", "Chosen Studio", "reproducible plan-only preview", "--plan-only"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("summary missing %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "Stale Flag") {
		t.Fatalf("summary must use final choices, not stale flags:\n%s", stdout)
	}
	if strings.Contains(stdout, "--approve-plan") {
		t.Fatalf("summary must not print an approval command:\n%s", stdout)
	}
}

func TestWizardTuiErrorExitEmitsNoSuccessGuidance(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
	fake := &fakeWizardInstaller{prepareErr: errors.New("probe failed")}

	program := func(_ context.Context, model wizardTuiModel) (wizardTuiModel, error) {
		model, cmd := wizardTuiDetect(t, model)
		if cmd == nil {
			t.Fatal("expected a detection command")
		}
		model = wizardTuiUpdate(t, model, cmd())
		if model.status != wizardTuiStatusError {
			t.Fatalf("status=%v, want error", model.status)
		}
		return wizardTuiUpdate(t, model, wizardTuiKeyRunes("q")), nil
	}

	var runErr error
	stdout := captureStdout(t, func() {
		runErr = RunWizard(WizardInput{
			Args:        []string{"--plan-only", "--final-artifact", finalPath},
			Registry:    templates.NewRegistry(),
			Persistence: persistence.NewPlaceholder(),
			Installer:   fake,
			IsTerminal:  func(*os.File) bool { return true },
			TuiProgram:  program,
		})
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "probe failed") {
		t.Fatalf("expected the prepare error to surface, got %v", runErr)
	}
	if strings.Contains(stdout, "preview complete") || strings.Contains(stdout, "--approve-plan") {
		t.Fatalf("errored preview must emit no guidance:\n%s", stdout)
	}
	if fake.executeCalls != 0 {
		t.Fatalf("execute calls=%d, want 0", fake.executeCalls)
	}
	assertNoWizardWrites(t, workspace)
}

func TestWizardTuiCanceledExitEmitsNoSuccessGuidance(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
	fake := &fakeWizardInstaller{plan: fakeWizardPlan()}

	program := func(_ context.Context, model wizardTuiModel) (wizardTuiModel, error) {
		return wizardTuiUpdate(t, model, wizardTuiKeyType(tea.KeyCtrlC)), nil
	}

	var runErr error
	stdout := captureStdout(t, func() {
		runErr = RunWizard(WizardInput{
			Args:        []string{"--plan-only", "--final-artifact", finalPath},
			Registry:    templates.NewRegistry(),
			Persistence: persistence.NewPlaceholder(),
			Installer:   fake,
			IsTerminal:  func(*os.File) bool { return true },
			TuiProgram:  program,
		})
	})
	if runErr != nil {
		t.Fatalf("canceled preview must not return an error, got %v", runErr)
	}
	if strings.Contains(stdout, "preview complete") || strings.Contains(stdout, "--approve-plan") {
		t.Fatalf("canceled preview must emit no guidance:\n%s", stdout)
	}
	if fake.prepareCalls != 0 || fake.executeCalls != 0 {
		t.Fatalf("canceled preview ran checks: prepare=%d execute=%d", fake.prepareCalls, fake.executeCalls)
	}
	assertNoWizardWrites(t, workspace)
}

func TestWizardTuiNonInteractiveKeepsPrintPath(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
	fake := &fakeWizardInstaller{plan: fakeWizardPlan()}

	programCalled := false
	program := func(_ context.Context, model wizardTuiModel) (wizardTuiModel, error) {
		programCalled = true
		return model, nil
	}

	stdout := captureStdout(t, func() {
		if err := RunWizard(WizardInput{
			Args:        []string{"--non-interactive", "--plan-only", "--final-artifact", finalPath},
			Registry:    templates.NewRegistry(),
			Persistence: persistence.NewPlaceholder(),
			Installer:   fake,
			IsTerminal:  func(*os.File) bool { return true },
			TuiProgram:  program,
		}); err != nil {
			t.Fatalf("RunWizard returned error: %v", err)
		}
	})
	if programCalled {
		t.Fatal("non-interactive plan-only must not launch the TUI")
	}
	for _, want := range []string{"[wizard] runtime prerequisite plan", "plan-only preview complete"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("non-interactive output missing %q:\n%s", want, stdout)
		}
	}
	assertNoWizardWrites(t, workspace)
}

const (
	longWizardTuiPath    = "/owned/nested-segment/nested-segment/nested-segment/nested-segment/nested-segment/nested-segment/godot-4.7.2/bin/godot"
	longWizardTuiEffect  = "downloads the official archive over https and verifies the pinned SHA-256 checksum before extraction, then records the verified binary in the launcher"
	longWizardTuiCommand = "download+verify+extract artifact-artifact-artifact-artifact-artifact-artifact-artifact-payload"
)

func longWizardTuiPlan() piinstall.Plan {
	return piinstall.Plan{
		GodotRequired: true,
		Steps: []piinstall.PlanStep{
			{
				Component: piinstall.ComponentNode,
				Action:    piinstall.ActionInstall,
				Reason:    "node is missing; install the pinned candidate",
				Command:   longWizardTuiCommand,
				Outputs:   []string{longWizardTuiPath},
				Effects:   []string{longWizardTuiEffect},
				Observed:  piinstall.ComponentState{Component: piinstall.ComponentNode, Compatibility: piinstall.CompatAbsent},
			},
			{
				Component: piinstall.ComponentShell,
				Action:    piinstall.ActionEnsure,
				Reason:    "reuse an exact pinned registration or install it after consent",
				Effects:   []string{"initializes Pi bootstrap settings and may change fullscreen state"},
				Observed: piinstall.ComponentState{
					Component:     piinstall.ComponentShell,
					Compatibility: piinstall.CompatUnknown,
					ProbeDeferred: true,
					Detail:        "tail-marker-zeta",
				},
			},
		},
	}
}

const longWizardTuiBlockedReason = "fail closed: engram output could not be parsed as a version"

// mixedBlockedWizardTuiPlan combines install, ensure, and blocked steps so a
// review can prove a blocked plan keeps every component reviewable.
func mixedBlockedWizardTuiPlan() piinstall.Plan {
	plan := longWizardTuiPlan()
	plan.Steps = append(plan.Steps, piinstall.PlanStep{
		Component: piinstall.ComponentEngramCore,
		Action:    piinstall.ActionBlocked,
		Reason:    longWizardTuiBlockedReason,
		Observed: piinstall.ComponentState{
			Component:     piinstall.ComponentEngramCore,
			Compatibility: piinstall.CompatUnknown,
		},
	})
	return plan
}

// wizardTuiStripWhitespace removes every whitespace run so a logical phrase can
// be matched across wrapped lines and across scroll frames.
func wizardTuiStripWhitespace(value string) string {
	return strings.Join(strings.Fields(value), "")
}

// wizardTuiScrollViewport walks the rendered viewport one line at a time and
// returns the final model plus every rendered frame. Reachability is proven
// from real Update/Viewport output, not from the raw body helper alone.
func wizardTuiScrollViewport(t *testing.T, model wizardTuiModel) (wizardTuiModel, string) {
	t.Helper()
	frames := []string{model.viewport.View()}
	steps := model.viewport.TotalLineCount() + model.viewport.Height + 1
	for i := 0; i < steps; i++ {
		next := wizardTuiUpdate(t, model, wizardTuiKeyRunes("j"))
		if next.viewport.YOffset == model.viewport.YOffset {
			break
		}
		model = next
		frames = append(frames, model.viewport.View())
	}
	return model, strings.Join(frames, "\n")
}

// wizardTuiAssertFits checks width AND height bounds and that the complete
// contextual help is still rendered, even when it wraps.
func wizardTuiAssertFits(t *testing.T, model wizardTuiModel, width, height int) {
	t.Helper()
	view := model.View()
	if got := lipgloss.Height(view); got > height {
		t.Fatalf("%dx%d view rendered %d lines:\n%s", width, height, got, view)
	}
	if got := lipgloss.Width(view); got > width {
		t.Fatalf("%dx%d view rendered %d columns:\n%s", width, height, got, view)
	}
	if help := wizardTuiStripWhitespace(model.helpText()); help != "" {
		if !strings.Contains(wizardTuiStripWhitespace(view), help) {
			t.Fatalf("%dx%d view lost its contextual help %q:\n%s", width, height, model.helpText(), view)
		}
	}
}

func TestWizardTuiBlockedPlanStaysFullyReviewable(t *testing.T) {
	plan := mixedBlockedWizardTuiPlan()
	fake := &fakeWizardInstaller{plan: plan}
	model, cmd := wizardTuiDetect(t, newWizardTuiTestModel(t, defaultWizardTuiState(), fake))
	if cmd == nil {
		t.Fatal("expected a detection command")
	}
	model = wizardTuiUpdate(t, model, cmd())
	if model.stage != wizardTuiStagePlan {
		t.Fatalf("blocked plan must open the full Plan review: stage=%v status=%v", model.stage, model.status)
	}
	if model.status != wizardTuiStatusBlocked || !model.plan.Blocked() {
		t.Fatalf("blocked state lost: status=%v blocked=%v", model.status, model.plan.Blocked())
	}
	if model.viewport.YOffset != 0 {
		t.Fatalf("blocked plan review must start at the top: offset=%d", model.viewport.YOffset)
	}

	// Compact mode must keep every component row and the blocking reason
	// visible without expanding anything, and must not leak the verbose wall.
	scrolled, frames := wizardTuiScrollViewport(t, model)
	haystack := wizardTuiStripWhitespace(frames)
	for _, want := range []string{
		"BLOCKED",
		"status: blocked",
		"node [install]",
		"shell [ensure]",
		string(piinstall.ComponentEngramCore),
		longWizardTuiBlockedReason,
	} {
		if !strings.Contains(haystack, wizardTuiStripWhitespace(want)) {
			t.Fatalf("compact blocked plan never revealed %q:\n%s", want, frames)
		}
	}
	for _, hidden := range []string{plan.Fingerprint(), longWizardTuiEffect, "$ " + longWizardTuiCommand} {
		if strings.Contains(haystack, wizardTuiStripWhitespace(hidden)) {
			t.Fatalf("compact blocked plan leaked verbose material %q:\n%s", hidden, frames)
		}
	}
	if fake.executeCalls != 0 {
		t.Fatalf("reviewing a blocked plan executed it: execute=%d", fake.executeCalls)
	}

	// Details must still toggle on a blocked plan and reveal every exact field:
	// fingerprint, destinations, effects, observed state, and the raw command.
	scrolled = wizardTuiUpdate(t, scrolled, wizardTuiKeyRunes("d"))
	scrolled = wizardTuiUpdate(t, scrolled, wizardTuiKeyRunes("g"))
	_, detailFrames := wizardTuiScrollViewport(t, scrolled)
	detailHaystack := wizardTuiStripWhitespace(detailFrames)
	for _, want := range []string{
		plan.Fingerprint(),
		longWizardTuiEffect,
		"nested-segment",
		"observed:",
		"$ " + longWizardTuiCommand,
	} {
		if !strings.Contains(detailHaystack, wizardTuiStripWhitespace(want)) {
			t.Fatalf("expanded blocked plan never revealed %q:\n%s", want, detailFrames)
		}
	}

	// A blocked plan must never reach an approval/execute outcome, and its final
	// outcome keeps the blocked state prominent.
	finish := wizardTuiUpdate(t, model, wizardTuiKeyEnter())
	if finish.stage != wizardTuiStageFinish {
		t.Fatalf("stage=%v, want finish", finish.stage)
	}
	if !finish.plan.Blocked() {
		t.Fatal("the final outcome lost the blocked state")
	}
	if !strings.Contains(wizardTuiStripWhitespace(finish.View()), "BLOCKED") {
		t.Fatalf("finish must keep the blocked state prominent:\n%s", finish.View())
	}
	if fake.executeCalls != 0 {
		t.Fatalf("finishing a blocked plan executed it: execute=%d", fake.executeCalls)
	}

	// Retry restarts detection only; it never approves or executes.
	before := model.generation
	retried := wizardTuiUpdate(t, model, wizardTuiKeyRunes("r"))
	if retried.stage != wizardTuiStageDetection || retried.status != wizardTuiStatusLoading {
		t.Fatalf("retry must restart detection: stage=%v status=%v", retried.stage, retried.status)
	}
	if retried.generation == before {
		t.Fatalf("retry did not advance the generation: %d", retried.generation)
	}

	// Back/change invalidates the blocked plan without approving it.
	back := wizardTuiUpdate(t, model, wizardTuiKeyEsc())
	if back.stage != wizardTuiStageProfile {
		t.Fatalf("stage=%v, want profile after back", back.stage)
	}
	if back.plan.Blocked() {
		t.Fatal("back must invalidate the blocked plan")
	}
	if fake.executeCalls != 0 {
		t.Fatalf("back/retry executed the plan: execute=%d", fake.executeCalls)
	}
}

func TestWizardTuiBlockedPlanRunOutcomeStaysBlocked(t *testing.T) {
	workspace := wizardWorkspaceFixture(t)
	setWorkingDirectory(t, workspace)
	finalPath := filepath.Join(workspace, "out", "wizard", "final.artifact.json")
	fake := &fakeWizardInstaller{plan: mixedBlockedWizardTuiPlan()}

	program := func(_ context.Context, model wizardTuiModel) (wizardTuiModel, error) {
		model, cmd := wizardTuiDetect(t, model)
		if cmd == nil {
			t.Fatal("expected a detection command")
		}
		model = wizardTuiUpdate(t, model, cmd())
		if model.stage != wizardTuiStagePlan || model.status != wizardTuiStatusBlocked {
			t.Fatalf("stage=%v status=%v, want a blocked plan review", model.stage, model.status)
		}
		return wizardTuiUpdate(t, model, wizardTuiKeyEnter()), nil
	}

	var runErr error
	stdout := captureStdout(t, func() {
		runErr = RunWizard(WizardInput{
			Args:        []string{"--plan-only", "--final-artifact", finalPath},
			Registry:    templates.NewRegistry(),
			Persistence: persistence.NewPlaceholder(),
			Installer:   fake,
			IsTerminal:  func(*os.File) bool { return true },
			TuiProgram:  program,
		})
	})
	if !errors.Is(runErr, errWizardRuntimeBlocked) {
		t.Fatalf("blocked preview must keep the blocked outcome, got %v", runErr)
	}
	if !strings.Contains(stdout, "status: blocked") {
		t.Fatalf("summary must retain the blocked status:\n%s", stdout)
	}
	if strings.Contains(stdout, "--approve-plan") {
		t.Fatalf("blocked preview printed approval guidance:\n%s", stdout)
	}
	if fake.executeCalls != 0 {
		t.Fatalf("execute calls=%d, want 0", fake.executeCalls)
	}
	assertNoWizardWrites(t, workspace)
}

func TestWizardTuiEveryStageFitsTerminalBounds(t *testing.T) {
	sizes := []struct{ width, height int }{{80, 24}, {60, 20}}

	advance := func(t *testing.T, model wizardTuiModel, steps int) wizardTuiModel {
		t.Helper()
		for i := 0; i < steps; i++ {
			model = wizardTuiUpdate(t, model, wizardTuiKeyEnter())
		}
		return model
	}
	readyPlan := func(t *testing.T) wizardTuiModel {
		t.Helper()
		model, cmd := wizardTuiDetect(t, newWizardTuiTestModel(t, defaultWizardTuiState(), &fakeWizardInstaller{plan: fakeWizardPlan()}))
		if cmd == nil {
			t.Fatal("expected a detection command")
		}
		return wizardTuiUpdate(t, model, cmd())
	}
	blockedPlan := func(t *testing.T) wizardTuiModel {
		t.Helper()
		model, cmd := wizardTuiDetect(t, newWizardTuiTestModel(t, defaultWizardTuiState(), &fakeWizardInstaller{plan: mixedBlockedWizardTuiPlan()}))
		if cmd == nil {
			t.Fatal("expected a detection command")
		}
		return wizardTuiUpdate(t, model, cmd())
	}

	cases := []struct {
		name  string
		build func(t *testing.T) wizardTuiModel
	}{
		{"welcome", func(t *testing.T) wizardTuiModel {
			return newWizardTuiTestModel(t, defaultWizardTuiState(), nil)
		}},
		{"use-mode", func(t *testing.T) wizardTuiModel {
			return advance(t, newWizardTuiTestModel(t, defaultWizardTuiState(), nil), 1)
		}},
		{"engine", func(t *testing.T) wizardTuiModel {
			return advance(t, newWizardTuiTestModel(t, defaultWizardTuiState(), nil), 2)
		}},
		{"setup-depth", func(t *testing.T) wizardTuiModel {
			return advance(t, newWizardTuiTestModel(t, defaultWizardTuiState(), nil), 3)
		}},
		{"profile", func(t *testing.T) wizardTuiModel {
			return advance(t, newWizardTuiTestModel(t, defaultWizardTuiState(), nil), 4)
		}},
		{"detection-loading", func(t *testing.T) wizardTuiModel {
			model, _ := wizardTuiDetect(t, newWizardTuiTestModel(t, defaultWizardTuiState(), nil))
			return model
		}},
		{"detection-error", func(t *testing.T) wizardTuiModel {
			model, _ := wizardTuiDetect(t, newWizardTuiTestModel(t, defaultWizardTuiState(), nil))
			return wizardTuiUpdate(t, model, wizardTuiPrepareMsg{generation: model.generation, err: errors.New("probe failed")})
		}},
		{"plan-ready", readyPlan},
		{"plan-details", func(t *testing.T) wizardTuiModel {
			return wizardTuiUpdate(t, readyPlan(t), wizardTuiKeyRunes("d"))
		}},
		{"plan-blocked", blockedPlan},
		{"finish", func(t *testing.T) wizardTuiModel {
			return wizardTuiUpdate(t, readyPlan(t), wizardTuiKeyEnter())
		}},
	}

	for _, size := range sizes {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%s-%dx%d", tc.name, size.width, size.height), func(t *testing.T) {
				model := tc.build(t)
				model = wizardTuiUpdate(t, model, tea.WindowSizeMsg{Width: size.width, Height: size.height})
				wizardTuiAssertFits(t, model, size.width, size.height)

				// Bounds must hold after a repeat resize and after a stage change.
				model = wizardTuiUpdate(t, model, tea.WindowSizeMsg{Width: size.width, Height: size.height})
				wizardTuiAssertFits(t, model, size.width, size.height)

				switch model.stage {
				case wizardTuiStageUseMode, wizardTuiStageEngine, wizardTuiStageSetupDepth:
					model = wizardTuiUpdate(t, model, wizardTuiKeyEnter())
					wizardTuiAssertFits(t, model, size.width, size.height)
				}
			})
		}
	}
}

func TestWizardTuiFinishScrollsFromTopToReplay(t *testing.T) {
	fake := &fakeWizardInstaller{plan: longWizardTuiPlan()}
	model, cmd := wizardTuiDetect(t, newWizardTuiTestModel(t, defaultWizardTuiState(), fake))
	if cmd == nil {
		t.Fatal("expected a detection command")
	}
	model = wizardTuiUpdate(t, model, cmd())
	model = wizardTuiUpdate(t, model, tea.WindowSizeMsg{Width: 60, Height: 20})

	// Expand the plan first: the compact default is intentionally short, so the
	// expanded fixture is what proves a scrolled plan hands off at the top.
	model = wizardTuiUpdate(t, model, wizardTuiKeyRunes("d"))
	model = wizardTuiUpdate(t, model, wizardTuiKeyRunes("G"))
	if model.viewport.YOffset == 0 {
		t.Fatal("expanded fixture plan must be scrollable")
	}
	model = wizardTuiUpdate(t, model, wizardTuiKeyEnter())
	if model.stage != wizardTuiStageFinish {
		t.Fatalf("stage=%v, want finish", model.stage)
	}
	if model.viewport.YOffset != 0 {
		t.Fatalf("Finish must start at the top: offset=%d", model.viewport.YOffset)
	}
	if model.viewport.TotalLineCount() <= model.viewport.Height {
		t.Fatalf("finish fixture must be scrollable: lines=%d height=%d", model.viewport.TotalLineCount(), model.viewport.Height)
	}

	scrolled, frames := wizardTuiScrollViewport(t, model)
	haystack := wizardTuiStripWhitespace(frames)
	replay := wizardReplayCommand(wizardTuiReplayArgs(scrolled.snapshot, scrolled.options.outDir, scrolled.options.finalArtifactPath), "--plan-only")
	for _, want := range []string{
		"Plan preview complete",
		"Game-Studio",
		"create_from_scratch",
		"godot-core",
		"recommended",
		wizardPlanStatus(scrolled.plan),
		scrolled.plan.Fingerprint(),
		replay,
	} {
		if !strings.Contains(haystack, wizardTuiStripWhitespace(want)) {
			t.Fatalf("finish scrolling never revealed %q:\n%s", want, frames)
		}
	}

	// The View must keep the footer exit/help visible after scrolling.
	view := scrolled.View()
	if !strings.Contains(wizardTuiStripWhitespace(view), wizardTuiStripWhitespace(scrolled.helpText())) {
		t.Fatalf("finish footer help is not visible:\n%s", view)
	}

	exited, quitCmd := wizardTuiUpdateCmd(t, scrolled, wizardTuiKeyEnter())
	if !exited.quit || quitCmd == nil {
		t.Fatalf("Enter must exit cleanly: quit=%v cmd=%v", exited.quit, quitCmd)
	}
}

func TestWizardTuiSelectionPagesKeepActiveChoiceVisible(t *testing.T) {
	sizes := []struct{ width, height int }{{80, 24}, {60, 20}}
	for _, size := range sizes {
		for _, stage := range []wizardTuiStage{wizardTuiStageUseMode, wizardTuiStageEngine, wizardTuiStageSetupDepth} {
			model := newWizardTuiTestModel(t, defaultWizardTuiState(), nil)
			model = wizardTuiUpdate(t, model, tea.WindowSizeMsg{Width: size.width, Height: size.height})
			model.stage = stage
			model.cursor = len(model.currentChoices()) - 1
			model.refreshBody()
			label := model.currentChoices()[model.cursor].Label
			if !strings.Contains(wizardTuiStripWhitespace(model.viewport.View()), wizardTuiStripWhitespace(label)) {
				t.Fatalf("%dx%d stage=%v active choice %q is off-screen:\n%s", size.width, size.height, stage, label, model.viewport.View())
			}
		}
	}
}
