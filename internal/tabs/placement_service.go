package tabs

import (
	"fmt"
	"strings"

	"github.com/donjor/zmux/internal/debug"
	"github.com/donjor/zmux/internal/tmux"
)

// Guarded placement operations + current-context resolution, shared by the CLI
// placement verbs (internal/cli) and the command palette so the two surfaces run
// the same clone-block guard, epilogue (reconcile + status repaint), and
// "current pane/window → logical tab" resolution instead of duplicating them.
//
// The raw moves (Hide/Show/Join/Promote) stay separate; these wrap them.

// HideTab parks t in the hidden dock after the clone-block guard, then heals.
func HideTab(r tmux.Runner, t *LogicalTab) error {
	if err := guardClones(r, t.Session); err != nil {
		return err
	}
	if err := Hide(r, t); err != nil {
		return err
	}
	placementEpilogue(r)
	return nil
}

// ShowTab returns a docked tab to its origin session, guarding the origin's
// clones, then heals. Returns the origin session name.
func ShowTab(r tmux.Runner, t *LogicalTab) (string, error) {
	if t.Placement == PlacementDock {
		if err := guardClones(r, t.OriginSession); err != nil {
			return "", err
		}
	}
	origin, err := Show(r, t)
	if err != nil {
		return "", err
	}
	placementEpilogue(r)
	return origin, nil
}

// JoinTab relocates t to live as a pane beside host, guarding both sessions'
// clones, then heals. Returns any non-fatal warnings.
func JoinTab(r tmux.Runner, t, host *LogicalTab, opts JoinOptions) ([]string, error) {
	if err := guardClones(r, t.Session); err != nil {
		return nil, err
	}
	if host.Session != t.Session {
		if err := guardClones(r, host.Session); err != nil {
			return nil, err
		}
	}
	warnings, err := Join(r, t, host, opts)
	if err != nil {
		return nil, err
	}
	placementEpilogue(r)
	return warnings, nil
}

// PromoteTab breaks t out into its own full window, guarding its session's
// clones, then heals. Returns the new window id and any non-fatal warnings.
func PromoteTab(r tmux.Runner, t *LogicalTab, after bool) (string, []string, error) {
	if err := guardClones(r, t.Session); err != nil {
		return "", nil, err
	}
	windowID, warnings, err := Promote(r, t, after)
	if err != nil {
		return "", nil, err
	}
	placementEpilogue(r)
	return windowID, warnings, nil
}

// CurrentTab resolves the caller's active pane to its logical tab. Reconciles
// first (best-effort) so a freshly-claimed tab is visible.
func CurrentTab(r tmux.Runner) (*LogicalTab, error) {
	if !r.IsInsideTmux() {
		return nil, fmt.Errorf("not inside tmux — pass a tab name")
	}
	if _, err := Reconcile(r); err != nil {
		debug.Log("placement: reconcile failed", "err", err)
	}
	paneID, err := currentValue(r, "#{pane_id}")
	if err != nil {
		return nil, fmt.Errorf("resolve current pane: %w", err)
	}
	all, err := ListLogicalTabs(r)
	if err != nil {
		return nil, fmt.Errorf("scan tabs: %w", err)
	}
	for i := range all {
		if all[i].PaneID == paneID {
			return &all[i], nil
		}
	}
	return nil, fmt.Errorf("current pane is not a zmux tab — pass a tab name")
}

// CurrentHost resolves the caller's active pane to the logical tab under the
// cursor — the default join destination.
func CurrentHost(r tmux.Runner) (*LogicalTab, error) {
	if !r.IsInsideTmux() {
		return nil, fmt.Errorf("not inside tmux — name the host tab")
	}
	all, err := ListLogicalTabs(r)
	if err != nil {
		return nil, fmt.Errorf("scan tabs: %w", err)
	}
	return CurrentHostFrom(all, r)
}

// CurrentHostFrom resolves the join host from an already-scanned tab list.
//
// Invariant: a logical tab is pane-canonical. Placement only says where that
// pane currently lives: full window, pane-of another tab, or dock. Therefore the
// "current tab" is the focused pane's logical tab, and the default join host is
// that same tab. When the focused pane is a full owner this preserves the old
// anchor; when it is a joined pane, a bare join lands beside the pane under the
// cursor. This helper deliberately reuses all instead of rescanning, so palette
// rows and their current-host exclusion come from one coherent snapshot.
func CurrentHostFrom(all []LogicalTab, r tmux.Runner) (*LogicalTab, error) {
	if !r.IsInsideTmux() {
		return nil, fmt.Errorf("not inside tmux — name the host tab")
	}
	paneID, err := currentValue(r, "#{pane_id}")
	if err != nil {
		return nil, fmt.Errorf("resolve current pane: %w", err)
	}
	for i := range all {
		if all[i].PaneID == paneID {
			return &all[i], nil
		}
	}
	return nil, fmt.Errorf("current pane is not a zmux tab")
}

// currentValue reads a tmux format against the caller's current client.
func currentValue(r tmux.Runner, format string) (string, error) {
	out, err := r.DisplayMessage("", format)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// guardClones enforces the clone predicate before a move: grouped sessions share
// one window set, so moving a tab would reshape another attached viewport.
func guardClones(r tmux.Runner, session string) error {
	blocked, err := CloneBlocked(r, session)
	if err != nil {
		return err
	}
	if blocked {
		return fmt.Errorf("session %q has attached grouped viewports — detach the other clients (or close their viewports) before moving tabs", session)
	}
	return nil
}

// placementEpilogue heals state after a move and repaints the bar. Best-effort:
// the move already happened; a failed repair just waits for the next reconcile.
func placementEpilogue(r tmux.Runner) {
	if _, err := Reconcile(r); err != nil {
		debug.Log("placement: post-move reconcile failed", "err", err)
	}
	if err := r.RefreshStatus(); err != nil {
		debug.Log("placement: status refresh failed", "err", err)
	}
}
