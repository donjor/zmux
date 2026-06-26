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

// TabHidePayload parks a tab in the hidden dock.
type TabHidePayload struct{ TabID string }

// TabShowPayload returns a docked tab to its origin session.
type TabShowPayload struct{ TabID string }

// TabPromotePayload breaks a pane-of tab out into a full window.
type TabPromotePayload struct{ TabID string }

// TabJoinPayload joins a tab into the current tab as a pane.
type TabJoinPayload struct{ TabID string }

// LogicalTabProvider enumerates eligible logical tabs and emits the targetful
// hide/show/full/pane family. Host and source are current-context for v1 (join
// lands in the current tab); an arbitrary host/source picker is deferred.
type LogicalTabProvider struct {
	Runner tmux.Runner
}

// Covers declares which dynamic action specs this family surfaces. The coverage
// gate uses it so a dynamic spec is satisfied by the family's declaration, never
// by requiring live tab rows in an empty test environment.
func (p *LogicalTabProvider) Covers() []string {
	return []string{"tab.hide", "tab.show", "tab.full", "tab.pane"}
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
	if host, herr := tabs.CurrentHost(p.Runner); herr == nil {
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
			out = append(out, Action{
				ID:       "tab:show:" + t.ID,
				Group:    "Tabs",
				Title:    "Show " + name,
				Subtitle: "hidden",
				Keywords: []string{"tab", "show", "unhide", "dock", name},
				Kind:     ActionExec,
				Payload:  TabShowPayload{TabID: t.ID},
			})
		case tabs.PlacementPaneOf:
			out = append(out,
				Action{
					ID:       "tab:full:" + t.ID,
					Group:    "Tabs",
					Title:    "Promote " + name + " to a full tab",
					Keywords: []string{"tab", "full", "promote", "break", name},
					Kind:     ActionExec,
					Payload:  TabPromotePayload{TabID: t.ID},
				},
				tabHideAction(t, name),
			)
		case tabs.PlacementFull:
			out = append(out, tabHideAction(t, name))
			if currentID != "" && t.ID != currentID {
				out = append(out, Action{
					ID:       "tab:pane:" + t.ID,
					Group:    "Tabs",
					Title:    "Join " + name + " into the current tab",
					Keywords: []string{"tab", "pane", "join", "split", name},
					Kind:     ActionExec,
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
		Title:    "Hide " + name,
		Keywords: []string{"tab", "hide", "park", "dock", name},
		Kind:     ActionExec,
		Payload:  TabHidePayload{TabID: t.ID},
	}
}
