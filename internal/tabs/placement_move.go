package tabs

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/donjor/zmux/internal/tmux"
)

// winSnapshot captures a window's layout facts before a join/break disturbs
// it — the S2 order: snapshot → move → restore layout iff pane count still
// matches (select-layout silently no-ops on mismatch, rc is useless) →
// re-zoom if it was zoomed and is still multi-pane.
type winSnapshot struct {
	WindowID   string
	Layout     string
	Zoomed     bool
	Panes      int
	ActivePane string // the pane to re-zoom (a zoomed window's active pane)
}

// snapshotWindow reads a window's pre-move layout facts. A window target
// resolves pane_id to the window's ACTIVE pane — exactly the one tmux had
// zoomed, if any.
func snapshotWindow(r tmux.Runner, windowID string) (winSnapshot, error) {
	out, err := r.DisplayMessage(windowID,
		"#{window_layout}\t#{window_zoomed_flag}\t#{window_panes}\t#{pane_id}")
	if err != nil {
		return winSnapshot{}, fmt.Errorf("snapshot window %s: %w", windowID, err)
	}
	f := strings.SplitN(strings.TrimSpace(out), "\t", 4)
	if len(f) < 4 {
		return winSnapshot{}, fmt.Errorf("snapshot window %s: malformed reply %q", windowID, out)
	}
	panes, _ := strconv.Atoi(f[2])
	return winSnapshot{
		WindowID:   windowID,
		Layout:     f[0],
		Zoomed:     f[1] == "1",
		Panes:      panes,
		ActivePane: f[3],
	}, nil
}

// restoreWindow best-efforts the S2 restore on a snapshotted window. The
// move already happened — failures here degrade to warnings, never errors.
// movedPane is the pane the verb relocated: it must never be re-zoomed in
// its OLD window (it isn't there anymore).
func restoreWindow(r tmux.Runner, snap winSnapshot, movedPane string) []string {
	out, err := r.DisplayMessage(snap.WindowID, "#{window_panes}")
	if err != nil {
		return nil // window dissolved with its last pane — nothing to restore
	}
	panes, _ := strconv.Atoi(strings.TrimSpace(out))
	var warnings []string
	if panes == snap.Panes && snap.Layout != "" {
		if err := r.SelectLayout(snap.WindowID, snap.Layout); err != nil {
			warnings = append(warnings, fmt.Sprintf("layout restore failed on %s: %v", snap.WindowID, err))
		}
	}
	if snap.Zoomed && panes > 1 && snap.ActivePane != movedPane {
		if err := r.ToggleZoom(snap.ActivePane); err != nil {
			warnings = append(warnings, fmt.Sprintf("re-zoom failed on %s: %v", snap.ActivePane, err))
		}
	}
	return warnings
}

// JoinOptions shape the split a joined tab lands in.
type JoinOptions struct {
	Direction tmux.SplitDirection // placement relative to the host pane
	Size      string              // tmux -l value such as "40%" or "80"
}

// Join relocates tab t into the host tab's window as a sibling pane —
// `tab pane`. Works from every placement: a full tab's window dissolves
// behind it (raw splits stay behind as an unmanaged window), a docked tab
// leaves the dock (hidden flag cleared), a rider just changes host. The
// advisory anchor records the host so windowOwner stays deterministic.
// Returned warnings are non-fatal restore failures (S2).
func Join(r tmux.Runner, t, host *LogicalTab, opts JoinOptions) ([]string, error) {
	if t.ID == host.ID {
		return nil, fmt.Errorf("tab %q cannot join itself", DisplayName(t))
	}
	if host.Placement == PlacementDock {
		return nil, fmt.Errorf("host tab %q is hidden — show it first (zmux tab show %s)",
			DisplayName(host), DisplayName(host))
	}
	if t.WindowID == host.WindowID {
		return nil, fmt.Errorf("tab %q already shares a window with %q", DisplayName(t), DisplayName(host))
	}

	hostSnap, err := snapshotWindow(r, host.WindowID)
	if err != nil {
		return nil, err
	}
	// The source window only needs restoring when the tab leaves siblings
	// behind (multi-pane); a single-pane window dissolves with the move.
	var srcSnap *winSnapshot
	if t.WindowPanes > 1 {
		if snap, serr := snapshotWindow(r, t.WindowID); serr == nil {
			srcSnap = &snap
		}
	}

	if err := r.JoinPane(tmux.JoinPaneOptions{
		Source:    t.PaneID,
		Target:    host.PaneID,
		Direction: opts.Direction,
		Size:      opts.Size,
		Detached:  true, // never steal focus — the join is visible regardless
	}); err != nil {
		return nil, fmt.Errorf("join tab %q into %q: %w", DisplayName(t), DisplayName(host), err)
	}

	writes := []tmux.OptionWrite{
		{Scope: tmux.ScopePane, Target: t.PaneID, Key: OptAnchor, Value: host.ID},
	}
	if t.Placement == PlacementDock {
		writes = append(writes, tmux.OptionWrite{
			Scope: tmux.ScopePane, Target: t.PaneID, Key: OptHidden, Unset: true,
		})
	}
	if err := r.ApplyOptions(writes); err != nil {
		return nil, fmt.Errorf("record anchor on %q: %w", DisplayName(t), err)
	}

	// Zoom restore on the host is intentionally skipped: the join ADDED a
	// pane the user asked to see — re-zooming the old pane would hide it.
	warnings := restoreLayoutOnly(r, hostSnap)
	if srcSnap != nil {
		warnings = append(warnings, restoreWindow(r, *srcSnap, t.PaneID)...)
	}
	return warnings, nil
}

