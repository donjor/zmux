package cli

import (
	"strings"
	"testing"

	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tmux"
)

// hide on a full tab: whole window moves into the (lazily created) dock,
// the origin session is recorded on the pane.
func TestTabHideFullTabMovesToDock(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	mock.LogicalRows = []tmux.LogicalPaneRow{
		logicalRow("%3", "test-session", "@5", 1, "ztab_bud001", "buddy"),
	}
	mock.DisplayMessageFunc = displayByFormat(map[string]string{
		"#{session_name}": "test-session",
		"session_group":   "\t1\t1\n", // ungrouped — not clone-blocked
		"#{window_id}":    "@99\n",    // fresh dock placeholder
	})

	rootCmd.SetArgs([]string{"tab", "hide", "buddy"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("tab hide failed: %v", err)
	}

	var moved, hiddenSet bool
	for _, c := range mock.Calls {
		if c.Method == "MoveWindow" && c.Args[0] == "@5" && c.Args[1] == tabs.DockSession+":" {
			moved = true
		}
		if c.Method == "ApplyOptions" && c.Args[0] == "-p" && c.Args[1] == "%3" &&
			c.Args[2] == tabs.OptHidden && c.Args[3] == "test-session" {
			hiddenSet = true
		}
	}
	if !moved || !hiddenSet {
		t.Errorf("expected move-to-dock + origin record: moved=%v hidden=%v", moved, hiddenSet)
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

// show returns a docked tab (resolved from its origin scope) to the origin
// session and clears the hidden flag.
func TestTabShowReturnsDockedTab(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	dock := logicalRow("%3", tabs.DockSession, "@7", 0, "ztab_bud001", "buddy")
	dock.Hidden = "test-session"
	mock.LogicalRows = []tmux.LogicalPaneRow{dock}
	mock.DisplayMessageFunc = displayByFormat(map[string]string{
		"#{session_name}": "test-session",
		"session_group":   "\t1\t1\n",
	})

	rootCmd.SetArgs([]string{"tab", "show", "buddy"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("tab show failed: %v", err)
	}

	var moved, cleared bool
	for _, c := range mock.Calls {
		if c.Method == "MoveWindow" && c.Args[0] == "@7" && c.Args[1] == "test-session:" {
			moved = true
		}
		if c.Method == "ApplyOptions" && c.Args[0] == "-p" && c.Args[1] == "%3" &&
			c.Args[2] == tabs.OptHidden && c.Args[4] == "unset=true" {
			cleared = true
		}
	}
	if !moved || !cleared {
		t.Errorf("expected move-to-origin + hidden clear: moved=%v cleared=%v", moved, cleared)
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
