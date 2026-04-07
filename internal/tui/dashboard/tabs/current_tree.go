package tabs

import (
	"fmt"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tui/outline"
	"github.com/donjor/zmux/internal/tui/views"
)

// buildRows constructs the unified session-tab tree:
//
//	workspace header         (depth 0, Current)
//	  current session        (depth 1, Current, always expanded)
//	    window …             (depth 2)
//	    window …
//	  sibling session        (depth 1, never expanded)
//	  sibling session
//
// Per spec, the current session's expansion is hard-coded true — we never
// consult the Tree's expansion map for this tab.
func (t *CurrentTab) buildRows() []outline.Row {
	rows := make([]outline.Row, 0, 8+len(t.windows)+len(t.siblings))

	// ── Workspace header ──
	// Count = current session + sibling sessions. Always ≥ 1 since we render
	// this tab with a known current session.
	wsID := outline.WorkspaceID(t.wsName)
	wsLabel := fmt.Sprintf("%s  %s", t.wsName, outline.FormatSessionCount(1+len(t.siblings)))
	rows = append(rows, outline.Row{
		ID:         wsID,
		Kind:       outline.RowWorkspaceHeader,
		Depth:      0,
		Label:      wsLabel,
		Selectable: true,
		Current:    true,
		Attached:   t.attached > 0,
		Expanded:   true,
		Data:       t.wsModel,
	})

	// ── Current session ──
	currID := outline.SessionID(t.sessionName)
	rows = append(rows, outline.Row{
		ID:         currID,
		Kind:       outline.RowSession,
		Depth:      1,
		ParentID:   wsID,
		Label:      t.sessionName,
		Selectable: true,
		Current:    true,
		Attached:   t.attached > 0,
		Expanded:   true,
	})

	// ── Windows ──
	if len(t.windows) == 0 {
		rows = append(rows, outline.Row{
			ID:       "placeholder:nowindows",
			Kind:     outline.RowPlaceholder,
			Depth:    2,
			ParentID: currID,
			Label:    "(no windows — tmux anomaly)",
		})
	}
	for i := range t.windows {
		w := t.windows[i]
		rows = append(rows, outline.Row{
			ID:         outline.WindowID(t.sessionName, w.Index),
			Kind:       outline.RowWindow,
			Depth:      2,
			ParentID:   currID,
			Label:      w.Name,
			Selectable: true,
			Attached:   w.Active,
			Data:       &t.windows[i],
		})
	}

	// ── Sibling sessions (collapsed, under the same workspace) ──
	for i := range t.siblings {
		s := t.siblings[i]
		rows = append(rows, outline.Row{
			ID:         outline.SessionID(s.Name),
			Kind:       outline.RowSession,
			Depth:      1,
			ParentID:   wsID,
			Label:      s.Name,
			Selectable: true,
			Attached:   s.Attached,
			Data:       &t.siblings[i],
		})
	}

	return rows
}

// ── Row rendering ──

// renderRow paints a single row with kind-specific formatting.
func (t *CurrentTab) renderRow(row *outline.Row, selected bool) string {
	switch row.Kind {
	case outline.RowWorkspaceHeader:
		return t.renderWorkspaceRow(row, selected)
	case outline.RowSession:
		return t.renderSessionRow(row, selected)
	case outline.RowWindow:
		return t.renderWindowTreeRow(row, selected)
	case outline.RowPlaceholder:
		return "    " + t.styles.Dim.Render(row.Label) + "\n"
	}
	return ""
}

func (t *CurrentTab) renderWorkspaceRow(row *outline.Row, selected bool) string {
	cursor := "  "
	if selected {
		cursor = t.styles.Accent.Render("▸ ")
	}
	arrow := "▼" // workspace header is logically always expanded here
	style := t.styles.Normal.Bold(true)
	if selected {
		style = t.styles.Accent.Bold(true)
	}
	line := "  " + cursor + t.styles.Dim.Render(arrow) + " " + style.Render(row.Label)
	if row.Attached {
		line += "  " + t.styles.Info.Render("attached")
	}
	return line + "\n"
}

func (t *CurrentTab) renderSessionRow(row *outline.Row, selected bool) string {
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
	if row.Current {
		line += "  " + t.styles.Success.Render("current")
	} else if row.Attached {
		line += "  " + t.styles.Info.Render("attached")
	}
	return line + "\n"
}

// renderWindowTreeRow renders a window row using the existing two-line
// WindowRow view component, indented to sit under its session row.
func (t *CurrentTab) renderWindowTreeRow(row *outline.Row, selected bool) string {
	w, ok := outline.RowData[windowDetail](row)
	if !ok || w == nil {
		return ""
	}

	view := views.WindowRow{
		Index:      w.Index,
		Name:       w.Name,
		IsActive:   w.Active,
		IsSelected: selected,
		Dir:        shortenDir(w.Dir),
		Uptime:     w.Uptime,
		IsIdle:     isIdleWindow(*w),
	}

	if len(w.Panes) > 0 {
		for _, p := range w.Panes {
			if p.Active {
				view.Command = p.Command
				break
			}
		}
		if view.Command == "" {
			view.Command = w.Panes[0].Command
		}
	}

	if w.Stats.CPU > 0.1 {
		view.CPU = fmt.Sprintf("%.1f%%", w.Stats.CPU)
	}
	if w.Stats.MemMB > 1.0 {
		if w.Stats.MemMB >= 1024 {
			view.Mem = fmt.Sprintf("%.1fGB", w.Stats.MemMB/1024)
		} else {
			view.Mem = fmt.Sprintf("%.0fMB", w.Stats.MemMB)
		}
	}

	rowStyles := views.SessionRowStyles{
		Normal:  t.styles.Normal,
		Accent:  t.styles.Accent,
		Dim:     t.styles.Dim,
		Info:    t.styles.Info,
		Success: t.styles.Success,
	}
	width := t.width
	if width <= 0 {
		width = 80
	}
	return views.RenderWindowRow(view, rowStyles, width)
}
