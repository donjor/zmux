package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/donjor/zmux/internal/tmux"
)

func newTestModel() (PickerModel, *tmux.MockRunner) {
	m := tmux.NewMockRunner()
	now := time.Now()
	m.Sessions = []tmux.Session{
		{Name: "dev", Windows: 3, Attached: true, Activity: now, Dir: "/home/user/work"},
		{Name: "tmp-1", Windows: 1, Attached: false, Activity: now, Dir: "/tmp"},
		{Name: "alpha", Windows: 2, Attached: false, Activity: now, Dir: "/home/user"},
	}

	styles := DefaultStyles()
	model := NewPickerModel(m, styles)
	return model, m
}

// simulateInit runs the Init cmd and feeds the result back to Update.
func simulateInit(model PickerModel) PickerModel {
	cmd := model.Init()
	if cmd != nil {
		msg := cmd()
		result, _ := model.Update(msg)
		model = result.(PickerModel)
	}
	return model
}

func sendKey(model PickerModel, keyStr string) PickerModel {
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(keyStr)}

	// Handle special keys.
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
	}

	result, _ := model.Update(msg)
	return result.(PickerModel)
}

func TestPickerInitLoadsSessions(t *testing.T) {
	model, _ := newTestModel()
	model = simulateInit(model)

	if len(model.filtered) != 3 {
		t.Fatalf("expected 3 filtered sessions, got %d", len(model.filtered))
	}
}

func TestPickerNavigateUpDown(t *testing.T) {
	model, _ := newTestModel()
	model = simulateInit(model)

	if model.cursor != 0 {
		t.Fatalf("expected cursor at 0, got %d", model.cursor)
	}

	model = sendKey(model, "j")
	if model.cursor != 1 {
		t.Errorf("expected cursor at 1 after j, got %d", model.cursor)
	}

	model = sendKey(model, "j")
	if model.cursor != 2 {
		t.Errorf("expected cursor at 2 after j, got %d", model.cursor)
	}

	// Should not go past the end.
	model = sendKey(model, "j")
	if model.cursor != 2 {
		t.Errorf("expected cursor clamped at 2, got %d", model.cursor)
	}

	model = sendKey(model, "k")
	if model.cursor != 1 {
		t.Errorf("expected cursor at 1 after k, got %d", model.cursor)
	}
}

func TestPickerQuit(t *testing.T) {
	model, _ := newTestModel()
	model = simulateInit(model)

	model = sendKey(model, "q")
	if !model.Quitting {
		t.Error("expected Quitting to be true after q")
	}
	if model.Action != "" {
		t.Errorf("expected empty action after quit, got %q", model.Action)
	}
}

func TestPickerEnterAttaches(t *testing.T) {
	model, _ := newTestModel()
	model = simulateInit(model)

	// Select first session and press enter.
	model = sendKey(model, "enter")

	if model.Action != "attach" {
		t.Errorf("expected action 'attach', got %q", model.Action)
	}
	if model.Chosen == "" {
		t.Error("expected a chosen session name")
	}
}

func TestPickerNewSession(t *testing.T) {
	model, _ := newTestModel()
	model = simulateInit(model)

	model = sendKey(model, "n")
	if model.Action != "new" {
		t.Errorf("expected action 'new', got %q", model.Action)
	}
}

func TestPickerTemplate(t *testing.T) {
	model, _ := newTestModel()
	model = simulateInit(model)

	model = sendKey(model, "t")
	if model.Action != "template" {
		t.Errorf("expected action 'template', got %q", model.Action)
	}
}

func TestPickerDeleteConfirm(t *testing.T) {
	model, _ := newTestModel()
	model = simulateInit(model)

	// Press d to trigger delete confirmation.
	model = sendKey(model, "d")
	if model.mode != modeConfirmDelete {
		t.Error("expected modeConfirmDelete after pressing d")
	}

	// Cancel with any key other than y.
	model = sendKey(model, "n")
	if model.mode != modeList {
		t.Error("expected modeList after canceling delete")
	}
}

func TestPickerDeleteConfirmYes(t *testing.T) {
	model, mock := newTestModel()
	model = simulateInit(model)

	// Press d then y to confirm delete.
	model = sendKey(model, "d")
	model = sendKey(model, "y")

	// Should have called KillSession.
	found := false
	for _, c := range mock.Calls {
		if c.Method == "KillSession" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected KillSession to be called after confirming delete")
	}

	// Should be back in list mode.
	if model.mode != modeList {
		t.Error("expected modeList after confirming delete")
	}
}

func TestPickerViewRendersContent(t *testing.T) {
	model, _ := newTestModel()
	model = simulateInit(model)

	view := model.View()

	if !strings.Contains(view, "zmux") {
		t.Error("expected view to contain 'zmux' title")
	}
	if !strings.Contains(view, "session picker") {
		t.Error("expected view to contain 'session picker' subtitle")
	}
	if !strings.Contains(view, "enter:attach") {
		t.Error("expected view to contain help text")
	}
}

func TestPickerEmptySessions(t *testing.T) {
	m := tmux.NewMockRunner()
	styles := DefaultStyles()
	model := NewPickerModel(m, styles)
	model = simulateInit(model)

	view := model.View()
	if !strings.Contains(view, "No sessions found") {
		t.Error("expected 'No sessions found' for empty session list")
	}
}

func TestPickerWindowSizeMsg(t *testing.T) {
	model, _ := newTestModel()

	result, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model = result.(PickerModel)

	if model.width != 120 {
		t.Errorf("expected width 120, got %d", model.width)
	}
	if model.height != 40 {
		t.Errorf("expected height 40, got %d", model.height)
	}
}

func TestDefaultStylesCreated(t *testing.T) {
	styles := DefaultStyles()

	// Just verify the styles are non-zero value (they have been configured).
	view := styles.Title.Render("test")
	if view == "" {
		t.Error("expected Title style to render content")
	}
}

func TestNewStylesWithNilPalette(t *testing.T) {
	styles := NewStyles(nil)
	view := styles.Title.Render("test")
	if view == "" {
		t.Error("expected default styles when palette is nil")
	}
}

func TestShortenPath(t *testing.T) {
	tests := []struct {
		path string
		want string // just check it doesn't crash and produces something
	}{
		{"/tmp", "/tmp"},
		{"", ""},
	}

	for _, tt := range tests {
		result := shortenPath(tt.path)
		// Just verify it doesn't panic and returns something.
		if tt.path != "" && result == "" {
			t.Errorf("shortenPath(%q) returned empty string", tt.path)
		}
	}
}

func TestPickerFilterMode(t *testing.T) {
	model, _ := newTestModel()
	model = simulateInit(model)

	// Enter filter mode.
	model = sendKey(model, "/")
	if model.mode != modeFilter {
		t.Error("expected modeFilter after pressing /")
	}

	// Press esc to exit filter mode.
	model = sendKey(model, "esc")
	if model.mode != modeList {
		t.Error("expected modeList after pressing esc in filter mode")
	}
}

func TestPickerSessionInfo(t *testing.T) {
	// Test that session info displays window count.
	model, _ := newTestModel()
	model = simulateInit(model)

	view := model.View()
	if !strings.Contains(view, "3w") {
		t.Error("expected view to contain window count '3w' for dev session")
	}
}
