package picker

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tui/outline"
	"github.com/donjor/zmux/internal/tui/workspaceview"
)

// viewList renders the outline via outline.ComputeWindow / RenderRow.
// The picker keeps its own per-kind renderers for now (rich formatting
// that's specific to this view); future phases may migrate to outline's
// shared renderers.
func (m PickerModel) viewList() string {
	var b strings.Builder

	// ── Input ──
	prompt := m.styles.Dim.Render("   ")
	b.WriteString(prompt + m.input.View() + "\n\n")

	// Compute visible window via outline helper.
	chrome := 10
	if m.height < 30 {
		chrome = 6
	}
	listHeight := m.height - chrome
	if listHeight < 3 {
		listHeight = 3
	}

	rows := m.tree.Rows
	start, end := outline.ComputeWindow(m.tree.Cursor, len(rows), listHeight)

	if start > 0 {
		b.WriteString(m.styles.Dim.Render("  ↑ more") + "\n")
	}

	confirming := (m.mode == modeConfirmDelete || m.mode == modeConfirmDeleteAttached) && m.confirm != nil

	for i := start; i < end; i++ {
		row := &rows[i]
		selected := i == m.tree.Cursor
		// Delete confirmation renders in place of the cursor row, so the
		// prompt sits exactly where the target is.
		if selected && confirming {
			b.WriteString(m.renderInlineConfirm(row))
			continue
		}
		switch row.Kind {
		case outline.RowTopAction:
			b.WriteString(m.renderTopAction(selected))
		case outline.RowWorkspaceHeader:
			if ws, ok := outline.RowData[workspaceview.WorkspaceViewModel](row); ok && ws != nil {
				b.WriteString(m.renderWorkspaceRow(*ws, selected))
			}
		case outline.RowSession:
			if s, ok := outline.RowData[session.SessionInfo](row); ok && s != nil {
				b.WriteString(m.renderSessionRow(*s, selected))
			}
		case outline.RowDivider:
			b.WriteString(m.styles.Dim.Render("  "+row.Label) + "\n")
		case outline.RowExternalGroup:
			b.WriteString(m.renderExternalGroupRow(row, selected))
		case outline.RowExternalEntry:
			b.WriteString(m.renderExternalEntryRow(row, selected))
		case outline.RowPlaceholder:
			b.WriteString(m.styles.Dim.Render("    "+row.Label) + "\n")
		}
	}

	if end < len(rows) {
		b.WriteString(m.styles.Dim.Render("  ↓ more") + "\n")
	}

	// Show-all affordance: the browse view is capped, so surface how many
	// workspaces are collapsed behind ctrl+h rather than truncating silently.
	if m.state.workspaceQuery == "" && !m.state.showAll {
		if hidden := len(m.workspaces) - len(m.filteredWorkspaces); hidden > 0 {
			b.WriteString(m.styles.Dim.Render(fmt.Sprintf("  + %d more (ctrl+h)", hidden)) + "\n")
		}
	}

	// Helpful empty state.
	if len(rows) <= 1 && m.state.workspaceQuery == "" {
		b.WriteString(m.styles.Dim.Render("  no workspaces yet — type a name to create one") + "\n")
	}

	return b.String()
}

// renderExternalGroupRow renders the "overmind: <label>  (3)" group header.
func (m PickerModel) renderExternalGroupRow(row *outline.Row, selected bool) string {
	cursor := "  "
	if selected {
		cursor = m.styles.Accent.Render("▸ ")
	}
	arrow := "▶"
	if row.Expanded {
		arrow = "▼"
	}
	style := m.styles.Dim.Bold(true)
	if selected {
		style = m.styles.Accent.Bold(true)
	}
	return "  " + cursor + style.Render(arrow+" "+row.Label) + "\n"
}

// renderExternalEntryRow renders a single external catalog entry.
func (m PickerModel) renderExternalEntryRow(row *outline.Row, selected bool) string {
	cursor := "    "
	if selected {
		cursor = "  " + m.styles.Accent.Render("▸ ")
	}
	icon := "○"
	iconStyle := m.styles.Dim
	if row.Attached {
		icon = "●"
		iconStyle = m.styles.Info
	}
	nameStyle := m.styles.Muted
	if selected {
		nameStyle = m.styles.Accent.Bold(true)
	}
	return "  " + cursor + iconStyle.Render(icon) + " " + nameStyle.Render(row.Label) + "\n"
}

// renderTopAction renders the top action row. Label depends on input state:
//   - empty: "+ new tmp session"
//   - typed: "+ create \"<name>\""
func (m PickerModel) renderTopAction(selected bool) string {
	query := m.state.workspaceQuery
	var label string
	if query == "" {
		label = "+ new tmp session"
	} else {
		label = "+ create \"" + query + "\""
	}
	if selected {
		return " " + m.styles.Accent.Render("▸ ") + m.styles.Accent.Bold(true).Render(label) + "\n"
	}
	return "   " + m.styles.Muted.Render(label) + "\n"
}

