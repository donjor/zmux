package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// memFS is an in-memory FS for testing workspace store operations.
type memFS struct {
	files   map[string][]byte
	homeDir string
}

func newMemFS(home string) *memFS {
	return &memFS{files: make(map[string][]byte), homeDir: home}
}

func (m *memFS) ReadFile(path string) ([]byte, error) {
	data, ok := m.files[path]
	if !ok {
		return nil, &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}
	}
	return data, nil
}
func (m *memFS) WriteFile(path string, data []byte, _ os.FileMode) error {
	m.files[path] = data
	return nil
}
func (m *memFS) MkdirAll(_ string, _ os.FileMode) error { return nil }
func (m *memFS) Stat(path string) (os.FileInfo, error) {
	if _, ok := m.files[path]; ok {
		return fakeInfo{name: path}, nil
	}
	return nil, &os.PathError{Op: "stat", Path: path, Err: os.ErrNotExist}
}
func (m *memFS) UserHomeDir() (string, error) { return m.homeDir, nil }
func (m *memFS) Glob(_ string) ([]string, error) { return nil, nil }

type fakeInfo struct{ name string }

func (f fakeInfo) Name() string      { return f.name }
func (f fakeInfo) Size() int64       { return 0 }
func (f fakeInfo) Mode() os.FileMode { return 0o644 }
func (f fakeInfo) ModTime() time.Time { return time.Time{} }
func (f fakeInfo) IsDir() bool       { return false }
func (f fakeInfo) Sys() any          { return nil }

func newTestStore() (*Store, *memFS) {
	fs := newMemFS("/home/user")
	return NewStore(fs), fs
}

func storeFilePath() string {
	return filepath.Join("/home/user", ".zmux", "workspaces.toml")
}

// ── Load (v2) ──

func TestLoadEmptyWhenFileAbsent(t *testing.T) {
	store, _ := newTestStore()
	st, err := store.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st.Version != 2 {
		t.Errorf("expected version 2, got %d", st.Version)
	}
	if len(st.Workspaces) != 0 {
		t.Errorf("expected empty workspaces, got %v", st.Workspaces)
	}
}

func TestLoadV2FromFile(t *testing.T) {
	store, fs := newTestStore()
	fs.files[storeFilePath()] = []byte(`version = 2

[workspaces.myapp]
root_dir = "/home/user/src/myapp"
last_active_session = "main"
sessions = ["main", "server"]
`)
	st, err := store.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st.Version != 2 {
		t.Errorf("version = %d; want 2", st.Version)
	}
	ws := st.Workspaces["myapp"]
	if ws == nil {
		t.Fatal("expected myapp workspace")
	}
	if ws.Name != "myapp" {
		t.Errorf("Name = %q; want myapp", ws.Name)
	}
	if ws.RootDir != "/home/user/src/myapp" {
		t.Errorf("RootDir = %q; want /home/user/src/myapp", ws.RootDir)
	}
	if ws.LastActiveSession != "main" {
		t.Errorf("LastActiveSession = %q; want main", ws.LastActiveSession)
	}
	if len(ws.Sessions) != 2 || ws.Sessions[0] != "main" || ws.Sessions[1] != "server" {
		t.Errorf("Sessions = %v; want [main server]", ws.Sessions)
	}
}

// ── Migration from v1 ──

func TestMigrateV1ToV2(t *testing.T) {
	store, fs := newTestStore()
	fs.files[storeFilePath()] = []byte("[sessions]\ndev = \"bridge\"\nmonitor = \"bridge\"\n")

	st, err := store.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st.Version != 2 {
		t.Errorf("version = %d; want 2", st.Version)
	}
	ws := st.Workspaces["bridge"]
	if ws == nil {
		t.Fatal("expected bridge workspace after migration")
	}
	if len(ws.Sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d: %v", len(ws.Sessions), ws.Sessions)
	}
	// Sessions should be sorted.
	if ws.Sessions[0] != "dev" || ws.Sessions[1] != "monitor" {
		t.Errorf("Sessions = %v; want [dev monitor]", ws.Sessions)
	}
}

func TestMigrateV1AutoSavesV2(t *testing.T) {
	store, fs := newTestStore()
	fs.files[storeFilePath()] = []byte("[sessions]\ndev = \"bridge\"\n")

	_, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}

	// File should now be v2 format.
	data := string(fs.files[storeFilePath()])
	if !strings.Contains(data, "version = 2") {
		t.Error("expected v2 format after migration save")
	}
}

