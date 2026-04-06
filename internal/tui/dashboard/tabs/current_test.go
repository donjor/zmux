package tabs

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui"
	"github.com/donjor/zmux/internal/tui/dashboard"
)

func newTestCurrentTab() (*CurrentTab, *tmux.MockRunner) {
	mock := tmux.NewMockRunner()
	mock.InsideTmux = true
	mock.DisplayMessageResult = "dev"
	now := time.Now()

	mock.Sessions = []tmux.Session{
		{Name: "dev", Windows: 3, Attached: true, Activity: now, Created: now.Add(-2 * time.Hour), Dir: "/home/user/work"},
		{Name: "api", Windows: 1, Attached: false, Activity: now, Created: now.Add(-1 * time.Hour), Dir: "/home/user/api"},
	}
	mock.Windows = map[string][]tmux.Window{
		"dev": {
			{Index: 1, Name: "editor", Active: true, Dir: "/home/user/work"},
			{Index: 2, Name: "server", Active: false, Dir: "/home/user/work"},
			{Index: 3, Name: "git", Active: false, Dir: "/home/user/work"},
		},
	}
	mock.Panes = map[string][]tmux.Pane{
		"dev": {
			{Index: 0, Active: true, Command: "nvim", PID: 1234, Dir: "/home/user/work", Width: 80, Height: 24},
		},
		"dev:1": {
			{Index: 0, Active: true, Command: "nvim", PID: 1234, Dir: "/home/user/work", Width: 80, Height: 24},
		},
		"dev:2": {
			{Index: 0, Active: true, Command: "node", PID: 1235, Dir: "/home/user/work", Width: 80, Height: 24},
		},
		"dev:3": {
			{Index: 0, Active: true, Command: "bash", PID: 1236, Dir: "/home/user/work", Width: 80, Height: 24},
		},
	}

	styles := tui.DefaultStyles()
	tab := NewCurrentTab(mock, styles)
	tab.Resize(80, 40)
	return tab, mock
}

func simulateCurrentActivate(tab *CurrentTab) *CurrentTab {
	cmd := tab.Activate(dashboard.ActivateInit)
	if cmd != nil {
		msg := cmd()
		if msg != nil {
			result, _ := tab.Update(msg)
			tab = result.(*CurrentTab)
		}
	}
	return tab
}

func sendCurrentKey(tab *CurrentTab, keyStr string) (*CurrentTab, tea.Cmd) {
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
	}

	result, cmd := tab.Update(msg)
	return result.(*CurrentTab), cmd
}

func TestCurrentTabID(t *testing.T) {
	tab, _ := newTestCurrentTab()
	if tab.ID() != dashboard.TabCurrent {
		t.Errorf("expected TabCurrent, got %s", tab.ID())
	}
}

func TestCurrentTabTitle(t *testing.T) {
	tab, _ := newTestCurrentTab()
	if tab.Title() != "This Session" {
		t.Errorf("expected 'This Session', got %q", tab.Title())
	}
}

func TestCurrentTabActivateLoadsWindows(t *testing.T) {
	tab, _ := newTestCurrentTab()
	tab = simulateCurrentActivate(tab)

	if tab.sessionName != "dev" {
		t.Errorf("expected session name 'dev', got %q", tab.sessionName)
	}
	if len(tab.windows) != 3 {
		t.Errorf("expected 3 windows, got %d", len(tab.windows))
	}
}

func TestCurrentTabNavigate(t *testing.T) {
	tab, _ := newTestCurrentTab()
	tab = simulateCurrentActivate(tab)

	if tab.cursor != 0 {
		t.Fatalf("expected cursor at 0, got %d", tab.cursor)
	}

	tab, _ = sendCurrentKey(tab, "j")
	if tab.cursor != 1 {
		t.Errorf("expected cursor at 1, got %d", tab.cursor)
	}

	tab, _ = sendCurrentKey(tab, "k")
	if tab.cursor != 0 {
		t.Errorf("expected cursor at 0, got %d", tab.cursor)
	}

	// Should not go below 0.
	tab, _ = sendCurrentKey(tab, "k")
	if tab.cursor != 0 {
		t.Errorf("expected cursor clamped at 0, got %d", tab.cursor)
	}
}

func TestCurrentTabNavigateDownBound(t *testing.T) {
	tab, _ := newTestCurrentTab()
	tab = simulateCurrentActivate(tab)

	// Move to end.
	for i := 0; i < 10; i++ {
		tab, _ = sendCurrentKey(tab, "j")
	}
	if tab.cursor != len(tab.windows)-1 {
		t.Errorf("expected cursor at end %d, got %d", len(tab.windows)-1, tab.cursor)
	}
}

