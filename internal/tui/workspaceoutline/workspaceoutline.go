// Package workspaceoutline builds the shared workspace / session / external
// outline rows consumed by the workspace+session picker (internal/tui/picker)
// and the dashboard Workspaces tab (internal/tui/dashboard/tabs).
//
// It owns the row STRUCTURE both surfaces previously reimplemented: the
// top-action → workspace → session → external ordering, the row kinds, stable
// IDs, depths, and parent links, plus the entire external-source section.
// Presentation that genuinely differs between the flat picker and the full
// manager — labels, the expansion model, badges, the current-session marker,
// empty-state placeholders, and the top-action row — is supplied through Policy
// callbacks. Filtering, workspace caps, and input handling stay in each caller,
// because those diverge by design and folding them in would only move the
// branching here.
package workspaceoutline

import (
	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/source"
	"github.com/donjor/zmux/internal/tui/outline"
	"github.com/donjor/zmux/internal/tui/workspaceview"
)

// Policy supplies the surface-specific decisions to Build. WorkspaceLabel,
// Expanded, and Sessions are required; the rest are optional.
type Policy struct {
	// TopAction, when non-nil, emits a leading RowTopAction with this label
	// (the picker's "+ new" affordance). The dashboard leaves it nil.
	TopAction func() string

	// WorkspaceLabel formats a workspace header's label.
	WorkspaceLabel func(ws *workspaceview.WorkspaceViewModel) string

	// SessionLabel formats a session row's label. Defaults to
	// session.LocalDisplayName when nil (both surfaces use that default today).
	SessionLabel func(s *session.SessionInfo) string

	// Expanded reports whether a workspace's sessions should be emitted. The
	// picker decides by cursor focus / active search; the dashboard by the
	// tree's saved chevron state (force-expanded while filtering).
	Expanded func(wsID string, ws *workspaceview.WorkspaceViewModel) bool

	// ShowChevron records the expansion state on the header row so a renderer
	// can draw a chevron. The picker leaves this false (it shows no chevrons —
	// whether children follow is its only signal).
	ShowChevron bool

	// Sessions returns the already-filtered sessions to emit for a workspace.
	Sessions func(ws *workspaceview.WorkspaceViewModel) []session.SessionInfo

	// DecorateWorkspace / DecorateSession mutate a freshly-built row to add
	// badges, current markers, etc. Optional.
	DecorateWorkspace func(row *outline.Row, ws *workspaceview.WorkspaceViewModel)
	DecorateSession   func(row *outline.Row, s *session.SessionInfo)

	// EmptyWorkspaceRow, when non-nil, returns an optional placeholder row to
	// emit under an expanded workspace with no sessions (return nil to emit
	// none). The dashboard uses this for its "(no live sessions)" line; the
	// picker leaves it nil.
	EmptyWorkspaceRow func(ws *workspaceview.WorkspaceViewModel) *outline.Row

	// ExternalQuery filters the external-source section. "" applies no filter
	// (the picker's behaviour); a non-empty query keeps matching groups/entries
	// and force-expands them (the dashboard's behaviour).
	ExternalQuery string
}

// Build assembles the outline rows for the given workspaces and external
// catalog under the policy. The workspaces slice is taken as already
// filtered/capped/ordered by the caller; Build does not filter or sort it.
func Build(workspaces []workspaceview.WorkspaceViewModel, cat *source.Catalog, tree *outline.Tree, p Policy) []outline.Row {
	rows := make([]outline.Row, 0, len(workspaces)*2+8)

	if p.TopAction != nil {
		rows = append(rows, outline.Row{
			ID:         outline.TopActionID(),
			Kind:       outline.RowTopAction,
			Label:      p.TopAction(),
			Selectable: true,
		})
	}

	for i := range workspaces {
		ws := &workspaces[i]
		wsID := outline.WorkspaceID(ws.Name)

		header := outline.Row{
			ID:         wsID,
			Kind:       outline.RowWorkspaceHeader,
			Label:      p.WorkspaceLabel(ws),
			Selectable: true,
			Attached:   ws.HasAttached,
			Data:       ws,
		}
		expanded := p.Expanded(wsID, ws)
		if p.ShowChevron {
			header.Expanded = expanded
		}
		if p.DecorateWorkspace != nil {
			p.DecorateWorkspace(&header, ws)
		}
		rows = append(rows, header)

		if !expanded {
			continue
		}

		sessions := p.Sessions(ws)
		if len(sessions) == 0 {
			if p.EmptyWorkspaceRow != nil {
				if ph := p.EmptyWorkspaceRow(ws); ph != nil {
					rows = append(rows, *ph)
				}
			}
			continue
		}
		for j := range sessions {
			s := &sessions[j]
			row := outline.Row{
				ID:         outline.SessionID(s.Name),
				Kind:       outline.RowSession,
				Depth:      1,
				ParentID:   wsID,
				Label:      sessionLabel(p, s),
				Selectable: true,
				Attached:   s.Attached,
				Data:       s,
			}
			if p.DecorateSession != nil {
				p.DecorateSession(&row, s)
			}
			rows = append(rows, row)
		}
	}

	return append(rows, BuildExternalRows(cat, tree, p.ExternalQuery)...)
}

func sessionLabel(p Policy, s *session.SessionInfo) string {
	if p.SessionLabel != nil {
		return p.SessionLabel(s)
	}
	return session.LocalDisplayName(*s)
}
