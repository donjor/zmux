package cli

import (
	"bytes"
	"strings"
	"testing"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/workspace"
)

func newRecipeTestApp() (*apppkg.App, *tmux.MockRunner, *memFS) {
	fs := newMemFS("/home/user")
	mock := tmux.NewMockRunner()
	mock.ServerUp = false
	profile := config.ProfileFromArgv("zmux", fs)
	return &apppkg.App{
		FS:             fs,
		Runner:         mock,
		WorkspaceStore: workspace.NewStoreAt(fs, "/home/user/.zmux/workspaces.toml"),
		Profile:        profile,
		ConfigPath:     profile.ConfigFile,
	}, mock, fs
}

func TestUpDryRunPrintsPlan(t *testing.T) {
	app, _, _ := newRecipeTestApp()
	root := NewRootCmd(app, testVersion)
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"up", "shell-fanout", "api", "web", "--dry-run"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"Recipe: shell-fanout", "Command: zmux run shell-fanout api web", "Inside zmux commands:", "create session api", "create session web", "Actions:"} {
		if !strings.Contains(got, want) {
			t.Fatalf("dry-run output missing %q:\n%s", want, got)
		}
	}
}

func TestRunRecipeDryRunPassesItems(t *testing.T) {
	app, _, _ := newRecipeTestApp()
	root := NewRootCmd(app, testVersion)
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"run", "shell-fanout", "api", "web", "--dry-run"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"Command: zmux run shell-fanout api web", "create session api", "create session web"} {
		if !strings.Contains(got, want) {
			t.Fatalf("run recipe dry-run output missing %q:\n%s", want, got)
		}
	}
}

func TestRunCommandFlagForcesCommandModeOnRecipeName(t *testing.T) {
	app, mock, _ := newRecipeTestApp()
	mock.InsideTmux = true
	mock.ServerUp = true
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	mock.DisplayMessageFunc = displayByFormat(map[string]string{
		"#{session_name}": "test-session",
	})

	root := NewRootCmd(app, testVersion)
	root.SetArgs([]string{"run", "--command", "dev", "-n", "dev", "-d"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, call := range mock.Calls {
		if call.Method == "NewWindow" && len(call.Args) > 1 && call.Args[1] == "dev" {
			return
		}
	}
	t.Fatalf("expected command mode to create dev tab; calls=%+v", mock.Calls)
}

func TestRunDetachAloneDoesNotOpenRecipeForm(t *testing.T) {
	app, _, _ := newRecipeTestApp()
	root := NewRootCmd(app, testVersion)
	root.SetArgs([]string{"run", "dev", "-d"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected command-mode session error outside tmux")
	}
	if !strings.Contains(err.Error(), "not inside tmux") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunUntilForcesCommandModeOnRecipeName(t *testing.T) {
	app, _, _ := newRecipeTestApp()
	root := NewRootCmd(app, testVersion)
	// --until is a command-mode readiness control. A recipe-named arg must not
	// swallow it into a recipe run; command mode then enforces the detach
	// requirement instead of silently dropping readiness.
	root.SetArgs([]string{"run", "dev", "--until", "ready"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "--until requires --detach") {
		t.Fatalf("expected --until to force command-mode validation, got %v", err)
	}
}

func TestRecipeListIncludesBundledRecipes(t *testing.T) {
	app, _, _ := newRecipeTestApp()
	root := NewRootCmd(app, testVersion)
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"recipe", "list"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "shell-fanout") || !strings.Contains(got, "worktree-shell") {
		t.Fatalf("list output missing bundled recipes:\n%s", got)
	}
}

func TestRecipeForkWritesProfileRecipe(t *testing.T) {
	app, _, fs := newRecipeTestApp()
	root := NewRootCmd(app, testVersion)
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"recipe", "fork", "worktree-shell"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	path := "/home/user/.zmux/recipes/worktree-shell.toml"
	if _, ok := fs.files[path]; !ok {
		t.Fatalf("fork did not write %s; files=%v", path, fs.files)
	}
	if !strings.Contains(out.String(), "forked worktree-shell") {
		t.Fatalf("unexpected fork output: %s", out.String())
	}
}

func TestRecipeLintReportsInvalidUserRecipe(t *testing.T) {
	app, _, fs := newRecipeTestApp()
	fs.files["/home/user/.zmux/recipes/bad.toml"] = []byte(`name =`)
	root := NewRootCmd(app, testVersion)
	root.SetArgs([]string{"recipe", "lint", "bad"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected lint error")
	}
	if !strings.Contains(err.Error(), "parse recipe") {
		t.Fatalf("unexpected lint error: %q", err.Error())
	}
}

func TestNewTemplateFlagDoesNotExist(t *testing.T) {
	app, _, _ := newRecipeTestApp()
	root := NewRootCmd(app, testVersion)
	root.SetArgs([]string{"new", "-t", "dev", "myapp"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected unknown template flag error")
	}
	if !strings.Contains(err.Error(), "unknown shorthand flag") {
		t.Fatalf("unexpected error: %q", err.Error())
	}
}
