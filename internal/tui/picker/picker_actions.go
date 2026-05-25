package picker

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/source"
	"github.com/donjor/zmux/internal/tui/outline"
	"github.com/donjor/zmux/internal/tui/workspaceview"
)

// handleEnter dispatches based on the row under the cursor.
func (m PickerModel) handleEnter() (tea.Model, tea.Cmd) {
	row := m.tree.CurrentSelectable()
	if row == nil {
		return m, nil
	}

	switch row.Kind {
	case outline.RowTopAction:
		return m.handleTopActionEnter()
	case outline.RowWorkspaceHeader:
		ws, _ := outline.RowData[workspaceview.WorkspaceViewModel](row)
		return m.handleWorkspaceEnter(ws)
	case outline.RowSession:
		s, _ := outline.RowData[session.SessionInfo](row)
		return m.handleSessionEnter(s)
	case outline.RowExternalGroup:
		// Toggle expansion and rebuild.
		m.tree.ToggleExpand(row.ID)
		m.buildOutline()
		return m, nil
	case outline.RowExternalEntry:
		return m.handleExternalEntryEnter(row)
	}
	return m, nil
}

// handleExternalEntryEnter converts the row into an attach/connect result.
func (m PickerModel) handleExternalEntryEnter(row *outline.Row) (tea.Model, tea.Cmd) {
	entry, _ := outline.RowData[source.CatalogEntry](row)
	if entry == nil {
		return m, nil
	}
	src := externalEntrySource(m.catalog, row)
	if src == nil {
		return m, nil
	}
	srcCopy := *src
	if src.Kind == source.SourceOvermind {
		m.Result = PickerResult{
			Action:         "overmind-connect",
			Session:        entry.Session,
			ExternalSource: &srcCopy,
		}
	} else {
		m.Result = PickerResult{
			Action:         "external-attach",
			Session:        entry.Session,
			ExternalSource: &srcCopy,
		}
	}
	m.Quitting = true
	return m, tea.Quit
}

func (m PickerModel) handleTopActionEnter() (tea.Model, tea.Cmd) {
	wsName := strings.TrimSpace(m.state.workspaceQuery)
	if wsName == "" {
		// Empty input → create tmp session (no workspace).
		m.Result = PickerResult{Action: "new"}
		m.Quitting = true
		return m, tea.Quit
	}
	// Typed workspace name → create workspace. If a session name was
	// also typed (e.g. "myapp dev"), pass it through so root.go creates
	// that session instead of the default "main".
	sessName := strings.TrimSpace(m.state.sessionQuery)
	m.Result = PickerResult{
		Action:    "workspace-create",
		Workspace: wsName,
		Name:      sessName, // "" → root.go defaults to "main"
	}
	m.Quitting = true
	return m, tea.Quit
}

func (m PickerModel) handleWorkspaceEnter(ws *workspaceview.WorkspaceViewModel) (tea.Model, tea.Cmd) {
	if ws == nil {
		return m, nil
	}

	// Session query present → user typed "workspace session" in the
	// search bar. This means "create a new session named <session> in
	// this workspace", equivalent to `zmux new <ws> <session>`.
	if m.state.sessionQuery != "" {
		m.Result = PickerResult{
			Action:    "new",
			Name:      m.state.sessionQuery,
			Workspace: ws.Name,
		}
		m.Quitting = true
		return m, tea.Quit
	}

	// No live sessions → create default session (named after workspace).
	if len(ws.LiveSessions) == 0 {
		m.Result = PickerResult{
			Action:    "new",
			Name:      ws.Name,
			Workspace: ws.Name,
		}
		m.Quitting = true
		return m, tea.Quit
	}

	// Has sessions, no session query → drill into the workspace and require the
	// user to pick an explicit session row. This avoids surprising auto-attach to
	// last-active when a workspace contains multiple sessions.
	return m.drillIntoWorkspaceSessions(ws.Name), nil
}

func (m PickerModel) drillIntoWorkspaceSessions(workspaceName string) PickerModel {
	wsID := outline.WorkspaceID(workspaceName)
	m.buildOutlineWithFocus(wsID)
	for i := range m.tree.Rows {
		row := &m.tree.Rows[i]
		if row.Kind == outline.RowSession && row.ParentID == wsID {
			m.tree.Cursor = i
			return m
		}
	}
	_ = m.tree.JumpToID(wsID)
	return m
}

func (m PickerModel) handleSessionEnter(s *session.SessionInfo) (tea.Model, tea.Cmd) {
	if s == nil {
		return m, nil
	}
	m.Result = PickerResult{Action: "attach", Session: s.Name}
	m.Quitting = true
	return m, tea.Quit
}

func (m PickerModel) reloadWorkspaces() tea.Cmd {
	if m.wsLoader == nil {
		return nil
	}
	loader := m.wsLoader
	return func() tea.Msg {
		return workspacesLoadedMsg{workspaces: loader()}
	}
}

// applyConfirmedDelete runs the mutation described by m.confirm. Safe to
// call unconditionally — it no-ops on nil. For a workspace target it kills
// every live session it can find (by name — the snapshot taken at ctrl+x
// time may be stale by a few hundred ms, but we re-resolve from the live
// workspace set rather than trusting the snapshot's session list).
func (m PickerModel) applyConfirmedDelete() {
	if m.confirm == nil {
		return
	}
	switch m.confirm.kind {
	case "session":
		_ = session.Kill(m.runner, m.confirm.name)

	case "workspace":
		// Re-resolve the workspace from the live snapshot so we act on
		// whatever sessions are live *now*, not whatever was live when
		// ctrl+x was pressed. (There is no stored-session fallback: the
		// confirm target carries only a name, so if the row is gone from the
		// live set there is nothing to enumerate.)
		for i := range m.workspaces {
			if m.workspaces[i].Name != m.confirm.name {
				continue
			}
			for _, s := range m.workspaces[i].LiveSessions {
				_ = session.Kill(m.runner, s.Name)
			}
			break
		}
		// If the workspace had already disappeared from the view-model between
		// ctrl+x and the confirm, there was nothing live to kill — we still drop
		// the store entry below.
		if m.wsStore != nil {
			_ = m.wsStore.DeleteWorkspace(m.confirm.name)
		}
	}
}
