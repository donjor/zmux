package cli

import (
	"strings"
	"testing"

	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tmux"
)

// hide on a full tab is refused: only joined panes are collapsible under a
// parent tab.
func TestTabHideFullTabErrors(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	mock.LogicalRows = []tmux.LogicalPaneRow{
		logicalRow("%3", "test-session", "@5", 1, "ztab_bud001", "buddy"),
	}
	mock.DisplayMessageFunc = displayByFormat(map[string]string{
		"#{session_name}": "test-session",
		"session_group":   "\t1\t1\n",
	})

	rootCmd.SetArgs([]string{"tab", "hide", "buddy"})
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
	err := rootCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "cannot be hidden") {
		t.Fatalf("expected full-tab hide refusal, got %v", err)
	}
	for _, c := range mock.Calls {
		if c.Method == "MoveWindow" || c.Method == "BreakPane" {
			t.Errorf("full-tab hide must not move anything: %s %v", c.Method, c.Args)
		}
	}
}

// Attached grouped viewports block every placement verb (S6).
func TestTabHideBlockedByAttachedClones(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	mock.LogicalRows = []tmux.LogicalPaneRow{
		logicalRow("%3", "test-session", "@5", 1, "ztab_bud001", "buddy"),
	}
	mock.DisplayMessageFunc = displayByFormat(map[string]string{
		"#{session_name}": "test-session",
		"session_group":   "g\t2\t1\n", // grouped, attached somewhere
	})

	rootCmd.SetArgs([]string{"tab", "hide", "buddy"})
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
	err := rootCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "viewports") {
		t.Fatalf("expected clone-block error, got %v", err)
	}
	for _, c := range mock.Calls {
		if c.Method == "MoveWindow" || c.Method == "BreakPane" {
			t.Errorf("blocked hide must not move anything: %s %v", c.Method, c.Args)
		}
	}
}

