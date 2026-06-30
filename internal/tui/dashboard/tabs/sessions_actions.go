package tabs

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/donjor/zmux/internal/keys"
	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/source"
	"github.com/donjor/zmux/internal/tui/dashboard"
	"github.com/donjor/zmux/internal/tui/outline"
	"github.com/donjor/zmux/internal/tui/workspaceoutline"
	"github.com/donjor/zmux/internal/tui/workspaceview"
	"github.com/donjor/zmux/internal/workspace"
)

// handleKey dispatches key presses based on the current mode.
func (t *SessionsTab) handleKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	switch t.mode {
	case sessionsModeRename:
		return t.handleRenameKey(msg)
	case sessionsModeCreate:
		return t.handleCreateKey(msg)
	case sessionsModeConfirmKill, sessionsModeConfirmKillAttached:
		return t.handleConfirmKillKey(msg)
	case sessionsModeMove:
		return t.handleMoveKey(msg)
	case sessionsModeSearch:
		return t.handleSearchKey(msg)
	default:
		return t.handleNormalKey(msg)
	}
}

// handleNormalKey routes single-key shortcuts in list mode. The same key
// can mean different things depending on the row kind under the cursor.
func (t *SessionsTab) handleNormalKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	s := msg.String()
	switch {
	case keys.DashNavUp.Matches(s):
		t.tree.MoveUp()
		return t, nil
	case keys.DashNavDown.Matches(s):
		t.tree.MoveDown()
		return t, nil
	case keys.DashNavTop.Matches(s):
		t.tree.JumpTop()
		return t, nil
	case keys.DashNavBottom.Matches(s):
		t.tree.JumpBottom()
		return t, nil
	case keys.DashSearch.Matches(s):
		return t.enterSearchMode()
	case s == "esc":
		// Reaches the tab only when a filter is active (see CapturesEscape);
		// the first Esc clears the filter, a second closes the dashboard.
		if t.searchQuery != "" {
			t.clearSearch()
		}
		return t, nil
	}

	row := t.tree.Current()
	if row == nil {
		return t, nil
	}

	switch {
	case keys.DashSelect.Matches(s):
		return t.handleEnter(row)
	case keys.DashCreate.Matches(s):
		return t.enterCreateSessionMode(row)
	case keys.DashCreateWorkspace.Matches(s):
		return t.enterCreateMode()
	case keys.DashRename.Matches(s):
		return t.handleRenameRequest(row)
	case keys.DashKill.Matches(s):
		return t.handleKillRequest(row)
	case keys.DashMove.Matches(s):
		return t.handleMoveRequest(row)
	}
	return t, nil
}

// ── Search mode ──

// enterSearchMode opens the `/` search input, pre-filled with any active
// filter so the user can refine it.
func (t *SessionsTab) enterSearchMode() (dashboard.Tab, tea.Cmd) {
	t.mode = sessionsModeSearch
	t.searchInput.SetValue(t.searchQuery)
	t.searchInput.CursorEnd()
	t.searchInput.Focus()
	return t, textinput.Blink
}

// handleSearchKey drives the inline search input. Typing live-filters the
// tree; Enter applies the filter and returns to list browsing (the filter
// stays active); Esc cancels the filter entirely. Arrow keys move the cursor
// through the filtered results without leaving the input.
func (t *SessionsTab) handleSearchKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	switch msg.String() {
	case "enter":
		t.searchQuery = strings.TrimSpace(t.searchInput.Value())
		t.finishSearch()
		return t, nil
	case "esc":
		t.searchQuery = ""
		t.searchInput.SetValue("")
		t.finishSearch()
		return t, nil
	case "up":
		t.tree.MoveUp()
		return t, nil
	case "down":
		t.tree.MoveDown()
		return t, nil
	}

	var cmd tea.Cmd
	t.searchInput, cmd = t.searchInput.Update(msg)
	t.searchQuery = strings.TrimSpace(t.searchInput.Value())
	t.tree.SetRows(t.buildRows())
	return t, cmd
}

// finishSearch transitions from search-edit mode back to list mode, flushing
// any data refetch staged while editing and rebuilding the (possibly still
// filtered) tree.
func (t *SessionsTab) finishSearch() {
	t.mode = sessionsModeList
	t.searchInput.Blur()
	if t.pending != nil {
		t.applyData(*t.pending)
		t.pending = nil
		return
	}
	t.tree.SetRows(t.buildRows())
}

// clearSearch drops the active filter and rebuilds the full tree.
func (t *SessionsTab) clearSearch() {
	t.searchQuery = ""
	t.searchInput.SetValue("")
	t.searchInput.Blur()
	t.tree.SetRows(t.buildRows())
}

