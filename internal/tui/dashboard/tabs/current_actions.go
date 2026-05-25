package tabs

// Per-kind action semantics for the Session & Workspace tab.
//
// | Key   | RowWorkspaceHeader       | RowSession (current)    | RowSession (sibling)     | RowWindow              |
// |-------|--------------------------|-------------------------|--------------------------|------------------------|
// | enter | no-op                    | focus session           | switch to session        | focus window/pane      |
// | r     | rename workspace         | rename session          | rename session           | rename window          |
// | x     | kill workspace (2-step)  | kill session (confirm)  | kill session (confirm)   | kill window/pane       |
// | n     | new session in workspace | new window              | new window in target     | new window             |
// | m     | no-op                    | no-op                   | no-op                    | move window to session |
// | <, >  | no-op                    | no-op                   | no-op                    | reorder window         |
//
// Splits:
//   - current_actions.go        — dispatcher, nav level, enter (focus/switch)
//   - current_actions_edit.go   — rename + create-session/window flows
//   - current_actions_kill.go   — kill + confirm (workspace/session/window/pane)
//   - current_actions_window.go — move (window→session) + reorder

import (
	tea "charm.land/bubbletea/v2"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui/dashboard"
	"github.com/donjor/zmux/internal/tui/outline"
)

// handleKey dispatches key presses based on the current mode.
func (t *CurrentTab) handleKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	switch t.mode {
	case currentModeRename:
		return t.handleRenameKey(msg)
	case currentModeCreate:
		return t.handleCreateKey(msg)
	case currentModeConfirmKill, currentModeConfirmKillAttached:
		return t.handleConfirmKillKey(msg)
	case currentModeMoveWindow:
		return t.handleMoveKey(msg)
	default:
		return t.handleListKey(msg)
	}
}

// handleListKey routes single-key shortcuts in list mode. The same key
// can mean different things depending on the row kind under the cursor.
func (t *CurrentTab) handleListKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		t.tree.MoveUp()
		return t, nil
	case "down", "j":
		t.tree.MoveDown()
		return t, nil
	case "g":
		t.tree.JumpTop()
		return t, nil
	case "G":
		t.tree.JumpBottom()
		return t, nil
	case "right", "l":
		return t.enterWindowLevel()
	case "left", "h":
		return t.exitWindowLevel()
	}

	row := t.tree.Current()
	if row == nil {
		return t, nil
	}

	switch msg.String() {
	case "enter":
		return t.actionEnter(row)
	case "r":
		return t.actionRename(row)
	case "x":
		return t.actionKill(row)
	case "n":
		return t.actionNew(row)
	case "m":
		return t.actionMove(row)
	case "<":
		return t.actionReorder(row, -1)
	case ">":
		return t.actionReorder(row, +1)
	}
	return t, nil
}

// ── nav level transitions (two-level cursor) ──

// enterWindowLevel descends the cursor into the current session row's
// windows. No-op if the cursor isn't on a session row, or if the session
// has no windows. Rebuilds the tree so the newly-selectable windows
// become reachable by j/k.
func (t *CurrentTab) enterWindowLevel() (dashboard.Tab, tea.Cmd) {
	if t.navLevel == navLevelWindow {
		return t, nil // already inside
	}
	row := t.tree.Current()
	if row == nil || row.Kind != outline.RowSession {
		return t, nil
	}

	// Pick the first window in that session (or no-op if none).
	var firstWindowID string
	for i := range t.tree.Rows {
		r := &t.tree.Rows[i]
		if r.Kind == outline.RowWindow && r.ParentID == row.ID {
			firstWindowID = r.ID
			break
		}
	}
	if firstWindowID == "" {
		return t, nil // session has no windows
	}

	t.navLevel = navLevelWindow
	t.expandedSessionID = row.ID
	t.tree.SetRows(t.buildRows())
	t.tree.JumpToID(firstWindowID)
	return t, nil
}

// exitWindowLevel returns the cursor to the owning session row. No-op
// if already at session level.
func (t *CurrentTab) exitWindowLevel() (dashboard.Tab, tea.Cmd) {
	if t.navLevel != navLevelWindow {
		return t, nil
	}
	parentID := t.expandedSessionID
	t.navLevel = navLevelSession
	t.expandedSessionID = ""
	t.tree.SetRows(t.buildRows())
	if parentID != "" {
		t.tree.JumpToID(parentID)
	}
	return t, nil
}

// ── enter ──

func (t *CurrentTab) actionEnter(row *outline.Row) (dashboard.Tab, tea.Cmd) {
	switch row.Kind {
	case outline.RowWorkspaceHeader:
		// No-op: workspace header enter is reserved in this view.
		return t, nil

	case outline.RowSession:
		// Current session → focus (no switch). Sibling → switch.
		if row.Current {
			name := t.sessionName
			return t, func() tea.Msg {
				return dashboard.QuitIntent{Action: "focus", Chosen: name}
			}
		}
		s, _ := outline.RowData[session.SessionInfo](row)
		if s == nil {
			return t, nil
		}
		name := s.Name
		return t, func() tea.Msg {
			return dashboard.QuitIntent{Action: "switch", Chosen: name}
		}

	case outline.RowPane:
		if p, ok := outline.RowData[tmux.Pane](row); ok && p != nil && p.ID != "" {
			runner := t.runner
			paneID := p.ID
			session := t.sessionName
			return t, func() tea.Msg {
				_ = runner.SelectPane(paneID)
				return dashboard.QuitIntent{Action: "focus", Chosen: session}
			}
		}

	case outline.RowWindow:
		// Current-session window: select + focus (no session switch).
		if w, ok := outline.RowData[windowDetail](row); ok && w != nil {
			session := t.sessionName
			runner := t.runner
			idx := w.Index
			return t, func() tea.Msg {
				_ = runner.SelectWindow(session, idx)
				return dashboard.QuitIntent{Action: "focus", Chosen: session}
			}
		}
		// Sibling-session window: switch to that session AND select that
		// window. We resolve the owning session via the row's ParentID.
		if w, ok := outline.RowData[tmux.Window](row); ok && w != nil {
			owner := t.siblingSessionForWindow(row)
			if owner == nil {
				return t, nil
			}
			name := owner.Name
			runner := t.runner
			idx := w.Index
			return t, func() tea.Msg {
				_ = runner.SelectWindow(name, idx)
				return dashboard.QuitIntent{Action: "switch", Chosen: name}
			}
		}
	}
	return t, nil
}
