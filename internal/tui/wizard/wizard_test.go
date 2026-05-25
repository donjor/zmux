package wizard

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/tui/styles"
	"github.com/donjor/zmux/internal/tui/tkey"
)

func newTestWizard() WizardModel {
	resolver := newTestThemeResolver()
	styles := styles.DefaultStyles()
	fs := &noopFS{}
	return NewWizardModel(fs, resolver, "dev", styles, config.ProfileFromArgv("zmux", fs))
}

func sendWizardKey(model WizardModel, keyStr string) WizardModel {
	msg := tkey.Type(keyStr)

	switch keyStr {
	case "enter":
		msg = tkey.Enter()
	case "esc":
		msg = tkey.Esc()
	case "up":
		msg = tkey.Up()
	case "down":
		msg = tkey.Down()
	case "ctrl+c":
		msg = tkey.Ctrl('c')
	case "shift+tab":
		msg = tkey.ShiftTab()
	}

	result, _ := model.Update(msg)
	return result.(WizardModel)
}

func sendWizardMsg(model WizardModel, msg tea.Msg) WizardModel {
	result, _ := model.Update(msg)
	return result.(WizardModel)
}

func TestWizardStartsAtWelcome(t *testing.T) {
	model := newTestWizard()

	if model.Step() != stepWelcome {
		t.Errorf("expected step %d (welcome), got %d", stepWelcome, model.Step())
	}
}

func TestWizardForwardNavigation(t *testing.T) {
	model := newTestWizard()

	// Welcome -> DepCheck (enter triggers async dep detect).
	model = sendWizardKey(model, "enter")
	if model.Step() != stepDepCheck {
		t.Errorf("expected step %d (depcheck), got %d", stepDepCheck, model.Step())
	}

	// Simulate deps detected.
	model = sendWizardMsg(model, detectDepsMsg{deps: WizardDeps{TmuxVersion: "3.4"}})

	// DepCheck -> DetectTargets.
	model = sendWizardKey(model, "enter")
	if model.Step() != stepDetectTargets {
		t.Errorf("expected step %d (detect targets), got %d", stepDetectTargets, model.Step())
	}

	// DetectTargets -> Theme.
	model = sendWizardKey(model, "enter")
	if model.Step() != stepTheme {
		t.Errorf("expected step %d (theme), got %d", stepTheme, model.Step())
	}

	// Theme -> BarPreset.
	model = sendWizardKey(model, "enter")
	if model.Step() != stepBarPreset {
		t.Errorf("expected step %d (bar preset), got %d", stepBarPreset, model.Step())
	}

	// BarPreset -> SyncTarget.
	model = sendWizardKey(model, "enter")
	if model.Step() != stepSyncTarget {
		t.Errorf("expected step %d (sync target), got %d", stepSyncTarget, model.Step())
	}

	// SyncTarget -> Summary.
	model = sendWizardKey(model, "enter")
	if model.Step() != stepSummary {
		t.Errorf("expected step %d (summary), got %d", stepSummary, model.Step())
	}
}

func TestWizardBackNavigation(t *testing.T) {
	model := newTestWizard()

	// Advance to theme step.
	model = sendWizardKey(model, "enter") // -> depcheck
	model = sendWizardMsg(model, detectDepsMsg{deps: WizardDeps{TmuxVersion: "3.4"}})
	model = sendWizardKey(model, "enter") // -> detect targets
	model = sendWizardKey(model, "enter") // -> theme

	if model.Step() != stepTheme {
		t.Fatalf("expected theme step, got %d", model.Step())
	}

	// Go back with shift+tab.
	model = sendWizardKey(model, "shift+tab")
	if model.Step() != stepDetectTargets {
		t.Errorf("expected step %d after back, got %d", stepDetectTargets, model.Step())
	}

	// Go back again with 'b'.
	model = sendWizardKey(model, "b")
	if model.Step() != stepDepCheck {
		t.Errorf("expected step %d after second back, got %d", stepDepCheck, model.Step())
	}
}

func TestWizardCancel(t *testing.T) {
	model := newTestWizard()

	model = sendWizardKey(model, "ctrl+c")
	if !model.Cancelled {
		t.Error("expected Cancelled to be true after ctrl+c")
	}
}

