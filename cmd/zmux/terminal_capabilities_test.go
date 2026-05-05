package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/donjor/zmux/internal/tmux"
)

func TestRunTerminalRefreshCurrentClient(t *testing.T) {
	mock := withMockApp(t)
	mock.DisplayMessageResult = "/dev/pts/13\tpi"

	cmd := newTerminalRefreshCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := runTerminalRefresh(cmd, &terminalRefreshFlags{}); err != nil {
		t.Fatalf("runTerminalRefresh failed: %v", err)
	}
	if !strings.Contains(out.String(), "refreshing tmux client /dev/pts/13 -> pi") {
		t.Fatalf("unexpected output: %s", out.String())
	}
	if len(mock.Calls) != 3 || mock.Calls[0].Method != "IsInsideTmux" || mock.Calls[1].Method != "DisplayMessage" || mock.Calls[2].Method != "RefreshClient" {
		t.Fatalf("unexpected calls: %#v", mock.Calls)
	}
	if mock.Calls[2].Args[0] != "/dev/pts/13" || mock.Calls[2].Args[1] != "pi" {
		t.Fatalf("unexpected refresh args: %#v", mock.Calls[2].Args)
	}
}

func TestRunTerminalRefreshExplicitTargetOutsideTmux(t *testing.T) {
	mock := withMockApp(t)
	mock.InsideTmux = false

	cmd := newTerminalRefreshCmd()
	if err := runTerminalRefresh(cmd, &terminalRefreshFlags{targetClient: "/dev/pts/13", session: "pi"}); err != nil {
		t.Fatalf("runTerminalRefresh explicit target failed: %v", err)
	}
	if len(mock.Calls) != 1 || mock.Calls[0].Method != "RefreshClient" || mock.Calls[0].Args[0] != "/dev/pts/13" || mock.Calls[0].Args[1] != "pi" {
		t.Fatalf("unexpected calls: %#v", mock.Calls)
	}
}

func TestRunTerminalCapabilitiesJSONOK(t *testing.T) {
	mock := withMockApp(t)
	mock.DisplayMessageResult = "/dev/pts/13"
	mock.Clients = []tmux.ClientInfo{{
		TTY:          "/dev/pts/13",
		SessionName:  "pi",
		WindowID:     "@50",
		WindowName:   "pi",
		PaneID:       "%139",
		TermName:     "xterm-256color",
		TermFeatures: "bpaste,RGB,title",
		Flags:        "attached,focused,UTF-8",
	}}
	t.Setenv("TERM", "tmux-256color")
	t.Setenv("COLORTERM", "truecolor")
	t.Setenv("TERM_PROGRAM", "ghostty")

	cmd := newTerminalCapabilitiesCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := runTerminalCapabilities(cmd, &terminalCapabilitiesFlags{json: true}); err != nil {
		t.Fatalf("runTerminalCapabilities failed: %v", err)
	}
	var result struct {
		OK         bool   `json:"ok"`
		Status     string `json:"status"`
		CurrentTTY string `json:"currentTTY"`
		Clients    []struct {
			Current  bool   `json:"current"`
			TermName string `json:"termName"`
			RGB      bool   `json:"rgb"`
		} `json:"clients"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if !result.OK || result.Status != "ok" || result.CurrentTTY != "/dev/pts/13" || len(result.Clients) != 1 || !result.Clients[0].Current || !result.Clients[0].RGB || result.Clients[0].TermName != "xterm-256color" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestRunTerminalCapabilitiesTextMissingRGB(t *testing.T) {
	mock := withMockApp(t)
	mock.DisplayMessageResult = "/dev/pts/13"
	mock.Clients = []tmux.ClientInfo{{
		TTY:          "/dev/pts/13",
		TermName:     "xterm-256color",
		TermFeatures: "bpaste,title",
		Flags:        "attached,focused,UTF-8",
	}}

	cmd := newTerminalCapabilitiesCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := runTerminalCapabilities(cmd, &terminalCapabilitiesFlags{}); err != nil {
		t.Fatalf("runTerminalCapabilities failed: %v", err)
	}
	text := out.String()
	for _, want := range []string{"✗ tmux truecolor: rgb_missing_current_client", "term=xterm-256color", "truecolor=missing", "zmux refresh"} {
		if !strings.Contains(text, want) {
			t.Fatalf("output missing %q:\n%s", want, text)
		}
	}
}
