package tabs

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/donjor/zmux/internal/tui/dashboard"
	"github.com/donjor/zmux/internal/tui/outline"
)

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
