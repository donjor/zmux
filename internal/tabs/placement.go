package tabs

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/donjor/zmux/internal/tmux"
)

// CloneBlocked reports whether a session sits in a session group with other
// live members and any attached client in the group — the S6 predicate
// (`session_group != "" && session_group_size > 1 && session_group_attached
// > 0`). Placement verbs are blocked then: grouped sessions share one window
// set, so a move/hide would yank the window out from under another viewport.
// Note the group name persists after a clone dies (size>1 is load-bearing),
// and a nested/headless clone shows att=0 but grpatt>0 — gate on the group
// counter, never session_attached.
func CloneBlocked(r tmux.Runner, session string) (bool, error) {
	out, err := r.DisplayMessage(session+":",
		"#{session_group}\t#{session_group_size}\t#{session_group_attached}")
	if err != nil {
		return false, fmt.Errorf("inspect session group: %w", err)
	}
	f := strings.SplitN(strings.TrimSpace(out), "\t", 3)
	if len(f) < 3 || f[0] == "" {
		return false, nil
	}
	size, _ := strconv.Atoi(f[1])
	attached, _ := strconv.Atoi(f[2])
	return size > 1 && attached > 0, nil
}

// EnsureDock lazily creates the marked dock session. Returns the placeholder
// window id to kill after the first move-in when the dock was just created
// (empty otherwise — tmux reaps the dock itself when its last window leaves,
// so an existing dock never carries a placeholder). A pre-existing UNMARKED
// __zmux_dock is not ours: refuse rather than adopt a user's session.
func EnsureDock(r tmux.Runner) (placeholder string, err error) {
	if r.HasSession(DockSession) {
		mark, merr := r.DisplayMessage(DockSession+":", "#{"+OptDockMark+"}")
		if merr != nil {
			return "", fmt.Errorf("inspect dock session: %w", merr)
		}
		if strings.TrimSpace(mark) != "1" {
			return "", fmt.Errorf("session %q exists but is not zmux's dock (no %s mark) — rename or kill it first",
				DockSession, OptDockMark)
		}
		return "", nil
	}
	if err := r.NewSession(DockSession, ""); err != nil {
		return "", fmt.Errorf("create dock session: %w", err)
	}
	if err := r.SetSessionOption(DockSession, OptDockMark, "1"); err != nil {
		return "", fmt.Errorf("mark dock session: %w", err)
	}
	out, derr := r.DisplayMessage(DockSession+":", "#{window_id}")
	if derr != nil {
		return "", nil // placeholder cleanup is best-effort; the dock works without it
	}
	return strings.TrimSpace(out), nil
}

// Hide parks a tab in the dock. Full tabs move their whole window (raw human
// splits ride along); pane-of tabs break just their pane out into a fresh
// dock window (S4 matrix). The origin session lands in @zmux_hidden — one
// option doubling as hidden-flag and `tab show` default target.
func Hide(r tmux.Runner, t *LogicalTab) error {
	if t.Placement == PlacementDock {
		return fmt.Errorf("tab %q is already hidden", DisplayName(t))
	}
	placeholder, err := EnsureDock(r)
	if err != nil {
		return err
	}
	switch t.Placement {
	case PlacementFull:
		if err := r.MoveWindow(t.WindowID, DockSession+":"); err != nil {
			return fmt.Errorf("hide tab %q: %w", DisplayName(t), err)
		}
	case PlacementPaneOf:
		if _, err := r.BreakPane(tmux.BreakPaneOptions{
			Source:   t.PaneID,
			Target:   DockSession + ":",
			Name:     DisplayName(t),
			Detached: true,
		}); err != nil {
			return fmt.Errorf("hide tab %q: %w", DisplayName(t), err)
		}
	case PlacementDock: // unreachable — guarded above
	}
	if placeholder != "" {
		// The fresh dock's placeholder shell dies once a real window is in.
		_ = r.KillWindowByID(placeholder)
	}
	return r.ApplyOptions([]tmux.OptionWrite{
		{Scope: tmux.ScopePane, Target: t.PaneID, Key: OptHidden, Value: t.Session},
	})
}

// Show returns a docked tab to its origin session as a full appended window
// and clears @zmux_hidden. It never auto-joins — rejoining as a pane is the
// `tab pane` verb's job. Returns the session shown into.
func Show(r tmux.Runner, t *LogicalTab) (string, error) {
	if t.Placement != PlacementDock {
		return "", fmt.Errorf("tab %q is not hidden (placement: %s)", DisplayName(t), t.Placement)
	}
	origin := t.OriginSession
	if origin == "" || origin == DockSession {
		return "", fmt.Errorf("tab %q has no recorded origin session", DisplayName(t))
	}
	if !r.HasSession(origin) {
		return "", fmt.Errorf("origin session %q is gone — cannot show tab %q there", origin, DisplayName(t))
	}
	if err := r.MoveWindow(t.WindowID, origin+":"); err != nil {
		return "", fmt.Errorf("show tab %q: %w", DisplayName(t), err)
	}
	err := r.ApplyOptions([]tmux.OptionWrite{
		{Scope: tmux.ScopePane, Target: t.PaneID, Key: OptHidden, Unset: true},
	})
	return origin, err
}

// DisplayName is a tab's addressable display name: label, else its id.
func DisplayName(t *LogicalTab) string {
	if t.Label != "" {
		return t.Label
	}
	return t.ID
}
