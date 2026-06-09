//go:build integration

package tests

import (
	"os/exec"
	"strings"
	"testing"
)

// tmuxAt drives a throwaway tmux server on a private socket (-L) with no
// config — the same clean-room shape as the plan 026 spike pack. Socket-
// scoped tmux is the sanctioned path for testing tmux behavior itself.
func tmuxAt(t *testing.T, sock string, args ...string) string {
	t.Helper()
	full := append([]string{"-L", sock, "-f", "/dev/null"}, args...)
	out, err := exec.Command("tmux", full...).CombinedOutput()
	if err != nil {
		t.Fatalf("tmux %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// TestPaneOptionSurvivesJoinBreak locks the pane-canonical premise of the
// tab-placements design (plan 026) into CI: identity/state stored on pane
// options must survive a join-pane + break-pane round-trip, because window
// options die with their window (verified in spike G). If tmux ever changes
// this, P1's storage model — and the whole P3 placement design — breaks.
func TestPaneOptionSurvivesJoinBreak(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	const sock = "zmux-it-tabstate"
	defer func() { _ = exec.Command("tmux", "-L", sock, "kill-server").Run() }()

	tmuxAt(t, sock, "new-session", "-d", "-s", "it", "-x", "100", "-y", "30")
	mainPane := tmuxAt(t, sock, "display-message", "-p", "-t", "it", "#{pane_id}")
	tmuxAt(t, sock, "new-window", "-t", "it", "-n", "second")
	tabPane := tmuxAt(t, sock, "list-panes", "-t", "it:second", "-F", "#{pane_id}")

	tmuxAt(t, sock, "set-option", "-p", "-t", tabPane, "@zmux_state", "attention")
	tmuxAt(t, sock, "set-option", "-p", "-t", tabPane, "@zmux_tab_id", "buddy-it")

	// join: the tab's pane embeds into the main window; its window dies.
	tmuxAt(t, sock, "join-pane", "-s", tabPane, "-t", mainPane)
	if got := tmuxAt(t, sock, "display-message", "-p", "-t", tabPane, "#{@zmux_state}"); got != "attention" {
		t.Fatalf("pane option lost on join-pane: got %q", got)
	}

	// break: back out into a fresh window — id and options must hold.
	tmuxAt(t, sock, "break-pane", "-s", tabPane)
	if got := tmuxAt(t, sock, "display-message", "-p", "-t", tabPane, "#{@zmux_state}"); got != "attention" {
		t.Fatalf("pane option lost on break-pane: got %q", got)
	}
	if got := tmuxAt(t, sock, "display-message", "-p", "-t", tabPane, "#{@zmux_tab_id}"); got != "buddy-it" {
		t.Fatalf("identity option lost on break-pane: got %q", got)
	}
}