func TestLoadV2DoesNotMigrate(t *testing.T) {
	store, fs := newTestStore()
	v2Content := `version = 2

[workspaces.myapp]
sessions = ["main"]
`
	fs.files[storeFilePath()] = []byte(v2Content)

	st, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if st.Version != 2 {
		t.Errorf("version = %d; want 2", st.Version)
	}
	if len(st.Workspaces) != 1 {
		t.Errorf("expected 1 workspace, got %d", len(st.Workspaces))
	}
}

// ── CreateWorkspace ──

func TestCreateWorkspace(t *testing.T) {
	store, _ := newTestStore()
	if err := store.CreateWorkspace("myapp", "/home/user/src/myapp"); err != nil {
		t.Fatal(err)
	}
	ws, err := store.GetWorkspace("myapp")
	if err != nil {
		t.Fatal(err)
	}
	if ws == nil {
		t.Fatal("expected workspace to be created")
	}
	if ws.RootDir != "/home/user/src/myapp" {
		t.Errorf("RootDir = %q; want /home/user/src/myapp", ws.RootDir)
	}
}

func TestCreateWorkspaceDuplicate(t *testing.T) {
	store, _ := newTestStore()
	_ = store.CreateWorkspace("myapp", "")
	err := store.CreateWorkspace("myapp", "")
	if err == nil {
		t.Error("expected error creating duplicate workspace")
	}
}

// ── DeleteWorkspace ──

func TestDeleteWorkspace(t *testing.T) {
	store, _ := newTestStore()
	_ = store.CreateWorkspace("myapp", "")
	if err := store.DeleteWorkspace("myapp"); err != nil {
		t.Fatal(err)
	}
	ws, _ := store.GetWorkspace("myapp")
	if ws != nil {
		t.Error("expected workspace to be deleted")
	}
}

func TestDeleteWorkspaceNotFound(t *testing.T) {
	store, _ := newTestStore()
	err := store.DeleteWorkspace("nonexistent")
	if err == nil {
		t.Error("expected error deleting nonexistent workspace")
	}
}

// ── RenameWorkspace ──

func TestRenameWorkspace(t *testing.T) {
	store, _ := newTestStore()
	_ = store.CreateWorkspace("old", "")
	_ = store.AddSession("old", "dev")

	if err := store.RenameWorkspace("old", "new"); err != nil {
		t.Fatal(err)
	}

	ws, _ := store.GetWorkspace("new")
	if ws == nil {
		t.Fatal("expected renamed workspace")
	}
	if len(ws.Sessions) != 1 || ws.Sessions[0] != "dev" {
		t.Errorf("sessions lost after rename: %v", ws.Sessions)
	}

	old, _ := store.GetWorkspace("old")
	if old != nil {
		t.Error("expected old workspace to be gone")
	}
}

// ── AddSession ──

func TestAddSession(t *testing.T) {
	store, _ := newTestStore()
	_ = store.CreateWorkspace("myapp", "")
	if err := store.AddSession("myapp", "main"); err != nil {
		t.Fatal(err)
	}
	sessions := store.SessionsIn("myapp")
	if len(sessions) != 1 || sessions[0] != "main" {
		t.Errorf("SessionsIn = %v; want [main]", sessions)
	}
}

func TestAddSessionCreatesWorkspace(t *testing.T) {
	store, _ := newTestStore()
	if err := store.AddSession("newws", "dev"); err != nil {
		t.Fatal(err)
	}
	ws, _ := store.GetWorkspace("newws")
	if ws == nil {
		t.Fatal("expected workspace to be auto-created")
	}
}

func TestAddSessionAlreadyInSameWorkspace(t *testing.T) {
	store, _ := newTestStore()
	_ = store.AddSession("myapp", "dev")
	// Adding again should be a noop.
	if err := store.AddSession("myapp", "dev"); err != nil {
		t.Fatalf("expected noop, got error: %v", err)
	}
}

func TestAddSessionAlreadyInOtherWorkspace(t *testing.T) {
	store, _ := newTestStore()
	_ = store.AddSession("myapp", "dev")
	err := store.AddSession("other", "dev")
	if err == nil {
		t.Error("expected error when session is in another workspace")
	}
}

// ── RemoveSession ──

func TestRemoveSession(t *testing.T) {
	store, _ := newTestStore()
	_ = store.AddSession("myapp", "dev")
	_ = store.AddSession("myapp", "server")

	if err := store.RemoveSession("dev"); err != nil {
		t.Fatal(err)
	}

	sessions := store.SessionsIn("myapp")
	if len(sessions) != 1 || sessions[0] != "server" {
		t.Errorf("SessionsIn = %v; want [server]", sessions)
	}
}

