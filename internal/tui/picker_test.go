package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tmux"
)

func newTestPicker() (PickerModel, *tmux.MockRunner) {
	m := tmux.NewMockRunner()
	now := time.Now()
	m.Sessions = []tmux.Session{
		{Name: "dev", Windows: 3, Attached: true, Activity: now, Created: now.Add(-2 * time.Hour), LastAttached: now.Add(-10 * time.Minute), Dir: "/home/user/work"},
		{Name: "tmp-1", Windows: 1, Attached: false, Activity: now, Created: now.Add(-5 * time.Minute), Dir: "/tmp"},
		{Name: "alpha", Windows: 2, Attached: false, Activity: now, Created: now.Add(-24 * time.Hour), LastAttached: now.Add(-1 * time.Hour), Dir: "/home/user"},
	}
	m.Windows["dev"] = []tmux.Window{
		{Index: 1, Name: "editor", Active: true},
		{Index: 2, Name: "server", Active: false},
		{Index: 3, Name: "git", Active: false},
	}
	m.Windows["tmp-1"] = []tmux.Window{
		{Index: 1, Name: "zsh", Active: true},
	}
	m.Windows["alpha"] = []tmux.Window{
		{Index: 1, Name: "vim", Active: true},
		{Index: 2, Name: "shell", Active: false},
	}

	styles := DefaultStyles()
	model := NewPickerModel(m, styles)

	sessions, _ := session.ListSessions(m)
	model.sessions = sessions
	model.filtered = sessions

	wins := make(map[string][]tmux.Window)
	for _, s := range sessions {
		w, _ := m.ListWindows(s.Name)
		wins[s.Name] = w
	}
	model.windows = wins

	return model, m
}

func newEmptyPicker() PickerModel {
	m := tmux.NewMockRunner()
	styles := DefaultStyles()
	model := NewPickerModel(m, styles)
	model.sessions = nil
	model.filtered = nil
	return model
}

// ── Basic rendering ──

func TestPickerRendersSessions(t *testing.T) {
	model, _ := newTestPicker()
	view := model.View()

	for _, name := range []string{"alpha", "dev", "tmp-1"} {
		if !strings.Contains(view, name) {
			t.Errorf("expected %q in view", name)
		}
	}
}

func TestPickerShowsNewSessionEntry(t *testing.T) {
	model, _ := newTestPicker()
	view := model.View()

	if !strings.Contains(view, "+ new session") {
		t.Error("expected '+ new session' entry in view")
	}
}

func TestPickerShowsWindowNames(t *testing.T) {
	model, _ := newTestPicker()
	view := model.View()

	if !strings.Contains(view, "editor") {
		t.Error("expected window name 'editor' in view")
	}
}

func TestPickerShowsIcons(t *testing.T) {
	model, _ := newTestPicker()
	// Move cursor off "+ new" so sessions are rendered with state icons.
	model.cursor = 1
	view := model.View()

	if !strings.Contains(view, "●") {
		t.Error("expected attached icon ● in view")
	}
	if !strings.Contains(view, "○") {
		t.Error("expected detached icon ○ in view")
	}
}

func TestPickerShowsDivider(t *testing.T) {
	model, _ := newTestPicker()
	view := model.View()

	if !strings.Contains(view, "──") {
		t.Error("expected divider line between named and tmp sessions")
	}
}

// ── Navigation ──

func TestPickerCursorStartsAtNew(t *testing.T) {
	model, _ := newTestPicker()
	if model.cursor != 0 {
		t.Errorf("expected cursor at 0 (+ new session), got %d", model.cursor)
	}
}

func TestPickerNavigateDown(t *testing.T) {
	model, _ := newTestPicker()
	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	m := result.(PickerModel)
	if m.cursor != 1 {
		t.Errorf("expected cursor 1, got %d", m.cursor)
	}
}

func TestPickerNavigateUp(t *testing.T) {
	model, _ := newTestPicker()
	model.cursor = 2
	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyUp})
	m := result.(PickerModel)
	if m.cursor != 1 {
		t.Errorf("expected cursor 1, got %d", m.cursor)
	}
}

// ── Enter behavior ──

func TestPickerEnterOnNewCreates(t *testing.T) {
	model, _ := newTestPicker()
	// Cursor starts at 0 (+ new session).
	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := result.(PickerModel)

	if m.Result.Action != "new" {
		t.Errorf("expected action 'new', got %q", m.Result.Action)
	}
}

func TestPickerEnterOnSessionAttaches(t *testing.T) {
	model, _ := newTestPicker()
	model.cursor = 1 // First session (alpha).

	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := result.(PickerModel)

	if m.Result.Action != "attach" {
		t.Errorf("expected action 'attach', got %q", m.Result.Action)
	}
	if m.Result.Session == "" {
		t.Error("expected session name in result")
	}
}

func TestPickerEnterEmptyCreatesTmp(t *testing.T) {
	model := newEmptyPicker()
	// Cursor at 0, no text.
	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := result.(PickerModel)

	if m.Result.Action != "new" {
		t.Errorf("expected action 'new', got %q", m.Result.Action)
	}
	if m.Result.Name != "" {
		t.Errorf("expected blank name for tmp, got %q", m.Result.Name)
	}
}

