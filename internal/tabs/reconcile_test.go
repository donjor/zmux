package tabs

import (
	"testing"

	"github.com/donjor/zmux/internal/tablabel"
	"github.com/donjor/zmux/internal/tmux"
)

// applied filters recorded batch writes: [scope target key value unset=…].
func applied(mock *tmux.MockRunner, key string) []tmux.MockCall {
	var out []tmux.MockCall
	for _, c := range mock.Calls {
		if c.Method == "ApplyOptions" && c.Args[2] == key {
			out = append(out, c)
		}
	}
	return out
}

// S9 c2: manual break → full tab in a window with no mirror → write it.
func TestReconcileWritesMissingMirror(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.LogicalRows = []tmux.LogicalPaneRow{
		row("%1", "work", "@1", "ztab_a", func(r *tmux.LogicalPaneRow) {
			r.Label = "buddy"
			r.State = "running"
		}),
	}
	res, err := Reconcile(mock)
	if err != nil {
		t.Fatal(err)
	}
	if res.MirrorsWritten != 1 {
		t.Fatalf("want 1 mirror write, got %+v", res)
	}
	labels := applied(mock, tablabel.Option)
	if len(labels) != 1 || labels[0].Args[0] != "-w" || labels[0].Args[1] != "@1" || labels[0].Args[3] != "buddy" {
		t.Errorf("mirror label write wrong: %v", labels)
	}
	states := applied(mock, "@zmux_state")
	if len(states) != 1 || states[0].Args[3] != "running" {
		t.Errorf("mirror state write wrong: %v", states)
	}
}

func TestReconcileLeavesHealthyMirrorAlone(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.LogicalRows = []tmux.LogicalPaneRow{
		row("%1", "work", "@1", "ztab_a", func(r *tmux.LogicalPaneRow) { r.Label = "buddy" }),
	}
	mock.WindowOptions = map[string]string{"@1\x00" + tablabel.Option: "buddy"}
	res, err := Reconcile(mock)
	if err != nil {
		t.Fatal(err)
	}
	if res != (ReconcileResult{}) {
		t.Fatalf("healthy state must be a no-op, got %+v", res)
	}
}

// S9 c1 / S5: managed pane left a multi-pane window — its mirror is stale.
func TestReconcileClearsStaleMirror(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.LogicalRows = []tmux.LogicalPaneRow{
		// raw pane left behind in the old host window, inheriting the label
		row("%2", "work", "@2", "", func(r *tmux.LogicalPaneRow) {
			r.Label = "buddy"
			r.WindowPanes = 2
		}),
		row("%3", "work", "@2", "", func(r *tmux.LogicalPaneRow) {
			r.Label = "buddy"
			r.WindowPanes = 2
		}),
		// the tab itself now lives elsewhere as a full tab with a mirror
		row("%1", "work", "@9", "ztab_a", func(r *tmux.LogicalPaneRow) { r.Label = "buddy" }),
	}
	mock.WindowOptions = map[string]string{
		"@2\x00" + tablabel.Option: "buddy",
		"@9\x00" + tablabel.Option: "buddy",
	}
	res, err := Reconcile(mock)
	if err != nil {
		t.Fatal(err)
	}
	if res.MirrorsCleared != 1 {
		t.Fatalf("want 1 stale mirror cleared, got %+v", res)
	}
	for _, c := range applied(mock, tablabel.Option) {
		if c.Args[1] == "@2" && c.Args[4] != "unset=true" {
			t.Errorf("stale window @2 must be unset, got %v", c)
		}
	}
}

// Raw pane label inherited from session/global scope (window reads empty):
// nothing of ours — never touch it.
func TestReconcileIgnoresForeignInheritedLabels(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.LogicalRows = []tmux.LogicalPaneRow{
		row("%2", "work", "@2", "", func(r *tmux.LogicalPaneRow) { r.Label = "ghost"; r.WindowPanes = 2 }),
		row("%3", "work", "@2", "", func(r *tmux.LogicalPaneRow) { r.Label = "ghost"; r.WindowPanes = 2 }),
	}
	res, err := Reconcile(mock)
	if err != nil {
		t.Fatal(err)
	}
	if res != (ReconcileResult{}) {
		t.Fatalf("foreign labels must not be touched, got %+v", res)
	}
}

// Legacy migration: window-scope label, single unmanaged pane → claim it.
func TestReconcileMigratesLegacyWindowLabel(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.LogicalRows = []tmux.LogicalPaneRow{
		row("%5", "work", "@4", "", func(r *tmux.LogicalPaneRow) { r.Label = "legacy" }),
	}
	mock.WindowOptions = map[string]string{"@4\x00" + tablabel.Option: "legacy"}
	res, err := Reconcile(mock)
	if err != nil {
		t.Fatal(err)
	}
	if res.Migrated != 1 || res.MirrorsCleared != 0 {
		t.Fatalf("want 1 migration, got %+v", res)
	}
	var stamped bool
	for _, c := range applied(mock, OptTabID) {
		if c.Args[0] == "-p" && c.Args[1] == "%5" {
			stamped = true
		}
	}
	if !stamped {
		t.Error("migration must stamp the pane with an id")
	}
}

