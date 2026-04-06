package tabs

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui"
	"github.com/donjor/zmux/internal/tui/dashboard"
)

func newTestSessionsTab() (*SessionsTab, *tmux.MockRunner) {
	mock := tmux.NewMockRunner()
	mock.InsideTmux = true
	now := time.Now()
	mock.Sessions = []tmux.Session{
		{Name: "dev", Windows: 3, Attached: true, Activity: now, Created: now.Add(-2 * time.Hour), Dir: "/home/user/work"},
		{Name: "api", Windows: 2, Attached: false, Activity: now, Created: now.Add(-1 * time.Hour), Dir: "/home/user/api"},
		{Name: "tmp-1", Windows: 1, Attached: false, Activity: now, Created: now.Add(-5 * time.Minute), Dir: "/tmp"},
	}
	mock.Windows = map[string][]tmux.Window{
		"dev": {
			{Index: 1, Name: "editor", Active: true, Dir: "/home/user/work"},
			{Index: 2, Name: "server", Active: false, Dir: "/home/user/work"},
			{Index: 3, Name: "git", Active: false, Dir: "/home/user/work"},
		},
		"api": {
			{Index: 1, Name: "main", Active: true, Dir: "/home/user/api"},
		},
	}

	styles := tui.DefaultStyles()
	tab := NewSessionsTab(mock, styles)
	tab.Resize(80, 40)
	return tab, mock
}

func simulateActivate(tab *SessionsTab) *SessionsTab {
	cmd := tab.Activate(dashboard.ActivateInit)
	if cmd == nil {
		return tab
	}
	// tea.Batch returns a BatchMsg ([]tea.Cmd) when called.
	msg := cmd()
	if msg == nil {
		return tab
	}
	// Handle BatchMsg: execute each sub-command and feed results.
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, subCmd := range batch {
			if subCmd == nil {
				continue
			}
			subMsg := subCmd()
			if subMsg != nil {
				result, _ := tab.Update(subMsg)
				tab = result.(*SessionsTab)
			}
		}
		// Clear catalog — tests use mock runner, not real tmux sockets.
		tab.catalog = nil
		return tab
	}
	// Single message path.
	result, _ := tab.Update(msg)
	tab = result.(*SessionsTab)
	tab.catalog = nil
	return tab
}

func sendSessionKey(tab *SessionsTab, keyStr string) (*SessionsTab, tea.Cmd) {
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
	return result.(*SessionsTab), cmd
}

func TestSessionsTabID(t *testing.T) {
	tab, _ := newTestSessionsTab()
	if tab.ID() != dashboard.TabSessions {
		t.Errorf("expected TabSessions, got %s", tab.ID())
	}
}

func TestSessionsTabTitle(t *testing.T) {
	tab, _ := newTestSessionsTab()
	if tab.Title() != "Sessions" {
		t.Errorf("expected 'Sessions', got %q", tab.Title())
	}
}

func TestSessionsTabActivateLoadsSessions(t *testing.T) {
	tab, _ := newTestSessionsTab()
	tab = simulateActivate(tab)

	if len(tab.sessions) != 3 {
		t.Errorf("expected 3 sessions, got %d", len(tab.sessions))
	}
}

func TestSessionsTabNavigateDown(t *testing.T) {
	tab, _ := newTestSessionsTab()
	tab = simulateActivate(tab)

	if tab.cursor != 0 {
		t.Fatalf("expected cursor at 0, got %d", tab.cursor)
	}

	tab, _ = sendSessionKey(tab, "j")
	if tab.cursor != 1 {
		t.Errorf("expected cursor at 1, got %d", tab.cursor)
	}

	tab, _ = sendSessionKey(tab, "j")
	if tab.cursor != 2 {
		t.Errorf("expected cursor at 2, got %d", tab.cursor)
	}

	// Should not go past end.
	tab, _ = sendSessionKey(tab, "j")
	if tab.cursor != 2 {
		t.Errorf("expected cursor clamped at 2, got %d", tab.cursor)
	}
}

func TestSessionsTabNavigateUp(t *testing.T) {
	tab, _ := newTestSessionsTab()
	tab = simulateActivate(tab)
	tab.cursor = 2

	tab, _ = sendSessionKey(tab, "k")
	if tab.cursor != 1 {
		t.Errorf("expected cursor at 1, got %d", tab.cursor)
	}

	tab, _ = sendSessionKey(tab, "k")
	if tab.cursor != 0 {
		t.Errorf("expected cursor at 0, got %d", tab.cursor)
	}

	// Should not go past start.
	tab, _ = sendSessionKey(tab, "k")
	if tab.cursor != 0 {
		t.Errorf("expected cursor clamped at 0, got %d", tab.cursor)
	}
}

