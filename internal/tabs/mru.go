package tabs

import (
	"strings"

	"github.com/donjor/zmux/internal/tmux"
)

// OptMRU is the session option holding the logical-tab MRU list:
// space-separated tab ids, most recent first. Session-option storage keeps
// recency local to the tmux server with no file locking; ids are pruned
// against live tabs, never trusted blindly.
const OptMRU = "@zmux_tab_mru"

// mruCap bounds the list; ids past it fall off rather than growing the
// option without limit.
const mruCap = 32

// ReadMRU returns the session's MRU tab ids, most recent first. The merged
// format read is safe here: OptMRU is only ever set at session scope.
func ReadMRU(r tmux.Runner, session string) []string {
	out, err := r.DisplayMessage(session+":", "#{"+OptMRU+"}")
	if err != nil || strings.TrimSpace(out) == "" {
		return nil
	}
	return strings.Fields(out)
}

// TouchMRU moves id to the front of the session's MRU list.
func TouchMRU(r tmux.Runner, session, id string) error {
	if id == "" {
		return nil
	}
	ids := []string{id}
	for _, prev := range ReadMRU(r, session) {
		if prev != id {
			ids = append(ids, prev)
		}
	}
	if len(ids) > mruCap {
		ids = ids[:mruCap]
	}
	return r.SetSessionOption(session, OptMRU, strings.Join(ids, " "))
}

// PruneMRU drops ids the live predicate rejects (dead tabs), rewriting the
// option only when something actually fell out.
func PruneMRU(r tmux.Runner, session string, live func(id string) bool) error {
	ids := ReadMRU(r, session)
	kept := ids[:0]
	for _, id := range ids {
		if live(id) {
			kept = append(kept, id)
		}
	}
	if len(kept) == len(ids) {
		return nil
	}
	return r.SetSessionOption(session, OptMRU, strings.Join(kept, " "))
}
