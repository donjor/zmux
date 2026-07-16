package cli

import (
	"strings"
	"testing"

	"github.com/donjor/zmux/internal/tmux"
)

// T-104 (055 P-001) — run-mechanics move-equivalence characterization. The
// unit-level pins (script writing, run-result/nonce parsing, start-grace
// clamping, plus the newly-testable interrupt and follow growth/reset loops)
// moved with the code into internal/runexec (see runexec_test.go). What stays
// here are the end-to-end rows that exercise the whole `zmux run` command
// through the extracted Executor: output dedupe and the stdout/stderr split.

// --- output dedupe + stdout/stderr split (WaitResult printNew, via run) ---

func TestRunWaitDedupesRepeatedOutputLines(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}

	// Each capture repeats an unchanged banner plus one already-seen line; the
	// printNew dedupe must emit "banner" exactly once across ticks. The result
	// is published on a later tick so the wait completes deterministically.
	base := runResultCaptureFunc(mock, 0)
	calls := 0
	mock.CapturePaneFunc = func(target string, lines int) (string, error) {
		calls++
		if calls < 3 {
			return "banner\nbanner\n", nil
		}
		return base(target, lines)
	}

	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"run", "make build", "-n", "build", "-s", "test-session", "-T", "5"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("run should succeed: %v", err)
		}
	})
	if n := strings.Count(out, "banner"); n != 1 {
		t.Fatalf("repeated banner should print once, printed %d times:\n%q", n, out)
	}
}

func TestRunStatusMessagesGoToStderrNotStdout(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}

	var stdout string
	stderr := captureStderr(t, func() {
		stdout = captureStdout(t, func() {
			rootCmd.SetArgs([]string{"run", "sleep 300", "-n", "server", "-s", "test-session", "-d"})
			if err := rootCmd.Execute(); err != nil {
				t.Fatalf("detached run failed: %v", err)
			}
		})
	})
	if !strings.Contains(stderr, "running in test-session:server") {
		t.Fatalf("status line should be on stderr: %q", stderr)
	}
	if strings.Contains(stdout, "running in") {
		t.Fatalf("status line must not leak to stdout: %q", stdout)
	}
}
