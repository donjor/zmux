package cli

import (
	"strings"
	"testing"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/workspace"
)

// execSessionRun builds the `session run` subcommand and executes it with the
// given args. Executing (vs calling RunE) is required so cobra populates
// ArgsLenAtDash for the `--` boundary check.
func execSessionRun(t *testing.T, app *apppkg.App, args ...string) error {
	t.Helper()
	cmd := newSessionRunCmd(app)
	cmd.SetArgs(args)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	return cmd.Execute()
}

func srHasCall(calls []tmux.MockCall, method string) bool {
	for _, c := range calls {
		if c.Method == method {
			return true
		}
	}
	return false
}

func srFindCall(calls []tmux.MockCall, method string) (tmux.MockCall, bool) {
	for _, c := range calls {
		if c.Method == method {
			return c, true
		}
	}
	return tmux.MockCall{}, false
}

// TestSessionRunCurrentWorkspaceCreatesFirstTab is the happy path: a detached
// session born in the current workspace, the command as its first window, and —
// critically — NO attach/switch and NO NewWindow (the focus + blank-tab guard).
func TestSessionRunCurrentWorkspaceCreatesFirstTab(t *testing.T) {
	app, mock := newTestApp(t)
	mock.InsideTmux = true
	mock.NewSessionWindowPaneID = "%42"
	mock.DisplayMessageFunc = func(_, format string) (string, error) {
		switch format {
		case "#{session_name}":
			return "main", nil
		case "#{pane_current_path}":
			return "/panedir", nil
		}
		return "", nil
	}
	if err := app.WorkspaceStore.CreateWorkspace("proj", "/repo"); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if err := app.WorkspaceStore.AddSession("proj", "main"); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	err := execSessionRun(t, app, "worker-auth", "-n", "auth-worker", "--",
		"codex", "-C", "/wt", "-s", "workspace-write", "-a", "on-request")
	if err != nil {
		t.Fatalf("session run: %v", err)
	}

	// Created with a named first window, in the caller's pane dir.
	nsw, ok := srFindCall(mock.Calls, "NewSessionWindow")
	if !ok {
		t.Fatal("expected NewSessionWindow call")
	}
	if got, want := nsw.Args, []string{workspace.RawSessionName("proj", "worker-auth"), "auth-worker", "/panedir"}; !equalArgs(got, want) {
		t.Errorf("NewSessionWindow args = %v, want %v", got, want)
	}

	// Focus + blank-tab regression guard: no attach/switch, and the command
	// runs in the birth window — never a second NewWindow / NewSession.
	for _, m := range []string{"SwitchClient", "AttachSession", "AttachSessionDetach", "NewWindow", "NewSession"} {
		if srHasCall(mock.Calls, m) {
			t.Errorf("unexpected %s call — session run must not %s", m, m)
		}
	}

	// Command sent to the birth pane.
	sk, ok := srFindCall(mock.Calls, "SendKeys")
	if !ok {
		t.Fatal("expected SendKeys call")
	}
	if sk.Args[0] != "%42" {
		t.Errorf("SendKeys target = %q, want %q", sk.Args[0], "%42")
	}
	if !strings.Contains(strings.Join(sk.Args, " "), "codex -C /wt -s workspace-write -a on-request") {
		t.Errorf("SendKeys did not carry the worker command: %v", sk.Args)
	}

	// Tagged into the workspace, but NOT made the default attach target.
	sessions := app.WorkspaceStore.SessionLabelsIn("proj")
	if !contains(sessions, "worker-auth") {
		t.Errorf("workspace proj sessions = %v, want to include worker-auth", sessions)
	}
	ws, _ := app.WorkspaceStore.GetWorkspace("proj")
	if ws != nil && ws.LastActiveSession == "worker-auth" {
		t.Errorf("worker became last-active session; should never SetLastActive")
	}
}

// TestSessionRunNamedWorkspaceOutsideTmux covers the conductor-outside-tmux
// path and the command-quoting fix: --workspace + --cwd, no current-session
// lookup, and post-`--` argv reconstructed with quoting intact.
func TestSessionRunNamedWorkspaceOutsideTmux(t *testing.T) {
	app, mock := newTestApp(t)
	mock.InsideTmux = false
	mock.NewSessionWindowPaneID = "%7"
	if err := app.WorkspaceStore.CreateWorkspace("proj", "/reporoot"); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}

	err := execSessionRun(t, app, "worker-x", "--workspace", "proj", "--cwd", "/worktree",
		"-n", "x-worker", "--", "bash", "-lc", "printf ready; sleep 2")
	if err != nil {
		t.Fatalf("session run: %v", err)
	}

	// --cwd wins; no current-session DisplayMessage needed outside tmux.
	nsw, _ := srFindCall(mock.Calls, "NewSessionWindow")
	if got, want := nsw.Args, []string{workspace.RawSessionName("proj", "worker-x"), "x-worker", "/worktree"}; !equalArgs(got, want) {
		t.Errorf("NewSessionWindow args = %v, want %v", got, want)
	}
	// No current-session resolution: --workspace means we never query
	// #{session_name} (internal stamping may still display other formats).
	for _, c := range mock.Calls {
		if c.Method == "DisplayMessage" && len(c.Args) >= 2 && c.Args[1] == "#{session_name}" {
			t.Error("unexpected #{session_name} lookup — outside tmux with --workspace needs none")
		}
	}

	// Quoting preserved: argv [bash -lc "printf ready; sleep 2"] must NOT
	// collapse to `bash -lc printf ready; sleep 2`.
	sk, _ := srFindCall(mock.Calls, "SendKeys")
	if !strings.Contains(strings.Join(sk.Args, " "), "bash -lc 'printf ready; sleep 2'") {
		t.Errorf("command quoting lost; SendKeys args = %v", sk.Args)
	}
}

