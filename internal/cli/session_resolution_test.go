package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tabstate"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/workspace"
)

func TestSessionKillResolvesWorkspaceSessionTarget(t *testing.T) {
	app, mock := newTestApp(t)
	mock.InsideTmux = false
	raw := seedWorkspaceSessions(t, app, mock, "qa-qol", "main", "worker")

	cmd := newSessionKillCmd(app)
	cmd.SetArgs([]string{"qa-qol/worker"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err != nil {
		t.Fatalf("session kill qa-qol/worker: %v", err)
	}

	kill, ok := srFindCall(mock.Calls, "KillSession")
	if !ok {
		t.Fatalf("expected KillSession call, calls = %#v", mock.Calls)
	}
	if kill.Args[0] != raw["worker"] {
		t.Fatalf("KillSession target = %q, want %q", kill.Args[0], raw["worker"])
	}
	if !mock.HasSession(raw["main"]) {
		t.Fatal("sibling main session should remain live")
	}
}

func TestPaneListSessionTargetResolvesWorkspaceSession(t *testing.T) {
	app, mock := newTestApp(t)
	mock.InsideTmux = false
	raw := seedWorkspaceSessions(t, app, mock, "qa-qol", "main")
	mock.Panes[raw["main"]] = []tmux.Pane{{ID: "%1", Session: raw["main"]}}

	cmd := newPaneListCmd(app, "list")
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := runPaneList(app, cmd, &paneListFlags{target: "qa-qol/main", session: true, quiet: true}); err != nil {
		t.Fatalf("pane list --session --target qa-qol/main: %v", err)
	}

	call, ok := srFindCall(mock.Calls, "ListPanes")
	if !ok {
		t.Fatalf("expected ListPanes call, calls = %#v", mock.Calls)
	}
	if call.Args[0] != raw["main"] {
		t.Fatalf("ListPanes target = %q, want %q", call.Args[0], raw["main"])
	}
	if strings.TrimSpace(out.String()) != "%1" {
		t.Fatalf("unexpected pane list output %q", out.String())
	}
}

func TestTabStateSessionFlagResolvesWorkspaceSession(t *testing.T) {
	app, mock := newTestApp(t)
	mock.InsideTmux = false
	raw := seedWorkspaceSessions(t, app, mock, "qa-qol", "main")
	mock.LogicalRows = []tmux.LogicalPaneRow{
		{Session: raw["main"], WindowID: "@1", WindowIndex: 1, WindowName: "scratch", WindowPanes: 1, PaneID: "%10", TabID: "ztab_scratch", Label: "scratch"},
	}

	svc := tabstate.New(app.Runner, func(string) string { return "" })
	tgt, err := resolveTabStateTarget(app, svc, tabStateArgs{action: "ready", positional: "scratch", session: "qa-qol/main"})
	if err != nil {
		t.Fatalf("resolve tab state target: %v", err)
	}
	if tgt.PaneID != "%10" || tgt.Window != raw["main"]+":1" {
		t.Fatalf("target = %+v, want pane %%10 window %s:1", tgt, raw["main"])
	}
}

func TestTabPeerSessionFlagResolvesWorkspaceSession(t *testing.T) {
	app, mock := newTestApp(t)
	mock.InsideTmux = false
	raw := seedWorkspaceSessions(t, app, mock, "qa-qol", "main")
	mock.LogicalRows = []tmux.LogicalPaneRow{
		{Session: raw["main"], WindowID: "@1", WindowIndex: 1, WindowName: "scratch", WindowPanes: 1, PaneID: "%10", TabID: "ztab_scratch", Label: "scratch"},
	}
	mock.DisplayMessageResult = "%10\t" + raw["main"] + ":1\n"

	cmd := newTabPeerCmd(app)
	cmd.SetArgs([]string{"start", "scratch", "-s", "qa-qol/main", "--role", "claude"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("tab peer start scratch -s qa-qol/main: %v", err)
	}

	seen := false
	for _, c := range mock.Calls {
		if c.Method == "ApplyOptions" && c.Args[0] == "-p" && c.Args[1] == "%10" && c.Args[2] == tabs.OptScope && c.Args[3] == tabs.ScopePeer && c.Args[4] == "unset=false" {
			seen = true
		}
	}
	if !seen {
		t.Fatalf("tab peer did not write peer scope to resolved pane %%10, calls = %#v", mock.Calls)
	}
}

func TestPlacementSessionResolvesWorkspaceSession(t *testing.T) {
	app, mock := newTestApp(t)
	mock.InsideTmux = false
	raw := seedWorkspaceSessions(t, app, mock, "qa-qol", "main")

	got, err := placementSession(app, "qa-qol/main")
	if err != nil {
		t.Fatalf("placementSession: %v", err)
	}
	if got != raw["main"] {
		t.Fatalf("placementSession = %q, want %q", got, raw["main"])
	}
}

func TestPaneCurrentRequiresCurrentProfile(t *testing.T) {
	app, _ := newTestApp(t)
	app.Runner.(*tmux.MockRunner).InsideTmux = false
	t.Setenv("TMUX_PANE", "%99")

	cmd := newPaneCurrentCmd(app)
	if err := runPaneCurrent(app, cmd, &paneCurrentFlags{}); err == nil {
		t.Fatal("expected pane current to fail outside current profile")
	}
}

func TestWhereRequiresCurrentProfile(t *testing.T) {
	app, _ := newTestApp(t)
	app.Runner.(*tmux.MockRunner).InsideTmux = false
	t.Setenv("TMUX_PANE", "%99")

	cmd := newWhereCmd(app)
	cmd.SetArgs([]string{})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected where to fail outside current profile")
	}
}

func TestSessionKillResolvesBareUniqueLabel(t *testing.T) {
	app, mock := newTestApp(t)
	mock.InsideTmux = false
	raw := seedWorkspaceSessions(t, app, mock, "qa-qol", "worker")

	cmd := newSessionKillCmd(app)
	cmd.SetArgs([]string{"worker"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err != nil {
		t.Fatalf("session kill worker: %v", err)
	}
	kill, ok := srFindCall(mock.Calls, "KillSession")
	if !ok || kill.Args[0] != raw["worker"] {
		t.Fatalf("KillSession call = %+v ok=%v, want %q", kill, ok, raw["worker"])
	}
}

func TestSessionKillKeepsRawFallback(t *testing.T) {
	app, mock := newTestApp(t)
	mock.InsideTmux = false
	mock.Sessions = []tmux.Session{{Name: "raw-debug-session"}}

	cmd := newSessionKillCmd(app)
	cmd.SetArgs([]string{"raw-debug-session"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err != nil {
		t.Fatalf("session kill raw-debug-session: %v", err)
	}
	kill, ok := srFindCall(mock.Calls, "KillSession")
	if !ok || kill.Args[0] != "raw-debug-session" {
		t.Fatalf("KillSession call = %+v ok=%v", kill, ok)
	}
}

func TestPaneListSessionTargetKeepsRawFallback(t *testing.T) {
	app, mock := newTestApp(t)
	mock.InsideTmux = false
	mock.Sessions = []tmux.Session{{Name: "raw-debug-session"}}
	mock.Panes["raw-debug-session"] = []tmux.Pane{{ID: "%1", Session: "raw-debug-session"}}

	cmd := newPaneListCmd(app, "list")
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := runPaneList(app, cmd, &paneListFlags{target: "raw-debug-session", session: true, quiet: true}); err != nil {
		t.Fatalf("pane list raw session: %v", err)
	}
	call, ok := srFindCall(mock.Calls, "ListPanes")
	if !ok || call.Args[0] != "raw-debug-session" {
		t.Fatalf("ListPanes call = %+v ok=%v", call, ok)
	}
}

func TestPaneListSessionTargetKeepsRawPaneFallback(t *testing.T) {
	app, mock := newTestApp(t)
	mock.InsideTmux = false
	mock.Panes["%5"] = []tmux.Pane{{ID: "%5", Session: "raw-debug-session"}}

	cmd := newPaneListCmd(app, "list")
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := runPaneList(app, cmd, &paneListFlags{target: "%5", session: true, quiet: true}); err != nil {
		t.Fatalf("pane list raw pane: %v", err)
	}
	call, ok := srFindCall(mock.Calls, "ListPanes")
	if !ok || call.Args[0] != "%5" {
		t.Fatalf("ListPanes call = %+v ok=%v", call, ok)
	}
}

func TestPaneListSessionTargetRejectsMissingWorkspaceLabel(t *testing.T) {
	app, mock := newTestApp(t)
	mock.InsideTmux = false
	if err := app.WorkspaceStore.CreateWorkspace("qa-qol", "/repo"); err != nil {
		t.Fatal(err)
	}

	cmd := newPaneListCmd(app, "list")
	err := runPaneList(app, cmd, &paneListFlags{target: "qa-qol/missing", session: true, quiet: true})
	if err == nil || !strings.Contains(err.Error(), "not in workspace") {
		t.Fatalf("error = %v, want workspace/session miss", err)
	}
}

func TestResolveSessionTargetDoesNotLetRawShadowWorkspaceLabel(t *testing.T) {
	app, mock := newTestApp(t)
	mock.InsideTmux = false
	raw := seedWorkspaceSessions(t, app, mock, "qa-qol", "worker")
	mock.Sessions = append(mock.Sessions, tmux.Session{Name: "worker"})

	got, err := resolveSessionTarget(app, "worker")
	if err != nil {
		t.Fatal(err)
	}
	if got != raw["worker"] {
		t.Fatalf("resolveSessionTarget(worker) = %q, want workspace label %q", got, raw["worker"])
	}
}

func TestPlacementSessionKeepsRawFallback(t *testing.T) {
	app, mock := newTestApp(t)
	mock.InsideTmux = false
	mock.Sessions = []tmux.Session{{Name: "raw-debug-session"}}

	got, err := placementSession(app, "raw-debug-session")
	if err != nil {
		t.Fatalf("placementSession raw: %v", err)
	}
	if got != "raw-debug-session" {
		t.Fatalf("placementSession raw = %q", got)
	}
}

func TestSessionKillRepairsLegacyWorkspaceSessionName(t *testing.T) {
	app, mock := newTestApp(t)
	mock.InsideTmux = false
	if err := app.WorkspaceStore.CreateWorkspace("qa-qol", "/repo"); err != nil {
		t.Fatal(err)
	}
	rec, err := workspace.NewSessionRecord("qa-qol", "worker")
	if err != nil {
		t.Fatal(err)
	}
	rec.LegacyTmuxName = "legacy-worker"
	if err := app.WorkspaceStore.AddSessionRecord("qa-qol", rec); err != nil {
		t.Fatal(err)
	}
	mock.Sessions = []tmux.Session{{Name: rec.LegacyTmuxName}}

	cmd := newSessionKillCmd(app)
	cmd.SetArgs([]string{"qa-qol/worker"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err != nil {
		t.Fatalf("session kill repaired target: %v", err)
	}
	if rename, ok := srFindCall(mock.Calls, "RenameSession"); !ok || rename.Args[0] != rec.LegacyTmuxName || rename.Args[1] != rec.TmuxName {
		t.Fatalf("RenameSession call = %+v ok=%v, want %q -> %q", rename, ok, rec.LegacyTmuxName, rec.TmuxName)
	}
	kill, ok := srFindCall(mock.Calls, "KillSession")
	if !ok || kill.Args[0] != rec.TmuxName {
		t.Fatalf("KillSession call = %+v ok=%v, want %q", kill, ok, rec.TmuxName)
	}
}
