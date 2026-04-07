package tabs

import (
	"fmt"
	"strings"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/source"
	"github.com/donjor/zmux/internal/tui"
	"github.com/donjor/zmux/internal/tui/outline"
)

// buildRows constructs the flat outline rows from the current snapshot data.
// Workspaces come first, followed by an external sources section (with a
// divider) when the catalog has any external groups.
func (t *SessionsTab) buildRows() []outline.Row {
	rows := make([]outline.Row, 0, 32)

	// ── Workspaces ──
	if len(t.workspaces) == 0 {
		rows = append(rows, outline.Row{
			ID:    "placeholder:noworkspaces",
			Kind:  outline.RowPlaceholder,
			Label: "No workspaces yet — press n to create one",
		})
	} else {
		for i := range t.workspaces {
			ws := &t.workspaces[i]
			rows = append(rows, t.buildWorkspaceRow(ws)...)
		}
	}

	// ── External sources (kept from original implementation) ──
	rows = append(rows, t.buildExternalRows()...)

	return rows
}

// buildWorkspaceRow returns the workspace header row plus its session rows
// when expanded. Pseudo workspaces (e.g. "temporary") are emitted but never
// participate in mutations.
func (t *SessionsTab) buildWorkspaceRow(ws *tui.WorkspaceViewModel) []outline.Row {
	wsID := outline.WorkspaceID(ws.Name)
	expanded := t.tree.IsExpanded(wsID)

	header := outline.Row{
		ID:         wsID,
		Kind:       outline.RowWorkspaceHeader,
		Depth:      0,
		Label:      formatSessionsWorkspaceLabel(ws),
		Selectable: true,
		Expanded:   expanded,
		Attached:   ws.HasAttached,
		Data:       ws,
	}
	if t.mode == sessionsModeMove && t.moveSt != nil {
		header.Badge = "target"
	}

	rows := []outline.Row{header}

	if !expanded {
		return rows
	}

	if len(ws.LiveSessions) == 0 {
		rows = append(rows, outline.Row{
			ID:       "placeholder:" + ws.Name,
			Kind:     outline.RowPlaceholder,
			Depth:    1,
			ParentID: wsID,
			Label:    "(no live sessions)",
		})
		return rows
	}

	for j := range ws.LiveSessions {
		s := &ws.LiveSessions[j]
		row := outline.Row{
			ID:         outline.SessionID(s.Name),
			Kind:       outline.RowSession,
			Depth:      1,
			ParentID:   wsID,
			Label:      s.Name,
			Selectable: true,
			Current:    s.Name == t.current,
			Attached:   s.Attached,
			Data:       s,
		}
		if t.mode == sessionsModeMove && t.moveSt != nil && s.Name == t.moveSt.sessionName {
			row.Badge = "→ moving"
		}
		rows = append(rows, row)
	}

	return rows
}

// buildExternalRows constructs the external-source section of the outline.
// Returns nil if there are no external groups.
func (t *SessionsTab) buildExternalRows() []outline.Row {
	if t.catalog == nil || len(t.catalog.External) == 0 {
		return nil
	}

	rows := []outline.Row{{
		ID:    "divider:external",
		Kind:  outline.RowDivider,
		Label: "── external ──",
	}}

	for i := range t.catalog.External {
		g := &t.catalog.External[i]
		kind := string(g.Source.Kind)
		key := source.GroupKey(g)
		groupID := outline.ExternalGroupID(kind, key)

		label := fmt.Sprintf("%s: %s", kind, g.Source.Label)
		if n := len(g.Entries); n > 0 {
			label += fmt.Sprintf("  (%d)", n)
		}
		if g.Source.Health == source.HealthDegraded {
			label += "  [degraded]"
		}

		rows = append(rows, outline.Row{
			ID:         groupID,
			Kind:       outline.RowExternalGroup,
			Label:      label,
			Selectable: true,
			Expanded:   t.tree.IsExpanded(groupID),
			Data:       g,
		})

		if !t.tree.IsExpanded(groupID) {
			continue
		}
		for j := range g.Entries {
			entry := g.Entries[j]
			rows = append(rows, outline.Row{
				ID:         outline.ExternalEntryID(kind, entry.Session),
				Kind:       outline.RowExternalEntry,
				Label:      entry.Session,
				Depth:      1,
				ParentID:   groupID,
				Selectable: true,
				Attached:   entry.Attached,
				Data:       &entry,
			})
		}
	}
	return rows
}

// formatSessionsWorkspaceLabel renders the workspace header label with
// session count and attached marker.
func formatSessionsWorkspaceLabel(ws *tui.WorkspaceViewModel) string {
	if ws == nil {
		return ""
	}
	count := outline.FormatSessionCount(ws.LiveSessionCount)
	label := fmt.Sprintf("%s  %s", ws.Name, count)
	if ws.HasAttached {
		label += "  [attached]"
	}
	return label
}

// externalGroupForRow looks up the source group that owns an external entry
// or group row. Returns (group, true) on hit.
func externalGroupForRow(cat *source.Catalog, row *outline.Row) (*source.SourceGroup, bool) {
	if cat == nil || row == nil {
		return nil, false
	}
	switch row.Kind {
	case outline.RowExternalGroup:
		if g, ok := outline.RowData[source.SourceGroup](row); ok {
			return g, true
		}
	case outline.RowExternalEntry:
		// Walk catalog to find the parent group of this entry's ID.
		for i := range cat.External {
			g := &cat.External[i]
			kind := string(g.Source.Kind)
			for j := range g.Entries {
				if outline.ExternalEntryID(kind, g.Entries[j].Session) == row.ID {
					return g, true
				}
			}
		}
	}
	return nil, false
}

