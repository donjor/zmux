package tabpicker

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tui/outline"
)

func (m TabPickerModel) View() tea.View {
	v := tea.NewView(m.view())
	v.AltScreen = true
	return v
}

func (m TabPickerModel) view() string {
	if m.Quitting {
		return ""
	}

	var b strings.Builder

	// Header — workspace name + the active scope.
	scope := "sessions"
	if m.nav == navTab {
		scope = "tabs · " + session.RootName(m.drilled)
	}
	header := m.wsName
	if header == "" {
		header = m.current
	}
	b.WriteString("  " + m.styles.Title.Bold(true).Render(header) +
		m.styles.Muted.Render(" "+scope) + "\n\n")

	// Input row, prefixed by the active mode.
	prefix := "▸ "
	switch m.mode {
	case tpModeNew:
		prefix = "new ▸ "
	case tpModeRename:
		prefix = "rename ▸ "
	}
	b.WriteString("  " + m.styles.Accent.Render(prefix) + m.input.View() + "\n\n")

	// Body — the outline rows.
	if len(m.tree.Rows) == 0 {
		b.WriteString(m.styles.Muted.Render("  no sessions") + "\n")
	} else {
		b.WriteString(m.renderRows())
	}

	// Help.
	b.WriteString("\n")
	b.WriteString(m.styles.Help.Render("  " + m.helpLine()))
	b.WriteString("\n")

	return b.String()
}

// renderRows renders every outline row, marking the cursor.
func (m TabPickerModel) renderRows() string {
	var b strings.Builder
	for i := range m.tree.Rows {
		row := &m.tree.Rows[i]
		selected := i == m.tree.Cursor && row.Selectable
		switch row.Kind {
		case outline.RowSession:
			b.WriteString(m.renderSessionRow(row, selected))
		case outline.RowWindow:
			b.WriteString(m.renderTabRow(row, selected))
		}
	}
	return b.String()
}

// renderSessionRow renders a session line: cursor, attach dot, name, and a
// tab-count badge.
func (m TabPickerModel) renderSessionRow(row *outline.Row, selected bool) string {
	cursor := "  "
	if selected {
		cursor = m.styles.Accent.Render("▸ ")
	}

	dot := " "
	if row.Attached {
		dot = m.styles.Success.Render("●")
	}

	nameStyle := m.styles.Normal.Bold(true)
	if selected {
		nameStyle = m.styles.Accent.Bold(true)
	}
	name := nameStyle.Render(fmt.Sprintf("%-16s", row.Label))

	count := ""
	if e := m.entryByNameConst(row.Label); e != nil {
		count = m.styles.Dim.Render(tabCountLabel(len(e.Windows)))
	}

	cur := ""
	if row.Current {
		cur = m.styles.Muted.Render(" (current)")
	}

	return fmt.Sprintf("  %s%s %s %s%s\n", cursor, dot, name, count, cur)
}

// renderTabRow renders a tab line nested under its session.
func (m TabPickerModel) renderTabRow(row *outline.Row, selected bool) string {
	cursor := "    "
	if selected {
		cursor = "  " + m.styles.Accent.Render("▸ ")
	}

	active := " "
	if row.Attached {
		active = m.styles.Success.Render("●")
	}

	idx := ""
	cmd := ""
	if t, ok := outline.RowData[tabEntry](row); ok && t != nil {
		idx = m.styles.Dim.Render(fmt.Sprintf("%d", t.Index))
		cmd = m.styles.Dim.Render(t.Command)
	}

	nameStyle := m.styles.Normal
	if selected {
		nameStyle = m.styles.Accent.Bold(true)
	}
	name := nameStyle.Render(fmt.Sprintf("%-14s", row.Label))

	return fmt.Sprintf("  %s%s %s %s  %s\n", cursor, active, idx, name, cmd)
}

// entryByNameConst is the value-receiver lookup used by the renderer.
func (m TabPickerModel) entryByNameConst(rootName string) *sessionEntry {
	for i := range m.entries {
		if session.RootName(m.entries[i].Info.Name) == rootName {
			return &m.entries[i]
		}
	}
	return nil
}

// helpLine returns the context-appropriate key hints.
func (m TabPickerModel) helpLine() string {
	switch m.mode {
	case tpModeNew:
		return "enter:create  esc:cancel"
	case tpModeRename:
		return "enter:rename  esc:cancel"
	}
	if m.nav == navTab {
		return strings.Join([]string{
			"enter:go", "h:back", "ctrl+n:new", "ctrl+r:rename", "ctrl+x:close", "</>:reorder", "esc:quit",
		}, "  ")
	}
	return strings.Join([]string{
		"enter:switch", "l:tabs", "↑/↓:move", "type:filter", "esc:quit",
	}, "  ")
}

// tabCountLabel returns "N tabs" with singular grammar.
func tabCountLabel(n int) string {
	if n == 1 {
		return "1 tab"
	}
	return fmt.Sprintf("%d tabs", n)
}
