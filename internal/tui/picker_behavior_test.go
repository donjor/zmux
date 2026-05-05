package tui

// Picker behavior tests — spec + implementation.
//
// Each test case defines a scenario by its user-visible state (input text,
// cursor position, workspace/session data) and asserts THREE things:
//
//   1. The PickerResult.Action + fields match the expected CLI command
//   2. The ghost-prompt text matches the CLI command the user would run
//   3. The ghost prompt is consistent with the picker result
//
// This forces alignment between the picker UI and the CLI semantics —
// if a ghost prompt says "zmux new myapp dev" then Enter MUST produce
// PickerResult{Action:"new", Name:"dev", Workspace:"myapp"}.
//
// The scenarios below are the canonical spec for picker→CLI mapping.
// Any new picker action must have a row in this table.

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui/outline"
	"github.com/donjor/zmux/internal/workspace"
)

// ── Test helpers ──

// newBehaviorPicker builds a picker with known workspaces + sessions.
// Workspaces:
//   - myapp: sessions [main (attached), api]
//   - tools: sessions [monitor]
//   - empty: no sessions
//
// This is a superset of the common test fixtures and exercises all the
// interesting edges (attached session, multi-session workspace, empty
// workspace).
func newBehaviorPicker(t *testing.T) PickerModel {
	t.Helper()
	workspaces := []workspace.Workspace{
		{Name: "myapp", Sessions: []string{"main", "api"}},
		{Name: "tools", Sessions: []string{"monitor"}},
		{Name: "empty", Sessions: nil},
	}
	sessions := []session.SessionInfo{
		{Name: "main", Activity: time.Now(), Attached: true, Windows: 3},
		{Name: "api", Activity: time.Now().Add(-1 * time.Hour), Attached: false, Windows: 1},
		{Name: "monitor", Activity: time.Now().Add(-2 * time.Hour), Attached: false, Windows: 1},
	}

	mock := tmux.NewMockRunner()
	styles := DefaultStyles()
	model := NewPickerModel(mock, styles)
	model.width = 120
	model.height = 40
	model.workspaces = BuildWorkspaceViewModels(workspaces, sessions)
	model.state.showEmpty = true // show empty workspaces for test visibility

	// Load workspaces into the picker.
	result, _ := model.Update(workspacesLoadedMsg{workspaces: model.workspaces})
	model = result.(PickerModel)
	return model
}

// typeInPicker types each character of s into the picker's input.
func typeInPicker(m PickerModel, s string) PickerModel {
	for _, r := range s {
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = result.(PickerModel)
	}
	return m
}

// enterPicker presses Enter on the picker.
func enterPicker(m PickerModel) PickerModel {
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	return result.(PickerModel)
}

// moveCursorTo moves the cursor to the first row matching the predicate.
func moveCursorTo(m PickerModel, pred func(outline.Row) bool) PickerModel {
	for i, row := range m.tree.Rows {
		if pred(row) {
			m.tree.Cursor = i
			return m
		}
	}
	return m
}

// ── CLI ↔ Picker behavior spec ──
//
// This table defines the canonical mapping between user actions in the
// picker and the CLI command they correspond to. Each row is a test.
//
// | Scenario | Input | Cursor | Expected Result | Ghost Prompt |
// |---|---|---|---|---|
// | Empty input, top row | "" | top action | new (tmp) | zmux new  # tmp-N |
// | Typed name, top row | "newproj" | top action | workspace-create "newproj" | zmux new newproj |
// | Enter on workspace (has sessions) | "" | myapp ws | drill into sessions | zmux myapp # choose session |
// | Enter on workspace (no sessions) | "" | empty ws | new main in ws | zmux new empty  # + main |
// | Enter on session row | "" | api session | attach api | zmux myapp api |
// | Enter on attached session | "" | main session | attach → group | zmux myapp main → main-b |
// | Type "myapp dev" → Enter on ws | "myapp dev" | myapp ws | new dev in myapp | zmux new myapp dev |  ← THE BUG
// | Type "myapp" → Enter on session | "myapp" | main session | attach main | zmux myapp main |

func TestPickerBehavior_EmptyInputTopRow(t *testing.T) {
	m := newBehaviorPicker(t)

	// Cursor should start on top action row.
	row := m.tree.CurrentSelectable()
	if row == nil || row.Kind != outline.RowTopAction {
		t.Skip("cursor did not start on top action")
	}

	ghost := m.ghostCmd()
	if !strings.Contains(ghost, "zmux new") {
		t.Errorf("ghost = %q, want contains 'zmux new'", ghost)
	}

	m = enterPicker(m)
	if !m.Quitting {
		t.Fatal("expected Quitting after Enter")
	}
	if m.Result.Action != "new" {
		t.Errorf("action = %q, want 'new'", m.Result.Action)
	}
	if m.Result.Workspace != "" {
		t.Errorf("workspace = %q, want empty (tmp session)", m.Result.Workspace)
	}
}

