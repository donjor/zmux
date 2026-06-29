package palette

import (
	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tmux"
)

// ── Logical-tab payloads (dynamic family) ──
//
// Each carries the stable tab id (@zmux_tab_id). The executor re-scans and
// matches by id at run time, so a tab that moved or closed between palette-open
// and execution is handled cleanly rather than acted on stale.

// TabHidePayload parks a joined pane under its parent.
type TabHidePayload struct{ TabID string }

// TabShowPayload rejoins a hidden pane under its recorded parent.
type TabShowPayload struct{ TabID string }

// TabPromotePayload breaks a visible or hidden pane-tab out into a full window.
type TabPromotePayload struct{ TabID string }

// TabJoinPayload joins a tab into the current tab as a pane.
type TabJoinPayload struct{ TabID string }

// LogicalTabProvider enumerates eligible logical tabs and emits the targetful
// hide/show/full/pane family. Host and source are current-context for v1 (join
// lands in the current tab); an arbitrary host/source picker is deferred.
type LogicalTabProvider struct {
	Runner tmux.Runner
}

func (p *LogicalTabProvider) Actions() ([]Action, error) {
	all, err := tabs.ListLogicalTabs(p.Runner)
	if err != nil {
		return nil, err
	}
	// The current host tab is the join destination, excluded from its own "join
	// into current" row. Best-effort: outside tmux there is simply no host, so
	// join rows are omitted rather than erroring.
	currentID := ""
	if host, herr := tabs.CurrentHostFrom(all, p.Runner); herr == nil {
		currentID = host.ID
	}
	return tabActionsFor(all, currentID), nil
}

// tabActionsFor builds the per-tab rows from a tab list and the current host id.
// Pure (no IO) so eligibility is unit-testable without scanning a live server.
func tabActionsFor(all []tabs.LogicalTab, currentID string) []Action {
	var out []Action
	for i := range all {
		t := &all[i]
		name := tabs.DisplayName(t)
		switch t.Placement {
		case tabs.PlacementDock:
			out = append(
				out,
				Action{
					ID:       "tab:show:" + t.ID,
					Group:    "Tabs",
					Title:    "Join back " + name,
					Subtitle: "hidden pane",
					Keywords: []string{"tab", "show", "unhide", "join", "parked", name},
					Kind:     ActionExec,
					Covers:   "tab.show",
					Payload:  TabShowPayload{TabID: t.ID},
				},
				Action{
					ID:       "tab:full:" + t.ID,
					Group:    "Tabs",
					Title:    "Promote " + name + " to a full tab",
					Subtitle: "hidden pane",
					Keywords: []string{"tab", "full", "promote", "parked", name},
					Kind:     ActionExec,
					Covers:   "tab.full",
					Payload:  TabPromotePayload{TabID: t.ID},
				},
			)
		case tabs.PlacementPaneOf:
			out = append(
				out,
				Action{
					ID:       "tab:full:" + t.ID,
					Group:    "Tabs",
					Title:    "Promote " + name + " to a full tab",
					Keywords: []string{"tab", "full", "promote", "break", name},
					Kind:     ActionExec,
					Covers:   "tab.full",
					Payload:  TabPromotePayload{TabID: t.ID},
				},
				tabHideAction(t, name),
			)
		case tabs.PlacementFull:
			if currentID != "" && t.ID != currentID {
				out = append(out, Action{
					ID:       "tab:pane:" + t.ID,
					Group:    "Tabs",
					Title:    "Join " + name + " into the current tab",
					Keywords: []string{"tab", "pane", "join", "split", name},
					Kind:     ActionExec,
					Covers:   "tab.pane",
					Payload:  TabJoinPayload{TabID: t.ID},
				})
			}
		}
	}
	return out
}

func tabHideAction(t *tabs.LogicalTab, name string) Action {
	return Action{
		ID:       "tab:hide:" + t.ID,
		Group:    "Tabs",
		Title:    "Hide pane " + name,
		Keywords: []string{"tab", "hide", "park", "pane", name},
		Kind:     ActionExec,
		Covers:   "tab.hide",
		Payload:  TabHidePayload{TabID: t.ID},
	}
}
