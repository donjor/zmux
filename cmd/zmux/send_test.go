package main

import (
	"testing"

	"github.com/donjor/zmux/internal/tmux"
)

func TestSendKeysToWindow(t *testing.T) {
	mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}

	sendSessionFlag = "test-session"
	defer func() { sendSessionFlag = "" }()

	rootCmd.SetArgs([]string{"send", "server", "C-c"})
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

func TestTypeAddsEnter(t *testing.T) {
	mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}

	sendSessionFlag = "test-session"
	defer func() { sendSessionFlag = "" }()

	rootCmd.SetArgs([]string{"type", "git", "git status"})
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
