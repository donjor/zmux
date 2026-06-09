package recipe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/tmux"
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

func TestBundledRecipes(t *testing.T) {
	defs := Bundled()
	for _, name := range []string{"dev", "webdev", "monitor", "claude", "shell-fanout", "worktree-shell"} {
		if _, ok := Find(defs, name); !ok {
			t.Fatalf("bundled recipe %q not found", name)
		}
	}
}

func TestPlanFanoutInterpolation(t *testing.T) {
	defs := Bundled()
	def, ok := Find(defs, "shell-fanout")
	if !ok {
		t.Fatal("shell-fanout not found")
	}
	plan, err := PlanRecipe(def.Recipe, PlanOptions{
		CWD:   "/repo/My App",
		Items: []string{"Feature A", "bug/fix"},
	}, emptyState())
	if err != nil {
		t.Fatal(err)
	}
	if plan.Workspace != "my-app" {
		t.Fatalf("workspace = %q, want my-app", plan.Workspace)
	}
	got := []string{plan.Sessions[0].Name, plan.Sessions[1].Name}
	want := []string{"feature-a", "bug-fix"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("session[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if !strings.Contains(RenderPlan(plan), "create session feature-a") {
		t.Fatalf("rendered plan missing created session:\n%s", RenderPlan(plan))
	}
}

func TestShellQuoteFilter(t *testing.T) {
	defs := Bundled()
	def, ok := Find(defs, "worktree-shell")
	if !ok {
		t.Fatal("worktree-shell not found")
	}
	plan, err := PlanRecipe(def.Recipe, PlanOptions{
		CWD:   "/repo/app",
		Items: []string{"feat one"},
	}, emptyState())
	if err != nil {
		t.Fatal(err)
	}
	cmd := plan.Sessions[0].Tabs[0].Command
	if !strings.Contains(cmd, "wt switch --create 'feat one'") {
		t.Fatalf("command = %q", cmd)
	}
}

func TestRunCommandQuotesItemsAndSeparatesFlagLikeItems(t *testing.T) {
	got := RunCommand("zzmux", "shell-fanout", []string{"api service", "--danger"}, true)
	want := "zzmux run --detach shell-fanout -- 'api service' --danger"
	if got != want {
		t.Fatalf("RunCommand() = %q, want %q", got, want)
	}
}

func TestRunCommandIncludesNonDefaultRecipeOptions(t *testing.T) {
	got := RunCommandWithOptions("zzmux", "dev", PlanOptions{
		CWD:       "/repo/app copy",
		Workspace: "app",
		Session:   "main",
		TabMode:   TabModeReady,
	}, PlanOptions{
		CWD:       "/repo/app",
		Workspace: "app",
		Session:   "main",
		TabMode:   TabModeRun,
	})
	want := "zzmux run --cwd '/repo/app copy' --tab-mode ready dev"
	if got != want {
		t.Fatalf("RunCommandWithOptions() = %q, want %q", got, want)
	}
}

func TestRenderPlanShowsOutsideCommandAndInsideCommands(t *testing.T) {
	defs := Bundled()
	def, _ := Find(defs, "worktree-shell")
	plan, err := PlanRecipe(def.Recipe, PlanOptions{
		Bin:   "zzmux",
		CWD:   "/repo/app",
		Items: []string{"feat one"},
	}, emptyState())
	if err != nil {
		t.Fatal(err)
	}
	rendered := RenderPlan(plan)
	for _, want := range []string{
		"Run from: outside zmux",
		"Command: zzmux run worktree-shell 'feat one'",
		"Inside zmux commands:",
		"feat-one:shell  wt switch --create 'feat one' -x \"$SHELL\"",
		"Reconcile:",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered plan missing %q:\n%s", want, rendered)
		}
	}
}

func TestSlugAvoidsNumericSessionPrefix(t *testing.T) {
	if got := Slug("123 feature"); got != "r-123-feature" {
		t.Fatalf("Slug numeric prefix = %q", got)
	}
}

func TestPlanReconcilesExistingSessionsAndTabs(t *testing.T) {
	defs := Bundled()
	def, _ := Find(defs, "shell-fanout")
	state := emptyState()
	state.Workspaces["app"] = WorkspaceState{Name: "app", Sessions: map[string]bool{"api": true}}
	state.Sessions["api"] = SessionState{Name: "api", Windows: map[string]WindowState{"shell": {Name: "shell"}}}

	plan, err := PlanRecipe(def.Recipe, PlanOptions{CWD: "/repo/app", Items: []string{"api", "web"}}, state)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Sessions[0].Exists || !plan.Sessions[0].Tabs[0].Exists {
		t.Fatalf("first session/tab should be existing: %+v", plan.Sessions[0])
	}
	if plan.Sessions[1].Exists {
		t.Fatalf("second session should be created: %+v", plan.Sessions[1])
	}
	rendered := RenderPlan(plan)
	if !strings.Contains(rendered, "existing session api") || !strings.Contains(rendered, "create session web") {
		t.Fatalf("rendered plan did not show reconcile state:\n%s", rendered)
	}
}