// restoreLayoutOnly applies just the layout half of the S2 restore — for
// windows where re-zooming would hide the pane the verb just placed.
func restoreLayoutOnly(r tmux.Runner, snap winSnapshot) []string {
	zoomless := snap
	zoomless.Zoomed = false
	return restoreWindow(r, zoomless, "")
}

// Promote breaks a pane-of tab out into its own full window, or returns a
// hidden pane as a full window in its origin session. S3 settled: append by
// default; after=true inserts visible panes directly after the old host window
// (break-pane -a) — indexes are never persisted. Returns the new window id and
// non-fatal restore warnings.
func Promote(r tmux.Runner, t *LogicalTab, after bool) (string, []string, error) {
	switch t.Placement {
	case PlacementFull:
		return "", nil, fmt.Errorf("tab %q is already a full tab", DisplayName(t))
	case PlacementDock:
		origin := t.OriginSession
		if origin == "" || origin == DockSession {
			return "", nil, fmt.Errorf("tab %q has no recorded origin session", DisplayName(t))
		}
		if !r.HasSession(origin) {
			return "", nil, fmt.Errorf("origin session %q is gone — cannot promote tab %q there", origin, DisplayName(t))
		}
		if err := r.MoveWindow(t.WindowID, origin+":"); err != nil {
			return "", nil, fmt.Errorf("promote hidden tab %q: %w", DisplayName(t), err)
		}
		if err := r.ApplyOptions([]tmux.OptionWrite{
			{Scope: tmux.ScopePane, Target: t.PaneID, Key: OptHidden, Unset: true},
			{Scope: tmux.ScopePane, Target: t.PaneID, Key: OptAnchor, Unset: true},
		}); err != nil {
			return "", nil, fmt.Errorf("clear hidden state on %q: %w", DisplayName(t), err)
		}
		return t.WindowID, nil, nil
	case PlacementPaneOf: // visible pane promotable path below
	}

	hostSnap, err := snapshotWindow(r, t.WindowID)
	if err != nil {
		return "", nil, err
	}

	opts := tmux.BreakPaneOptions{
		Source:   t.PaneID,
		Target:   t.Session + ":", // bare colon appends in the same session
		Name:     DisplayName(t),
		Detached: true,
	}
	if after {
		opts.Target = t.WindowID // -a inserts directly after the old host
		opts.After = true
	}
	windowID, err := r.BreakPane(opts)
	if err != nil {
		return "", nil, fmt.Errorf("promote tab %q: %w", DisplayName(t), err)
	}

	// Full tabs carry no anchor — leaving the old host's id behind would be
	// drift the reconciler has to chase.
	if err := r.ApplyOptions([]tmux.OptionWrite{
		{Scope: tmux.ScopePane, Target: t.PaneID, Key: OptAnchor, Unset: true},
	}); err != nil {
		return "", nil, fmt.Errorf("clear anchor on %q: %w", DisplayName(t), err)
	}

	return strings.TrimSpace(windowID), restoreWindow(r, hostSnap, t.PaneID), nil
}
