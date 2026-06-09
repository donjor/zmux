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