func TestSessionsTabEnterSwitchesSession(t *testing.T) {
	tab, _ := newTestSessionsTab()
	tab = simulateActivate(tab)

	_, cmd := sendSessionKey(tab, "enter")
	if cmd == nil {
		t.Fatal("expected command from enter")
	}

	msg := cmd()
	intent, ok := msg.(dashboard.QuitIntent)
	if !ok {
		t.Fatalf("expected QuitIntent, got %T", msg)
	}
	if intent.Action != "switch" {
		t.Errorf("expected action 'switch', got %q", intent.Action)
	}
}

func TestSessionsTabNewSession(t *testing.T) {
	tab, _ := newTestSessionsTab()
	tab = simulateActivate(tab)

	_, cmd := sendSessionKey(tab, "n")
	if cmd == nil {
		t.Fatal("expected command from n")
	}

	msg := cmd()
	intent, ok := msg.(dashboard.QuitIntent)
	if !ok {
		t.Fatalf("expected QuitIntent, got %T", msg)
	}
	if intent.Action != "new" {
		t.Errorf("expected action 'new', got %q", intent.Action)
	}
}

func TestSessionsTabRenameEntersMode(t *testing.T) {
	tab, _ := newTestSessionsTab()
	tab = simulateActivate(tab)

	tab, _ = sendSessionKey(tab, "r")
	if tab.mode != sessionsModeRename {
		t.Errorf("expected sessionsModeRename, got %d", tab.mode)
	}
}

func TestSessionsTabRenameCancel(t *testing.T) {
	tab, _ := newTestSessionsTab()
	tab = simulateActivate(tab)

	tab, _ = sendSessionKey(tab, "r")
	tab, _ = sendSessionKey(tab, "esc")

	if tab.mode != sessionsModeList {
		t.Errorf("expected sessionsModeList after esc, got %d", tab.mode)
	}
}

func TestSessionsTabRenameConfirm(t *testing.T) {
	tab, mock := newTestSessionsTab()
	tab = simulateActivate(tab)

	tab, _ = sendSessionKey(tab, "r")
	tab.renameInput.SetValue("newname")

	tab, cmd := sendSessionKey(tab, "enter")

	if tab.mode != sessionsModeList {
		t.Errorf("expected sessionsModeList after rename confirm, got %d", tab.mode)
	}

	if cmd != nil {
		msg := cmd()
		if msg != nil {
			tab.Update(msg)
		}
	}

	// Should have called RenameSession.
	found := false
	for _, c := range mock.Calls {
		if c.Method == "RenameSession" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected RenameSession to be called")
	}
}

func TestSessionsTabKillConfirmDialog(t *testing.T) {
	tab, _ := newTestSessionsTab()
	tab = simulateActivate(tab)

	tab, _ = sendSessionKey(tab, "x")
	if tab.mode != sessionsModeConfirmKill {
		t.Errorf("expected sessionsModeConfirmKill, got %d", tab.mode)
	}
	if tab.killTarget == "" {
		t.Error("expected killTarget to be set")
	}
}

func TestSessionsTabKillCancel(t *testing.T) {
	tab, _ := newTestSessionsTab()
	tab = simulateActivate(tab)

	tab, _ = sendSessionKey(tab, "x")
	tab, _ = sendSessionKey(tab, "n") // any key except y cancels

	if tab.mode != sessionsModeList {
		t.Errorf("expected sessionsModeList after cancel, got %d", tab.mode)
	}
}

func TestSessionsTabKillConfirm(t *testing.T) {
	tab, mock := newTestSessionsTab()
	tab = simulateActivate(tab)

	tab, _ = sendSessionKey(tab, "x")
	_, cmd := sendSessionKey(tab, "y")

	if cmd != nil {
		msg := cmd()
		if msg != nil {
			// Don't need to update, just check mock calls.
		}
	}

	found := false
	for _, c := range mock.Calls {
		if c.Method == "KillSession" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected KillSession to be called")
	}
}

func TestSessionsTabPreview(t *testing.T) {
	tab, _ := newTestSessionsTab()
	tab = simulateActivate(tab)

	_, cmd := sendSessionKey(tab, "p")
	if cmd == nil {
		t.Fatal("expected command from p")
	}

	msg := cmd()
	if msg == nil {
		t.Fatal("expected preview message")
	}

	result, _ := tab.Update(msg)
	tab = result.(*SessionsTab)

	if tab.mode != sessionsModePreview {
		t.Errorf("expected sessionsModePreview, got %d", tab.mode)
	}
}

func TestSessionsTabPreviewDismiss(t *testing.T) {
	tab, _ := newTestSessionsTab()
	tab = simulateActivate(tab)
	tab.mode = sessionsModePreview
	tab.previewContent = "hello"

	tab, _ = sendSessionKey(tab, "x")
	if tab.mode != sessionsModeList {
		t.Errorf("expected sessionsModeList after dismiss, got %d", tab.mode)
	}
	if tab.previewContent != "" {
		t.Error("expected previewContent to be cleared")
	}
}

