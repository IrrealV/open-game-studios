package commands

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"open-game-studios/internal/piinstall"
)

const (
	// wizardTuiMaxWidth bounds the content frame on wide terminals.
	wizardTuiMaxWidth = 92
	// wizardTuiMinWidth / wizardTuiMinHeight define the honest too-small screen.
	wizardTuiMinWidth  = 50
	wizardTuiMinHeight = 16
)

// wizardTuiChromeHeight measures the ACTUAL rendered header, dividers, and
// footer. Help text and the header wrap on narrow terminals, so the body height
// must be derived from the rendered chrome instead of a fixed line count.
func (m wizardTuiModel) wizardTuiChromeHeight() int {
	header := lipgloss.Height(renderWizardTuiHeader(m))
	footer := lipgloss.Height(renderWizardTuiFooter(m))
	return header + footer + 2 // two divider rows
}

// The palette is dark-compatible and degrades to plain text under monochrome.
// Nothing mutates the global lipgloss renderer, so a no-color terminal stays
// fully legible through the marker glyphs and layout.
var (
	wizardTuiHeadingColor = lipgloss.AdaptiveColor{Light: "#9A5B00", Dark: "#E0A458"}
	wizardTuiAccentColor  = lipgloss.AdaptiveColor{Light: "#00695C", Dark: "#5FD7A7"}
	wizardTuiBodyColor    = lipgloss.AdaptiveColor{Light: "#1F1F1F", Dark: "#EDEDED"}
	wizardTuiMutedColor   = lipgloss.AdaptiveColor{Light: "#5F5F5F", Dark: "#9E9E9E"}
	wizardTuiWarnColor    = lipgloss.AdaptiveColor{Light: "#8A5A00", Dark: "#E0A458"}
	wizardTuiDangerColor  = lipgloss.AdaptiveColor{Light: "#9B1C1C", Dark: "#FF8A80"}
	wizardTuiDividerColor = lipgloss.AdaptiveColor{Light: "#C4C4C4", Dark: "#3C3C3C"}
)

var (
	wizardTuiWordmarkStyle = lipgloss.NewStyle().Bold(true).Foreground(wizardTuiHeadingColor)
	wizardTuiStageStyle    = lipgloss.NewStyle().Foreground(wizardTuiMutedColor)
	wizardTuiBodyStyle     = lipgloss.NewStyle().Foreground(wizardTuiBodyColor)
	wizardTuiSectionStyle  = lipgloss.NewStyle().Bold(true).Foreground(wizardTuiHeadingColor)
	wizardTuiSelectedStyle = lipgloss.NewStyle().Bold(true).Foreground(wizardTuiAccentColor)
	wizardTuiOptionStyle   = lipgloss.NewStyle().Foreground(wizardTuiBodyColor)
	wizardTuiNoteStyle     = lipgloss.NewStyle().Foreground(wizardTuiMutedColor)
	wizardTuiHelpStyle     = lipgloss.NewStyle().Foreground(wizardTuiMutedColor)
	wizardTuiWarnStyle     = lipgloss.NewStyle().Foreground(wizardTuiWarnColor)
	wizardTuiDangerStyle   = lipgloss.NewStyle().Bold(true).Foreground(wizardTuiDangerColor)
	wizardTuiAccentStyle   = lipgloss.NewStyle().Foreground(wizardTuiAccentColor)
	wizardTuiDividerStyle  = lipgloss.NewStyle().Foreground(wizardTuiDividerColor)
)

func (m wizardTuiModel) View() string {
	width := m.width
	height := m.height
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}
	if width < wizardTuiMinWidth || height < wizardTuiMinHeight {
		return renderWizardTuiTooSmall(width, height)
	}
	if m.viewport.Width <= 0 || m.viewport.Height <= 0 {
		m.resize(width, height)
	}
	frame := strings.Join([]string{
		renderWizardTuiHeader(m),
		wizardTuiDivider(m.frameWidth()),
		m.viewport.View(),
		wizardTuiDivider(m.frameWidth()),
		renderWizardTuiFooter(m),
	}, "\n")
	return lipgloss.NewStyle().MarginLeft(2).Render(frame)
}

func renderWizardTuiHeader(m wizardTuiModel) string {
	wordmark := wizardTuiWordmarkStyle.Render("OGS · OPEN GAME STUDIOS")
	right := wizardTuiStageStyle.Render(fmt.Sprintf("plan preview · %s", m.stage.title()))
	width := m.frameWidth()
	gap := width - lipgloss.Width(wordmark) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return lipgloss.NewStyle().Width(width).Render(wordmark + strings.Repeat(" ", gap) + right)
}

