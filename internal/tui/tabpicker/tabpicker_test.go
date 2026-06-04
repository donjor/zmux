package tabpicker

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui/outline"
	"github.com/donjor/zmux/internal/tui/styles"
	"github.com/donjor/zmux/internal/tui/tkey"
)

// newTestPicker builds a switcher over two workspace sessions ("dev" with
// three tabs, "test" with two), current = "dev". Sessions load
// synchronously so tests don't drain Init().
func newTestPicker(t *testing.T) TabPickerModel {
	t.Helper()
	mock := tmux.NewMockRunner()
	mock.Windows = map[string][]tmux.Window{
		"dev": {
			{Index: 1, Name: "editor", Active: true},
			{Index: 2, Name: "server", Active: false},
			{Index: 3, Name: "git", Active: false},
		},
		"test": {
			{Index: 1, Name: "watch", Active: true},
			{Index: 2, Name: "shell", Active: false},
		},
	}
	mock.Panes = map[string][]tmux.Pane{
		"dev": {
			{WindowIndex: 1, Active: true, Command: "nvim"},
			{WindowIndex: 2, Active: true, Command: "go run"},
			{WindowIndex: 3, Active: true, Command: "lazygit"},
		},
		"test": {
			{WindowIndex: 1, Active: true, Command: "vitest"},
			{WindowIndex: 2, Active: true, Command: "bash"},
		},
	}

	infos := []session.SessionInfo{
		{Name: "dev", Attached: true},
		{Name: "test", Attached: false},
	}

	m := NewTabPickerModel(mock, "myapp", "dev", infos, styles.DefaultStyles())
	m.width = 80
	m.height = 24

	out, _ := m.Update(m.loadSessions()())
	return out.(TabPickerModel)
}

func send(m TabPickerModel, k string) TabPickerModel {
	var msg tea.KeyMsg
	switch k {
	case "enter":
		msg = tkey.Enter()
	case "esc":
		msg = tkey.Esc()
	case "up":
		msg = tkey.Up()
	case "down":
		msg = tkey.Down()
	case "left":
		msg = tkey.Left()
	case "right":
		msg = tkey.Right()
	case "ctrl+c":
		msg = tkey.Ctrl('c')
	case "ctrl+n":
		msg = tkey.Ctrl('n')
	case "ctrl+r":
		msg = tkey.Ctrl('r')
	case "ctrl+x":
		msg = tkey.Ctrl('x')
	default:
		msg = tkey.Type(k)
	}
	out, _ := m.Update(msg)
	return out.(TabPickerModel)
}

func TestLoadsSessionsCurrentFirst(t *testing.T) {
	m := newTestPicker(t)
	if len(m.entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(m.entries))
	}
	if m.entries[0].Info.Name != "dev" {
		t.Errorf("first entry = %q, want dev (current-first)", m.entries[0].Info.Name)
	}
	if len(m.entries[0].Windows) != 3 {
		t.Errorf("dev tabs = %d, want 3", len(m.entries[0].Windows))
	}
	if m.nav != navSession {
		t.Errorf("nav = %v, want session", m.nav)
	}
	if m.focused != "dev" {
		t.Errorf("focused = %q, want dev", m.focused)
	}
}

func TestSessionLevelEnterSwitches(t *testing.T) {
	m := newTestPicker(t)
	m = send(m, "down") // focus → test
	if m.focused != "test" {
		t.Fatalf("focused = %q, want test", m.focused)
	}
	m = send(m, "enter")
	if !m.Quitting {
		t.Fatal("expected Quitting after enter")
	}
	if m.Result.Action != "switch" || m.Result.Session != "test" {
		t.Errorf("result = %+v, want switch/test", m.Result)
	}
}

func TestFocusClampsAtEdges(t *testing.T) {
	m := newTestPicker(t)
	m = send(m, "up") // already at top
	if m.focused != "dev" {
		t.Errorf("top clamp: focused = %q, want dev", m.focused)
	}
	m = send(m, "down")
	m = send(m, "down") // past bottom
	if m.focused != "test" {
		t.Errorf("bottom clamp: focused = %q, want test", m.focused)
	}
}

func TestDrillIntoTabsAndSelect(t *testing.T) {
	m := newTestPicker(t)
	m = send(m, "right") // drill into dev's tabs
	if m.nav != navTab {
		t.Fatalf("nav = %v, want tab", m.nav)
	}
	if m.drilled != "dev" {
		t.Fatalf("drilled = %q, want dev", m.drilled)
	}
	// Cursor should sit on the first tab row.
	m = send(m, "down") // → tab index 2 (server)
	m = send(m, "enter")
	if m.Result.Action != "select" || m.Result.Session != "dev" || m.Result.Index != 2 {
		t.Errorf("result = %+v, want select/dev/2", m.Result)
	}
}

