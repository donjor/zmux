package cli

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tmux"
)

func TestIsRosterTabName(t *testing.T) {
	roster := []string{"dev", "scratch", "claude", "codex", "codex-peer", "claude-peer", "worker-auth", "worker"}
	for _, n := range roster {
		if !isRosterTabName(n) {
			t.Errorf("%q should be a roster name", n)
		}
	}
	adhoc := []string{"eval-2", "test-run", "test", "build", "lint", "auth-test", "peer", "myworkerish-tab"}
	for _, n := range adhoc {
		if isRosterTabName(n) {
			t.Errorf("%q should NOT be a roster name", n)
		}
	}
}

func TestRunCreatesNewWindow(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}

	rootCmd.SetArgs([]string{"run", "npm run dev", "-n", "server", "-s", "test-session", "-d"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("run command failed: %v", err)
	}

	found := map[string]bool{}
	for _, c := range mock.Calls {
		found[c.Method] = true
	}
	if !found["NewWindow"] {
		t.Error("expected NewWindow call")
	}
	if !found["SendKeys"] {
		t.Error("expected SendKeys call")
	}
}

func TestDetachedRunUntilAcceptsReadinessAlreadyPrintedAfterLaunch(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.NewWindowPaneID = "%7"
	mock.CapturedPaneContent = "ready localhost:43123\n"

	rootCmd.SetArgs([]string{"run", "npm run dev", "-n", "server", "-s", "test-session", "-d", "--until", "ready|localhost", "-T", "2"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("fast readiness emitted before wait registration must still pass: %v", err)
	}
}

func TestDetachedRunUntilAcceptsByteIdenticalReadinessAfterRestart(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.Windows["test-session"] = []tmux.Window{{Index: 1, Name: "server", Label: "server", Active: true}}
	captures := []string{"READY\n", "READY\nREADY\n"}
	mock.CapturePaneFunc = func(_ string, _ int) (string, error) {
		if len(captures) == 1 {
			return captures[0], nil
		}
		output := captures[0]
		captures = captures[1:]
		return output, nil
	}

	rootCmd.SetArgs([]string{"run", "npm run dev", "-n", "server", "-s", "test-session", "-d", "--until", "READY", "-T", "2"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("byte-identical readiness beyond the pre-launch count must pass: %v", err)
	}
}

func TestRunUntilRequiresDetachedRuntime(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}

	rootCmd.SetArgs([]string{"run", "npm run dev", "-n", "server", "-s", "test-session", "--until", "ready"})
	if err := rootCmd.Execute(); err == nil || !strings.Contains(err.Error(), "--until requires --detach") {
		t.Fatalf("expected detached-runtime validation, got %v", err)
	}
}

func TestRunReusesExistingWindow(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.Windows["test-session"] = []tmux.Window{
		{Index: 1, Name: "server", Active: true},
	}

	rootCmd.SetArgs([]string{"run", "npm run dev", "-n", "server", "-s", "test-session", "-d"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("run command failed: %v", err)
	}

	for _, c := range mock.Calls {
		if c.Method == "NewWindow" {
			t.Error("should not create new window when it already exists")
		}
	}
}

// A long-running process makes tmux auto-rename the window away from its zmux
// name (e.g. "server" → "node"). Reuse must still find it via @zmux_label and
// target it by index (session:name no longer resolves).
func TestRunReusesAutoRenamedWindowByLabel(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.Windows["test-session"] = []tmux.Window{
		{Index: 3, Name: "node", Label: "server", Active: true},
	}

	rootCmd.SetArgs([]string{"run", "npm run dev", "-n", "server", "-s", "test-session", "-d"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("run command failed: %v", err)
	}

	sentToIndex := false
	for _, c := range mock.Calls {
		if c.Method == "NewWindow" {
			t.Error("should reuse the auto-renamed window, not create a duplicate")
		}
		if c.Method == "SendKeys" && len(c.Args) > 0 && c.Args[0] == "test-session:3" {
			sentToIndex = true
		}
	}
	if !sentToIndex {
		t.Error("expected SendKeys targeted by index (test-session:3), not by stale name")
	}
}

// An unlabeled tab matched by live name (manual / pre-fix) must get claimed
// as a logical tab on reuse — pane-scoped id + pane-canonical label —
// otherwise it heals only on create and the first restart after auto-rename
// would miss again.
func TestRunBackfillsLabelOnNameReuse(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.Windows["test-session"] = []tmux.Window{
		{Index: 2, Name: "server", Active: true}, // no label yet
	}
	mock.DisplayMessageFunc = displayByFormat(map[string]string{
		"#{pane_id}": "%9\ttest-session:2\n",
	})

	rootCmd.SetArgs([]string{"run", "npm run dev", "-n", "server", "-s", "test-session", "-d"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("run command failed: %v", err)
	}

	var stamped, labeled, mirrored bool
	for _, c := range mock.Calls {
		if c.Method == "NewWindow" {
			t.Error("should reuse the existing tab, not create one")
		}
		if c.Method != "ApplyOptions" {
			continue
		}
		switch {
		case c.Args[0] == "-p" && c.Args[1] == "%9" && c.Args[2] == "@zmux_tab_id":
			stamped = true
		case c.Args[0] == "-p" && c.Args[1] == "%9" && c.Args[2] == "@zmux_label" && c.Args[3] == "server":
			labeled = true
		case c.Args[0] == "-w" && c.Args[1] == "test-session:2" && c.Args[2] == "@zmux_label" && c.Args[3] == "server":
			mirrored = true
		}
	}
	if !stamped || !labeled || !mirrored {
		t.Errorf("expected claim on the reused unlabeled tab: stamped=%v labeled=%v mirrored=%v", stamped, labeled, mirrored)
	}
}

// A labeled tab wins over a different tab that merely shares the live name.
func TestRunPrefersLabelOverNameCollision(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 2}}
	mock.Windows["test-session"] = []tmux.Window{
		{Index: 1, Name: "server", Active: false},               // coincidental live name
		{Index: 5, Name: "node", Label: "server", Active: true}, // the real zmux-run tab
	}

	rootCmd.SetArgs([]string{"run", "npm run dev", "-n", "server", "-s", "test-session", "-d"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("run command failed: %v", err)
	}

	for _, c := range mock.Calls {
		if c.Method == "SendKeys" {
			if len(c.Args) == 0 || c.Args[0] != "test-session:5" {
				t.Errorf("expected SendKeys to the labeled tab (test-session:5), got %v", c.Args)
			}
		}
	}
}

// A named tab created by run must be stamped at birth — pane-scoped id +
// label on the pane NewWindow reports — so a later `run -n <name>` finds the
// logical tab after auto-rename; -d must create it detached.
func TestRunLabelsAndDetachesNewNamedWindow(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.NewWindowPaneID = "%7"

	rootCmd.SetArgs([]string{"run", "npm run dev", "-n", "server", "-s", "test-session", "-d"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("run command failed: %v", err)
	}

	var stamped, labeled, detached bool
	for _, c := range mock.Calls {
		if c.Method == "ApplyOptions" && c.Args[0] == "-p" && c.Args[1] == "%7" {
			switch {
			case c.Args[2] == "@zmux_tab_id":
				stamped = true
			case c.Args[2] == "@zmux_label" && c.Args[3] == "server":
				labeled = true
			}
		}
		if c.Method == "NewWindow" {
			for _, a := range c.Args {
				if a == "detached=true" {
					detached = true
				}
			}
		}
	}
	if !stamped || !labeled {
		t.Errorf("expected new tab stamped with id+label on its pane: stamped=%v labeled=%v", stamped, labeled)
	}
	if !detached {
		t.Error("expected -d to create the window detached (no focus steal)")
	}
}

func TestRunNoFocusCanStillWaitForCompletion(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.NewWindowPaneID = "%8"
	mock.CapturePaneFunc = runResultCaptureFunc(mock, 0)

	rootCmd.SetArgs([]string{"run", "make build", "-n", "build", "-s", "test-session", "--no-focus", "-T", "5"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("run --no-focus should still wait for completion: %v", err)
	}

	var detached, stagedRun bool
	for _, c := range mock.Calls {
		if c.Method == "NewWindow" {
			for _, arg := range c.Args {
				if arg == "detached=true" {
					detached = true
				}
			}
		}
		if c.Method == "SetPaneOption" && len(c.Args) >= 3 && c.Args[1] == tabs.OptNextRunID {
			stagedRun = true
		}
	}
	if !detached {
		t.Error("expected --no-focus to create the new window detached")
	}
	if !stagedRun {
		t.Error("expected --no-focus without -d to retain blocking lifecycle wait")
	}
}

func TestRunDerivesNameFromCommand(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}

	rootCmd.SetArgs([]string{"run", "npm run dev", "-s", "test-session", "-d"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("run command failed: %v", err)
	}

	for _, c := range mock.Calls {
		if c.Method == "NewWindow" && len(c.Args) >= 2 {
			if c.Args[1] != "npm" {
				t.Errorf("expected window name 'npm', got %q", c.Args[1])
			}
		}
	}
}

// runResultCaptureFunc simulates the shell lifecycle hook completing the
// command: once run has staged @zmux_next_run_id, the capture path publishes a
// matching silent @zmux_run_result pane option.
func runResultCaptureFunc(mock *tmux.MockRunner, exitCode int) func(string, int) (string, error) {
	return func(string, int) (string, error) {
		if mock.PaneOptions == nil {
			mock.PaneOptions = map[string]string{}
		}
		for _, c := range mock.Calls {
			if c.Method == "SetPaneOption" && len(c.Args) >= 3 && c.Args[1] == tabs.OptNextRunID {
				mock.PaneOptions[c.Args[0]+"\x00"+tabs.OptRunResult] = fmt.Sprintf("%s:%d", c.Args[2], exitCode)
				return "building...\ndone\n", nil
			}
		}
		return "command not sent yet\n", nil
	}
}

func TestRunWaitDetectsRunResult(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.CapturePaneFunc = runResultCaptureFunc(mock, 0)

	rootCmd.SetArgs([]string{"run", "make build", "-n", "build", "-s", "test-session", "-T", "5"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("run --wait should succeed when run result is published: %v", err)
	}
}

func TestRunWaitDetectsNonZeroExit(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.CapturePaneFunc = runResultCaptureFunc(mock, 1)

	rootCmd.SetArgs([]string{"run", "make build", "-n", "build", "-s", "test-session", "-T", "5"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for non-zero exit code")
	}
}

// A reused tab can still carry a previous run result. The per-invocation nonce
// must keep the wait from matching it — the only acceptable outcome here is a timeout.
func TestRunWaitIgnoresStaleRunResult(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.CapturedPaneContent = "old output\n"
	mock.PaneOptions = map[string]string{"test-session:build\x00" + tabs.OptRunResult: "deadbeef42:0"}

	rootCmd.SetArgs([]string{"run", "make build", "-n", "build", "-s", "test-session", "-T", "1"})
	err := rootCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Errorf("expected timeout (stale run result must not satisfy the wait), got %v", err)
	}
}

// run --wait scans the bounded tail capture, so --lines must bound what it
// reads/prints the same way watch does (shared CapturePane wrapper, P2).
func TestRunWaitCaptureRespectsLines(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.CapturedPaneContent = bigCapture(10) // run result never appears → timeout

	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"run", "make build", "-n", "build", "-s", "test-session", "--lines", "3", "-T", "1"})
		_ = rootCmd.Execute()
	})

	if strings.Contains(out, "line 7\n") {
		t.Errorf("run --lines 3 should not surface head line 7 (only last 3 lines):\n%q", out)
	}
	if !strings.Contains(out, "line 10") {
		t.Errorf("run --lines 3 should surface tail line 10:\n%q", out)
	}
}

func TestRunFailsWithoutSession(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.InsideTmux = false

	rootCmd.SetArgs([]string{"run", "echo hello"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when not in tmux and no --session")
	}
}

func TestRunFailsSessionNotExist(t *testing.T) {
	rootCmd, _ := withMockApp(t)

	rootCmd.SetArgs([]string{"run", "echo hello", "-s", "nonexistent"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when session doesn't exist")
	}
}

// Single-line commands are typed verbatim so they land in the tab's shell
// history (a human can Up-arrow to re-run them). Temp scripts are reserved for
// commands that cannot be delivered as one prompt line.
func TestRunSimpleCommandSendsLiteral(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}

	rootCmd.SetArgs([]string{"run", "bun run dev", "-n", "server", "-s", "test-session", "-d"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("run command failed: %v", err)
	}

	for _, c := range mock.Calls {
		if c.Method == "SendKeys" && len(c.Args) >= 3 && c.Args[1] == "-l" {
			sent := c.Args[2]
			// Literal command at the prompt (Up-arrow re-runs it). Lifecycle
			// is owned by shell hooks, so no sentinel/epilogue is appended.
			if sent != "bun run dev" {
				t.Errorf("expected literal command at the prompt, got %q", sent)
			}
			if strings.HasPrefix(sent, "bash /") {
				t.Errorf("simple command must not be script-wrapped: %q", sent)
			}
			return
		}
	}
	t.Error("expected SendKeys call")
}

func TestRunMultilineCommandUsesScript(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}

	rootCmd.SetArgs([]string{"run", "echo a\necho b", "-n", "multi", "-s", "test-session", "-d"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("run command failed: %v", err)
	}

	for _, c := range mock.Calls {
		if c.Method == "SendKeys" && len(c.Args) >= 3 && c.Args[1] == "-l" {
			if !strings.HasPrefix(c.Args[2], "bash ") || !strings.Contains(c.Args[2], "zmux-cmd-") {
				t.Errorf("expected temp-script indirection in SendKeys, got %q", c.Args[2])
			}
			return
		}
	}
	t.Error("expected SendKeys call")
}

func TestIsSimpleCommand(t *testing.T) {
	simple := []string{
		"bun run dev",
		"npm test",
		"ls",
		`go test ./... -run "TestFoo"`,
		`printf 'a\n'`,
		"echo hi!",
		"-v",
		"Enter",
	}
	for _, c := range simple {
		if !isSimpleCommand(c) {
			t.Errorf("expected simple: %q", c)
		}
	}

	complexCmds := []string{
		"",
		"echo a\necho b", // multiline — each line would submit separately
		"echo a\tb",      // literal tab triggers shell completion
	}
	for _, c := range complexCmds {
		if isSimpleCommand(c) {
			t.Errorf("expected complex: %q", c)
		}
	}
}

// stateWrites extracts the sequence of @zmux_state values written to pane
// options. Run itself should no longer write running/done/failed in the normal
// path; shell-event owns that lifecycle. State writes remain relevant for
// stale-state clears and shell-event tests.
func stateWrites(mock *tmux.MockRunner) []string {
	var out []string
	for _, c := range mock.Calls {
		if c.Method == "ApplyOptions" && c.Args[0] == "-p" && c.Args[2] == "@zmux_state" && c.Args[4] == "unset=false" {
			out = append(out, c.Args[3])
		}
	}
	return out
}

// runStateDisplayFunc routes the session lookup and target resolution used by
// run's stale-state clear and wait-pane resolution.
func runStateDisplayFunc(paneID, window string) func(target, format string) (string, error) {
	return func(target, format string) (string, error) {
		switch {
		case format == "#{session_name}":
			return "test-session", nil
		case strings.Contains(format, "#{pane_id}"):
			return paneID + "\t" + window + "\n", nil
		case strings.Contains(format, "@zmux_state"):
			return "", nil
		}
		return "", nil
	}
}

func TestRunWaitStagesRunIDButDoesNotWriteGlyphState(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.CapturePaneFunc = runResultCaptureFunc(mock, 0)
	mock.DisplayMessageFunc = runStateDisplayFunc("%9", "test-session:5")

	rootCmd.SetArgs([]string{"run", "make build", "-n", "build", "-s", "test-session", "-T", "5"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if got := stateWrites(mock); len(got) != 0 {
		t.Fatalf("run must not write glyph lifecycle state directly, got %v", got)
	}
	staged := false
	for _, c := range mock.Calls {
		if c.Method == "SetPaneOption" && c.Args[1] == tabs.OptNextRunID {
			staged = true
		}
	}
	if !staged {
		t.Fatal("blocking run must stage a pane-option run id")
	}
}

func TestRunWaitPropagatesFailedRunResultWithoutWritingGlyph(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.CapturePaneFunc = runResultCaptureFunc(mock, 2)
	mock.DisplayMessageFunc = runStateDisplayFunc("%9", "test-session:5")

	rootCmd.SetArgs([]string{"run", "make build", "-n", "build", "-s", "test-session", "-T", "5"})
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
	if err := rootCmd.Execute(); err == nil {
		t.Fatal("non-zero exit must propagate as an error")
	}

	if got := stateWrites(mock); len(got) != 0 {
		t.Fatalf("run must not write failed glyph directly, got %v", got)
	}
}

func TestRunFastFailsWhenLifecycleNeverStarts(t *testing.T) {
	oldGrace := runLifecycleStartGrace
	runLifecycleStartGrace = 10 * time.Millisecond
	t.Cleanup(func() { runLifecycleStartGrace = oldGrace })

	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.CapturePaneFunc = func(string, int) (string, error) {
		if mock.PaneOptions == nil {
			mock.PaneOptions = map[string]string{}
		}
		for _, c := range mock.Calls {
			if c.Method == "SetPaneOption" && c.Args[1] == tabs.OptNextRunID {
				mock.PaneOptions[c.Args[0]+"\x00"+tabs.OptNextRunID] = c.Args[2]
			}
		}
		return "already printed output\n", nil
	}
	mock.DisplayMessageFunc = runStateDisplayFunc("%9", "test-session:5")

	rootCmd.SetArgs([]string{"run", "echo ok", "-n", "plain", "-s", "test-session", "-T", "5"})
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
	err := rootCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "waiting for shell lifecycle to start") {
		t.Fatalf("expected lifecycle-start nudge, got %v", err)
	}
	for _, c := range mock.Calls {
		if c.Method == "UnsetPaneOption" && c.Args[1] == tabs.OptNextRunID {
			return
		}
	}
	t.Fatal("fast-fail should clear the staged run id")
}

func TestRunTimeoutDoesNotFabricateState(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.CapturedPaneContent = "still building...\n"
	mock.DisplayMessageFunc = runStateDisplayFunc("%9", "test-session:5")

	rootCmd.SetArgs([]string{"run", "make build", "-n", "build", "-s", "test-session", "-T", "1"})
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
	if err := rootCmd.Execute(); err == nil {
		t.Fatal("timeout must error")
	}

	if got := stateWrites(mock); len(got) != 0 {
		t.Fatalf("timeout must not fabricate glyph state, got %v", got)
	}
}

func TestRunDetachedDoesNotAppendExitEpilogue(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.DisplayMessageFunc = runStateDisplayFunc("%9", "test-session:5")

	rootCmd.SetArgs([]string{"run", "sleep 300", "-n", "work", "-s", "test-session", "-d"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	for _, c := range mock.Calls {
		if c.Method != "SendKeys" || len(c.Args) < 3 || c.Args[1] != "-l" {
			continue
		}
		if sent := c.Args[2]; sent != "sleep 300" {
			t.Fatalf("detached run must send the command without lifecycle epilogue: %q", sent)
		}
		return
	}
	t.Fatal("no SendKeys recorded")
}

// run reuse is typing-by-proxy too (ratified clear table): a stale
// ready/done/failed from the previous run must be cleared BEFORE the new command
// is delivered, same as send/type — not merely overwritten by running after.
func TestRunReuseClearsStaleStateBeforeInput(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.Windows["test-session"] = []tmux.Window{
		{Index: 1, Name: "build", Active: true},
	}
	mock.DisplayMessageFunc = func(target, format string) (string, error) {
		switch {
		case format == "#{session_name}":
			return "test-session", nil
		case strings.Contains(format, "#{pane_id}"):
			return "%9\ttest-session:1\n", nil
		case strings.Contains(format, "@zmux_state"):
			return "failed\n", nil
		}
		return "", nil
	}

	rootCmd.SetArgs([]string{"run", "make build", "-n", "build", "-s", "test-session", "-d"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	clearedBeforeSend := false
	for _, c := range mock.Calls {
		if c.Method == "ApplyOptions" && c.Args[0] == "-p" && c.Args[2] == "@zmux_state" && c.Args[4] == "unset=true" {
			clearedBeforeSend = true
		}
		if c.Method == "SendKeys" {
			break
		}
	}
	if !clearedBeforeSend {
		t.Fatal("run reuse must clear stale failed before delivering input")
	}
}

func TestSendClearsReadyBeforeInput(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.DisplayMessageFunc = func(target, format string) (string, error) {
		switch {
		case format == "#{session_name}":
			return "test-session", nil
		case strings.Contains(format, "#{pane_id}"):
			return "%4\ttest-session:2\n", nil
		case strings.Contains(format, "@zmux_state"):
			return "ready\n", nil
		}
		return "", nil
	}

	rootCmd.SetArgs([]string{"send", "buddy", "C-c"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("send failed: %v", err)
	}

	clearedBeforeSend := false
	for _, c := range mock.Calls {
		if c.Method == "ApplyOptions" && c.Args[0] == "-p" && c.Args[2] == "@zmux_state" && c.Args[4] == "unset=true" {
			clearedBeforeSend = true
		}
		if c.Method == "SendKeys" {
			break
		}
	}
	if !clearedBeforeSend {
		t.Fatal("send must clear ready before delivering input")
	}
}