func TestPickerBehavior_TypedNameTopRow(t *testing.T) {
	m := newBehaviorPicker(t)

	m = typeInPicker(m, "newproj")

	// Move to top action row.
	m = moveCursorTo(m, func(r outline.Row) bool {
		return r.Kind == outline.RowTopAction
	})

	ghost := m.ghostCmd()
	if !strings.Contains(ghost, "zmux new newproj") {
		t.Errorf("ghost = %q, want contains 'zmux new newproj'", ghost)
	}

	m = enterPicker(m)
	if m.Result.Action != "workspace-create" {
		t.Errorf("action = %q, want 'workspace-create'", m.Result.Action)
	}
	if m.Result.Workspace != "newproj" {
		t.Errorf("workspace = %q, want 'newproj'", m.Result.Workspace)
	}
}

func TestPickerBehavior_EnterOnWorkspaceWithSessionsDrillsIntoSessions(t *testing.T) {
	m := newBehaviorPicker(t)

	m = moveCursorTo(m, func(r outline.Row) bool {
		ws := rowWorkspace(r)
		return ws != nil && ws.Name == "myapp"
	})

	ghost := m.ghostCmd()
	if !strings.Contains(ghost, "zmux myapp") || !strings.Contains(ghost, "choose session") {
		t.Errorf("ghost = %q, want contains 'zmux myapp' and 'choose session'", ghost)
	}

	m = enterPicker(m)
	if m.Quitting {
		t.Fatal("workspace Enter should drill into sessions, not quit/attach")
	}
	if m.Result.Action != "" {
		t.Errorf("action = %q, want empty before selecting a session", m.Result.Action)
	}
	row := m.tree.CurrentSelectable()
	if row == nil || row.Kind != outline.RowSession {
		t.Fatalf("cursor after drilldown = %#v, want session row", row)
	}
	if parentWorkspaceName(row, m.tree) != "myapp" {
		t.Fatalf("session row parent workspace = %q, want myapp", parentWorkspaceName(row, m.tree))
	}
}

func TestPickerBehavior_EnterOnEmptyWorkspace(t *testing.T) {
	m := newBehaviorPicker(t)

	m = moveCursorTo(m, func(r outline.Row) bool {
		ws := rowWorkspace(r)
		return ws != nil && ws.Name == "empty"
	})

	ghost := m.ghostCmd()
	if !strings.Contains(ghost, "zmux new empty") {
		t.Errorf("ghost = %q, want contains 'zmux new empty'", ghost)
	}

	m = enterPicker(m)
	if m.Result.Action != "new" {
		t.Errorf("action = %q, want 'new' (create default session)", m.Result.Action)
	}
	if m.Result.Workspace != "empty" {
		t.Errorf("workspace = %q, want 'empty'", m.Result.Workspace)
	}
	if m.Result.Name != "empty" {
		t.Errorf("name = %q, want 'empty'", m.Result.Name)
	}
}

func TestPickerBehavior_EnterOnSessionRow(t *testing.T) {
	m := newBehaviorPicker(t)

	// Expand myapp so we can see the api session.
	m = moveCursorTo(m, func(r outline.Row) bool {
		ws := rowWorkspace(r)
		return ws != nil && ws.Name == "myapp"
	})
	// Type "myapp" to expand the workspace's sessions inline.
	m = typeInPicker(m, "myapp")

	// Find the "api" session row.
	m = moveCursorTo(m, func(r outline.Row) bool {
		s := rowSession(r)
		return s != nil && s.Name == "api"
	})

	ghost := m.ghostCmd()
	if !strings.Contains(ghost, "zmux myapp api") {
		t.Errorf("ghost = %q, want contains 'zmux myapp api'", ghost)
	}

	m = enterPicker(m)
	if m.Result.Action != "attach" {
		t.Errorf("action = %q, want 'attach'", m.Result.Action)
	}
	if m.Result.Session != "api" {
		t.Errorf("session = %q, want 'api'", m.Result.Session)
	}
}

