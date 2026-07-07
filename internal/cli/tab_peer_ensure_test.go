package cli

import (
	"strings"
	"testing"

	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tmux"
)

func TestTabPeerEnsureCreatesMissingPeerAndSendsCommand(t *testing.T) {
	root, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.NewWindowPaneID = "%9"
	mock.DisplayMessageFunc = func(target, format string) (string, error) {
		if target == "%9" {
			return "%9\ttest-session:2", nil
		}
		return "test-session", nil
	}
	out := captureStdout(t, func() {
		root.SetArgs([]string{"tab", "peer", "ensure", "claude-peer", "-s", "test-session", "--command", "claude", "--role", "claude", "--json"})
		if err := root.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})
	if !strings.Contains(out, `"created": true`) || !strings.Contains(out, `"commandSent": true`) {
		t.Fatalf("expected created commandSent JSON:\n%s", out)
	}
	var newWindow, sendCommand, turnRunning bool
	for _, c := range mock.Calls {
		if c.Method == "NewWindow" && c.Args[0] == "test-session" && c.Args[1] == "claude-peer" && c.Args[3] == "detached=true" {
			newWindow = true
		}
		if c.Method == "SendKeys" && c.Args[0] == "%9" && len(c.Args) >= 3 && c.Args[1] == "-l" && c.Args[2] == "claude" {
			sendCommand = true
		}
		if c.Method == "ApplyOptions" && c.Args[1] == "%9" && c.Args[2] == tabs.OptTurnState && c.Args[3] == tabs.TurnRunning {
			turnRunning = true
		}
	}
	if !newWindow || !sendCommand || !turnRunning {
		t.Fatalf("missing expected create/send/running calls: new=%v send=%v running=%v calls=%+v", newWindow, sendCommand, turnRunning, mock.Calls)
	}
}

func TestTabPeerEnsureReusesExistingWithoutSendingCommand(t *testing.T) {
	root, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.LogicalRows = []tmux.LogicalPaneRow{logicalRow("%4", "test-session", "@3", 2, "ztab_peer", "claude-peer")}
	mock.PaneOptions = map[string]string{"%4\x00" + tabs.OptScope: tabs.ScopePeer}
	out := captureStdout(t, func() {
		root.SetArgs([]string{"tab", "peer", "ensure", "claude-peer", "-s", "test-session", "--command", "claude", "--json"})
		if err := root.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})
	if !strings.Contains(out, `"reused": true`) || !strings.Contains(out, "launch command not sent") {
		t.Fatalf("expected reuse warning JSON:\n%s", out)
	}
	for _, c := range mock.Calls {
		if c.Method == "SendKeys" && len(c.Args) >= 3 && c.Args[1] == "-l" && c.Args[2] == "claude" {
			t.Fatalf("ensure must not send launch command into an existing peer without --restart: %+v", mock.Calls)
		}
	}
}

func TestTypeMarkPeerRunningWritesLifecycle(t *testing.T) {
	root, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.LogicalRows = []tmux.LogicalPaneRow{logicalRow("%4", "test-session", "@3", 2, "ztab_peer", "claude-peer")}
	mock.PaneOptions = map[string]string{
		"%4\x00" + tabs.OptScope:     tabs.ScopePeer,
		"%4\x00" + tabs.OptTurnState: tabs.TurnReady,
		"%4\x00" + tabs.OptTurnSeq:   "5",
	}
	mock.DisplayMessageFunc = func(target, format string) (string, error) {
		if strings.Contains(format, "pane_id") {
			return "%4\ttest-session:2", nil
		}
		return "test-session", nil
	}
	out := captureStdout(t, func() {
		root.SetArgs([]string{"type", "claude-peer", "hello", "-s", "test-session", "--mark-peer-running", "--json"})
		if err := root.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})
	if !strings.Contains(out, `"typed": true`) {
		t.Fatalf("expected typed JSON:\n%s", out)
	}
	var sentText, turnRunning bool
	sentIndex, runningIndex := -1, -1
	for i, c := range mock.Calls {
		if c.Method == "SendKeys" && len(c.Args) >= 3 && c.Args[0] == "%4" && c.Args[1] == "-l" && c.Args[2] == "hello" {
			sentText = true
			sentIndex = i
		}
		if c.Method == "ApplyOptions" && c.Args[1] == "%4" && c.Args[2] == tabs.OptTurnState && c.Args[3] == tabs.TurnRunning {
			turnRunning = true
			if runningIndex < 0 {
				runningIndex = i
			}
		}
	}
	if !sentText || !turnRunning || runningIndex > sentIndex {
		t.Fatalf("expected running lifecycle before text send: sent=%v running=%v runningIndex=%d sentIndex=%d calls=%+v", sentText, turnRunning, runningIndex, sentIndex, mock.Calls)
	}
}
