package tabs

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tui/dashboard"
	"github.com/donjor/zmux/internal/tui/outline"
)

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
		case "window":
			cmd = t.renameWindow(t.rename.sessionName, t.rename.oldName, newName, t.rename.windowIndex)
			jumpTo = outline.WindowID(t.rename.sessionName, t.rename.windowIndex)
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
	switch msg.String() {
	case "enter":
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

	case "esc":
		t.exitMode()
		return t, nil
	}

	var cmd tea.Cmd
	t.createInput, cmd = t.createInput.Update(msg)
	return t, cmd
}
