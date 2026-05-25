package cli

import (
	"sync/atomic"
	"testing"

	"github.com/donjor/zmux/internal/tmux"
)

func TestWatchCapturesOutput(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	mock.CapturedPaneContent = "line 1\nline 2\nline 3\n"

	rootCmd.SetArgs([]string{"watch", "server", "-s", "test-session"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("watch command failed: %v", err)
	}

	found := false
	for _, c := range mock.Calls {
		if c.Method == "CapturePane" {
			found = true
			if len(c.Args) > 0 && c.Args[0] != "test-session:server" {
				t.Errorf("expected target 'test-session:server', got %q", c.Args[0])
			}
		}
	}
	if !found {
		t.Error("expected CapturePane call")
	}
}

func TestWatchUntilIgnoresBaselineOutput(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}

	// Simulate: baseline has "ready" from a previous run.
	// New output after baseline has different content.
	// Pattern should NOT match baseline, only new output.
	var callCount atomic.Int32
	mock.CapturePaneFunc = func(target string, lines int) (string, error) {
		n := callCount.Add(1)
		if n <= 1 {
			// First call = baseline capture. Contains stale "ready" from before.
			return "old stuff\nready\nmore old stuff\n", nil
		}
		if n <= 3 {
			// Polls 2-3: same old content, no new lines yet.
			return "old stuff\nready\nmore old stuff\n", nil
		}
		// Poll 4+: new output appears with fresh "ready" signal.
		return "old stuff\nready\nmore old stuff\nstarting server...\nready on port 3000\n", nil
	}

	rootCmd.SetArgs([]string{"watch", "server", "-s", "test-session", "--until", "ready on port", "-T", "5"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("watch --until should match new output: %v", err)
	}

	// Should have made multiple CapturePane calls (baseline + polls).
	count := callCount.Load()
	if count < 3 {
		t.Errorf("expected at least 3 CapturePane calls (baseline + polls), got %d", count)
	}
}

func TestWatchUntilTimesOutWhenPatternOnlyInBaseline(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}

	// Pattern "ready" only exists in baseline. No new output contains it.
	mock.CapturedPaneContent = "old stuff\nready\nmore old stuff\n"

	rootCmd.SetArgs([]string{"watch", "server", "-s", "test-session", "--until", "ready", "-T", "2"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected timeout when pattern only exists in baseline")
	}
}

func TestWatchUntilMatchesNewOutputImmediately(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}

	var callCount atomic.Int32
	mock.CapturePaneFunc = func(target string, lines int) (string, error) {
		n := callCount.Add(1)
		if n <= 1 {
			// Baseline: no "error" present.
			return "starting...\n", nil
		}
		// First poll: "error" appears — should match immediately.
		return "starting...\nerror: build failed\n", nil
	}

	rootCmd.SetArgs([]string{"watch", "server", "-s", "test-session", "--until", "error", "-T", "5"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("should match new 'error' line immediately: %v", err)
	}
}
