package cli

import (
	"strings"
	"testing"

	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tmux"
)

func TestWaitAllowsFreshAfterTurnGeneration(t *testing.T) {
	root, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.LogicalRows = []tmux.LogicalPaneRow{logicalRow("%4", "test-session", "@3", 2, "ztab_peer", "claude-peer")}
	mock.PaneOptions = map[string]string{
		"%4\x00" + tabs.OptTurnState: tabs.TurnReady,
		"%4\x00" + tabs.OptTurnSeq:   "10",
	}
	out := captureStdout(t, func() {
		root.SetArgs([]string{"wait", "claude-peer", "-s", "test-session", "--for", "turn:ready", "--fresh-after", "9", "--json"})
		if err := root.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})
	if !strings.Contains(out, `"met": true`) || !strings.Contains(out, `"basis": "turnState"`) {
		t.Fatalf("expected met turnState JSON, got:\n%s", out)
	}
}

func TestWaitTurnSetReportsFiredState(t *testing.T) {
	root, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.LogicalRows = []tmux.LogicalPaneRow{logicalRow("%4", "test-session", "@3", 2, "ztab_peer", "claude-peer")}
	mock.PaneOptions = map[string]string{
		"%4\x00" + tabs.OptTurnState: tabs.TurnFailed,
		"%4\x00" + tabs.OptTurnSeq:   "10",
	}
	out := captureStdout(t, func() {
		root.SetArgs([]string{"wait", "claude-peer", "-s", "test-session", "--for", "turn:ready,failed,attention", "--fresh-after", "9", "--json"})
		if err := root.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})
	if !strings.Contains(out, `"met": true`) || !strings.Contains(out, `"state": "failed"`) {
		t.Fatalf("expected met turn set reporting failed, got:\n%s", out)
	}
}

func TestTabInspectJSONIncludesStatusTailAndWarnings(t *testing.T) {
	root, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.LogicalRows = []tmux.LogicalPaneRow{logicalRow("%3", "test-session", "@2", 1, "ztab_build", "build")}
	mock.CapturedPaneContent = "line one\nline two\n"
	mock.PaneOptions = map[string]string{
		"%3\x00" + tabs.OptCmdState:    tabs.CmdFailed,
		"%3\x00" + tabs.OptCmdSeq:      "12",
		"%3\x00" + tabs.OptCmdLastExit: "2",
	}
	out := captureStdout(t, func() {
		root.SetArgs([]string{"tab", "inspect", "build", "-s", "test-session", "--json"})
		if err := root.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})
	if !strings.Contains(out, `"cmdState": "failed"`) || !strings.Contains(out, `"outputTail": "line one\nline two\n"`) || !strings.Contains(out, "last command exited with 2") {
		t.Fatalf("inspect JSON missing expected fields:\n%s", out)
	}
}
