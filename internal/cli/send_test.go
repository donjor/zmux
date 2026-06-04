package cli

import (
	"testing"

	"github.com/donjor/zmux/internal/tmux"
)

func TestSendKeysToWindow(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}

	rootCmd.SetArgs([]string{"send", "server", "C-c", "-s", "test-session"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("send command failed: %v", err)
	}

	found := false
	for _, c := range mock.Calls {
		if c.Method == "SendKeys" {
			found = true
			// Should target "test-session:server"
			if len(c.Args) > 0 && c.Args[0] != "test-session:server" {
				t.Errorf("expected target 'test-session:server', got %q", c.Args[0])
			}
		}
	}
	if !found {
		t.Error("expected SendKeys call")
	}
}

// When a long-running process has auto-renamed the window away from its zmux
// name, send must still find it via @zmux_label and target it by index —
// session:name no longer resolves.
func TestSendResolvesAutoRenamedWindowByLabel(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	mock.Windows["test-session"] = []tmux.Window{
		{Index: 4, Name: "node", Label: "server", Active: true},
	}

	rootCmd.SetArgs([]string{"send", "server", "C-c", "-s", "test-session"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("send command failed: %v", err)
	}

	for _, c := range mock.Calls {
		if c.Method == "SendKeys" {
			if len(c.Args) == 0 || c.Args[0] != "test-session:4" {
				t.Errorf("expected target 'test-session:4' (by index), got %v", c.Args)
			}
		}
	}
}

// type shares the resolver — confirm it also reaches an auto-renamed window
// via @zmux_label, targeted by index.
func TestTypeResolvesAutoRenamedWindowByLabel(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	mock.Windows["test-session"] = []tmux.Window{
		{Index: 4, Name: "node", Label: "server", Active: true},
	}

	rootCmd.SetArgs([]string{"type", "server", "echo hi", "-s", "test-session"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("type command failed: %v", err)
	}

	for _, c := range mock.Calls {
		if c.Method == "SendKeys" {
			if len(c.Args) == 0 || c.Args[0] != "test-session:4" {
				t.Errorf("expected target 'test-session:4' (by index), got %v", c.Args)
			}
		}
	}
}

// send claims the name as a stable @zmux_label when it matches an unlabeled
// window by live name — and must do so BEFORE the keys, so a `send X C-c` that
// drifts the name still leaves X reachable for the follow-up command.
func TestSendClaimsLabelBeforeKeysOnNameMatch(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	mock.Windows["test-session"] = []tmux.Window{
		{Index: 2, Name: "devserver", Active: true}, // matched by name, no label yet
	}

	rootCmd.SetArgs([]string{"send", "devserver", "C-c", "-s", "test-session"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("send command failed: %v", err)
	}

	labelIdx, sendIdx := -1, -1
	for i, c := range mock.Calls {
		if c.Method == "SetWindowOption" && len(c.Args) == 3 &&
			c.Args[0] == "test-session:2" && c.Args[1] == "@zmux_label" && c.Args[2] == "devserver" {
			labelIdx = i
		}
		if c.Method == "SendKeys" {
			sendIdx = i
		}
	}
	if labelIdx == -1 {
		t.Fatal("expected send to claim @zmux_label=devserver on the unlabeled name-match")
	}
	if sendIdx == -1 || labelIdx > sendIdx {
		t.Errorf("expected label claim (idx %d) BEFORE SendKeys (idx %d)", labelIdx, sendIdx)
	}
}

// watch is read-only: it resolves an auto-renamed window by label but must
// never claim/mutate window options.
func TestWatchDoesNotClaimLabel(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	mock.Windows["test-session"] = []tmux.Window{
		{Index: 2, Name: "logs", Active: true}, // unlabeled name-match
	}
	mock.CapturedPaneContent = "x\n"

	rootCmd.SetArgs([]string{"watch", "logs", "-s", "test-session"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("watch command failed: %v", err)
	}

	for _, c := range mock.Calls {
		if c.Method == "SetWindowOption" {
			t.Errorf("watch must not mutate window options, got %s %v", c.Method, c.Args)
		}
	}
}

func TestTypeAddsEnter(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}

	rootCmd.SetArgs([]string{"type", "git", "git status", "-s", "test-session"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("type command failed: %v", err)
	}

	for _, c := range mock.Calls {
		if c.Method == "SendKeys" {
			// Should include "Enter" as last arg.
			if len(c.Args) < 3 || c.Args[len(c.Args)-1] != "Enter" {
				t.Errorf("expected Enter as last key, got args: %v", c.Args)
			}
		}
	}
}
