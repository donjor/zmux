package tabs

import (
	"os"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui/outline"
)

// currentPaneID returns the tmux pane id of the running shell, or "" if
// not inside tmux. Used by buildRows to mark the caller's pane.
func currentPaneID() string { return os.Getenv("TMUX_PANE") }

// buildRows constructs the expanded-all-tabs session-tab layout:
//
//	workspace banner        (depth 0, Selectable)
//	(separator)             (RowPlaceholder, !Selectable)
//	current session         (depth 1, Current, Expanded)
//	  window                (depth 2, Data: *windowDetail)
//	(separator)
//	sibling session         (depth 1, Data: *session.SessionInfo)
//	  window                (depth 2, Data: *tmux.Window)
//
// Every session is expanded with its windows. The current session's
// windows carry rich windowDetail (CPU/mem/uptime); sibling windows
// carry plain tmux.Window since we don't fetch process stats for them.
//
// Rendering for each row kind lives in current_tree_render.go.
func (t *CurrentTab) buildRows() []outline.Row {
	paneCount := 0
	for _, w := range t.windows {
		paneCount += len(w.Panes)
	}
	est := 4 + len(t.windows) + paneCount + 3*len(t.siblings)
	rows := make([]outline.Row, 0, est)

	// ── Workspace banner ──
	wsID := outline.WorkspaceID(t.wsName)
	rows = append(rows, outline.Row{
		ID:         wsID,
		Kind:       outline.RowWorkspaceHeader,
		Depth:      0,
		Label:      t.wsName,
		Selectable: true,
		Attached:   t.attached > 0,
		Expanded:   true,
		Data:       t.wsModel,
	})
	rows = append(rows, separatorRow("sep:ws"))

	// ── Current session + its windows ──
	currID := outline.SessionID(t.sessionName)
	rows = append(rows, outline.Row{
		ID:         currID,
		Kind:       outline.RowSession,
		Depth:      1,
		ParentID:   wsID,
		Label:      t.sessionName,
		Selectable: true,
		Current:    true,
		Attached:   t.attached > 0,
		Expanded:   true,
	})
	if len(t.windows) == 0 {
		rows = append(rows, outline.Row{
			ID:       "placeholder:nowindows",
			Kind:     outline.RowPlaceholder,
			Depth:    2,
			ParentID: currID,
			Label:    "(no windows)",
		})
	}
	for i := range t.windows {
		w := t.windows[i]
		winID := outline.WindowID(t.sessionName, w.Index)
		rows = append(rows, outline.Row{
			ID:         winID,
			Kind:       outline.RowWindow,
			Depth:      2,
			ParentID:   currID,
			Label:      w.Name,
			Selectable: t.windowSelectable(currID),
			Attached:   w.Active,
			Data:       &t.windows[i],
		})
		for j := range t.windows[i].Panes {
			p := t.windows[i].Panes[j]
			rows = append(rows, outline.Row{
				ID:         outline.PaneID(t.sessionName, p.ID),
				Kind:       outline.RowPane,
				Depth:      3,
				ParentID:   winID,
				Label:      p.ID,
				Selectable: t.windowSelectable(currID),
				Current:    p.ID != "" && p.ID == currentPaneID(),
				Attached:   p.Active,
				Data:       &t.windows[i].Panes[j],
			})
		}
	}

	// ── Sibling sessions, each expanded with its windows ──
	for i := range t.siblings {
		s := t.siblings[i]
		sessID := outline.SessionID(s.Name)

		rows = append(rows, separatorRow("sep:"+s.Name))

		rows = append(rows, outline.Row{
			ID:         sessID,
			Kind:       outline.RowSession,
			Depth:      1,
			ParentID:   wsID,
			Label:      sessionInfoLabel(&s),
			Selectable: true,
			Attached:   s.Attached,
			Expanded:   true,
			Data:       &t.siblings[i],
		})

		wins := t.siblingWindows[s.Name]
		if len(wins) == 0 {
			rows = append(rows, outline.Row{
				ID:       "placeholder:" + s.Name + ":nowindows",
				Kind:     outline.RowPlaceholder,
				Depth:    2,
				ParentID: sessID,
				Label:    "(no windows)",
			})
			continue
		}
		for j := range wins {
			w := wins[j]
			rows = append(rows, outline.Row{
				ID:         outline.WindowID(s.Name, w.Index),
				Kind:       outline.RowWindow,
				Depth:      2,
				ParentID:   sessID,
				Label:      w.Name,
				Selectable: t.windowSelectable(sessID),
				Attached:   w.Active,
				Data:       &wins[j],
			})
		}
	}

	return rows
}

// windowSelectable reports whether window rows belonging to the given
// session should be selectable under the current nav level. In session
// level, windows are never selectable (the cursor hops session-to-
// session). In window level, only the expanded session's windows are
// selectable.
func (t *CurrentTab) windowSelectable(sessionRowID string) bool {
	if t.navLevel != navLevelWindow {
		return false
	}
	return sessionRowID == t.expandedSessionID
}

// separatorRow is a blank non-selectable row used between sessions.
func separatorRow(id string) outline.Row {
	return outline.Row{
		ID:         id,
		Kind:       outline.RowPlaceholder,
		Depth:      0,
		Label:      "",
		Selectable: false,
	}
}

// siblingSessionForWindow returns the sibling session owning a window row,
// or nil if the window belongs to the current session.
func (t *CurrentTab) siblingSessionForWindow(row *outline.Row) *session.SessionInfo {
	for i := range t.siblings {
		if row.ParentID == outline.SessionID(t.siblings[i].Name) {
			return &t.siblings[i]
		}
	}
	return nil
}

// windowSpec is the unified addressing for a window row. It resolves
// the owning session (current or sibling) and the window's identity,
// hiding the windowDetail-vs-tmux.Window payload split from action
// handlers.
type windowSpec struct {
	Session string
	Index   int
	Name    string
	Dir     string
}

// windowSpecFromRow extracts a windowSpec from a window row. Returns
// ok=false if the row isn't a window row or the payload is missing.
func (t *CurrentTab) windowSpecFromRow(row *outline.Row) (windowSpec, bool) {
	if row == nil || row.Kind != outline.RowWindow {
		return windowSpec{}, false
	}
	if w, ok := outline.RowData[windowDetail](row); ok && w != nil {
		return windowSpec{Session: t.sessionName, Index: w.Index, Name: w.Name, Dir: w.Dir}, true
	}
	if w, ok := outline.RowData[tmux.Window](row); ok && w != nil {
		if s := t.siblingSessionForWindow(row); s != nil {
			return windowSpec{Session: s.Name, Index: w.Index, Name: w.Name, Dir: w.Dir}, true
		}
	}
	return windowSpec{}, false
}
