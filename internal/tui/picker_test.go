package tui

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

// newTestPickerWithWorkspaces builds a picker with 2 real workspaces and a
// tmp session (which lands under the "temporary" pseudo-workspace).
func newTestPickerWithWorkspaces() PickerModel {
	m := tmux.NewMockRunner()
	now := time.Now()
	m.Sessions = []tmux.Session{
		{Name: "dev", Windows: 3, Attached: true, Activity: now, Created: now.Add(-2 * time.Hour), LastAttached: now.Add(-10 * time.Minute), Dir: "/home/user/bridge"},
		{Name: "monitor", Windows: 1, Attached: false, Activity: now.Add(-1 * time.Hour), Created: now.Add(-3 * time.Hour), Dir: "/home/user/bridge"},
		{Name: "zmux", Windows: 2, Attached: false, Activity: now.Add(-30 * time.Minute), Created: now.Add(-24 * time.Hour), Dir: "/home/user/zmux"},
		{Name: "tmp-1", Windows: 1, Attached: false, Activity: now.Add(-5 * time.Minute), Created: now.Add(-5 * time.Minute), Dir: "/tmp"},
	}
	m.Windows["dev"] = []tmux.Window{{Index: 1, Name: "editor", Active: true}, {Index: 2, Name: "server", Active: false}, {Index: 3, Name: "git", Active: false}}
	m.Windows["monitor"] = []tmux.Window{{Index: 1, Name: "htop", Active: true}}
	m.Windows["zmux"] = []tmux.Window{{Index: 1, Name: "nvim", Active: true}}
	m.Windows["tmp-1"] = []tmux.Window{{Index: 1, Name: "zsh", Active: true}}

	styles := DefaultStyles()
	model := NewPickerModel(m, styles)
	model.width = 120
	model.height = 40

	sessions, _ := session.ListSessions(m)
	workspaces := []workspace.Workspace{
		{Name: "bridge", RootDir: "/home/user/bridge", Sessions: []string{"dev", "monitor"}, LastActiveSession: "dev"},
		{Name: "zmux-dev", RootDir: "/home/user/zmux", Sessions: []string{"zmux"}, LastActiveSession: "zmux"},
	}
	model.workspaces = BuildWorkspaceViewModels(workspaces, sessions)
	model.filteredWorkspaces = model.workspaces
	model.buildOutline()

	wins := make(map[string][]tmux.Window)
	for _, s := range sessions {
		w, _ := m.ListWindows(s.Name)
		wins[s.Name] = w
	}
	model.windows = wins

	return model
}

func newEmptyPicker() PickerModel {
	m := tmux.NewMockRunner()
	styles := DefaultStyles()
	model := NewPickerModel(m, styles)
	model.width = 120
	model.height = 40
	model.buildOutline()
	return model
}

// findRowIndex returns the index of the first row matching the predicate.
func findRowIndex(m PickerModel, pred func(outline.Row) bool) int {
	for i, row := range m.tree.Rows {
		if pred(row) {
			return i
		}
	}
	return -1
}

// rowWorkspace returns the workspace payload from a row, or nil.
func rowWorkspace(r outline.Row) *WorkspaceViewModel {
	ws, _ := outline.RowData[WorkspaceViewModel](&r)
	return ws
}

// rowSession returns the session payload from a row, or nil.
func rowSession(r outline.Row) *session.SessionInfo {
	s, _ := outline.RowData[session.SessionInfo](&r)
	return s
}

// ── Basic rendering ──

func TestPickerShowsWorkspaces(t *testing.T) {
	model := newTestPickerWithWorkspaces()
	view := model.View()

	if !strings.Contains(view, "bridge") {
		t.Error("expected workspace 'bridge' in view")
	}
	if !strings.Contains(view, "zmux-dev") {
		t.Error("expected workspace 'zmux-dev' in view")
	}
}

func TestPickerShowsTmpSessionTopAction(t *testing.T) {
	model := newTestPickerWithWorkspaces()
	view := model.View()

	if !strings.Contains(view, "+ new tmp session") {
		t.Error("expected '+ new tmp session' top action")
	}
}