func wizardTuiDivider(width int) string {
	if width < 1 {
		width = 1
	}
	return wizardTuiDividerStyle.Render(strings.Repeat("─", width))
}

func renderWizardTuiFooter(m wizardTuiModel) string {
	return lipgloss.NewStyle().Width(m.frameWidth()).Render(wizardTuiHelpStyle.Render(m.helpText()))
}

func (m wizardTuiModel) helpText() string {
	switch m.stage {
	case wizardTuiStageWelcome:
		return "↑/↓ move · Enter start · q exit"
	case wizardTuiStageUseMode, wizardTuiStageEngine, wizardTuiStageSetupDepth:
		return "↑/↓ move · Enter/space select · Esc back · q exit"
	case wizardTuiStageProfile:
		return "type to edit · Enter continue · Esc back · Ctrl+C cancel"
	case wizardTuiStageDetection:
		if m.status == wizardTuiStatusError {
			return "r retry · Esc change selections · q exit · Ctrl+C cancel"
		}
		return "preparing plan… · Esc change selections · q exit · Ctrl+C cancel"
	case wizardTuiStagePlan:
		// The disclosure label must state what pressing d will do, so the
		// compact and expanded reviews never show the same misleading help.
		details := "d show details"
		if m.details {
			details = "d collapse details"
		}
		if m.plan.Blocked() {
			return "↑/↓ scroll · PgUp/PgDn page · " + details + " · r retry · Esc change · q exit"
		}
		return "↑/↓ scroll · PgUp/PgDn page · " + details + " · Enter finish preview · Esc back · q exit"
	case wizardTuiStageFinish:
		return "↑/↓ scroll · PgUp/PgDn page · Enter or q to exit · nothing was installed"
	default:
		return ""
	}
}

func (m wizardTuiModel) bodyContent(width int) string {
	switch m.stage {
	case wizardTuiStageWelcome:
		return renderWizardTuiWelcome(m, width)
	case wizardTuiStageUseMode, wizardTuiStageEngine, wizardTuiStageSetupDepth:
		return renderWizardTuiChoices(m, width)
	case wizardTuiStageProfile:
		return renderWizardTuiProfile(m, width)
	case wizardTuiStageDetection:
		return renderWizardTuiDetection(m, width)
	case wizardTuiStagePlan:
		return renderWizardTuiPlan(m, width)
	case wizardTuiStageFinish:
		return renderWizardTuiFinish(m, width)
	default:
		return ""
	}
}

func wrapWizardTuiBody(lines []string, width int) string {
	if width < 8 {
		width = 8
	}
	return lipgloss.NewStyle().Width(width).Render(strings.Join(lines, "\n"))
}

func wizardTuiChoiceLine(selected bool, label, note string, deferred bool) string {
	marker := "  "
	labelStyle := wizardTuiOptionStyle
	if selected {
		// The ">" marker keeps focus visible without relying on color alone.
		marker = wizardTuiAccentStyle.Render("> ")
		labelStyle = wizardTuiSelectedStyle
	}
	line := marker + labelStyle.Render(label)
	if deferred {
		line += " " + wizardTuiWarnStyle.Render("[deferred]")
	}
	if note != "" {
		line += "  " + wizardTuiNoteStyle.Render(note)
	}
	return line
}

func renderWizardTuiWelcome(m wizardTuiModel, width int) string {
	lines := []string{
		wizardTuiBodyStyle.Render("This guided preview inspects the workspace and builds the real"),
		wizardTuiBodyStyle.Render("runtime prerequisite plan. It never installs, approves, or writes."),
		"",
		wizardTuiChoiceLine(m.cursor == 0, "Start", "build the prerequisite plan preview", false),
		wizardTuiChoiceLine(m.cursor == 1, "Exit", "leave without changing anything", false),
	}
	return wrapWizardTuiBody(lines, width)
}

func renderWizardTuiChoices(m wizardTuiModel, width int) string {
	lines := make([]string, 0)
	switch m.stage {
	case wizardTuiStageUseMode:
		lines = append(lines, wizardTuiNoteStyle.Render("What are you setting up?"), "")
	case wizardTuiStageEngine:
		lines = append(lines, wizardTuiNoteStyle.Render("Which engine context should the studio assume?"), "")
	case wizardTuiStageSetupDepth:
		lines = append(lines, wizardTuiNoteStyle.Render("How much default scaffolding should the plan assume?"), "")
	}
	for i, choice := range m.currentChoices() {
		lines = append(lines, wizardTuiChoiceLine(i == m.cursor, choice.Label, choice.Note, choice.Deferred))
	}
	return wrapWizardTuiBody(lines, width)
}