func TestWizardThemeNavigation(t *testing.T) {
	model := newTestWizard()

	// Advance to theme step.
	model = sendWizardKey(model, "enter") // -> depcheck
	model = sendWizardMsg(model, detectDepsMsg{deps: WizardDeps{TmuxVersion: "3.4"}})
	model = sendWizardKey(model, "enter") // -> detect targets
	model = sendWizardKey(model, "enter") // -> theme

	initialCursor := model.themeCursor

	// Move down.
	model = sendWizardKey(model, "j")
	if model.themeCursor != initialCursor+1 {
		t.Errorf("expected cursor %d after j, got %d", initialCursor+1, model.themeCursor)
	}

	// Move up.
	model = sendWizardKey(model, "k")
	if model.themeCursor != initialCursor {
		t.Errorf("expected cursor %d after k, got %d", initialCursor, model.themeCursor)
	}
}

func TestWizardPresetNavigation(t *testing.T) {
	model := newTestWizard()

	// Advance to bar preset step.
	model = sendWizardKey(model, "enter") // -> depcheck
	model = sendWizardMsg(model, detectDepsMsg{deps: WizardDeps{TmuxVersion: "3.4"}})
	model = sendWizardKey(model, "enter") // -> detect targets
	model = sendWizardKey(model, "enter") // -> theme
	model = sendWizardKey(model, "enter") // -> bar preset

	if model.Step() != stepBarPreset {
		t.Fatalf("expected bar preset step, got %d", model.Step())
	}

	// Move down.
	model = sendWizardKey(model, "j")
	if model.presetCursor != 1 {
		t.Errorf("expected preset cursor 1 after j, got %d", model.presetCursor)
	}

	// Confirm selection.
	model = sendWizardKey(model, "enter")
	if model.chosenPreset != "minimal" {
		t.Errorf("expected chosen preset 'minimal', got %q", model.chosenPreset)
	}
}

func TestWizardConfigWritten(t *testing.T) {
	model := newTestWizard()

	// Advance to summary.
	model = sendWizardKey(model, "enter") // -> depcheck
	model = sendWizardMsg(model, detectDepsMsg{deps: WizardDeps{TmuxVersion: "3.4"}})
	model = sendWizardKey(model, "enter") // -> detect targets
	model = sendWizardKey(model, "enter") // -> theme
	model = sendWizardKey(model, "enter") // -> bar preset
	model = sendWizardKey(model, "enter") // -> sync target
	model = sendWizardKey(model, "enter") // -> summary

	if model.Step() != stepSummary {
		t.Fatalf("expected summary step, got %d", model.Step())
	}

	// Trigger write.
	model = sendWizardKey(model, "enter") // -> writing

	if model.Step() != stepWriting {
		t.Fatalf("expected writing step, got %d", model.Step())
	}

	// Simulate config written.
	model = sendWizardMsg(model, configWrittenMsg{})
	if model.Step() != stepSuccess {
		t.Errorf("expected success step, got %d", model.Step())
	}
	if !model.Done {
		t.Error("expected Done to be true after config written")
	}
}

func TestWizardConfigWriteError(t *testing.T) {
	model := newTestWizard()

	// Simulate config write error from writing step.
	model.step = stepWriting
	model = sendWizardMsg(model, configWriteErrMsg{err: errors.New("disk full")})

	if model.Step() != stepSuccess {
		t.Errorf("expected success step (with error), got %d", model.Step())
	}
	if model.Error == nil {
		t.Error("expected non-nil Error after config write failure")
	}
}

func TestWizardViewRendersContent(t *testing.T) {
	model := newTestWizard()
	model.width = 80
	model.height = 40

	// Welcome view.
	view := model.view()
	if !strings.Contains(view, "zmux init") {
		t.Error("expected welcome view to contain 'zmux init'")
	}
	if !strings.Contains(view, "Welcome") {
		t.Error("expected welcome view to contain 'Welcome'")
	}

	// Dep check view.
	model = sendWizardKey(model, "enter")
	model = sendWizardMsg(model, detectDepsMsg{deps: WizardDeps{TmuxVersion: "3.4", ClipboardTool: "wl-copy"}})
	view = model.view()
	if !strings.Contains(view, "Dependencies") {
		t.Error("expected dep check view to contain 'Dependencies'")
	}

	// Summary view.
	model.step = stepSummary
	view = model.view()
	if !strings.Contains(view, "Summary") {
		t.Error("expected summary view to contain 'Summary'")
	}
	if !strings.Contains(view, model.chosenTheme) {
		t.Errorf("expected summary to contain chosen theme %q", model.chosenTheme)
	}
}

