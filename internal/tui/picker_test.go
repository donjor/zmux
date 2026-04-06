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
	model.buildItems()

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
	model.buildItems()
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

func TestPickerCtrlXOnSessionEntersDeleteMode(t *testing.T) {
	model, _ := newTestPicker()
	model.cursor = 1 // On a session, not "+ new".

	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	m := result.(PickerModel)
	if m.mode != modeConfirmDelete {
		t.Error("expected confirm delete mode")
	}
}

func TestPickerCtrlXOnNewDoesNothing(t *testing.T) {
	model, _ := newTestPicker()
	// cursor at 0 (+ new session).
	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
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

func TestPickerFilterMovesToMatch(t *testing.T) {
	model, _ := newTestPicker()
	model.input.SetValue("dev")
	model.applyFilter()
	// Cursor should move off "+ create" to the matching session.
	if model.cursor != 1 {
		t.Errorf("expected cursor 1 (matching session), got %d", model.cursor)
	}
}

func TestPickerFilterClearedResetsToCreate(t *testing.T) {
	model, _ := newTestPicker()
	model.input.SetValue("dev")
	model.applyFilter()
	if model.cursor != 1 {
		t.Fatalf("setup: expected cursor 1, got %d", model.cursor)
	}
	// Clear search — cursor should reset to 0 (+ new session).
	model.input.SetValue("")
	model.applyFilter()
	if model.cursor != 0 {
		t.Errorf("expected cursor 0 after clearing search, got %d", model.cursor)
	}
}

func TestPickerArrowNavNoResetOnEmptyQuery(t *testing.T) {
	model, _ := newTestPicker()
	// Navigate down with no search text.
	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	m := result.(PickerModel)
	if m.cursor != 1 {
		t.Fatalf("setup: expected cursor 1, got %d", m.cursor)
	}
	// applyFilter runs on text input updates but cursor should stay put
	// because query hasn't changed (still empty).
	m.applyFilter()
	if m.cursor != 1 {
		t.Errorf("expected cursor 1 (unchanged), got %d", m.cursor)
	}
}

func TestPickerFilterNoMatchStaysOnCreate(t *testing.T) {
	model, _ := newTestPicker()
	model.input.SetValue("nonexistent")
	model.applyFilter()
	// No matches — cursor stays at 0 (create).
	if model.cursor != 0 {
		t.Errorf("expected cursor 0 (create), got %d", model.cursor)
	}
}

func TestPickerFilterMatchLandsOnWorkspaceHeader(t *testing.T) {
	model := newTestPickerWithWorkspaces()
	model.input.SetValue("dev")
	model.applyFilter()
	// With workspaces, items[0] is workspace header "bridge" (selectable), items[1] is "dev".
	// Cursor should land on workspace header at 1 (it's selectable now).
	if model.cursor != 1 {
		t.Errorf("expected cursor 1 (workspace header, selectable), got %d", model.cursor)
	}
}

// ── Workspace grouping ──

func newTestPickerWithWorkspaces() PickerModel {
	m := tmux.NewMockRunner()
	now := time.Now()
	m.Sessions = []tmux.Session{
		{Name: "dev", Windows: 3, Attached: true, Activity: now, Created: now.Add(-2 * time.Hour), Dir: "/home/user/bridge"},
		{Name: "monitor", Windows: 1, Attached: false, Activity: now, Created: now.Add(-1 * time.Hour), Dir: "/home/user/bridge"},
		{Name: "zmux", Windows: 2, Attached: false, Activity: now, Created: now.Add(-3 * time.Hour), Dir: "/home/user/zmux"},
		{Name: "tmp-1", Windows: 1, Attached: false, Activity: now, Created: now.Add(-5 * time.Minute), Dir: "/tmp"},
	}
	m.Windows["dev"] = []tmux.Window{{Index: 1, Name: "editor", Active: true}}
	m.Windows["monitor"] = []tmux.Window{{Index: 1, Name: "htop", Active: true}}
	m.Windows["zmux"] = []tmux.Window{{Index: 1, Name: "nvim", Active: true}}
	m.Windows["tmp-1"] = []tmux.Window{{Index: 1, Name: "zsh", Active: true}}

	styles := DefaultStyles()
	model := NewPickerModel(m, styles)

	sessions, _ := session.ListSessions(m)
	model.sessions = sessions
	model.filtered = sessions

	// Set workspace state.
	model.workspaceState = map[string]string{
		"dev":     "bridge",
		"monitor": "bridge",
	}
	model.buildItems()

	wins := make(map[string][]tmux.Window)
	for _, s := range sessions {
		w, _ := m.ListWindows(s.Name)
		wins[s.Name] = w
	}
	model.windows = wins

	return model
}

func TestPickerWorkspaceGrouping(t *testing.T) {
	model := newTestPickerWithWorkspaces()

	// Should have items: header(bridge), dev, monitor, header(sessions), zmux, header(temporary), tmp-1
	if len(model.items) != 7 {
		t.Fatalf("expected 7 items, got %d", len(model.items))
	}

	// First item: workspace header "bridge"
	if !model.items[0].IsHeader || model.items[0].Header != "bridge" {
		t.Errorf("items[0] should be header 'bridge', got IsHeader=%v Header=%q", model.items[0].IsHeader, model.items[0].Header)
	}

	// Sessions under bridge
	if model.items[1].Session == nil || model.items[1].Session.Name != "dev" {
		t.Errorf("items[1] should be session 'dev'")
	}
	if model.items[2].Session == nil || model.items[2].Session.Name != "monitor" {
		t.Errorf("items[2] should be session 'monitor'")
	}

	// Untagged section
	if !model.items[3].IsHeader || model.items[3].Header != "other" {
		t.Errorf("items[3] should be header 'sessions'")
	}
	if model.items[4].Session == nil || model.items[4].Session.Name != "zmux" {
		t.Errorf("items[4] should be session 'zmux'")
	}

	// Temporary section
	if !model.items[5].IsHeader || model.items[5].Header != "temporary" {
		t.Errorf("items[5] should be header 'temporary'")
	}
	if model.items[6].Session == nil || model.items[6].Session.Name != "tmp-1" {
		t.Errorf("items[6] should be session 'tmp-1'")
	}
}

func TestPickerWorkspaceHeadersSelectable(t *testing.T) {
	model := newTestPickerWithWorkspaces()

	// Start at cursor 0 (+ new session), go down.
	// Cursor 1 = item[0] = workspace header "bridge" — should be selectable (not skipped).
	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	m := result.(PickerModel)
	// Workspace headers are now selectable, so cursor lands on 1.
	if m.cursor != 1 {
		t.Errorf("expected cursor 1 (workspace header), got %d", m.cursor)
	}
}

func TestPickerWorkspaceGroupedSessionInheritsWorkspace(t *testing.T) {
	m := tmux.NewMockRunner()
	now := time.Now()
	m.Sessions = []tmux.Session{
		{Name: "dev", Windows: 2, Attached: true, Activity: now, Created: now.Add(-2 * time.Hour), Dir: "/home/user/bridge"},
		{Name: "dev-b", Windows: 2, Attached: false, Activity: now, Created: now.Add(-1 * time.Hour), Dir: "/home/user/bridge"},
	}

	styles := DefaultStyles()
	model := NewPickerModel(m, styles)

	sessions, _ := session.ListSessions(m)
	model.sessions = sessions
	model.filtered = sessions

	// Only root "dev" is tagged; dev-b should inherit via RootName.
	model.workspaceState = map[string]string{
		"dev": "bridge",
	}
	model.buildItems()

	// Expect: header(bridge), dev, dev-b — dev-b inherits root's workspace.
	if len(model.items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(model.items))
	}
	if !model.items[0].IsHeader || model.items[0].Header != "bridge" {
		t.Error("items[0] should be header 'bridge'")
	}
	if model.items[1].Session == nil || model.items[1].Session.Name != "dev" {
		t.Error("items[1] should be session 'dev'")
	}
	if model.items[2].Session == nil || model.items[2].Session.Name != "dev-b" {
		t.Error("items[2] should be session 'dev-b'")
	}
}

func TestPickerWorkspaceViewRendersHeaders(t *testing.T) {
	model := newTestPickerWithWorkspaces()
	view := model.View()

	if !strings.Contains(view, "bridge") {
		t.Error("expected workspace header 'bridge' in view")
	}
	if !strings.Contains(view, "other") {
		t.Error("expected 'other' header in view")
	}
	if !strings.Contains(view, "temporary") {
		t.Error("expected 'temporary' header in view")
	}
}
