package tabs

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui/outline"
)

// buildRows constructs the expanded-all-tabs session-tab layout:
//
//	workspace banner        (depth 0, Selectable)
//	(separator)             (RowPlaceholder, !Selectable)
//	current session         (depth 1, Current, Expanded)
//	  window                (depth 2, Data: *windowDetail)
//	(separator)
//	sibling session         (depth 1, Data: *session.SessionInfo)
//	  window                (depth 2, Data: *tmux.Window)
//
// Every session is expanded with its windows. The current session's
// windows carry rich windowDetail (CPU/mem/uptime); sibling windows
// carry plain tmux.Window since we don't fetch process stats for them.
func (t *CurrentTab) buildRows() []outline.Row {
	paneCount := 0
	for _, w := range t.windows {
		paneCount += len(w.Panes)
	}
	est := 4 + len(t.windows) + paneCount + 3*len(t.siblings)
	rows := make([]outline.Row, 0, est)

	// ── Workspace banner ──
	wsID := outline.WorkspaceID(t.wsName)
	rows = append(rows, outline.Row{
		ID:         wsID,
		Kind:       outline.RowWorkspaceHeader,
		Depth:      0,
		Label:      t.wsName,
		Selectable: true,
		Attached:   t.attached > 0,
		Expanded:   true,
		Data:       t.wsModel,
	})
	rows = append(rows, separatorRow("sep:ws"))

	// ── Current session + its windows ──
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
	if len(t.windows) == 0 {
		rows = append(rows, outline.Row{
			ID:       "placeholder:nowindows",
			Kind:     outline.RowPlaceholder,
			Depth:    2,
			ParentID: currID,
			Label:    "(no windows)",
		})
	}
	for i := range t.windows {
		w := t.windows[i]
		winID := outline.WindowID(t.sessionName, w.Index)
		rows = append(rows, outline.Row{
			ID:         winID,
			Kind:       outline.RowWindow,
			Depth:      2,
			ParentID:   currID,
			Label:      w.Name,
			Selectable: t.windowSelectable(currID),
			Attached:   w.Active,
			Data:       &t.windows[i],
		})
		for j := range t.windows[i].Panes {
			p := t.windows[i].Panes[j]
			rows = append(rows, outline.Row{
				ID:         outline.PaneID(t.sessionName, p.ID),
				Kind:       outline.RowPane,
				Depth:      3,
				ParentID:   winID,
				Label:      p.ID,
				Selectable: t.windowSelectable(currID),
				Current:    p.ID != "" && p.ID == currentPaneID(),
				Attached:   p.Active,
				Data:       &t.windows[i].Panes[j],
			})
		}
	}

	// ── Sibling sessions, each expanded with its windows ──
	for i := range t.siblings {
		s := t.siblings[i]
		sessID := outline.SessionID(s.Name)

		rows = append(rows, separatorRow("sep:"+s.Name))

		rows = append(rows, outline.Row{
			ID:         sessID,
			Kind:       outline.RowSession,
			Depth:      1,
			ParentID:   wsID,
			Label:      s.Name,
			Selectable: true,
			Attached:   s.Attached,
			Expanded:   true,
			Data:       &t.siblings[i],
		})

		wins := t.siblingWindows[s.Name]
		if len(wins) == 0 {
			rows = append(rows, outline.Row{
				ID:       "placeholder:" + s.Name + ":nowindows",
				Kind:     outline.RowPlaceholder,
				Depth:    2,
				ParentID: sessID,
				Label:    "(no windows)",
			})
			continue
		}
		for j := range wins {
			w := wins[j]
			rows = append(rows, outline.Row{
				ID:         outline.WindowID(s.Name, w.Index),
				Kind:       outline.RowWindow,
				Depth:      2,
				ParentID:   sessID,
				Label:      w.Name,
				Selectable: t.windowSelectable(sessID),
				Attached:   w.Active,
				Data:       &wins[j],
			})
		}
	}

	return rows
}

// windowSelectable reports whether window rows belonging to the given
// session should be selectable under the current nav level. In session
// level, windows are never selectable (the cursor hops session-to-
// session). In window level, only the expanded session's windows are
// selectable.
func (t *CurrentTab) windowSelectable(sessionRowID string) bool {
	if t.navLevel != navLevelWindow {
		return false
	}
	return sessionRowID == t.expandedSessionID
}

// separatorRow is a blank non-selectable row used between sessions.
func separatorRow(id string) outline.Row {
	return outline.Row{
		ID:         id,
		Kind:       outline.RowPlaceholder,
		Depth:      0,
		Label:      "",
		Selectable: false,
	}
}

// siblingSessionForWindow returns the sibling session owning a window row,
// or nil if the window belongs to the current session.
func (t *CurrentTab) siblingSessionForWindow(row *outline.Row) *session.SessionInfo {
	for i := range t.siblings {
		if row.ParentID == outline.SessionID(t.siblings[i].Name) {
			return &t.siblings[i]
		}
	}
	return nil
}

// windowSpec is the unified addressing for a window row. It resolves
// the owning session (current or sibling) and the window's identity,
// hiding the windowDetail-vs-tmux.Window payload split from action
// handlers.
type windowSpec struct {
	Session string
	Index   int
	Name    string
	Dir     string
}

// windowSpecFromRow extracts a windowSpec from a window row. Returns
// ok=false if the row isn't a window row or the payload is missing.
func (t *CurrentTab) windowSpecFromRow(row *outline.Row) (windowSpec, bool) {
	if row == nil || row.Kind != outline.RowWindow {
		return windowSpec{}, false
	}
	if w, ok := outline.RowData[windowDetail](row); ok && w != nil {
		return windowSpec{Session: t.sessionName, Index: w.Index, Name: w.Name, Dir: w.Dir}, true
	}
	if w, ok := outline.RowData[tmux.Window](row); ok && w != nil {
		if s := t.siblingSessionForWindow(row); s != nil {
			return windowSpec{Session: s.Name, Index: w.Index, Name: w.Name, Dir: w.Dir}, true
		}
	}
	return windowSpec{}, false
}

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

// renderWorkspaceRow renders the workspace banner + a horizontal rule.
func (t *CurrentTab) renderWorkspaceRow(row *outline.Row, selected bool) string {
	cursor := "  "
	if selected {
		cursor = t.styles.Accent.Render("▸ ")
	}
	nameStyle := t.styles.Normal.Bold(true)
	if selected {
		nameStyle = t.styles.Accent.Bold(true)
	}

	sessionCount := 1 + len(t.siblings)
	meta := fmt.Sprintf("%d %s", sessionCount, pluralize("session", sessionCount))
	if row.Attached {
		meta += " · attached"
	}

	line := "  " + cursor + nameStyle.Render(row.Label) +
		"   " + t.styles.Dim.Render(meta) + "\n"

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

	return "  " + cursor + iconStyle.Render(icon) + " " + nameStyle.Render(padded) + "   " + meta + "\n"
}

// renderWindowRow renders a single window line. Current-session windows
// (Data: *windowDetail) get rich CPU/mem/uptime metadata; sibling
// windows (Data: *tmux.Window) get just index + name + dir.
//
// Windows indent 2 columns deeper than their session row so they read
// as nested children.
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

func currentPaneID() string { return os.Getenv("TMUX_PANE") }

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
		line += "  " + t.styles.Dim.Render(strings.Join(v.Meta, "  "))
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