// ── View rendering ──

// View renders the workspaces tab content.
func (t *SessionsTab) View() string {
	var b strings.Builder

	// Header line: workspace count + current session marker.
	count := len(t.workspaces)
	header := fmt.Sprintf("%d workspaces", count)
	if count == 1 {
		header = "1 workspace"
	}
	if t.current != "" {
		header += "  " + t.styles.Dim.Render("|") + "  " + t.styles.Success.Render(t.current)
	}
	b.WriteString("\n")
	b.WriteString("  " + t.styles.Dim.Render(header) + "\n\n")

	// Overlays render above the list.
	switch t.mode {
	case sessionsModeRename:
		b.WriteString(t.renderRenameOverlay())
	case sessionsModeCreate:
		b.WriteString(t.renderCreateOverlay())
	case sessionsModeConfirmKill:
		b.WriteString(t.renderConfirmOverlay(1))
	case sessionsModeConfirmKillAttached:
		b.WriteString(t.renderConfirmOverlay(2))
	}

	// Compute scroll window.
	rows := t.tree.Rows
	listHeight := t.height - 8
	if listHeight < 5 {
		listHeight = 12
	}
	start, end := outline.ComputeWindow(t.tree.Cursor, len(rows), listHeight)

	if start > 0 {
		b.WriteString(t.styles.Dim.Render(fmt.Sprintf("  ↑ %d more", start)) + "\n")
	}

	for i := start; i < end; i++ {
		b.WriteString(t.renderRow(&rows[i], i == t.tree.Cursor))
	}

	if end < len(rows) {
		b.WriteString(t.styles.Dim.Render(fmt.Sprintf("  ↓ %d more", len(rows)-end)) + "\n")
	}

	if len(rows) == 0 {
		b.WriteString(t.styles.Dim.Render("  (no rows)") + "\n")
	}

	return b.String()
}

// renderRow paints a single row with kind-specific formatting.
func (t *SessionsTab) renderRow(row *outline.Row, selected bool) string {
	switch row.Kind {
	case outline.RowWorkspaceHeader:
		return t.renderWorkspaceRowView(row, selected)
	case outline.RowSession:
		return t.renderSessionRowView(row, selected)
	case outline.RowExternalGroup:
		return t.renderExternalGroupRowView(row, selected)
	case outline.RowExternalEntry:
		return t.renderExternalEntryRowView(row, selected)
	case outline.RowDivider:
		return "  " + t.styles.Dim.Render(row.Label) + "\n"
	case outline.RowPlaceholder:
		return "    " + t.styles.Dim.Render(row.Label) + "\n"
	}
	return ""
}

func (t *SessionsTab) renderWorkspaceRowView(row *outline.Row, selected bool) string {
	cursor := "  "
	if selected {
		cursor = t.styles.Accent.Render("▸ ")
	}
	arrow := "▶"
	if row.Expanded {
		arrow = "▼"
	}
	style := t.styles.Normal.Bold(true)
	if selected {
		style = t.styles.Accent.Bold(true)
	}
	line := "  " + cursor + t.styles.Dim.Render(arrow) + " " + style.Render(row.Label)
	if row.Badge != "" {
		line += "  " + t.styles.Info.Render("["+row.Badge+"]")
	}
	return line + "\n"
}

func (t *SessionsTab) renderSessionRowView(row *outline.Row, selected bool) string {
	cursor := "    "
	if selected {
		cursor = "  " + t.styles.Accent.Render("▸ ")
	}
	icon := "○"
	iconStyle := t.styles.Dim
	if row.Attached {
		icon = "●"
		iconStyle = t.styles.Info
	}
	if selected {
		iconStyle = t.styles.Accent
	}
	nameStyle := t.styles.Normal
	if row.Current {
		nameStyle = t.styles.Success.Bold(true)
	}
	if selected {
		nameStyle = t.styles.Accent.Bold(true)
	}
	line := "  " + cursor + iconStyle.Render(icon) + " " + nameStyle.Render(row.Label)
	if s, ok := outline.RowData[session.SessionInfo](row); ok && s != nil {
		line += "  " + t.styles.Dim.Render(fmt.Sprintf("%dw", s.Windows))
	}
	if row.Attached {
		line += "  " + t.styles.Info.Render("attached")
	}
	if row.Badge != "" {
		line += "  " + t.styles.Info.Render(row.Badge)
	}
	return line + "\n"
}

func (t *SessionsTab) renderExternalGroupRowView(row *outline.Row, selected bool) string {
	cursor := "  "
	if selected {
		cursor = t.styles.Accent.Render("▸ ")
	}
	arrow := "▶"
	if row.Expanded {
		arrow = "▼"
	}
	style := t.styles.Dim.Bold(true)
	if selected {
		style = t.styles.Accent.Bold(true)
	}
	return "  " + cursor + style.Render(arrow+" "+row.Label) + "\n"
}

func (t *SessionsTab) renderExternalEntryRowView(row *outline.Row, selected bool) string {
	cursor := "    "
	if selected {
		cursor = "  " + t.styles.Accent.Render("▸ ")
	}
	icon := "○"
	iconStyle := t.styles.Dim
	if row.Attached {
		icon = "●"
		iconStyle = t.styles.Info
	}
	nameStyle := t.styles.Muted
	if selected {
		nameStyle = t.styles.Accent.Bold(true)
	}
	return "  " + cursor + iconStyle.Render(icon) + " " + nameStyle.Render(row.Label) + "\n"
}
