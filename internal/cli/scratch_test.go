package cli

import (
	"strings"
	"testing"

	"github.com/donjor/zmux/internal/tmux"
)

// TestScratchExtractRefusesOutsideTmux verifies the command requires a tmux
// context — the popup contract assumes $TMUX is set.
func TestScratchExtractRefusesOutsideTmux(t *testing.T) {
	app := newDefaultTestApp()
	mock := app.Runner.(*tmux.MockRunner)
	mock.InsideTmux = false

	rootCmd := NewRootCmd(app, testVersion)
	rootCmd.SetArgs([]string{"scratch", "extract"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when running outside tmux")
	}
	if !strings.Contains(err.Error(), "scratch popup") {
		t.Errorf("error should mention scratch popup, got %q", err)
	}
}

// TestScratchVerbRunsInScratchLane verifies the bare-positional verb
// `zmux scratch '<cmd>'` runs in the shared scratch lane and claims its label,
// coexisting with the `extract` subcommand.
func TestScratchVerbRunsInScratchLane(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.NewWindowPaneID = "%7"
	mock.CapturePaneFunc = runResultCaptureFunc(mock, 0)

	rootCmd.SetArgs([]string{"scratch", "bun run lint", "-s", "test-session", "-T", "5"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("scratch verb failed: %v", err)
	}
	var created, labeled bool
	for _, c := range mock.Calls {
		if c.Method == "NewWindow" {
			created = true
			if c.Args[1] != scratchLane {
				t.Errorf("scratch verb should target the scratch lane, got NewWindow name %q", c.Args[1])
			}
		}
		if c.Method == "ApplyOptions" && c.Args[0] == "-p" && c.Args[1] == "%7" &&
			c.Args[2] == "@zmux_label" && c.Args[3] == scratchLane {
			labeled = true
		}
	}
	if !created || !labeled {
		t.Errorf("scratch verb must create + claim the scratch lane: created=%v labeled=%v", created, labeled)
	}
}

// TestScratchVerbReusesExistingScratchLane verifies the verb reuses the lane.
func TestScratchVerbReusesExistingScratchLane(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 2}}
	mock.LogicalRows = []tmux.LogicalPaneRow{logicalRow("%4", "test-session", "@3", 4, "ztab_scratch", scratchLane)}
	mock.CapturePaneFunc = runResultCaptureFunc(mock, 0)

	rootCmd.SetArgs([]string{"scratch", "go test ./...", "-s", "test-session", "-T", "5"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("scratch verb failed: %v", err)
	}
	for _, c := range mock.Calls {
		if c.Method == "NewWindow" {
			t.Errorf("scratch verb must reuse the existing scratch lane, not create a tab: %+v", mock.Calls)
		}
	}
}

// TestScratchExtractCreatesWindowAndClosesPopup verifies the happy path: the
// command asks tmux for the parent session, creates a new window there with
// cwd, then issues display-popup -C to close itself.
func TestScratchExtractCreatesWindowAndClosesPopup(t *testing.T) {
	app := newDefaultTestApp()
	mock := app.Runner.(*tmux.MockRunner)
	mock.InsideTmux = true
	mock.DisplayMessageResult = "myws"

	rootCmd := NewRootCmd(app, testVersion)
	rootCmd.SetArgs([]string{"scratch", "extract"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	var sawNewWindow, sawPopupClose bool
	for _, c := range mock.Calls {
		switch c.Method {
		case "NewWindow":
			sawNewWindow = true
			if len(c.Args) < 1 || c.Args[0] != "myws" {
				t.Errorf("NewWindow target = %v, want myws", c.Args)
			}
		case "DisplayPopup":
			if len(c.Args) >= 1 && c.Args[0] == "-C" {
				sawPopupClose = true
			}
		}
	}
	if !sawNewWindow {
		t.Error("expected NewWindow call")
	}
	if !sawPopupClose {
		t.Error("expected DisplayPopup -C call to close the popup")
	}
}