func TestWizardDetectsGhosttyTarget(t *testing.T) {
	model := newTestWizard()

	// Simulate deps with ghostty and nvim.
	deps := WizardDeps{
		TmuxVersion: "3.4",
		HasGhostty:  true,
		HasNvim:     true,
	}
	model = sendWizardMsg(model, detectDepsMsg{deps: deps})

	// Sync targets should include ghostty and nvim.
	if len(model.syncTargets) != 3 {
		t.Errorf("expected 3 sync targets (none, ghostty, nvim), got %d: %v",
			len(model.syncTargets), model.syncTargets)
	}
}

func TestWizardCancelledViewEmpty(t *testing.T) {
	model := newTestWizard()
	model.Cancelled = true

	view := model.view()
	if view != "" {
		t.Errorf("expected empty view when cancelled, got %q", view)
	}
}

func TestWizardSuccessViewShowsNextSteps(t *testing.T) {
	model := newTestWizard()
	model.step = stepSuccess
	model.Done = true
	model.width = 80
	model.height = 40

	view := model.view()
	if !strings.Contains(view, "successfully") {
		t.Error("expected success view to contain 'successfully'")
	}
	if !strings.Contains(view, "tmux source-file") {
		t.Error("expected success view to contain source-file instruction")
	}
}

// ── Extra coverage added for P8 ──

// Walk to a specific step with detectDepsMsg injected along the way.
func walkToStep(t *testing.T, target wizardStep) WizardModel {
	t.Helper()
	m := newTestWizard()
	m.width = 80
	m.height = 40
	if target == stepWelcome {
		return m
	}
	m = sendWizardKey(m, "enter") // → depcheck
	m = sendWizardMsg(m, detectDepsMsg{deps: WizardDeps{TmuxVersion: "3.4"}})
	if target == stepDepCheck {
		return m
	}
	m = sendWizardKey(m, "enter") // → detect targets
	if target == stepDetectTargets {
		return m
	}
	m = sendWizardKey(m, "enter") // → theme
	if target == stepTheme {
		return m
	}
	m = sendWizardKey(m, "enter") // → bar preset
	if target == stepBarPreset {
		return m
	}
	m = sendWizardKey(m, "enter") // → sync target
	if target == stepSyncTarget {
		return m
	}
	m = sendWizardKey(m, "enter") // → summary
	if target == stepSummary {
		return m
	}
	m = sendWizardKey(m, "enter") // → writing
	return m
}

func TestWizardChosenAccessors(t *testing.T) {
	m := newTestWizard()
	if m.ChosenTheme() != "ayu-dark" {
		t.Errorf("ChosenTheme() = %q, want ayu-dark", m.ChosenTheme())
	}
	if m.ChosenPreset() != "default" {
		t.Errorf("ChosenPreset() = %q, want default", m.ChosenPreset())
	}
	if m.ChosenSync() != "none" {
		t.Errorf("ChosenSync() = %q, want none", m.ChosenSync())
	}
}

func TestWizardInitReturnsNil(t *testing.T) {
	m := newTestWizard()
	if cmd := m.Init(); cmd != nil {
		t.Errorf("Init() should return nil, got %v", cmd)
	}
}

func TestWizardHandleSuccessEnterQuits(t *testing.T) {
	m := newTestWizard()
	m.step = stepSuccess
	_, cmd := m.Update(tkey.Enter())
	if cmd == nil {
		t.Error("enter on success should return tea.Quit cmd")
	}
}

func TestWizardHandleSuccessQQuits(t *testing.T) {
	m := newTestWizard()
	m.step = stepSuccess
	_, cmd := m.Update(tkey.Rune('q'))
	if cmd == nil {
		t.Error("q on success should return tea.Quit cmd")
	}
}

