package tabs

import (
	"strings"
	"testing"

	"github.com/donjor/zmux/internal/tmux"
)

const snapFmt = "#{window_layout}\t#{window_zoomed_flag}\t#{window_panes}\t#{pane_id}"

// displayByTarget routes mock DisplayMessage replies by (target, format) —
// snapshot and restore hit the same windows with different formats.
func displayByTarget(routes map[[2]string]string) func(target, format string) (string, error) {
	return func(target, format string) (string, error) {
		return routes[[2]string{target, format}], nil
	}
}

func fullTab(id, pane, session, window string, panes int) *LogicalTab {
	return &LogicalTab{
		ID: id, Label: id, PaneID: pane, Session: session,
		OriginSession: session, WindowID: window, WindowPanes: panes,
		Placement: PlacementFull,
	}
}

func TestJoinMovesPaneAndAnchors(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.DisplayMessageFunc = displayByTarget(map[[2]string]string{
		{"@2", snapFmt}:           "L2\t0\t1\t%2\n",
		{"@2", "#{window_panes}"}: "2\n", // host gained a pane — counts differ
	})
	src := fullTab("buddy", "%1", "dev", "@1", 1)
	host := fullTab("work", "%2", "dev", "@2", 1)

	warnings, err := Join(mock, src, host, JoinOptions{Direction: tmux.SplitRight, Size: "40%"})
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}

	var joined, anchored bool
	for _, c := range mock.Calls {
		if c.Method == "JoinPane" && c.Args[0] == "%1" && c.Args[1] == "%2" &&
			c.Args[2] == "right" && c.Args[3] == "40%" && c.Args[4] == "detached=true" {
			joined = true
		}
		if c.Method == "ApplyOptions" && c.Args[0] == "-p" && c.Args[1] == "%1" &&
			c.Args[2] == OptAnchor && c.Args[3] == "work" {
			anchored = true
		}
		if c.Method == "SelectLayout" {
			t.Errorf("layout restore must skip on pane-count mismatch: %v", c.Args)
		}
		if c.Method == "ToggleZoom" {
			t.Errorf("nothing was zoomed — no re-zoom: %v", c.Args)
		}
	}
	if !joined || !anchored {
		t.Errorf("want join + anchor write: joined=%v anchored=%v", joined, anchored)
	}
}

func assertNoPeerMetadataWrites(t *testing.T, mock *tmux.MockRunner) {
	t.Helper()
	peerKeys := map[string]bool{
		OptTurnState: true, OptTurnAt: true, OptPeerRole: true, OptPeerHostTab: true,
		OptPeerHostPane: true, OptPeerTopic: true, OptPeerTurns: true, OptPeerLastTurn: true,
		OptKeepUntil: true, OptParkUntil: true,
	}
	for _, c := range mock.Calls {
		if c.Method == "ApplyOptions" && peerKeys[c.Args[2]] {
			t.Fatalf("placement operation touched peer metadata %s: %#v", c.Args[2], c.Args)
		}
	}
}

func TestPlacementMovesPreservePeerMetadataOptions(t *testing.T) {
	joinMock := tmux.NewMockRunner()
	joinMock.DisplayMessageFunc = displayByTarget(map[[2]string]string{
		{"@2", snapFmt}:           "L2\t0\t1\t%2\n",
		{"@2", "#{window_panes}"}: "2\n",
	})
	peer := fullTab("claude-peer", "%1", "dev", "@1", 1)
	host := fullTab("work", "%2", "dev", "@2", 1)
	if _, err := Join(joinMock, peer, host, JoinOptions{}); err != nil {
		t.Fatalf("Join peer tab: %v", err)
	}
	assertNoPeerMetadataWrites(t, joinMock)

	promoteMock := tmux.NewMockRunner()
	promoteMock.BreakPaneWindowID = "@7"
	promoteMock.DisplayMessageFunc = displayByTarget(map[[2]string]string{
		{"@2", snapFmt}:           "HL\t0\t2\t%2\n",
		{"@2", "#{window_panes}"}: "1\n",
	})
	rider := fullTab("claude-peer", "%3", "dev", "@2", 2)
	rider.Placement = PlacementPaneOf
	if _, _, err := Promote(promoteMock, rider, false); err != nil {
		t.Fatalf("Promote peer tab: %v", err)
	}
	assertNoPeerMetadataWrites(t, promoteMock)
}

