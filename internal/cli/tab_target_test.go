package cli

import (
	"io"
	"os"
	"strings"
	"testing"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/workspace"
)

// logicalRow builds a LogicalPaneRow for choke-point tests.
func logicalRow(paneID, session, windowID string, windowIndex int, tabID, label string) tmux.LogicalPaneRow {
	return tmux.LogicalPaneRow{
		PaneID:      paneID,
		Session:     session,
		WindowID:    windowID,
		WindowIndex: windowIndex,
		WindowName:  label,
		WindowPanes: 1,
		PaneActive:  true,
		TabID:       tabID,
		Label:       label,
	}
}

// A docked tab stays addressable by label from its origin session — input
// targets the pane id, which is valid wherever the pane lives (S7).
func TestSendReachesDockedTabByLabel(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	dock := logicalRow("%5", "__zmux_dock", "@9", 0, "ztab_dock01", "logs")
	dock.Hidden = "test-session"
	mock.LogicalRows = []tmux.LogicalPaneRow{dock}

	rootCmd.SetArgs([]string{"send", "logs", "C-c", "-s", "test-session"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("send command failed: %v", err)
	}

	sent := false
	for _, c := range mock.Calls {
		if c.Method == "SendKeys" {
			if c.Args[0] != "%5" {
				t.Errorf("expected SendKeys to the docked pane %%5, got %v", c.Args)
			}
			sent = true
		}
	}
	if !sent {
		t.Fatal("expected SendKeys to the docked tab")
	}
}

// run reuse on a logical tab: input lands on the resolved pane — no duplicate
// target resolution, no window-name guess. Glyph lifecycle is shell-owned, so
// detached run does not write running state itself.
func TestRunReusesLogicalTabPane(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.LogicalRows = []tmux.LogicalPaneRow{
		logicalRow("%3", "test-session", "@2", 1, "ztab_srv001", "server"),
	}

	rootCmd.SetArgs([]string{"run", "npm run dev", "-n", "server", "-s", "test-session", "-d"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("run command failed: %v", err)
	}

	var sentToPane bool
	for _, c := range mock.Calls {
		if c.Method == "NewWindow" {
			t.Error("should reuse the logical tab, not create a window")
		}
		if c.Method == "SendKeys" && c.Args[0] == "%3" {
			sentToPane = true
		}
		if c.Method == "ApplyOptions" && c.Args[0] == "-p" && c.Args[1] == "%3" && c.Args[2] == "@zmux_state" && c.Args[3] == "running" {
			t.Errorf("detached run must not write running state directly: %v", c.Args)
		}
	}
	if !sentToPane {
		t.Errorf("expected input on the tab's pane: sent=%v", sentToPane)
	}
}

// watch resolves logical tabs read-only: captures the tab's canonical pane,
// never mutates options.
func TestWatchCapturesLogicalTabPane(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	mock.LogicalRows = []tmux.LogicalPaneRow{
		logicalRow("%8", "test-session", "@4", 2, "ztab_buddy1", "buddy"),
	}
	mock.CapturedPaneContent = "x\n"

	rootCmd.SetArgs([]string{"watch", "buddy", "-s", "test-session"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("watch command failed: %v", err)
	}

	captured := false
	for _, c := range mock.Calls {
		if c.Method == "CapturePane" && c.Args[0] == "%8" {
			captured = true
		}
		if c.Method == "ApplyOptions" || c.Method == "SetWindowOption" {
			t.Errorf("watch must not mutate options, got %s %v", c.Method, c.Args)
		}
	}
	if !captured {
		t.Fatal("expected CapturePane on the tab's pane")
	}
}

// A duplicate label inside the scope must error with the candidate ids, not
// guess a target.
func TestSendAmbiguousLabelErrors(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	mock.LogicalRows = []tmux.LogicalPaneRow{
		logicalRow("%1", "test-session", "@1", 0, "ztab_aaa111", "buddy"),
		logicalRow("%2", "test-session", "@2", 1, "ztab_bbb222", "buddy"),
	}

	rootCmd.SetArgs([]string{"send", "buddy", "C-c", "-s", "test-session"})
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
	err := rootCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected ambiguity error with ids, got %v", err)
	}
	for _, c := range mock.Calls {
		if c.Method == "SendKeys" {
			t.Errorf("must not send to a guessed target: %v", c.Args)
		}
	}
}

// tab kill on a pane-of tab kills just the pane — the host window survives.
func TestTabKillPaneOfKillsPaneOnly(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	host := logicalRow("%1", "test-session", "@2", 0, "ztab_host01", "main")
	host.WindowPanes = 2
	rider := logicalRow("%2", "test-session", "@2", 0, "ztab_rider1", "tests")
	rider.WindowPanes = 2
	rider.Anchor = "ztab_host01"
	rider.PaneActive = false
	mock.LogicalRows = []tmux.LogicalPaneRow{host, rider}

	rootCmd.SetArgs([]string{"tab", "kill", "tests"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("tab kill failed: %v", err)
	}

	killedPane := false
	for _, c := range mock.Calls {
		if c.Method == "KillWindow" || c.Method == "KillWindowByID" {
			t.Errorf("pane-of kill must not kill the window: %s %v", c.Method, c.Args)
		}
		if c.Method == "KillPane" && c.Args[0] == "%2" {
			killedPane = true
		}
	}
	if !killedPane {
		t.Fatal("expected KillPane on the rider's pane")
	}
}

// tab kill addresses the bare window index shown in `zmux tabs` (the `N:`
// column) when no name/label matches — the read-tabs → act convenience.
func TestTabKillByDisplayIndex(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Windows["test-session"] = []tmux.Window{
		{Index: 0, Name: "main", Active: true},
		{Index: 1, Name: "tests"},
	}

	rootCmd.SetArgs([]string{"tab", "kill", "1"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("tab kill by index failed: %v", err)
	}

	killed := false
	for _, c := range mock.Calls {
		if c.Method == "KillWindow" && len(c.Args) == 2 && c.Args[0] == "test-session" && c.Args[1] == "1" {
			killed = true
		}
	}
	if !killed {
		t.Fatal("expected KillWindow on window index 1")
	}
}

// tab kill on a full tab kills its window by id, guarded against the last
// window of the session.
func TestTabKillFullTabKillsWindow(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Windows["test-session"] = []tmux.Window{
		{Index: 0, Name: "main", Active: true},
		{Index: 1, Name: "tests"},
	}
	mock.LogicalRows = []tmux.LogicalPaneRow{
		logicalRow("%2", "test-session", "@5", 1, "ztab_tests1", "tests"),
	}

	rootCmd.SetArgs([]string{"tab", "kill", "tests"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("tab kill failed: %v", err)
	}

	killed := false
	for _, c := range mock.Calls {
		if c.Method == "KillWindowByID" && c.Args[0] == "@5" {
			killed = true
		}
	}
	if !killed {
		t.Fatal("expected KillWindowByID on the tab's window")
	}
}

func TestTabKillResolvesSourceSessionFlag(t *testing.T) {
	app, mock := newTestApp(t)
	source := workspace.RawSessionName("proj", "worker")
	current := workspace.RawSessionName("proj", "main")
	if err := app.WorkspaceStore.AddSession("proj", "worker"); err != nil {
		t.Fatal(err)
	}
	if err := app.WorkspaceStore.AddSession("proj", "main"); err != nil {
		t.Fatal(err)
	}
	mock.DisplayMessageResult = current
	mock.Sessions = []tmux.Session{{Name: source}, {Name: current}}
	mock.Windows[source] = []tmux.Window{
		{Index: 0, Name: "worker", Active: true},
		{Index: 1, Name: "tests"},
	}
	mock.LogicalRows = []tmux.LogicalPaneRow{
		logicalRow("%2", source, "@5", 1, "ztab_tests1", "tests"),
	}

	rootCmd := NewRootCmd(app, testVersion)
	rootCmd.SetArgs([]string{"tab", "kill", "tests", "--session", "proj/worker"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("tab kill failed: %v", err)
	}

	killed := false
	for _, c := range mock.Calls {
		if c.Method == "KillWindowByID" && c.Args[0] == "@5" {
			killed = true
		}
	}
	if !killed {
		t.Fatalf("expected KillWindowByID @5, calls = %#v", mock.Calls)
	}
}

func TestTabKillSourceSessionFlagReportsResolveError(t *testing.T) {
	app, _ := newTestApp(t)
	if err := app.WorkspaceStore.CreateWorkspace("proj", "/repo"); err != nil {
		t.Fatal(err)
	}

	rootCmd := NewRootCmd(app, testVersion)
	rootCmd.SetArgs([]string{"tab", "kill", "tests", "--session", "proj/missing"})
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
	err := rootCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "resolve source session") {
		t.Fatalf("expected source-session resolve error, got %v", err)
	}
}

func TestTabKillPaneFlagRejectsSessionFlag(t *testing.T) {
	rootCmd, _ := withMockApp(t)
	rootCmd.SetArgs([]string{"tab", "kill", "--pane", "%2", "--session", "proj/worker"})
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
	err := rootCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--session cannot be combined with --pane") {
		t.Fatalf("expected --pane/--session error, got %v", err)
	}
}

func TestTabKillPaneFlagUsesLogicalTabSemanticsAndNotifies(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Windows["test-session"] = []tmux.Window{
		{Index: 0, Name: "main", Active: true},
		{Index: 1, Name: "tests"},
	}
	mock.LogicalRows = []tmux.LogicalPaneRow{
		logicalRow("%2", "test-session", "@5", 1, "ztab_tests1", "tests"),
	}

	rootCmd.SetArgs([]string{"tab", "kill", "--pane", "%2", "--notify"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("tab kill --pane --notify failed: %v", err)
	}

	var killed, flashed bool
	for _, c := range mock.Calls {
		if c.Method == "KillWindowByID" && c.Args[0] == "@5" {
			killed = true
		}
		if c.Method == "ShowMessage" && strings.Contains(c.Args[0], "killed: tests") {
			flashed = true
		}
	}
	if !killed || !flashed {
		t.Fatalf("expected full tab kill-by-pane with notification: killed=%v flashed=%v calls=%#v", killed, flashed, mock.Calls)
	}
}

func TestTabKillLastTabGuard(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Windows["test-session"] = []tmux.Window{
		{Index: 0, Name: "main", Active: true},
	}
	mock.LogicalRows = []tmux.LogicalPaneRow{
		logicalRow("%1", "test-session", "@1", 0, "ztab_main01", "main"),
	}

	rootCmd.SetArgs([]string{"tab", "kill", "main"})
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
	err := rootCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "last tab") {
		t.Fatalf("expected last-tab guard, got %v", err)
	}
}

// tab move only moves full tabs — a pane-of tab has no window of its own.
func TestTabMoveRejectsPaneOf(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}, {Name: "other"}}
	host := logicalRow("%1", "test-session", "@2", 0, "ztab_host01", "main")
	host.WindowPanes = 2
	rider := logicalRow("%2", "test-session", "@2", 0, "ztab_rider1", "tests")
	rider.WindowPanes = 2
	rider.Anchor = "ztab_host01"
	mock.LogicalRows = []tmux.LogicalPaneRow{host, rider}

	rootCmd.SetArgs([]string{"tab", "move", "tests", "other"})
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
	err := rootCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "pane-of") {
		t.Fatalf("expected pane-of rejection, got %v", err)
	}
	for _, c := range mock.Calls {
		if c.Method == "MoveWindow" {
			t.Errorf("must not move the host window: %v", c.Args)
		}
	}
}

// tab move on a full logical tab moves its window by id.
func TestTabMoveFullTabMovesWindowByID(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}, {Name: "other"}}
	mock.Windows["test-session"] = []tmux.Window{
		{Index: 0, Name: "main", Active: true},
		{Index: 1, Name: "tests"},
	}
	mock.LogicalRows = []tmux.LogicalPaneRow{
		logicalRow("%2", "test-session", "@5", 1, "ztab_tests1", "tests"),
	}

	rootCmd.SetArgs([]string{"tab", "move", "tests", "other"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("tab move failed: %v", err)
	}

	moved := false
	for _, c := range mock.Calls {
		if c.Method == "MoveWindow" {
			if c.Args[0] != "@5" || c.Args[1] != "other:" {
				t.Errorf("expected MoveWindow @5 → other:, got %v", c.Args)
			}
			moved = true
		}
	}
	if !moved {
		t.Fatal("expected MoveWindow call")
	}
}

