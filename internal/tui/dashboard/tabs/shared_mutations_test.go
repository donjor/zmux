package tabs

import (
	"testing"

	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/workspace"
)

// ── renameWorkspaceMutation ──

func TestRenameWorkspaceMutationNilStoreNoop(t *testing.T) {
	if err := renameWorkspaceMutation(nil, "old", "new"); err != nil {
		t.Errorf("nil store should be a no-op, got err: %v", err)
	}
}

func TestRenameWorkspaceMutationRenamesInStore(t *testing.T) {
	fs := newSessionsMemFS("/home/user")
	store := workspace.NewStore(fs)
	if err := store.CreateWorkspace("old", ""); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := renameWorkspaceMutation(store, "old", "new"); err != nil {
		t.Fatalf("rename: %v", err)
	}

	all, _ := store.ListWorkspaces()
	foundNew, foundOld := false, false
	for _, w := range all {
		if w.Name == "new" {
			foundNew = true
		}
		if w.Name == "old" {
			foundOld = true
		}
	}
	if foundOld {
		t.Errorf("old name should not be present after rename")
	}
	if !foundNew {
		t.Errorf("renamed workspace not found in store: %v", all)
	}
}

// ── renameSessionMutation ──

func mockHasCall(m *tmux.MockRunner, method string, args ...string) bool {
	for _, c := range m.Calls {
		if c.Method != method {
			continue
		}
		if len(c.Args) != len(args) {
			continue
		}
		match := true
		for i := range args {
			if c.Args[i] != args[i] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func TestRenameSessionMutationCallsRunnerThenStore(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.Sessions = []tmux.Session{{Name: "old", Windows: 1}}

	fs := newSessionsMemFS("/home/user")
	store := workspace.NewStore(fs)
	if err := store.CreateWorkspace("ws", ""); err != nil {
		t.Fatalf("create ws: %v", err)
	}
	if err := store.AddSession("ws", "old"); err != nil {
		t.Fatalf("add session: %v", err)
	}

	if err := renameSessionMutation(mock, store, "old", "new"); err != nil {
		t.Fatalf("rename: %v", err)
	}

	if !mockHasCall(mock, "RenameSession", "old", "new") {
		t.Errorf("expected RenameSession(old,new), got: %v", mock.Calls)
	}

	// Store should have the new mapping.
	if name, _ := store.WorkspaceFor("new"); name != "ws" {
		t.Errorf("expected new session mapped to ws, got %q", name)
	}
}

// ── killSessionMutation ──

func TestKillSessionMutationCallsRunner(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.Sessions = []tmux.Session{{Name: "doomed", Windows: 1}}

	if err := killSessionMutation(mock, "doomed"); err != nil {
		t.Fatalf("kill: %v", err)
	}

	if !mockHasCall(mock, "KillSession", "doomed") {
		t.Errorf("expected KillSession(doomed), got: %v", mock.Calls)
	}
}

// ── killWorkspaceMutation ──

func TestKillWorkspaceMutationKillsAllSessionsThenDeletesWorkspace(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.Sessions = []tmux.Session{
		{Name: "a", Windows: 1},
		{Name: "b", Windows: 1},
	}

	fs := newSessionsMemFS("/home/user")
	store := workspace.NewStore(fs)
	if err := store.CreateWorkspace("ws", ""); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := killWorkspaceMutation(mock, store, "ws", []string{"a", "b"}); err != nil {
		t.Fatalf("kill ws: %v", err)
	}

	if !mockHasCall(mock, "KillSession", "a") {
		t.Errorf("expected KillSession(a), got: %v", mock.Calls)
	}
	if !mockHasCall(mock, "KillSession", "b") {
		t.Errorf("expected KillSession(b), got: %v", mock.Calls)
	}

	all, _ := store.ListWorkspaces()
	for _, w := range all {
		if w.Name == "ws" {
			t.Errorf("workspace should be deleted, still present: %v", all)
		}
	}
}

func TestKillWorkspaceMutationNilStoreKillsSessionsOnly(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.Sessions = []tmux.Session{{Name: "x", Windows: 1}}

	if err := killWorkspaceMutation(mock, nil, "ws", []string{"x"}); err != nil {
		t.Fatalf("kill ws: %v", err)
	}
	if !mockHasCall(mock, "KillSession", "x") {
		t.Errorf("expected KillSession(x), got: %v", mock.Calls)
	}
}
