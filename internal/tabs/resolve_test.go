package tabs

import (
	"errors"
	"strings"
	"testing"
)

func fixture() []LogicalTab {
	return []LogicalTab{
		{ID: "ztab_a", Label: "buddy", PaneID: "%1", Session: "work", OriginSession: "work", Placement: PlacementFull},
		{ID: "ztab_b", Label: "buddy", PaneID: "%2", Session: "other", OriginSession: "other", Placement: PlacementFull},
		{ID: "ztab_c", Label: "logs", PaneID: "%3", Session: DockSession, OriginSession: "work", Placement: PlacementDock},
		{ID: "ztab_d", Label: "unique", PaneID: "%4", Session: "third", OriginSession: "third", Placement: PlacementFull},
	}
}

func TestResolveExactID(t *testing.T) {
	got, err := Resolve(fixture(), "ztab_b", "work")
	if err != nil || got.PaneID != "%2" {
		t.Fatalf("id resolution must ignore scope: %+v %v", got, err)
	}
}

// report 039: session-group clones repeat one shared pane as a same-ID row per
// clone session. ID resolution must prefer the in-scope clone so a session-only
// caller doesn't falsely refuse a tab the user sees as local.
func TestResolveExactIDPrefersInScopeClone(t *testing.T) {
	clones := []LogicalTab{
		// Same ID under the root, then the clone — root listed first.
		{ID: "ztab_g", Label: "srv", PaneID: "%7", Session: "dev", OriginSession: "dev", Placement: PlacementFull},
		{ID: "ztab_g", Label: "srv", PaneID: "%7", Session: "dev-b", OriginSession: "dev-b", Placement: PlacementFull},
	}
	got, err := Resolve(clones, "ztab_g", "dev-b")
	if err != nil {
		t.Fatalf("clone id resolve failed: %v", err)
	}
	if !got.InScope("dev-b") {
		t.Fatalf("want the in-scope clone row (dev-b), got session %q", got.Session)
	}
}

func TestResolveLabelInScope(t *testing.T) {
	got, err := Resolve(fixture(), "buddy", "work")
	if err != nil || got.ID != "ztab_a" {
		t.Fatalf("want scope-local buddy, got %+v %v", got, err)
	}
}

func TestResolveDockedByOriginScope(t *testing.T) {
	got, err := Resolve(fixture(), "logs", "work")
	if err != nil || got.ID != "ztab_c" {
		t.Fatalf("docked tab must resolve in its origin scope: %+v %v", got, err)
	}
}

func TestResolveUniqueGlobalFallback(t *testing.T) {
	got, err := Resolve(fixture(), "unique", "work")
	if err != nil || got.ID != "ztab_d" {
		t.Fatalf("unique global label should resolve cross-scope: %+v %v", got, err)
	}
}

func TestResolveAmbiguousCrossScope(t *testing.T) {
	_, err := Resolve(fixture(), "buddy", "third")
	var amb *AmbiguousError
	if !errors.As(err, &amb) {
		t.Fatalf("duplicate labels outside scope must be ambiguous, got %v", err)
	}
	if !strings.Contains(amb.Error(), "ztab_a") || !strings.Contains(amb.Error(), "ztab_b") {
		t.Errorf("ambiguity message must list ids: %s", amb.Error())
	}
}

func TestResolveNotFound(t *testing.T) {
	if _, err := Resolve(fixture(), "nope", "work"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

// indexFixture: a "work" session with full tabs at window index 1 and 3, a
// pane-of rider in window 3, and a docked tab — plus an unrelated session's
// full tab also at index 1, to prove scoping.
func indexFixture() []LogicalTab {
	return []LogicalTab{
		{ID: "ztab_1", Label: "edit", PaneID: "%1", Session: "work", OriginSession: "work", Placement: PlacementFull, WindowIndex: 1},
		{ID: "ztab_3", Label: "serve", PaneID: "%3", Session: "work", OriginSession: "work", Placement: PlacementFull, WindowIndex: 3},
		{ID: "ztab_r", Label: "rider", PaneID: "%4", Session: "work", OriginSession: "work", Placement: PlacementPaneOf, WindowIndex: 3},
		{ID: "ztab_d", Label: "logs", PaneID: "%5", Session: DockSession, OriginSession: "work", Placement: PlacementDock, WindowIndex: 1},
		{ID: "ztab_o", Label: "other", PaneID: "%9", Session: "other", OriginSession: "other", Placement: PlacementFull, WindowIndex: 1},
	}
}

func TestTabAtIndex(t *testing.T) {
	fix := indexFixture()
	cases := []struct {
		name    string
		session string
		n       int
		want    string // PaneID, or "" for nil
	}{
		{"full at 1", "work", 1, "%1"},
		{"full at 3 is the owner not the rider", "work", 3, "%3"},
		{"renumber gap is empty", "work", 2, ""},
		{"zero is invalid", "work", 0, ""},
		{"negative is invalid", "work", -1, ""},
		{"out of range", "work", 9, ""},
		{"empty session", "", 1, ""},
		{"dock tab at win 1 not index-addressable", "work", 1, "%1"},
	}
	for _, c := range cases {
		got := TabAtIndex(fix, c.session, c.n)
		var pane string
		if got != nil {
			pane = got.PaneID
		}
		if pane != c.want {
			t.Errorf("%s: TabAtIndex(%q,%d) = %q, want %q", c.name, c.session, c.n, pane, c.want)
		}
	}
}
