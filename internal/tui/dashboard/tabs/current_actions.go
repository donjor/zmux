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
// Session-to-workspace moves live in the Workspaces tab. Kill workspace
// uses the same double-confirm path as Phase 3.

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui"
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

// ── rename ──

func (t *CurrentTab) actionRename(row *outline.Row) (dashboard.Tab, tea.Cmd) {
	switch row.Kind {
	case outline.RowWorkspaceHeader:
		if t.wsName == "" {
			return t, nil
		}
		t.rename = &renameState{kind: "workspace", oldName: t.wsName}
		t.mode = currentModeRename
		t.renameInput.SetValue(t.wsName)
		t.renameInput.Focus()
		return t, textinput.Blink

	case outline.RowSession:
		name := t.sessionName
		if !row.Current {
			if s, _ := outline.RowData[session.SessionInfo](row); s != nil {
				name = s.Name
			}
		}
		t.rename = &renameState{kind: "session", oldName: name}
		t.mode = currentModeRename
		t.renameInput.SetValue(name)
		t.renameInput.Focus()
		return t, textinput.Blink

	case outline.RowWindow:
		spec, ok := t.windowSpecFromRow(row)
		if !ok {
			return t, nil
		}
		t.rename = &renameState{
			kind:        "window",
			oldName:     spec.Name,
			sessionName: spec.Session,
			windowIndex: spec.Index,
		}
		t.mode = currentModeRename
		t.renameInput.SetValue(spec.Name)
		t.renameInput.Focus()
		return t, textinput.Blink
	}
	return t, nil
}

func (t *CurrentTab) handleRenameKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
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
			jumpTo = outline.SessionID(newName)
		case "window":
			cmd = t.renameWindow(t.rename.sessionName, t.rename.oldName, newName, t.rename.windowIndex)
			jumpTo = outline.WindowID(t.rename.sessionName, t.rename.windowIndex)
		}
		t.exitMode()
		t.pendingJumpTo = jumpTo
		return t, cmd

	case tea.KeyEscape:
		t.exitMode()
		return t, nil
	}

	var cmd tea.Cmd
	t.renameInput, cmd = t.renameInput.Update(msg)
	return t, cmd
}

// ── kill ──

