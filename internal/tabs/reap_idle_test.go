package tabs

import (
	"testing"

	"github.com/donjor/zmux/internal/tmux"
)

// T-106/T-404 (055, B-01, reap side). PaneIsLive treats six shells as "idle at
// a prompt"; every other command is a live foreground process. Empty/unknown
// command is deliberately LIVE (false negatives over false kills). T-404/S-008
// folded both consumers onto the one exported IsIdleShell predicate, so the bar
// renderers now recognize the same six shells (locked from the bar side in
// internal/bar/idle_shell_test.go).
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
		// PaneIsLive is exactly the negation of the shared predicate.
		if got := IsIdleShell(c.cmd); got == c.wantLive {
			t.Errorf("IsIdleShell(%q) = %v, want %v", c.cmd, got, !c.wantLive)
		}
	}
}
