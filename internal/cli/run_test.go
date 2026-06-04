package cli

import (
	"testing"

	"github.com/donjor/zmux/internal/tmux"
)

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

// An unlabeled tab matched by live name (manual / pre-fix) must get the stable
// label backfilled on reuse — otherwise it heals only on create and the first
// restart after auto-rename would miss again.
func TestRunBackfillsLabelOnNameReuse(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.Windows["test-session"] = []tmux.Window{
		{Index: 2, Name: "server", Active: true}, // no label yet
	}

	rootCmd.SetArgs([]string{"run", "npm run dev", "-n", "server", "-s", "test-session", "-d"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("run command failed: %v", err)
	}

	backfilled := false
	for _, c := range mock.Calls {
		if c.Method == "NewWindow" {
			t.Error("should reuse the existing tab, not create one")
		}
		if c.Method == "SetWindowOption" && len(c.Args) == 3 &&
			c.Args[0] == "test-session:2" && c.Args[1] == "@zmux_label" && c.Args[2] == "server" {
			backfilled = true
		}
	}
	if !backfilled {
		t.Error("expected @zmux_label backfilled on the reused unlabeled tab")
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

// A named tab created by run must be tagged with @zmux_label so a later
// `run -n <name>` reuses it after auto-rename; -d must create it detached.
func TestRunLabelsAndDetachesNewNamedWindow(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}

	rootCmd.SetArgs([]string{"run", "npm run dev", "-n", "server", "-s", "test-session", "-d"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("run command failed: %v", err)
	}

	labeled, detached := false, false
	for _, c := range mock.Calls {
		if c.Method == "SetWindowOption" && len(c.Args) == 3 &&
			c.Args[1] == "@zmux_label" && c.Args[2] == "server" {
			labeled = true
		}
		if c.Method == "NewWindow" {
			for _, a := range c.Args {
				if a == "detached=true" {
					detached = true
				}
			}
		}
	}
	if !labeled {
		t.Error("expected @zmux_label set to 'server' on the new tab")
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

func TestRunWaitDetectsSentinel(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	// Mock CapturePane to return output with the sentinel.
	mock.CapturedPaneContent = "building...\ndone\n:::AGENT_DONE 0:::\n"

	rootCmd.SetArgs([]string{"run", "make build", "-n", "build", "-s", "test-session", "-T", "5"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("run --wait should succeed when sentinel found: %v", err)
	}
}

func TestRunWaitDetectsNonZeroExit(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.CapturedPaneContent = "error: build failed\n:::AGENT_DONE 1:::\n"

	rootCmd.SetArgs([]string{"run", "make build", "-n", "build", "-s", "test-session", "-T", "5"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for non-zero exit code")
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