func TestCurrentTabEnterFocusesWindow(t *testing.T) {
	tab, _ := newTestCurrentTab()
	tab = simulateCurrentActivate(tab)

	_, cmd := sendCurrentKey(tab, "enter")
	if cmd == nil {
		t.Fatal("expected command from enter")
	}

	msg := cmd()
	intent, ok := msg.(dashboard.QuitIntent)
	if !ok {
		t.Fatalf("expected QuitIntent, got %T", msg)
	}
	if intent.Action != "focus" {
		t.Errorf("expected action 'focus', got %q", intent.Action)
	}
}

func TestCurrentTabRenameEntersMode(t *testing.T) {
	tab, _ := newTestCurrentTab()
	tab = simulateCurrentActivate(tab)

	tab, _ = sendCurrentKey(tab, "r")
	if tab.mode != currentModeRename {
		t.Errorf("expected currentModeRename, got %d", tab.mode)
	}
}

func TestCurrentTabRenameCancel(t *testing.T) {
	tab, _ := newTestCurrentTab()
	tab = simulateCurrentActivate(tab)

	tab, _ = sendCurrentKey(tab, "r")
	tab, _ = sendCurrentKey(tab, "esc")

	if tab.mode != currentModeList {
		t.Errorf("expected currentModeList after esc, got %d", tab.mode)
	}
}

func TestCurrentTabRenameConfirm(t *testing.T) {
	tab, mock := newTestCurrentTab()
	tab = simulateCurrentActivate(tab)

	tab, _ = sendCurrentKey(tab, "r")
	tab.renameInput.SetValue("newname")

	tab, cmd := sendCurrentKey(tab, "enter")

	if tab.mode != currentModeList {
		t.Errorf("expected currentModeList after rename confirm, got %d", tab.mode)
	}

	if cmd != nil {
		msg := cmd()
		if msg != nil {
			tab.Update(msg)
		}
	}

	found := false
	for _, c := range mock.Calls {
		if c.Method == "RenameWindow" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected RenameWindow to be called")
	}
}

func TestCurrentTabCloseConfirmDialog(t *testing.T) {
	tab, _ := newTestCurrentTab()
	tab = simulateCurrentActivate(tab)

	tab, _ = sendCurrentKey(tab, "x")
	if tab.mode != currentModeConfirmClose {
		t.Errorf("expected currentModeConfirmClose, got %d", tab.mode)
	}
	if tab.closeTarget == "" {
		t.Error("expected closeTarget to be set")
	}
}

func TestCurrentTabCloseCancel(t *testing.T) {
	tab, _ := newTestCurrentTab()
	tab = simulateCurrentActivate(tab)

	tab, _ = sendCurrentKey(tab, "x")
	tab, _ = sendCurrentKey(tab, "n")

	if tab.mode != currentModeList {
		t.Errorf("expected currentModeList after cancel, got %d", tab.mode)
	}
}

func TestCurrentTabCloseConfirm(t *testing.T) {
	tab, mock := newTestCurrentTab()
	tab = simulateCurrentActivate(tab)

	tab, _ = sendCurrentKey(tab, "x")
	_, cmd := sendCurrentKey(tab, "y")

	if cmd != nil {
		msg := cmd()
		if msg != nil {
			// Don't need to update, just check mock calls.
		}
	}

	found := false
	for _, c := range mock.Calls {
		if c.Method == "KillWindow" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected KillWindow to be called")
	}
}

func TestCurrentTabNewWindow(t *testing.T) {
	tab, mock := newTestCurrentTab()
	tab = simulateCurrentActivate(tab)

	_, cmd := sendCurrentKey(tab, "n")
	if cmd == nil {
		t.Fatal("expected command from n")
	}

	msg := cmd()
	if msg != nil {
		tab.Update(msg)
	}

	found := false
	for _, c := range mock.Calls {
		if c.Method == "NewWindow" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected NewWindow to be called")
	}
}

func TestCurrentTabSwapWindow(t *testing.T) {
	tab, mock := newTestCurrentTab()
	tab = simulateCurrentActivate(tab)

	// Move cursor to window at index 1 (second window).
	tab, _ = sendCurrentKey(tab, "j")

	// Swap right.
	_, cmd := sendCurrentKey(tab, ">")
	if cmd == nil {
		t.Fatal("expected command from >")
	}

	msg := cmd()
	if msg != nil {
		tab.Update(msg)
	}

	found := false
	for _, c := range mock.Calls {
		if c.Method == "SwapWindow" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected SwapWindow to be called")
	}
}

