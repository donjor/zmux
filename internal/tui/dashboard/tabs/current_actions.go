package tabs

// Per-kind action semantics for the Session & Workspace tab.
//
// | Key   | RowWorkspaceHeader       | RowSession (current)    | RowSession (sibling)     | RowWindow              |
// |-------|--------------------------|-------------------------|--------------------------|------------------------|
// | enter | no-op                    | focus session           | switch to session        | focus window/pane      |
// | r     | rename workspace         | rename session          | rename session           | rename window          |
// | x     | kill workspace (2-step)  | kill session (confirm)  | kill session (confirm)   | kill window/pane       |
// | c     | new session in workspace | new window              | new window in target     | new window             |
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

	"github.com/donjor/zmux/internal/keys"
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
	case currentModeSearch:
		return t.handleSearchKey(msg)
	default:
		return t.handleListKey(msg)
	}
}

// handleListKey routes single-key shortcuts in list mode. The same key
// can mean different things depending on the row kind under the cursor.
func (t *CurrentTab) handleListKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
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
	case s == "right" || s == "l":
		return t.enterWindowLevel()
	case s == "left" || s == "h":
		return t.exitWindowLevel()
	}

	if t.sessionName == "" {
		if keys.DashCreate.Matches(s) {
			return t, func() tea.Msg {
				return dashboard.QuitIntent{Action: "new"}
			}
		}
		return t, nil
	}

	// Search + quick-jump require an active session (the no-session view has no
	// list to filter or number, and would show no search input).
	switch {
	case keys.DashSearch.Matches(s):
		return t.enterSearchMode()
	case s == "esc":
		// Reaches the tab only when a filter is active (see CapturesEscape);
		// the first Esc clears the filter, a second closes the dashboard.
		if t.searchQuery != "" {
			t.clearSearch()
		}
		return t, nil
	case len(s) == 1 && s[0] >= '1' && s[0] <= '9':
		return t.handleSessionDigit(int(s[0] - '0'))
	}

	row := t.tree.Current()
	if row == nil {
		return t, nil
	}

	switch {
	case keys.DashSelect.Matches(s):
		return t.actionEnter(row)
	case keys.DashRename.Matches(s):
		return t.actionRename(row)
	case keys.DashKill.Matches(s):
		return t.actionKill(row)
	case keys.DashCreate.Matches(s):
		return t.actionNew(row)
	case keys.DashMove.Matches(s):
		return t.actionMove(row)
	case s == "<":
		return t.actionReorder(row, -1)
	case s == ">":
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