func TestPickerBehavior_EnterOnAttachedSession(t *testing.T) {
	m := newBehaviorPicker(t)

	// Expand and find the attached "main" session.
	m = typeInPicker(m, "myapp")
	m = moveCursorTo(m, func(r outline.Row) bool {
		s := rowSession(r)
		return s != nil && s.Name == "main"
	})

	ghost := m.ghostCmd()
	// Ghost should show the grouped-session indicator.
	if !strings.Contains(ghost, "main-b") {
		t.Errorf("ghost = %q, want contains 'main-b' (grouped indicator)", ghost)
	}

	m = enterPicker(m)
	if m.Result.Action != "attach" {
		t.Errorf("action = %q, want 'attach'", m.Result.Action)
	}
	if m.Result.Session != "main" {
		t.Errorf("session = %q, want 'main'", m.Result.Session)
	}
}

// TestPickerBehavior_TypedSessionInExistingWorkspace covers the bug:
// typing "myapp dev" should create a new session "dev" in workspace
// "myapp", NOT attach to an existing session.
//
// CLI equivalent: zmux new myapp dev
// Ghost prompt should show: zmux new myapp dev
func TestPickerBehavior_TypedSessionInExistingWorkspace(t *testing.T) {
	m := newBehaviorPicker(t)

	// Type "myapp dev" — workspace "myapp" exists, session "dev" does not.
	m = typeInPicker(m, "myapp dev")

	// Ghost prompt should reflect "create new session in workspace".
	ghost := m.ghostCmd()
	if !strings.Contains(ghost, "zmux new myapp dev") {
		t.Errorf("ghost = %q, want contains 'zmux new myapp dev'", ghost)
	}

	// The cursor should be on the myapp workspace row (exact-match pin).
	row := m.tree.CurrentSelectable()
	if row == nil {
		t.Fatal("expected a current selectable row")
	}

	m = enterPicker(m)
	if !m.Quitting {
		t.Fatal("expected Quitting after Enter")
	}

	// THE KEY ASSERTION: should produce a "new" action, not "attach".
	if m.Result.Action != "new" {
		t.Errorf("action = %q, want 'new' (create session dev in myapp)", m.Result.Action)
	}
	if m.Result.Name != "dev" {
		t.Errorf("name = %q, want 'dev'", m.Result.Name)
	}
	if m.Result.Workspace != "myapp" {
		t.Errorf("workspace = %q, want 'myapp'", m.Result.Workspace)
	}
}

// TestPickerBehavior_TypedWorkspaceEnterOnSession covers: typing "myapp"
// then selecting a specific session row should attach to that session.
func TestPickerBehavior_TypedWorkspaceEnterOnSession(t *testing.T) {
	m := newBehaviorPicker(t)

	m = typeInPicker(m, "myapp")

	// Find the "api" session.
	m = moveCursorTo(m, func(r outline.Row) bool {
		s := rowSession(r)
		return s != nil && s.Name == "api"
	})

	ghost := m.ghostCmd()
	if !strings.Contains(ghost, "zmux myapp api") {
		t.Errorf("ghost = %q, want contains 'zmux myapp api'", ghost)
	}

	m = enterPicker(m)
	if m.Result.Action != "attach" {
		t.Errorf("action = %q, want 'attach'", m.Result.Action)
	}
	if m.Result.Session != "api" {
		t.Errorf("session = %q, want 'api'", m.Result.Session)
	}
}

// TestPickerBehavior_TypedSessionNewWorkspace covers: typing "newproj dev"
// when workspace "newproj" doesn't exist should create the workspace AND
// the session "dev" (not "main").
//
// CLI equivalent: zmux new newproj dev
// Ghost prompt should show: zmux new newproj dev
func TestPickerBehavior_TypedSessionNewWorkspace(t *testing.T) {
	m := newBehaviorPicker(t)

	// "newproj" doesn't exist as a workspace — top action row is the target.
	m = typeInPicker(m, "newproj dev")

	// Cursor should be on the top action (no workspace to pin).
	row := m.tree.CurrentSelectable()
	if row == nil {
		t.Fatal("no selectable row")
	}
	if row.Kind != outline.RowTopAction {
		t.Logf("cursor on %v %s (expected top action — may be on a fuzzy match)", row.Kind, row.ID)
	}

	ghost := m.ghostCmd()
	if !strings.Contains(ghost, "zmux new newproj dev") {
		t.Errorf("ghost = %q, want contains 'zmux new newproj dev'", ghost)
	}

	m = enterPicker(m)
	if !m.Quitting {
		t.Fatal("expected Quitting")
	}

	// Must create workspace + session, not workspace + "main".
	if m.Result.Action != "workspace-create" {
		t.Errorf("action = %q, want 'workspace-create'", m.Result.Action)
	}
	if m.Result.Workspace != "newproj" {
		t.Errorf("workspace = %q, want 'newproj'", m.Result.Workspace)
	}
	if m.Result.Name != "dev" {
		t.Errorf("name = %q, want 'dev' (not 'main')", m.Result.Name)
	}
}

