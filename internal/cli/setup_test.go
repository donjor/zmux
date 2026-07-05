package cli

import (
	"strings"
	"testing"

	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/setup"
)

func TestSetupDoctorDetectsStaleLoadedShell(t *testing.T) {
	a, _ := newTestApp(t)
	a.Profile = config.Profile{Name: "zzmux"}
	fs := a.FS.(*memFS)

	plan, ok := setup.PlanShellIntegration(setup.ShellInput{Shell: setup.Bash, Home: "/home/user", Bin: "zmux", BashProfile: "/home/user/.profile"})
	if !ok {
		t.Fatal("expected bash plan")
	}
	if _, err := plan.Apply(fs, setup.ApplyOptions{}); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SHELL", "/bin/bash")
	t.Setenv("TMUX", "/tmp/zzmux,1,0")
	t.Setenv("TMUX_PANE", "%1")
	t.Setenv("ZMUX_SHELL_INTEGRATION_VERSION", "")

	root := NewRootCmd(a, testVersion)
	root.SetArgs([]string{"setup", "doctor"})
	root.SilenceUsage = true
	root.SilenceErrors = true
	var err error
	out := captureStdout(t, func() { err = root.Execute() })
	if err == nil {
		t.Fatal("expected stale loaded shell to fail doctor")
	}
	for _, want := range []string{"ok: /home/user/.bashrc", "stale: current shell loaded integration version missing", "open a fresh zmux tab/shell"} {
		if !strings.Contains(out, want) {
			t.Fatalf("doctor output missing %q:\n%s", want, out)
		}
	}
}

func TestTopLevelDoctorAliasPassesWithCurrentFiles(t *testing.T) {
	a, _ := newTestApp(t)
	fs := a.FS.(*memFS)

	plan, ok := setup.PlanShellIntegration(setup.ShellInput{Shell: setup.Bash, Home: "/home/user", Bin: "zmux", BashProfile: "/home/user/.profile"})
	if !ok {
		t.Fatal("expected bash plan")
	}
	if _, err := plan.Apply(fs, setup.ApplyOptions{}); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SHELL", "/bin/bash")
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")

	root := NewRootCmd(a, testVersion)
	root.SetArgs([]string{"doctor"})
	root.SilenceUsage = true
	root.SilenceErrors = true
	var err error
	out := captureStdout(t, func() { err = root.Execute() })
	if err != nil {
		t.Fatalf("expected top-level doctor to pass, got %v\n%s", err, out)
	}
	if !strings.Contains(out, "zmux shell integration doctor") {
		t.Fatalf("doctor output missing header:\n%s", out)
	}
}

func TestSetupDoctorPassesWithCurrentFilesOutsideTmux(t *testing.T) {
	a, _ := newTestApp(t)
	fs := a.FS.(*memFS)

	plan, ok := setup.PlanShellIntegration(setup.ShellInput{Shell: setup.Bash, Home: "/home/user", Bin: "zmux", BashProfile: "/home/user/.profile"})
	if !ok {
		t.Fatal("expected bash plan")
	}
	if _, err := plan.Apply(fs, setup.ApplyOptions{}); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SHELL", "/bin/bash")
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")

	root := NewRootCmd(a, testVersion)
	root.SetArgs([]string{"setup", "doctor"})
	root.SilenceUsage = true
	root.SilenceErrors = true
	var err error
	out := captureStdout(t, func() { err = root.Execute() })
	if err != nil {
		t.Fatalf("expected doctor to pass, got %v\n%s", err, out)
	}
	if !strings.Contains(out, "ok: not inside tmux") {
		t.Fatalf("doctor output missing outside-tmux note:\n%s", out)
	}
}
