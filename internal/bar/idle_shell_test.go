package bar

import (
	"testing"
)

// T-106 (055 P-001) — idle-shell predicate drift characterization (B-01, bar
// side). All three bar render sites (renderLeftAux for Hacker, renderLeftHacker,
// renderRightStarship) suppress the process token for ONLY {bash,zsh,fish},
// whereas internal/tabs.PaneIsLive treats a wider set — {..,sh,dash,ksh} — as
// idle. This pins the CURRENT drift: sh/dash/ksh render as live processes in
// every bar even though reap considers them idle prompts. T-404 (S-008) folds
// both consumers onto one predicate; these assertions lock the drifted behavior
// so the unification is provably a change only for sh/dash/ksh.

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
		{"sh", true},   // DRIFT: reap treats as idle, bars show it
		{"dash", true}, // DRIFT
		{"ksh", true},  // DRIFT
		{"nvim", true}, // genuine live process
	}

	for name, render := range sites {
		for _, c := range cases {
			if got := barShowsProc(render, c.cmd); got != c.wantShown {
				t.Errorf("%s: proc %q shown=%v, want %v", name, c.cmd, got, c.wantShown)
			}
		}
	}
}
