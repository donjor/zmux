package picker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/donjor/zmux/internal/tui/outline"
	"github.com/donjor/zmux/internal/tui/workspaceview"
)

// logo renders the zmux block-art banner (matches v0).
var logo = "" +
	"░█████████ ░█████████████  ░██    ░██ ░██    ░██\n" +
	"     ░███  ░██   ░██   ░██ ░██    ░██  ░██  ░██\n" +
	"   ░███    ░██   ░██   ░██ ░██    ░██   ░█████\n" +
	" ░███      ░██   ░██   ░██ ░██   ░███  ░██  ░██\n" +
	"░█████████ ░██   ░██   ░██  ░█████░██ ░██    ░██"

// View renders the picker's outer chrome (logo, list/dialog, help bar,
// ghost prompt). Mode-specific bodies live in picker_view_list.go and
// picker_view_templates.go; the help bar and ghost prompt live in
// picker_view_help.go.
func (m PickerModel) View() tea.View {
	v := tea.NewView(m.view())
	v.AltScreen = true
	return v
}

func (m PickerModel) view() string {
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

	// Delete confirmation renders inline on the cursor row (see viewList),
	// not as a detached overlay.

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
	if ws, ok := outline.RowData[workspaceview.WorkspaceViewModel](parent); ok && ws != nil {
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