func renderWizardTuiProfile(m wizardTuiModel, width int) string {
	lines := []string{
		wizardTuiNoteStyle.Render("Name the profile a future install would generate."),
		"",
		"  " + m.profile.View(),
	}
	if m.inputError != "" {
		lines = append(lines, "", wizardTuiDangerStyle.Render(m.inputError))
	}
	lines = append(lines, "", wizardTuiNoteStyle.Render("Press Enter to prepare the prerequisite plan preview."))
	return wrapWizardTuiBody(lines, width)
}

func renderWizardTuiDetection(m wizardTuiModel, width int) string {
	if m.status == wizardTuiStatusLoading {
		lines := []string{
			m.spinner.View() + " " + wizardTuiBodyStyle.Render("Inspecting the workspace and preparing the real prerequisite plan…"),
			"",
			wizardTuiNoteStyle.Render("Read-only: no install, config, artifact, or memory write happens here."),
		}
		return wrapWizardTuiBody(lines, width)
	}
	// A prepare failure with no valid plan is a distinct error state; a blocked
	// plan is NOT an error and is reviewed in the Plan stage instead.
	lines := []string{wizardTuiDangerStyle.Render("Plan preparation failed."), ""}
	if m.prepareErr != nil {
		lines = append(lines, wizardTuiBodyStyle.Render(m.prepareErr.Error()))
	}
	lines = append(lines, "", wizardTuiNoteStyle.Render("Press r to retry, Esc to change selections."))
	lines = appendWizardTuiPreflight(lines, m.preflight)
	return wrapWizardTuiBody(lines, width)
}

func appendWizardTuiPreflight(lines, preflight []string) []string {
	if len(preflight) == 0 {
		return lines
	}
	lines = append(lines, "", wizardTuiSectionStyle.Render("Preflight"))
	for _, line := range preflight {
		lines = append(lines, wizardTuiNoteStyle.Render("  "+line))
	}
	return lines
}

// wizardTuiPlanDetailsHint is the explicit compact-mode notice. It names the
// exact material Details reveals so a compact review never implies that paths,
// effects, or the fingerprint do not exist.
const wizardTuiPlanDetailsHint = "press d for exact details: reasons, destinations, effects, observed state, commands, fingerprint"

// renderWizardTuiPlan renders the prerequisite plan. The default view is a
// compact, scannable component/action list; Details expands every exact field.
// Both modes keep the blocked banner, the read-only cue, and the group counts.
func renderWizardTuiPlan(m wizardTuiModel, width int) string {
	plan := m.plan
	lines := make([]string, 0, 24)
	if plan.Blocked() {
		lines = append(lines,
			wizardTuiDangerStyle.Render("BLOCKED — this plan still needs human resolution."),
			wizardTuiNoteStyle.Render("This is a read-only review: nothing here approves, installs, or writes."),
			wizardTuiNoteStyle.Render("Resolve the blocked component, then retry or change selections."),
		)
	}
	lines = append(lines,
		wizardTuiBodyStyle.Render(fmt.Sprintf("status: %s    godot required: %t", wizardPlanStatus(plan), plan.GodotRequired)),
		wizardTuiNoteStyle.Render("Read-only preview: nothing is installed, approved, or written."),
	)
	if counts := wizardTuiPlanCounts(plan); counts != "" {
		lines = append(lines, wizardTuiAccentStyle.Render(counts))
	}
	if m.details {
		lines = append(lines, wizardTuiNoteStyle.Render("exact fingerprint: "+plan.Fingerprint()))
	} else {
		lines = append(lines, wizardTuiNoteStyle.Render(wizardTuiPlanDetailsHint))
	}
	lines = append(lines, "")
	groups := wizardTuiPlanGroups(plan)
	if len(groups) == 0 {
		lines = append(lines, wizardTuiBodyStyle.Render("No prerequisite actions: every component is already compatible."))
	}
	for _, group := range groups {
		lines = append(lines, wizardTuiSectionStyle.Render(fmt.Sprintf("%s (%d)", strings.ToUpper(group.Title), len(group.Steps))))
		lines = append(lines, wizardTuiNoteStyle.Render("  "+group.Note))
		for _, step := range group.Steps {
			lines = append(lines, renderWizardTuiPlanStep(step, m.details)...)
		}
		lines = append(lines, "")
	}
	lines = appendWizardTuiPreflight(lines, m.preflight)
	return wrapWizardTuiBody(lines, width)
}

