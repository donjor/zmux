package cli

import (
	"strings"
	"testing"

	"github.com/donjor/zmux/internal/tmux"
)

// displayByFormat routes mock DisplayMessage responses by format substring —
// tab state resolution, state reads, and visibility checks use distinct
// formats in one command run.
func displayByFormat(routes map[string]string) func(target, format string) (string, error) {
	return func(target, format string) (string, error) {
		if out, ok := routes[format]; ok { // exact format first — substrings overlap
			return out, nil
		}
		for key, out := range routes {
			if strings.Contains(format, key) {
				return out, nil
			}
		}
		return "", nil
	}
}

func TestTabStateSetWritesPaneAndMirror(t *testing.T) {
	root, mock := withMockApp(t)
	mock.DisplayMessageResult = "%3\tdev:2\n"

	root.SetArgs([]string{"tab", "state", "running", "%3", "--source", "run"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	var pane, win, refresh int
	for _, c := range mock.Calls {
		switch {
		case c.Method == "ApplyOptions" && c.Args[0] == "-p":
			pane++
			if c.Args[1] != "%3" {
				t.Fatalf("canonical write must target the pane, got %v", c.Args)
			}
		case c.Method == "ApplyOptions" && c.Args[0] == "-w":
			win++
			if c.Args[1] != "dev:2" {
				t.Fatalf("mirror write must target the window, got %v", c.Args)
			}
		case c.Method == "RefreshStatus":
			refresh++
		}
	}
	if pane == 0 || win == 0 || refresh != 1 {
		t.Fatalf("want pane+mirror writes and one refresh, got pane=%d win=%d refresh=%d", pane, win, refresh)
	}
}

func TestTabStateBareNameResolvesLabelAware(t *testing.T) {
	root, mock := withMockApp(t)
	mock.Windows = map[string][]tmux.Window{
		"test-session": {{Index: 3, Name: "node", Label: "buddy"}}, // auto-renamed, label survives
	}
	mock.DisplayMessageFunc = displayByFormat(map[string]string{
		"#{session_name}": "test-session",
		"#{pane_id}\t#{session_name}:#{window_index}": "%5\ttest-session:3\n",
	})

	root.SetArgs([]string{"tab", "state", "attention", "buddy"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	// resolution must have queried the label-resolved window target
	found := false
	for _, c := range mock.Calls {
		if c.Method == "DisplayMessage" && c.Args[0] == "test-session:3" {
			found = true
		}
	}
	if !found {
		t.Fatalf("bare name must resolve via label to test-session:3; calls: %v", mock.Calls)
	}
}

func TestTabStateClearIfOnlyMatching(t *testing.T) {
	root, mock := withMockApp(t)
	mock.DisplayMessageFunc = displayByFormat(map[string]string{
		"#{pane_id}":    "%2\tdev:0\n",
		"#{@zmux_state": "done\n", // current state — focus rule must NOT clear done
	})

	root.SetArgs([]string{"tab", "state", "clear", "--target", "%2", "--if", "attention"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	for _, c := range mock.Calls {
		if c.Method == "ApplyOptions" {
			t.Fatalf("done must not be cleared by --if attention, got %v", c)
		}
	}
}

func TestTabStateDoneByVisibilityHiddenStoresAttention(t *testing.T) {
	root, mock := withMockApp(t)
	mock.DisplayMessageFunc = displayByFormat(map[string]string{
		"#{pane_id}":       "%1\tdev:4\n",
		"#{window_active}": "0\t1\n", // background window, attached session
	})

	root.SetArgs([]string{"tab", "state", "done", "--target", "%1", "--by-visibility", "--source", "claude-stop"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	for _, c := range mock.Calls {
		if c.Method == "ApplyOptions" && c.Args[0] == "-p" && c.Args[2] == "@zmux_state" && c.Args[4] == "unset=false" {
			if c.Args[3] != "attention" {
				t.Fatalf("hidden pane must store attention, got %v", c.Args)
			}
			return
		}
	}
	t.Fatal("no state write recorded")
}

func TestTabStateQuietFailsOpen(t *testing.T) {
	root, mock := withMockApp(t)
	mock.InsideTmux = false
	t.Setenv("TMUX_PANE", "")

	root.SetArgs([]string{"tab", "state", "done", "--quiet"})
	if err := root.Execute(); err != nil {
		t.Fatalf("--quiet must never fail, got %v", err)
	}
	for _, c := range mock.Calls {
		if c.Method == "ApplyOptions" {
			t.Fatalf("no writes expected without a target, got %v", c)
		}
	}
}

func TestTabStateNoTargetErrorsWithoutQuiet(t *testing.T) {
	root, mock := withMockApp(t)
	mock.InsideTmux = false
	t.Setenv("TMUX_PANE", "")

	root.SetArgs([]string{"tab", "state", "done"})
	root.SilenceUsage = true
	root.SilenceErrors = true
	if err := root.Execute(); err == nil {
		t.Fatal("missing target without --quiet must error")
	}
}

func TestTabStateFlagValidation(t *testing.T) {
	for _, args := range [][]string{
		{"tab", "state", "running", "%1", "--if", "attention"}, // --if needs clear
		{"tab", "state", "running", "%1", "--by-visibility"},   // --by-visibility needs done
		{"tab", "state", "clear", "%1", "--by-visibility"},     // ditto
		{"tab", "state", "bogus", "%1"},                        // unknown state
		{"tab", "state", "clear", "%1", "--if", "notastate"},   // unknown --if state
	} {
		root, mock := withMockApp(t)
		mock.DisplayMessageResult = "%1\tdev:0\n"
		root.SetArgs(args)
		root.SilenceUsage = true
		root.SilenceErrors = true
		if err := root.Execute(); err == nil {
			t.Fatalf("args %v must error", args)
		}
	}
}

// state-exit is the run epilogue: exit code → done/failed, resolved from the
// calling pane, always silent.
func TestTabStateExitMapsCodeToState(t *testing.T) {
	cases := []struct {
		code      string
		wantState string
		wantMsg   string
	}{
		{"0", "done", ""},
		{"2", "failed", "exit 2"},
	}
	for _, tc := range cases {
		root, mock := withMockApp(t)
		t.Setenv("TMUX_PANE", "%7")
		mock.DisplayMessageResult = "%7\tdev:3\n"

		root.SetArgs([]string{"tab", "state-exit", tc.code})
		if err := root.Execute(); err != nil {
			t.Fatalf("state-exit %s failed: %v", tc.code, err)
		}

		var gotState, gotMsg string
		for _, c := range mock.Calls {
			if c.Method != "ApplyOptions" || c.Args[0] != "-p" || c.Args[4] != "unset=false" {
				continue
			}
			if c.Args[2] == "@zmux_state" {
				gotState = c.Args[3]
			}
			if c.Args[2] == "@zmux_state_msg" {
				gotMsg = c.Args[3]
			}
		}
		if gotState != tc.wantState || gotMsg != tc.wantMsg {
			t.Fatalf("code %s: got state=%q msg=%q, want %q/%q",
				tc.code, gotState, gotMsg, tc.wantState, tc.wantMsg)
		}
	}
}

func TestTabStateExitGarbageStaysSilent(t *testing.T) {
	root, mock := withMockApp(t)
	root.SetArgs([]string{"tab", "state-exit", "notanumber"})
	if err := root.Execute(); err != nil {
		t.Fatalf("garbage exit code must stay silent, got: %v", err)
	}
	for _, c := range mock.Calls {
		if c.Method == "ApplyOptions" {
			t.Fatalf("garbage exit code must not write state: %v", c)
		}
	}
}