func TestPickerShowsCreateOnType(t *testing.T) {
	model := newTestPickerWithWorkspaces()
	model.input.SetValue("myapp")
	model.onInputChanged()
	view := model.View()

	if !strings.Contains(view, "+ create \"myapp\"") {
		t.Error("expected '+ create' top action when typed")
	}
}

func TestPickerShowsTemporaryPseudoWorkspace(t *testing.T) {
	model := newTestPickerWithWorkspaces()
	view := model.View()

	if !strings.Contains(view, "temporary") {
		t.Error("expected 'temporary' pseudo-workspace for tmp-1")
	}
}

// ── Items list structure ──

func TestPickerItemsTopActionFirst(t *testing.T) {
	model := newTestPickerWithWorkspaces()

	if len(model.tree.Rows) == 0 {
		t.Fatal("expected items list to be populated")
	}
	if model.tree.Rows[0].Kind != outline.RowTopAction {
		t.Error("expected first item to be top action")
	}
}

func TestPickerItemsContainWorkspaces(t *testing.T) {
	model := newTestPickerWithWorkspaces()

	found := 0
	for _, item := range model.tree.Rows {
		if item.Kind == outline.RowWorkspaceHeader && rowWorkspace(item) != nil {
			if rowWorkspace(item).Name == "bridge" || rowWorkspace(item).Name == "zmux-dev" {
				found++
			}
		}
	}
	if found != 2 {
		t.Errorf("expected 2 workspace items, found %d", found)
	}
}

// ── Cursor navigation ──

func TestPickerCursorStartsAtTopAction(t *testing.T) {
	model := newTestPickerWithWorkspaces()
	if model.tree.Cursor != 0 {
		t.Errorf("expected cursor 0 at start, got %d", model.tree.Cursor)
	}
}

func TestPickerArrowDown(t *testing.T) {
	model := newTestPickerWithWorkspaces()

	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	m := result.(PickerModel)

	if m.tree.Cursor != 1 {
		t.Errorf("expected cursor 1 after Down, got %d", m.tree.Cursor)
	}
}

func TestPickerArrowDownTraverses(t *testing.T) {
	model := newTestPickerWithWorkspaces()

	m := model
	for i := 0; i < 10; i++ {
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = result.(PickerModel)
	}

	// Should have advanced but not past end.
	if m.tree.Cursor >= len(m.tree.Rows) {
		t.Errorf("cursor out of bounds: %d (items=%d)", m.tree.Cursor, len(m.tree.Rows))
	}
	if m.tree.Cursor == 0 {
		t.Error("expected cursor to move from 0")
	}
}

func TestPickerArrowUp(t *testing.T) {
	model := newTestPickerWithWorkspaces()
	model.tree.Cursor = 2

	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyUp})
	m := result.(PickerModel)

	if m.tree.Cursor != 1 {
		t.Errorf("expected cursor 1, got %d", m.tree.Cursor)
	}
}

func TestPickerArrowUpClampsAtZero(t *testing.T) {
	model := newTestPickerWithWorkspaces()

	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyUp})
	m := result.(PickerModel)

	if m.tree.Cursor != 0 {
		t.Errorf("expected cursor clamped at 0, got %d", m.tree.Cursor)
	}
}

// Expansion: cursor on workspace shows its sessions beneath.
func TestPickerCursorOnWorkspaceExpandsSessions(t *testing.T) {
	model := newTestPickerWithWorkspaces()

	// Move to first workspace (cursor 1).
	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	m := result.(PickerModel)

	// After expansion, there should be session items directly following.
	if m.tree.Cursor+1 >= len(m.tree.Rows) {
		t.Fatalf("expected items beyond cursor, got %d items", len(m.tree.Rows))
	}
	next := m.tree.Rows[m.tree.Cursor+1]
	if next.Kind != outline.RowSession {
		t.Errorf("expected next item to be a session under the workspace, got kind=%d", next.Kind)
	}
}

