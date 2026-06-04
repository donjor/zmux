package bar

import "testing"

func TestCompactDots_EmptyWhenSingleSession(t *testing.T) {
	got := CompactDots([]string{"main"}, "main", nil)
	if got != "" {
		t.Errorf("single session should produce empty dots, got %q", got)
	}
}

func TestCompactDots_CurrentIsFilled(t *testing.T) {
	got := CompactDots([]string{"main", "api"}, "main", nil)
	if got != "●○" {
		t.Errorf("nil states: want %q, got %q", "●○", got)
	}
}

func TestCompactDots_AttachedElsewhereGetsRing(t *testing.T) {
	// Three siblings: current=main, api attached elsewhere, db inactive.
	states := []AttachState{
		AttachUnknown, // main — current; state value is ignored for the current pill
		AttachLocal,   // api — attached by another client
		AttachUnknown, // db — exists, no client
	}
	got := CompactDots([]string{"main", "api", "db"}, "main", states)
	if got != "●◉○" {
		t.Errorf("with states: want %q, got %q", "●◉○", got)
	}
}

func TestCompactDots_RemoteRendersSameRingAsLocal(t *testing.T) {
	// AttachRemote is reserved for the SSH future. Until the renderer grows
	// a distinct glyph, it must render the same ring as Local so users
	// still see "attached somewhere" without a regression.
	states := []AttachState{AttachUnknown, AttachRemote}
	got := CompactDots([]string{"main", "api"}, "main", states)
	if got != "●◉" {
		t.Errorf("remote state: want %q (same as local for now), got %q", "●◉", got)
	}
}

func TestCompactDots_NilStatesFallsBackToBinary(t *testing.T) {
	// Callers that don't populate states (e.g. the dashboard bar preview)
	// must keep working — every non-current sibling renders as `○`.
	got := CompactDots([]string{"main", "api", "db"}, "api", nil)
	if got != "○●○" {
		t.Errorf("nil states: want %q, got %q", "○●○", got)
	}
}

func TestCompactDots_ShortStatesIsSafe(t *testing.T) {
	// states slice may be shorter than sessions (defensive guard). Indices
	// beyond states should be treated as Unknown.
	got := CompactDots([]string{"main", "api", "db"}, "main", []AttachState{AttachUnknown})
	if got != "●○○" {
		t.Errorf("short states: want %q, got %q", "●○○", got)
	}
}
