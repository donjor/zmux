package palette

import (
	"testing"

	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tmux"
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
		"tab:pane:ztab_full", // full (not current) → join into current
		"tab:full:ztab_pane", // pane-of → promote
		"tab:hide:ztab_pane", // pane-of → hide under parent
		"tab:show:ztab_dock", // hidden pane → join back
		"tab:full:ztab_dock", // hidden pane → promote to full
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
	// Full tabs are not hideable; hidden panes are already hidden.
	for _, bad := range []string{"tab:hide:ztab_full", "tab:hide:ztab_cur", "tab:hide:ztab_dock"} {
		if _, ok := got[bad]; ok {
			t.Errorf("wrongly offered %q", bad)
		}
	}
}

func TestTabActionsForNoCurrentHostOmitsJoin(t *testing.T) {
	all := []tabs.LogicalTab{{ID: "ztab_full", Label: "work", Placement: tabs.PlacementFull}}
	got := payloadIDs(tabActionsFor(all, "")) // no current host (e.g. outside tmux)
	if _, ok := got["tab:pane:ztab_full"]; ok {
		t.Errorf("join row emitted with no current host")
	}
	if _, ok := got["tab:hide:ztab_full"]; ok {
		t.Errorf("full-tab hide row should not appear without a host")
	}
}

func paletteRow(pane, session, window, tabID, label string, mod ...func(*tmux.LogicalPaneRow)) tmux.LogicalPaneRow {
	r := tmux.LogicalPaneRow{
		PaneID:      pane,
		Session:     session,
		WindowID:    window,
		TabID:       tabID,
		Label:       label,
		WindowPanes: 1,
	}
	for _, fn := range mod {
		fn(&r)
	}
	return r
}

func paletteDisplayPane(paneID string) func(string, string) (string, error) {
	return func(_, format string) (string, error) {
		if format == "#{pane_id}" {
			return paneID + "\n", nil
		}
		return "", nil
	}
}

func paletteCallCount(mock *tmux.MockRunner, method string) int {
	var n int
	for _, c := range mock.Calls {
		if c.Method == method {
			n++
		}
	}
	return n
}

func TestLogicalTabProviderUsesSameScanForCurrentHost(t *testing.T) {
	first := []tmux.LogicalPaneRow{
		paletteRow("%1", "work", "@1", "ztab_current", "current"),
		paletteRow("%2", "work", "@2", "ztab_other", "other"),
	}
	second := []tmux.LogicalPaneRow{
		paletteRow("%1", "work", "@1", "ztab_wrong", "wrong"),
		paletteRow("%2", "work", "@2", "ztab_other", "other"),
	}
	mock := tmux.NewMockRunner()
	mock.InsideTmux = true
	mock.LogicalRowsByCall = [][]tmux.LogicalPaneRow{first, second}
	mock.DisplayMessageFunc = paletteDisplayPane("%1")

	actions, err := (&LogicalTabProvider{Runner: mock}).Actions()
	if err != nil {
		t.Fatalf("Actions failed: %v", err)
	}
	got := payloadIDs(actions)
	if _, ok := got["tab:pane:ztab_current"]; ok {
		t.Fatalf("provider emitted a join-into-self row from a skewed host scan: %v", got)
	}
	if calls := paletteCallCount(mock, "ListLogicalPaneRows"); calls != 1 {
		t.Fatalf("provider scanned %d times, want one coherent scan", calls)
	}
}

func TestLogicalTabProviderFocusedRiderCanJoinWindowOwnerIntoRider(t *testing.T) {
	host := paletteRow("%1", "work", "@1", "ztab_host", "host", func(r *tmux.LogicalPaneRow) {
		r.WindowPanes = 2
	})
	rider := paletteRow("%2", "work", "@1", "ztab_rider", "rider", func(r *tmux.LogicalPaneRow) {
		r.WindowPanes = 2
		r.Anchor = "ztab_host"
	})
	other := paletteRow("%3", "work", "@2", "ztab_other", "other")
	mock := tmux.NewMockRunner()
	mock.InsideTmux = true
	mock.LogicalRows = []tmux.LogicalPaneRow{host, rider, other}
	mock.DisplayMessageFunc = paletteDisplayPane("%2")

	actions, err := (&LogicalTabProvider{Runner: mock}).Actions()
	if err != nil {
		t.Fatalf("Actions failed: %v", err)
	}
	got := payloadIDs(actions)
	if _, ok := got["tab:pane:ztab_host"]; !ok {
		t.Fatalf("focused rider should allow joining the window owner into it; got %v", got)
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