// Down from workspace traverses into sessions, not past them.
func TestPickerDownFromWorkspaceIntoSessions(t *testing.T) {
	model := newTestPickerWithWorkspaces()

	// cursor 0 → 1 (first workspace with sessions)
	m := model
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = result.(PickerModel)
	// cursor 1 → 2 (should be a session of that workspace)
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = result.(PickerModel)

	item := m.tree.CurrentSelectable()
	if item == nil {
		t.Fatal("expected item under cursor")
	}
	if item.Kind != outline.RowSession {
		t.Errorf("expected session at cursor, got kind=%d", item.Kind)
	}
}

// ── Enter behavior ──

func TestPickerEnterEmptyCreatesTmp(t *testing.T) {
	model := newTestPickerWithWorkspaces()

	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := result.(PickerModel)

	if !m.Quitting {
		t.Error("expected quitting after Enter")
	}
	if m.Result.Action != "new" {
		t.Errorf("expected action 'new' (tmp), got %q", m.Result.Action)
	}
	if m.Result.Workspace != "" {
		t.Errorf("expected empty workspace for tmp, got %q", m.Result.Workspace)
	}
}

func TestPickerEnterOnTopActionWithTypedCreatesWorkspace(t *testing.T) {
	model := newTestPickerWithWorkspaces()
	model.input.SetValue("myproject")
	model.onInputChanged()
	// Ensure cursor is on top action.
	model.tree.Cursor = 0

	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := result.(PickerModel)

	if m.Result.Action != "workspace-create" {
		t.Errorf("expected action 'workspace-create', got %q", m.Result.Action)
	}
	if m.Result.Workspace != "myproject" {
		t.Errorf("expected workspace 'myproject', got %q", m.Result.Workspace)
	}
}

func TestPickerEnterOnWorkspaceWithSessionsAttaches(t *testing.T) {
	model := newTestPickerWithWorkspaces()

	// Move to "bridge" workspace.
	idx := findRowIndex(model, func(it outline.Row) bool {
		return it.Kind == outline.RowWorkspaceHeader && rowWorkspace(it) != nil && rowWorkspace(it).Name == "bridge"
	})
	if idx < 0 {
		t.Fatal("bridge workspace not found")
	}
	model.tree.Cursor = idx
	model.buildOutline()

	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := result.(PickerModel)

	if m.Result.Action != "attach" {
		t.Errorf("expected action 'attach', got %q", m.Result.Action)
	}
	if m.Result.Session == "" {
		t.Error("expected session name in result")
	}
	if m.Result.Workspace != "bridge" {
		t.Errorf("expected workspace 'bridge', got %q", m.Result.Workspace)
	}
}

func TestPickerEnterOnEmptyWorkspaceCreatesMain(t *testing.T) {
	// Set up a workspace with NO live sessions.
	mm := tmux.NewMockRunner()
	styles := DefaultStyles()
	model := NewPickerModel(mm, styles)
	model.width = 120
	model.height = 40

	workspaces := []workspace.Workspace{
		{Name: "empty-ws", Sessions: []string{}},
	}
	model.workspaces = BuildWorkspaceViewModels(workspaces, nil)
	model.state.showEmpty = true // ensure visible
	model.filteredWorkspaces = model.workspaces
	model.buildOutline()

	// Find the workspace item.
	idx := findRowIndex(model, func(it outline.Row) bool {
		return it.Kind == outline.RowWorkspaceHeader && rowWorkspace(it) != nil && rowWorkspace(it).Name == "empty-ws"
	})
	if idx < 0 {
		t.Fatal("empty-ws not in items list")
	}
	model.tree.Cursor = idx

	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := result.(PickerModel)

	if m.Result.Action != "new" {
		t.Errorf("expected action 'new' (create main), got %q", m.Result.Action)
	}
	if m.Result.Name != "main" {
		t.Errorf("expected session name 'main', got %q", m.Result.Name)
	}
	if m.Result.Workspace != "empty-ws" {
		t.Errorf("expected workspace 'empty-ws', got %q", m.Result.Workspace)
	}
}

