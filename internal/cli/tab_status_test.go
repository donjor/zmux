package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tmux"
)

func TestTabStatusJSONReportsLifecycle(t *testing.T) {
	root, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.LogicalRows = []tmux.LogicalPaneRow{
		logicalRow("%3", "test-session", "@2", 1, "ztab_build", "build"),
	}
	mock.PaneOptions = map[string]string{
		"%3\x00@zmux_state":            "done",
		"%3\x00" + tabs.OptCmdState:    tabs.CmdDone,
		"%3\x00" + tabs.OptCmdSeq:      "42",
		"%3\x00" + tabs.OptCmdLastExit: "0",
		"%3\x00" + tabs.OptCmdText:     "make build",
		"%3\x00" + tabs.OptScope:       tabs.ScopeTask,
	}

	out := captureStdout(t, func() {
		root.SetArgs([]string{"tab", "status", "build", "-s", "test-session", "--json"})
		if err := root.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})
	var got tabStatusOutput
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	if got.Tab != "build" || got.PaneID != "%3" || got.State != "done" || got.ResolvedState != "done" || got.CmdState != tabs.CmdDone || got.CmdSeq != "42" || got.LastExit != "0" || got.Command != "make build" {
		t.Fatalf("unexpected status: %+v", got)
	}
}

func TestTabStatusMissingTabErrors(t *testing.T) {
	root, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}

	root.SetArgs([]string{"tab", "status", "missing", "-s", "test-session", "--json"})
	root.SilenceUsage = true
	root.SilenceErrors = true
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "no tab") {
		t.Fatalf("expected missing-tab error, got %v", err)
	}
}

func TestTabStatusTextReportsPeerReady(t *testing.T) {
	root, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.LogicalRows = []tmux.LogicalPaneRow{
		logicalRow("%4", "test-session", "@3", 2, "ztab_peer", "claude-peer"),
	}
	mock.PaneOptions = map[string]string{
		"%4\x00@zmux_state":          "ready",
		"%4\x00" + tabs.OptTurnState: tabs.TurnReady,
		"%4\x00" + tabs.OptTurnAt:    "1782860400",
		"%4\x00" + tabs.OptPeerRole:  "claude",
	}

	out := captureStdout(t, func() {
		root.SetArgs([]string{"tab", "status", "claude-peer", "-s", "test-session"})
		if err := root.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})
	for _, want := range []string{"tab: claude-peer", "state: ready", "turn-state: ready (at 1782860400)"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in output:\n%s", want, out)
		}
	}
}
