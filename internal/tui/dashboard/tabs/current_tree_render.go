package tabs

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui/outline"
)

// ── Row rendering ──
//
// Layout (option C, all tabs expanded):
//
//	  myapp                               3 sessions · attached
//	  ─────────────────────────────────────────────────────────
//	  ● third                          current · 1 tab · 1 client
//	      1  bash       ~/proj       <1m  0.9%  36MB
//
//	  ○ main-test                      2 tabs
//	      1  bash       ~
//	      2  vim        ~/proj
//
//	  ○ two                            1 tab
//	      1  bash       ~
//
// The workspace banner gets a horizontal rule below it. Sessions are
// separated by blank rows (RowPlaceholder with Selectable=false).

// renderRow paints a single row with kind-specific formatting.
func (t *CurrentTab) renderRow(row *outline.Row, selected bool) string {
	switch row.Kind {
	case outline.RowWorkspaceHeader:
		return t.renderWorkspaceRow(row, selected)
	case outline.RowSession:
		return t.renderSessionRow(row, selected)
	case outline.RowWindow:
		return t.renderWindowRow(row, selected)
	case outline.RowPane:
		return t.renderPaneRow(row, selected)
	case outline.RowPlaceholder:
		if row.Label == "" {
			return "\n" // blank separator between sessions
		}
		return "        " + t.styles.Dim.Render(row.Label) + "\n"
	}
	return ""
}

// renderWorkspaceRow renders the workspace banner + a horizontal rule. The
// "N sessions in <ws>" scope meta lives in the pinned chrome above the
// viewport (renderScopeCue) so it stays visible while rows scroll; the banner
// row itself is just the actionable workspace target (rename / kill / new).
func (t *CurrentTab) renderWorkspaceRow(row *outline.Row, selected bool) string {
	cursor := "  "
	if selected {
		cursor = t.styles.Accent.Render("▸ ")
	}
	nameStyle := t.styles.Normal.Bold(true)
	if selected {
		nameStyle = t.styles.Accent.Bold(true)
	}

	line := "  " + cursor + nameStyle.Render(row.Label) + "\n"

	// Horizontal rule under the banner.
	ruleWidth := t.width - 4
	if ruleWidth < 20 {
		ruleWidth = 20
	}
	line += "  " + t.styles.Dim.Render(strings.Repeat("─", ruleWidth)) + "\n"
	return line
}

// renderSessionRow renders a session header line.
//
//	● third                current · 1 tab · 1 client   (current)
//	○ main-test            2 tabs                       (sibling)
func (t *CurrentTab) renderSessionRow(row *outline.Row, selected bool) string {
	cursor := "  "
	if selected {
		cursor = t.styles.Accent.Render("▸ ")
	}
	icon := "○"
	iconStyle := t.styles.Dim
	if row.Attached {
		icon = "●"
		iconStyle = t.styles.Info
	}
	nameStyle := t.styles.Normal.Bold(true)
	if row.Current {
		nameStyle = t.styles.Success.Bold(true)
	}
	if selected {
		iconStyle = t.styles.Accent
		nameStyle = t.styles.Accent.Bold(true)
	}

	// Quick-jump number badge (matches the 1-9 digit handler). Only the first
	// nine sessions are jumpable, so only those are numbered; others get an
	// equal-width blank so the icon column stays aligned.
	num := "    "
	if n := t.sessionNumberForRow(row.ID); n >= 1 && n <= 9 {
		numStyle := t.styles.Dim
		if selected {
			numStyle = t.styles.Accent
		}
		num = numStyle.Render(fmt.Sprintf("[%d]", n)) + " "
	}

	// Pad the name to a consistent column for alignment (visual width,
	// not byte length, so wide-rune names line up).
	padded := padVisual(row.Label, 24)

	// Meta: current gets "current · N tabs · M clients"; siblings get "N tabs".
	var meta string
	if row.Current {
		parts := []string{t.styles.Success.Render("current")}
		parts = append(parts, t.styles.Dim.Render(fmt.Sprintf("%d %s", len(t.windows), pluralize("tab", len(t.windows)))))
		if t.attached > 0 {
			parts = append(parts, t.styles.Dim.Render(fmt.Sprintf("%d %s", t.attached, pluralize("client", t.attached))))
		}
		meta = strings.Join(parts, t.styles.Dim.Render(" · "))
	} else if s, ok := outline.RowData[session.SessionInfo](row); ok && s != nil {
		tabCount := s.Windows
		if wins, ok := t.siblingWindows[s.Name]; ok {
			tabCount = len(wins)
		}
		meta = t.styles.Dim.Render(fmt.Sprintf("%d %s", tabCount, pluralize("tab", tabCount)))
		if row.Attached {
			meta += t.styles.Dim.Render(" · ") + t.styles.Info.Render("attached")
		}
	}

	return "  " + cursor + num + iconStyle.Render(icon) + " " + nameStyle.Render(padded) + "   " + meta + "\n"
}