func TestPickerEnterOnSessionAttaches(t *testing.T) {
	model := newTestPickerWithWorkspaces()

	// Find a session item.
	idx := findRowIndex(model, func(it outline.Row) bool {
		return it.Kind == outline.RowSession
	})
	if idx < 0 {
		// Sessions only appear when parent workspace is expanded (cursor on it).
		// Move cursor to first workspace first.
		model.tree.Cursor = 1
		model.buildOutline()
		idx = findRowIndex(model, func(it outline.Row) bool {
			return it.Kind == outline.RowSession
		})
	}
	if idx < 0 {
		t.Fatal("no session items found even after expanding")
	}
	model.tree.Cursor = idx

	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := result.(PickerModel)

	if m.Result.Action != "attach" {
		t.Errorf("expected action 'attach', got %q", m.Result.Action)
	}
	if m.Result.Session == "" {
		t.Error("expected session name in result")
	}
}

// ── Search ──

func TestPickerSearchFiltersWorkspaces(t *testing.T) {
	model := newTestPickerWithWorkspaces()

	model.input.SetValue("bri")
	model.onInputChanged()

	if len(model.filteredWorkspaces) != 1 {
		t.Errorf("expected 1 match for 'bri', got %d", len(model.filteredWorkspaces))
	}
	if model.filteredWorkspaces[0].Name != "bridge" {
		t.Errorf("expected 'bridge', got %q", model.filteredWorkspaces[0].Name)
	}
}

func TestPickerSearchExpandsMatched(t *testing.T) {
	model := newTestPickerWithWorkspaces()
	model.input.SetValue("bri")
	model.onInputChanged()

	// Search-expand: the matched workspace's sessions should be in items.
	found := false
	for _, item := range model.tree.Rows {
		if item.Kind == outline.RowSession && rowSession(item) != nil && rowSession(item).Name == "dev" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'dev' session in items list (search expansion)")
	}
}

func TestPickerSearchPopulatesMatchIndexes(t *testing.T) {
	model := newTestPickerWithWorkspaces()
	model.input.SetValue("bri")
	model.onInputChanged()

	found := false
	for _, ws := range model.filteredWorkspaces {
		if ws.Name == "bridge" {
			found = true
			if len(ws.MatchedIndexes) == 0 {
				t.Error("expected MatchedIndexes populated on match")
			}
			break
		}
	}
	if !found {
		t.Fatal("expected 'bridge' in filtered workspaces")
	}
}

// Typing an exact workspace name moves cursor to it.
func TestPickerExactMatchMovesCursor(t *testing.T) {
	model := newTestPickerWithWorkspaces()

	for _, r := range "bridge" {
		result, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		model = result.(PickerModel)
	}

	if model.tree.Cursor == 0 {
		t.Errorf("expected cursor off top action on exact match, got 0")
	}
	item := model.tree.CurrentSelectable()
	if item == nil || item.Kind != outline.RowWorkspaceHeader || rowWorkspace(*item) == nil || rowWorkspace(*item).Name != "bridge" {
		t.Errorf("expected cursor on 'bridge' workspace, got %+v", item)
	}
}

// Typing a partial match stays on top action (doesn't force selection).
func TestPickerPartialMatchStaysOnTop(t *testing.T) {
	model := newTestPickerWithWorkspaces()

	for _, r := range "br" {
		result, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		model = result.(PickerModel)
	}

	if model.tree.Cursor != 0 {
		t.Errorf("expected cursor to stay at 0 for partial match, got %d", model.tree.Cursor)
	}
}

// Backspacing out of an exact match resets the cursor.
func TestPickerBackspaceAfterExactMatchResets(t *testing.T) {
	model := newTestPickerWithWorkspaces()

	model.input.SetValue("bridge")
	model.onInputChanged()
	if model.tree.Cursor == 0 {
		t.Fatal("setup: expected cursor to move to exact match")
	}

	model.input.SetValue("bridg")
	model.onInputChanged()
	if model.tree.Cursor != 0 {
		t.Errorf("expected cursor reset to 0 after losing exact match, got %d", model.tree.Cursor)
	}
}

