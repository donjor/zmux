package cli

import (
	"strings"
	"testing"

	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/spf13/cobra"
)

// logTestApp wires a mock runner + in-memory FS with a known LogsDir so the
// deterministic log path is stable, and exposes the FS for seeding/inspection.
func logTestApp(t *testing.T) (*cobra.Command, *tmux.MockRunner, *memFS) {
	t.Helper()
	a, mock := newTestApp(t)
	a.Profile = config.Profile{LogsDir: "/logs"}
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	fs := a.FS.(*memFS)
	return NewRootCmd(a, testVersion), mock, fs
}

// pipeState makes #{pane_pipe} report on/off while keeping other DisplayMessage
// reads (current session, etc.) working.
func pipeState(mock *tmux.MockRunner, on bool) {
	mock.DisplayMessageFunc = func(_, format string) (string, error) {
		if strings.Contains(format, "pane_pipe") {
			if on {
				return "1", nil
			}
			return "0", nil
		}
		return "test-session", nil
	}
}

func findCall(calls []tmux.MockCall, method string) (tmux.MockCall, bool) {
	for _, c := range calls {
		if c.Method == method {
			return c, true
		}
	}
	return tmux.MockCall{}, false
}

func TestLogStartOpensPipeAndRecordsPath(t *testing.T) {
	rootCmd, mock, _ := logTestApp(t)
	pipeState(mock, false) // not already piped

	rootCmd.SetArgs([]string{"log", "start", "server", "-s", "test-session"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("log start failed: %v", err)
	}

	pipe, ok := findCall(mock.Calls, "PipePane")
	if !ok {
		t.Fatal("expected a PipePane call")
	}
	if pipe.Args[0] != "test-session:server" {
		t.Errorf("PipePane target = %q, want test-session:server", pipe.Args[0])
	}
	cmd := pipe.Args[1]
	if !strings.Contains(cmd, "log-sink --file") || !strings.Contains(cmd, "--max-bytes") {
		t.Errorf("pipe command should invoke the sink: %q", cmd)
	}
	if !strings.Contains(cmd, "/logs/test-session__test-session-server.log") {
		t.Errorf("pipe command should target the deterministic log path: %q", cmd)
	}
	set, ok := findCall(mock.Calls, "SetPaneOption")
	if !ok || set.Args[1] != optLog {
		t.Errorf("expected SetPaneOption %s, got %v", optLog, set)
	}
}

func TestLogStartRefusesWhenAlreadyLogging(t *testing.T) {
	rootCmd, mock, _ := logTestApp(t)
	pipeState(mock, true) // already piped

	rootCmd.SetArgs([]string{"log", "start", "server", "-s", "test-session"})
	err := rootCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "already being logged") {
		t.Fatalf("expected already-logging error, got %v", err)
	}
	if _, ok := findCall(mock.Calls, "PipePane"); ok {
		t.Error("must not open a second pipe when one is already active")
	}
}

func TestLogStopClosesPipe(t *testing.T) {
	rootCmd, mock, _ := logTestApp(t)
	pipeState(mock, true) // currently piped

	rootCmd.SetArgs([]string{"log", "stop", "server", "-s", "test-session"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("log stop failed: %v", err)
	}

	pipe, ok := findCall(mock.Calls, "PipePane")
	if !ok {
		t.Fatal("expected a PipePane call to close the pipe")
	}
	if pipe.Args[1] != "" {
		t.Errorf("stop must pass an empty command (got %q) to close the pipe", pipe.Args[1])
	}
	if _, ok := findCall(mock.Calls, "UnsetPaneOption"); !ok {
		t.Error("stop should clear the @zmux_log pane option")
	}
}

func TestLogStopRefusesWhenNotLogging(t *testing.T) {
	rootCmd, mock, _ := logTestApp(t)
	pipeState(mock, false) // not piped

	rootCmd.SetArgs([]string{"log", "stop", "server", "-s", "test-session"})
	err := rootCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "not being logged") {
		t.Fatalf("expected not-logging error, got %v", err)
	}
}

func TestLogTailPrintsRecordedFile(t *testing.T) {
	rootCmd, mock, fs := logTestApp(t)
	pipeState(mock, true)
	path := "/logs/test-session__test-session-server.log"
	mock.PaneOptions = map[string]string{"test-session:server\x00" + optLog: path}
	fs.files[path] = []byte("first line\nsecond line\nthird line\n")

	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"log", "tail", "server", "-s", "test-session", "-n", "2"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("log tail failed: %v", err)
		}
	})

	if strings.Contains(out, "first line") {
		t.Errorf("--lines 2 should drop the oldest line, got:\n%q", out)
	}
	if !strings.Contains(out, "second line") || !strings.Contains(out, "third line") {
		t.Errorf("tail should print the last 2 lines, got:\n%q", out)
	}
}

func TestLogTailErrorsWhenNoLog(t *testing.T) {
	rootCmd, mock, _ := logTestApp(t)
	pipeState(mock, false)

	rootCmd.SetArgs([]string{"log", "tail", "server", "-s", "test-session"})
	err := rootCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "no log for tab") {
		t.Fatalf("expected no-log error, got %v", err)
	}
}

func TestLogStatusIncludesRawWindowRecording(t *testing.T) {
	rootCmd, mock, _ := logTestApp(t)
	// A raw/legacy window (no @zmux_tab_id) logged via resolveTabTarget's
	// fallback — the kind ListLogicalTabs drops. Scanning only logical tabs
	// would report "nothing" while this recording is live, so status must scan
	// all panes and surface it by window name.
	logPath := "/logs/test-session__-4.log"
	mock.LogicalRows = []tmux.LogicalPaneRow{
		{PaneID: "%4", Session: "test-session", WindowName: "legacy-build"},
	}
	mock.PaneOptions = map[string]string{"%4\x00" + optLog: logPath}
	mock.DisplayMessageFunc = func(target, format string) (string, error) {
		if strings.Contains(format, "pane_pipe") {
			if target == "%4" {
				return "1", nil
			}
			return "0", nil
		}
		return "test-session", nil
	}

	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"log", "status"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("log status failed: %v", err)
		}
	})
	if strings.Contains(out, "no tabs are being logged") {
		t.Fatalf("status dropped a raw-window recording:\n%s", out)
	}
	for _, want := range []string{"legacy-build", "%4", logPath} {
		if !strings.Contains(out, want) {
			t.Errorf("status missing %q:\n%s", want, out)
		}
	}
}
