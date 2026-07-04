package bar

import (
	"fmt"
	"strings"
	"time"

	"github.com/donjor/zmux/internal/tabstate"
	"github.com/donjor/zmux/internal/theme"
)

// stateGlyphs maps tab lifecycle states to bar glyphs. BMP-only, written as
// \u escapes — plane-15 icons break grep/Edit tooling (bar glyph gotchas).
var stateGlyphs = map[tabstate.State]string{
	tabstate.StateAttention: "●", // ● needs the human
	tabstate.StateFailed:    "✗", // ✗ errored
	tabstate.StateRunning:   "◐", // ◐ in flight (static fallback; see SpinnerFrame)
	tabstate.StateReady:     "↩", // ↩ answer ready / user's move
	tabstate.StateDone:      "✓", // ✓ plain command finished, unacknowledged
}

// spinnerFrames step the running glyph once per second. tmux 3.4 has format
// arithmetic but no ticking now-variable (window_activity freezes on quiet
// panes), so the frame is picked by a #(bar-spinner) status job — tmux
// re-runs it every status-interval and shares one cached job across all
// window cells (identical command string = one cache key).
var spinnerFrames = [4]string{"◐", "◓", "◑", "◒"}

// SpinnerFrame returns the running-state glyph for a moment in time.
func SpinnerFrame(now time.Time) string {
	return spinnerFrames[now.Unix()%int64(len(spinnerFrames))]
}

// StateGlyph returns the lifecycle glyph for a state ("" when unknown) —
// the one glyph vocabulary shared by the bar and TUI surfaces (tabpicker).
func StateGlyph(st tabstate.State) string {
	return stateGlyphs[st]
}

// stateColor maps states to semantic palette roles (purpose, not color).
func stateColor(p *theme.Palette, st tabstate.State) string {
	switch st {
	case tabstate.StateAttention:
		return p.Accent.Hex()
	case tabstate.StateFailed:
		return p.Error.Hex()
	case tabstate.StateRunning:
		return p.Info.Hex()
	case tabstate.StateReady:
		return p.Info.Hex()
	case tabstate.StateDone:
		return p.Success.Hex()
	}
	return p.Dim.Hex()
}

// tabStateFragment renders the @zmux_state window mirror as a colored glyph
// suffix (` ◐`); empty when no state is set, so stateless tabs are untouched.
// No fg restore after the glyph: every preset follows the name with its own
// style directive (cap/separator/reset), same assumption the label overlay
// already leans on for its dim `[#W]` bracket.
// With a zmux binary available the running glyph animates via the
// bar-spinner status job; without one (tests, binary-less generation) it
// stays the static ◐.
func tabStateFragment(p *theme.Palette, zmuxBin string) string {
	return tabstate.StatusFragment(func(st tabstate.State) string {
		glyph := stateGlyphs[st]
		if st == tabstate.StateRunning && zmuxBin != "" {
			glyph = fmt.Sprintf("#(%s bar-spinner)", zmuxBin)
		}
		return fmt.Sprintf(" #[fg=%s]%s", stateColor(p, st), glyph)
	})
}

// withTabStateFormats injects the state glyph inside the tab pill, right
// after the window name, so it inherits the pill background instead of
// floating on the bar before the pill's cap. Same `#W` substitution seam as
// tab labels — and it must run BEFORE withTabLabelFormats: the label
// expression expands `#W` into a conditional containing several `#W`
// tokens, which would each sprout a glyph.
func withTabStateFormats(opts []TmuxOption, palette *theme.Palette, zmuxBin string) []TmuxOption {
	frag := tabStateFragment(palette, zmuxBin)
	for i := range opts {
		switch opts[i].Key {
		case "window-status-format", "window-status-current-format":
			opts[i].Value = strings.ReplaceAll(opts[i].Value, "#W", "#W"+frag)
		}
	}
	return opts
}