// handleEnter dispatches Enter based on the row kind.
func (t *SessionsTab) handleEnter(row *outline.Row) (dashboard.Tab, tea.Cmd) {
	switch row.Kind {
	case outline.RowWorkspaceHeader:
		// While a filter is active rows are force-expanded for visibility
		// (rowPolicy's Expanded callback), so toggling would only mutate saved
		// expansion state invisibly — make it a no-op until the filter clears.
		if t.searchQuery == "" {
			t.tree.ToggleExpand(row.ID)
			t.tree.SetRows(t.buildRows())
		}
		return t, nil

	case outline.RowSession:
		s, _ := outline.RowData[session.SessionInfo](row)
		if s == nil {
			return t, nil
		}
		name := s.Name
		return t, func() tea.Msg {
			return dashboard.QuitIntent{Action: "switch", Chosen: name}
		}

	case outline.RowExternalGroup:
		if t.searchQuery == "" {
			t.tree.ToggleExpand(row.ID)
			t.tree.SetRows(t.buildRows())
		}
		return t, nil

	case outline.RowExternalEntry:
		return t.handleExternalEntryEnter(row)
	}
	return t, nil
}

// handleExternalEntryEnter converts an external row into a quit intent.
func (t *SessionsTab) handleExternalEntryEnter(row *outline.Row) (dashboard.Tab, tea.Cmd) {
	g, ok := workspaceoutline.ExternalGroupForRow(t.catalog, row)
	if !ok || g == nil {
		return t, nil
	}
	entry, _ := outline.RowData[source.CatalogEntry](row)
	if entry == nil {
		return t, nil
	}
	srcCopy := g.Source
	if srcCopy.Kind == source.SourceOvermind && srcCopy.Overmind != nil {
		return t, func() tea.Msg {
			return dashboard.QuitIntent{
				Action: "overmind-connect",
				Chosen: entry.Session + "\t" + srcCopy.Overmind.ControlSocket,
			}
		}
	}
	epArgs := strings.Join(srcCopy.Endpoint.Args(), " ")
	return t, func() tea.Msg {
		return dashboard.QuitIntent{
			Action: "external-attach",
			Chosen: entry.Session + "\t" + epArgs,
		}
	}
}

// ── Mode entry helpers ──

// enterCreateMode opens the create-WORKSPACE input (the C key). createWsTarget
// is cleared so handleCreateKey commits a new workspace.
func (t *SessionsTab) enterCreateMode() (dashboard.Tab, tea.Cmd) {
	t.createWsTarget = ""
	t.mode = sessionsModeCreate
	t.createInput.Placeholder = "workspace name..."
	t.createInput.SetValue("")
	t.createInput.Focus()
	return t, textinput.Blink
}

// enterCreateSessionMode opens the create-SESSION input (the c key), targeting
// the workspace at the cursor's scope: the workspace header itself, or the
// parent workspace of the session under the cursor. When no real workspace
// resolves (placeholder / external / pseudo rows, e.g. the empty sessionless
// tab), c escalates to create-WORKSPACE — so c and C both create a workspace
// when there is nothing to nest a session under.
func (t *SessionsTab) enterCreateSessionMode(row *outline.Row) (dashboard.Tab, tea.Cmd) {
	wsName := t.workspaceForRow(row)
	if wsName == "" {
		return t.enterCreateMode()
	}
	t.createWsTarget = wsName
	t.mode = sessionsModeCreate
	t.createInput.Placeholder = "session name..."
	t.createInput.SetValue("")
	t.createInput.Focus()
	return t, textinput.Blink
}

// workspaceForRow resolves the real workspace a row belongs to: a workspace
// header's own name, or the parent workspace of a session row. Returns "" for
// pseudo workspaces and non-workspace rows (external groups/entries).
func (t *SessionsTab) workspaceForRow(row *outline.Row) string {
	switch row.Kind {
	case outline.RowWorkspaceHeader:
		ws, _ := outline.RowData[workspaceview.WorkspaceViewModel](row)
		if ws == nil || ws.IsPseudo {
			return ""
		}
		return ws.Name
	case outline.RowSession:
		parentRow, _ := t.tree.FindByID(row.ParentID)
		if parentRow == nil {
			return ""
		}
		if ws, ok := outline.RowData[workspaceview.WorkspaceViewModel](parentRow); ok && ws != nil && !ws.IsPseudo {
			return ws.Name
		}
	}
	return ""
}