func TestDrillThenBackToSessions(t *testing.T) {
	m := newTestPicker(t)
	m = send(m, "down")  // focus test
	m = send(m, "right") // drill test
	if m.drilled != "test" {
		t.Fatalf("drilled = %q, want test", m.drilled)
	}
	m = send(m, "left") // back
	if m.nav != navSession {
		t.Errorf("nav = %v, want session", m.nav)
	}
	if m.focused != "test" {
		t.Errorf("focus after back = %q, want test (re-pinned)", m.focused)
	}
}

func TestTabLevelCtrlXClose(t *testing.T) {
	m := newTestPicker(t)
	m = send(m, "right") // drill dev, cursor on tab 1
	m = send(m, "ctrl+x")
	if m.Result.Action != "close" || m.Result.Session != "dev" || m.Result.Index != 1 {
		t.Errorf("result = %+v, want close/dev/1", m.Result)
	}
}

func TestTabLevelCtrlRSeedsName(t *testing.T) {
	m := newTestPicker(t)
	m = send(m, "right") // drill dev
	m = send(m, "ctrl+r")
	if m.mode != tpModeRename {
		t.Fatalf("mode = %v, want rename", m.mode)
	}
	if m.input.Value() != "editor" {
		t.Errorf("seed = %q, want editor", m.input.Value())
	}
	if m.renameIdx != 1 {
		t.Errorf("renameIdx = %d, want 1", m.renameIdx)
	}
}

func TestTabLevelCtrlNNewMode(t *testing.T) {
	m := newTestPicker(t)
	m = send(m, "right")
	m = send(m, "ctrl+n")
	if m.mode != tpModeNew {
		t.Errorf("mode = %v, want new", m.mode)
	}
	m = send(m, "enter") // blank name
	if m.Result.Action != "new" || m.Result.Session != "dev" {
		t.Errorf("result = %+v, want new/dev", m.Result)
	}
}

func TestTabLevelSwapEmits(t *testing.T) {
	m := newTestPicker(t)
	m = send(m, "right") // drill dev, cursor tab 1
	m = send(m, ">")
	if m.Result.Action != "swap" || m.Result.Delta != 1 || m.Result.Index != 1 {
		t.Errorf("result = %+v, want swap/+1/1", m.Result)
	}
}

func TestSessionFilterNarrows(t *testing.T) {
	m := newTestPicker(t)
	m = send(m, "t") // matches "test"
	if len(m.filtered) != 1 {
		t.Fatalf("filtered = %d, want 1", len(m.filtered))
	}
	if m.filtered[0].Info.Name != "test" {
		t.Errorf("match = %q, want test", m.filtered[0].Info.Name)
	}
	if m.focused != "test" {
		t.Errorf("focus = %q, want test (followed filter)", m.focused)
	}
}

func TestEscClearsSessionFilterFirst(t *testing.T) {
	m := newTestPicker(t)
	m = send(m, "t")
	m = send(m, "esc") // clears query, does not quit
	if m.Quitting {
		t.Error("esc with query should clear, not quit")
	}
	if m.input.Value() != "" {
		t.Errorf("query after esc = %q, want empty", m.input.Value())
	}
}

func TestEscOnEmptyQuits(t *testing.T) {
	m := newTestPicker(t)
	m = send(m, "esc")
	if !m.Quitting {
		t.Error("esc on empty query should quit")
	}
}

func TestTabFilterNarrows(t *testing.T) {
	m := newTestPicker(t)
	m = send(m, "right") // drill dev
	m = send(m, "s")     // matches "server"
	// Only the matching window row should be selectable.
	var tabRows int
	for i := range m.tree.Rows {
		if m.tree.Rows[i].Kind == outline.RowWindow {
			tabRows++
		}
	}
	if tabRows != 1 {
		t.Errorf("visible tab rows = %d, want 1", tabRows)
	}
}

func TestViewRendersSessionsAndTabs(t *testing.T) {
	m := newTestPicker(t)
	v := m.view()
	if !strings.Contains(v, "myapp") {
		t.Error("view missing workspace header")
	}
	if !strings.Contains(v, "dev") || !strings.Contains(v, "test") {
		t.Error("view missing a session name")
	}
	// Focused session's tabs preview inline.
	if !strings.Contains(v, "editor") {
		t.Error("view missing focused session's tab")
	}
}

func TestViewEmptyWhenQuitting(t *testing.T) {
	m := newTestPicker(t)
	m.Quitting = true
	if v := m.view(); v != "" {
		t.Errorf("quitting view non-empty: %q", v)
	}
}