func TestJoinDockedTabClearsHidden(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.DisplayMessageFunc = displayByTarget(map[[2]string]string{
		{"@2", snapFmt}:           "L2\t0\t1\t%2\n",
		{"@2", "#{window_panes}"}: "2\n",
	})
	src := fullTab("logs", "%5", DockSession, "@9", 1)
	src.Placement = PlacementDock
	src.OriginSession = "dev"
	host := fullTab("work", "%2", "dev", "@2", 1)

	if _, err := Join(mock, src, host, JoinOptions{}); err != nil {
		t.Fatalf("Join from dock: %v", err)
	}

	var hiddenCleared bool
	for _, c := range mock.Calls {
		if c.Method == "ApplyOptions" && c.Args[1] == "%5" &&
			c.Args[2] == OptHidden && c.Args[4] == "unset=true" {
			hiddenCleared = true
		}
	}
	if !hiddenCleared {
		t.Error("joining out of the dock must clear @zmux_hidden")
	}
}

func TestJoinRejectsBadHosts(t *testing.T) {
	mock := tmux.NewMockRunner()
	src := fullTab("buddy", "%1", "dev", "@1", 1)

	self := *src
	if _, err := Join(mock, src, &self, JoinOptions{}); err == nil ||
		!strings.Contains(err.Error(), "join itself") {
		t.Errorf("self-join must error, got %v", err)
	}

	docked := fullTab("logs", "%5", DockSession, "@9", 1)
	docked.Placement = PlacementDock
	if _, err := Join(mock, src, docked, JoinOptions{}); err == nil ||
		!strings.Contains(err.Error(), "hidden") {
		t.Errorf("dock host must error, got %v", err)
	}

	sibling := fullTab("twin", "%8", "dev", "@1", 2)
	if _, err := Join(mock, src, sibling, JoinOptions{}); err == nil ||
		!strings.Contains(err.Error(), "already shares") {
		t.Errorf("same-window join must error, got %v", err)
	}

	for _, c := range mock.Calls {
		if c.Method == "JoinPane" {
			t.Errorf("rejected joins must never reach tmux: %v", c.Args)
		}
	}
}

// A multi-pane source window that stays behind gets the S2 restore: layout
// skipped on count change, the previously-zoomed surviving pane re-zoomed.
func TestJoinRestoresSourceZoom(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.DisplayMessageFunc = displayByTarget(map[[2]string]string{
		{"@2", snapFmt}:           "HL\t0\t1\t%2\n",
		{"@2", "#{window_panes}"}: "2\n",
		{"@1", snapFmt}:           "SL\t1\t3\t%9\n", // zoomed on a raw sibling
		{"@1", "#{window_panes}"}: "2\n",            // tab pane left — 3 → 2
	})
	src := fullTab("buddy", "%1", "dev", "@1", 3)
	host := fullTab("work", "%2", "dev", "@2", 1)

	if _, err := Join(mock, src, host, JoinOptions{}); err != nil {
		t.Fatalf("Join: %v", err)
	}

	var rezoomed bool
	for _, c := range mock.Calls {
		if c.Method == "ToggleZoom" && c.Args[0] == "%9" {
			rezoomed = true
		}
		if c.Method == "SelectLayout" {
			t.Errorf("source lost a pane — layout restore must skip: %v", c.Args)
		}
	}
	if !rezoomed {
		t.Error("source window's surviving zoomed pane must be re-zoomed")
	}
}

