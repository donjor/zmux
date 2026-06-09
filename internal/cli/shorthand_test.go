package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/workspace"
)

// memFS is a minimal in-memory config.FS for CLI tests.
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
func (m *memFS) Glob(pattern string) ([]string, error) {
	var matches []string
	for path := range m.files {
		ok, err := filepath.Match(pattern, path)
		if err != nil {
			return nil, err
		}
		if ok {
			matches = append(matches, path)
		}
	}
	return matches, nil
}

type fakeInfo struct{ name string }

func (f fakeInfo) Name() string       { return f.name }
func (f fakeInfo) Size() int64        { return 0 }
func (f fakeInfo) Mode() os.FileMode  { return 0o644 }
func (f fakeInfo) ModTime() time.Time { return time.Time{} }
func (f fakeInfo) IsDir() bool        { return false }
func (f fakeInfo) Sys() any           { return nil }

func withMockAppAndStore(t *testing.T) (*apppkg.App, *tmux.MockRunner, *workspace.Store) {
	t.Helper()

	fs := newMemFS("/home/user")
	mock := tmux.NewMockRunner()
	mock.InsideTmux = false
	mock.ServerUp = true

	store := workspace.NewStore(fs)
	_ = store.CreateWorkspace("myapp", "/home/user/myapp")
	_ = store.AddSession("myapp", "main")
	_ = store.CreateWorkspace("empty", "/home/user/empty")

	mock.Sessions = []tmux.Session{{Name: "main", Activity: time.Now()}}

	a := &apppkg.App{
		FS:             fs,
		Runner:         mock,
		WorkspaceStore: store,
	}
	return a, mock, store
}

func TestShorthandSingleArgRemoved(t *testing.T) {
	a, _, _ := withMockAppAndStore(t)
	err := resolveShorthand(a, []string{"myapp"})
	if err == nil {
		t.Fatal("expected shorthand removal error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "shorthand was removed") || !strings.Contains(msg, "zmux open myapp") || !strings.Contains(msg, "zmux new myapp") || !strings.Contains(msg, "zmux run myapp") {
		t.Fatalf("unexpected error: %q", msg)
	}
}

func TestShorthandTwoArgRemoved(t *testing.T) {
	a, _, _ := withMockAppAndStore(t)
	err := resolveShorthand(a, []string{"myapp", "main"})
	if err == nil {
		t.Fatal("expected shorthand removal error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "shorthand was removed") || !strings.Contains(msg, "zmux open myapp main") || !strings.Contains(msg, "zmux new myapp main") || !strings.Contains(msg, "zmux run myapp main") {
		t.Fatalf("unexpected error: %q", msg)
	}
}

func TestShorthandRemovedUsesActiveProfileName(t *testing.T) {
	a, _, _ := withMockAppAndStore(t)
	a.Profile = config.ProfileFromArgv("zzmux", a.FS)
	err := resolveShorthand(a, []string{"myapp"})
	if err == nil {
		t.Fatal("expected shorthand removal error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "zzmux open myapp") || !strings.Contains(msg, "zzmux new myapp") || !strings.Contains(msg, "zzmux run myapp") {
		t.Fatalf("unexpected error: %q", msg)
	}
}
