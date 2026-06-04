package wspicker

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tui/styles"
	"github.com/donjor/zmux/internal/tui/workspaceview"
	"github.com/donjor/zmux/internal/workspace"
)

func vm(name string, sessions ...session.SessionInfo) workspaceview.WorkspaceViewModel {
	return workspaceview.WorkspaceViewModel{
		Workspace:    workspace.Workspace{Name: name},
		LiveSessions: sessions,
	}
}

func newTestModel(workspaces []workspaceview.WorkspaceViewModel) Model {
	loader := func() []workspaceview.WorkspaceViewModel { return workspaces }
	m := NewModel(loader, styles.DefaultStyles())
	// Drive the load synchronously. tea.Batch from Init wraps commands and
	// returns a BatchMsg, so call the loader directly and push the typed
	// loadedMsg through Update instead of running tea.Program.
	updated, _ := m.Update(loadedMsg{workspaces: loader()})
	return updated.(Model)
}

func typeKeys(m Model, keys ...string) Model {
	for _, k := range keys {
		updated, _ := m.Update(tea.KeyPressMsg{Code: keyCodeFor(k), Text: k})
		m = updated.(Model)
	}
	return m
}

// keyCodeFor maps a single-char "k" to the corresponding rune for KeyPressMsg.
// Multi-char names like "enter" / "esc" are forwarded via Text; the rune is 0
// in those cases, which bubbletea/v2 tolerates.
func keyCodeFor(k string) rune {
	if len(k) == 1 {
		return rune(k[0])
	}
	return 0
}

func sendNamed(m Model, name string) Model {
	updated, cmd := m.Update(tea.KeyPressMsg{Code: 0, Text: name})
	m = updated.(Model)
	// Selection is now a two-step: Enter → OnSelect cmd → workspaceSelectedMsg
	// → Update sets Result+Quit. Drain the resulting command(s) so the typed
	// follow-up message is delivered (bounded; these cmds return instantly).
	for i := 0; cmd != nil && i < 4; i++ {
		msg := cmd()
		if msg == nil {
			break
		}
		updated, cmd = m.Update(msg)
		m = updated.(Model)
	}
	return m
}

// focusedName returns the name of the workspace under the embedded list's
// cursor, or "" if none. The wrapper exposes list state only through the
// component's public surface (Focused / FilterText) — fuzzy-match internals
// are covered by workspacelist's own tests.
func focusedName(m Model) string {
	if ws := m.list.Focused(); ws != nil {
		return ws.Name
	}
	return ""
}

func TestWspicker_LoadsAndFocusesFirst(t *testing.T) {
	wss := []workspaceview.WorkspaceViewModel{
		vm("alpha", session.SessionInfo{Name: "main"}),
		vm("beta"),
	}
	m := newTestModel(wss)

	if got := focusedName(m); got != "alpha" {
		t.Fatalf("focused = %q, want alpha", got)
	}
}

// ShowEmpty:true parity — workspaces with no live sessions (beta) must still
// appear; the component hides them by default.
func TestWspicker_ShowsEmptyWorkspaces(t *testing.T) {
	wss := []workspaceview.WorkspaceViewModel{
		vm("alpha", session.SessionInfo{Name: "main"}),
		vm("beta"),
	}
	m := newTestModel(wss)
	m = typeKeys(m, "b", "e", "t", "a")

	if got := focusedName(m); got != "beta" {
		t.Fatalf("focused after filtering to 'beta' = %q, want beta (empty workspaces must show)", got)
	}
}

func TestWspicker_FuzzyFilters(t *testing.T) {
	wss := []workspaceview.WorkspaceViewModel{vm("alpha"), vm("beta"), vm("gamma")}
	m := newTestModel(wss)
	m = typeKeys(m, "b", "e")

	if got := focusedName(m); got != "beta" {
		t.Fatalf("focused after 'be' = %q, want beta", got)
	}
}

func TestWspicker_EnterReturnsSwitch(t *testing.T) {
	wss := []workspaceview.WorkspaceViewModel{vm("alpha"), vm("beta")}
	m := newTestModel(wss)

	m = sendNamed(m, "enter")
	if !m.Quitting {
		t.Fatal("expected Quitting after Enter")
	}
	if m.Result.Action != "switch" {
		t.Errorf("Result.Action = %q, want switch", m.Result.Action)
	}
	if m.Result.Workspace != "alpha" {
		t.Errorf("Result.Workspace = %q, want alpha", m.Result.Workspace)
	}
}

func TestWspicker_EscClearsThenQuits(t *testing.T) {
	wss := []workspaceview.WorkspaceViewModel{vm("alpha")}
	m := newTestModel(wss)

	m = typeKeys(m, "a")
	if m.list.FilterText() != "a" {
		t.Fatalf("filter = %q, want 'a'", m.list.FilterText())
	}

	// First esc clears the filter (handled inside the embedded list).
	m = sendNamed(m, "esc")
	if m.Quitting {
		t.Fatal("esc with input should clear, not quit")
	}
	if m.list.FilterText() != "" {
		t.Errorf("filter after esc = %q, want empty", m.list.FilterText())
	}

	// Second esc quits without a result.
	m = sendNamed(m, "esc")
	if !m.Quitting {
		t.Fatal("esc on empty input should quit")
	}
	if m.Result.Action != "" {
		t.Errorf("Result.Action = %q, want empty", m.Result.Action)
	}
}

func TestWspicker_EmptyListEnterIsNoop(t *testing.T) {
	m := newTestModel(nil)
	m = sendNamed(m, "enter")
	if m.Quitting {
		t.Fatal("enter on empty list should not quit")
	}
}