// Ambiguous legacy window (multi-pane, no managed pane) is cleared, not
// guessed at — adopting the wrong pane is worse than dropping a mirror.
func TestReconcileAmbiguousLegacyClearsInsteadOfClaiming(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.LogicalRows = []tmux.LogicalPaneRow{
		row("%5", "work", "@4", "", func(r *tmux.LogicalPaneRow) { r.Label = "legacy"; r.WindowPanes = 2 }),
		row("%6", "work", "@4", "", func(r *tmux.LogicalPaneRow) { r.Label = "legacy"; r.WindowPanes = 2 }),
	}
	mock.WindowOptions = map[string]string{"@4\x00" + tablabel.Option: "legacy"}
	res, err := Reconcile(mock)
	if err != nil {
		t.Fatal(err)
	}
	if res.Migrated != 0 || res.MirrorsCleared != 1 {
		t.Fatalf("ambiguous window must clear, not claim: %+v", res)
	}
}

// Anchor hygiene: full tab with a leftover anchor loses it; pane-of tab with
// a wrong advisory anchor gets the live owner written back.
func TestReconcileAnchorRepairs(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.LogicalRows = []tmux.LogicalPaneRow{
		row("%1", "work", "@1", "ztab_full", func(r *tmux.LogicalPaneRow) { r.Anchor = "ztab_gone" }),
		row("%2", "work", "@2", "ztab_host"),
		row("%3", "work", "@2", "ztab_rider", func(r *tmux.LogicalPaneRow) { r.Anchor = "ztab_stale" }),
	}
	mock.WindowOptions = map[string]string{}
	res, err := Reconcile(mock)
	if err != nil {
		t.Fatal(err)
	}
	if res.AnchorsFixed != 2 {
		t.Fatalf("want 2 anchor fixes, got %+v", res)
	}
	var unsetFull, repairedRider bool
	for _, c := range applied(mock, OptAnchor) {
		if c.Args[1] == "%1" && c.Args[4] == "unset=true" {
			unsetFull = true
		}
		if c.Args[1] == "%3" && c.Args[3] == "ztab_host" {
			repairedRider = true
		}
	}
	if !unsetFull || !repairedRider {
		t.Errorf("anchor repairs missing: unsetFull=%v repairedRider=%v", unsetFull, repairedRider)
	}
}

// A tab shown by hand (raw move-window out of the dock) keeps a stale
// @zmux_hidden — clear it so scopes don't double-claim it.
func TestReconcileClearsStaleHidden(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.LogicalRows = []tmux.LogicalPaneRow{
		row("%1", "work", "@1", "ztab_a", func(r *tmux.LogicalPaneRow) { r.Hidden = "work" }),
	}
	res, err := Reconcile(mock)
	if err != nil {
		t.Fatal(err)
	}
	if res.HiddenCleared != 1 {
		t.Fatalf("want stale hidden cleared, got %+v", res)
	}
}

// S9 c3/c4: dead tabs fall out of every session MRU.
func TestReconcilePrunesMRU(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.LogicalRows = []tmux.LogicalPaneRow{
		row("%1", "work", "@1", "ztab_live"),
	}
	mock.DisplayMessageResult = "ztab_live ztab_dead"
	res, err := Reconcile(mock)
	if err != nil {
		t.Fatal(err)
	}
	if res.MRUPruned != 1 {
		t.Fatalf("want 1 MRU prune, got %+v", res)
	}
	var rewrote bool
	for _, c := range mock.Calls {
		if c.Method == "SetSessionOption" && c.Args[1] == OptMRU && c.Args[2] == "ztab_live" {
			rewrote = true
		}
	}
	if !rewrote {
		t.Error("MRU must be rewritten without the dead id")
	}
}

// Dock windows are parking, not presentation — no mirrors there.
func TestReconcileSkipsDockMirrors(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.LogicalRows = []tmux.LogicalPaneRow{
		row("%9", DockSession, "@7", "ztab_hid", func(r *tmux.LogicalPaneRow) {
			r.Label = "logs"
			r.Hidden = "work"
		}),
	}
	res, err := Reconcile(mock)
	if err != nil {
		t.Fatal(err)
	}
	if res.MirrorsWritten != 0 || res.HiddenCleared != 0 {
		t.Fatalf("docked tab must not get mirrors or lose hidden, got %+v", res)
	}
}