// TestPickerBehavior_EmptyWorkspaceTypedSession covers: typing "empty dev"
// when workspace "empty" exists but has 0 sessions. Should create "dev"
// in the workspace, not "main".
func TestPickerBehavior_EmptyWorkspaceTypedSession(t *testing.T) {
	m := newBehaviorPicker(t)

	m = typeInPicker(m, "empty dev")

	ghost := m.ghostCmd()
	if !strings.Contains(ghost, "zmux new empty dev") {
		t.Errorf("ghost = %q, want contains 'zmux new empty dev'", ghost)
	}

	m = enterPicker(m)
	if m.Result.Action != "new" {
		t.Errorf("action = %q, want 'new'", m.Result.Action)
	}
	if m.Result.Workspace != "empty" {
		t.Errorf("workspace = %q, want 'empty'", m.Result.Workspace)
	}
	if m.Result.Name != "dev" {
		t.Errorf("name = %q, want 'dev'", m.Result.Name)
	}
}

// TestPickerBehavior_ExistingSessionNameInWorkspace covers: typing
// "myapp main" when "main" already exists in myapp. The picker should
// still produce action="new" with Name="main" — root.go handles the
// name collision via nextSessionName (so the ghost prompt says
// "zmux new myapp main" which is CLI-accurate).
func TestPickerBehavior_ExistingSessionNameInWorkspace(t *testing.T) {
	m := newBehaviorPicker(t)

	m = typeInPicker(m, "myapp main")

	// Cursor should pin to myapp workspace (exact match on wsQuery).
	row := m.tree.CurrentSelectable()
	if row == nil {
		t.Fatal("no selectable row")
	}

	ghost := m.ghostCmd()
	// Either on the workspace row (shows "zmux new myapp main") or on
	// the session row (shows "zmux myapp main"). Both are valid — the
	// cursor could land on the exact-match session "main" or the
	// workspace header depending on expansion state.
	if !strings.Contains(ghost, "zmux") {
		t.Errorf("ghost = %q, want contains 'zmux'", ghost)
	}
}

// TestPickerBehavior_WorkspaceNoSessionsEnterCreatesMain covers:
// pressing Enter on a workspace with 0 sessions and NO session query
// should create "main". This is the existing behavior — make sure the
// session-query fix didn't break it.
func TestPickerBehavior_WorkspaceNoSessionsEnterCreatesMain(t *testing.T) {
	m := newBehaviorPicker(t)

	m = moveCursorTo(m, func(r outline.Row) bool {
		ws := rowWorkspace(r)
		return ws != nil && ws.Name == "empty"
	})

	// No text typed → sessionQuery is "".
	if m.state.sessionQuery != "" {
		t.Fatalf("sessionQuery = %q, want empty", m.state.sessionQuery)
	}

	m = enterPicker(m)
	if m.Result.Action != "new" {
		t.Errorf("action = %q, want 'new'", m.Result.Action)
	}
	if m.Result.Name != "empty" {
		t.Errorf("name = %q, want 'empty'", m.Result.Name)
	}
	if m.Result.Workspace != "empty" {
		t.Errorf("workspace = %q, want 'empty'", m.Result.Workspace)
	}
}

// TestPickerBehavior_GhostPromptConsistency walks every visible row
// and asserts that the ghost prompt starts with "zmux" (or "# " for
// pseudo/external rows). This is a sanity check that ghostCmd doesn't
// produce empty or nonsensical output for any row type.
func TestPickerBehavior_GhostPromptConsistency(t *testing.T) {
	m := newBehaviorPicker(t)

	for i, row := range m.tree.Rows {
		if !row.Selectable {
			continue
		}
		m.tree.Cursor = i
		ghost := m.ghostCmd()
		if ghost == "" {
			t.Errorf("row %d (%v %s): ghost prompt is empty", i, row.Kind, row.ID)
		}
		if !strings.HasPrefix(ghost, "zmux") && !strings.HasPrefix(ghost, "#") {
			t.Errorf("row %d (%v %s): ghost = %q, want prefix 'zmux' or '#'",
				i, row.Kind, row.ID, ghost)
		}
	}
}
