package tabstate

import (
	"errors"
	"fmt"
	"strings"

	"github.com/donjor/zmux/internal/tmux"
)

// ErrNoTarget means there is nothing to act on: no explicit target and the
// caller is not inside a tmux pane. Hook-mode (--quiet) callers treat it as
// a silent no-op — hooks outside tmux must never produce noisy failures.
var ErrNoTarget = errors.New("no target pane (no target given and not inside a tmux pane)")

// Target is a resolved state-write destination: the canonical pane plus the
// window carrying the presentation mirror.
type Target struct {
	PaneID string // %N — canonical write target
	Window string // session:window_index — mirror target
}

// ResolveTarget resolves a raw spec to a Target. Empty spec falls back to
// the caller's own pane via $TMUX_PANE, then — still inside tmux ($TMUX
// set) — to the client's current pane (empty display-message target, the
// same idiom bar_render uses). Window/session specs resolve to that
// window's active pane (one display-message resolves both halves).
// Label-aware tab-name resolution is the CLI layer's job — by the time a
// spec reaches here it is a tmux target.
func ResolveTarget(r tmux.Runner, spec string, getenv func(string) string) (Target, error) {
	raw := spec
	if raw == "" {
		raw = getenv("TMUX_PANE")
	}
	if raw == "" && getenv("TMUX") == "" {
		return Target{}, ErrNoTarget
	}
	out, err := r.DisplayMessage(raw, "#{pane_id}\t#{session_name}:#{window_index}")
	if err != nil {
		return Target{}, fmt.Errorf("resolve target %q: %w", raw, err)
	}
	parts := strings.SplitN(strings.TrimSpace(out), "\t", 2)
	if len(parts) != 2 || parts[0] == "" {
		return Target{}, fmt.Errorf("resolve target %q: unexpected display output %q", raw, out)
	}
	return Target{PaneID: parts[0], Window: parts[1]}, nil
}