func TestPromoteBreaksRiderOut(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.BreakPaneWindowID = "@7"
	mock.DisplayMessageFunc = displayByTarget(map[[2]string]string{
		{"@2", snapFmt}:           "HL\t0\t2\t%2\n",
		{"@2", "#{window_panes}"}: "1\n",
	})
	rider := fullTab("tests", "%3", "dev", "@2", 2)
	rider.Placement = PlacementPaneOf
	rider.AnchorID = "work"

	windowID, warnings, err := Promote(mock, rider, false)
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	if windowID != "@7" {
		t.Errorf("windowID = %q, want @7", windowID)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}

	var broke, unanchored bool
	for _, c := range mock.Calls {
		if c.Method == "BreakPane" && c.Args[0] == "%3" && c.Args[1] == "dev:" &&
			c.Args[2] == "tests" && c.Args[3] == "after=false" && c.Args[4] == "detached=true" {
			broke = true
		}
		if c.Method == "ApplyOptions" && c.Args[1] == "%3" &&
			c.Args[2] == OptAnchor && c.Args[4] == "unset=true" {
			unanchored = true
		}
	}
	if !broke || !unanchored {
		t.Errorf("want break + anchor clear: broke=%v unanchored=%v", broke, unanchored)
	}
}

func TestPromoteAfterInsertsBehindHost(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.BreakPaneWindowID = "@7"
	mock.DisplayMessageFunc = displayByTarget(map[[2]string]string{
		{"@2", snapFmt}:           "HL\t0\t2\t%2\n",
		{"@2", "#{window_panes}"}: "1\n",
	})
	rider := fullTab("tests", "%3", "dev", "@2", 2)
	rider.Placement = PlacementPaneOf

	if _, _, err := Promote(mock, rider, true); err != nil {
		t.Fatalf("Promote --after: %v", err)
	}

	var afterHost bool
	for _, c := range mock.Calls {
		if c.Method == "BreakPane" && c.Args[1] == "@2" && c.Args[3] == "after=true" {
			afterHost = true
		}
	}
	if !afterHost {
		t.Error("--after must break-pane -a targeting the old host window")
	}
}

func TestPromoteRejectsFullAndPromotesDocked(t *testing.T) {
	mock := tmux.NewMockRunner()

	full := fullTab("buddy", "%1", "dev", "@1", 1)
	if _, _, err := Promote(mock, full, false); err == nil ||
		!strings.Contains(err.Error(), "already a full tab") {
		t.Errorf("full promote must error, got %v", err)
	}

	docked := fullTab("logs", "%5", DockSession, "@9", 1)
	docked.Placement = PlacementDock
	docked.OriginSession = "dev"
	mock.Sessions = []tmux.Session{{Name: "dev"}}
	if windowID, warnings, err := Promote(mock, docked, false); err != nil || windowID != "@9" || len(warnings) != 0 {
		t.Fatalf("docked promote = windowID %q warnings %v err %v, want @9/no warnings/nil", windowID, warnings, err)
	}
	var moved, hiddenCleared, anchorCleared bool
	for _, c := range mock.Calls {
		if c.Method == "MoveWindow" && c.Args[0] == "@9" && c.Args[1] == "dev:" {
			moved = true
		}
		if c.Method == "ApplyOptions" && c.Args[1] == "%5" && c.Args[2] == OptHidden && c.Args[4] == "unset=true" {
			hiddenCleared = true
		}
		if c.Method == "ApplyOptions" && c.Args[1] == "%5" && c.Args[2] == OptAnchor && c.Args[4] == "unset=true" {
			anchorCleared = true
		}
	}
	if !moved || !hiddenCleared || !anchorCleared {
		t.Errorf("docked promote incomplete: moved=%v hiddenCleared=%v anchorCleared=%v calls=%#v", moved, hiddenCleared, anchorCleared, mock.Calls)
	}
}

// Breaking a rider out of a host that was zoomed on ANOTHER pane re-zooms
// that pane (still multi-pane after the break).
func TestPromoteRestoresHostZoom(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.BreakPaneWindowID = "@7"
	mock.DisplayMessageFunc = displayByTarget(map[[2]string]string{
		{"@2", snapFmt}:           "HL\t1\t3\t%2\n", // zoomed on the host's own pane
		{"@2", "#{window_panes}"}: "2\n",
	})
	rider := fullTab("tests", "%3", "dev", "@2", 3)
	rider.Placement = PlacementPaneOf

	if _, _, err := Promote(mock, rider, false); err != nil {
		t.Fatalf("Promote: %v", err)
	}

	var rezoomed bool
	for _, c := range mock.Calls {
		if c.Method == "ToggleZoom" && c.Args[0] == "%2" {
			rezoomed = true
		}
	}
	if !rezoomed {
		t.Error("host's zoomed pane must be re-zoomed after the break")
	}
}