// handleRenameRequest enters rename mode for a workspace or session row.
func (t *SessionsTab) handleRenameRequest(row *outline.Row) (dashboard.Tab, tea.Cmd) {
	switch row.Kind {
	case outline.RowWorkspaceHeader:
		ws, _ := outline.RowData[workspaceview.WorkspaceViewModel](row)
		if ws == nil || ws.IsPseudo {
			return t, nil
		}
		t.rename = &renameState{kind: "workspace", oldName: ws.Name}
		t.mode = sessionsModeRename
		t.renameInput.SetValue(ws.Name)
		t.renameInput.Focus()
		return t, textinput.Blink

	case outline.RowSession:
		s, _ := outline.RowData[session.SessionInfo](row)
		if s == nil {
			return t, nil
		}
		if s.PinnedView {
			return t, func() tea.Msg {
				return dashboard.SetStatusIntent{Text: "Pinned views cannot be renamed; rename the root session", IsError: true}
			}
		}
		t.rename = &renameState{kind: "session", oldName: s.Name}
		t.mode = sessionsModeRename
		t.renameInput.SetValue(s.Name)
		t.renameInput.Focus()
		return t, textinput.Blink

	case outline.RowExternalEntry:
		// 'r' on overmind process = restart.
		g, ok := workspaceoutline.ExternalGroupForRow(t.catalog, row)
		if !ok || g == nil || g.Source.Kind != source.SourceOvermind || g.Source.Overmind == nil {
			return t, nil
		}
		entry, _ := outline.RowData[source.CatalogEntry](row)
		if entry == nil {
			return t, nil
		}
		cs := g.Source.Overmind.ControlSocket
		proc := entry.Session
		return t, func() tea.Msg {
			_ = t.overmind.Restart(cs, proc)
			return dashboard.SetStatusIntent{Text: "Restarted " + proc}
		}
	}
	return t, nil
}

// handleKillRequest enters confirm-kill mode for a workspace, session, or
// dispatches an overmind stop for an external entry.
func (t *SessionsTab) handleKillRequest(row *outline.Row) (dashboard.Tab, tea.Cmd) {
	switch row.Kind {
	case outline.RowWorkspaceHeader:
		ws, _ := outline.RowData[workspaceview.WorkspaceViewModel](row)
		if ws == nil || ws.IsPseudo {
			return t, nil
		}
		t.confirm = &confirmState{kind: "workspace", name: ws.Name, attached: ws.HasAttached}
		t.mode = sessionsModeConfirmKill
		return t, nil

	case outline.RowSession:
		s, _ := outline.RowData[session.SessionInfo](row)
		if s == nil {
			return t, nil
		}
		t.confirm = &confirmState{kind: "session", name: s.Name}
		t.mode = sessionsModeConfirmKill
		return t, nil

	case outline.RowExternalEntry:
		g, ok := workspaceoutline.ExternalGroupForRow(t.catalog, row)
		if !ok || g == nil || g.Source.Kind != source.SourceOvermind || g.Source.Overmind == nil {
			return t, nil
		}
		entry, _ := outline.RowData[source.CatalogEntry](row)
		if entry == nil {
			return t, nil
		}
		cs := g.Source.Overmind.ControlSocket
		proc := entry.Session
		return t, func() tea.Msg {
			_ = t.overmind.Stop(cs, proc)
			return dashboard.SetStatusIntent{Text: "Stopped " + proc}
		}
	}
	return t, nil
}

// handleMoveRequest enters move mode when the cursor is on a session row.
func (t *SessionsTab) handleMoveRequest(row *outline.Row) (dashboard.Tab, tea.Cmd) {
	if row.Kind != outline.RowSession {
		return t, nil
	}
	s, _ := outline.RowData[session.SessionInfo](row)
	if s == nil {
		return t, nil
	}
	if s.PinnedView {
		return t, func() tea.Msg {
			return dashboard.SetStatusIntent{Text: "Pinned views cannot be moved; move the root session", IsError: true}
		}
	}
	parent := row.ParentID
	originName := ""
	if parentRow, _ := t.tree.FindByID(parent); parentRow != nil {
		if ws, ok := outline.RowData[workspaceview.WorkspaceViewModel](parentRow); ok && ws != nil {
			originName = ws.Name
		}
	}

	t.moveSt = &moveState{sessionName: s.Name, originWorkspace: originName}
	t.mode = sessionsModeMove

	// Rebuild rows so badges appear, then snap cursor to nearest workspace
	// header (preferring origin) so up/down navigates workspaces only.
	t.tree.SetRows(t.buildRows())
	t.snapCursorToWorkspace(originName)
	return t, nil
}

// snapCursorToWorkspace places the cursor on the workspace header for the
// given name, falling back to the first workspace header in the tree.
func (t *SessionsTab) snapCursorToWorkspace(prefer string) {
	if prefer != "" {
		if t.tree.JumpToID(outline.WorkspaceID(prefer)) {
			return
		}
	}
	for i := range t.tree.Rows {
		if t.tree.Rows[i].Kind == outline.RowWorkspaceHeader {
			t.tree.Cursor = i
			return
		}
	}
}

