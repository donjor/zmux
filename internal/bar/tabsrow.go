package bar

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tabstate"
	"github.com/donjor/zmux/internal/theme"
	"github.com/donjor/zmux/internal/tmux"
)

// RenderTabsRow renders the logical tab row for one session from a single
// server scan: every window of the session in index order (label-aware
// names, pane-canonical state glyphs), pane-of tabs riding inside their host
// cell, and the session's docked tabs grouped in a trailing dim section —
// hidden, never invisible. The #() job re-runs every status interval, so the
// running spinner frame is picked here from the wall clock.
//
// Cells wear the preset's chrome (renderTabCell — the window-status-format
// port); prefix mirrors #{client_prefix} for the presets that tint on it.
//
// Renders for the RAW session (a grouped clone has its own current-window
// pointer); docked-tab origin matching is root-resolved by the caller.
func RenderTabsRow(p *theme.Palette, preset Preset, session, originScope string, rows []tmux.LogicalPaneRow, prefix bool, now time.Time) string {
	if session == "" {
		return ""
	}
	all := tabs.FromRows(rows)

	// Windows of this session, first-seen per window id, index order.
	type winInfo struct {
		index  int
		id     string
		name   string
		active bool
	}
	seen := make(map[string]bool)
	var wins []winInfo
	for _, r := range rows {
		if r.Session != session || seen[r.WindowID] {
			continue
		}
		seen[r.WindowID] = true
		wins = append(wins, winInfo{index: r.WindowIndex, id: r.WindowID, name: r.WindowName, active: r.WindowActive})
	}
	sort.Slice(wins, func(i, j int) bool { return wins[i].index < wins[j].index })

	fullByWin := make(map[string]*tabs.LogicalTab)
	ridersByWin := make(map[string][]*tabs.LogicalTab)
	var hidden []*tabs.LogicalTab
	for i := range all {
		t := &all[i]
		switch {
		case t.Placement == tabs.PlacementDock:
			if t.OriginSession == originScope {
				hidden = append(hidden, t)
			}
		case t.Session != session:
		case t.Placement == tabs.PlacementFull:
			fullByWin[t.WindowID] = t
		default:
			ridersByWin[t.WindowID] = append(ridersByWin[t.WindowID], t)
		}
	}

	var b strings.Builder
	for i, w := range wins {
		if i > 0 {
			b.WriteString(tabCellSep(p, preset))
		}
		name := w.name
		full := fullByWin[w.id]
		if full != nil && full.Label != "" {
			name = full.Label
		}
		// Cell body: name + glyph + riders, no bg directives so the
		// preset chrome's pill background flows through. The outer full-tab
		// range covers the whole preset cell; rider labels briefly close it and
		// install their own pane range so right-click targets the text under the
		// cursor instead of the host.
		var cell strings.Builder
		hostRange := ""
		if full != nil {
			hostRange = tabPaneRange(full.PaneID)
		}
		cell.WriteString(name)
		if full != nil {
			cell.WriteString(tabStateGlyph(p, full.State, now))
		}
		// Pane-of tabs ride inside the host cell: +name, own state glyph.
		for _, r := range ridersByWin[w.id] {
			if hostRange != "" {
				cell.WriteString(tabPaneRangeEnd())
			}
			riderRange := tabPaneRange(r.PaneID)
			cell.WriteString(riderRange)
			fmt.Fprintf(&cell, "#[fg=%s,nobold]+%s", p.Muted.Hex(), tabs.DisplayName(r))
			cell.WriteString(tabStateGlyph(p, r.State, now))
			if riderRange != "" {
				cell.WriteString(tabPaneRangeEnd())
			}
			if hostRange != "" {
				cell.WriteString(hostRange)
			}
		}
		rendered := renderTabCell(p, preset, w.index, cell.String(), w.active, prefix)
		if hostRange != "" {
			rendered = hostRange + rendered + tabPaneRangeEnd()
		}
		b.WriteString(rendered)
	}

	if len(hidden) > 0 {
		fmt.Fprintf(&b, "#[fg=%s,nobold] (dock ", p.Dim.Hex())
		for i, h := range hidden {
			if i > 0 {
				b.WriteString(" ")
			}
			hiddenRange := tabPaneRange(h.PaneID)
			b.WriteString(hiddenRange)
			// Re-dim per entry — the previous tab's state glyph fg would
			// otherwise bleed into this name. Render the 1-based dock index as
			// an accent badge so the prefix+H shortcut's numeric target is visible
			// as an affordance, not just a name prefix.
			fmt.Fprintf(&b, "#[fg=%s,bold][%d]", p.Accent.Hex(), i+1)
			fmt.Fprintf(&b, "#[fg=%s,nobold] %s~", p.Dim.Hex(), tabs.DisplayName(h))
			b.WriteString(tabStateGlyph(p, h.State, now))
			if hiddenRange != "" {
				b.WriteString(tabPaneRangeEnd())
			}
		}
		fmt.Fprintf(&b, "#[fg=%s,nobold])", p.Dim.Hex())
	}

	return b.String()
}

func tabPaneRange(paneID string) string {
	if paneID == "" {
		return ""
	}
	return fmt.Sprintf("#[range=pane|%s]", paneID)
}

func tabPaneRangeEnd() string { return "#[norange]" }

// tabStateGlyph renders a pane-canonical state as a colored glyph suffix
// with a leading space (`name ✓`, matching the old in-format fragment);
// empty when unset/invalid so stateless tabs stay untouched. The fg is NOT
// restored — every caller follows with its own style directive.
func tabStateGlyph(p *theme.Palette, raw string, now time.Time) string {
	st, err := tabstate.Parse(raw)
	if err != nil {
		return ""
	}
	glyph := stateGlyphs[st]
	if st == tabstate.StateRunning {
		glyph = SpinnerFrame(now)
	}
	return fmt.Sprintf(" #[fg=%s]%s", stateColor(p, st), glyph)
}