// Space separator filters sessions within the matched workspace.
func TestPickerSpaceSeparatorFiltersSessions(t *testing.T) {
	model := newTestPickerWithWorkspaces()

	model.input.SetValue("bridge dev")
	model.onInputChanged()

	// Look for session "dev" in items.
	found := false
	for _, item := range model.tree.Rows {
		if item.Kind == outline.RowSession && rowSession(item) != nil && rowSession(item).Name == "dev" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'dev' session visible with 'bridge dev' filter")
	}
}

// ── Esc ──

func TestPickerEscClearsQuery(t *testing.T) {
	model := newTestPickerWithWorkspaces()
	model.input.SetValue("bri")
	model.onInputChanged()

	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m := result.(PickerModel)

	if m.Quitting {
		t.Error("should not quit when clearing query")
	}
	if m.input.Value() != "" {
		t.Errorf("expected input cleared, got %q", m.input.Value())
	}
}

func TestPickerEscEmptyQuits(t *testing.T) {
	model := newTestPickerWithWorkspaces()

	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m := result.(PickerModel)

	if !m.Quitting {
		t.Error("expected quitting when Esc with empty input")
	}
}

// ── Ctrl+X / delete ──

func TestPickerCtrlXOnWorkspaceEntersDelete(t *testing.T) {
	model := newTestPickerWithWorkspaces()

	idx := findRowIndex(model, func(it outline.Row) bool {
		return it.Kind == outline.RowWorkspaceHeader && rowWorkspace(it) != nil && !rowWorkspace(it).IsPseudo
	})
	if idx < 0 {
		t.Fatal("no real workspace found")
	}
	model.tree.Cursor = idx

	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	m := result.(PickerModel)

	if m.mode != modeConfirmDelete {
		t.Error("expected confirm delete mode on workspace")
	}
}

func TestPickerCtrlXOnTopActionDoesNothing(t *testing.T) {
	model := newTestPickerWithWorkspaces()

	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	m := result.(PickerModel)

	if m.mode == modeConfirmDelete {
		t.Error("should not enter delete mode on top action")
	}
}

// TestPickerDeleteAttachedWorkspaceTwoStep covers the H4 correctness fix:
// deleting a workspace with a live attached client must require a second
// y/N confirm before the mutation runs, so the user doesn't silently kill
// their only tmux client from the outside-tmux picker.
func TestPickerDeleteAttachedWorkspaceTwoStep(t *testing.T) {
	// Build a workspace with one attached session.
	workspaces := []workspace.Workspace{
		{Name: "webapp", Sessions: []string{"main"}},
	}
	sessions := []session.SessionInfo{
		{Name: "main", Activity: time.Now(), Attached: true},
	}

	mm := tmux.NewMockRunner()
	styles := DefaultStyles()
	model := NewPickerModel(mm, styles)
	model.width = 120
	model.height = 40
	model.workspaces = BuildWorkspaceViewModels(workspaces, sessions)

	// Simulate initial load so filteredWorkspaces + tree are populated.
	result, _ := model.Update(workspacesLoadedMsg{workspaces: model.workspaces})
	model = result.(PickerModel)

	// Land the cursor on the webapp workspace row.
	idx := findRowIndex(model, func(it outline.Row) bool {
		ws := rowWorkspace(it)
		return ws != nil && ws.Name == "webapp"
	})
	if idx < 0 {
		t.Fatal("webapp workspace row not found")
	}
	model.tree.Cursor = idx

	// Press ctrl+x → enter first-step confirm.
	result, _ = model.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	model = result.(PickerModel)
	if model.mode != modeConfirmDelete {
		t.Fatalf("step 1: expected modeConfirmDelete, got %v", model.mode)
	}
	if model.confirm == nil || !model.confirm.attached {
		t.Fatalf("step 1: confirm snapshot missing or not flagged as attached")
	}

	// Press y → should NOT kill yet; should advance to second step.
	result, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	model = result.(PickerModel)
	if model.mode != modeConfirmDeleteAttached {
		t.Fatalf("step 2: expected modeConfirmDeleteAttached after first y, got %v", model.mode)
	}

	// Inspect the mock — no kill should have landed yet.
	for _, call := range mm.Calls {
		if call.Method == "KillSession" {
			t.Fatalf("step 2: a kill ran before the second confirm: %+v", call)
		}
	}

	// Press y again → kill runs, mode resets.
	result, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	model = result.(PickerModel)
	if model.mode != modeNormal {
		t.Errorf("after confirm: expected modeNormal, got %v", model.mode)
	}
	if model.confirm != nil {
		t.Errorf("after confirm: expected confirm snapshot cleared, got %+v", model.confirm)
	}

	// A KillSession call should now exist for "main".
	killed := false
	for _, call := range mm.Calls {
		if call.Method == "KillSession" && len(call.Args) > 0 && call.Args[0] == "main" {
			killed = true
			break
		}
	}
	if !killed {
		t.Errorf("after confirm: expected KillSession main, got calls %+v", mm.Calls)
	}
}

