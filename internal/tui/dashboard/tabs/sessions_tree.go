package tabs

import (
	"fmt"
	"strings"

	"github.com/sahilm/fuzzy"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/source"
	"github.com/donjor/zmux/internal/tui/outline"
	"github.com/donjor/zmux/internal/tui/workspaceview"
)

// buildRows constructs the flat outline rows from the current snapshot data.
// Workspaces come first, followed by an external sources section (with a
// divider) when the catalog has any external groups. When a search filter is
// active (t.searchQuery), rows are filtered across all kinds — see
// buildWorkspaceRow / buildExternalRows.
func (t *SessionsTab) buildRows() []outline.Row {
	q := strings.TrimSpace(t.searchQuery)
	// The move-target picker must list every workspace, so a committed filter
	// is suspended (not cleared) for the duration of move mode — both exit
	// paths rebuild once mode returns to list, restoring the filter.
	if t.mode == sessionsModeMove {
		q = ""
	}
	rows := make([]outline.Row, 0, 32)

	// ── Workspaces ──
	if len(t.workspaces) == 0 && q == "" {
		rows = append(rows, outline.Row{
			ID:    "placeholder:noworkspaces",
			Kind:  outline.RowPlaceholder,
			Label: "No workspaces yet — press n to create one",
		})
	} else {
		for i := range t.workspaces {
			ws := &t.workspaces[i]
			rows = append(rows, t.buildWorkspaceRow(ws, q)...)
		}
	}

	// ── External sources ──
	rows = append(rows, t.buildExternalRows(q)...)

	if q != "" && len(rows) == 0 {
		rows = append(rows, outline.Row{
			ID:    "placeholder:nomatch",
			Kind:  outline.RowPlaceholder,
			Label: fmt.Sprintf("no matches for %q", q),
		})
	}

	return rows
}

// matchQuery reports whether target fuzzy-matches the (non-empty) query.
// Matching is on raw names/fields, never decorated labels (counts, glyphs).
func matchQuery(query, target string) bool {
	if query == "" {
		return true
	}
	return len(fuzzy.Find(query, []string{target})) > 0
}

// rowsContain reports whether any row has the given ID. Used to detect a
// jump target hidden by an active filter before SetRowsAndJumpTo.
func rowsContain(rows []outline.Row, id string) bool {
	for i := range rows {
		if rows[i].ID == id {
			return true
		}
	}
	return false
}

// buildWorkspaceRow returns the workspace header row plus its session rows
// when expanded. Pseudo workspaces (e.g. "temporary") are emitted but never
// participate in mutations.
//
// When q is non-empty the workspace is filtered: the header is kept if the
// workspace name matches (then all its sessions show) or if any session name
// matches (then only the matching sessions show); a workspace with no match
// is dropped entirely. While filtering, children are force-expanded for
// visibility without touching the tree's saved expansion state.
func (t *SessionsTab) buildWorkspaceRow(ws *workspaceview.WorkspaceViewModel, q string) []outline.Row {
	wsID := outline.WorkspaceID(ws.Name)

	wsMatch := q == "" || matchQuery(q, ws.Name)
	var matchingSessions []int
	if q != "" && !wsMatch {
		for j := range ws.LiveSessions {
			if matchQuery(q, sessionInfoLabel(&ws.LiveSessions[j])) {
				matchingSessions = append(matchingSessions, j)
			}
		}
		if len(matchingSessions) == 0 {
			return nil
		}
	}

	expanded := t.tree.IsExpanded(wsID)
	if q != "" {
		expanded = true
	}

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
		if q != "" {
			// Matched on workspace name but has no sessions — header only.
			return rows
		}
		rows = append(rows, outline.Row{
			ID:       "placeholder:" + ws.Name,
			Kind:     outline.RowPlaceholder,
			Depth:    1,
			ParentID: wsID,
			Label:    "(no live sessions)",
		})
		return rows
	}

	if q != "" && !wsMatch {
		for _, j := range matchingSessions {
			rows = append(rows, t.buildSessionRow(&ws.LiveSessions[j], wsID))
		}
	} else {
		for j := range ws.LiveSessions {
			rows = append(rows, t.buildSessionRow(&ws.LiveSessions[j], wsID))
		}
	}

	return rows
}

// buildSessionRow builds a single session row under the given workspace.
func (t *SessionsTab) buildSessionRow(s *session.SessionInfo, wsID string) outline.Row {
	row := outline.Row{
		ID:         outline.SessionID(s.Name),
		Kind:       outline.RowSession,
		Depth:      1,
		ParentID:   wsID,
		Label:      sessionInfoLabel(s),
		Selectable: true,
		Current:    s.Name == t.current,
		Attached:   s.Attached,
		Data:       s,
	}
	if t.mode == sessionsModeMove && t.moveSt != nil && s.Name == t.moveSt.sessionName {
		row.Badge = "→ moving"
	}
	return row
}

