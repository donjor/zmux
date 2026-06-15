package workspace

import (
	"testing"

	"github.com/donjor/zmux/internal/tmux"
)

func newCreateTestStore(t *testing.T) *Store {
	t.Helper()
	store := NewStore(newMemFS("/home/user"))
	if err := store.CreateWorkspace("dev", "/home/user/dev"); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	return store
}

func hasCall(m *tmux.MockRunner, method string) bool {
	for _, c := range m.Calls {
		if c.Method == method {
			return true
		}
	}
	return false
}

// TestCreateManagedSessionCanonical is the shared-core happy path: a canonical
// zws_<ws>__<label> session, identity metadata stamped, store record written.
func TestCreateManagedSessionCanonical(t *testing.T) {
	mock := tmux.NewMockRunner()
	store := newCreateTestStore(t)

	rec, err := CreateManagedSession(mock, store, "dev", "worker", "/home/user/dev")
	if err != nil {
		t.Fatalf("CreateManagedSession: %v", err)
	}

	want := RawSessionName("dev", "worker")
	if rec.TmuxName != want {
		t.Errorf("TmuxName = %q, want %q", rec.TmuxName, want)
	}
	if !mock.HasSession(want) {
		t.Errorf("expected tmux session %q to exist", want)
	}

	ws, _ := store.GetWorkspace("dev")
	found := false
	for _, s := range ws.Sessions {
		if s.TmuxName == want && s.Label == "worker" {
			found = true
		}
	}
	if !found {
		t.Errorf("session not recorded under dev: %+v", ws.Sessions)
	}
}

// TestCreateManagedSessionRejectsInvalidLabel guards the validation gate that
// the old raw-label dashboard create skipped entirely.
func TestCreateManagedSessionRejectsInvalidLabel(t *testing.T) {
	mock := tmux.NewMockRunner()
	store := newCreateTestStore(t)

	if _, err := CreateManagedSession(mock, store, "dev", "bad label!", "/home/user/dev"); err == nil {
		t.Fatal("expected invalid-label error")
	}
	if hasCall(mock, "NewSession") {
		t.Error("created a tmux session for an invalid label")
	}
}

// TestCreateManagedSessionCollisionLeavesNoOrphan ensures a name collision fails
// before creating anything — no NewSession, no KillSession, no store record.
func TestCreateManagedSessionCollisionLeavesNoOrphan(t *testing.T) {
	mock := tmux.NewMockRunner()
	store := newCreateTestStore(t)
	canonical := RawSessionName("dev", "worker")
	mock.Sessions = []tmux.Session{{Name: canonical}}

	if _, err := CreateManagedSession(mock, store, "dev", "worker", "/home/user/dev"); err == nil {
		t.Fatal("expected collision error")
	}
	if hasCall(mock, "NewSession") {
		t.Error("created a tmux session despite the collision")
	}
	if hasCall(mock, "KillSession") {
		t.Error("killed a session — the pre-existing one must be left untouched")
	}
	ws, _ := store.GetWorkspace("dev")
	if len(ws.Sessions) != 0 {
		t.Errorf("expected no store record on collision, got %+v", ws.Sessions)
	}
}