// show rejoins a docked pane to its recorded parent and clears the hidden flag.
func TestTabShowRejoinsDockedPaneToParent(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	host := logicalRow("%2", "test-session", "@5", 1, "ztab_host001", "work")
	dock := logicalRow("%3", tabs.DockSession, "@7", 0, "ztab_bud001", "buddy")
	dock.Hidden = "test-session"
	dock.Anchor = "ztab_host001"
	mock.LogicalRows = []tmux.LogicalPaneRow{host, dock}
	mock.DisplayMessageFunc = displayByFormat(map[string]string{
		"#{session_name}": "test-session",
		"session_group":   "\t1\t1\n",
		"#{window_layout}\t#{window_zoomed_flag}\t#{window_panes}\t#{pane_id}": "L\t0\t1\t%2\n",
		"#{window_panes}": "2\n",
	})

	rootCmd.SetArgs([]string{"tab", "show", "buddy"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("tab show failed: %v", err)
	}

	var joined, cleared bool
	for _, c := range mock.Calls {
		if c.Method == "JoinPane" && c.Args[0] == "%3" && c.Args[1] == "%2" {
			joined = true
		}
		if c.Method == "ApplyOptions" && c.Args[0] == "-p" && c.Args[1] == "%3" &&
			c.Args[2] == tabs.OptHidden && c.Args[4] == "unset=true" {
			cleared = true
		}
	}
	if !joined || !cleared {
		t.Errorf("expected join-to-parent + hidden clear: joined=%v cleared=%v calls=%#v", joined, cleared, mock.Calls)
	}
}

func TestTabShowNumericArgTargetsHiddenIndexUnderCurrentParent(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	host := logicalRow("%2", "test-session", "@5", 1, "ztab_host001", "work")
	first := logicalRow("%3", tabs.DockSession, "@7", 0, "ztab_log001", "logs")
	first.Hidden = "test-session"
	first.Anchor = "ztab_host001"
	second := logicalRow("%4", tabs.DockSession, "@8", 1, "ztab_dbg001", "debug")
	second.Hidden = "test-session"
	second.Anchor = "ztab_host001"
	mock.LogicalRows = []tmux.LogicalPaneRow{host, first, second}
	mock.DisplayMessageFunc = displayByFormat(map[string]string{
		"#{session_name}": "test-session",
		"#{pane_id}":      "%2\n",
		"session_group":   "\t1\t1\n",
		"#{window_layout}\t#{window_zoomed_flag}\t#{window_panes}\t#{pane_id}": "L\t0\t1\t%2\n",
		"#{window_panes}": "2\n",
	})

	rootCmd.SetArgs([]string{"tab", "show", "2"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("tab show 2 failed: %v", err)
	}

	var joinedSecond, joinedFirst bool
	for _, c := range mock.Calls {
		if c.Method == "JoinPane" && c.Args[0] == "%4" && c.Args[1] == "%2" {
			joinedSecond = true
		}
		if c.Method == "JoinPane" && c.Args[0] == "%3" {
			joinedFirst = true
		}
	}
	if !joinedSecond || joinedFirst {
		t.Fatalf("expected hidden index 2 to join second pane only: joinedSecond=%v joinedFirst=%v calls=%#v", joinedSecond, joinedFirst, mock.Calls)
	}
}

func TestTabShowPaneFlagTargetsDockedPaneAndNotifies(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	host := logicalRow("%2", "test-session", "@5", 1, "ztab_host001", "work")
	dock := logicalRow("%4", tabs.DockSession, "@7", 0, "ztab_log001", "logs")
	dock.Hidden = "test-session"
	dock.Anchor = "ztab_host001"
	mock.LogicalRows = []tmux.LogicalPaneRow{host, dock}
	mock.DisplayMessageFunc = displayByFormat(map[string]string{
		"#{session_name}": "test-session",
		"session_group":   "\t1\t1\n",
		"#{window_layout}\t#{window_zoomed_flag}\t#{window_panes}\t#{pane_id}": "L\t0\t1\t%2\n",
		"#{window_panes}": "2\n",
	})

	rootCmd.SetArgs([]string{"tab", "show", "--pane", "%4", "--notify"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("tab show --pane --notify failed: %v", err)
	}

	var joined, flashed bool
	for _, c := range mock.Calls {
		if c.Method == "JoinPane" && c.Args[0] == "%4" && c.Args[1] == "%2" {
			joined = true
		}
		if c.Method == "ShowMessage" && strings.Contains(c.Args[0], "shown: logs") {
			flashed = true
		}
	}
	if !joined || !flashed {
		t.Fatalf("expected docked pane rejoined with notification: joined=%v flashed=%v calls=%#v", joined, flashed, mock.Calls)
	}
}

func TestTabShowVisibleTabErrors(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	mock.LogicalRows = []tmux.LogicalPaneRow{
		logicalRow("%3", "test-session", "@5", 1, "ztab_bud001", "buddy"),
	}
	mock.DisplayMessageFunc = displayByFormat(map[string]string{
		"#{session_name}": "test-session",
		"session_group":   "\t1\t1\n",
	})

	rootCmd.SetArgs([]string{"tab", "show", "buddy"})
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
	err := rootCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "not hidden") {
		t.Fatalf("expected not-hidden error, got %v", err)
	}
}

// Placement verbs need a logical tab — a name that resolves to nothing
// must say so rather than touch tmux.
func TestTabHideUnknownTabErrors(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	mock.DisplayMessageFunc = displayByFormat(map[string]string{
		"#{session_name}": "test-session",
	})

	rootCmd.SetArgs([]string{"tab", "hide", "ghost"})
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
	err := rootCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "not a zmux tab") {
		t.Fatalf("expected not-a-tab error, got %v", err)
	}
	for _, c := range mock.Calls {
		if c.Method == "MoveWindow" || c.Method == "BreakPane" {
			t.Errorf("must not move anything for an unknown tab: %s %v", c.Method, c.Args)
		}
	}
}

func TestTabHideDefaultsToCurrentPaneTabAndNotifies(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	host := logicalRow("%2", "test-session", "@5", 1, "ztab_wrk001", "work")
	rider := logicalRow("%3", "test-session", "@5", 1, "ztab_log001", "logs")
	rider.Anchor = "ztab_wrk001"
	mock.LogicalRows = []tmux.LogicalPaneRow{host, rider}
	mock.DisplayMessageFunc = displayByFormat(map[string]string{
		"#{pane_id}":    "%3\n",
		"session_group": "\t1\t1\n",
		"#{window_id}":  "@99\n",
	})

	rootCmd.SetArgs([]string{"tab", "hide", "--notify"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("tab hide --notify failed: %v", err)
	}

	var broke, flashed bool
	for _, c := range mock.Calls {
		if c.Method == "BreakPane" && c.Args[0] == "%3" && c.Args[1] == tabs.DockSession+":" {
			broke = true
		}
		if c.Method == "ShowMessage" && strings.Contains(c.Args[0], "hidden: logs") {
			flashed = true
		}
	}
	if !broke || !flashed {
		t.Fatalf("expected current pane hidden with notification: broke=%v flashed=%v calls=%#v", broke, flashed, mock.Calls)
	}
}

func TestTabHidePaneFlagTargetsPaneAndNotifies(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	host := logicalRow("%2", "test-session", "@2", 1, "ztab_wrk001", "work")
	clicked := logicalRow("%3", "test-session", "@2", 1, "ztab_tst001", "tests")
	clicked.Anchor = "ztab_wrk001"
	focused := logicalRow("%5", "test-session", "@5", 2, "ztab_foc001", "focus")
	mock.LogicalRows = []tmux.LogicalPaneRow{host, clicked, focused}
	mock.DisplayMessageFunc = displayByFormat(map[string]string{
		"#{pane_id}":    "%2\n",
		"session_group": "\t1\t1\n",
		"#{window_id}":  "@99\n",
	})

	rootCmd.SetArgs([]string{"tab", "hide", "--pane", "%3", "--notify"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("tab hide --pane --notify failed: %v", err)
	}

	var brokeClicked, movedFocused, flashed bool
	for _, c := range mock.Calls {
		if c.Method == "BreakPane" && c.Args[0] == "%3" && c.Args[1] == tabs.DockSession+":" {
			brokeClicked = true
		}
		if (c.Method == "MoveWindow" || c.Method == "BreakPane") && c.Args[0] == "%5" {
			movedFocused = true
		}
		if c.Method == "ShowMessage" && strings.Contains(c.Args[0], "hidden: tests") {
			flashed = true
		}
	}
	if !brokeClicked || movedFocused || !flashed {
		t.Fatalf("expected clicked pane tab hidden, not focused tab: brokeClicked=%v movedFocused=%v flashed=%v calls=%#v",
			brokeClicked, movedFocused, flashed, mock.Calls)
	}
}

// pane joins a tab into an explicit --into host: join-pane detached, anchor
// recorded on the moved pane.
func TestTabPaneJoinsIntoHost(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	mock.LogicalRows = []tmux.LogicalPaneRow{
		logicalRow("%2", "test-session", "@2", 0, "ztab_wrk001", "work"),
		logicalRow("%3", "test-session", "@5", 1, "ztab_bud001", "buddy"),
	}
	mock.DisplayMessageFunc = displayByFormat(map[string]string{
		"#{session_name}": "test-session",
		"session_group":   "\t1\t1\n",
		"#{window_layout}\t#{window_zoomed_flag}\t#{window_panes}\t#{pane_id}": "L\t0\t1\t%2\n",
		"#{window_panes}": "2\n",
	})

	rootCmd.SetArgs([]string{"tab", "pane", "buddy", "--into", "work"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("tab pane failed: %v", err)
	}

	var joined, anchored bool
	for _, c := range mock.Calls {
		if c.Method == "JoinPane" && c.Args[0] == "%3" && c.Args[1] == "%2" &&
			c.Args[2] == "right" && c.Args[4] == "detached=true" {
			joined = true
		}
		if c.Method == "ApplyOptions" && c.Args[0] == "-p" && c.Args[1] == "%3" &&
			c.Args[2] == tabs.OptAnchor && c.Args[3] == "ztab_wrk001" {
			anchored = true
		}
	}
	if !joined || !anchored {
		t.Errorf("expected join + anchor: joined=%v anchored=%v", joined, anchored)
	}
}

// Bare tab pane uses the focused pane's logical tab as the host. When the
// cursor is on a rider pane, the join anchors beside that rider instead of the
// shared window's full owner.
func TestTabPaneBareHostUsesFocusedRider(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	host := logicalRow("%2", "test-session", "@2", 1, "ztab_wrk001", "work")
	host.WindowPanes = 2
	rider := logicalRow("%4", "test-session", "@2", 1, "ztab_ride01", "rider")
	rider.WindowPanes = 2
	rider.Anchor = "ztab_wrk001"
	mock.LogicalRows = []tmux.LogicalPaneRow{
		host,
		rider,
		logicalRow("%3", "test-session", "@5", 2, "ztab_bud001", "buddy"),
	}
	mock.DisplayMessageFunc = displayByFormat(map[string]string{
		"#{session_name}": "test-session",
		"#{pane_id}":      "%4\n",
		"session_group":   "\t1\t1\n",
		"#{window_layout}\t#{window_zoomed_flag}\t#{window_panes}\t#{pane_id}": "L\t0\t2\t%4\n",
		"#{window_panes}": "3\n",
	})

	rootCmd.SetArgs([]string{"tab", "pane", "buddy"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("tab pane failed: %v", err)
	}

	var joinedBesideRider, anchoredToRider bool
	for _, c := range mock.Calls {
		if c.Method == "JoinPane" && c.Args[0] == "%3" && c.Args[1] == "%4" {
			joinedBesideRider = true
		}
		if c.Method == "ApplyOptions" && c.Args[0] == "-p" && c.Args[1] == "%3" &&
			c.Args[2] == tabs.OptAnchor && c.Args[3] == "ztab_ride01" {
			anchoredToRider = true
		}
	}
	if !joinedBesideRider || !anchoredToRider {
		t.Errorf("expected join beside focused rider: joined=%v anchored=%v", joinedBesideRider, anchoredToRider)
	}
}

func TestTabPaneFocusFlagSelectsJoinedPane(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	mock.LogicalRows = []tmux.LogicalPaneRow{
		logicalRow("%2", "test-session", "@2", 1, "ztab_wrk001", "work"),
		logicalRow("%3", "test-session", "@5", 2, "ztab_bud001", "buddy"),
	}
	mock.DisplayMessageFunc = displayByFormat(map[string]string{
		"#{session_name}": "test-session",
		"session_group":   "\t1\t1\n",
		"#{window_layout}\t#{window_zoomed_flag}\t#{window_panes}\t#{pane_id}": "L\t0\t1\t%2\n",
		"#{window_panes}": "2\n",
	})

	rootCmd.SetArgs([]string{"tab", "pane", "buddy", "--into", "work", "--focus"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("tab pane --focus failed: %v", err)
	}

	var selected bool
	for _, c := range mock.Calls {
		if c.Method == "SelectPane" && c.Args[0] == "%3" {
			selected = true
		}
	}
	if !selected {
		t.Fatalf("expected joined pane to be selected, calls=%#v", mock.Calls)
	}
}

// --notify on a successful join flashes the outcome via display-message
// instead of stdout (which run-shell would dump as a sticky takeover).
func TestTabPaneNotifyFlashesSuccess(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	mock.LogicalRows = []tmux.LogicalPaneRow{
		logicalRow("%2", "test-session", "@2", 0, "ztab_wrk001", "work"),
		logicalRow("%3", "test-session", "@5", 1, "ztab_bud001", "buddy"),
	}
	mock.DisplayMessageFunc = displayByFormat(map[string]string{
		"#{session_name}": "test-session",
		"session_group":   "\t1\t1\n",
		"#{window_layout}\t#{window_zoomed_flag}\t#{window_panes}\t#{pane_id}": "L\t0\t1\t%2\n",
		"#{window_panes}": "2\n",
	})

	rootCmd.SetArgs([]string{"tab", "pane", "buddy", "--into", "work", "--notify"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("tab pane --notify failed: %v", err)
	}

	var flashed bool
	for _, c := range mock.Calls {
		if c.Method == "ShowMessage" && strings.Contains(c.Args[0], "beside work") {
			flashed = true
		}
	}
	if !flashed {
		t.Error("expected success to be flashed via ShowMessage")
	}
}

// --notify (the prefix+J/prefix+F run-shell path) swallows the error to exit 0
// and flashes it via display-message, so tmux never shows the view-mode
// takeover the user must keypress away.
func TestTabPaneNotifyRoutesFailureToMessage(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	mock.DisplayMessageFunc = displayByFormat(map[string]string{
		"#{session_name}": "test-session",
	})
	// No logical rows → "ghost" resolves to nothing.

	rootCmd.SetArgs([]string{"tab", "pane", "--notify", "ghost"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("--notify must exit 0 even on failure, got: %v", err)
	}

	var flashed bool
	for _, c := range mock.Calls {
		if c.Method == "ShowMessage" && strings.Contains(c.Args[0], "not a zmux tab") {
			flashed = true
		}
	}
	if !flashed {
		t.Error("expected the failure to be flashed via ShowMessage")
	}
}

func TestTabSplitCreatesDetachedThenJoinsSnapshottedHost(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	mock.NewWindowPaneID = "%9"
	host := logicalRow("%2", "test-session", "@2", 1, "ztab_wrk001", "work")
	created := logicalRow("%9", "test-session", "@9", 2, "ztab_new001", "")
	mock.LogicalRowsByCall = [][]tmux.LogicalPaneRow{
		{host},          // CurrentHost snapshot before the create.
		{host, created}, // Lookup of the freshly stamped tab.
		{host, created}, // Placement epilogue reconcile.
	}
	mock.DisplayMessageFunc = displayByFormat(map[string]string{
		"#{pane_id}":           "%2\n",
		"#{pane_current_path}": "/repo/current\n",
		"session_group":        "\t1\t1\n",
		"#{window_layout}\t#{window_zoomed_flag}\t#{window_panes}\t#{pane_id}": "L\t0\t1\t%2\n",
		"#{window_panes}": "2\n",
	})

	rootCmd.SetArgs([]string{"tab", "split"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("tab split failed: %v", err)
	}

	order := callOrder(mock.Calls, "ListLogicalPaneRows", "NewWindow", "ApplyOptions", "JoinPane")
	for name, idx := range order {
		if idx < 0 {
			t.Fatalf("missing %s call in %#v", name, mock.Calls)
		}
	}
	if order["ListLogicalPaneRows"] >= order["NewWindow"] || order["NewWindow"] >= order["ApplyOptions"] || order["ApplyOptions"] >= order["JoinPane"] {
		t.Fatalf("bad split ordering, want host scan < detached create < stamp < join; got %v calls=%#v", order, mock.Calls)
	}

	var detachedCreate, joinedBesideHost, stamped bool
	for _, c := range mock.Calls {
		switch c.Method {
		case "NewWindow":
			detachedCreate = c.Args[0] == "test-session" && c.Args[1] == "" &&
				c.Args[2] == "/repo/current" && c.Args[3] == "detached=true"
		case "ApplyOptions":
			if c.Args[0] == "-p" && c.Args[1] == "%9" && c.Args[2] == tabs.OptTabID {
				stamped = true
			}
		case "JoinPane":
			joinedBesideHost = c.Args[0] == "%9" && c.Args[1] == "%2" &&
				c.Args[2] == "right" && c.Args[4] == "detached=true"
		}
	}
	if !detachedCreate || !stamped || !joinedBesideHost {
		t.Fatalf("expected detached create + stamp + join beside snapshotted host: detached=%v stamped=%v joined=%v calls=%#v",
			detachedCreate, stamped, joinedBesideHost, mock.Calls)
	}
}

func TestTabSplitFocusFlagSelectsCreatedPane(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	mock.NewWindowPaneID = "%9"
	host := logicalRow("%2", "test-session", "@2", 1, "ztab_wrk001", "work")
	createdNew := logicalRow("%9", "test-session", "@9", 2, "ztab_new001", "")
	createdJoined := logicalRow("%9", "test-session", "@2", 1, "ztab_new001", "")
	createdJoined.WindowPanes = 2
	mock.LogicalRowsByCall = [][]tmux.LogicalPaneRow{
		{host},
		{host, createdNew},
		{host, createdJoined},
	}
	mock.DisplayMessageFunc = displayByFormat(map[string]string{
		"#{pane_id}":           "%2\n",
		"#{pane_current_path}": "/repo/current\n",
		"session_group":        "\t1\t1\n",
		"#{window_layout}\t#{window_zoomed_flag}\t#{window_panes}\t#{pane_id}": "L\t0\t1\t%2\n",
		"#{window_panes}": "2\n",
	})

	rootCmd.SetArgs([]string{"tab", "split", "--focus"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("tab split --focus failed: %v", err)
	}

	var selected bool
	for _, c := range mock.Calls {
		if c.Method == "SelectPane" && c.Args[0] == "%9" {
			selected = true
		}
	}
	if !selected {
		t.Fatalf("expected created pane to be selected, calls=%#v", mock.Calls)
	}
}

func TestTabSplitNotifyRoutesFailureToMessage(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.DisplayMessageFunc = displayByFormat(map[string]string{
		"#{pane_id}": "%404\n",
	})

	rootCmd.SetArgs([]string{"tab", "split", "--notify"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("--notify must exit 0 even on failure, got: %v", err)
	}

	var flashed bool
	for _, c := range mock.Calls {
		if c.Method == "ShowMessage" && strings.Contains(c.Args[0], "current pane is not a zmux tab") {
			flashed = true
		}
		if c.Method == "NewWindow" {
			t.Fatalf("split must not create a tab when host resolution fails: %#v", mock.Calls)
		}
	}
	if !flashed {
		t.Error("expected the failure to be flashed via ShowMessage")
	}
}

func callOrder(calls []tmux.MockCall, methods ...string) map[string]int {
	out := map[string]int{}
	for _, method := range methods {
		out[method] = -1
	}
	for i, c := range calls {
		if _, ok := out[c.Method]; ok && out[c.Method] < 0 {
			out[c.Method] = i
		}
	}
	return out
}

func TestTabIndexArg(t *testing.T) {
	ok := []struct {
		in   string
		want int
	}{{"1", 1}, {"2", 2}, {"42", 42}, {"007", 7}}
	for _, c := range ok {
		if n, got := tabIndexArg(c.in); !got || n != c.want {
			t.Errorf("tabIndexArg(%q) = (%d,%v), want (%d,true)", c.in, n, got, c.want)
		}
	}
	for _, bad := range []string{"", "0", "-1", "+2", " 2", "2x", "x", "1.5", "two"} {
		if _, got := tabIndexArg(bad); got {
			t.Errorf("tabIndexArg(%q) should be rejected", bad)
		}
	}
}

// tab pane <N> joins the full tab at window index N (opt-in, placement-only).
func TestTabPaneJoinsByIndex(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	mock.LogicalRows = []tmux.LogicalPaneRow{
		logicalRow("%2", "test-session", "@2", 1, "ztab_wrk001", "work"),
		logicalRow("%3", "test-session", "@5", 2, "ztab_bud001", "buddy"),
	}
	mock.DisplayMessageFunc = displayByFormat(map[string]string{
		"#{session_name}": "test-session",
		"session_group":   "\t1\t1\n",
		"#{window_layout}\t#{window_zoomed_flag}\t#{window_panes}\t#{pane_id}": "L\t0\t1\t%2\n",
		"#{window_panes}": "2\n",
	})

	// "2" → the full tab at window index 2 (buddy), joined into work.
	rootCmd.SetArgs([]string{"tab", "pane", "2", "--into", "work"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("tab pane by index failed: %v", err)
	}
	var joined bool
	for _, c := range mock.Calls {
		if c.Method == "JoinPane" && c.Args[0] == "%3" && c.Args[1] == "%2" {
			joined = true
		}
	}
	if !joined {
		t.Errorf("expected window-index-2 tab (%%3) joined into work (%%2)")
	}
}

// A tab literally labeled "2" wins over window index 2 (numeric-label
// precedence) — id/label resolution runs before the index fallback.
func TestTabPaneNumericLabelBeatsIndex(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	mock.LogicalRows = []tmux.LogicalPaneRow{
		logicalRow("%2", "test-session", "@2", 1, "ztab_wrk001", "work"),   // host, index 1
		logicalRow("%7", "test-session", "@7", 2, "ztab_fil001", "filler"), // window index 2
		logicalRow("%3", "test-session", "@5", 3, "ztab_two001", "2"),      // labeled "2", index 3
	}
	mock.DisplayMessageFunc = displayByFormat(map[string]string{
		"#{session_name}": "test-session",
		"session_group":   "\t1\t1\n",
		"#{window_layout}\t#{window_zoomed_flag}\t#{window_panes}\t#{pane_id}": "L\t0\t1\t%2\n",
		"#{window_panes}": "2\n",
	})

	rootCmd.SetArgs([]string{"tab", "pane", "2", "--into", "work"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("tab pane label-vs-index failed: %v", err)
	}
	for _, c := range mock.Calls {
		if c.Method == "JoinPane" {
			if c.Args[0] != "%3" {
				t.Errorf("numeric label must win: joined %s, want the labeled-2 tab %%3", c.Args[0])
			}
		}
	}
}

// A tab labeled "2" in ANOTHER session must not be grabbed by index 2 in the
// current one: tabs.Resolve has a unique-server-wide fallback, so the index
// precheck must reject out-of-scope label matches (codex diff review).
func TestTabPaneIndexIgnoresCrossSessionLabel(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}, {Name: "other"}}
	mock.LogicalRows = []tmux.LogicalPaneRow{
		logicalRow("%2", "test-session", "@2", 1, "ztab_wrk001", "work"),  // host, index 1
		logicalRow("%5", "test-session", "@5", 2, "ztab_idx002", "buddy"), // index 2 here
		logicalRow("%9", "other", "@9", 1, "ztab_two099", "2"),            // labeled "2" elsewhere
	}
	mock.DisplayMessageFunc = displayByFormat(map[string]string{
		"#{session_name}": "test-session",
		"session_group":   "\t1\t1\n",
		"#{window_layout}\t#{window_zoomed_flag}\t#{window_panes}\t#{pane_id}": "L\t0\t1\t%2\n",
		"#{window_panes}": "2\n",
	})

	rootCmd.SetArgs([]string{"tab", "pane", "2", "--into", "work"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("tab pane cross-session index failed: %v", err)
	}
	for _, c := range mock.Calls {
		if c.Method == "JoinPane" && c.Args[0] != "%5" {
			t.Errorf("index 2 must pick this session's tab %%5, not cross-session label-2: got %s", c.Args[0])
		}
	}
}

func TestTabPaneCloneBlocked(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	mock.LogicalRows = []tmux.LogicalPaneRow{
		logicalRow("%2", "test-session", "@2", 0, "ztab_wrk001", "work"),
		logicalRow("%3", "test-session", "@5", 1, "ztab_bud001", "buddy"),
	}
	mock.DisplayMessageFunc = displayByFormat(map[string]string{
		"#{session_name}": "test-session",
		"session_group":   "g\t2\t1\n", // grouped + attached → blocked
	})

	rootCmd.SetArgs([]string{"tab", "pane", "buddy", "--into", "work"})
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
	err := rootCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "viewports") {
		t.Fatalf("expected clone-block error, got %v", err)
	}
	for _, c := range mock.Calls {
		if c.Method == "JoinPane" {
			t.Errorf("blocked pane verb must not join: %v", c.Args)
		}
	}
}

// full promotes a rider out of its host into an appended window in the same
// session, clearing the advisory anchor.
func TestTabFullPromotesRider(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	mock.BreakPaneWindowID = "@7"
	host := logicalRow("%2", "test-session", "@2", 0, "ztab_wrk001", "work")
	host.WindowPanes = 2
	rider := logicalRow("%3", "test-session", "@2", 0, "ztab_tst001", "tests")
	rider.WindowPanes = 2
	rider.Anchor = "ztab_wrk001"
	mock.LogicalRows = []tmux.LogicalPaneRow{host, rider}
	mock.DisplayMessageFunc = displayByFormat(map[string]string{
		"#{session_name}": "test-session",
		"session_group":   "\t1\t1\n",
		"#{window_layout}\t#{window_zoomed_flag}\t#{window_panes}\t#{pane_id}": "L\t0\t2\t%2\n",
		"#{window_panes}": "1\n",
	})

	rootCmd.SetArgs([]string{"tab", "full", "tests"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("tab full failed: %v", err)
	}

	var broke, unanchored bool
	for _, c := range mock.Calls {
		if c.Method == "BreakPane" && c.Args[0] == "%3" && c.Args[1] == "test-session:" &&
			c.Args[2] == "tests" && c.Args[4] == "detached=true" {
			broke = true
		}
		if c.Method == "ApplyOptions" && c.Args[0] == "-p" && c.Args[1] == "%3" &&
			c.Args[2] == tabs.OptAnchor && c.Args[4] == "unset=true" {
			unanchored = true
		}
	}
	if !broke || !unanchored {
		t.Errorf("expected break + anchor clear: broke=%v unanchored=%v", broke, unanchored)
	}
}

func TestTabFullDefaultsToCurrentPaneTab(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	mock.BreakPaneWindowID = "@7"
	host := logicalRow("%2", "test-session", "@2", 0, "ztab_wrk001", "work")
	host.WindowPanes = 2
	rider := logicalRow("%3", "test-session", "@2", 0, "ztab_tst001", "tests")
	rider.WindowPanes = 2
	rider.Anchor = "ztab_wrk001"
	mock.LogicalRows = []tmux.LogicalPaneRow{host, rider}
	mock.DisplayMessageFunc = displayByFormat(map[string]string{
		"#{session_name}": "test-session",
		"#{pane_id}":      "%3\n",
		"session_group":   "\t1\t1\n",
		"#{window_layout}\t#{window_zoomed_flag}\t#{window_panes}\t#{pane_id}": "L\t0\t2\t%2\n",
		"#{window_panes}": "1\n",
	})

	rootCmd.SetArgs([]string{"tab", "full", "--after"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("tab full failed: %v", err)
	}

	var broke bool
	for _, c := range mock.Calls {
		if c.Method == "BreakPane" && c.Args[0] == "%3" && c.Args[1] == "@2" &&
			c.Args[2] == "tests" && c.Args[3] == "after=true" {
			broke = true
		}
	}
	if !broke {
		t.Errorf("expected no-arg full to promote current pane-tab after host")
	}
}

func TestTabFullPaneFlagTargetsPaneAndNotifies(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	mock.BreakPaneWindowID = "@7"
	host := logicalRow("%2", "test-session", "@2", 0, "ztab_wrk001", "work")
	host.WindowPanes = 2
	rider := logicalRow("%3", "test-session", "@2", 0, "ztab_tst001", "tests")
	rider.WindowPanes = 2
	rider.Anchor = "ztab_wrk001"
	mock.LogicalRows = []tmux.LogicalPaneRow{host, rider}
	mock.DisplayMessageFunc = displayByFormat(map[string]string{
		"#{session_name}": "test-session",
		"#{pane_id}":      "%2\n",
		"session_group":   "\t1\t1\n",
		"#{window_layout}\t#{window_zoomed_flag}\t#{window_panes}\t#{pane_id}": "L\t0\t2\t%2\n",
		"#{window_panes}": "1\n",
	})

	rootCmd.SetArgs([]string{"tab", "full", "--pane", "%3", "--after", "--notify"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("tab full --pane --notify failed: %v", err)
	}

	var brokeClicked, brokeFocused, flashed bool
	for _, c := range mock.Calls {
		if c.Method == "BreakPane" && c.Args[0] == "%3" && c.Args[1] == "@2" &&
			c.Args[2] == "tests" && c.Args[3] == "after=true" {
			brokeClicked = true
		}
		if c.Method == "BreakPane" && c.Args[0] == "%2" {
			brokeFocused = true
		}
		if c.Method == "ShowMessage" && strings.Contains(c.Args[0], "full: tests") {
			flashed = true
		}
	}
	if !brokeClicked || brokeFocused || !flashed {
		t.Fatalf("expected clicked pane tab promoted, not focused tab: brokeClicked=%v brokeFocused=%v flashed=%v calls=%#v",
			brokeClicked, brokeFocused, flashed, mock.Calls)
	}
}

func TestTabFullOnFullTabErrors(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	mock.LogicalRows = []tmux.LogicalPaneRow{
		logicalRow("%3", "test-session", "@5", 1, "ztab_bud001", "buddy"),
	}
	mock.DisplayMessageFunc = displayByFormat(map[string]string{
		"#{session_name}": "test-session",
		"session_group":   "\t1\t1\n",
	})

	rootCmd.SetArgs([]string{"tab", "full", "buddy"})
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
	err := rootCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "already a full tab") {
		t.Fatalf("expected already-full error, got %v", err)
	}
}

func TestTabPaneConflictingDirectionsError(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	mock.DisplayMessageFunc = displayByFormat(map[string]string{
		"#{session_name}": "test-session",
	})

	rootCmd.SetArgs([]string{"tab", "pane", "buddy", "--into", "work", "--left", "--down"})
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
	err := rootCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "one direction") {
		t.Fatalf("expected direction-conflict error, got %v", err)
	}
}
