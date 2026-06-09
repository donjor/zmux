package tabstate

import "fmt"

// StatusFragment builds a nested tmux format conditional over the window
// mirror (#{@zmux_state}): render(st) is emitted verbatim for the matching
// state, empty string when no state is set. Callers (the bar) own glyphs and
// styling; this owns only the conditional structure, so the format stays in
// one place if option names ever change.
func StatusFragment(render func(State) string) string {
	out := ""
	for i := len(All) - 1; i >= 0; i-- {
		st := All[i]
		out = fmt.Sprintf("#{?#{==:#{%s},%s},%s,%s}", OptState, st, render(st), out)
	}
	return out
}
