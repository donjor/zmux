package cli

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tmux"
)

// T-104 (055 P-001) — run-mechanics move-equivalence characterization. These
// pin the units that S-009/T-402 relocate to internal/runexec (script writing,
// run-result lifecycle polling, readiness, nonce/exit parsing, start-grace
// clamping, output dedupe, stdout/stderr split) so the move can be proven
// behavior-preserving. The existing run_test.go already covers success/nonzero/
// stale-nonce/start-timeout/result-timeout/quoting-vs-script/lines; this file
// fills the remaining acceptance rows.
//
// NOTE for T-402: interrupt (Ctrl+C) semantics and follow growth/reset run
// inside blocking loops that only exit on os.Interrupt. They become
// deterministically testable once the extraction introduces the ctx/writers
// seam S-009 mandates; they are pinned there, not here (recorded in
// worker-notes).

// --- writeCommandScript content contract (moves to runexec) ---

func TestWriteCommandScriptContract(t *testing.T) {
	path, cleanup, err := writeCommandScript("echo hi", 30)
	if err != nil {
		t.Fatalf("writeCommandScript: %v", err)
	}
	if cleanup != nil {
		defer cleanup()
	}
	defer os.Remove(path)

	if !strings.HasPrefix(filepathBase(path), "zmux-cmd-") || !strings.HasSuffix(path, ".sh") {
		t.Fatalf("script path not a zmux-cmd temp file: %q", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read script: %v", err)
	}
	script := string(data)

	wantParts := []string{
		"#!/usr/bin/env bash\n",                    // shebang
		`printf '\033[2m$ %s\033[0m\n'`,            // dim echo of the command
		"__zmux_cmd_cleanup() { __zmux_status=$?;", // preserve exit status
		"rm -f",                        // self-delete
		"trap __zmux_cmd_cleanup EXIT", // cleanup on exit
		"echo hi\n",                    // the command body, verbatim
	}
	for _, part := range wantParts {
		if !strings.Contains(script, part) {
			t.Fatalf("script missing %q:\n%s", part, script)
		}
	}
}

func TestWriteCommandScriptEscapesEchoedCommand(t *testing.T) {
	// The echoed (printf %q) copy escapes backslashes and quotes so the dim
	// preview line cannot break the script; the executed body stays verbatim.
	path, cleanup, err := writeCommandScript(`grep "a\b" .`, 30)
	if err != nil {
		t.Fatalf("writeCommandScript: %v", err)
	}
	if cleanup != nil {
		defer cleanup()
	}
	defer os.Remove(path)
	data, _ := os.ReadFile(path)
	script := string(data)
	if !strings.Contains(script, `\\b`) || !strings.Contains(script, `\"a`) {
		t.Fatalf("echoed command not escaped (backslash/quote):\n%s", script)
	}
	// The real command runs verbatim on its own line.
	if !strings.Contains(script, "\n"+`grep "a\b" .`+"\n") {
		t.Fatalf("command body not preserved verbatim:\n%s", script)
	}
}

// --- start-grace clamp (pure) ---

func TestRunLifecycleStartDeadline(t *testing.T) {
	grace := runLifecycleStartGrace
	cases := []struct {
		name    string
		timeout time.Duration
		want    time.Duration
	}{
		{"zero timeout uses full grace", 0, grace},
		{"timeout above grace clamps to grace", grace + time.Minute, grace},
		{"timeout below grace uses the shorter timeout", time.Second, time.Second},
		{"timeout equal to grace uses grace", grace, grace},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := runLifecycleStartDeadline(c.timeout); got != c.want {
				t.Fatalf("runLifecycleStartDeadline(%v) = %v, want %v", c.timeout, got, c.want)
			}
		})
	}
}

// --- nonce-scoped run-result parsing (moves to runexec) ---

func TestRunResultExitNonceMatching(t *testing.T) {
	app, mock := newTestApp(t)
	pane := "%9"
	set := func(v string) {
		mock.PaneOptions = map[string]string{pane + "\x00" + tabs.OptRunResult: v}
	}

	set("abc123:0")
	if code, ok := runResultExit(app, pane, "abc123"); !ok || code != 0 {
		t.Fatalf("matching nonce exit 0: code=%d ok=%v", code, ok)
	}
	set("abc123:7")
	if code, ok := runResultExit(app, pane, "abc123"); !ok || code != 7 {
		t.Fatalf("matching nonce exit 7: code=%d ok=%v", code, ok)
	}
	set("deadbeef:0")
	if _, ok := runResultExit(app, pane, "abc123"); ok {
		t.Fatal("wrong nonce must not match")
	}
	set("abc123:notint")
	if _, ok := runResultExit(app, pane, "abc123"); ok {
		t.Fatal("malformed exit code must not match")
	}
	set("")
	if _, ok := runResultExit(app, pane, "abc123"); ok {
		t.Fatal("empty run result must not match")
	}
}

func TestRunLifecycleStartedDetectsConsumedNonce(t *testing.T) {
	app, mock := newTestApp(t)
	pane := "%9"
	mock.PaneOptions = map[string]string{pane + "\x00" + tabs.OptNextRunID: "abc123"}
	if runLifecycleStarted(app, pane, "abc123") {
		t.Fatal("staged nonce still present -> lifecycle not started yet")
	}
	mock.PaneOptions[pane+"\x00"+tabs.OptNextRunID] = ""
	if !runLifecycleStarted(app, pane, "abc123") {
		t.Fatal("nonce cleared/consumed -> lifecycle started")
	}
}

// --- output dedupe + stdout/stderr split (waitForRunResult printNew) ---

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

func filepathBase(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[i+1:]
	}
	return p
}
