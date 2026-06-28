package tabs

import (
	"fmt"
	"strings"

	"github.com/sahilm/fuzzy"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tui/outline"
	"github.com/donjor/zmux/internal/tui/scroll"
	"github.com/donjor/zmux/internal/tui/workspaceoutline"
	"github.com/donjor/zmux/internal/tui/workspaceview"
)

// buildRows constructs the flat outline rows from the current snapshot data via
// the shared workspaceoutline builder: workspaces first, then an external
// sources section (with a divider) when the catalog has any external groups.
// The builder owns row STRUCTURE; this tab supplies the dashboard policy
// (chevrons, the current marker, move-mode badges, the empty-workspace line)
// plus the filtering that diverges from the picker — see rowPolicy /
// visibleWorkspaces. Top-level empty states are layered on after the build.
func (t *SessionsTab) buildRows() []outline.Row {
	q := strings.TrimSpace(t.searchQuery)
	// The move-target picker must list every workspace, so a committed filter
	// is suspended (not cleared) for the duration of move mode — both exit
	// paths rebuild once mode returns to list, restoring the filter.
	if t.mode == sessionsModeMove {
		q = ""
	}

	rows := workspaceoutline.Build(t.visibleWorkspaces(q), t.catalog, t.tree, t.rowPolicy(q))

	if len(t.workspaces) == 0 && q == "" {
		// No workspaces and no filter: lead with a hint; any external rows the
		// builder produced still follow.
		return append([]outline.Row{{
			ID:    "placeholder:noworkspaces",
			Kind:  outline.RowPlaceholder,
			Label: "No workspaces yet — press C to create one",
		}}, rows...)
	}
	if q != "" && len(rows) == 0 {
		rows = append(rows, outline.Row{
			ID:    "placeholder:nomatch",
			Kind:  outline.RowPlaceholder,
			Label: fmt.Sprintf("no matches for %q", q),
		})
	}

	return rows
}

// visibleWorkspaces returns the workspaces to emit under the active filter. An
// empty query keeps every workspace; a non-empty query keeps a workspace whose
// name matches (all its sessions show) or that owns a matching session (only
// those show — the row filtering happens in rowPolicy's Sessions callback).
func (t *SessionsTab) visibleWorkspaces(q string) []workspaceview.WorkspaceViewModel {
	if q == "" {
		return t.workspaces
	}
	var out []workspaceview.WorkspaceViewModel
	for i := range t.workspaces {
		ws := &t.workspaces[i]
		if matchQuery(q, ws.Name) {
			out = append(out, *ws)
			continue
		}
		for j := range ws.LiveSessions {
			if matchQuery(q, sessionInfoLabel(&ws.LiveSessions[j])) {
				out = append(out, *ws)
				break
			}
		}
	}
	return out
}

// rowPolicy maps the dashboard's presentation onto the shared builder: full
// chevrons, the tree's saved expansion (force-expanded while filtering), the
// current-session marker, move-mode badges, and the "(no live sessions)" line.
// Session filtering under a query stays here so the builder owns only structure.
func (t *SessionsTab) rowPolicy(q string) workspaceoutline.Policy {
	moving := ""
	if t.mode == sessionsModeMove && t.moveSt != nil {
		moving = t.moveSt.sessionName
	}
	return workspaceoutline.Policy{
		WorkspaceLabel: formatSessionsWorkspaceLabel,
		ShowChevron:    true,
		Expanded: func(wsID string, _ *workspaceview.WorkspaceViewModel) bool {
			if q != "" {
				return true
			}
			return t.tree.IsExpanded(wsID)
		},
		Sessions: func(ws *workspaceview.WorkspaceViewModel) []session.SessionInfo {
			if q == "" || matchQuery(q, ws.Name) {
				return ws.LiveSessions
			}
			var out []session.SessionInfo
			for j := range ws.LiveSessions {
				if matchQuery(q, sessionInfoLabel(&ws.LiveSessions[j])) {
					out = append(out, ws.LiveSessions[j])
				}
			}
			return out
		},
		DecorateWorkspace: func(row *outline.Row, _ *workspaceview.WorkspaceViewModel) {
			if t.mode == sessionsModeMove && t.moveSt != nil {
				row.Badge = "target"
			}
		},
		DecorateSession: func(row *outline.Row, s *session.SessionInfo) {
			if s.Name == t.current {
				row.Current = true
			}
			if moving != "" && s.Name == moving {
				row.Badge = "→ moving"
			}
		},
		EmptyWorkspaceRow: func(ws *workspaceview.WorkspaceViewModel) *outline.Row {
			if q != "" {
				// Matched on workspace name but no sessions — header only.
				return nil
			}
			return &outline.Row{
				ID:       "placeholder:" + ws.Name,
				Kind:     outline.RowPlaceholder,
				Depth:    1,
				ParentID: outline.WorkspaceID(ws.Name),
				Label:    "(no live sessions)",
			}
		},
		ExternalQuery: q,
	}
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

func sessionInfoLabel(s *session.SessionInfo) string {
	if s == nil {
		return ""
	}
	return session.LocalDisplayName(*s)
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
	return scroll.Scrollable(t.vp, t.styles)
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