func TestPickerEnterWithTypedNameCreates(t *testing.T) {
	model, _ := newTestPicker()
	model.input.SetValue("myproject")
	model.applyFilter()
	// No match, cursor still at 0 (+ new).

	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := result.(PickerModel)

	if m.Result.Action != "new" {
		t.Errorf("expected action 'new', got %q", m.Result.Action)
	}
	if m.Result.Name != "myproject" {
		t.Errorf("expected name 'myproject', got %q", m.Result.Name)
	}
}

func TestPickerEnterWithMatchAttaches(t *testing.T) {
	model, _ := newTestPicker()
	model.input.SetValue("dev")
	model.applyFilter()
	model.cursor = 1 // The matched "dev" session.

	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := result.(PickerModel)

	if m.Result.Action != "attach" {
		t.Errorf("expected action 'attach', got %q", m.Result.Action)
	}
	if m.Result.Session != "dev" {
		t.Errorf("expected session 'dev', got %q", m.Result.Session)
	}
}

func TestPickerNewEntryUpdatesWithQuery(t *testing.T) {
	model, _ := newTestPicker()
	model.input.SetValue("myapp")
	view := model.View()

	if !strings.Contains(view, `+ create "myapp"`) {
		t.Error("expected '+ create \"myapp\"' when text is typed")
	}
}

// ── Quit behavior ──

func TestPickerCtrlCQuits(t *testing.T) {
	model, _ := newTestPicker()
	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m := result.(PickerModel)
	if !m.Quitting {
		t.Error("expected quitting after ctrl+c")
	}
}

func TestPickerEscClearsInput(t *testing.T) {
	model, _ := newTestPicker()
	model.input.SetValue("something")
	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m := result.(PickerModel)
	if m.Quitting {
		t.Error("should not quit when esc clears input")
	}
	if m.input.Value() != "" {
		t.Error("expected input cleared after esc")
	}
}

func TestPickerEscEmptyQuits(t *testing.T) {
	model, _ := newTestPicker()
	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m := result.(PickerModel)
	if !m.Quitting {
		t.Error("expected quitting when esc with empty input")
	}
}

// ── Delete ──

func TestPickerCtrlDOnSessionEntersDeleteMode(t *testing.T) {
	model, _ := newTestPicker()
	model.cursor = 1 // On a session, not "+ new".

	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	m := result.(PickerModel)
	if m.mode != modeConfirmDelete {
		t.Error("expected confirm delete mode")
	}
}

func TestPickerCtrlDOnNewDoesNothing(t *testing.T) {
	model, _ := newTestPicker()
	// cursor at 0 (+ new session).
	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	m := result.(PickerModel)
	if m.mode == modeConfirmDelete {
		t.Error("should not enter delete mode on + new entry")
	}
}

func TestPickerDeleteConfirmY(t *testing.T) {
	model, _ := newTestPicker()
	model.cursor = 1
	model.mode = modeConfirmDelete
	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m := result.(PickerModel)
	if m.mode != modeNormal {
		t.Error("expected back to normal mode after confirm")
	}
}

// ── Template ──

func TestPickerCtrlTOpensTemplates(t *testing.T) {
	model, _ := newTestPicker()
	model.templates = []session.Template{
		{Name: "dev", Description: "Dev environment"},
	}
	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	m := result.(PickerModel)
	if m.mode != modeTemplateSelect {
		t.Error("expected template select mode")
	}
}

// ── Empty state ──

func TestPickerEmptySessions(t *testing.T) {
	model := newEmptyPicker()
	view := model.View()

	if !strings.Contains(view, "░█████████") {
		t.Error("expected big logo when no sessions")
	}
	// Should still have "+ new session" entry.
	if !strings.Contains(view, "+ new session") {
		t.Error("expected '+ new session' entry even with no sessions")
	}
}

// ── Ghost command ──

func TestPickerGhostCmdOnNew(t *testing.T) {
	model, _ := newTestPicker()
	// Cursor at 0.
	view := model.View()
	if !strings.Contains(view, "zmux new") {
		t.Error("expected 'zmux new' in ghost prompt when on + new")
	}
}

func TestPickerGhostCmdOnSession(t *testing.T) {
	model, _ := newTestPicker()
	model.cursor = 1
	view := model.View()
	// Should show "zmux <sessionname>".
	if !strings.Contains(view, "zmux alpha") && !strings.Contains(view, "zmux dev") {
		t.Error("expected 'zmux <name>' in ghost prompt when on a session")
	}
}

// ── Fuzzy filter ──

func TestPickerFuzzyFilter(t *testing.T) {
	model, _ := newTestPicker()
	model.input.SetValue("dev")
	model.applyFilter()
	if len(model.filtered) != 1 {
		t.Errorf("expected 1 filtered result for 'dev', got %d", len(model.filtered))
	}
}

func TestPickerFuzzyPartial(t *testing.T) {
	model, _ := newTestPicker()
	model.input.SetValue("al")
	model.applyFilter()
	if len(model.filtered) != 1 {
		t.Errorf("expected 1 filtered result for 'al', got %d", len(model.filtered))
	}
}