// TestPickerDeleteAttachedWorkspaceCancelledOnSecondStep ensures we can back
// out of the second-step confirmation without killing anything.
func TestPickerDeleteAttachedWorkspaceCancelledOnSecondStep(t *testing.T) {
	workspaces := []workspace.Workspace{
		{Name: "webapp", Sessions: []string{"main"}},
	}
	sessions := []session.SessionInfo{
		{Name: "main", Activity: time.Now(), Attached: true},
	}

	mm := tmux.NewMockRunner()
	styles := DefaultStyles()
	model := NewPickerModel(mm, styles)
	model.width = 120
	model.height = 40
	model.workspaces = BuildWorkspaceViewModels(workspaces, sessions)

	result, _ := model.Update(workspacesLoadedMsg{workspaces: model.workspaces})
	model = result.(PickerModel)

	idx := findRowIndex(model, func(it outline.Row) bool {
		ws := rowWorkspace(it)
		return ws != nil && ws.Name == "webapp"
	})
	model.tree.Cursor = idx

	// step 1 confirm
	result, _ = model.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	model = result.(PickerModel)
	result, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	model = result.(PickerModel)
	if model.mode != modeConfirmDeleteAttached {
		t.Fatalf("expected second-step confirm, got %v", model.mode)
	}

	// Cancel with 'n' — any non-y should back out.
	result, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	model = result.(PickerModel)
	if model.mode != modeNormal {
		t.Errorf("expected modeNormal after cancel, got %v", model.mode)
	}
	if model.confirm != nil {
		t.Errorf("expected confirm cleared after cancel")
	}

	// No kill should have landed.
	for _, call := range mm.Calls {
		if call.Method == "KillSession" {
			t.Errorf("kill ran after cancel: %+v", call)
		}
	}
}

// TestPickerDeleteUnattachedWorkspaceSingleStep sanity-checks the normal
// (non-attached) flow — one y should be enough.
func TestPickerDeleteUnattachedWorkspaceSingleStep(t *testing.T) {
	workspaces := []workspace.Workspace{
		{Name: "webapp", Sessions: []string{"main"}},
	}
	sessions := []session.SessionInfo{
		{Name: "main", Activity: time.Now(), Attached: false},
	}

	mm := tmux.NewMockRunner()
	styles := DefaultStyles()
	model := NewPickerModel(mm, styles)
	model.width = 120
	model.height = 40
	model.workspaces = BuildWorkspaceViewModels(workspaces, sessions)

	result, _ := model.Update(workspacesLoadedMsg{workspaces: model.workspaces})
	model = result.(PickerModel)

	idx := findRowIndex(model, func(it outline.Row) bool {
		ws := rowWorkspace(it)
		return ws != nil && ws.Name == "webapp"
	})
	model.tree.Cursor = idx

	result, _ = model.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	model = result.(PickerModel)
	result, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	model = result.(PickerModel)

	if model.mode != modeNormal {
		t.Errorf("unattached path should reach modeNormal after single y, got %v", model.mode)
	}

	killed := false
	for _, call := range mm.Calls {
		if call.Method == "KillSession" && len(call.Args) > 0 && call.Args[0] == "main" {
			killed = true
			break
		}
	}
	if !killed {
		t.Errorf("unattached path did not kill main; calls=%+v", mm.Calls)
	}
}

