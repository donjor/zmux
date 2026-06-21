package cli

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

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

// sentinelCaptureFunc simulates the command completing in the tab: it scrapes
// the nonce off the sentinel the run command actually sent, and returns a
// capture containing that sentinel with the given exit code.
func sentinelCaptureFunc(mock *tmux.MockRunner, exitCode int) func(string, int) (string, error) {
	re := regexp.MustCompile(`:::AGENT_DONE:([0-9a-f]+):\$\?:::`)
	return func(string, int) (string, error) {
		for _, c := range mock.Calls {
			if c.Method != "SendKeys" {
				continue
			}
			if m := re.FindStringSubmatch(strings.Join(c.Args, " ")); m != nil {
				return fmt.Sprintf("building...\ndone\n:::AGENT_DONE:%s:%d:::\n", m[1], exitCode), nil
			}
		}
		return "command not sent yet\n", nil
	}
}

func TestRunWaitDetectsSentinel(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.CapturePaneFunc = sentinelCaptureFunc(mock, 0)

	rootCmd.SetArgs([]string{"run", "make build", "-n", "build", "-s", "test-session", "-T", "5"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("run --wait should succeed when sentinel found: %v", err)
	}
}

func TestRunWaitDetectsNonZeroExit(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.CapturePaneFunc = sentinelCaptureFunc(mock, 1)

	rootCmd.SetArgs([]string{"run", "make build", "-n", "build", "-s", "test-session", "-T", "5"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for non-zero exit code")
	}
}

// A reused tab can still show a previous run's sentinel on screen (or one
// recalled from shell history). The per-invocation nonce must keep the wait
// from matching it — the only acceptable outcome here is a timeout.
func TestRunWaitIgnoresStaleSentinel(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.CapturedPaneContent = "old output\n:::AGENT_DONE:deadbeef42:0:::\n:::AGENT_DONE 0:::\n"

	rootCmd.SetArgs([]string{"run", "make build", "-n", "build", "-s", "test-session", "-T", "1"})
	err := rootCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Errorf("expected timeout (stale sentinel must not satisfy the wait), got %v", err)
	}
}

// run --wait scans the bounded tail capture, so --lines must bound what it
// reads/prints the same way watch does (shared CapturePane wrapper, P2).
func TestRunWaitCaptureRespectsLines(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.CapturedPaneContent = bigCapture(10) // sentinel never appears → timeout

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

// Simple commands are typed verbatim so they land in the tab's shell history
// (a human can Up-arrow to re-run them); only commands interactive bash or
// send-keys could reinterpret go via the temp-script indirection.
func TestRunSimpleCommandSendsLiteral(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}

	rootCmd.SetArgs([]string{"run", "bun run dev", "-n", "server", "-s", "test-session", "-d"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("run command failed: %v", err)
	}

	for _, c := range mock.Calls {
		if c.Method == "SendKeys" {
			sent := c.Args[1]
			// Literal command at the prompt (Up-arrow re-runs it) — the
			// detached state-exit epilogue rides the same line, like the
			// wait-mode sentinel does. No temp-script indirection.
			if !strings.HasPrefix(sent, "bun run dev; ") {
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

func TestRunComplexCommandUsesScript(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}

	rootCmd.SetArgs([]string{"run", "echo a\necho b", "-n", "multi", "-s", "test-session", "-d"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("run command failed: %v", err)
	}

	for _, c := range mock.Calls {
		if c.Method == "SendKeys" {
			if !strings.HasPrefix(c.Args[1], "bash ") || !strings.Contains(c.Args[1], "zmux-cmd-") {
				t.Errorf("expected temp-script indirection in SendKeys, got %q", c.Args[1])
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
		`npm test; echo ":::AGENT_DONE:1a2b3c4d:$?:::"`, // wait-mode sentinel suffix stays simple
	}
	for _, c := range simple {
		if !isSimpleCommand(c) {
			t.Errorf("expected simple: %q", c)
		}
	}

	complexCmds := []string{
		"",
		"echo a\necho b", // multiline — each line would submit separately
		"echo hi!",       // bash history expansion
		`printf 'a\n'`,   // backslash escapes
		"echo a\tb",      // literal tab triggers shell completion
		"-v",             // leading dash — send-keys flag
		"Enter",          // tmux key-name collision
	}
	for _, c := range complexCmds {
		if isSimpleCommand(c) {
			t.Errorf("expected complex: %q", c)
		}
	}
}

// stateWrites extracts the sequence of @zmux_state values written to pane
// options — the lifecycle trail run/send/type leave behind. State writes are
// batched: the mock records one ApplyOptions entry per write as
// [scope target key value unset=bool].
func stateWrites(mock *tmux.MockRunner) []string {
	var out []string
	for _, c := range mock.Calls {
		if c.Method == "ApplyOptions" && c.Args[0] == "-p" && c.Args[2] == "@zmux_state" && c.Args[4] == "unset=false" {
			out = append(out, c.Args[3])
		}
	}
	return out
}

// runStateDisplayFunc routes the session lookup and target resolution that
// run's state writes perform; CapturePaneFunc handles the sentinel side.
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

func TestRunWaitWritesRunningThenDone(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.CapturePaneFunc = sentinelCaptureFunc(mock, 0)
	mock.DisplayMessageFunc = runStateDisplayFunc("%9", "test-session:5")

	rootCmd.SetArgs([]string{"run", "make build", "-n", "build", "-s", "test-session", "-T", "5"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	got := stateWrites(mock)
	if len(got) != 2 || got[0] != "running" || got[1] != "done" {
		t.Fatalf("want [running done], got %v", got)
	}
}

func TestRunWaitWritesFailedWithExitMsg(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.CapturePaneFunc = sentinelCaptureFunc(mock, 2)
	mock.DisplayMessageFunc = runStateDisplayFunc("%9", "test-session:5")

	rootCmd.SetArgs([]string{"run", "make build", "-n", "build", "-s", "test-session", "-T", "5"})
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
	if err := rootCmd.Execute(); err == nil {
		t.Fatal("non-zero exit must propagate as an error")
	}

	got := stateWrites(mock)
	if len(got) != 2 || got[1] != "failed" {
		t.Fatalf("want [running failed], got %v", got)
	}
	msgFound := false
	for _, c := range mock.Calls {
		if c.Method == "ApplyOptions" && c.Args[0] == "-p" && c.Args[2] == "@zmux_state_msg" && c.Args[3] == "exit 2" {
			msgFound = true
		}
	}
	if !msgFound {
		t.Fatal("failed state must carry msg 'exit 2'")
	}
}

func TestRunTimeoutLeavesRunning(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.CapturedPaneContent = "still building...\n" // sentinel never appears
	mock.DisplayMessageFunc = runStateDisplayFunc("%9", "test-session:5")

	rootCmd.SetArgs([]string{"run", "make build", "-n", "build", "-s", "test-session", "-T", "1"})
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
	if err := rootCmd.Execute(); err == nil {
		t.Fatal("timeout must error")
	}

	got := stateWrites(mock)
	if len(got) != 1 || got[0] != "running" {
		t.Fatalf("timeout must not fabricate done/failed — want [running], got %v", got)
	}
}

// Detached runs get the state-exit epilogue: nobody waits on a sentinel,
// so the pane itself reports done/failed when the command exits — otherwise
// the running glyph (spinner) never stops.
func TestRunDetachedAppendsExitEpilogue(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.DisplayMessageFunc = runStateDisplayFunc("%9", "test-session:5")

	rootCmd.SetArgs([]string{"run", "sleep 300", "-n", "work", "-s", "test-session", "-d"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	for _, c := range mock.Calls {
		if c.Method != "SendKeys" {
			continue
		}
		sent := c.Args[1]
		if !strings.HasPrefix(sent, "sleep 300; ") || !strings.HasSuffix(sent, " tab state-exit $?") {
			t.Fatalf("detached run must append the state-exit epilogue: %q", sent)
		}
		if strings.Contains(sent, zmuxSentinelPrefix) {
			t.Fatalf("detached run must not carry a wait sentinel: %q", sent)
		}
		return
	}
	t.Fatal("no SendKeys recorded")
}

// run reuse is typing-by-proxy too (ratified clear table): a stale
// done|failed from the previous run must be cleared BEFORE the new command
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

func TestSendClearsDoneBeforeInput(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.DisplayMessageFunc = func(target, format string) (string, error) {
		switch {
		case format == "#{session_name}":
			return "test-session", nil
		case strings.Contains(format, "#{pane_id}"):
			return "%4\ttest-session:2\n", nil
		case strings.Contains(format, "@zmux_state"):
			return "done\n", nil
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
		t.Fatal("send must clear done before delivering input")
	}
}
