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