// renderPaneRow renders a pane row nested under its window.
func (t *CurrentTab) renderPaneRow(row *outline.Row, selected bool) string {
	cursor := "          "
	if selected {
		cursor = "        " + t.styles.Accent.Render("▸ ")
	}
	p, ok := outline.RowData[tmux.Pane](row)
	if !ok || p == nil {
		return ""
	}
	idStyle := t.styles.Dim
	cmdStyle := t.styles.Dim
	if p.Active {
		cmdStyle = t.styles.Info
	}
	if row.Current {
		idStyle = t.styles.Success
		cmdStyle = t.styles.Success
	}
	if selected {
		idStyle = t.styles.Accent
		cmdStyle = t.styles.Accent.Bold(true)
	}
	title := p.Title
	if title == "" {
		title = p.Command
	}
	if title == "" {
		title = "pane"
	}
	meta := []string{fmt.Sprintf("%dx%d", p.Width, p.Height)}
	if row.Current {
		meta = append([]string{t.styles.Success.Render("you")}, meta...)
	}
	if p.Active {
		meta = append(meta, "active")
	}
	return cursor + idStyle.Render(p.ID) + " " +
		cmdStyle.Render(padVisual(title, 18)) + "  " +
		t.styles.Dim.Render(strings.Join(meta, "  ")) + "\n"
}

// renderWindowRow renders a single window line. Current-session windows
// (Data: *windowDetail) get rich CPU/mem/uptime metadata; sibling
// windows (Data: *tmux.Window) get just index + name + dir.
//
// Windows indent 2 columns deeper than their session row so they read
// as nested children.
func (t *CurrentTab) renderWindowRow(row *outline.Row, selected bool) string {
	cursor := "      "
	if selected {
		cursor = "    " + t.styles.Accent.Render("▸ ")
	}

	// In window-level nav, dim windows outside the expanded session so
	// it's visually obvious where the cursor lives.
	dimmed := t.navLevel == navLevelWindow && row.ParentID != t.expandedSessionID

	view, ok := windowViewFromRow(row)
	if !ok {
		return ""
	}
	return t.paintWindowRow(view, cursor, selected, dimmed)
}

// windowView is the subset of window data needed for rendering, hoisted
// above the windowDetail/tmux.Window split so the paint function works
// uniformly for current and sibling windows.
type windowView struct {
	Index   int
	Name    string
	Dir     string
	Active  bool
	Command string   // active-pane command (current session only)
	Meta    []string // uptime + CPU% + mem (current session only)
}

func windowViewFromRow(row *outline.Row) (windowView, bool) {
	if w, ok := outline.RowData[windowDetail](row); ok && w != nil {
		return windowView{
			Index:   w.Index,
			Name:    w.Name,
			Dir:     w.Dir,
			Active:  w.Active,
			Command: activePaneCommand(w),
			Meta:    windowStatsMeta(w),
		}, true
	}
	if w, ok := outline.RowData[tmux.Window](row); ok && w != nil {
		return windowView{Index: w.Index, Name: w.Name, Dir: w.Dir, Active: w.Active}, true
	}
	return windowView{}, false
}

// paintWindowRow renders a windowView. Padding uses lipgloss.Width so
// multi-byte or wide-rune names line up visually, not byte-wise.
func (t *CurrentTab) paintWindowRow(v windowView, cursor string, selected, dimmed bool) string {
	idxStyle := t.styles.Dim
	nameStyle := t.styles.Normal
	if v.Active {
		nameStyle = t.styles.Info
	}
	if selected {
		idxStyle = t.styles.Accent
		nameStyle = t.styles.Accent.Bold(true)
	}
	if dimmed {
		idxStyle = t.styles.Dim
		nameStyle = t.styles.Dim
	}

	line := cursor + idxStyle.Render(fmt.Sprintf("%d", v.Index)) + " " +
		nameStyle.Render(padVisual(v.Name, 20))

	if v.Dir != "" || v.Command != "" || len(v.Meta) > 0 {
		dir := shortenDir(v.Dir)
		line += "  " + t.styles.Dim.Render(padVisual(dir, 18))
	}
	if v.Command != "" && v.Command != v.Name {
		line += "  " + t.styles.Dim.Render(v.Command)
	}
	if len(v.Meta) > 0 {
		line += "  " + t.styles.Dim.Render(v.Meta[0])
		for _, m := range v.Meta[1:] {
			line += "  " + t.styles.Dim.Render(m)
		}
	}
	return line + "\n"
}

// activePaneCommand returns the command running in the window's active
// pane (falling back to the first pane's command).
func activePaneCommand(w *windowDetail) string {
	for _, p := range w.Panes {
		if p.Active {
			return p.Command
		}
	}
	if len(w.Panes) > 0 {
		return w.Panes[0].Command
	}
	return ""
}

// windowStatsMeta formats uptime / CPU / memory into a slice of display
// fragments. Skips thresholds to keep idle rows uncluttered.
func windowStatsMeta(w *windowDetail) []string {
	var meta []string
	if w.Uptime != "" {
		meta = append(meta, w.Uptime)
	}
	if w.Stats.CPU > 0.1 {
		meta = append(meta, fmt.Sprintf("%.1f%%", w.Stats.CPU))
	}
	if w.Stats.MemMB > 1.0 {
		if w.Stats.MemMB >= 1024 {
			meta = append(meta, fmt.Sprintf("%.1fGB", w.Stats.MemMB/1024))
		} else {
			meta = append(meta, fmt.Sprintf("%.0fMB", w.Stats.MemMB))
		}
	}
	return meta
}

// padVisual right-pads s to width columns using lipgloss.Width — which
// accounts for multi-byte / wide runes — so alignment survives
// non-ASCII labels.
func padVisual(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

// pluralize returns "tab" or "tabs" based on count.
func pluralize(word string, n int) string {
	if n == 1 {
		return word
	}
	return word + "s"
}