func (t *CurrentTab) actionKill(row *outline.Row) (dashboard.Tab, tea.Cmd) {
	switch row.Kind {
	case outline.RowWorkspaceHeader:
		if t.wsName == "" {
			return t, nil
		}
		attached := t.attached > 0
		if ws, _ := outline.RowData[tui.WorkspaceViewModel](row); ws != nil {
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

	// Workspace with attached sessions: route through the second confirmation.
	if t.confirm.kind == "workspace" && t.confirm.attached && t.mode != currentModeConfirmKillAttached {
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

// ── new ──

func (t *CurrentTab) actionNew(row *outline.Row) (dashboard.Tab, tea.Cmd) {
	switch row.Kind {
	case outline.RowWorkspaceHeader:
		// Prompt for a new session name in this workspace.
		t.mode = currentModeCreate
		t.createInput.SetValue("")
		t.createInput.Focus()
		return t, textinput.Blink

	case outline.RowSession:
		// New window in the target session (current or sibling).
		target := t.sessionName
		dir := t.sessionDir
		if !row.Current {
			if s, _ := outline.RowData[session.SessionInfo](row); s != nil {
				target = s.Name
				dir = s.Dir
			}
		}
		return t, t.newWindow(target, dir)

	case outline.RowWindow:
		// "n" on a window creates a new tab in that window's session
		// (which may be a sibling, not the current session).
		spec, ok := t.windowSpecFromRow(row)
		if !ok {
			return t, nil
		}
		dir := spec.Dir
		if dir == "" {
			dir = t.sessionDir
		}
		return t, t.newWindow(spec.Session, dir)
	}
	return t, nil
}

func (t *CurrentTab) handleCreateKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		name := strings.TrimSpace(t.createInput.Value())
		if name == "" {
			t.exitMode()
			return t, nil
		}
		cmd := t.createSessionInWorkspace(t.wsName, name)
		t.exitMode()
		// Re-set pendingJumpTo AFTER exitMode — exitMode may apply stale
		// pending data which consumes pendingJumpTo with old rows. The
		// mutation-triggered refetch needs it to land the cursor on the
		// newly created session.
		t.pendingJumpTo = outline.SessionID(name)
		return t, cmd

	case tea.KeyEscape:
		t.exitMode()
		return t, nil
	}

	var cmd tea.Cmd
	t.createInput, cmd = t.createInput.Update(msg)
	return t, cmd
}

// ── move (window to session) ──

func (t *CurrentTab) actionMove(row *outline.Row) (dashboard.Tab, tea.Cmd) {
	if row.Kind != outline.RowWindow {
		return t, nil
	}
	w, _ := outline.RowData[windowDetail](row)
	if w == nil {
		return t, nil
	}
	t.moveSt = &moveState{sessionName: t.sessionName, windowIndex: w.Index}
	return t, t.fetchMoveDestinations()
}

func (t *CurrentTab) handleMoveKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	switch msg.String() {
	case "esc":
		t.exitMode()
		return t, nil

	case "up", "k":
		if t.moveCursor > 0 {
			t.moveCursor--
		}
		return t, nil

	case "down", "j":
		if t.moveCursor < len(t.moveTargets)-1 {
			t.moveCursor++
		}
		return t, nil

	case "enter":
		if t.moveCursor >= len(t.moveTargets) || t.moveSt == nil {
			t.exitMode()
			return t, nil
		}
		dst := t.moveTargets[t.moveCursor].Name
		src := fmt.Sprintf("%s:%d", t.sessionName, t.moveSt.windowIndex)
		runner := t.runner
		reqID := t.reqID
		t.exitMode()
		return t, func() tea.Msg {
			_ = runner.MoveWindow(src, dst)
			return currentMutationDoneMsg{reqID: reqID}
		}
	}
	return t, nil
}

// ── reorder (window only) ──

func (t *CurrentTab) actionReorder(row *outline.Row, delta int) (dashboard.Tab, tea.Cmd) {
	if row.Kind != outline.RowWindow {
		return t, nil
	}
	w, _ := outline.RowData[windowDetail](row)
	if w == nil {
		return t, nil
	}

	// Find neighbour in the windows slice.
	cursorIdx := -1
	for i := range t.windows {
		if t.windows[i].Index == w.Index {
			cursorIdx = i
			break
		}
	}
	if cursorIdx < 0 {
		return t, nil
	}
	neighbourIdx := cursorIdx + delta
	if neighbourIdx < 0 || neighbourIdx >= len(t.windows) {
		return t, nil
	}

	idx1 := t.windows[cursorIdx].Index
	idx2 := t.windows[neighbourIdx].Index
	sessionName := t.sessionName
	runner := t.runner

	// Optimistic local swap.
	t.windows[cursorIdx], t.windows[neighbourIdx] = t.windows[neighbourIdx], t.windows[cursorIdx]
	t.windows[cursorIdx].Index = idx1
	t.windows[neighbourIdx].Index = idx2

	// Rebuild rows and land the cursor on the window at its new position.
	// No pendingJumpTo dance here — the swap is synchronous, so we jump
	// immediately rather than staging for the next data apply.
	t.tree.SetRowsAndJumpTo(t.buildRows(), outline.WindowID(sessionName, idx2))

	return t, func() tea.Msg {
		_ = runner.SwapWindow(sessionName, idx1, idx2)
		return nil // no refetch — local state already matches
	}
}

// Mutation helpers (renameWorkspace, killWorkspace, newWindow, etc.) live
// in current_data.go alongside fetchData. The split keeps current_actions.go
// focused on key routing.
