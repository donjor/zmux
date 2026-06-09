package tabs

import (
	"github.com/donjor/zmux/internal/tablabel"
	"github.com/donjor/zmux/internal/tabstate"
	"github.com/donjor/zmux/internal/tmux"
)

// ReconcileResult counts what a reconcile pass changed — zero everywhere
// means tmux state and logical metadata already agreed.
type ReconcileResult struct {
	MirrorsWritten int // full tabs whose window mirror was missing/stale
	MirrorsCleared int // windows mirroring a tab they no longer host
	AnchorsFixed   int // advisory anchors cleared (full) or repaired (pane-of)
	HiddenCleared  int // stale @zmux_hidden on tabs no longer docked
	Migrated       int // legacy window-labeled windows claimed onto panes
	MRUPruned      int // sessions whose MRU lists dropped dead ids
}

// mirrorKeys are the window-scope presentation options a full tab's window
// carries. Source/at/msg state details heal on the next state write; the
// reconciler owns label and state, which the bar renders from.
var mirrorKeys = []string{
	tablabel.Option, tablabel.SourceOption,
	tabstate.OptState, tabstate.OptSource, tabstate.OptAt, tabstate.OptMsg,
}

// Reconcile repairs drift between physical tmux state and logical-tab
// metadata after moves zmux didn't make (manual join/break, killed windows,
// dead dock). One scan; tmux is physical truth throughout. It never adopts
// raw panes — the only claim is the unambiguous legacy case: a window-scoped
// label on a single-pane window whose pane is unmanaged.
//
// Run before logical listing surfaces and before/after placement verbs.
func Reconcile(r tmux.Runner) (ReconcileResult, error) {
	var res ReconcileResult
	rows, err := r.ListLogicalPaneRows()
	if err != nil {
		return res, err
	}
	tabs := FromRows(rows)
	byPane := make(map[string]*LogicalTab, len(tabs))
	for i := range tabs {
		byPane[tabs[i].PaneID] = &tabs[i]
	}

	var writes []tmux.OptionWrite

	// Windows hosting a FULL tab: mirror must match the pane-canonical
	// label/state. Owner windows in the dock stay unmirrored (dock windows
	// are parking, not presentation).
	fullByWindow := make(map[string]*LogicalTab)
	for i := range tabs {
		if tabs[i].Placement == PlacementFull {
			fullByWindow[tabs[i].WindowID] = &tabs[i]
		}
	}
	for windowID, tab := range fullByWindow {
		label, err := r.ShowWindowOption(windowID, tablabel.Option)
		if err != nil {
			return res, err
		}
		state, err := r.ShowWindowOption(windowID, tabstate.OptState)
		if err != nil {
			return res, err
		}
		if label == tab.Label && state == tab.State {
			continue
		}
		res.MirrorsWritten++
		if tab.Label != "" {
			writes = append(writes,
				tmux.OptionWrite{Scope: tmux.ScopeWindow, Target: windowID, Key: tablabel.Option, Value: tab.Label})
		}
		if state != tab.State {
			writes = append(writes, tmux.OptionWrite{
				Scope: tmux.ScopeWindow, Target: windowID, Key: tabstate.OptState,
				Value: tab.State, Unset: tab.State == "",
			})
		}
	}

	// Stale mirrors: a window-scope label with no full tab in the window.
	// Candidates surface in the scan itself — a raw pane whose merged Label
	// is non-empty inherited it from somewhere, and zmux only ever sets
	// labels at pane and window scope. Verify scope-exactly, then either
	// migrate (legacy single-pane window, unmanaged pane) or clear.
	seen := make(map[string]bool)
	for _, row := range rows {
		if row.TabID != "" || row.Label == "" || seen[row.WindowID] {
			continue
		}
		seen[row.WindowID] = true
		if row.Session == DockSession || fullByWindow[row.WindowID] != nil {
			continue
		}
		wlabel, err := r.ShowWindowOption(row.WindowID, tablabel.Option)
		if err != nil {
			return res, err
		}
		if wlabel == "" {
			continue // inherited from a scope zmux doesn't write; not ours
		}
		hostsManaged := false
		for _, other := range rows {
			if other.WindowID == row.WindowID && other.TabID != "" {
				hostsManaged = true
				break
			}
		}
		if !hostsManaged && row.WindowPanes == 1 {
			// Legacy window-labeled tab from before pane-canonical identity.
			if _, err := MigrateWindowLabel(r, row.WindowID, row.PaneID); err != nil {
				return res, err
			}
			res.Migrated++
			continue
		}
		// Mirror outlived its tab (pane moved away or died) — clear it.
		res.MirrorsCleared++
		for _, key := range mirrorKeys {
			writes = append(writes,
				tmux.OptionWrite{Scope: tmux.ScopeWindow, Target: row.WindowID, Key: key, Unset: true})
		}
	}

	// Advisory metadata: anchors only mean something while pane-of; hidden
	// origin only while docked.
	for i := range tabs {
		t := &tabs[i]
		advisory := rowAnchor(rows, t.PaneID)
		switch t.Placement {
		case PlacementFull:
			if advisory != "" {
				res.AnchorsFixed++
				writes = append(writes,
					tmux.OptionWrite{Scope: tmux.ScopePane, Target: t.PaneID, Key: OptAnchor, Unset: true})
			}
		case PlacementPaneOf:
			if advisory != t.AnchorID && t.AnchorID != "" {
				res.AnchorsFixed++
				writes = append(writes,
					tmux.OptionWrite{Scope: tmux.ScopePane, Target: t.PaneID, Key: OptAnchor, Value: t.AnchorID})
			}
		case PlacementDock:
			// anchors are stale history while docked; cheap to leave, they
			// get rewritten on the next pane placement
		}
		if t.Placement != PlacementDock && rowHidden(rows, t.PaneID) != "" {
			res.HiddenCleared++
			writes = append(writes,
				tmux.OptionWrite{Scope: tmux.ScopePane, Target: t.PaneID, Key: OptHidden, Unset: true})
		}
	}

	if err := r.ApplyOptions(writes); err != nil {
		return res, err
	}

	// MRU lists per (non-reserved) session: drop ids that no longer exist.
	liveIDs := make(map[string]bool, len(tabs))
	for _, t := range tabs {
		liveIDs[t.ID] = true
	}
	sessions := make(map[string]bool)
	for _, row := range rows {
		if !IsReservedSession(row.Session) {
			sessions[row.Session] = true
		}
	}
	for session := range sessions {
		before := ReadMRU(r, session)
		if len(before) == 0 {
			continue
		}
		dropped := false
		for _, id := range before {
			if !liveIDs[id] {
				dropped = true
				break
			}
		}
		if !dropped {
			continue
		}
		if err := PruneMRU(r, session, func(id string) bool { return liveIDs[id] }); err != nil {
			return res, err
		}
		res.MRUPruned++
	}
	return res, nil
}

func rowAnchor(rows []tmux.LogicalPaneRow, paneID string) string {
	for _, row := range rows {
		if row.PaneID == paneID {
			return row.Anchor
		}
	}
	return ""
}

func rowHidden(rows []tmux.LogicalPaneRow, paneID string) string {
	for _, row := range rows {
		if row.PaneID == paneID {
			return row.Hidden
		}
	}
	return ""
}
