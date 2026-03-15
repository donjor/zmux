package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func newTestPaletteActions() []PaletteAction {
	return []PaletteAction{
		{Name: "Switch Session", Hotkey: "enter"},
		{Name: "New Session", Hotkey: "n"},
		{Name: "New from Template", Hotkey: "t"},
		{Name: "Rename Session", Hotkey: "r"},
		{Name: "Kill Session", Hotkey: "d"},
		{Name: "Theme Browser", Hotkey: "T"},
		{Name: "Cleanup Tmp", Hotkey: "c"},
	}
}

func newTestPalette() PaletteModel {
	styles := DefaultStyles()
	return NewPaletteModel(newTestPaletteActions(), styles)
}

func sendPaletteKey(model PaletteModel, keyStr string) PaletteModel {
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
	case "tab":
		msg = tea.KeyMsg{Type: tea.KeyTab}
	}

	result, _ := model.Update(msg)
	return result
}

func TestPaletteInitShowsAllActions(t *testing.T) {
	m := newTestPalette()

	if len(m.filtered) != 7 {
		t.Fatalf("expected 7 filtered actions, got %d", len(m.filtered))
	}
}

func TestPaletteNavigateUpDown(t *testing.T) {
	m := newTestPalette()

	if m.cursor != 0 {
		t.Fatalf("expected cursor at 0, got %d", m.cursor)
	}

	m = sendPaletteKey(m, "down")
	if m.cursor != 1 {
		t.Errorf("expected cursor at 1 after down, got %d", m.cursor)
	}

	m = sendPaletteKey(m, "down")
	if m.cursor != 2 {
		t.Errorf("expected cursor at 2 after down, got %d", m.cursor)
	}

	m = sendPaletteKey(m, "up")
	if m.cursor != 1 {
		t.Errorf("expected cursor at 1 after up, got %d", m.cursor)
	}

	// Should not go below 0.
	m = sendPaletteKey(m, "up")
	m = sendPaletteKey(m, "up")
	if m.cursor != 0 {
		t.Errorf("expected cursor clamped at 0, got %d", m.cursor)
	}
}

func TestPaletteEscDismisses(t *testing.T) {
	m := newTestPalette()

	m = sendPaletteKey(m, "esc")
	if !m.Quitting {
		t.Error("expected Quitting to be true after esc")
	}
}

func TestPaletteEnterChoosesAction(t *testing.T) {
	m := newTestPalette()

	m = sendPaletteKey(m, "enter")
	if m.Chosen == nil {
		t.Fatal("expected Chosen to be non-nil after enter")
	}
	if m.Chosen.Name != "Switch Session" {
		t.Errorf("expected chosen action 'Switch Session', got %q", m.Chosen.Name)
	}
}

func TestPaletteFuzzySearch(t *testing.T) {
	m := newTestPalette()

	// Type "kill" to filter.
	m = sendPaletteKey(m, "k")
	m = sendPaletteKey(m, "i")
	m = sendPaletteKey(m, "l")
	m = sendPaletteKey(m, "l")

	if len(m.filtered) != 1 {
		t.Fatalf("expected 1 filtered action for 'kill', got %d", len(m.filtered))
	}
	if m.filtered[0].Name != "Kill Session" {
		t.Errorf("expected 'Kill Session', got %q", m.filtered[0].Name)
	}
}

func TestPaletteFuzzySearchPartial(t *testing.T) {
	m := newTestPalette()

	// Type "new" to filter - should match "New Session" and "New from Template".
	m = sendPaletteKey(m, "n")
	m = sendPaletteKey(m, "e")
	m = sendPaletteKey(m, "w")

	if len(m.filtered) < 2 {
		t.Fatalf("expected at least 2 filtered actions for 'new', got %d", len(m.filtered))
	}
}

func TestPaletteReset(t *testing.T) {
	m := newTestPalette()

	// Filter and choose.
	m = sendPaletteKey(m, "k")
	m = sendPaletteKey(m, "enter")

	m.Reset()

	if m.Chosen != nil {
		t.Error("expected Chosen to be nil after reset")
	}
	if m.Quitting {
		t.Error("expected Quitting to be false after reset")
	}
	if len(m.filtered) != len(m.actions) {
		t.Errorf("expected all actions after reset, got %d", len(m.filtered))
	}
	if m.cursor != 0 {
		t.Errorf("expected cursor at 0 after reset, got %d", m.cursor)
	}
}

func TestPaletteViewRendersContent(t *testing.T) {
	m := newTestPalette()
	m.width = 80
	m.height = 40

	view := m.View()

	if !strings.Contains(view, "zmux") {
		t.Error("expected view to contain 'zmux' title")
	}
	if !strings.Contains(view, "command palette") {
		t.Error("expected view to contain 'command palette' subtitle")
	}
	if !strings.Contains(view, "Switch Session") {
		t.Error("expected view to contain 'Switch Session' action")
	}
	if !strings.Contains(view, "enter:execute") {
		t.Error("expected view to contain help text")
	}
}

func TestPaletteViewShowsHotkeys(t *testing.T) {
	m := newTestPalette()
	m.width = 80
	m.height = 40

	view := m.View()

	// Check that hotkey hints are shown.
	for _, a := range m.actions {
		if a.Hotkey != "" && !strings.Contains(view, a.Hotkey) {
			t.Errorf("expected view to contain hotkey %q for action %q", a.Hotkey, a.Name)
		}
	}
}

func TestPaletteWindowSizeMsg(t *testing.T) {
	m := newTestPalette()

	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	if m.width != 120 {
		t.Errorf("expected width 120, got %d", m.width)
	}
	if m.height != 40 {
		t.Errorf("expected height 40, got %d", m.height)
	}
}

func TestPaletteNoMatchesMessage(t *testing.T) {
	m := newTestPalette()
	m.width = 80
	m.height = 40

	// Type something that matches nothing.
	m = sendPaletteKey(m, "z")
	m = sendPaletteKey(m, "z")
	m = sendPaletteKey(m, "z")
	m = sendPaletteKey(m, "z")
	m = sendPaletteKey(m, "z")

	view := m.View()
	if !strings.Contains(view, "No matching actions") {
		t.Error("expected 'No matching actions' when filter matches nothing")
	}
}

func TestPaletteEnterWithHandler(t *testing.T) {
	handlerCalled := false
	actions := []PaletteAction{
		{
			Name:   "Test Action",
			Hotkey: "x",
			Handler: func() tea.Cmd {
				handlerCalled = true
				return nil
			},
		},
	}
	styles := DefaultStyles()
	m := NewPaletteModel(actions, styles)

	m = sendPaletteKey(m, "enter")

	if !handlerCalled {
		t.Error("expected handler to be called on enter")
	}
	if m.Chosen == nil || m.Chosen.Name != "Test Action" {
		t.Error("expected Chosen to be 'Test Action'")
	}
}
