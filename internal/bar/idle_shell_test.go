package bar

import (
	"testing"
)

// T-106/T-404 (055, B-01, bar side). All three bar render sites (renderLeftAux
// for Hacker, renderLeftHacker, renderRightStarship) now suppress the process
// token for the full idle-shell set via tabs.IsIdleShell — the same predicate
// the reaper uses through PaneIsLive. T-404 (S-008) folded both consumers onto
// that one predicate; these assertions lock the unified behavior, where the
// only change from the prior {bash,zsh,fish}-only bar check is that sh/dash/ksh
// are now correctly treated as idle prompts (suppressed) in every bar too.

// barShowsProc reports whether a render site emits the process token for cmd,
// isolating the proc field by diffing against an empty-command render (every
// other context field is held constant).
func barShowsProc(render func(cmd string) string, cmd string) bool {
	return render(cmd) != render("")
}

func TestBarProcSuppressionDrift(t *testing.T) {
	p := testPalette()

	sites := map[string]func(string) string{
		"leftAux-hacker": func(cmd string) string {
			c := baseCtx()
			c.TopRowActive = true
			c.PaneCmd = cmd
			return RenderLeft(p, c, Hacker)
		},
		"leftHacker": func(cmd string) string {
			c := baseCtx()
			c.TopRowActive = false
			c.PaneCmd = cmd
			return RenderLeft(p, c, Hacker)
		},
		"rightStarship": func(cmd string) string {
			c := baseCtx()
			c.PaneCmd = cmd
			return RenderRight(p, c, Starship)
		},
	}

	cases := []struct {
		cmd       string
		wantShown bool // bar renders it as an active process
	}{
		{"bash", false},
		{"zsh", false},
		{"fish", false},
		{"sh", false},   // unified: idle prompt, suppressed like the others
		{"dash", false}, // unified
		{"ksh", false},  // unified
		{"nvim", true},  // genuine live process
	}

	for name, render := range sites {
		for _, c := range cases {
			if got := barShowsProc(render, c.cmd); got != c.wantShown {
				t.Errorf("%s: proc %q shown=%v, want %v", name, c.cmd, got, c.wantShown)
			}
		}
	}
}
