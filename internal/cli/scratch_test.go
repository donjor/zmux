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