func TestRemoveSessionLeavesWorkspace(t *testing.T) {
	store, _ := newTestStore()
	_ = store.AddSession("myapp", "dev")

	if err := store.RemoveSession("dev"); err != nil {
		t.Fatal(err)
	}

	// Workspace should still exist (empty but persistent).
	ws, _ := store.GetWorkspace("myapp")
	if ws == nil {
		t.Error("expected workspace to persist after last session removed")
	}
}

// ── MoveSession ──

func TestMoveSession(t *testing.T) {
	store, _ := newTestStore()
	_ = store.AddSession("myapp", "dev")
	_ = store.CreateWorkspace("other", "")

	if err := store.MoveSession("dev", "other"); err != nil {
		t.Fatal(err)
	}

	if sessions := store.SessionsIn("myapp"); len(sessions) != 0 {
		t.Errorf("expected myapp empty, got %v", sessions)
	}
	if sessions := store.SessionsIn("other"); len(sessions) != 1 || sessions[0] != "dev" {
		t.Errorf("expected other to have dev, got %v", sessions)
	}
}

// ── RenameSession ──

func TestRenameSession(t *testing.T) {
	store, _ := newTestStore()
	_ = store.AddSession("myapp", "dev")
	_ = store.SetLastActive("myapp", "dev")

	if err := store.RenameSession("dev", "development"); err != nil {
		t.Fatal(err)
	}

	sessions := store.SessionsIn("myapp")
	if len(sessions) != 1 || sessions[0] != "development" {
		t.Errorf("SessionsIn = %v; want [development]", sessions)
	}

	// Last active should also be updated.
	ws, _ := store.GetWorkspace("myapp")
	if ws.LastActiveSession != "development" {
		t.Errorf("LastActiveSession = %q; want development", ws.LastActiveSession)
	}
}

// ── WorkspaceFor with grouped sessions ──

func TestWorkspaceForGroupedSession(t *testing.T) {
	store, _ := newTestStore()
	_ = store.AddSession("bridge", "dev")

	ws, ok := store.WorkspaceFor("dev-b")
	if !ok || ws != "bridge" {
		t.Errorf("WorkspaceFor(dev-b) = %q, %v; want bridge, true", ws, ok)
	}
}

func TestWorkspaceForUntaggedReturnsNotFound(t *testing.T) {
	store, _ := newTestStore()
	_, ok := store.WorkspaceFor("untagged")
	if ok {
		t.Error("expected untagged session to not be found")
	}
}

// ── SessionsIn ──

func TestSessionsInReturnsOrdered(t *testing.T) {
	store, _ := newTestStore()
	_ = store.AddSession("myapp", "main")
	_ = store.AddSession("myapp", "server")
	_ = store.AddSession("myapp", "claude")

	got := store.SessionsIn("myapp")
	// Order is insertion order.
	want := []string{"main", "server", "claude"}
	if len(got) != len(want) {
		t.Fatalf("SessionsIn = %v; want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("SessionsIn[%d] = %q; want %q", i, got[i], want[i])
		}
	}
}

// ── SessionPosition ──

func TestSessionPosition(t *testing.T) {
	store, _ := newTestStore()
	_ = store.AddSession("myapp", "main")
	_ = store.AddSession("myapp", "server")
	_ = store.AddSession("myapp", "claude")

	pos, count, ok := store.SessionPosition("server")
	if !ok {
		t.Fatal("expected session to be found")
	}
	if pos != 2 || count != 3 {
		t.Errorf("pos=%d, count=%d; want 2, 3", pos, count)
	}
}

func TestSessionPositionGroupedSession(t *testing.T) {
	store, _ := newTestStore()
	_ = store.AddSession("myapp", "main")

	pos, count, ok := store.SessionPosition("main-b")
	if !ok {
		t.Fatal("expected grouped session to resolve")
	}
	if pos != 1 || count != 1 {
		t.Errorf("pos=%d, count=%d; want 1, 1", pos, count)
	}
}

func TestSessionPositionNotFound(t *testing.T) {
	store, _ := newTestStore()
	_, _, ok := store.SessionPosition("nonexistent")
	if ok {
		t.Error("expected not found")
	}
}

// ── SetLastActive ──

func TestSetLastActive(t *testing.T) {
	store, _ := newTestStore()
	_ = store.AddSession("myapp", "main")
	_ = store.AddSession("myapp", "server")

	if err := store.SetLastActive("myapp", "server"); err != nil {
		t.Fatal(err)
	}

	ws, _ := store.GetWorkspace("myapp")
	if ws.LastActiveSession != "server" {
		t.Errorf("LastActiveSession = %q; want server", ws.LastActiveSession)
	}
}

// ── Reconcile ──