func TestCurrentTabMoveWindow(t *testing.T) {
	tab, _ := newTestCurrentTab()
	tab = simulateCurrentActivate(tab)

	_, cmd := sendCurrentKey(tab, "m")
	if cmd == nil {
		t.Fatal("expected command from m")
	}

	msg := cmd()
	if msg == nil {
		t.Fatal("expected move destinations message")
	}

	result, _ := tab.Update(msg)
	tab = result.(*CurrentTab)

	if tab.mode != currentModeMoveWindow {
		t.Errorf("expected currentModeMoveWindow, got %d", tab.mode)
	}
}

func TestCurrentTabMoveCancelOnEsc(t *testing.T) {
	tab, _ := newTestCurrentTab()
	tab = simulateCurrentActivate(tab)
	tab.mode = currentModeMoveWindow
	tab.moveTargets = []moveTarget{{Name: "other", Windows: 1}}

	tab, _ = sendCurrentKey(tab, "esc")
	if tab.mode != currentModeList {
		t.Errorf("expected currentModeList after esc, got %d", tab.mode)
	}
}

func TestCurrentTabViewRendersWindowList(t *testing.T) {
	tab, _ := newTestCurrentTab()
	tab = simulateCurrentActivate(tab)

	view := tab.View()

	if !strings.Contains(view, "editor") {
		t.Error("expected view to contain window 'editor'")
	}
	if !strings.Contains(view, "server") {
		t.Error("expected view to contain window 'server'")
	}
	if !strings.Contains(view, "git") {
		t.Error("expected view to contain window 'git'")
	}
}

func TestCurrentTabViewShowsSessionName(t *testing.T) {
	tab, _ := newTestCurrentTab()
	tab = simulateCurrentActivate(tab)

	view := tab.View()

	if !strings.Contains(view, "dev") {
		t.Error("expected view to contain session name 'dev'")
	}
}

func TestCurrentTabViewShowsWindowCount(t *testing.T) {
	tab, _ := newTestCurrentTab()
	tab = simulateCurrentActivate(tab)

	view := tab.View()

	if !strings.Contains(view, "3 tabs") {
		t.Error("expected view to contain '3 tabs'")
	}
}

func TestCurrentTabViewEmptySession(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.DisplayMessageResult = ""
	styles := tui.DefaultStyles()
	tab := NewCurrentTab(mock, styles)
	tab.Resize(80, 40)

	tab = simulateCurrentActivate(tab)

	view := tab.View()
	if !strings.Contains(view, "No active session") {
		t.Error("expected empty state message")
	}
}

func TestCurrentTabDeactivateDropsOverlays(t *testing.T) {
	tab, _ := newTestCurrentTab()
	tab = simulateCurrentActivate(tab)

	tab.mode = currentModeConfirmClose
	tab.closeTarget = "editor"

	tab.Deactivate()

	if tab.mode != currentModeList {
		t.Errorf("expected mode reset to list, got %d", tab.mode)
	}
}

func TestCurrentTabShortHelp(t *testing.T) {
	tab, _ := newTestCurrentTab()

	help := tab.ShortHelp()
	if !strings.Contains(help, "enter:focus") {
		t.Error("expected help to contain 'enter:focus'")
	}
	if !strings.Contains(help, "n:new") {
		t.Error("expected help to contain 'n:new'")
	}
	if !strings.Contains(help, "</>:reorder") {
		t.Error("expected help to contain '</>:reorder'")
	}
}

func TestCurrentTabJumpToTopBottom(t *testing.T) {
	tab, _ := newTestCurrentTab()
	tab = simulateCurrentActivate(tab)

	tab, _ = sendCurrentKey(tab, "G")
	if tab.cursor != len(tab.windows)-1 {
		t.Errorf("expected cursor at end, got %d", tab.cursor)
	}

	tab, _ = sendCurrentKey(tab, "g")
	if tab.cursor != 0 {
		t.Errorf("expected cursor at 0, got %d", tab.cursor)
	}
}

func TestIsIdleWindow(t *testing.T) {
	// Shell with no CPU.
	w := windowDetail{
		Panes: []tmux.Pane{
			{Active: true, Command: "bash"},
		},
		Stats: tmux.ProcessStats{CPU: 0.0},
	}
	if !isIdleWindow(w) {
		t.Error("expected bash with 0 CPU to be idle")
	}

	// Shell with CPU.
	w.Stats.CPU = 2.0
	if isIdleWindow(w) {
		t.Error("expected bash with CPU to not be idle")
	}

	// Non-shell.
	w2 := windowDetail{
		Panes: []tmux.Pane{
			{Active: true, Command: "nvim"},
		},
		Stats: tmux.ProcessStats{CPU: 0.0},
	}
	if isIdleWindow(w2) {
		t.Error("expected nvim to not be idle")
	}
}