func TestSessionsTabMoveTab(t *testing.T) {
	tab, _ := newTestSessionsTab()
	tab = simulateActivate(tab)

	_, cmd := sendSessionKey(tab, "m")
	if cmd == nil {
		t.Fatal("expected command from m")
	}

	msg := cmd()
	if msg == nil {
		t.Fatal("expected move destinations message")
	}

	result, _ := tab.Update(msg)
	tab = result.(*SessionsTab)

	if tab.mode != sessionsModeMove {
		t.Errorf("expected sessionsModeMove, got %d", tab.mode)
	}
}

func TestSessionsTabMoveCancelOnEsc(t *testing.T) {
	tab, _ := newTestSessionsTab()
	tab = simulateActivate(tab)
	tab.mode = sessionsModeMove
	tab.moveTargets = []session.SessionInfo{{Name: "other"}}

	tab, _ = sendSessionKey(tab, "esc")
	if tab.mode != sessionsModeList {
		t.Errorf("expected sessionsModeList after esc, got %d", tab.mode)
	}
}

func TestSessionsTabCleanupTmp(t *testing.T) {
	tab, mock := newTestSessionsTab()
	tab = simulateActivate(tab)

	_, cmd := sendSessionKey(tab, "c")
	if cmd != nil {
		msg := cmd()
		if msg != nil {
			tab.Update(msg)
		}
	}

	found := false
	for _, c := range mock.Calls {
		if c.Method == "KillSession" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected KillSession to be called for tmp session cleanup")
	}
}

func TestSessionsTabViewRendersSessionList(t *testing.T) {
	tab, _ := newTestSessionsTab()
	tab = simulateActivate(tab)

	view := tab.View()

	if !strings.Contains(view, "dev") {
		t.Error("expected view to contain session 'dev'")
	}
	if !strings.Contains(view, "api") {
		t.Error("expected view to contain session 'api'")
	}
	if !strings.Contains(view, "tmp-1") {
		t.Error("expected view to contain session 'tmp-1'")
	}
}

func TestSessionsTabViewShowsWindowNames(t *testing.T) {
	tab, _ := newTestSessionsTab()
	tab = simulateActivate(tab)

	view := tab.View()

	if !strings.Contains(view, "editor") {
		t.Error("expected view to contain window name 'editor'")
	}
	if !strings.Contains(view, "server") {
		t.Error("expected view to contain window name 'server'")
	}
}

func TestSessionsTabViewShowsAttachedStatus(t *testing.T) {
	tab, _ := newTestSessionsTab()
	tab = simulateActivate(tab)

	view := tab.View()

	if !strings.Contains(view, "attached") {
		t.Error("expected view to contain 'attached' status")
	}
}

func TestSessionsTabViewEmptyState(t *testing.T) {
	mock := tmux.NewMockRunner()
	styles := tui.DefaultStyles()
	tab := NewSessionsTab(mock, styles)
	tab.Resize(80, 40)

	// Activate with no sessions.
	tab = simulateActivate(tab)

	view := tab.View()
	if !strings.Contains(view, "No sessions") {
		t.Error("expected empty state message")
	}
}

func TestSessionsTabDeactivateDropsOverlays(t *testing.T) {
	tab, _ := newTestSessionsTab()
	tab = simulateActivate(tab)

	// Enter confirm kill mode.
	tab.mode = sessionsModeConfirmKill
	tab.killTarget = "dev"

	tab.Deactivate()

	if tab.mode != sessionsModeList {
		t.Errorf("expected mode reset to list, got %d", tab.mode)
	}
}

func TestSessionsTabSelectionPersistence(t *testing.T) {
	tab, _ := newTestSessionsTab()
	tab = simulateActivate(tab)

	// Sessions are sorted alphabetically: api, dev, tmp-1.
	// Cursor starts at 0 (api). Move to "dev" (index 1).
	tab, _ = sendSessionKey(tab, "j")
	if tab.selectedName != "dev" {
		t.Errorf("expected selectedName 'dev', got %q", tab.selectedName)
	}

	// Simulate a refetch that keeps the same sessions.
	tab.restoreCursor()
	if tab.cursor != 1 {
		t.Errorf("expected cursor restored to 1 (dev), got %d", tab.cursor)
	}
}

func TestSessionsTabShortHelp(t *testing.T) {
	tab, _ := newTestSessionsTab()

	help := tab.ShortHelp()
	if !strings.Contains(help, "enter:switch") {
		t.Error("expected help to contain 'enter:switch'")
	}
	if !strings.Contains(help, "n:new") {
		t.Error("expected help to contain 'n:new'")
	}
}

func TestSessionsTabJumpToTopBottom(t *testing.T) {
	tab, _ := newTestSessionsTab()
	tab = simulateActivate(tab)

	tab, _ = sendSessionKey(tab, "G")
	if tab.cursor != len(tab.sessions)-1 {
		t.Errorf("expected cursor at end, got %d", tab.cursor)
	}

	tab, _ = sendSessionKey(tab, "g")
	if tab.cursor != 0 {
		t.Errorf("expected cursor at 0, got %d", tab.cursor)
	}
}