// renderWorkspaceRow renders a single workspace row.
func (m PickerModel) renderWorkspaceRow(ws workspaceview.WorkspaceViewModel, selected bool) string {
	cursor := "  "
	if selected {
		cursor = m.styles.Accent.Render("▸ ")
	}

	// Name with matched-char underlines. Empty workspaces (and pseudo
	// buckets) render grayed unless they're the current row.
	nameStyle := m.styles.Normal.Bold(true)
	switch {
	case selected:
		nameStyle = m.styles.Accent.Bold(true)
	case ws.IsPseudo, ws.LiveSessionCount == 0:
		nameStyle = m.styles.Dim
	}
	name := m.renderNameWithMatches(ws.Name, ws.MatchedIndexes, nameStyle)
	padding := 16 - len(ws.Name)
	if padding > 0 {
		name += strings.Repeat(" ", padding)
	}

	// Session count.
	countStr := fmt.Sprintf("%d sessions", ws.LiveSessionCount)
	if ws.LiveSessionCount == 1 {
		countStr = "1 session"
	}
	countRender := m.styles.Dim.Render(fmt.Sprintf("%-12s", countStr))

	// Root dir.
	dirStr := ""
	if ws.RootDir != "" {
		dirStr = shortenPath(ws.RootDir)
	}
	dirRender := m.styles.Dim.Render(fmt.Sprintf("%-20s", dirStr))

	// Last activity.
	activityStr := ""
	if !ws.LastActivity.IsZero() {
		activityStr = session.HumanAge(ws.LastActivity) + " ago"
	}

	// Attached indicator.
	attachedTag := ""
	if ws.HasAttached {
		attachedTag = "  " + m.styles.Info.Render("attached")
	}

	line := "  " + cursor + name + "  " + countRender + "  " + dirRender
	if activityStr != "" {
		line += "  " + m.styles.Dim.Render(activityStr)
	}
	line += attachedTag
	return line + "\n"
}

// renderSessionRow renders a session as a nested child under a workspace.
func (m PickerModel) renderSessionRow(s session.SessionInfo, selected bool) string {
	// Icon.
	icon := "○"
	iconStyle := m.styles.Dim
	if s.Attached {
		icon = "●"
		iconStyle = m.styles.Info
	}
	if selected {
		iconStyle = m.styles.Accent
	}

	nameStyle := m.styles.Normal
	if selected {
		nameStyle = m.styles.Accent.Bold(true)
	}
	if s.IsTmp && !selected {
		nameStyle = m.styles.Dim
	}

	cursor := "    "
	if selected {
		cursor = "  " + m.styles.Accent.Render("▸ ")
	}

	// Window names.
	winStr := ""
	if wins, ok := m.windows[s.Name]; ok && len(wins) > 0 {
		names := make([]string, 0, len(wins))
		for _, w := range wins {
			names = append(names, w.Name)
		}
		winStr = "[" + strings.Join(names, ", ") + "]"
	} else if s.Windows > 0 {
		winStr = fmt.Sprintf("%dw", s.Windows)
	}

	attachedTag := ""
	if s.Attached {
		attachedTag = "  " + m.styles.Info.Render("attached")
	}

	line := "  " + cursor + iconStyle.Render(icon) + " " + nameStyle.Render(fmt.Sprintf("%-14s", session.LocalDisplayName(s)))
	if winStr != "" {
		line += "  " + m.styles.Dim.Render(winStr)
	}
	line += attachedTag
	return line + "\n"
}

// renderNameWithMatches renders a name with matched-character underlines.
func (m PickerModel) renderNameWithMatches(name string, matches []int, baseStyle lipgloss.Style) string {
	if len(matches) == 0 {
		return baseStyle.Render(name)
	}
	matchSet := make(map[int]bool, len(matches))
	for _, idx := range matches {
		matchSet[idx] = true
	}
	underlineStyle := baseStyle.Underline(true)
	var b strings.Builder
	for i, r := range name {
		if matchSet[i] {
			b.WriteString(underlineStyle.Render(string(r)))
		} else {
			b.WriteString(baseStyle.Render(string(r)))
		}
	}
	return b.String()
}

// renderInlineConfirm builds the red delete prompt rendered in place of the
// cursor row. Confirm with y or ctrl+x again; any other key cancels. Relies on
// the confirm-target snapshot so the copy stays stable if an async refresh
// shifts the underlying rows mid-confirmation. `row` is the cursor row, used
// only to match the list's indent (sessions sit one level deeper).
func (m PickerModel) renderInlineConfirm(row *outline.Row) string {
	indent := "  "
	if row.Kind == outline.RowSession {
		indent = "    "
	}
	cursor := m.styles.Accent.Render("▸ ")
	keys := m.styles.Dim.Render(" (y / ctrl+x to confirm · esc cancel)")
	nameStyled := m.styles.Error.Bold(true).Render(m.confirm.name)

	// Second step: attached-workspace "this will disconnect you" warning.
	if m.mode == modeConfirmDeleteAttached {
		body := m.styles.Error.Render("⚠ ") + nameStyled +
			m.styles.Error.Render(" has live clients — disconnect & delete?")
		return indent + cursor + body + keys + "\n"
	}

	// First step: normal confirm.
	body := m.styles.Error.Render("delete "+m.confirm.kind+" ") + nameStyled
	if m.confirm.kind == "workspace" && m.confirm.liveCount > 0 {
		unit := "sessions"
		if m.confirm.liveCount == 1 {
			unit = "session"
		}
		body += m.styles.Error.Render(fmt.Sprintf(" + %d %s", m.confirm.liveCount, unit))
	}
	body += m.styles.Error.Render("?")
	return indent + cursor + body + keys + "\n"
}
