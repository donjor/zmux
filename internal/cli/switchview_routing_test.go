package cli

import (
	"testing"

	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui/tabpicker"
)

// These tests pin the workspace-independent-views behavior: in-tmux switches
// route through session.SwitchView, so switching to a session attached
// elsewhere yields an independent grouped clone, and tab selection targets the
// session actually viewed (never the shared root).

func TestCycleWorkspaceSessionClonesAttachedSibling(t *testing.T) {
	app, mock := newTestApp(t)
	main := addFallbackSession(t, app, "proj", "main")
	other := addFallbackSession(t, app, "proj", "other")
	mock.DisplayMessageFunc = displayByFormat(map[string]string{"#{session_name}": main})
	mock.Sessions = []tmux.Session{
		{Name: main, Attached: true},
		{Name: other, Attached: true}, // sibling attached elsewhere → must clone
	}

	if err := cycleWorkspaceSession(app, 1); err != nil {
		t.Fatalf("cycleWorkspaceSession: %v", err)
	}
	clone := other + "__clone_b"
	if !fallbackMockHasCall(mock.Calls, "NewGroupedSession", other, clone) {
		t.Errorf("expected clone of attached sibling %s, calls = %v", other, mock.Calls)
	}
	if !fallbackMockHasCall(mock.Calls, "SwitchClient", clone) {
		t.Error("expected switch to the clone, not the shared root")
	}
}

func TestCycleWorkspaceSessionPlainWhenSiblingFree(t *testing.T) {
	app, mock := newTestApp(t)
	main := addFallbackSession(t, app, "proj", "main")
	other := addFallbackSession(t, app, "proj", "other")
	mock.DisplayMessageFunc = displayByFormat(map[string]string{"#{session_name}": main})
	mock.Sessions = []tmux.Session{
		{Name: main, Attached: true},
		{Name: other, Attached: false}, // free → plain switch, no clone
	}

	if err := cycleWorkspaceSession(app, 1); err != nil {
		t.Fatalf("cycleWorkspaceSession: %v", err)
	}
	if !fallbackMockHasCall(mock.Calls, "SwitchClient", other) {
		t.Error("expected a plain SwitchClient to the free sibling")
	}
	for _, c := range mock.Calls {
		if c.Method == "NewGroupedSession" {
			t.Errorf("must not clone a free sibling: %+v", c)
		}
	}
}

func TestSwitchToWorkspacePositionClonesAttachedSibling(t *testing.T) {
	app, mock := newTestApp(t)
	main := addFallbackSession(t, app, "proj", "main")
	other := addFallbackSession(t, app, "proj", "other")
	mock.DisplayMessageFunc = displayByFormat(map[string]string{"#{session_name}": main})
	mock.Sessions = []tmux.Session{
		{Name: main, Attached: true},
		{Name: other, Attached: true}, // attached elsewhere → must clone
	}

	// Resolve the sibling's 1-based position without assuming ordering.
	pos := 0
	for i, tname := range app.WorkspaceStore.SessionTargetsIn("proj") {
		if tname == other {
			pos = i + 1
		}
	}
	if pos == 0 {
		t.Fatalf("sibling %q not found in workspace targets", other)
	}

	if err := switchToWorkspacePosition(app, pos); err != nil {
		t.Fatalf("switchToWorkspacePosition: %v", err)
	}
	clone := other + "__clone_b"
	if !fallbackMockHasCall(mock.Calls, "NewGroupedSession", other, clone) {
		t.Errorf("prefix+alt+N to an attached sibling must clone %s", other)
	}
	if !fallbackMockHasCall(mock.Calls, "SwitchClient", clone) {
		t.Error("expected switch to the clone")
	}
}

func TestApplySelectFromCloneTargetsClone(t *testing.T) {
	app, mock := newTestApp(t)
	// The client is viewing the clone dev-b; the picker selects a tab in dev
	// (same root). Selection must land on dev-b, not the root dev.
	mock.DisplayMessageFunc = displayByFormat(map[string]string{"#{session_name}": "dev-b"})
	mock.Sessions = []tmux.Session{
		{Name: "dev", Attached: true},
		{Name: "dev-b", Group: "dev", Clone: true, Attached: true},
	}

	err := applyTabPickerResult(app, "dev-b", tabpicker.TabPickerResult{
		Action: "select", Session: "dev", Index: 2, TabID: "ztab_x",
	})
	if err != nil {
		t.Fatalf("apply select: %v", err)
	}

	var onClone, onRoot bool
	for _, c := range mock.Calls {
		if c.Method == "SelectWindow" && len(c.Args) == 2 && c.Args[1] == "2" {
			switch c.Args[0] {
			case "dev-b":
				onClone = true
			case "dev":
				onRoot = true
			}
		}
	}
	if !onClone {
		t.Error("select from a clone must SelectWindow on the clone")
	}
	if onRoot {
		t.Error("must not SelectWindow on the root — that re-collapses the shared view")
	}
	if got := mruWrite(mock.Calls, "dev"); got != "ztab_x" {
		t.Errorf("MRU must key on the root dev, got %q", got)
	}
}

func TestApplySelectCrossSessionClonesAttached(t *testing.T) {
	app, mock := newTestApp(t)
	mock.DisplayMessageFunc = displayByFormat(map[string]string{"#{session_name}": "dev"})
	mock.Sessions = []tmux.Session{
		{Name: "dev", Attached: true},
		{Name: "other", Attached: true}, // attached elsewhere → clone
	}

	err := applyTabPickerResult(app, "dev", tabpicker.TabPickerResult{
		Action: "select", Session: "other", Index: 1, TabID: "ztab_y",
	})
	if err != nil {
		t.Fatalf("apply select: %v", err)
	}
	if !fallbackMockHasCall(mock.Calls, "NewGroupedSession", "other", "other-b") {
		t.Error("expected a clone of the attached cross-session target")
	}
	if !fallbackMockHasCall(mock.Calls, "SelectWindow", "other-b", "1") {
		t.Error("select must target the clone's window")
	}
	for _, c := range mock.Calls {
		if c.Method == "SelectWindow" && c.Args[0] == "other" {
			t.Error("must not SelectWindow on the shared root")
		}
	}
}

func TestHandleDashboardSwitchClonesAttached(t *testing.T) {
	app, mock := newTestApp(t)
	mock.InsideTmux = true
	mock.DisplayMessageFunc = displayByFormat(map[string]string{"#{session_name}": "dev"})
	mock.Sessions = []tmux.Session{
		{Name: "dev", Attached: true},
		{Name: "prod", Attached: true},
	}

	if err := handleDashboardResult(app, "switch", "prod"); err != nil {
		t.Fatalf("handleDashboardResult: %v", err)
	}
	if !fallbackMockHasCall(mock.Calls, "NewGroupedSession", "prod", "prod-b") {
		t.Error("dashboard switch to an attached session must clone")
	}
	if !fallbackMockHasCall(mock.Calls, "SwitchClient", "prod-b") {
		t.Error("expected switch to the clone")
	}
}

func TestHandleDashboardNewSwitchesPlainly(t *testing.T) {
	app, mock := newTestApp(t)
	mock.InsideTmux = true

	if err := handleDashboardResult(app, "new", ""); err != nil {
		t.Fatalf("handleDashboardResult: %v", err)
	}
	for _, c := range mock.Calls {
		if c.Method == "NewGroupedSession" {
			t.Errorf("creating a new session must never clone: %+v", c)
		}
	}
}
