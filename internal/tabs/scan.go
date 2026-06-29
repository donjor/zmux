package tabs

import (
	"github.com/donjor/zmux/internal/tabstate"
	"github.com/donjor/zmux/internal/tmux"
)

// ListLogicalTabs scans the whole server once (list-panes -a) and returns
// every zmux-managed tab with live-computed placement. Raw panes — no
// pane-scoped @zmux_tab_id — are never tabs, even when format inheritance
// leaks a window label onto them (TabID can't leak: it is never window-set).
func ListLogicalTabs(r tmux.Runner) ([]LogicalTab, error) {
	rows, err := r.ListLogicalPaneRows()
	if err != nil {
		return nil, err
	}
	return FromRows(rows), nil
}

// FromRows computes logical tabs from an existing scan — for callers that
// need both the raw rows (window list, raw panes) and the tabs from one
// round-trip, like the bar's tabs row.
func FromRows(rows []tmux.LogicalPaneRow) []LogicalTab {
	// Managed rows per window, in scan order (tmux lists panes in index
	// order, so "first" is deterministic across scans).
	byWindow := make(map[string][]tmux.LogicalPaneRow)
	for _, row := range rows {
		if row.TabID == "" {
			continue
		}
		byWindow[row.WindowID] = append(byWindow[row.WindowID], row)
	}

	tabs := make([]LogicalTab, 0, len(byWindow))
	for _, row := range rows {
		if row.TabID == "" {
			continue
		}
		t := LogicalTab{
			ID:            row.TabID,
			Label:         row.Label,
			PaneID:        row.PaneID,
			Session:       row.Session,
			OriginSession: row.Session,
			WindowID:      row.WindowID,
			WindowIndex:   row.WindowIndex,
			WindowName:    row.WindowName,
			WindowPanes:   row.WindowPanes,
			State:         row.State,
			Visible:       row.WindowActive,
			Active:        row.WindowActive && row.PaneActive,
			Command:       row.Command,
			Dir:           row.Dir,
			Title:         row.Title,
			Activity:      row.WindowActivity,
		}
		if row.Session == DockSession {
			t.Placement = PlacementDock
			t.AnchorID = row.Anchor
			if row.Hidden != "" {
				t.OriginSession = row.Hidden
			}
		} else {
			owner := windowOwner(byWindow[row.WindowID])
			if owner.PaneID == row.PaneID {
				t.Placement = PlacementFull
			} else {
				t.Placement = PlacementPaneOf
				t.AnchorID = owner.TabID
			}
		}
		tabs = append(tabs, t)
	}
	return tabs
}

// windowOwner picks which managed pane owns (presents as) a shared window:
// the first managed pane NOT advisorily anchored to another managed tab in
// the same window, falling back to the first managed pane. Single-managed
// windows trivially own themselves — raw sibling panes never demote a tab.
func windowOwner(managed []tmux.LogicalPaneRow) tmux.LogicalPaneRow {
	if len(managed) == 1 {
		return managed[0]
	}
	inWindow := make(map[string]bool, len(managed))
	for _, row := range managed {
		inWindow[row.TabID] = true
	}
	for _, row := range managed {
		if row.Anchor == "" || !inWindow[row.Anchor] || row.Anchor == row.TabID {
			return row
		}
	}
	return managed[0]
}

// ByID returns the tab with the given id, or nil.
func ByID(tabs []LogicalTab, id string) *LogicalTab {
	for i := range tabs {
		if tabs[i].ID == id {
			return &tabs[i]
		}
	}
	return nil
}

// InScope reports whether a tab belongs to a session scope: physically in
// it, or docked with it as recorded origin.
func (t *LogicalTab) InScope(session string) bool {
	return t.Session == session || t.OriginSession == session
}

// StateOf parses the tab's raw state; ok is false when unset/invalid.
func (t *LogicalTab) StateOf() (tabstate.State, bool) {
	st, err := tabstate.Parse(t.State)
	return st, err == nil
}