// ── Ctrl+H toggle ──

func TestPickerToggleEmptyVisibility(t *testing.T) {
	workspaces := []workspace.Workspace{
		{Name: "withSess", Sessions: []string{"live"}},
		{Name: "empty1", Sessions: nil},
		{Name: "empty2", Sessions: nil},
	}
	sessions := []session.SessionInfo{
		{Name: "live", Activity: time.Now(), IsTmp: false},
	}

	mm := tmux.NewMockRunner()
	styles := DefaultStyles()
	model := NewPickerModel(mm, styles)
	model.width = 120
	model.height = 40
	model.workspaces = BuildWorkspaceViewModels(workspaces, sessions)

	// Simulate initial load.
	result, _ := model.Update(workspacesLoadedMsg{workspaces: model.workspaces})
	mm2 := result.(PickerModel)

	if len(mm2.filteredWorkspaces) != 1 {
		t.Errorf("expected 1 visible workspace (empty hidden), got %d", len(mm2.filteredWorkspaces))
	}

	result, _ = mm2.Update(tea.KeyMsg{Type: tea.KeyCtrlH})
	mm2 = result.(PickerModel)

	if len(mm2.filteredWorkspaces) != 3 {
		t.Errorf("expected 3 visible workspaces after toggle, got %d", len(mm2.filteredWorkspaces))
	}
}

// Search always shows empty workspaces even when hidden by default.
func TestPickerSearchShowsEmptyWorkspaces(t *testing.T) {
	workspaces := []workspace.Workspace{
		{Name: "withSess", Sessions: []string{"live"}},
		{Name: "empty-ws", Sessions: nil},
	}
	sessions := []session.SessionInfo{
		{Name: "live", IsTmp: false},
	}

	mm := tmux.NewMockRunner()
	model := NewPickerModel(mm, DefaultStyles())
	model.workspaces = BuildWorkspaceViewModels(workspaces, sessions)

	model.input.SetValue("empty")
	model.onInputChanged()

	found := false
	for _, ws := range model.filteredWorkspaces {
		if ws.Name == "empty-ws" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'empty-ws' visible when searching")
	}
}

// ── Tab autocomplete + ghost ──

func TestPickerTabAutocomplete(t *testing.T) {
	model := newTestPickerWithWorkspaces()

	// Simulate load to set suggestions.
	result, _ := model.Update(workspacesLoadedMsg{workspaces: model.workspaces})
	m := result.(PickerModel)

	for _, r := range "bri" {
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = result.(PickerModel)
	}

	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = result.(PickerModel)

	if m.input.Value() != "bridge" {
		t.Errorf("expected autocomplete to 'bridge', got %q", m.input.Value())
	}
}

func TestPickerGhostCompletion(t *testing.T) {
	model := newTestPickerWithWorkspaces()
	result, _ := model.Update(workspacesLoadedMsg{workspaces: model.workspaces})
	m := result.(PickerModel)

	for _, r := range "bri" {
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = result.(PickerModel)
	}

	rendered := m.input.View()
	if !strings.Contains(rendered, "dge") {
		t.Errorf("expected ghost 'dge' in rendered input, got: %q", rendered)
	}
}

// ── Ctrl+C ──

func TestPickerCtrlCQuits(t *testing.T) {
	model := newTestPickerWithWorkspaces()

	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m := result.(PickerModel)

	if !m.Quitting {
		t.Error("expected quitting after Ctrl+C")
	}
}

// ── Empty state ──

func TestPickerEmptyShowsLogo(t *testing.T) {
	model := newEmptyPicker()
	view := model.View()

	if !strings.Contains(view, "░█████████") {
		t.Error("expected big logo when no workspaces")
	}
}