func TestWizardHandleSyncTargetCursor(t *testing.T) {
	m := walkToStep(t, stepSyncTarget)
	// Only "none" is available without ghostty/nvim — seed extra targets
	// to exercise the cursor.
	m.syncTargets = []string{"none", "ghostty", "nvim"}

	m = sendWizardKey(m, "j")
	if m.syncCursor != 1 {
		t.Errorf("after j: syncCursor = %d, want 1", m.syncCursor)
	}
	m = sendWizardKey(m, "j")
	if m.syncCursor != 2 {
		t.Errorf("after j: syncCursor = %d, want 2", m.syncCursor)
	}
	// Bottom clamp.
	m = sendWizardKey(m, "j")
	if m.syncCursor != 2 {
		t.Errorf("at bottom: syncCursor = %d, want 2", m.syncCursor)
	}
	m = sendWizardKey(m, "k")
	if m.syncCursor != 1 {
		t.Errorf("after k: syncCursor = %d, want 1", m.syncCursor)
	}
	// Enter commits the selection.
	m = sendWizardKey(m, "enter")
	if m.ChosenSync() != "ghostty" {
		t.Errorf("after enter: ChosenSync() = %q, want ghostty", m.ChosenSync())
	}
	if m.Step() != stepSummary {
		t.Errorf("after enter: Step() = %d, want stepSummary", m.Step())
	}
}

func TestWizardThemeEnterCommitsSelection(t *testing.T) {
	m := walkToStep(t, stepTheme)
	m.themeCursor = 0
	originalTheme := m.themes[0].Name

	m = sendWizardKey(m, "enter")
	if m.ChosenTheme() != originalTheme {
		t.Errorf("after enter: ChosenTheme = %q, want %q", m.ChosenTheme(), originalTheme)
	}
	if m.Step() != stepBarPreset {
		t.Errorf("after enter: Step = %d, want stepBarPreset", m.Step())
	}
}

// ── View rendering for every step ──

func TestWizardViewsRenderWithoutPanic(t *testing.T) {
	// Cover each per-step view path with a non-zero terminal size.
	for _, step := range []wizardStep{
		stepWelcome, stepDepCheck, stepDetectTargets, stepTheme,
		stepBarPreset, stepSyncTarget, stepSummary, stepWriting, stepSuccess,
	} {
		m := newTestWizard()
		m.width = 80
		m.height = 40
		m.step = step
		// Seed deps so dep-check view has content to render.
		m.deps = WizardDeps{TmuxVersion: "3.4", ClipboardTool: "wl-copy"}

		view := m.view()
		if view == "" {
			t.Errorf("step %d: View() returned empty", step)
		}
	}
}

func TestWizardBackNavFromWelcomeIsNoop(t *testing.T) {
	m := newTestWizard()
	m = sendWizardKey(m, "shift+tab")
	if m.Step() != stepWelcome {
		t.Errorf("shift+tab on welcome advanced to %d", m.Step())
	}
}

func TestWizardCanNavigateBackBoundary(t *testing.T) {
	// Back-nav should be disabled on welcome, writing, and success.
	for _, step := range []wizardStep{stepWelcome, stepWriting, stepSuccess} {
		m := newTestWizard()
		m.step = step
		if m.canNavigateBack() {
			t.Errorf("canNavigateBack() true on step %d, want false", step)
		}
	}
	// And enabled on the middle steps.
	for _, step := range []wizardStep{stepDepCheck, stepDetectTargets, stepTheme, stepBarPreset, stepSyncTarget, stepSummary} {
		m := newTestWizard()
		m.step = step
		if !m.canNavigateBack() {
			t.Errorf("canNavigateBack() false on step %d, want true", step)
		}
	}
}

func TestRestartCmdExported(t *testing.T) {
	fs := &noopFS{}
	if s := RestartCmd(config.ProfileFromArgv("zmux", fs)); s == "" {
		t.Error("RestartCmd() returned empty string")
	}
	zz := RestartCmd(config.ProfileFromArgv("zzmux", fs))
	if !strings.Contains(zz, "-L zzmux") {
		t.Errorf("zzmux RestartCmd should target -L zzmux, got %q", zz)
	}
}
