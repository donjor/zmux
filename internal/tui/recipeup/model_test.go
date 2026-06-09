package recipeup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/workspace"
)

type memFS struct {
	files map[string][]byte
	home  string
}

func newMemFS(home string) *memFS {
	return &memFS{files: map[string][]byte{}, home: home}
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

func newTestApp() (*apppkg.App, *tmux.MockRunner, *memFS) {
	fs := newMemFS("/home/user")
	runner := tmux.NewMockRunner()
	runner.ServerUp = false
	profile := config.ProfileFromArgv("zmux", fs)
	return &apppkg.App{
		FS:             fs,
		Runner:         runner,
		WorkspaceStore: workspace.NewStoreAt(fs, "/home/user/.zmux/workspaces.toml"),
		Profile:        profile,
		ConfigPath:     profile.ConfigFile,
	}, runner, fs
}

func TestSnapshotBrowser(t *testing.T) {
	app, _, _ := newTestApp()
	out, err := Snapshot(app)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "zmux recipes") || !strings.Contains(out, "shell-fanout") || !strings.Contains(out, "Command") || !strings.Contains(out, "TOML") {
		t.Fatalf("snapshot missing browser content:\n%s", out)
	}
}

func TestSnapshotPlan(t *testing.T) {
	app, _, _ := newTestApp()
	out, err := SnapshotPlan(app, "shell-fanout", []string{"api"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Dry-run plan", "Outside zmux", "zmux run shell-fanout api", "Inside zmux commands", "session api"} {
		if !strings.Contains(out, want) {
			t.Fatalf("snapshot missing %q:\n%s", want, out)
		}
	}
}

func TestSnapshotPlanUsesProfileBinary(t *testing.T) {
	app, _, _ := newTestApp()
	app.Profile = config.ProfileFromArgv("zzmux", app.FS)
	out, err := SnapshotPlan(app, "shell-fanout", []string{"api"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "zzmux run shell-fanout api") {
		t.Fatalf("snapshot missing dry-run plan:\n%s", out)
	}
}

func TestSnapshotShowsLintErrorCount(t *testing.T) {
	app, _, fs := newTestApp()
	fs.files["/home/user/.zmux/recipes/bad.toml"] = []byte(`name =`)
	out, err := Snapshot(app)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "lint: 1 error") {
		t.Fatalf("snapshot missing lint error footer:\n%s", out)
	}
}

func TestKeyboardSelectAndCancelDryRun(t *testing.T) {
	app, _, _ := newTestApp()
	m := New(app)
	m.cursor = recipeIndex(t, m, "dev")
	updated, _ := m.Update(key("enter"))
	m = updated.(Model)
	if m.screen != screenInput {
		t.Fatalf("screen = %v, want input", m.screen)
	}
	updated, _ = m.Update(key("esc"))
	m = updated.(Model)
	if m.screen != screenBrowser {
		t.Fatalf("screen = %v, want browser", m.screen)
	}
}

func TestKeyboardForkBundledRecipe(t *testing.T) {
	app, _, fs := newTestApp()
	m := New(app)
	m.cursor = recipeIndex(t, m, "worktree-shell")
	updated, _ := m.Update(key("f"))
	m = updated.(Model)
	path := "/home/user/.zmux/recipes/worktree-shell.toml"
	if _, ok := fs.files[path]; !ok {
		t.Fatalf("fork did not write %s", path)
	}
	if m.err != nil {
		t.Fatalf("unexpected error: %v", m.err)
	}
}

func recipeIndex(t *testing.T, m Model, name string) int {
	t.Helper()
	for i, def := range m.defs {
		if def.Recipe.Name == name {
			return i
		}
	}
	t.Fatalf("recipe %q not found", name)
	return 0
}

func key(name string) tea.KeyPressMsg {
	if len(name) == 1 {
		return tea.KeyPressMsg{Code: rune(name[0]), Text: name}
	}
	return tea.KeyPressMsg{Text: name}
}