func TestDiscoveryUserRecipesOverrideBundled(t *testing.T) {
	fs := newMemFS("/home/u")
	profile := config.ProfileFromArgv("zzmux", fs)
	fs.files["/home/u/.zzmux/recipes/dev.toml"] = []byte(`
name = "dev"
description = "User override"
kind = "session"
[[tabs]]
name = "shell"
`)
	defs, err := Load(fs, DefaultDirs(fs, profile))
	if err != nil {
		t.Fatal(err)
	}
	def, ok := Find(defs, "dev")
	if !ok {
		t.Fatal("dev recipe not found")
	}
	if def.Source != SourceUser || def.Recipe.Description != "User override" {
		t.Fatalf("def = %+v", def)
	}
}

func TestConfiguredDirsUsesActiveProfileForDefaultConfig(t *testing.T) {
	fs := newMemFS("/home/u")
	profile := config.ProfileFromArgv("zzmux", fs)
	dirs := ConfiguredDirs(fs, profile, config.DefaultConfig())
	if len(dirs) != 1 || dirs[0] != "/home/u/.zzmux/recipes" {
		t.Fatalf("dirs = %#v, want zzmux profile recipe dir", dirs)
	}
}

func TestConfiguredDirsKeepsExplicitRecipePaths(t *testing.T) {
	fs := newMemFS("/home/u")
	profile := config.ProfileFromArgv("zzmux", fs)
	cfg := config.DefaultConfig()
	cfg.Recipes.Paths = []string{"~/shared-recipes"}
	dirs := ConfiguredDirs(fs, profile, cfg)
	if len(dirs) != 1 || dirs[0] != "~/shared-recipes" {
		t.Fatalf("dirs = %#v, want explicit configured path", dirs)
	}
}

func TestPlanRejectsDuplicateFanoutSessions(t *testing.T) {
	defs := Bundled()
	def, _ := Find(defs, "shell-fanout")
	_, err := PlanRecipe(def.Recipe, PlanOptions{CWD: "/repo/app", Items: []string{"api!", "api?"}}, emptyState())
	if err == nil {
		t.Fatal("expected duplicate session error")
	}
	if !strings.Contains(err.Error(), `duplicate session "api"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteFocusesExistingTabByActualIndex(t *testing.T) {
	runner := tmux.NewMockRunner()
	runner.Sessions = []tmux.Session{{Name: "api"}}
	runner.Windows["api"] = []tmux.Window{
		{Index: 1, Name: "main"},
		{Index: 3, Name: "shell"},
	}
	plan := Plan{
		RecipeName:   "shell-fanout",
		Workspace:    "app",
		CWD:          "/repo/app",
		FocusSession: "api",
		FocusTab:     "shell",
		Sessions: []PlannedSession{{
			Name:   "api",
			Exists: true,
			Tabs:   []PlannedTab{{Name: "shell", Exists: true}},
		}},
	}
	if err := Execute(runner, nil, plan); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, call := range runner.Calls {
		if call.Method == "SelectWindow" && len(call.Args) == 2 && call.Args[0] == "api" && call.Args[1] == "3" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected SelectWindow(api, 3), calls=%+v", runner.Calls)
	}
}

func TestLintReportsInvalidUserRecipe(t *testing.T) {
	fs := newMemFS("/home/u")
	profile := config.ProfileFromArgv("zmux", fs)
	fs.files["/home/u/.zmux/recipes/bad.toml"] = []byte(`name =`)
	results := Lint(fs, DefaultDirs(fs, profile), []string{"bad"})
	if len(results) != 1 {
		t.Fatalf("got %d lint results, want 1", len(results))
	}
	if results[0].Err == nil {
		t.Fatal("expected lint error for invalid TOML")
	}
}

func TestBuildStateFromMockRunner(t *testing.T) {
	runner := tmux.NewMockRunner()
	runner.Sessions = []tmux.Session{{Name: "api"}}
	runner.Windows["api"] = []tmux.Window{{Name: "shell"}}
	state, err := BuildState(runner, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := state.Sessions["api"].Windows["shell"]; !ok {
		t.Fatalf("state missing api:shell: %+v", state)
	}
}

func emptyState() State {
	return State{Sessions: map[string]SessionState{}, Workspaces: map[string]WorkspaceState{}}
}
