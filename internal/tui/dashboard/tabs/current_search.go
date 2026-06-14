package tabs

// Search + numbered quick-jump for the Session & Workspace tab. Mirrors the
// Workspaces tab's `/` search model (sessions_actions.go) over this tab's
// single-workspace session list. Filtering happens while building rows
// (current_tree.go), so navigation, the digit handlers, and the [N] badges
// all operate on the same visible set.

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/donjor/zmux/internal/tui/dashboard"
	"github.com/donjor/zmux/internal/tui/outline"
)

// ── Search mode ──

// enterSearchMode opens the `/` search input, pre-filled with any active
// filter. Search is session-scoped, so a window-level cursor is collapsed
// first — its expanded session may be filtered out.
func (t *CurrentTab) enterSearchMode() (dashboard.Tab, tea.Cmd) {
	if t.navLevel == navLevelWindow {
		t.navLevel = navLevelSession
		t.expandedSessionID = ""
	}
	t.mode = currentModeSearch
	t.searchInput.SetValue(t.searchQuery)
	t.searchInput.CursorEnd()
	t.searchInput.Focus()
	t.tree.SetRows(t.buildRows())
	return t, textinput.Blink
}

// handleSearchKey drives the inline search input. Typing live-filters the
// session list; Enter applies the filter and returns to list browsing (the
// filter stays active); Esc cancels it entirely. Arrow keys move the cursor
// through the filtered results without leaving the input.
func (t *CurrentTab) handleSearchKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
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

// finishSearch transitions from search-edit back to list mode, flushing any
// data refetch staged while editing and rebuilding the (possibly still
// filtered) tree.
func (t *CurrentTab) finishSearch() {
	t.mode = currentModeList
	t.searchInput.Blur()
	if t.pending != nil {
		t.applyData(*t.pending)
		t.pending = nil
		return
	}
	t.tree.SetRows(t.buildRows())
}

// clearSearch drops the active filter and rebuilds the full tree.
func (t *CurrentTab) clearSearch() {
	t.searchQuery = ""
	t.searchInput.SetValue("")
	t.searchInput.Blur()
	t.tree.SetRows(t.buildRows())
}

// ── Numbered quick-jump ──

// handleSessionDigit activates the nth visible session (1-based), mirroring
// tmux's prefix+number: focus the current session, switch to a sibling. No-op
// when fewer than n sessions are visible under the active filter.
func (t *CurrentTab) handleSessionDigit(n int) (dashboard.Tab, tea.Cmd) {
	// Count over session rows, so collapse a window-level cursor first.
	if t.navLevel == navLevelWindow {
		t.navLevel = navLevelSession
		t.expandedSessionID = ""
		t.tree.SetRows(t.buildRows())
	}
	id := t.nthSessionRowID(n)
	if id == "" {
		return t, nil
	}
	t.tree.JumpToID(id)
	return t.actionEnter(t.tree.Current())
}

// nthSessionRowID returns the stable ID of the nth visible selectable session
// row (1-based), or "" if there are fewer than n.
func (t *CurrentTab) nthSessionRowID(n int) string {
	count := 0
	for i := range t.tree.Rows {
		r := &t.tree.Rows[i]
		if r.Kind == outline.RowSession && r.Selectable {
			count++
			if count == n {
				return r.ID
			}
		}
	}
	return ""
}

// sessionNumberForRow returns the 1-based quick-jump position of the session
// row with the given ID among the visible selectable session rows, or 0 if
// not found. Used to paint the [N] badge so the digit and the badge agree.
func (t *CurrentTab) sessionNumberForRow(id string) int {
	count := 0
	for i := range t.tree.Rows {
		r := &t.tree.Rows[i]
		if r.Kind == outline.RowSession && r.Selectable {
			count++
			if r.ID == id {
				return count
			}
		}
	}
	return 0
}
