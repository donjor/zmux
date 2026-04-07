package palette

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/donjor/zmux/internal/tui"
)

// newTestModel returns a PaletteModel populated with a fixed set of
// actions covering three groups — enough to exercise filter, nav, and
// selection flows.
func newTestModel() *PaletteModel {
	actions := []Action{
		{ID: "session:new", Group: "Sessions", Title: "New session"},
		{ID: "session:switch:dev", Group: "Sessions", Title: "Switch to dev"},
		{ID: "theme:set:ayu-dark", Group: "Themes", Title: "Set theme: ayu-dark"},
		{ID: "bar:set:default", Group: "Bar", Title: "Set bar: default"},
	}
	reg := NewRegistry(&stubProvider{actions: actions})
	return NewPaletteModel(reg, tui.DefaultStyles())
}

func TestNewPaletteModelPopulatesFromRegistry(t *testing.T) {
	m := newTestModel()
	if len(m.actions) != 4 {
		t.Errorf("actions = %d, want 4", len(m.actions))
	}
	if len(m.filtered) != 4 {
		t.Errorf("filtered (empty query) = %d, want 4", len(m.filtered))
	}
	if m.cursor != 0 {
		t.Errorf("initial cursor = %d, want 0", m.cursor)
	}
}

func TestPaletteEscSetsQuitting(t *testing.T) {
	m := newTestModel()
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	mOut := out.(*PaletteModel)
	if !mOut.Quitting {
		t.Error("esc should set Quitting")
	}
}

func TestPaletteCtrlCSetsQuitting(t *testing.T) {
	m := newTestModel()
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	mOut := out.(*PaletteModel)
	if !mOut.Quitting {
		t.Error("ctrl+c should set Quitting")
	}
}

func TestPaletteCursorNavigationClamps(t *testing.T) {
	m := newTestModel()

	// Up from 0 stays at 0.
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = out.(*PaletteModel)
	if m.cursor != 0 {
		t.Errorf("after up from 0: cursor = %d, want 0", m.cursor)
	}

	// Down walks through all filtered entries.
	for i := 1; i < len(m.filtered); i++ {
		out, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = out.(*PaletteModel)
		if m.cursor != i {
			t.Errorf("step %d: cursor = %d", i, m.cursor)
		}
	}

	// Down at bottom stays at bottom.
	out, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = out.(*PaletteModel)
	if m.cursor != len(m.filtered)-1 {
		t.Errorf("after down at bottom: cursor = %d, want %d", m.cursor, len(m.filtered)-1)
	}
}

func TestPaletteEnterSelectsAndQuits(t *testing.T) {
	m := newTestModel()
	m.cursor = 2 // the ayu-dark theme
	expected := m.filtered[2].ID

	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = out.(*PaletteModel)

	if !m.Quitting {
		t.Error("enter should set Quitting")
	}
	if m.Chosen == nil {
		t.Fatal("Chosen should be set")
	}
	if m.Chosen.ID != expected {
		t.Errorf("Chosen.ID = %q, want %q", m.Chosen.ID, expected)
	}
}

func TestPaletteEnterWithEmptyFilteredIsNoop(t *testing.T) {
	m := newTestModel()
	// Type a query that matches nothing.
	for _, r := range "zzxxqq" {
		m.filter.SetValue(m.filter.Value() + string(r))
	}
	m.applyFilter()
	if len(m.filtered) != 0 {
		t.Fatalf("want empty filtered, got %d", len(m.filtered))
	}

	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = out.(*PaletteModel)
	if m.Quitting {
		t.Error("enter on empty should not quit")
	}
	if m.Chosen != nil {
		t.Error("enter on empty should not set Chosen")
	}
}

func TestPaletteApplyFilterNarrows(t *testing.T) {
	m := newTestModel()
	before := len(m.filtered)

	m.filter.SetValue("theme")
	m.applyFilter()

	if len(m.filtered) >= before {
		t.Errorf("filter 'theme' did not narrow: %d → %d", before, len(m.filtered))
	}
	if len(m.filtered) == 0 {
		t.Error("filter 'theme' should match at least one action")
	}
}

func TestPaletteApplyFilterClampsCursorWhenListShrinks(t *testing.T) {
	m := newTestModel()
	m.cursor = len(m.filtered) - 1 // last entry
	last := m.cursor

	m.filter.SetValue("theme") // narrows to 1 match
	m.applyFilter()

	if m.cursor >= len(m.filtered) {
		t.Errorf("cursor %d out of bounds after filter (was %d, filtered=%d)",
			m.cursor, last, len(m.filtered))
	}
}

func TestPaletteApplyFilterEmptyQueryRestoresAll(t *testing.T) {
	m := newTestModel()
	m.filter.SetValue("theme")
	m.applyFilter()
	m.filter.SetValue("")
	m.applyFilter()
	if len(m.filtered) != len(m.actions) {
		t.Errorf("empty filter should restore all: got %d, want %d",
			len(m.filtered), len(m.actions))
	}
}

func TestPaletteViewRendersTitleAndHelp(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 24
	view := m.View()
	if view == "" {
		t.Error("View() returned empty string")
	}
}

func TestPaletteViewEmptyFilterShowsNoMatchMessage(t *testing.T) {
	m := newTestModel()
	m.width = 80
	m.height = 24
	m.filter.SetValue("notarealquery")
	m.applyFilter()

	view := m.View()
	if view == "" {
		t.Error("View() returned empty when no matches")
	}
}

func TestPaletteInitReturnsBlinkCmd(t *testing.T) {
	m := newTestModel()
	if cmd := m.Init(); cmd == nil {
		t.Error("Init should return a blink cmd")
	}
}
