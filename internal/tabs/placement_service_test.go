package tabs

import (
	"testing"

	"github.com/donjor/zmux/internal/tmux"
)

func currentPaneRunner(paneID string, rows []tmux.LogicalPaneRow) *tmux.MockRunner {
	mock := tmux.NewMockRunner()
	mock.InsideTmux = true
	mock.LogicalRows = rows
	mock.DisplayMessageFunc = func(_, format string) (string, error) {
		if format == "#{pane_id}" {
			return paneID + "\n", nil
		}
		return "", nil
	}
	return mock
}

func listLogicalCalls(mock *tmux.MockRunner) int {
	var n int
	for _, c := range mock.Calls {
		if c.Method == "ListLogicalPaneRows" {
			n++
		}
	}
	return n
}

func TestCurrentHostFocusedFullKeepsFullOwner(t *testing.T) {
	host := row("%1", "work", "@1", "ztab_host", func(r *tmux.LogicalPaneRow) {
		r.Label = "host"
		r.WindowPanes = 2
	})
	rider := row("%2", "work", "@1", "ztab_rider", func(r *tmux.LogicalPaneRow) {
		r.Label = "rider"
		r.Anchor = "ztab_host"
		r.WindowPanes = 2
	})
	mock := currentPaneRunner("%1", []tmux.LogicalPaneRow{host, rider})

	got, err := CurrentHost(mock)
	if err != nil {
		t.Fatalf("CurrentHost failed: %v", err)
	}
	if got.ID != "ztab_host" || got.Placement != PlacementFull {
		t.Fatalf("host = %+v, want focused full owner ztab_host", got)
	}
}

func TestCurrentHostFocusedRiderIsPaneCanonical(t *testing.T) {
	host := row("%1", "work", "@1", "ztab_host", func(r *tmux.LogicalPaneRow) {
		r.Label = "host"
		r.WindowPanes = 2
	})
	rider := row("%2", "work", "@1", "ztab_rider", func(r *tmux.LogicalPaneRow) {
		r.Label = "rider"
		r.Anchor = "ztab_host"
		r.WindowPanes = 2
	})
	mock := currentPaneRunner("%2", []tmux.LogicalPaneRow{host, rider})

	got, err := CurrentHost(mock)
	if err != nil {
		t.Fatalf("CurrentHost failed: %v", err)
	}
	if got.ID != "ztab_rider" || got.Placement != PlacementPaneOf {
		t.Fatalf("host = %+v, want focused rider ztab_rider", got)
	}
}

func TestCurrentHostDockedPaneHasNoFullOwnerButResolvesByPane(t *testing.T) {
	host := row("%1", DockSession, "@9", "ztab_host", func(r *tmux.LogicalPaneRow) {
		r.Label = "host"
		r.Hidden = "work"
		r.WindowPanes = 2
	})
	rider := row("%2", DockSession, "@9", "ztab_rider", func(r *tmux.LogicalPaneRow) {
		r.Label = "rider"
		r.Hidden = "work"
		r.WindowPanes = 2
	})
	all := FromRows([]tmux.LogicalPaneRow{host, rider})
	for _, tab := range all {
		if tab.Placement == PlacementFull {
			t.Fatalf("dock scan unexpectedly elected a full owner: %+v", all)
		}
	}
	mock := currentPaneRunner("%2", []tmux.LogicalPaneRow{host, rider})

	got, err := CurrentHost(mock)
	if err != nil {
		t.Fatalf("CurrentHost failed: %v", err)
	}
	if got.ID != "ztab_rider" || got.Placement != PlacementDock {
		t.Fatalf("host = %+v, want focused docked rider ztab_rider", got)
	}
}

func TestCurrentHostFromUsesProvidedScan(t *testing.T) {
	first := []tmux.LogicalPaneRow{
		row("%1", "work", "@1", "ztab_first", func(r *tmux.LogicalPaneRow) { r.Label = "first" }),
	}
	all := FromRows(first)
	mock := currentPaneRunner("%1", []tmux.LogicalPaneRow{
		row("%1", "work", "@1", "ztab_fresh", func(r *tmux.LogicalPaneRow) { r.Label = "fresh" }),
	})

	got, err := CurrentHostFrom(all, mock)
	if err != nil {
		t.Fatalf("CurrentHostFrom failed: %v", err)
	}
	if got.ID != "ztab_first" {
		t.Fatalf("host = %s, want id from provided scan ztab_first", got.ID)
	}
	if calls := listLogicalCalls(mock); calls != 0 {
		t.Fatalf("CurrentHostFrom rescanned %d times", calls)
	}
}

func TestCurrentHostUnmanagedFocusedPaneErrors(t *testing.T) {
	mock := currentPaneRunner("%raw", []tmux.LogicalPaneRow{
		row("%1", "work", "@1", "ztab_host", func(r *tmux.LogicalPaneRow) { r.Label = "host" }),
	})

	_, err := CurrentHost(mock)
	if err == nil {
		t.Fatal("expected unmanaged current pane to error")
	}
}
