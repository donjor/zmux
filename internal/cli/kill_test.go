package cli

import (
	"strings"
	"testing"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/workspace"
)

func execKill(t *testing.T, app *apppkg.App, args ...string) error {
	t.Helper()
	cmd := newKillCmd(app)
	cmd.SetArgs(args)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	return cmd.Execute()
}

// seedWorkspaceSessions creates a workspace and registers each label as a live
// tmux session under its managed raw name, returning the raw name per label.
func seedWorkspaceSessions(t *testing.T, app *apppkg.App, mock *tmux.MockRunner, ws string, labels ...string) map[string]string {
	t.Helper()
	if err := app.WorkspaceStore.CreateWorkspace(ws, "/repo"); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	raw := make(map[string]string, len(labels))
	for _, label := range labels {
		if err := app.WorkspaceStore.AddSession(ws, label); err != nil {
			t.Fatalf("seed session %q: %v", label, err)
		}
		name := workspace.RawSessionName(ws, label)
		raw[label] = name
		mock.Sessions = append(mock.Sessions, tmux.Session{Name: name})
	}
	return raw
}

// TestKillWorkspaceSessionTarget is the core regression: `kill ws/label`
// resolves the managed session and kills exactly it, leaving siblings alive.
func TestKillWorkspaceSessionTarget(t *testing.T) {
	app, mock := newTestApp(t)
	mock.InsideTmux = false // force the workspace/session branch, not the current-session shortcut
	raw := seedWorkspaceSessions(t, app, mock, "qa-qol", "main", "api", "worker")

	if err := execKill(t, app, "qa-qol/api"); err != nil {
		t.Fatalf("kill qa-qol/api: %v", err)
	}

	kill, ok := srFindCall(mock.Calls, "KillSession")
	if !ok {
		t.Fatalf("expected KillSession call, calls = %v", mock.Calls)
	}
	if kill.Args[0] != raw["api"] {
		t.Errorf("KillSession target = %q, want %q", kill.Args[0], raw["api"])
	}
	if !mock.HasSession(raw["main"]) || !mock.HasSession(raw["worker"]) {
		t.Error("sibling sessions main/worker must stay alive after killing api")
	}
	if mock.HasSession(raw["api"]) {
		t.Error("api session should be gone")
	}
}

// TestKillUnknownSessionTargetErrors: a workspace/session target that names a
// missing label reports the workspace miss, not the old generic error.
func TestKillUnknownSessionTargetErrors(t *testing.T) {
	app, mock := newTestApp(t)
	mock.InsideTmux = false
	seedWorkspaceSessions(t, app, mock, "qa-qol", "main")

	err := execKill(t, app, "qa-qol/ghost")
	if err == nil {
		t.Fatal("expected error for unknown session label")
	}
	if !strings.Contains(err.Error(), "not in workspace") {
		t.Errorf("error = %q, want it to mention the workspace miss", err.Error())
	}
	if srHasCall(mock.Calls, "KillSession") {
		t.Error("no session should be killed on resolution failure")
	}
}

// TestKillWorkspaceYesSkipsPrompt: `kill ws --yes` tears down every live session
// without reading stdin, so scripted/agent teardown never blocks on the prompt.
func TestKillWorkspaceYesSkipsPrompt(t *testing.T) {
	app, mock := newTestApp(t)
	mock.InsideTmux = false
	raw := seedWorkspaceSessions(t, app, mock, "qa-qol", "main", "api", "worker")

	if err := execKill(t, app, "qa-qol", "--yes"); err != nil {
		t.Fatalf("kill qa-qol --yes: %v", err)
	}

	for label, name := range raw {
		if mock.HasSession(name) {
			t.Errorf("session %q (%s) should be killed by workspace teardown", label, name)
		}
	}
	if ws, _ := app.WorkspaceStore.GetWorkspace("qa-qol"); ws != nil {
		t.Error("workspace record should be removed after kill --yes")
	}
}
