package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func newTestWizard() WizardModel {
	resolver := newTestThemeResolver()
	styles := DefaultStyles()
	fs := &noopFS{}
	return NewWizardModel(fs, resolver, "dev", styles)
}

func sendWizardKey(model WizardModel, keyStr string) WizardModel {
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(keyStr)}

	switch keyStr {
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEscape}
	case "up":
		msg = tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		msg = tea.KeyMsg{Type: tea.KeyDown}
	case "ctrl+c":
		msg = tea.KeyMsg{Type: tea.KeyCtrlC}
	case "shift+tab":
		msg = tea.KeyMsg{Type: tea.KeyShiftTab}
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
	view := model.View()
	if !strings.Contains(view, "zmux init") {
		t.Error("expected welcome view to contain 'zmux init'")
	}
	if !strings.Contains(view, "Welcome") {
		t.Error("expected welcome view to contain 'Welcome'")
	}

	// Dep check view.
	model = sendWizardKey(model, "enter")
	model = sendWizardMsg(model, detectDepsMsg{deps: WizardDeps{TmuxVersion: "3.4", ClipboardTool: "wl-copy"}})
	view = model.View()
	if !strings.Contains(view, "Dependencies") {
		t.Error("expected dep check view to contain 'Dependencies'")
	}

	// Summary view.
	model.step = stepSummary
	view = model.View()
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

	view := model.View()
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

	view := model.View()
	if !strings.Contains(view, "successfully") {
		t.Error("expected success view to contain 'successfully'")
	}
	if !strings.Contains(view, "tmux source-file") {
		t.Error("expected success view to contain source-file instruction")
	}
}