// ── Mode key handlers ──

func (t *SessionsTab) handleRenameKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	switch msg.String() {
	case "enter":
		newName := strings.TrimSpace(t.renameInput.Value())
		if newName == "" || t.rename == nil || newName == t.rename.oldName {
			t.exitMode()
			return t, nil
		}
		var cmd tea.Cmd
		var jumpTo string
		switch t.rename.kind {
		case "workspace":
			cmd = t.renameWorkspace(t.rename.oldName, newName)
			jumpTo = outline.WorkspaceID(newName)
		case "session":
			cmd = t.renameSession(t.rename.oldName, newName)
			jumpTo = outline.SessionID(renamedSessionTarget(t.wsStore, t.rename.oldName, newName))
		}
		t.exitMode()
		t.pendingJumpTo = jumpTo
		return t, cmd

	case "esc":
		t.exitMode()
		return t, nil
	}

	var cmd tea.Cmd
	t.renameInput, cmd = t.renameInput.Update(msg)
	return t, cmd
}

func (t *SessionsTab) handleCreateKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	switch msg.String() {
	case "enter":
		name := strings.TrimSpace(t.createInput.Value())
		if name == "" {
			t.exitMode()
			return t, nil
		}
		// createWsTarget discriminates the two create flows: empty = new
		// workspace (C), set = new session in that workspace (c).
		target := t.createWsTarget
		var cmd tea.Cmd
		var jumpTo string
		if target == "" {
			cmd = t.createWorkspace(name)
			jumpTo = outline.WorkspaceID(name)
		} else {
			cmd = t.createSessionInWorkspace(target, name)
			if rec, err := workspace.NewSessionRecord(target, name); err == nil {
				jumpTo = outline.SessionID(rec.TmuxName)
			}
		}
		t.exitMode()
		// Re-set pendingJumpTo AFTER exitMode — see current_actions.go for
		// the full explanation of the stale-pending-data race.
		t.pendingJumpTo = jumpTo
		return t, cmd

	case "esc":
		t.exitMode()
		return t, nil
	}

	var cmd tea.Cmd
	t.createInput, cmd = t.createInput.Update(msg)
	return t, cmd
}

func (t *SessionsTab) handleConfirmKillKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	if t.confirm == nil {
		t.exitMode()
		return t, nil
	}
	if msg.String() != "y" && msg.String() != "Y" {
		t.exitMode()
		return t, nil
	}

	// Workspace with attached sessions: route through the second confirmation.
	if t.confirm.kind == "workspace" && t.confirm.attached && t.mode != sessionsModeConfirmKillAttached {
		t.mode = sessionsModeConfirmKillAttached
		return t, nil
	}

	var cmd tea.Cmd
	switch t.confirm.kind {
	case "workspace":
		cmd = t.killWorkspace(t.confirm.name)
	case "session":
		cmd = t.killSession(t.confirm.name)
	}
	t.exitMode()
	return t, cmd
}

func (t *SessionsTab) handleMoveKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	switch msg.String() {
	case "esc":
		t.exitMode()
		t.tree.SetRows(t.buildRows())
		return t, nil

	case "up", "k":
		t.tree.MoveUp()
		t.snapCursorToWorkspaceFromCurrent(-1)
		return t, nil

	case "down", "j":
		t.tree.MoveDown()
		t.snapCursorToWorkspaceFromCurrent(+1)
		return t, nil

	case "enter":
		row := t.tree.Current()
		if row == nil || row.Kind != outline.RowWorkspaceHeader {
			return t, nil
		}
		ws, _ := outline.RowData[workspaceview.WorkspaceViewModel](row)
		if ws == nil || ws.IsPseudo || t.moveSt == nil {
			return t, nil
		}
		// Same workspace = no-op.
		if ws.Name == t.moveSt.originWorkspace {
			t.exitMode()
			t.tree.SetRows(t.buildRows())
			return t, nil
		}
		cmd := t.moveSessionTo(t.moveSt.sessionName, ws.Name)
		t.exitMode()
		return t, cmd
	}
	return t, nil
}

// snapCursorToWorkspaceFromCurrent walks past non-workspace rows in the
// given direction (+1 or -1) until it lands on a workspace header. Used by
// move-mode navigation so up/down only steps between workspaces.
func (t *SessionsTab) snapCursorToWorkspaceFromCurrent(dir int) {
	for {
		row := t.tree.Current()
		if row == nil {
			return
		}
		if row.Kind == outline.RowWorkspaceHeader {
			return
		}
		prev := t.tree.Cursor
		if dir > 0 {
			t.tree.MoveDown()
		} else {
			t.tree.MoveUp()
		}
		if t.tree.Cursor == prev {
			return // hit an edge
		}
	}
}
