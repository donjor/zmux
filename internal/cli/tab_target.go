package cli

import (
	"errors"
	"fmt"
	"os"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/tablabel"
	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tabstate"
	"github.com/donjor/zmux/internal/tmux"
)

// resolvedTab is the result of the tab-targeting choke point: every command
// that addresses a tab by name (watch/send/type/run/state/label/kill/move)
// resolves through here, so logical tabs stay reachable wherever their pane
// physically lives — full window, pane of another tab, or parked in the dock.
type resolvedTab struct {
	// Target is the tmux target input/capture commands address. Logical hits
	// target the pane id, which stays valid across placement moves; legacy
	// window hits target session:index (stable across auto-rename).
	Target string
	// Tab is non-nil when a logical (pane-canonical) tab matched.
	Tab *tabs.LogicalTab
	// Win is non-nil when only the legacy window pass matched.
	Win *tmux.Window

	// state is the resolved state-write destination; stateOK guards it.
	// Populated by the mutation path so the clear-stale + mark-running pair
	// riding every input delivery doesn't re-resolve the target per write.
	state   tabstate.Target
	stateOK bool
}

// found reports whether anything matched — false means the raw session:name
// fallback, where tmux's own error surfaces and create-on-miss callers create.
func (rt resolvedTab) found() bool { return rt.Tab != nil || rt.Win != nil }

// resolveTabTarget maps a tab name to a tmux target, read-only. Resolution
// order (ratified): logical tab by exact id → exact label in session scope →
// legacy window label/name → raw session:name fallback. Only ambiguity
// errors; a missing tab falls through so callers that create-on-miss keep
// working. A scan failure degrades silently to the legacy path.
func resolveTabTarget(app *apppkg.App, session, name string) (resolvedTab, error) {
	if all, err := tabs.ListLogicalTabs(app.Runner); err == nil {
		t, rerr := tabs.Resolve(all, name, session)
		switch {
		case rerr == nil:
			return resolvedTab{
				Target: t.PaneID,
				Tab:    t,
				// Docked tabs mirror onto their dock window — never rendered,
				// rewritten on show; pane-canonical state is what matters.
				state:   tabstate.Target{PaneID: t.PaneID, Window: fmt.Sprintf("%s:%d", t.Session, t.WindowIndex)},
				stateOK: true,
			}, nil
		case !errors.Is(rerr, tabs.ErrNotFound):
			return resolvedTab{}, rerr // ambiguous — never guess a target
		}
	}
	if w := findWindow(app, session, name); w != nil {
		return resolvedTab{Target: fmt.Sprintf("%s:%d", session, w.Index), Win: w}, nil
	}
	return resolvedTab{Target: fmt.Sprintf("%s:%s", session, name)}, nil
}

// resolveTabTargetForMutation is resolveTabTarget for commands that deliver
// input (send/type/run/state). Two extras over the read-only path:
//
//   - claim: a legacy window matched by live name with no label yet is
//     claimed as a logical tab — pane-scoped id + pane-canonical label via
//     tabs.Stamp — so the tab stays addressable as <claimLabel> after tmux
//     auto-renames it and keeps its identity across placement moves. Empty
//     claimLabel skips the claim (run with a name derived from the command —
//     incidental names must not become stable labels).
//   - the state-write destination resolves once here, shared by the
//     clear-stale and mark-running writes that follow.
func resolveTabTargetForMutation(app *apppkg.App, session, name, claimLabel string) (resolvedTab, error) {
	rt, err := resolveTabTarget(app, session, name)
	if err != nil || rt.Win == nil {
		return rt, err
	}
	// One display round-trip resolves the legacy window's active pane for
	// both the state writes and the claim. Best-effort: a dead window just
	// leaves state writes on the spec-resolution fallback.
	if t, terr := tabstate.ResolveTarget(app.Runner, rt.Target, os.Getenv); terr == nil {
		rt.state, rt.stateOK = t, true
		if rt.Win.Label == "" && claimLabel != "" {
			_, _ = tabs.Stamp(app.Runner, t.PaneID, rt.Target, claimLabel, tablabel.SourcePane)
		}
	}
	return rt, nil
}

// stateTarget returns the resolved state destination, falling back to spec
// resolution for raw-fallback targets.
func (rt resolvedTab) stateTarget(svc *tabstate.Service) (tabstate.Target, error) {
	if rt.stateOK {
		return rt.state, nil
	}
	return svc.Resolve(rt.Target)
}

// markState best-effort sets a lifecycle state on the resolved tab. State
// writes piggyback on run/send/type and must never fail the command that
// triggered them — a dead pane or detached server just skips the glyph.
func (rt resolvedTab) markState(app *apppkg.App, st tabstate.State, source, msg string) {
	svc := tabstate.New(app.Runner, os.Getenv)
	t, err := rt.stateTarget(svc)
	if err != nil {
		return
	}
	_ = svc.Set(t, st, source, msg)
}

// clearStale clears done|failed before new input reaches the tab — the
// ratified rule: user input (typing-by-proxy included) acknowledges a
// finished state. attention/running are never cleared here.
func (rt resolvedTab) clearStale(app *apppkg.App) {
	svc := tabstate.New(app.Runner, os.Getenv)
	t, err := rt.stateTarget(svc)
	if err != nil {
		return
	}
	_, _ = svc.ClearIf(t, tabstate.StateDone, tabstate.StateFailed)
}
