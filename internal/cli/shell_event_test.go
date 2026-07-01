package cli

import (
	"testing"

	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tmux"
)

func TestShellEventStartRecordsCommandLifecycleAndRunID(t *testing.T) {
	root, mock := withMockApp(t)
	mock.DisplayMessageResult = "%3	dev:2\n"
	mock.PaneOptions = map[string]string{
		"%3\x00" + tabs.OptTabID:     "ztab_shell",
		"%3\x00" + tabs.OptNextRunID: "abc123",
	}

	root.SetArgs([]string{"shell-event", "start", "--pane", "%3", "--command", "sleep 1"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	seen := map[string]string{}
	for _, c := range mock.Calls {
		if c.Method == "ApplyOptions" && c.Args[0] == "-p" && c.Args[1] == "%3" && c.Args[4] == "unset=false" {
			seen[c.Args[2]] = c.Args[3]
		}
	}
	if seen[tabs.OptCmdState] != tabs.CmdRunning {
		t.Fatalf("cmd state = %q, want running (seen=%v)", seen[tabs.OptCmdState], seen)
	}
	if seen[tabs.OptCmdRunID] != "abc123" {
		t.Fatalf("cmd run id = %q, want abc123 (seen=%v)", seen[tabs.OptCmdRunID], seen)
	}
	if seen["@zmux_state"] != "running" {
		t.Fatalf("glyph state = %q, want running (seen=%v)", seen["@zmux_state"], seen)
	}
}

func TestShellEventEndPublishesRunResultAndFailedGlyph(t *testing.T) {
	root, mock := withMockApp(t)
	mock.DisplayMessageResult = "%3	dev:2\n"
	mock.PaneOptions = map[string]string{
		"%3\x00" + tabs.OptTabID:    "ztab_shell",
		"%3\x00" + tabs.OptCmdRunID: "abc123",
	}

	root.SetArgs([]string{"shell-event", "end", "--pane", "%3", "--exit", "7"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	seen := map[string]string{}
	for _, c := range mock.Calls {
		if c.Method == "ApplyOptions" && c.Args[0] == "-p" && c.Args[1] == "%3" && c.Args[4] == "unset=false" {
			seen[c.Args[2]] = c.Args[3]
		}
	}
	if seen[tabs.OptCmdState] != tabs.CmdFailed {
		t.Fatalf("cmd state = %q, want failed (seen=%v)", seen[tabs.OptCmdState], seen)
	}
	if seen[tabs.OptRunResult] != "abc123:7" {
		t.Fatalf("run result = %q, want abc123:7 (seen=%v)", seen[tabs.OptRunResult], seen)
	}
	if seen["@zmux_state"] != "failed" || seen["@zmux_state_msg"] != "exit 7" {
		t.Fatalf("glyph/msg = %q/%q, want failed/exit 7 (seen=%v)", seen["@zmux_state"], seen["@zmux_state_msg"], seen)
	}
}

func TestShellEventNoopsForUnmanagedPane(t *testing.T) {
	root, mock := withMockApp(t)
	mock.DisplayMessageFunc = displayByFormat(map[string]string{
		"#{pane_id}":   "%3	dev:2\n",
		"#{window_id}": "@2\n",
	})
	mock.Panes = map[string][]tmux.Pane{}

	root.SetArgs([]string{"shell-event", "start", "--pane", "%3", "--command", "echo nope"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	for _, c := range mock.Calls {
		if c.Method == "ApplyOptions" && c.Args[2] == tabs.OptCmdState {
			t.Fatalf("unmanaged pane must not get command state: %v", c.Args)
		}
	}
}
