package tabs

import (
	"strings"
	"testing"

	"github.com/donjor/zmux/internal/tmux"
)

func TestDisplayNameFallback(t *testing.T) {
	cases := []struct {
		name string
		tab  LogicalTab
		want string
	}{
		{"label wins", LogicalTab{Label: "work", WindowName: "vim", ID: "ztab_a"}, "work"},
		{"window name when unlabeled", LogicalTab{WindowName: "bash", ID: "ztab_b"}, "bash"},
		{"id only as last resort", LogicalTab{ID: "ztab_c"}, "ztab_c"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DisplayName(&c.tab); got != c.want {
				t.Errorf("DisplayName = %q, want %q", got, c.want)
			}
		})
	}
}

// S6 predicate truth table: group name alone never blocks (it persists after
// clone death); size>1 without attachments never blocks (gates on attached
// only, by design); both together do.
func TestCloneBlockedPredicate(t *testing.T) {
	cases := []struct {
		name string
		out  string
		want bool
	}{
		{"ungrouped", "\t1\t1", false},
		{"stale group name, size 1", "g\t1\t1", false},
		{"clones exist, nobody attached", "g\t2\t0", false},
		{"clones exist, attached", "g\t2\t1", true},
		{"headless nested clone (grpatt only)", "g\t2\t1", true},
	}
	for _, tc := range cases {
		mock := tmux.NewMockRunner()
		mock.DisplayMessageResult = tc.out + "\n"
		got, err := CloneBlocked(mock, "dev")
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got != tc.want {
			t.Errorf("%s: CloneBlocked=%v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestEnsureDockRefusesUnmarkedCollision(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.Sessions = []tmux.Session{{Name: DockSession}}
	mock.DisplayMessageResult = "\n" // no @zmux_dock mark — not ours

	if _, err := EnsureDock(mock); err == nil || !strings.Contains(err.Error(), "not zmux's dock") {
		t.Fatalf("expected unmarked-collision refusal, got %v", err)
	}
	for _, c := range mock.Calls {
		if c.Method == "NewSession" {
			t.Errorf("must not create over a collision: %v", c.Args)
		}
	}
}

func TestEnsureDockLazyCreateMarksAndReportsPlaceholder(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.DisplayMessageResult = "@99\n" // placeholder window id after create

	placeholder, err := EnsureDock(mock)
	if err != nil {
		t.Fatalf("EnsureDock: %v", err)
	}
	if placeholder != "@99" {
		t.Errorf("placeholder = %q, want @99", placeholder)
	}
	var created, marked bool
	for _, c := range mock.Calls {
		if c.Method == "NewSession" && c.Args[0] == DockSession {
			created = true
		}
		if c.Method == "SetSessionOption" && c.Args[0] == DockSession && c.Args[1] == OptDockMark && c.Args[2] == "1" {
			marked = true
		}
	}
	if !created || !marked {
		t.Errorf("lazy create must create+mark: created=%v marked=%v", created, marked)
	}
}

func TestEnsureDockExistingMarkedIsNoop(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.Sessions = []tmux.Session{{Name: DockSession}}
	mock.DisplayMessageResult = "1\n" // marked ours

	placeholder, err := EnsureDock(mock)
	if err != nil || placeholder != "" {
		t.Fatalf("existing marked dock: placeholder=%q err=%v", placeholder, err)
	}
	for _, c := range mock.Calls {
		if c.Method == "NewSession" {
			t.Errorf("must not recreate an existing dock: %v", c.Args)
		}
	}
}

func TestHideFullErrors(t *testing.T) {
	mock := tmux.NewMockRunner()
	tab := &LogicalTab{
		ID: "ztab_a", Label: "buddy", PaneID: "%3", Session: "dev",
		OriginSession: "dev", WindowID: "@5", Placement: PlacementFull,
	}

	if err := Hide(mock, tab); err == nil || !strings.Contains(err.Error(), "cannot be hidden") {
		t.Fatalf("expected full-tab hide refusal, got %v", err)
	}
	for _, c := range mock.Calls {
		if c.Method == "MoveWindow" || c.Method == "BreakPane" {
			t.Errorf("full-tab hide must not move anything: %s %v", c.Method, c.Args)
		}
	}
}

// hide(pane-of) = break the pane out into its own detached dock window.
func TestHidePaneOfBreaksPane(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.Sessions = []tmux.Session{{Name: DockSession}}
	mock.DisplayMessageResult = "1\n" // dock exists, marked
	tab := &LogicalTab{
		ID: "ztab_r", Label: "tests", PaneID: "%7", Session: "dev",
		OriginSession: "dev", WindowID: "@5", Placement: PlacementPaneOf, AnchorID: "ztab_h",
	}

	if err := Hide(mock, tab); err != nil {
		t.Fatalf("Hide: %v", err)
	}
	var broke bool
	for _, c := range mock.Calls {
		if c.Method == "MoveWindow" {
			t.Error("pane-of tabs break their pane out, never move the host window")
		}
		if c.Method == "BreakPane" {
			broke = true
		}
	}
	if !broke {
		t.Error("expected BreakPane into the dock")
	}
}

func TestHideDockedErrors(t *testing.T) {
	mock := tmux.NewMockRunner()
	tab := &LogicalTab{
		ID: "ztab_a", PaneID: "%3", Session: DockSession,
		OriginSession: "dev", WindowID: "@5", Placement: PlacementDock,
	}
	if err := Hide(mock, tab); err == nil || !strings.Contains(err.Error(), "already hidden") {
		t.Fatalf("expected already-hidden error, got %v", err)
	}
}

func TestShowRejoinsToRecordedParentAndClearsHidden(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.Sessions = []tmux.Session{{Name: "dev"}}
	mock.DisplayMessageFunc = displayByTarget(map[[2]string]string{
		{"@2", snapFmt}:           "L\t0\t1\t%2\n",
		{"@2", "#{window_panes}"}: "2\n",
	})
	mock.LogicalRows = []tmux.LogicalPaneRow{
		{PaneID: "%2", Session: "dev", WindowID: "@2", WindowIndex: 1, WindowName: "work", WindowPanes: 1, TabID: "ztab_h", Label: "work"},
		{PaneID: "%3", Session: DockSession, WindowID: "@7", WindowIndex: 0, WindowName: "buddy", WindowPanes: 1, TabID: "ztab_a", Label: "buddy", Anchor: "ztab_h", Hidden: "dev"},
	}
	tab := &LogicalTab{
		ID: "ztab_a", Label: "buddy", PaneID: "%3", Session: DockSession,
		OriginSession: "dev", WindowID: "@7", Placement: PlacementDock, AnchorID: "ztab_h",
	}

	origin, err := Show(mock, tab)
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if origin != "dev" {
		t.Errorf("origin = %q, want dev", origin)
	}
	var joined, cleared bool
	for _, c := range mock.Calls {
		if c.Method == "JoinPane" && c.Args[0] == "%3" && c.Args[1] == "%2" {
			joined = true
		}
		if c.Method == "ApplyOptions" && c.Args[0] == "-p" && c.Args[1] == "%3" &&
			c.Args[2] == OptHidden && c.Args[4] == "unset=true" {
			cleared = true
		}
	}
	if !joined || !cleared {
		t.Errorf("joined=%v hiddenCleared=%v calls=%#v", joined, cleared, mock.Calls)
	}
}

func TestShowNotHiddenErrors(t *testing.T) {
	mock := tmux.NewMockRunner()
	tab := &LogicalTab{
		ID: "ztab_a", PaneID: "%3", Session: "dev",
		OriginSession: "dev", WindowID: "@5", Placement: PlacementFull,
	}
	if _, err := Show(mock, tab); err == nil || !strings.Contains(err.Error(), "not hidden") {
		t.Fatalf("expected not-hidden error, got %v", err)
	}
}

func TestShowDeadOriginErrors(t *testing.T) {
	mock := tmux.NewMockRunner() // no sessions — origin is gone
	tab := &LogicalTab{
		ID: "ztab_a", PaneID: "%3", Session: DockSession,
		OriginSession: "dev", WindowID: "@7", Placement: PlacementDock,
	}
	if _, err := Show(mock, tab); err == nil || !strings.Contains(err.Error(), "gone") {
		t.Fatalf("expected dead-origin error, got %v", err)
	}
	for _, c := range mock.Calls {
		if c.Method == "MoveWindow" {
			t.Errorf("must not move toward a dead origin: %v", c.Args)
		}
	}
}
