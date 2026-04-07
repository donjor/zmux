package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/donjor/zmux/internal/tmux"
)

// newTestTabPicker builds a tab picker backed by a mock runner with
// three windows in session "dev". It loads the tabs synchronously so
// tests don't need to drain Init() commands.
func newTestTabPicker(t *testing.T) TabPickerModel {
	t.Helper()
	mock := tmux.NewMockRunner()
	mock.Windows = map[string][]tmux.Window{
		"dev": {
			{Index: 1, Name: "editor", Active: true},
			{Index: 2, Name: "server", Active: false},
			{Index: 3, Name: "git", Active: false},
		},
	}
	// Panes aren't strictly required but listLoadTabs reads them for cmd.
	mock.Panes = map[string][]tmux.Pane{
		"dev": {
			{WindowIndex: 1, Active: true, Command: "nvim"},
			{WindowIndex: 2, Active: true, Command: "go run"},
			{WindowIndex: 3, Active: true, Command: "lazygit"},
		},
	}

	m := NewTabPickerModel(mock, "dev", DefaultStyles())
	m.width = 80
	m.height = 24

	// Drive the load command directly rather than through Init() batch.
	msg := m.loadTabs()()
	out, _ := m.Update(msg)
	return out.(TabPickerModel)
}

func sendTabKey(m TabPickerModel, k string) TabPickerModel {
	var msg tea.KeyMsg
	switch k {
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
	case "ctrl+n":
		msg = tea.KeyMsg{Type: tea.KeyCtrlN}
	case "ctrl+r":
		msg = tea.KeyMsg{Type: tea.KeyCtrlR}
	case "ctrl+x":
		msg = tea.KeyMsg{Type: tea.KeyCtrlX}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
	}
	out, _ := m.Update(msg)
	return out.(TabPickerModel)
}

func TestTabPickerLoadsTabs(t *testing.T) {
	m := newTestTabPicker(t)
	if len(m.tabs) != 3 {
		t.Fatalf("tabs = %d, want 3", len(m.tabs))
	}
	if len(m.filtered) != 3 {
		t.Errorf("filtered = %d, want 3", len(m.filtered))
	}
	if m.tree.Cursor != 0 {
		t.Errorf("cursor = %d, want 0", m.tree.Cursor)
	}
}

func TestTabPickerNavigation(t *testing.T) {
	m := newTestTabPicker(t)

	m = sendTabKey(m, "down")
	if m.tree.Cursor != 1 {
		t.Errorf("after down: cursor = %d, want 1", m.tree.Cursor)
	}

	m = sendTabKey(m, "down")
	if m.tree.Cursor != 2 {
		t.Errorf("after 2x down: cursor = %d, want 2", m.tree.Cursor)
	}

	// Clamp at bottom.
	m = sendTabKey(m, "down")
	if m.tree.Cursor != 2 {
		t.Errorf("bottom clamp: cursor = %d, want 2", m.tree.Cursor)
	}

	m = sendTabKey(m, "up")
	if m.tree.Cursor != 1 {
		t.Errorf("after up: cursor = %d, want 1", m.tree.Cursor)
	}
}

func TestTabPickerEnterSelectsByIndex(t *testing.T) {
	m := newTestTabPicker(t)
	m = sendTabKey(m, "down") // cursor → 1 (window index 2)
	m = sendTabKey(m, "enter")

	if !m.Quitting {
		t.Fatal("expected Quitting after enter")
	}
	if m.Result.Action != "select" {
		t.Errorf("action = %q, want select", m.Result.Action)
	}
	if m.Result.Index != 2 {
		t.Errorf("index = %d, want 2", m.Result.Index)
	}
}

func TestTabPickerCtrlXClose(t *testing.T) {
	m := newTestTabPicker(t)
	m = sendTabKey(m, "ctrl+x")

	if m.Result.Action != "close" {
		t.Errorf("action = %q, want close", m.Result.Action)
	}
	if m.Result.Index != 1 {
		t.Errorf("index = %d, want 1", m.Result.Index)
	}
}

func TestTabPickerCtrlRRenameModeSeedsName(t *testing.T) {
	m := newTestTabPicker(t)
	m = sendTabKey(m, "ctrl+r")

	if m.mode != tpModeRename {
		t.Errorf("mode = %v, want rename", m.mode)
	}
	if m.input.Value() != "editor" {
		t.Errorf("input seeded with %q, want editor", m.input.Value())
	}
	if m.renameIdx != 1 {
		t.Errorf("renameIdx = %d, want 1", m.renameIdx)
	}
}

func TestTabPickerCtrlNNewTabMode(t *testing.T) {
	m := newTestTabPicker(t)
	m = sendTabKey(m, "ctrl+n")
	if m.mode != tpModeNew {
		t.Errorf("mode = %v, want new", m.mode)
	}
}

func TestTabPickerEscOnEmptyQueryQuits(t *testing.T) {
	m := newTestTabPicker(t)
	m = sendTabKey(m, "esc")
	if !m.Quitting {
		t.Error("esc on empty query should quit")
	}
}

func TestTabPickerEscClearsQueryFirst(t *testing.T) {
	m := newTestTabPicker(t)
	// Type a query.
	m = sendTabKey(m, "e")
	if m.input.Value() != "e" {
		t.Fatalf("query = %q, want e", m.input.Value())
	}
	// esc clears query, does not quit.
	m = sendTabKey(m, "esc")
	if m.Quitting {
		t.Error("esc with query should clear, not quit")
	}
	if m.input.Value() != "" {
		t.Errorf("query after esc = %q, want empty", m.input.Value())
	}
}

func TestTabPickerFilterNarrowsAndClampsCursor(t *testing.T) {
	m := newTestTabPicker(t)
	m.tree.Cursor = 2 // land on "git"

	// Narrow to "editor" — one match.
	m.input.SetValue("editor")
	m.applyFilter()

	if len(m.filtered) != 1 {
		t.Errorf("filtered = %d, want 1", len(m.filtered))
	}
	if m.tree.Cursor != 0 {
		t.Errorf("cursor after narrow = %d, want 0 (clamped)", m.tree.Cursor)
	}
}

func TestTabPickerCursorStableAcrossFilterClear(t *testing.T) {
	// Stable-ID behaviour: land on "server", filter + clear, cursor
	// should land back on "server", not snap to row 0.
	m := newTestTabPicker(t)
	m.tree.Cursor = 1 // "server"

	// Narrow to server.
	m.input.SetValue("server")
	m.applyFilter()
	cur := m.currentTab()
	if cur == nil || cur.Name != "server" {
		t.Fatalf("after narrow, current = %v, want server", cur)
	}

	// Clear filter.
	m.input.SetValue("")
	m.applyFilter()
	cur = m.currentTab()
	if cur == nil || cur.Name != "server" {
		t.Errorf("after clear, current = %v, want server (stable ID restore)", cur)
	}
}

func TestTabPickerViewRenders(t *testing.T) {
	m := newTestTabPicker(t)
	view := m.View()
	if !strings.Contains(view, "dev") {
		t.Error("view missing session name")
	}
	if !strings.Contains(view, "editor") {
		t.Error("view missing tab name")
	}
}

func TestTabPickerViewEmptyWhenQuitting(t *testing.T) {
	m := newTestTabPicker(t)
	m.Quitting = true
	if v := m.View(); v != "" {
		t.Errorf("quitting view non-empty: %q", v)
	}
}
