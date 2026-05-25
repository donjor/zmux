package terminal

import (
	"context"
	"testing"

	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/wm"
)

type fakeAdapter struct {
	windows []wm.Window
	err     error
}

func (f fakeAdapter) Windows(context.Context) ([]wm.Window, error) { return f.windows, f.err }

type fakeProcess struct {
	ancestor bool
	err      error
}

func (f fakeProcess) IsAncestor(_, _ int) (bool, error) { return f.ancestor, f.err }

func TestResolveCurrentOK(t *testing.T) {
	mock := currentMock()
	resolver := Resolver{Runner: mock, Adapter: fakeAdapter{windows: []wm.Window{matchingWindow(true)}}, Process: fakeProcess{ancestor: true}, CurrentPaneID: "%139"}
	result, err := resolver.Resolve(context.Background())
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if !result.OK || result.Status != StatusOK {
		t.Fatalf("expected ok, got %#v", result)
	}
	if result.Target == nil || result.Target.Geometry != "12,57 2536x1371" || result.Target.WindowAddress != "0x1" {
		t.Fatalf("unexpected target: %#v", result.Target)
	}
	if result.Tmux == nil || result.Tmux.ClientSession != "pi" || result.Tmux.SessionID != "$28" || result.Tmux.PaneID != "%139" {
		t.Fatalf("unexpected tmux context: %#v", result.Tmux)
	}
}

func TestResolveCurrentNotInTmux(t *testing.T) {
	mock := currentMock()
	mock.InsideTmux = false
	resolver := Resolver{Runner: mock, Adapter: fakeAdapter{}, CurrentPaneID: "%139"}
	result, err := resolver.Resolve(context.Background())
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if result.OK || result.Status != StatusNotInTmux {
		t.Fatalf("expected not_in_tmux, got %#v", result)
	}
}

func TestResolveCurrentUnsupportedTmuxFormat(t *testing.T) {
	mock := currentMock()
	mock.DisplayMessageResult = "#{client_tty}\tpi\t$28\t@50\t1\tparley\t%139"
	resolver := Resolver{Runner: mock, Adapter: fakeAdapter{}, CurrentPaneID: "%139"}
	result, err := resolver.Resolve(context.Background())
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if result.OK || result.Status != StatusUnsupported {
		t.Fatalf("expected unsupported, got %#v", result)
	}
}

func TestResolveCurrentNotFound(t *testing.T) {
	mock := currentMock()
	resolver := Resolver{Runner: mock, Adapter: fakeAdapter{windows: []wm.Window{{Title: "Ghostty", Visible: true}}}, Process: fakeProcess{ancestor: true}, CurrentPaneID: "%139"}
	result, err := resolver.Resolve(context.Background())
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if result.OK || result.Status != StatusNotFound {
		t.Fatalf("expected not_found, got %#v", result)
	}
	if result.Target != nil {
		t.Fatalf("not_found must not include target geometry: %#v", result.Target)
	}
}

func TestResolveCurrentRejectsProcessMismatch(t *testing.T) {
	mock := currentMock()
	resolver := Resolver{Runner: mock, Adapter: fakeAdapter{windows: []wm.Window{matchingWindow(true)}}, Process: fakeProcess{ancestor: false}, CurrentPaneID: "%139"}
	result, err := resolver.Resolve(context.Background())
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if result.OK || result.Status != StatusNotFound {
		t.Fatalf("expected process mismatch to be not_found, got %#v", result)
	}
}

func TestResolveCurrentHidden(t *testing.T) {
	mock := currentMock()
	resolver := Resolver{Runner: mock, Adapter: fakeAdapter{windows: []wm.Window{matchingWindow(false)}}, Process: fakeProcess{ancestor: true}, CurrentPaneID: "%139"}
	result, err := resolver.Resolve(context.Background())
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if result.OK || result.Status != StatusHidden {
		t.Fatalf("expected hidden, got %#v", result)
	}
	if result.Target == nil || result.Target.Geometry != "" || result.Target.Visible {
		t.Fatalf("hidden target must not expose geometry: %#v", result.Target)
	}
}

func TestResolveCurrentAmbiguousVisible(t *testing.T) {
	mock := currentMock()
	w1 := matchingWindow(true)
	w2 := matchingWindow(true)
	w2.Address = "0x2"
	resolver := Resolver{Runner: mock, Adapter: fakeAdapter{windows: []wm.Window{w1, w2}}, Process: fakeProcess{ancestor: true}, CurrentPaneID: "%139"}
	result, err := resolver.Resolve(context.Background())
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if result.OK || result.Status != StatusAmbiguous {
		t.Fatalf("expected ambiguous, got %#v", result)
	}
}

func TestResolveCurrentRawGroupedSessionIDs(t *testing.T) {
	mock := currentMock()
	mock.DisplayMessageResult = "/dev/pts/26\tbridge-b\t$21\t@36\t3\tbash\t%36"
	mock.Clients = []tmux.ClientInfo{{TTY: "/dev/pts/26", SessionName: "bridge-b", SessionID: "$21", SessionGroup: "bridge", WindowID: "@36", WindowIndex: 3, WindowName: "bash", PaneID: "%36", PID: 3055165}}
	windows := []wm.Window{
		{WM: "hyprland", Address: "0x-root", PID: 100, Title: "zmux:v1;tty=/dev/pts/6;sid=$20;wid=@33;pane=%33 bridge:1:claude", Visible: true},
		{WM: "hyprland", Address: "0x-clone", PID: 200, Title: "zmux:v1;tty=/dev/pts/26;sid=$21;wid=@36;pane=%36 bridge-b:3:bash", Visible: true, Geometry: "1,2 3x4"},
	}
	resolver := Resolver{Runner: mock, Adapter: fakeAdapter{windows: windows}, Process: fakeProcess{ancestor: true}, CurrentPaneID: "%36"}
	result, err := resolver.Resolve(context.Background())
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if !result.OK || result.Target.WindowAddress != "0x-clone" {
		t.Fatalf("expected raw grouped session match, got %#v", result)
	}
}

func currentMock() *tmux.MockRunner {
	mock := tmux.NewMockRunner()
	mock.InsideTmux = true
	mock.DisplayMessageResult = "/dev/pts/13\tpi\t$28\t@50\t1\tparley\t%139"
	mock.Clients = []tmux.ClientInfo{{TTY: "/dev/pts/13", SessionName: "pi", SessionID: "$28", WindowID: "@50", WindowIndex: 1, WindowName: "parley", PaneID: "%139", PID: 2028292}}
	return mock
}

func matchingWindow(visible bool) wm.Window {
	return wm.Window{
		WM:        "hyprland",
		Address:   "0x1",
		Class:     "com.mitchellh.ghostty",
		Title:     "zmux:v1;tty=/dev/pts/13;sid=$28;wid=@50;pane=%139 pi:1:parley",
		PID:       11474,
		Workspace: "2",
		Geometry:  "12,57 2536x1371",
		Visible:   visible,
	}
}