func TestReconcileRemovesDeadSessions(t *testing.T) {
	store, _ := newTestStore()
	_ = store.AddSession("myapp", "dev")
	_ = store.AddSession("myapp", "dead")
	_ = store.AddSession("other", "zmux")

	live := map[string]bool{"dev": true, "zmux": true}
	if err := store.Reconcile(live); err != nil {
		t.Fatal(err)
	}

	sessions := store.SessionsIn("myapp")
	if len(sessions) != 1 || sessions[0] != "dev" {
		t.Errorf("expected [dev], got %v", sessions)
	}
}

func TestReconcileKeepsEmptyWorkspaces(t *testing.T) {
	store, _ := newTestStore()
	_ = store.AddSession("myapp", "dev")

	// Kill the only session.
	live := map[string]bool{"other": true}
	_ = store.Reconcile(live)

	// Workspace should persist (empty).
	ws, _ := store.GetWorkspace("myapp")
	if ws == nil {
		t.Error("expected empty workspace to persist")
	}
}

func TestReconcileAutoHealsUnmanaged(t *testing.T) {
	store, _ := newTestStore()
	_ = store.AddSession("myapp", "dev")

	// "unmanaged" is a live session not in any workspace.
	live := map[string]bool{"dev": true, "unmanaged": true}
	if err := store.Reconcile(live); err != nil {
		t.Fatal(err)
	}

	// "unmanaged" should now be in workspace "unmanaged".
	ws, ok := store.WorkspaceFor("unmanaged")
	if !ok || ws != "unmanaged" {
		t.Errorf("WorkspaceFor(unmanaged) = %q, %v; want unmanaged, true", ws, ok)
	}
}

func TestReconcileSkipsOnEmptyLive(t *testing.T) {
	store, _ := newTestStore()
	_ = store.AddSession("myapp", "dev")

	if err := store.Reconcile(map[string]bool{}); err != nil {
		t.Fatal(err)
	}

	// Should not have removed anything.
	sessions := store.SessionsIn("myapp")
	if len(sessions) != 1 {
		t.Errorf("expected session preserved, got %v", sessions)
	}
}

// ── Save/Load round-trip ──

func TestV2SaveLoadRoundTrip(t *testing.T) {
	store, _ := newTestStore()
	_ = store.CreateWorkspace("myapp", "/home/user/src/myapp")
	_ = store.AddSession("myapp", "main")
	_ = store.AddSession("myapp", "server")
	_ = store.SetLastActive("myapp", "server")

	// Re-load and verify.
	st, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if st.Version != 2 {
		t.Errorf("version = %d; want 2", st.Version)
	}
	ws := st.Workspaces["myapp"]
	if ws == nil {
		t.Fatal("expected myapp workspace")
	}
	if len(ws.Sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(ws.Sessions))
	}
	if ws.LastActiveSession != "server" {
		t.Errorf("LastActiveSession = %q; want server", ws.LastActiveSession)
	}
}

func TestV2SaveLoadWithEmptyWorkspace(t *testing.T) {
	store, _ := newTestStore()
	_ = store.CreateWorkspace("empty", "")

	st, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	ws := st.Workspaces["empty"]
	if ws == nil {
		t.Fatal("expected empty workspace to persist")
	}
	if len(ws.Sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(ws.Sessions))
	}
}

// ── ListWorkspaces ──

func TestListWorkspaces(t *testing.T) {
	store, _ := newTestStore()
	_ = store.CreateWorkspace("zmux", "")
	_ = store.CreateWorkspace("bridge", "")
	_ = store.CreateWorkspace("admin", "")

	workspaces, err := store.ListWorkspaces()
	if err != nil {
		t.Fatal(err)
	}
	if len(workspaces) != 3 {
		t.Fatalf("expected 3, got %d", len(workspaces))
	}
	// Should be sorted.
	if workspaces[0].Name != "admin" || workspaces[1].Name != "bridge" || workspaces[2].Name != "zmux" {
		t.Errorf("order wrong: %v", workspaces)
	}
}

// ── EnsureWorkspace ──

func TestEnsureWorkspaceCreatesNew(t *testing.T) {
	store, _ := newTestStore()
	ws, err := store.EnsureWorkspace("myapp", "/src")
	if err != nil {
		t.Fatal(err)
	}
	if ws.Name != "myapp" || ws.RootDir != "/src" {
		t.Errorf("unexpected workspace: %+v", ws)
	}
}

func TestEnsureWorkspaceReturnsExisting(t *testing.T) {
	store, _ := newTestStore()
	_ = store.CreateWorkspace("myapp", "/original")

	ws, err := store.EnsureWorkspace("myapp", "/different")
	if err != nil {
		t.Fatal(err)
	}
	// Should return existing, not overwrite.
	if ws.RootDir != "/original" {
		t.Errorf("RootDir = %q; want /original (existing)", ws.RootDir)
	}
}