// wizardTuiPlanCounts summarizes the real grouping so the compact review still
// communicates how much work the plan contains.
func wizardTuiPlanCounts(plan piinstall.Plan) string {
	groups := wizardTuiPlanGroups(plan)
	counts := make([]string, 0, len(groups))
	for _, group := range groups {
		counts = append(counts, fmt.Sprintf("%d %s", len(group.Steps), strings.ToLower(group.Title)))
	}
	return strings.Join(counts, " · ")
}

// renderWizardTuiPlanStep renders one component row. Compact mode keeps the
// component, its action, and any blocking reason visible; Details reveals the
// exact reason, destinations, effects, observed state, and raw command so no
// material field is silently dropped.
func renderWizardTuiPlanStep(step piinstall.PlanStep, details bool) []string {
	lines := []string{
		wizardTuiBodyStyle.Render(fmt.Sprintf("  > %s [%s]", step.Component, step.Action)),
	}
	if !details {
		// A blocked component's reason must stay visible without expanding
		// anything: the fail-closed cause is part of the compact review.
		if step.Action == piinstall.ActionBlocked && step.Reason != "" {
			lines = append(lines, wizardTuiDangerStyle.Render("      ✗ "+step.Reason))
		}
		return lines
	}
	if step.Reason != "" {
		lines = append(lines, wizardTuiNoteStyle.Render("      "+step.Reason))
	}
	for _, output := range step.Outputs {
		lines = append(lines, wizardTuiAccentStyle.Render("      → "+output))
	}
	for _, effect := range step.Effects {
		lines = append(lines, wizardTuiWarnStyle.Render("      ! "+effect))
	}
	lines = append(lines, wizardTuiNoteStyle.Render("      · observed: "+describeComponentState(step.Observed)))
	if step.Command != "" {
		lines = append(lines, wizardTuiNoteStyle.Render("      $ "+step.Command))
	}
	return lines
}

// renderWizardTuiFinish renders the no-install completion summary.
func renderWizardTuiFinish(m wizardTuiModel, width int) string {
	snapshot := m.snapshot
	lines := make([]string, 0, 28)
	if m.plan.Blocked() {
		lines = append(lines,
			wizardTuiDangerStyle.Render("BLOCKED plan — resolve the blocked components before any install."),
			wizardTuiNoteStyle.Render("This preview cannot approve or execute; the blocked state carries into the outcome."),
			"",
		)
	}
	lines = append(lines,
		wizardTuiSectionStyle.Render("Plan preview complete — nothing was installed, approved, or written."),
		"",
		wizardTuiBodyStyle.Render("  profile:      "+snapshot.ProfileName),
		wizardTuiBodyStyle.Render("  workflow:     "+wizardTuiChoiceLabel(m.useModeChoices, snapshot.UseMode)),
		wizardTuiBodyStyle.Render("  engine:       "+wizardTuiChoiceLabel(m.engineChoices, snapshot.EnginePack)),
		wizardTuiBodyStyle.Render("  setup depth:  "+wizardTuiChoiceLabel(m.depthChoices, snapshot.SetupDepth)),
		wizardTuiBodyStyle.Render("  status:       "+wizardPlanStatus(m.plan)),
		wizardTuiBodyStyle.Render("  fingerprint:  "+m.plan.Fingerprint()),
		"",
		wizardTuiNoteStyle.Render("First cut: this preview edits project essentials only. Optional integrations,"),
		wizardTuiNoteStyle.Render("resource editing, and real installation stay on the existing wizard paths."),
		"",
		wizardTuiBodyStyle.Render("Replay this exact read-only preview:"),
		wizardTuiAccentStyle.Render("  "+wizardReplayCommand(wizardTuiReplayArgs(snapshot, m.options.outDir, m.options.finalArtifactPath), "--plan-only")),
		"",
		wizardTuiNoteStyle.Render("Installation and approval are not available from this preview."),
	)
	return wrapWizardTuiBody(lines, width)
}

func renderWizardTuiTooSmall(width, height int) string {
	if width < 1 {
		width = 1
	}
	lines := []string{
		wizardTuiWordmarkStyle.Render("OGS · studio wizard"),
		fmt.Sprintf("Terminal too small (%dx%d; need at least %dx%d).", width, height, wizardTuiMinWidth, wizardTuiMinHeight),
		"Resize the window, or run:",
		"  game-studio wizard --non-interactive --plan-only",
		"Press q to exit.",
	}
	visible := lines
	if height > 0 && height < len(lines) {
		visible = lines[:height]
	}
	return lipgloss.NewStyle().Width(max(width, 1)).Render(strings.Join(visible, "\n"))
}
