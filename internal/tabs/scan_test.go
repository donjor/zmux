package tabs

import (
	"testing"

	"github.com/donjor/zmux/internal/tmux"
)

func row(pane, session, window, tabID string, mod ...func(*tmux.LogicalPaneRow)) tmux.LogicalPaneRow {
	r := tmux.LogicalPaneRow{
		PaneID: pane, Session: session, WindowID: window, TabID: tabID,
		WindowPanes: 1,
	}
	for _, fn := range mod {
		fn(&r)
	}
	return r
}

func TestScanIgnoresRawPanes(t *testing.T) {
	tabs := FromRows([]tmux.LogicalPaneRow{
		row("%0", "work", "@0", ""),
		// window-inherited label on a raw pane must not make it a tab
		row("%1", "work", "@1", "", func(r *tmux.LogicalPaneRow) { r.Label = "ghost" }),
	})
	if len(tabs) != 0 {
		t.Fatalf("raw panes became tabs: %+v", tabs)
	}
}

func TestScanFullPlacement(t *testing.T) {
	tabs := FromRows([]tmux.LogicalPaneRow{
		row("%1", "work", "@1", "ztab_a", func(r *tmux.LogicalPaneRow) { r.Label = "buddy" }),
	})
	if len(tabs) != 1 || tabs[0].Placement != PlacementFull {
		t.Fatalf("want one full tab, got %+v", tabs)
	}
	if tabs[0].OriginSession != "work" {
		t.Errorf("origin should default to session, got %q", tabs[0].OriginSession)
	}
}

// A raw human split inside a tab's window never demotes the tab to pane-of.
func TestScanRawSiblingKeepsFull(t *testing.T) {
	tabs := FromRows([]tmux.LogicalPaneRow{
		row("%1", "work", "@1", "ztab_a", func(r *tmux.LogicalPaneRow) { r.WindowPanes = 2 }),
		row("%2", "work", "@1", "", func(r *tmux.LogicalPaneRow) { r.WindowPanes = 2 }),
	})
	if len(tabs) != 1 || tabs[0].Placement != PlacementFull {
		t.Fatalf("raw sibling demoted the tab: %+v", tabs)
	}
}

func TestScanPaneOfSharedWindow(t *testing.T) {
	// buddy joined into the work tab's window, anchored advisorily
	tabs := FromRows([]tmux.LogicalPaneRow{
		row("%1", "work", "@1", "ztab_work"),
		row("%2", "work", "@1", "ztab_buddy", func(r *tmux.LogicalPaneRow) { r.Anchor = "ztab_work" }),
	})
	if len(tabs) != 2 {
		t.Fatalf("want 2 tabs, got %d", len(tabs))
	}
	workTab, buddy := tabs[0], tabs[1]
	if workTab.Placement != PlacementFull {
		t.Errorf("host tab should be full, got %s", workTab.Placement)
	}
	if buddy.Placement != PlacementPaneOf || buddy.AnchorID != "ztab_work" {
		t.Errorf("joined tab should be pane-of host: %+v", buddy)
	}
}

// Manual join with no anchors: first managed pane (lowest index — tmux scan
// order) owns the window deterministically.
func TestScanPaneOfWithoutAnchorsIsDeterministic(t *testing.T) {
	tabs := FromRows([]tmux.LogicalPaneRow{
		row("%1", "work", "@1", "ztab_first"),
		row("%2", "work", "@1", "ztab_second"),
	})
	if tabs[0].Placement != PlacementFull || tabs[1].Placement != PlacementPaneOf {
		t.Fatalf("owner not deterministic: %+v", tabs)
	}
	if tabs[1].AnchorID != "ztab_first" {
		t.Errorf("pane-of should anchor to live owner, got %q", tabs[1].AnchorID)
	}
}

func TestScanDockPlacementAndOrigin(t *testing.T) {
	tabs := FromRows([]tmux.LogicalPaneRow{
		row("%9", DockSession, "@7", "ztab_hid", func(r *tmux.LogicalPaneRow) { r.Hidden = "work" }),
	})
	if len(tabs) != 1 || tabs[0].Placement != PlacementDock {
		t.Fatalf("want docked tab, got %+v", tabs)
	}
	if tabs[0].OriginSession != "work" || !tabs[0].InScope("work") {
		t.Errorf("docked tab must keep origin scope: %+v", tabs[0])
	}
}

func TestScanVisibilityAndActive(t *testing.T) {
	tabs := FromRows([]tmux.LogicalPaneRow{
		row("%1", "work", "@1", "ztab_a", func(r *tmux.LogicalPaneRow) {
			r.WindowActive = true
			r.PaneActive = false
		}),
	})
	if !tabs[0].Visible || tabs[0].Active {
		t.Errorf("window-active inactive pane: want visible, not active: %+v", tabs[0])
	}
}
