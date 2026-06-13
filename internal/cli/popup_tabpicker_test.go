package cli

import (
	"strings"
	"testing"

	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui/tabpicker"
)

// mruWrite returns the value written to the session's tab MRU option, or "".
func mruWrite(calls []tmux.MockCall, session string) string {
	for _, c := range calls {
		if c.Method == "SetSessionOption" && len(c.Args) == 3 &&
			c.Args[0] == session && c.Args[1] == tabs.OptMRU {
			return c.Args[2]
		}
	}
	return ""
}

func TestApplySelectTouchesMRU(t *testing.T) {
	a, mock := newTestApp(t)
	mock.DisplayMessageFunc = displayByFormat(map[string]string{
		tabs.OptMRU: "", // empty MRU before the touch
	})

	err := applyTabPickerResult(a, "dev", tabpicker.TabPickerResult{
		Action: "select", Session: "dev", Index: 2, TabID: "ztab_bud",
	})
	if err != nil {
		t.Fatalf("apply select: %v", err)
	}

	var selected bool
	for _, c := range mock.Calls {
		if c.Method == "SelectWindow" && c.Args[0] == "dev" && c.Args[1] == "2" {
			selected = true
		}
	}
	if !selected {
		t.Error("select must SelectWindow dev:2")
	}
	if got := mruWrite(mock.Calls, "dev"); got != "ztab_bud" {
		t.Errorf("MRU write = %q, want ztab_bud", got)
	}
}

func TestApplySelectRawWindowSkipsMRU(t *testing.T) {
	a, mock := newTestApp(t)

	err := applyTabPickerResult(a, "dev", tabpicker.TabPickerResult{
		Action: "select", Session: "dev", Index: 1,
	})
	if err != nil {
		t.Fatalf("apply select: %v", err)
	}
	if got := mruWrite(mock.Calls, "dev"); got != "" {
		t.Errorf("raw window must not touch MRU, wrote %q", got)
	}
}

func TestApplySelectPaneFocusesPane(t *testing.T) {
	a, mock := newTestApp(t)
	mock.DisplayMessageFunc = displayByFormat(map[string]string{
		tabs.OptMRU: "ztab_bud", // rider touch moves ahead of the host
	})

	err := applyTabPickerResult(a, "dev", tabpicker.TabPickerResult{
		Action: "select-pane", Session: "dev", Index: 2, Pane: "%3", TabID: "ztab_tst",
	})
	if err != nil {
		t.Fatalf("apply select-pane: %v", err)
	}

	var window, pane bool
	for _, c := range mock.Calls {
		if c.Method == "SelectWindow" && c.Args[0] == "dev" && c.Args[1] == "2" {
			window = true
		}
		if c.Method == "SelectPane" && c.Args[0] == "%3" {
			pane = true
		}
	}
	if !window || !pane {
		t.Errorf("select-pane must focus window then pane: window=%v pane=%v", window, pane)
	}
	if got := mruWrite(mock.Calls, "dev"); got != "ztab_tst ztab_bud" {
		t.Errorf("MRU write = %q, want ztab_tst first", got)
	}
}

func TestApplyClosePaneKillsPaneOnly(t *testing.T) {
	a, mock := newTestApp(t)

	err := applyTabPickerResult(a, "dev", tabpicker.TabPickerResult{
		Action: "close-pane", Session: "dev", Pane: "%3",
	})
	if err != nil {
		t.Fatalf("apply close-pane: %v", err)
	}

	var killedPane bool
	for _, c := range mock.Calls {
		if c.Method == "KillPane" && c.Args[0] == "%3" {
			killedPane = true
		}
		if strings.HasPrefix(c.Method, "KillWindow") {
			t.Errorf("close-pane must never kill a window: %+v", c)
		}
	}
	if !killedPane {
		t.Error("close-pane must KillPane %3")
	}
}

func TestApplyCloseRefusesLastWindow(t *testing.T) {
	a, mock := newTestApp(t)
	mock.Windows["dev"] = []tmux.Window{{Index: 1, Name: "main"}}

	err := applyTabPickerResult(a, "dev", tabpicker.TabPickerResult{
		Action: "close", Session: "dev", Index: 1,
	})
	if err == nil || !strings.Contains(err.Error(), "last tab") {
		t.Fatalf("expected last-tab refusal, got %v", err)
	}
	for _, c := range mock.Calls {
		if c.Method == "KillWindow" {
			t.Fatalf("must not kill the last window: %+v", c)
		}
	}
}

// show from the picker mirrors `zmux tab show`: clone-block on the origin,
// move the window home, record the selection in the origin's MRU.
func TestApplyShowReturnsDockedTab(t *testing.T) {
	a, mock := newTestApp(t)
	mock.Sessions = []tmux.Session{{Name: "dev"}}
	docked := logicalRow("%4", tabs.DockSession, "@9", 0, "ztab_log", "logs")
	docked.Hidden = "dev"
	mock.LogicalRows = []tmux.LogicalPaneRow{docked}
	mock.DisplayMessageFunc = displayByFormat(map[string]string{
		"session_group": "\t1\t1\n", // ungrouped — not clone-blocked
		tabs.OptMRU:     "",
	})

	err := applyTabPickerResult(a, "dev", tabpicker.TabPickerResult{
		Action: "show", Session: "dev", TabID: "ztab_log",
	})
	if err != nil {
		t.Fatalf("apply show: %v", err)
	}

	var moved bool
	for _, c := range mock.Calls {
		if c.Method == "MoveWindow" && c.Args[0] == "@9" && c.Args[1] == "dev:" {
			moved = true
		}
	}
	if !moved {
		t.Error("show must move the docked window back to its origin")
	}
	if got := mruWrite(mock.Calls, "dev"); got != "ztab_log" {
		t.Errorf("MRU write = %q, want ztab_log", got)
	}
}

func TestApplyShowMissingTabErrors(t *testing.T) {
	a, _ := newTestApp(t)

	err := applyTabPickerResult(a, "dev", tabpicker.TabPickerResult{
		Action: "show", Session: "dev", TabID: "ztab_gone",
	})
	if err == nil || !strings.Contains(err.Error(), "no longer exists") {
		t.Errorf("show on a dead tab must error, got %v", err)
	}
}
