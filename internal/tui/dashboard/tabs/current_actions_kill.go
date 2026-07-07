package tabs

import (
	tea "charm.land/bubbletea/v2"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui/dashboard"
	"github.com/donjor/zmux/internal/tui/outline"
	"github.com/donjor/zmux/internal/tui/workspaceview"
)

// actionKill dispatches kill for workspace/session/window/pane rows.
// Workspace kill on an attached workspace routes through a second
// confirmation step (currentModeConfirmKillAttached).
func (t *CurrentTab) actionKill(row *outline.Row) (dashboard.Tab, tea.Cmd) {
	switch row.Kind {
	case outline.RowWorkspaceHeader:
		if t.wsName == "" {
			return t, nil
		}
		attached := t.attached > 0
		if ws, _ := outline.RowData[workspaceview.WorkspaceViewModel](row); ws != nil {
			attached = ws.HasAttached
		}
		t.confirm = &confirmState{kind: "workspace", name: t.wsName, attached: attached}
		t.mode = currentModeConfirmKill
		return t, nil

	case outline.RowSession:
		name := t.sessionName
		if !row.Current {
			if s, _ := outline.RowData[session.SessionInfo](row); s != nil {
				name = s.Name
			}
		}
		t.confirm = &confirmState{kind: "session", name: name}
		t.mode = currentModeConfirmKill
		return t, nil

	case outline.RowWindow:
		spec, ok := t.windowSpecFromRow(row)
		if !ok {
			return t, nil
		}
		t.confirm = &confirmState{
			kind:        "window",
			name:        spec.Name,
			sessionName: spec.Session,
			windowIndex: spec.Index,
		}
		t.mode = currentModeConfirmKill
		return t, nil

	case outline.RowPane:
		if p, ok := outline.RowData[tmux.Pane](row); ok && p != nil && p.ID != "" {
			name := p.Title
			if name == "" {
				name = p.ID
			}
			t.confirm = &confirmState{kind: "pane", name: name, paneID: p.ID}
			t.mode = currentModeConfirmKill
		}
		return t, nil
	}
	return t, nil
}

func (t *CurrentTab) handleConfirmKillKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	if t.confirm == nil {
		t.exitMode()
		return t, nil
	}
	if msg.String() != "y" && msg.String() != "Y" {
		t.exitMode()
		return t, nil
	}

	if confirmKillEscalate(t.confirm, t.mode == currentModeConfirmKillAttached) {
		t.mode = currentModeConfirmKillAttached
		return t, nil
	}

	var cmd tea.Cmd
	switch t.confirm.kind {
	case "workspace":
		cmd = t.killWorkspace(t.confirm.name)
	case "session":
		cmd = t.killSession(t.confirm.name)
	case "window":
		cmd = t.killWindow(t.confirm.sessionName, t.confirm.windowIndex)
	case "pane":
		cmd = t.killPane(t.confirm.paneID)
	}
	t.exitMode()
	return t, cmd
}
