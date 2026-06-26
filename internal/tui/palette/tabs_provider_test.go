package palette

import (
	"testing"

	"github.com/donjor/zmux/internal/tabs"
)

func payloadIDs(as []Action) map[string]string {
	// returns map[actionID]title for the rows.
	m := map[string]string{}
	for _, a := range as {
		m[a.ID] = a.Title
	}
	return m
}

func TestTabActionsForEligibilityByPlacement(t *testing.T) {
	all := []tabs.LogicalTab{
		{ID: "ztab_full", Label: "work", Placement: tabs.PlacementFull},
		{ID: "ztab_cur", Label: "current", Placement: tabs.PlacementFull},
		{ID: "ztab_pane", Label: "peer", Placement: tabs.PlacementPaneOf},
		{ID: "ztab_dock", Label: "parked", Placement: tabs.PlacementDock},
	}
	got := payloadIDs(tabActionsFor(all, "ztab_cur"))

	want := []string{
		"tab:hide:ztab_full",  // full → hide
		"tab:pane:ztab_full",  // full (not current) → join into current
		"tab:hide:ztab_cur",   // current full → hide (allowed)
		"tab:full:ztab_pane",  // pane-of → promote
		"tab:hide:ztab_pane",  // pane-of → hide (breaks out to dock)
		"tab:show:ztab_dock",  // dock → show
	}
	for _, id := range want {
		if _, ok := got[id]; !ok {
			t.Errorf("missing row %q; got %v", id, got)
		}
	}

	// The current tab must NOT offer to join into itself.
	if _, ok := got["tab:pane:ztab_cur"]; ok {
		t.Errorf("current tab offered a join-into-self row")
	}
	// A docked tab is not hideable (already hidden) and not promotable.
	for _, bad := range []string{"tab:hide:ztab_dock", "tab:full:ztab_dock"} {
		if _, ok := got[bad]; ok {
			t.Errorf("dock tab wrongly offered %q", bad)
		}
	}
}

func TestTabActionsForNoCurrentHostOmitsJoin(t *testing.T) {
	all := []tabs.LogicalTab{{ID: "ztab_full", Label: "work", Placement: tabs.PlacementFull}}
	got := payloadIDs(tabActionsFor(all, "")) // no current host (e.g. outside tmux)
	if _, ok := got["tab:pane:ztab_full"]; ok {
		t.Errorf("join row emitted with no current host")
	}
	if _, ok := got["tab:hide:ztab_full"]; !ok {
		t.Errorf("hide row should still appear without a host")
	}
}

// TestExecutorTabPlacementStaleID covers the re-resolve guard: a payload whose
// tab vanished before execution returns PostError, not a blind op.
func TestExecutorTabPlacementStaleID(t *testing.T) {
	for _, payload := range []any{
		TabHidePayload{TabID: "ztab_gone"},
		TabShowPayload{TabID: "ztab_gone"},
		TabPromotePayload{TabID: "ztab_gone"},
		TabJoinPayload{TabID: "ztab_gone"},
	} {
		exe, _, _ := newTestExecutor(t) // empty mock → no logical tabs
		post := exe.Run(Action{Payload: payload})
		if post.Kind != PostError {
			t.Errorf("%#v: kind = %v, want PostError for missing tab", payload, post.Kind)
		}
	}
}
