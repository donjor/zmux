package cli

import (
	"reflect"
	"testing"

	"github.com/donjor/zmux/internal/bar"
	"github.com/donjor/zmux/internal/session"
)

// A killed session's pill must drop from the top row once tmux no longer lists
// it — the store still has the record, but render filters to the live set.
func TestFilterLiveSessionsDropsDeadSessions(t *testing.T) {
	labels := []string{"main", "api", "worker"}
	targets := []string{"main", "api", "worker"}
	live := []session.SessionInfo{{Name: "main"}, {Name: "worker"}} // api was killed

	gotLabels, gotTargets := filterLiveSessions(labels, targets, live)

	wantLabels := []string{"main", "worker"}
	wantTargets := []string{"main", "worker"}
	if !reflect.DeepEqual(gotLabels, wantLabels) {
		t.Errorf("labels = %v, want %v", gotLabels, wantLabels)
	}
	if !reflect.DeepEqual(gotTargets, wantTargets) {
		t.Errorf("targets = %v, want %v", gotTargets, wantTargets)
	}
}

// A grouped clone (dev-b) keeps its root (dev) alive, so the root pill stays.
func TestFilterLiveSessionsKeepsRootForGroupedClone(t *testing.T) {
	labels := []string{"dev"}
	targets := []string{"dev"}
	live := []session.SessionInfo{{Name: "dev-b"}} // only the clone is live

	gotLabels, gotTargets := filterLiveSessions(labels, targets, live)
	if !reflect.DeepEqual(gotLabels, []string{"dev"}) || !reflect.DeepEqual(gotTargets, []string{"dev"}) {
		t.Errorf("grouped clone should keep root pill: labels=%v targets=%v", gotLabels, gotTargets)
	}
}

// On an empty live set every session is filtered out — the caller's
// single-session fallback then restores the current session.
func TestFilterLiveSessionsEmptyLiveDropsAll(t *testing.T) {
	labels, targets := filterLiveSessions([]string{"a", "b"}, []string{"a", "b"}, nil)
	if len(labels) != 0 || len(targets) != 0 {
		t.Errorf("empty live set should drop all, got labels=%v targets=%v", labels, targets)
	}
}

// Attach states are index-aligned to the (filtered) target list and computed
// from the shared live set: the current session is left Unknown, an attached
// sibling is AttachLocal, a dead one stays Unknown.
func TestAttachStatesForUsesLiveSubset(t *testing.T) {
	live := []session.SessionInfo{
		{Name: "main", Attached: true},
		{Name: "api", AttachedClients: 0},
	}
	states := attachStatesFor(live, []string{"main", "api"}, "api")

	want := []bar.AttachState{bar.AttachLocal, bar.AttachUnknown}
	if !reflect.DeepEqual(states, want) {
		t.Errorf("states = %v, want %v", states, want)
	}
}
