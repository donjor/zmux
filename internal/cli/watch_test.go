package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
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

// watch must read from an auto-renamed window via its @zmux_label, targeting
// by index rather than the stale session:name.
func TestWatchResolvesAutoRenamedWindowByLabel(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	mock.Windows["test-session"] = []tmux.Window{
		{Index: 4, Name: "node", Label: "server", Active: true},
	}
	mock.CapturedPaneContent = "line 1\n"

	rootCmd.SetArgs([]string{"watch", "server", "-s", "test-session"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("watch command failed: %v", err)
	}

	for _, c := range mock.Calls {
		if c.Method == "CapturePane" {
			if len(c.Args) == 0 || c.Args[0] != "test-session:4" {
				t.Errorf("expected target 'test-session:4' (by index), got %v", c.Args)
			}
		}
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

func TestWatchIdleReturnsAfterStableOutput(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}

	// Output changes for the first two polls, then goes stable.
	var callCount atomic.Int32
	mock.CapturePaneFunc = func(target string, lines int) (string, error) {
		n := callCount.Add(1)
		if n <= 2 {
			return fmt.Sprintf("streaming chunk %d\n", n), nil
		}
		return "streaming chunk 2\nfinal answer\n", nil
	}

	rootCmd.SetArgs([]string{"watch", "server", "-s", "test-session", "--idle", "1", "-T", "10"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("watch --idle should return once output is stable: %v", err)
	}

	// Needs the changing polls plus at least the 1s stability window (2+ polls).
	if count := callCount.Load(); count < 4 {
		t.Errorf("expected at least 4 CapturePane calls (changes + stability window), got %d", count)
	}
}

// A screen that is static because the process is quietly working (e.g. an
// agent CLI waiting on a remote model) still counts as stable — returning
// there is BY DESIGN. zmux judges only screen stability; the caller judges
// what the capture means.
func TestWatchIdleReturnsOnStaticScreenRegardlessOfProcessState(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	mock.CapturedPaneContent = "> prompt submitted\n(no answer yet)\n"

	rootCmd.SetArgs([]string{"watch", "server", "-s", "test-session", "--idle", "1", "-T", "10"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("static screen must satisfy --idle even mid-turn: %v", err)
	}
}

func TestWatchIdleTimesOutWhenOutputKeepsChanging(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}

	// Every capture is different — never stable.
	var callCount atomic.Int32
	mock.CapturePaneFunc = func(target string, lines int) (string, error) {
		return fmt.Sprintf("tick %d\n", callCount.Add(1)), nil
	}

	rootCmd.SetArgs([]string{"watch", "server", "-s", "test-session", "--idle", "2", "-T", "1"})
	if err := rootCmd.Execute(); err == nil {
		t.Error("expected timeout error when output never stabilizes")
	}
}

// Capture errors must not age the stability streak into a false "stable".
func TestWatchIdleCaptureErrorResetsStability(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}

	// One good capture, then persistent errors. Without the reset, the
	// initial capture's streak would age past --idle and report success.
	var callCount atomic.Int32
	mock.CapturePaneFunc = func(target string, lines int) (string, error) {
		if callCount.Add(1) == 1 {
			return "looks stable\n", nil
		}
		return "", fmt.Errorf("pane gone")
	}

	rootCmd.SetArgs([]string{"watch", "server", "-s", "test-session", "--idle", "1", "-T", "2"})
	if err := rootCmd.Execute(); err == nil {
		t.Error("expected timeout — capture errors must not count toward stability")
	}
}

// An already-stable pane must succeed even with --timeout == --idle: the
// streak is seeded by an immediate capture (not the first 500ms tick) and
// stability is evaluated before the deadline within a tick. Pre-fix this
// combination could never succeed (codex-review catch, dogfood run).
func TestWatchIdleStableFromStartSucceedsWithEqualTimeout(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	mock.CapturedPaneContent = "same screen\n"

	rootCmd.SetArgs([]string{"watch", "server", "-s", "test-session", "--idle", "1", "-T", "1"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("stable-from-start pane must beat --timeout == --idle: %v", err)
	}
}

// When captures fail persistently, the timeout error must say so instead of
// the misleading "output never stable".
func TestWatchIdleTimeoutReportsPersistentCaptureFailure(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	mock.CapturePaneFunc = func(target string, lines int) (string, error) {
		return "", fmt.Errorf("pane gone")
	}

	rootCmd.SetArgs([]string{"watch", "server", "-s", "test-session", "--idle", "1", "-T", "1"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "captures failing") {
		t.Errorf("timeout error should surface capture failure, got: %v", err)
	}
}

func TestWatchIdleRejectsInvalidValues(t *testing.T) {
	for _, args := range [][]string{
		{"watch", "server", "-s", "test-session", "--idle", "0"},
		{"watch", "server", "-s", "test-session", "--idle", "-3"},
		{"watch", "server", "-s", "test-session", "--idle", "3", "--until", "ready"},
		{"watch", "server", "-s", "test-session", "--idle", "3", "-f"},
	} {
		rootCmd, mock := withMockApp(t)
		mock.Sessions = []tmux.Session{{Name: "test-session"}}
		rootCmd.SetArgs(args)
		if err := rootCmd.Execute(); err == nil {
			t.Errorf("expected error for args %v", args)
		}
	}
}

// ── --lines bounds capture height (P2) ──

// captureStdout redirects os.Stdout for the duration of fn and returns
// everything written to it. watch prints via fmt.Print (os.Stdout), so this
// is the only way to assert the printed line count.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		var b strings.Builder
		_, _ = io.Copy(&b, r)
		done <- b.String()
	}()
	fn()
	_ = w.Close()
	os.Stdout = orig
	return <-done
}

func nonEmptyLineCount(s string) int {
	n := 0
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			n++
		}
	}
	return n
}

// bigCapture returns n numbered lines ("line 1\n…line n\n").
func bigCapture(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	return b.String()
}

func TestWatchPlainRespectsLines(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	mock.CapturedPaneContent = bigCapture(10) // capture is larger than --lines

	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"watch", "server", "-s", "test-session", "--lines", "3"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("watch failed: %v", err)
		}
	})

	if got := nonEmptyLineCount(out); got > 3 {
		t.Errorf("watch --lines 3 printed %d lines, want <= 3:\n%q", got, out)
	}
	// And it must be the tail, not the head.
	if !strings.Contains(out, "line 10") || strings.Contains(out, "line 1\n") {
		t.Errorf("watch --lines 3 should print the tail (line 10, not line 1), got:\n%q", out)
	}
}

func TestWatchUntilTimeoutCaptureRespectsLines(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	mock.CapturedPaneContent = bigCapture(10) // pattern never matches → timeout prints capture

	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"watch", "server", "-s", "test-session", "--until", "NEVER_MATCHES", "--lines", "3", "-T", "1"})
		_ = rootCmd.Execute() // expected timeout
	})

	if got := nonEmptyLineCount(out); got > 3 {
		t.Errorf("watch --until best-effort capture printed %d lines, want <= 3:\n%q", got, out)
	}
}

func TestWatchIdleStableCaptureRespectsLines(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	mock.CapturedPaneContent = bigCapture(10) // stable from the start

	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"watch", "server", "-s", "test-session", "--idle", "1", "--lines", "3", "-T", "5"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("watch --idle on a stable pane should succeed: %v", err)
		}
	})

	if got := nonEmptyLineCount(out); got > 3 {
		t.Errorf("watch --idle capture printed %d lines, want <= 3:\n%q", got, out)
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
