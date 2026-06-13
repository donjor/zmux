package picker

import (
	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tui/outline"
	"github.com/donjor/zmux/internal/tui/workspaceview"
)

// onInputChanged is the callback invoked whenever the textinput value changes.
// It re-parses the query, refilters workspaces, and rebuilds the outline,
// biasing the cursor to an exact-match workspace when one exists.
func (m *PickerModel) onInputChanged() {
	raw := m.input.Value()
	wsQuery, sessQuery := parseQuery(raw)

	queryChanged := wsQuery != m.state.workspaceQuery || sessQuery != m.state.sessionQuery
	m.state.workspaceQuery = wsQuery
	m.state.sessionQuery = sessQuery

	m.filteredWorkspaces = m.visibleWorkspaces(wsQuery)

	// Exact-workspace-match biases cursor to that workspace so when we
	// build rows below the workspace is automatically expanded. We remember
	// the target ID and jump to it after build.
	var pinTarget string
	if wsQuery != "" {
		for _, ws := range m.filteredWorkspaces {
			if ws.Name == wsQuery {
				pinTarget = outline.WorkspaceID(ws.Name)
				break
			}
		}
	}

	// Reset cursor on query change if we don't have a pin target.
	if queryChanged && pinTarget == "" {
		m.tree.Cursor = 0
	}

	if pinTarget != "" {
		// Build once, then jump to the target so expansion logic sees the
		// target as focused on the next build.
		m.buildOutlineWithFocus(pinTarget)
		m.tree.JumpToID(pinTarget)
		m.buildOutlineWithFocus(pinTarget)
	} else {
		m.buildOutline()
	}
}

// maxBrowseWorkspaces caps the default (unfiltered) workspace list. Empty
// workspaces are always listed (grayed by the renderer); the cap just keeps the
// browse view short. The remainder is revealed with ctrl+h (show-all). Pseudo
// workspaces (e.g. "temporary", which holds live tmp sessions) are exempt from
// the cap so active sessions are never hidden behind show-all.
const maxBrowseWorkspaces = 10

// visibleWorkspaces returns the workspaces to render. A search shows every
// fuzzy match; the default browse view shows all workspaces (empties included,
// grayed) but caps the count at maxBrowseWorkspaces unless show-all is on.
func (m *PickerModel) visibleWorkspaces(query string) []workspaceview.WorkspaceViewModel {
	if query != "" {
		return matchWorkspaces(query, m.workspaces)
	}
	if m.state.showAll {
		return m.workspaces
	}
	return capWorkspaces(m.workspaces, maxBrowseWorkspaces)
}

// capWorkspaces returns at most `limit` real workspaces (in their existing MRU
// order) followed by every pseudo workspace, so live tmp sessions stay visible
// even when older workspaces are collapsed behind show-all.
func capWorkspaces(all []workspaceview.WorkspaceViewModel, limit int) []workspaceview.WorkspaceViewModel {
	if len(all) <= limit {
		return all
	}
	capped := make([]workspaceview.WorkspaceViewModel, 0, limit+1)
	real := 0
	var pseudos []workspaceview.WorkspaceViewModel
	for _, ws := range all {
		if ws.IsPseudo {
			pseudos = append(pseudos, ws)
			continue
		}
		if real < limit {
			capped = append(capped, ws)
			real++
		}
	}
	return append(capped, pseudos...)
}

// applyFilter recomputes filteredWorkspaces and rebuilds the outline.
func (m *PickerModel) applyFilter() {
	m.filteredWorkspaces = m.visibleWorkspaces(m.state.workspaceQuery)
	m.buildOutline()
}

// buildOutline rebuilds the outline rows from current state and pushes
// them into the tree (which preserves cursor by ID).
func (m *PickerModel) buildOutline() {
	m.buildOutlineWithFocus("")
}

// buildOutlineWithFocus is like buildOutline but accepts an explicit
// workspace ID to treat as "focused" (expanded) during the build. Used by
// the exact-match flow in onInputChanged where the cursor hasn't moved yet
// but we want the matched workspace expanded.
func (m *PickerModel) buildOutlineWithFocus(forceFocusWS string) {
	rows := []outline.Row{
		{
			ID:         outline.TopActionID(),
			Kind:       outline.RowTopAction,
			Label:      topActionLabel(m.state.workspaceQuery),
			Selectable: true,
		},
	}

	hasSearch := m.state.workspaceQuery != "" || m.state.sessionQuery != ""

	// Which workspace is "focused" for expansion purposes?
	focusedWS := forceFocusWS
	if focusedWS == "" && !hasSearch {
		if row := m.tree.Current(); row != nil {
			switch row.Kind {
			case outline.RowWorkspaceHeader:
				focusedWS = row.ID
			case outline.RowSession:
				focusedWS = row.ParentID
			}
		}
	}

	for i := range m.filteredWorkspaces {
		ws := &m.filteredWorkspaces[i]
		wsID := outline.WorkspaceID(ws.Name)

		// Filter sessions by session query.
		sessions := ws.LiveSessions
		if m.state.sessionQuery != "" {
			sessions = matchSessions(m.state.sessionQuery, sessions)
		}

		// Expand when searching, or when this is the focused workspace.
		// Note: Expanded isn't set on the row because picker_view doesn't
		// render expansion chevrons — whether children follow is the
		// only signal it needs.
		expanded := hasSearch || wsID == focusedWS

		rows = append(rows, outline.Row{
			ID:         wsID,
			Kind:       outline.RowWorkspaceHeader,
			Label:      formatWorkspaceLabel(ws),
			Selectable: true,
			Attached:   ws.HasAttached,
			Data:       ws,
		})

		if expanded {
			for j := range sessions {
				s := sessions[j]
				rows = append(rows, outline.Row{
					ID:         outline.SessionID(s.Name),
					Kind:       outline.RowSession,
					Depth:      1,
					ParentID:   wsID,
					Label:      pickerSessionLabel(s),
					Selectable: true,
					Attached:   s.Attached,
					Data:       &s,
				})
			}
		}
	}

	// External sources below the workspaces.
	rows = append(rows, buildExternalRows(m.catalog, m.tree)...)

	m.tree.SetRows(rows)
}

func pickerSessionLabel(s session.SessionInfo) string {
	return session.LocalDisplayName(s)
}

// topActionLabel returns the display label for the top action row based
// on the current search query.
func topActionLabel(query string) string {
	if query == "" {
		return "+ new tmp session"
	}
	return "+ create \"" + query + "\""
}

// formatWorkspaceLabel returns the display label for a workspace header row.
// Kept as a helper so the outline builder and views can stay in sync.
func formatWorkspaceLabel(ws *workspaceview.WorkspaceViewModel) string {
	if ws == nil {
		return ""
	}
	return ws.Name
}
