package tabs

import (
	"testing"

	"github.com/donjor/zmux/internal/tmux"
)

// T-106 (055 P-001) — reap idle-shell predicate (B-01, reap side). PaneIsLive
// treats six shells as "idle at a prompt"; every other command is a live
// foreground process. Empty/unknown command is deliberately LIVE (false
// negatives over false kills). Pinned before T-404/S-008 folds both consumers
// onto one exported predicate. NOTE the current drift: the bar renderers
// recognize only {bash,zsh,fish} as idle, so sh/dash/ksh diverge — locked from
// the bar side in internal/bar/idle_shell_test.go.
func TestPaneIsLiveIdleShells(t *testing.T) {
	cases := []struct {
		cmd      string
		wantLive bool
	}{
		{"bash", false},
		{"zsh", false},
		{"fish", false},
		{"sh", false},
		{"dash", false},
		{"ksh", false},
		{"", true}, // unknown/empty → live (safe default)
		{"nvim", true},
		{"python", true},
		{"claude", true},
	}
	for _, c := range cases {
		if got := PaneIsLive(tmux.LogicalPaneRow{Command: c.cmd}); got != c.wantLive {
			t.Errorf("PaneIsLive(%q) = %v, want %v", c.cmd, got, c.wantLive)
		}
	}
}
