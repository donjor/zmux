package main

import (
	"os"
	"testing"
	"time"

	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/workspace"
)

// memFS is a minimal in-memory config.FS for shorthand tests.
type memFS struct {
	files map[string][]byte
	home  string
}

func newMemFS(home string) *memFS {
	return &memFS{files: make(map[string][]byte), home: home}
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
func (m *memFS) UserHomeDir() (string, error) { return m.home, nil }
func (m *memFS) Glob(_ string) ([]string, error) {
	return nil, nil
}

type fakeInfo struct{ name string }

func (f fakeInfo) Name() string       { return f.name }
func (f fakeInfo) Size() int64        { return 0 }
func (f fakeInfo) Mode() os.FileMode  { return 0o644 }
func (f fakeInfo) ModTime() time.Time { return time.Time{} }
func (f fakeInfo) IsDir() bool        { return false }
func (f fakeInfo) Sys() any           { return nil }

// withMockAppAndStore sets up the global app with a tmux mock and an
// in-memory workspace store that has one workspace "myapp" with
// session "main" registered.
func withMockAppAndStore(t *testing.T) (*tmux.MockRunner, *workspace.Store) {
	t.Helper()

	fs := newMemFS("/home/user")
	mock := tmux.NewMockRunner()
	mock.InsideTmux = false
	mock.ServerUp = true

	store := workspace.NewStore(fs)
	_ = store.CreateWorkspace("myapp", "/home/user/myapp")
	_ = store.AddSession("myapp", "main")
	_ = store.CreateWorkspace("empty", "/home/user/empty")

	// Seed mock with one live session "main".
	mock.Sessions = []tmux.Session{
		{Name: "main", Activity: time.Now()},
	}

	orig := app
	app = &App{
		FS:             fs,
		Runner:         mock,
		WorkspaceStore: store,
	}
	t.Cleanup(func() { app = orig })
	return mock, store
}

// ── Two-arg shorthand: zmux <ws> <session> ──

func TestShorthand_TwoArg_AttachExistingSession(t *testing.T) {
	mock, _ := withMockAppAndStore(t)

	err := resolveShorthand([]string{"myapp", "main"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should have called AttachSession (or NewGroupedSession if attached).
	found := false
	for _, call := range mock.Calls {
		if call.Method == "AttachSession" || call.Method == "NewGroupedSession" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected AttachSession or NewGroupedSession call, got: %+v", mock.Calls)
	}
}

func TestShorthand_TwoArg_CreateMissingSession(t *testing.T) {
	mock, store := withMockAppAndStore(t)

	// "dev" doesn't exist as a tmux session.
	err := resolveShorthand([]string{"myapp", "dev"})
	if err != nil {
		t.Fatalf("expected no error (should create session), got: %v", err)
	}

	// Should have called NewSession to create "dev".
	found := false
	for _, call := range mock.Calls {
		if call.Method == "NewSession" && len(call.Args) > 0 && call.Args[0] == "dev" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected NewSession(dev, ...) call, got: %+v", mock.Calls)
	}

	// "dev" should be registered in the workspace store.
	ws, _ := store.GetWorkspace("myapp")
	if ws == nil {
		t.Fatal("workspace myapp should still exist")
	}
	hasSession := false
	for _, s := range ws.Sessions {
		if s == "dev" {
			hasSession = true
			break
		}
	}
	if !hasSession {
		t.Errorf("session 'dev' not registered in workspace store; sessions=%v", ws.Sessions)
	}
}

func TestShorthand_TwoArg_WorkspaceNotFound(t *testing.T) {
	withMockAppAndStore(t)

	err := resolveShorthand([]string{"nonexistent", "dev"})
	if err == nil {
		t.Fatal("expected error for nonexistent workspace")
	}
	if !contains(err.Error(), "not found") {
		t.Errorf("error = %q, want contains 'not found'", err.Error())
	}
}

// ── Single-arg shorthand: zmux <name> ──

func TestShorthand_SingleArg_WorkspaceAttachLastActive(t *testing.T) {
	mock, _ := withMockAppAndStore(t)

	err := resolveShorthand([]string{"myapp"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should attach to the live session "main".
	found := false
	for _, call := range mock.Calls {
		if call.Method == "AttachSession" || call.Method == "NewGroupedSession" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected session attach call, got: %+v", mock.Calls)
	}
}

func TestShorthand_SingleArg_SessionFallback(t *testing.T) {
	mock, _ := withMockAppAndStore(t)

	// "main" exists as a session but isn't in a workspace called "main".
	err := resolveShorthand([]string{"main"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	found := false
	for _, call := range mock.Calls {
		if call.Method == "AttachSession" || call.Method == "NewGroupedSession" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected session attach call, got: %+v", mock.Calls)
	}
}

func TestShorthand_SingleArg_CreateSession(t *testing.T) {
	mock, _ := withMockAppAndStore(t)

	// "brand-new" doesn't exist as workspace or session.
	err := resolveShorthand([]string{"brand-new"})
	if err != nil {
		t.Fatalf("expected no error (should create session), got: %v", err)
	}

	found := false
	for _, call := range mock.Calls {
		if call.Method == "NewSession" && len(call.Args) > 0 && call.Args[0] == "brand-new" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected NewSession(brand-new, ...) call, got: %+v", mock.Calls)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsImpl(s, substr))
}

func containsImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