func TestPickerEmptyShowsTopAction(t *testing.T) {
	model := newEmptyPicker()
	view := model.View()

	if !strings.Contains(view, "+ new tmp session") {
		t.Error("expected '+ new tmp session' top action in empty state")
	}
}

// ── Ghost command ──

func TestPickerGhostCmdTopActionEmpty(t *testing.T) {
	model := newTestPickerWithWorkspaces()
	cmd := model.ghostCmd()
	if !strings.Contains(cmd, "zmux new") || !strings.Contains(cmd, "tmp") {
		t.Errorf("expected 'zmux new ... tmp' ghost, got %q", cmd)
	}
}

func TestPickerGhostCmdTopActionTyped(t *testing.T) {
	model := newTestPickerWithWorkspaces()
	model.input.SetValue("myapp")
	model.onInputChanged()
	model.tree.Cursor = 0
	cmd := model.ghostCmd()
	if cmd != "zmux new myapp" {
		t.Errorf("expected 'zmux new myapp', got %q", cmd)
	}
}

func TestPickerGhostCmdOnWorkspace(t *testing.T) {
	model := newTestPickerWithWorkspaces()
	idx := findRowIndex(model, func(it outline.Row) bool {
		return it.Kind == outline.RowWorkspaceHeader && rowWorkspace(it) != nil && rowWorkspace(it).Name == "bridge"
	})
	if idx < 0 {
		t.Fatal("bridge not found")
	}
	model.tree.Cursor = idx
	model.buildOutline()

	cmd := model.ghostCmd()
	if !strings.HasPrefix(cmd, "zmux bridge") {
		t.Errorf("expected ghost to start with 'zmux bridge', got %q", cmd)
	}
}

// ── parseQuery ──

func TestParseQuery(t *testing.T) {
	tests := []struct {
		raw     string
		wantWS  string
		wantSes string
	}{
		{"", "", ""},
		{"myapp", "myapp", ""},
		{"myapp ", "myapp", ""},
		{"myapp auth", "myapp", "auth"},
		{"myapp feat-auth", "myapp", "feat-auth"},
		{" auth", "", "auth"},
	}
	for _, tt := range tests {
		ws, ses := parseQuery(tt.raw)
		if ws != tt.wantWS || ses != tt.wantSes {
			t.Errorf("parseQuery(%q) = (%q, %q), want (%q, %q)", tt.raw, ws, ses, tt.wantWS, tt.wantSes)
		}
	}
}

// ── BuildWorkspaceViewModels ──

func TestBuildWorkspaceViewModels(t *testing.T) {
	now := time.Now()
	workspaces := []workspace.Workspace{
		{Name: "app", Sessions: []string{"main", "test"}, RootDir: "/app"},
		{Name: "empty-ws", Sessions: []string{"dead"}, RootDir: "/empty"},
	}
	sessions := []session.SessionInfo{
		{Name: "main", Windows: 3, Attached: true, Activity: now, IsTmp: false},
		{Name: "test", Windows: 1, Attached: false, Activity: now.Add(-1 * time.Hour), IsTmp: false},
		{Name: "tmp-1", Windows: 1, Attached: false, Activity: now.Add(-5 * time.Minute), IsTmp: true},
	}

	models := BuildWorkspaceViewModels(workspaces, sessions)

	if len(models) < 2 {
		t.Fatalf("expected at least 2 view models, got %d", len(models))
	}
	if models[0].Name != "app" {
		t.Errorf("expected first workspace 'app' (MRU), got %q", models[0].Name)
	}
	if models[0].LiveSessionCount != 2 {
		t.Errorf("expected 2 live sessions in 'app', got %d", models[0].LiveSessionCount)
	}

	var tempFound bool
	for _, m := range models {
		if m.Name == "temporary" && m.IsPseudo {
			tempFound = true
			if m.LiveSessionCount != 1 {
				t.Errorf("expected 1 tmp session in temporary, got %d", m.LiveSessionCount)
			}
		}
	}
	if !tempFound {
		t.Error("expected 'temporary' pseudo-workspace")
	}
}