func TestTabMoveResolvesSourceSessionFlag(t *testing.T) {
	app, mock := newTestApp(t)
	source := workspace.RawSessionName("proj", "worker")
	dest := workspace.RawSessionName("proj", "main")
	if err := app.WorkspaceStore.AddSession("proj", "worker"); err != nil {
		t.Fatal(err)
	}
	if err := app.WorkspaceStore.AddSession("proj", "main"); err != nil {
		t.Fatal(err)
	}
	mock.DisplayMessageResult = dest
	mock.Sessions = []tmux.Session{{Name: source}, {Name: dest}}
	mock.Windows[source] = []tmux.Window{
		{Index: 0, Name: "worker", Active: true},
		{Index: 1, Name: "tests"},
	}
	mock.LogicalRows = []tmux.LogicalPaneRow{
		logicalRow("%2", source, "@5", 1, "ztab_tests1", "tests"),
	}

	rootCmd := NewRootCmd(app, testVersion)
	rootCmd.SetArgs([]string{"tab", "move", "tests", "main", "--session", "proj/worker"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("tab move failed: %v", err)
	}

	moved := false
	for _, c := range mock.Calls {
		if c.Method == "MoveWindow" {
			if c.Args[0] != "@5" || c.Args[1] != dest+":" {
				t.Errorf("expected MoveWindow @5 -> %s:, got %v", dest, c.Args)
			}
			moved = true
		}
	}
	if !moved {
		t.Fatal("expected MoveWindow call")
	}
}

func TestTabMoveResolvesDestinationWorkspaceLabel(t *testing.T) {
	app, mock := newTestApp(t)
	source := workspace.RawSessionName("proj", "main")
	dest := workspace.RawSessionName("proj", "other")
	if err := app.WorkspaceStore.AddSession("proj", "main"); err != nil {
		t.Fatal(err)
	}
	if err := app.WorkspaceStore.AddSession("proj", "other"); err != nil {
		t.Fatal(err)
	}
	mock.DisplayMessageResult = source
	mock.Sessions = []tmux.Session{{Name: source}, {Name: dest}}
	mock.Windows[source] = []tmux.Window{
		{Index: 0, Name: "main", Active: true},
		{Index: 1, Name: "tests"},
	}
	mock.LogicalRows = []tmux.LogicalPaneRow{
		logicalRow("%2", source, "@5", 1, "ztab_tests1", "tests"),
	}

	rootCmd := NewRootCmd(app, testVersion)
	rootCmd.SetArgs([]string{"tab", "move", "tests", "other"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("tab move failed: %v", err)
	}

	moved := false
	for _, c := range mock.Calls {
		if c.Method == "MoveWindow" {
			if c.Args[0] != "@5" || c.Args[1] != dest+":" {
				t.Errorf("expected MoveWindow @5 -> %s:, got %v", dest, c.Args)
			}
			moved = true
		}
	}
	if !moved {
		t.Fatal("expected MoveWindow call")
	}
}

func TestTabMoveRejectsCrossWorkspaceWithoutForce(t *testing.T) {
	app, mock := tabMoveCrossWorkspaceApp(t)
	rootCmd := NewRootCmd(app, testVersion)
	rootCmd.SetArgs([]string{"tab", "move", "tests", "tools/other"})
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true

	err := rootCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "across workspaces") {
		t.Fatalf("expected cross-workspace refusal, got %v", err)
	}
	for _, c := range mock.Calls {
		if c.Method == "MoveWindow" {
			t.Errorf("must not move across workspaces without force: %v", c.Args)
		}
	}
}

func TestTabMoveForceAllowsCrossWorkspace(t *testing.T) {
	app, mock := tabMoveCrossWorkspaceApp(t)
	dest := workspace.RawSessionName("tools", "other")
	rootCmd := NewRootCmd(app, testVersion)
	rootCmd.SetArgs([]string{"tab", "move", "tests", "tools/other", "-f"})

	var err error
	stderr := captureStderr(t, func() {
		err = rootCmd.Execute()
	})
	if err != nil {
		t.Fatalf("tab move failed: %v", err)
	}
	if !strings.Contains(stderr, "across workspaces") {
		t.Fatalf("expected cross-workspace warning, got %q", stderr)
	}

	moved := false
	for _, c := range mock.Calls {
		if c.Method == "MoveWindow" {
			if c.Args[0] != "@5" || c.Args[1] != dest+":" {
				t.Errorf("expected MoveWindow @5 -> %s:, got %v", dest, c.Args)
			}
			moved = true
		}
	}
	if !moved {
		t.Fatal("expected MoveWindow call")
	}
}

func TestResolveTabTargetWarnsWhenBareNameResolvesOutsideSession(t *testing.T) {
	app, mock := newTestApp(t)
	mock.Sessions = []tmux.Session{{Name: "source"}, {Name: "peer"}}
	mock.LogicalRows = []tmux.LogicalPaneRow{
		logicalRow("%9", "peer", "@9", 0, "ztab_peer01", "claude-peer"),
	}

	var (
		rt  resolvedTab
		err error
	)
	stderr := captureStderr(t, func() {
		rt, err = resolveTabTarget(app, "source", "claude-peer")
	})
	if err != nil {
		t.Fatalf("resolveTabTarget failed: %v", err)
	}
	if rt.Target != "%9" {
		t.Fatalf("resolveTabTarget target = %q, want %%9", rt.Target)
	}
	if !strings.Contains(stderr, "outside the current session") {
		t.Fatalf("expected cross-session warning, got %q", stderr)
	}
}

// report 039: a mutation must never resolve a tab in another session via the
// server-wide convenience. The match is dropped → in-session raw fallback, no
// "outside the current session" warning (refusing is correct, "pass -s" would
// mislead).
func TestResolveTabTargetForMutationRefusesCrossSession(t *testing.T) {
	app, mock := newTestApp(t)
	mock.Sessions = []tmux.Session{{Name: "source"}, {Name: "peer"}}
	mock.LogicalRows = []tmux.LogicalPaneRow{
		logicalRow("%9", "peer", "@9", 0, "ztab_peer01", "codex-peer"),
	}

	var (
		rt  resolvedTab
		err error
	)
	stderr := captureStderr(t, func() {
		rt, err = resolveTabTargetForMutation(app, "source", "codex-peer", "codex-peer")
	})
	if err != nil {
		t.Fatalf("resolveTabTargetForMutation failed: %v", err)
	}
	if rt.found() {
		t.Fatalf("mutation resolved a cross-session tab (Tab=%v Win=%v) — must refuse", rt.Tab, rt.Win)
	}
	if rt.Target != "source:codex-peer" {
		t.Fatalf("target = %q, want in-session raw fallback source:codex-peer", rt.Target)
	}
	if strings.Contains(stderr, "outside the current session") {
		t.Fatalf("unexpected cross-session warning on the refusal path: %q", stderr)
	}
}

// report 039: dropping the cross-session match must fall through to the
// in-session findWindow pass, NOT straight to raw fallback — otherwise a local
// legacy window of the same name is shadowed and a duplicate is created. This
// is the edge that justifies the choke-point break over a post-filter.
func TestResolveTabTargetScopedPrefersInSessionLegacyWindow(t *testing.T) {
	app, mock := newTestApp(t)
	mock.Sessions = []tmux.Session{{Name: "source"}, {Name: "peer"}}
	mock.LogicalRows = []tmux.LogicalPaneRow{
		logicalRow("%9", "peer", "@9", 0, "ztab_peer01", "build"),
	}
	mock.Windows["source"] = []tmux.Window{
		{Index: 3, Name: "build"}, // unlabeled legacy window, same name, local
	}

	rt, err := resolveTabTargetScoped(app, "source", "build", scopeSessionOnly)
	if err != nil {
		t.Fatalf("resolveTabTargetScoped failed: %v", err)
	}
	if rt.Win == nil {
		t.Fatalf("expected the in-session legacy window to resolve, got Tab=%v Target=%q", rt.Tab, rt.Target)
	}
	if rt.Target != "source:3" {
		t.Fatalf("target = %q, want the local window source:3", rt.Target)
	}
}

// report 016: a roster name (codex-peer) live in 2+ SIBLING sessions made the
// server-wide resolve ambiguous, which the create path returned as a fatal
// error — refusing to spawn locally. Under session-only scope every match is
// out-of-session, so the ambiguity must be dropped and resolution fall through
// to the in-session create fallback, exactly like the unique cross-session case.
func TestResolveTabTargetForMutationDropsCrossSessionAmbiguity(t *testing.T) {
	app, mock := newTestApp(t)
	mock.Sessions = []tmux.Session{{Name: "source"}, {Name: "peerA"}, {Name: "peerB"}}
	mock.LogicalRows = []tmux.LogicalPaneRow{
		logicalRow("%1", "peerA", "@1", 0, "ztab_peerA01", "codex-peer"),
		logicalRow("%2", "peerB", "@2", 0, "ztab_peerB01", "codex-peer"),
	}

	rt, err := resolveTabTargetForMutation(app, "source", "codex-peer", "codex-peer")
	if err != nil {
		t.Fatalf("errored on a purely cross-session ambiguity (report 016): %v", err)
	}
	if rt.found() {
		t.Fatalf("resolved a cross-session tab (Tab=%v Win=%v) — must create locally", rt.Tab, rt.Win)
	}
	if rt.Target != "source:codex-peer" {
		t.Fatalf("target = %q, want in-session raw fallback source:codex-peer", rt.Target)
	}
}

// The drop must be precise: an ambiguity that INCLUDES an in-session match is a
// real collision and must still surface — a blanket "scopeSessionOnly → break"
// would silently spawn a duplicate over it.
func TestResolveTabTargetScopedSurfacesInSessionAmbiguity(t *testing.T) {
	app, mock := newTestApp(t)
	mock.Sessions = []tmux.Session{{Name: "source"}}
	mock.LogicalRows = []tmux.LogicalPaneRow{
		logicalRow("%1", "source", "@1", 0, "ztab_src01", "dup"),
		logicalRow("%2", "source", "@2", 1, "ztab_src02", "dup"),
	}

	if _, err := resolveTabTargetScoped(app, "source", "dup", scopeSessionOnly); err == nil {
		t.Fatalf("expected an in-session ambiguity to surface, got nil")
	}
}

func tabMoveCrossWorkspaceApp(t *testing.T) (*apppkg.App, *tmux.MockRunner) {
	t.Helper()
	app, mock := newTestApp(t)
	source := workspace.RawSessionName("proj", "main")
	dest := workspace.RawSessionName("tools", "other")
	if err := app.WorkspaceStore.AddSession("proj", "main"); err != nil {
		t.Fatal(err)
	}
	if err := app.WorkspaceStore.AddSession("tools", "other"); err != nil {
		t.Fatal(err)
	}
	mock.DisplayMessageResult = source
	mock.Sessions = []tmux.Session{{Name: source}, {Name: dest}}
	mock.Windows[source] = []tmux.Window{
		{Index: 0, Name: "main", Active: true},
		{Index: 1, Name: "tests"},
	}
	mock.LogicalRows = []tmux.LogicalPaneRow{
		logicalRow("%2", source, "@5", 1, "ztab_tests1", "tests"),
	}
	return app, mock
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	os.Stderr = w
	defer func() {
		os.Stderr = oldStderr
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close stderr pipe: %v", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	return string(out)
}
