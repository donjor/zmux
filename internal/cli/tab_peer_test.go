package cli

import (
	"strings"
	"testing"

	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tmux"
)

func TestTabPeerStartMissingTabErrors(t *testing.T) {
	root, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}

	root.SetArgs([]string{"tab", "peer", "start", "missing", "-s", "test-session", "--role", "claude"})
	root.SilenceUsage = true
	root.SilenceErrors = true
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "no tab") {
		t.Fatalf("expected missing-tab error, got %v", err)
	}
}

func TestTabPeerStartWritesMetadataAndRunningGlyph(t *testing.T) {
	root, mock := withMockApp(t)
	mock.DisplayMessageResult = "%3\tdev:2\n"

	root.SetArgs([]string{"tab", "peer", "start", "%3", "--role", "claude", "--host-tab", "ztab_host", "--host-pane", "%9", "--topic", "plan review"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	want := map[string]string{
		tabs.OptScope:        tabs.ScopePeer,
		tabs.OptOrigin:       tabs.OriginAgent,
		tabs.OptPeerRole:     "claude",
		tabs.OptPeerHostTab:  "ztab_host",
		tabs.OptPeerHostPane: "%9",
		tabs.OptPeerTopic:    "plan review",
		tabs.OptTurnState:    tabs.TurnRunning,
		"@zmux_state":        "running",
	}
	seen := map[string]string{}
	for _, c := range mock.Calls {
		if c.Method == "ApplyOptions" && c.Args[0] == "-p" && c.Args[1] == "%3" && c.Args[4] == "unset=false" {
			seen[c.Args[2]] = c.Args[3]
		}
		if c.Method == "ApplyOptions" && c.Args[2] == tabs.OptKeep && c.Args[4] != "unset=true" {
			t.Fatal("peer start must only clear @zmux_keep, never set it")
		}
	}
	for k, v := range want {
		if seen[k] != v {
			t.Fatalf("%s = %q, want %q (seen=%v)", k, seen[k], v, seen)
		}
	}
}

func TestTabPeerWaitingMarksDone(t *testing.T) {
	root, mock := withMockApp(t)
	mock.DisplayMessageResult = "%3\tdev:2\n"
	mock.PaneOptions = map[string]string{"%3\x00" + tabs.OptScope: tabs.ScopePeer}

	root.SetArgs([]string{"tab", "peer", "waiting", "%3", "--source", "claude-stop"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var turn, glyph, source string
	for _, c := range mock.Calls {
		if c.Method == "ApplyOptions" && c.Args[0] == "-p" && c.Args[1] == "%3" && c.Args[4] == "unset=false" {
			switch c.Args[2] {
			case tabs.OptTurnState:
				turn = c.Args[3]
			case "@zmux_state":
				glyph = c.Args[3]
			case "@zmux_state_source":
				source = c.Args[3]
			}
		}
	}
	if turn != tabs.TurnWaiting || glyph != "done" || source != "claude-stop" {
		t.Fatalf("turn/glyph/source = %q/%q/%q, want waiting/done/claude-stop", turn, glyph, source)
	}
}
