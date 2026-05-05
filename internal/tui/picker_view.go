package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tui/outline"
)

// logo renders the zmux block-art banner (matches v0).
var logo = "" +
	"░█████████ ░█████████████  ░██    ░██ ░██    ░██\n" +
	"     ░███  ░██   ░██   ░██ ░██    ░██  ░██  ░██\n" +
	"   ░███    ░██   ░██   ░██ ░██    ░██   ░█████\n" +
	" ░███      ░██   ░██   ░██ ░██   ░███  ░██  ░██\n" +
	"░█████████ ░██   ░██   ░██  ░█████░██ ░██    ░██"

func (m PickerModel) View() string {
	if m.Quitting {
		return ""
	}

	var b strings.Builder

	compact := m.height < 30

	// ── Header ──
	if len(m.workspaces) == 0 && !compact {
		b.WriteString("\n")
		for _, line := range strings.Split(logo, "\n") {
			b.WriteString("  " + m.styles.Accent.Render(line) + "\n")
		}
		b.WriteString("  " + m.styles.Dim.Render(strings.Repeat("━", 56)) + "\n")
	} else if !compact {
		b.WriteString("  " + m.styles.Title.Bold(true).Render("zmux") + "\n")
		b.WriteString("  " + m.styles.Dim.Render(fmt.Sprintf("%d workspaces • prefix: ctrl+space", len(m.workspaces))) + "\n")
	}

	// ── Mode-specific content ──
	switch m.mode {
	case modeTemplateSelect:
		b.WriteString(m.viewTemplateSelect())
	case modeTemplateName:
		b.WriteString(m.viewTemplateNameInput())
	default:
		b.WriteString(m.viewList())
	}

	// ── Delete confirmation overlay ──
	if (m.mode == modeConfirmDelete || m.mode == modeConfirmDeleteAttached) && m.confirm != nil {
		b.WriteString("\n")
		prompt := m.renderDeletePrompt()
		b.WriteString(prompt + "\n")
	}

	// ── Help bar ──
	b.WriteString(m.viewHelp())

	// ── Ghost prompt ──
	sep := m.styles.Dim.Render("  " + strings.Repeat("━", 56))
	b.WriteString("\n" + sep + "\n")

	dir := "~"
	if cwd, err := os.Getwd(); err == nil {
		dir = shortenPath(cwd)
	}
	cmd := m.ghostCmd()

	dirStyle := m.styles.Muted
	chevron := m.styles.Accent.Render("❯")
	cmdStyle := m.styles.Normal
	b.WriteString("  " + dirStyle.Render(dir) + "  " + chevron + " " + cmdStyle.Render(cmd) + "\n")

	return b.String()
}

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

	for i := start; i < end; i++ {
		row := &rows[i]
		selected := i == m.tree.Cursor
		switch row.Kind {
		case outline.RowTopAction:
			b.WriteString(m.renderTopAction(selected))
		case outline.RowWorkspaceHeader:
			if ws, ok := outline.RowData[WorkspaceViewModel](row); ok && ws != nil {
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

	// Helpful empty state.
	if len(rows) <= 1 && m.state.workspaceQuery == "" && !m.state.showEmpty {
		b.WriteString(m.styles.Dim.Render("  no workspaces with live sessions (ctrl+h to show empty)") + "\n")
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
func (m PickerModel) renderWorkspaceRow(ws WorkspaceViewModel, selected bool) string {
	cursor := "  "
	if selected {
		cursor = m.styles.Accent.Render("▸ ")
	}

	// Name with matched-char underlines.
	nameStyle := m.styles.Normal.Bold(true)
	if selected {
		nameStyle = m.styles.Accent.Bold(true)
	}
	if ws.IsPseudo && !selected {
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

	line := "  " + cursor + iconStyle.Render(icon) + " " + nameStyle.Render(fmt.Sprintf("%-14s", s.Name))
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

// ── Template views ──

func (m PickerModel) viewTemplateSelect() string {
	var b strings.Builder

	label := m.styles.Accent.Bold(true).Render("  Select Template")
	b.WriteString(label + "\n\n")

	if len(m.templates) == 0 {
		b.WriteString(m.styles.Muted.Render("  No templates available") + "\n")
		return b.String()
	}

	for i, tmpl := range m.templates {
		selected := i == m.templateCursor

		cursor := "  "
		if selected {
			cursor = m.styles.Accent.Render("▸ ")
		}

		nameStyle := m.styles.Normal.Bold(true)
		if selected {
			nameStyle = m.styles.Accent.Bold(true)
		}

		line := "  " + cursor + nameStyle.Render(tmpl.Name)
		if tmpl.Description != "" {
			line += "  " + m.styles.Dim.Render(tmpl.Description)
		}
		if len(tmpl.Windows) > 0 {
			winNames := make([]string, 0, len(tmpl.Windows))
			for _, w := range tmpl.Windows {
				winNames = append(winNames, w.Name)
			}
			line += "  " + m.styles.Dim.Render("["+strings.Join(winNames, ", ")+"]")
		}

		b.WriteString(line + "\n")
	}

	b.WriteString("\n" + m.styles.Dim.Render("  enter:select  esc:cancel") + "\n")
	return b.String()
}

func (m PickerModel) viewTemplateNameInput() string {
	var b strings.Builder

	label := m.styles.Accent.Bold(true).Render("  New from Template")
	tmplName := m.styles.Info.Render(m.selectedTemplate)
	b.WriteString(label + "  " + tmplName + "\n\n")

	prompt := m.styles.Accent.Render("  name ▸ ")
	b.WriteString(prompt + m.nameInput.View() + "\n")
	b.WriteString("\n" + m.styles.Dim.Render("  enter:create  esc:back") + "\n")

	return b.String()
}

// renderDeletePrompt builds the red y/N prompt shown in the overlay for
// both confirm steps. Relies on the confirm-target snapshot so the copy
// stays stable if the cursor shifts mid-confirmation.
func (m PickerModel) renderDeletePrompt() string {
	if m.confirm == nil {
		return ""
	}
	// Second step: attached-workspace "this will detach you" warning.
	if m.mode == modeConfirmDeleteAttached {
		nameStyled := m.styles.Error.Bold(true).Render(m.confirm.name)
		lead := m.styles.Error.Render("  ⚠ ") +
			m.styles.Error.Render("workspace ") + nameStyled +
			m.styles.Error.Render(" has live clients — this will disconnect them. ") +
			m.styles.Dim.Render("(y/N)")
		return lead
	}
	// First step: normal confirm.
	nameStyled := m.styles.Error.Bold(true).Render(m.confirm.name)
	body := m.styles.Error.Render("  Delete "+m.confirm.kind+" ") + nameStyled
	if m.confirm.kind == "workspace" && m.confirm.liveCount > 0 {
		body += m.styles.Error.Render(fmt.Sprintf(" (%d live sessions)", m.confirm.liveCount))
	}
	body += m.styles.Error.Render("? ") + m.styles.Dim.Render("(y/N)")
	return body
}

// ── Help bar ──

func (m PickerModel) viewHelp() string {
	switch m.mode {
	case modeConfirmDelete:
		return m.styles.Help.Render("  y:confirm  any:cancel")
	case modeConfirmDeleteAttached:
		return m.styles.Help.Render("  y:confirm detach  any:cancel")
	case modeTemplateSelect:
		return m.styles.Help.Render("  enter:select  esc:cancel")
	case modeTemplateName:
		return m.styles.Help.Render("  enter:create  esc:back")
	}

	parts := []string{}
	row := m.tree.CurrentSelectable()

	if row == nil {
		parts = append(parts, "enter:tmp")
	} else {
		switch row.Kind {
		case outline.RowTopAction:
			if m.state.workspaceQuery == "" {
				parts = append(parts, "enter:tmp")
			} else {
				parts = append(parts, "enter:create")
			}
		case outline.RowWorkspaceHeader:
			if ws, ok := outline.RowData[WorkspaceViewModel](row); ok && ws != nil && len(ws.LiveSessions) == 0 {
				parts = append(parts, "enter:create+attach")
			} else {
				parts = append(parts, "enter:attach")
			}
			parts = append(parts, "ctrl+x:kill")
		case outline.RowSession:
			parts = append(parts, "enter:attach")
			parts = append(parts, "ctrl+x:kill")
		case outline.RowExternalGroup:
			parts = append(parts, "enter:toggle")
		case outline.RowExternalEntry:
			parts = append(parts, "enter:connect")
		}
	}

	parts = append(parts, "tab:complete")
	toggleLabel := "ctrl+h:show-empty"
	if m.state.showEmpty {
		toggleLabel = "ctrl+h:hide-empty"
	}
	parts = append(parts, toggleLabel)
	parts = append(parts, "ctrl+t:template")
	if m.state.workspaceQuery != "" || m.state.sessionQuery != "" {
		parts = append(parts, "esc:clear")
	} else {
		parts = append(parts, "esc:quit")
	}

	return m.styles.Help.Render("  " + strings.Join(parts, "  "))
}

// ── Ghost command ──

func (m PickerModel) ghostCmd() string {
	switch m.mode {
	case modeTemplateName:
		name := strings.TrimSpace(m.nameInput.Value())
		if name != "" {
			return "zmux new -t " + m.selectedTemplate + " " + name
		}
		return "zmux new -t " + m.selectedTemplate
	case modeTemplateSelect:
		if m.templateCursor < len(m.templates) {
			return "zmux new -t " + m.templates[m.templateCursor].Name
		}
		return "zmux new -t ..."
	case modeConfirmDelete, modeConfirmDeleteAttached:
		if m.confirm != nil {
			return "zmux kill " + m.confirm.name
		}
		return "zmux kill ..."
	}

	row := m.tree.CurrentSelectable()
	if row == nil {
		return "zmux new"
	}

	switch row.Kind {
	case outline.RowTopAction:
		wsQuery := strings.TrimSpace(m.state.workspaceQuery)
		sessQuery := strings.TrimSpace(m.state.sessionQuery)
		if wsQuery != "" && sessQuery != "" {
			return "zmux new " + wsQuery + " " + sessQuery
		}
		if wsQuery != "" {
			return "zmux new " + wsQuery
		}
		return "zmux new  # tmp-N session"
	case outline.RowWorkspaceHeader:
		ws, _ := outline.RowData[WorkspaceViewModel](row)
		if ws == nil {
			return "zmux"
		}
		if ws.IsPseudo {
			return "# " + ws.Name
		}
		// Session query present → "zmux new <ws> <session>".
		if m.state.sessionQuery != "" {
			return "zmux new " + ws.Name + " " + m.state.sessionQuery
		}
		if len(ws.LiveSessions) == 0 {
			return "zmux new " + ws.Name
		}
		return "zmux " + ws.Name + "  # choose session"
	case outline.RowSession:
		s, _ := outline.RowData[session.SessionInfo](row)
		if s == nil {
			return "zmux"
		}
		wsName := parentWorkspaceName(row, m.tree)
		if s.Attached {
			if wsName != "" {
				return "zmux " + wsName + " " + s.Name + "  →  " + s.Name + "-b"
			}
			return "zmux " + s.Name + "  →  " + s.Name + "-b"
		}
		if wsName != "" {
			return "zmux " + wsName + " " + s.Name
		}
		return "zmux " + s.Name
	case outline.RowExternalEntry:
		return "# " + row.Label
	case outline.RowExternalGroup:
		return "# " + row.Label
	}
	return "zmux"
}

// parentWorkspaceName returns the workspace name for a session row by
// following its ParentID up to the workspace header. Returns "" if the
// parent is missing or not a workspace header.
func parentWorkspaceName(row *outline.Row, tree *outline.Tree) string {
	if row == nil || row.ParentID == "" {
		return ""
	}
	parent, _ := tree.FindByID(row.ParentID)
	if parent == nil || parent.Kind != outline.RowWorkspaceHeader {
		return ""
	}
	if ws, ok := outline.RowData[WorkspaceViewModel](parent); ok && ws != nil {
		return ws.Name
	}
	return ""
}

// shortenPath replaces the home directory with ~ and truncates long paths.
func shortenPath(path string) string {
	home, err := os.UserHomeDir()
	if err == nil && strings.HasPrefix(path, home) {
		path = "~" + path[len(home):]
	}
	parts := strings.Split(path, string(filepath.Separator))
	if len(parts) > 3 {
		path = filepath.Join("...", parts[len(parts)-2], parts[len(parts)-1])
	}
	return path
}