// TestSessionRunValidationErrors: every bad invocation errors BEFORE creating
// anything (no NewSessionWindow).
func TestSessionRunValidationErrors(t *testing.T) {
	tests := []struct {
		name     string
		inside   bool
		seedWS   bool // create "proj" + tag current session "main" to it
		curSess  string
		preExist string // pre-existing tmux session name
		args     []string
		wantErr  string
	}{
		{
			name:   "missing -n",
			inside: true, seedWS: true, curSess: "main",
			args:    []string{"wk", "--", "echo", "hi"},
			wantErr: "tab name is required",
		},
		{
			name:    "outside tmux without --workspace",
			inside:  false,
			args:    []string{"wk", "-n", "t", "--", "echo", "hi"},
			wantErr: "outside tmux",
		},
		{
			name:   "current session not in a workspace",
			inside: true, curSess: "orphan",
			args:    []string{"wk", "-n", "t", "--", "echo", "hi"},
			wantErr: "not in a workspace",
		},
		{
			name:   "unknown --workspace",
			inside: true, curSess: "main",
			args:    []string{"wk", "--workspace", "ghost", "-n", "t", "--", "echo", "hi"},
			wantErr: `workspace "ghost" does not exist`,
		},
		{
			name:   "session already exists",
			inside: true, seedWS: true, curSess: "main", preExist: workspace.RawSessionName("proj", "wk"),
			args:    []string{"wk", "-n", "t", "--", "echo", "hi"},
			wantErr: "already exists",
		},
		{
			name:   "invalid session name (leading digit)",
			inside: true, seedWS: true, curSess: "main",
			args:    []string{"9bad", "-n", "t", "--", "echo", "hi"},
			wantErr: "invalid session label",
		},
		{
			name:   "command not after --",
			inside: true, seedWS: true, curSess: "main",
			args:    []string{"wk", "-n", "t", "echo", "hi"},
			wantErr: "must follow",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app, mock := newTestApp(t)
			mock.InsideTmux = tc.inside
			if tc.curSess != "" {
				mock.DisplayMessageResult = tc.curSess
			}
			if tc.preExist != "" {
				mock.Sessions = append(mock.Sessions, tmux.Session{Name: tc.preExist})
			}
			if tc.seedWS {
				_ = app.WorkspaceStore.CreateWorkspace("proj", "/repo")
				_ = app.WorkspaceStore.AddSession("proj", "main")
			}
			err := execSessionRun(t, app, tc.args...)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tc.wantErr)
			}
			if srHasCall(mock.Calls, "NewSessionWindow") {
				t.Error("a validation failure must not create a session")
			}
		})
	}
}

// TestSessionRunRollsBackOnTagFailure: if workspace tagging fails after the
// session was created, the new session is killed so nothing half-built lingers.
// Triggered by a worker name already tagged to a different workspace.
func TestSessionRunRollsBackOnTagFailure(t *testing.T) {
	app, mock := newTestApp(t)
	mock.InsideTmux = true
	mock.NewSessionWindowPaneID = "%9"
	mock.DisplayMessageResult = "main"
	_ = app.WorkspaceStore.CreateWorkspace("proj", "/repo")
	_ = app.WorkspaceStore.AddSession("proj", "main")
	// worker-z's proj identity is already tagged to a different workspace, so
	// AddSessionRecord("proj", rec) will fail — but HasSession is false (not a
	// live tmux session).
	_ = app.WorkspaceStore.CreateWorkspace("other", "/o")
	rec, _ := workspace.NewSessionRecord("proj", "worker-z")
	_ = app.WorkspaceStore.AddSessionRecord("other", rec)

	err := execSessionRun(t, app, "worker-z", "--workspace", "proj", "-n", "zt", "--", "echo", "hi")
	if err == nil {
		t.Fatal("expected tag-failure error")
	}
	kill, ok := srFindCall(mock.Calls, "KillSession")
	if !ok || len(kill.Args) == 0 || kill.Args[0] != workspace.RawSessionName("proj", "worker-z") {
		t.Errorf("expected rollback KillSession(%s), calls = %v", workspace.RawSessionName("proj", "worker-z"), mock.Calls)
	}
}

func equalArgs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
