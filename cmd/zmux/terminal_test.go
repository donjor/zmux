package main

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/donjor/zmux/internal/procfs"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/wm"
)

type fakeTerminalAdapter struct{ windows []wm.Window }

func (f fakeTerminalAdapter) Windows(context.Context) ([]wm.Window, error) { return f.windows, nil }

type fakeTerminalProcess struct{}

func (fakeTerminalProcess) IsAncestor(_, _ int) (bool, error) { return true, nil }

func TestRunTerminalCurrentJSONOK(t *testing.T) {
	mock := withMockApp(t)
	mock.DisplayMessageResult = "/dev/pts/13\tpi\t$28\t@50\t1\tparley\t%139"
	mock.Clients = []tmux.ClientInfo{{TTY: "/dev/pts/13", SessionName: "pi", SessionID: "$28", WindowID: "@50", WindowIndex: 1, WindowName: "parley", PaneID: "%139", PID: 2028292}}
	t.Setenv("TMUX_PANE", "%139")
	withTerminalAdapter(t, fakeTerminalAdapter{windows: []wm.Window{{WM: "hyprland", Address: "0x1", Class: "com.mitchellh.ghostty", Title: "zmux:v1;tty=/dev/pts/13;sid=$28;wid=@50;pane=%139 pi:1:parley", PID: 11474, Workspace: "2", Geometry: "12,57 2536x1371", Visible: true}}})
	withTerminalProcess(t, fakeTerminalProcess{})

	cmd := newTerminalCurrentCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := runTerminalCurrent(cmd, &terminalCurrentFlags{json: true}); err != nil {
		t.Fatalf("runTerminalCurrent failed: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if result["ok"] != true || result["status"] != "ok" || result["schemaVersion"] != "zmux-terminal-current/v1" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestRunTerminalCurrentJSONNotInTmux(t *testing.T) {
	mock := withMockApp(t)
	mock.InsideTmux = false
	cmd := newTerminalCurrentCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := runTerminalCurrent(cmd, &terminalCurrentFlags{json: true}); err != nil {
		t.Fatalf("runTerminalCurrent failed: %v", err)
	}
	var result struct {
		OK     bool   `json:"ok"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if result.OK || result.Status != "not_in_tmux" {
		t.Fatalf("expected not_in_tmux, got %#v", result)
	}
}

func withTerminalAdapter(t *testing.T, adapter wm.Adapter) {
	t.Helper()
	orig := newTerminalAdapter
	newTerminalAdapter = func() wm.Adapter { return adapter }
	t.Cleanup(func() { newTerminalAdapter = orig })
}

func withTerminalProcess(t *testing.T, process procfs.Inspector) {
	t.Helper()
	orig := newTerminalProcess
	newTerminalProcess = func() procfs.Inspector { return process }
	t.Cleanup(func() { newTerminalProcess = orig })
}
