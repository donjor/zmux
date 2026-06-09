package bar

import (
	"strings"
	"testing"
	"time"

	"github.com/donjor/zmux/internal/tablabel"
	"github.com/donjor/zmux/internal/tabstate"
)

func TestWindowFormatsCarryStateGlyphsEveryPreset(t *testing.T) {
	p := testPalette()

	for _, preset := range AllPresets() {
		t.Run(preset.String(), func(t *testing.T) {
			opts := Generate(preset, p)

			for _, key := range []string{"window-status-format", "window-status-current-format"} {
				opt, ok := findOpt(opts, key)
				if !ok {
					t.Fatalf("missing %q", key)
				}
				// state conditional present, attention outermost
				if !strings.Contains(opt.Value, "#{?#{==:#{"+tabstate.OptState+"},attention}") {
					t.Errorf("%s missing state fragment: %q", key, opt.Value)
				}
				// every state renders its glyph in its semantic color
				for st, glyph := range map[tabstate.State]string{
					tabstate.StateAttention: p.Accent.Hex(),
					tabstate.StateFailed:    p.Error.Hex(),
					tabstate.StateRunning:   p.Info.Hex(),
					tabstate.StateDone:      p.Success.Hex(),
				} {
					if !strings.Contains(opt.Value, "#[fg="+glyph+"]"+stateGlyphs[st]) {
						t.Errorf("%s missing %s glyph/color: %q", key, st, opt.Value)
					}
				}
				// label overlay must survive the state pass
				if !strings.Contains(opt.Value, "#{?"+tablabel.Option+",") {
					t.Errorf("%s lost the tab label overlay: %q", key, opt.Value)
				}
				// placement: the glyph lives INSIDE the pill, after the name
				// (label expression), not prefixed to the whole format where
				// it would float on the bar background before the pill cap.
				stateIdx := strings.Index(opt.Value, "#{?#{==:#{"+tabstate.OptState+"}")
				labelIdx := strings.Index(opt.Value, "#{?"+tablabel.Option+",")
				if stateIdx < labelIdx {
					t.Errorf("%s state glyph precedes the name — must render inside the pill after it: %q", key, opt.Value)
				}
			}
		})
	}
}

func TestTabStateFragmentEmptyWhenUnset(t *testing.T) {
	frag := tabStateFragment(testPalette(), "")
	// nested conditional falls through to empty — a stateless tab renders
	// exactly nothing extra (the innermost alternative is the empty string)
	if !strings.HasSuffix(frag, ",}}}}") {
		t.Fatalf("fragment must end in empty fall-through: %q", frag)
	}
}

func TestRunningGlyphAnimatesViaSpinnerJobWithBinary(t *testing.T) {
	p := testPalette()

	// with a binary: running renders the #() spinner job, not the static ◐
	frag := tabStateFragment(p, "/usr/bin/zmux")
	if !strings.Contains(frag, "#(/usr/bin/zmux bar-spinner)") {
		t.Fatalf("running glyph must be the bar-spinner job: %q", frag)
	}
	// other states stay static even with a binary
	for _, st := range []tabstate.State{tabstate.StateAttention, tabstate.StateDone, tabstate.StateFailed} {
		if !strings.Contains(frag, stateGlyphs[st]) {
			t.Errorf("%s lost its static glyph: %q", st, frag)
		}
	}

	// without a binary: static fallback, no dangling job reference
	frag = tabStateFragment(p, "")
	if strings.Contains(frag, "bar-spinner") {
		t.Fatalf("binary-less fragment must not reference the spinner job: %q", frag)
	}
	if !strings.Contains(frag, stateGlyphs[tabstate.StateRunning]) {
		t.Fatalf("binary-less running glyph must fall back to static: %q", frag)
	}
}

func TestSpinnerFrameCyclesPerSecond(t *testing.T) {
	base := time.Unix(1780654436, 0)
	seen := map[string]bool{}
	for i := range 4 {
		f := SpinnerFrame(base.Add(time.Duration(i) * time.Second))
		if seen[f] {
			t.Fatalf("frame %q repeated within one cycle", f)
		}
		seen[f] = true
	}
	// wraps: second 4 repeats second 0
	if SpinnerFrame(base) != SpinnerFrame(base.Add(4*time.Second)) {
		t.Fatal("spinner must cycle with period 4")
	}
}