func sessionInfoLabel(s *session.SessionInfo) string {
	if s == nil {
		return ""
	}
	return session.LocalDisplayName(*s)
}

// buildExternalRows constructs the external-source section of the outline.
// Returns nil if there are no external groups (or none survive the filter).
//
// When q is non-empty a group is kept if its source label or kind matches
// (then all its entries show) or if any entry matches (then only the matching
// entries show). The divider is emitted only when at least one group survives.
func (t *SessionsTab) buildExternalRows(q string) []outline.Row {
	if t.catalog == nil || len(t.catalog.External) == 0 {
		return nil
	}

	var body []outline.Row
	for i := range t.catalog.External {
		g := &t.catalog.External[i]
		kind := string(g.Source.Kind)
		key := source.GroupKey(g)
		groupID := outline.ExternalGroupID(kind, key)

		groupMatch := q == "" || matchQuery(q, g.Source.Label) || matchQuery(q, kind)
		var matchingEntries []int
		if q != "" && !groupMatch {
			for j := range g.Entries {
				if matchQuery(q, g.Entries[j].Session) {
					matchingEntries = append(matchingEntries, j)
				}
			}
			if len(matchingEntries) == 0 {
				continue
			}
		}

		label := fmt.Sprintf("%s: %s", kind, g.Source.Label)
		if n := len(g.Entries); n > 0 {
			label += fmt.Sprintf("  (%d)", n)
		}
		if g.Source.Health == source.HealthDegraded {
			label += "  [degraded]"
		}

		expanded := t.tree.IsExpanded(groupID)
		if q != "" {
			expanded = true
		}

		body = append(body, outline.Row{
			ID:         groupID,
			Kind:       outline.RowExternalGroup,
			Label:      label,
			Selectable: true,
			Expanded:   expanded,
			Data:       g,
		})

		if !expanded {
			continue
		}

		emit := func(j int) {
			entry := g.Entries[j]
			body = append(body, outline.Row{
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
		if q != "" && !groupMatch {
			for _, j := range matchingEntries {
				emit(j)
			}
		} else {
			for j := range g.Entries {
				emit(j)
			}
		}
	}

	if len(body) == 0 {
		return nil
	}

	rows := []outline.Row{{
		ID:    "divider:external",
		Kind:  outline.RowDivider,
		Label: "── external ──",
	}}
	return append(rows, body...)
}

// formatSessionsWorkspaceLabel renders the workspace header label with
// session count and attached marker.
func formatSessionsWorkspaceLabel(ws *workspaceview.WorkspaceViewModel) string {
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
		header += "  " + t.styles.Dim.Render("|") + "  " + t.styles.Success.Render(sessionLabelForName(t.current, t.workspaces))
	}
	// Active-filter chip (shown while a committed filter narrows the tree).
	if t.mode != sessionsModeSearch && t.searchQuery != "" {
		header += "  " + t.styles.Dim.Render("|") + "  " + t.styles.Info.Render("filter: "+t.searchQuery)
	}
	b.WriteString("\n")
	b.WriteString("  " + t.styles.Dim.Render(header) + "\n\n")

	// Overlays render above the scrollable content.
	switch t.mode {
	case sessionsModeRename:
		b.WriteString(t.renderRenameOverlay())
	case sessionsModeCreate:
		b.WriteString(t.renderCreateOverlay())
	case sessionsModeConfirmKill:
		b.WriteString(t.renderConfirmOverlay(1))
	case sessionsModeConfirmKillAttached:
		b.WriteString(t.renderConfirmOverlay(2))
	case sessionsModeSearch:
		b.WriteString(t.renderSearchOverlay())
	}

	// Render ALL rows — viewport handles clipping and scrolling.
	rows := t.tree.Rows
	cursorLine := 0
	lineCount := 0
	for i := range rows {
		if i == t.tree.Cursor {
			cursorLine = lineCount
		}
		rendered := t.renderRow(&rows[i], i == t.tree.Cursor)
		b.WriteString(rendered)
		lineCount += strings.Count(rendered, "\n")
	}
	if len(rows) == 0 {
		b.WriteString(t.styles.Dim.Render("  (no rows)") + "\n")
	}

	t.vp.SetContent(b.String())
	ensureCursorVisible(&t.vp, cursorLine)
	return renderScrollable(t.vp, t.styles)
}

func sessionLabelForName(name string, workspaces []workspaceview.WorkspaceViewModel) string {
	for i := range workspaces {
		for j := range workspaces[i].LiveSessions {
			s := workspaces[i].LiveSessions[j]
			if s.Name == name {
				return session.QualifiedDisplayName(s)
			}
		}
	}
	return name
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
		if s, ok2 := outline.RowData[session.SessionInfo](row); ok2 && s != nil && s.AttachedClients > 1 {
			line += "  " + t.styles.Info.Render(fmt.Sprintf("attached ×%d", s.AttachedClients))
		} else {
			line += "  " + t.styles.Info.Render("attached")
		}
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
