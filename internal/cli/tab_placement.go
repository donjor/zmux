package cli

import (
	"fmt"
	"strings"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/debug"
	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/spf13/cobra"
)

func newTabHideCmd(app *apppkg.App) *cobra.Command {
	var sessionFlag string

	cmd := &cobra.Command{
		Use:   "hide <tab>",
		Short: "Park a tab in the hidden dock (keeps running, off the bar)",
		Long: `Hide a tab in the reserved dock session. The process keeps running and
stays addressable — send/type/watch/run -n reach it by name or id — it just
leaves the bar and the window list. Bring it back with zmux tab show.

A full tab moves its whole window; a tab living as a pane inside another tab
breaks out into the dock on its own.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			session, err := placementSession(app, sessionFlag)
			if err != nil {
				return err
			}
			t, err := resolvePlacementTab(app, session, args[0])
			if err != nil {
				return err
			}
			if err := blockOnAttachedClones(app, t.Session); err != nil {
				return err
			}
			if err := tabs.Hide(app.Runner, t); err != nil {
				return err
			}
			placementEpilogue(app)
			name := tabs.DisplayName(t)
			fmt.Fprintf(cmd.OutOrStdout(), "hidden: %s (show with: zmux tab show %s)\n", name, name)
			return nil
		},
	}
	cmd.Flags().StringVarP(&sessionFlag, "session", "s", "", "session for tab-name targets (default: current)")
	return cmd
}

func newTabShowCmd(app *apppkg.App) *cobra.Command {
	var sessionFlag string

	cmd := &cobra.Command{
		Use:   "show <tab>",
		Short: "Return a hidden tab from the dock to its origin session",
		Long: `Show a hidden tab: its window moves back to the session it was hidden
from (recorded at hide time) as a full tab, appended after the existing tabs.
It never auto-joins into another tab — use zmux tab pane for that.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			session, err := placementSession(app, sessionFlag)
			if err != nil {
				return err
			}
			t, err := resolvePlacementTab(app, session, args[0])
			if err != nil {
				return err
			}
			if t.Placement == tabs.PlacementDock {
				if err := blockOnAttachedClones(app, t.OriginSession); err != nil {
					return err
				}
			}
			origin, err := tabs.Show(app.Runner, t)
			if err != nil {
				return err
			}
			placementEpilogue(app)
			fmt.Fprintf(cmd.OutOrStdout(), "shown: %s → %s\n", tabs.DisplayName(t), origin)
			return nil
		},
	}
	cmd.Flags().StringVarP(&sessionFlag, "session", "s", "", "session for tab-name targets (default: current)")
	return cmd
}

func newTabPaneCmd(app *apppkg.App) *cobra.Command {
	var sessionFlag string
	var intoFlag string
	var sizeFlag string
	var dirRight, dirLeft, dirUp, dirDown bool

	cmd := &cobra.Command{
		Use:   "pane <tab>",
		Short: "Join a tab into another tab's window as a pane",
		Long: `Relocate a tab to live as a pane beside another tab (default: the tab
under your cursor). The tab keeps its id, label, and state — send/type/watch
still reach it by name. Promote it back out with zmux tab full.

Direction flags place it relative to the host pane (default: --right).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := paneDirection(dirRight, dirLeft, dirUp, dirDown)
			if err != nil {
				return err
			}
			session, err := placementSession(app, sessionFlag)
			if err != nil {
				return err
			}
			t, err := resolvePlacementTab(app, session, args[0])
			if err != nil {
				return err
			}
			host, err := resolvePaneHost(app, session, intoFlag)
			if err != nil {
				return err
			}
			if err := blockOnAttachedClones(app, t.Session); err != nil {
				return err
			}
			if host.Session != t.Session {
				if err := blockOnAttachedClones(app, host.Session); err != nil {
					return err
				}
			}
			warnings, err := tabs.Join(app.Runner, t, host, tabs.JoinOptions{
				Direction: dir,
				Size:      sizeFlag,
			})
			if err != nil {
				return err
			}
			placementEpilogue(app)
			for _, w := range warnings {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", w)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "pane: %s → beside %s (%s)\n",
				tabs.DisplayName(t), tabs.DisplayName(host), host.Session)
			return nil
		},
	}
	cmd.Flags().StringVarP(&sessionFlag, "session", "s", "", "session for tab-name targets (default: current)")
	cmd.Flags().StringVar(&intoFlag, "into", "", "host tab to join (default: the tab under your cursor)")
	cmd.Flags().StringVar(&sizeFlag, "size", "", "pane size, e.g. 40% or 80")
	cmd.Flags().BoolVar(&dirRight, "right", false, "place right of the host pane (default)")
	cmd.Flags().BoolVar(&dirLeft, "left", false, "place left of the host pane")
	cmd.Flags().BoolVar(&dirUp, "up", false, "place above the host pane")
	cmd.Flags().BoolVar(&dirDown, "down", false, "place below the host pane")
	return cmd
}

func newTabFullCmd(app *apppkg.App) *cobra.Command {
	var sessionFlag string
	var afterFlag bool

	cmd := &cobra.Command{
		Use:   "full [tab]",
		Short: "Promote a pane-of tab back to a full window",
		Long: `Break a tab living as a pane inside another tab out into its own full
window, appended after the session's existing tabs (S3: indexes are never
persisted). With no tab argument, the focused pane-tab is promoted. --after
inserts it directly after its old host window instead.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			session, err := placementSession(app, sessionFlag)
			if err != nil {
				return err
			}
			var t *tabs.LogicalTab
			if len(args) == 0 {
				t, err = resolveCurrentPlacementTab(app)
				if err != nil {
					return err
				}
			} else {
				t, err = resolvePlacementTab(app, session, args[0])
				if err != nil {
					return err
				}
			}
			if err := blockOnAttachedClones(app, t.Session); err != nil {
				return err
			}
			windowID, warnings, err := tabs.Promote(app.Runner, t, afterFlag)
			if err != nil {
				return err
			}
			placementEpilogue(app)
			for _, w := range warnings {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", w)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "full: %s → %s (%s)\n",
				tabs.DisplayName(t), t.Session, windowID)
			return nil
		},
	}
	cmd.Flags().StringVarP(&sessionFlag, "session", "s", "", "session for tab-name targets (default: current)")
	cmd.Flags().BoolVar(&afterFlag, "after", false, "insert directly after the old host window instead of appending")
	return cmd
}

func resolveCurrentPlacementTab(app *apppkg.App) (*tabs.LogicalTab, error) {
	if !app.Runner.IsInsideTmux() {
		return nil, fmt.Errorf("not inside tmux — pass a tab name")
	}
	if _, err := tabs.Reconcile(app.Runner); err != nil {
		debug.Log("placement: reconcile failed", "err", err)
	}
	out, err := app.Runner.DisplayMessage("", "#{pane_id}")
	if err != nil {
		return nil, fmt.Errorf("resolve current pane: %w", err)
	}
	paneID := strings.TrimSpace(out)
	all, err := tabs.ListLogicalTabs(app.Runner)
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

// paneDirection folds the four direction flags into one split direction —
// at most one may be set; none means right (peer-beside-work default).
func paneDirection(right, left, up, down bool) (tmux.SplitDirection, error) {
	var dir tmux.SplitDirection
	var n int
	for _, c := range []struct {
		set bool
		d   tmux.SplitDirection
	}{
		{right, tmux.SplitRight},
		{left, tmux.SplitLeft},
		{up, tmux.SplitUp},
		{down, tmux.SplitDown},
	} {
		if c.set {
			dir = c.d
			n++
		}
	}
	if n > 1 {
		return "", fmt.Errorf("pick one direction flag (--right/--left/--up/--down)")
	}
	if n == 0 {
		dir = tmux.SplitRight
	}
	return dir, nil
}

// resolvePaneHost resolves the join destination: an explicit --into tab, or
// the caller's current window mapped to its owning logical tab.
func resolvePaneHost(app *apppkg.App, session, into string) (*tabs.LogicalTab, error) {
	if into != "" {
		return resolvePlacementTab(app, session, into)
	}
	if !app.Runner.IsInsideTmux() {
		return nil, fmt.Errorf("not inside tmux — use --into to name the host tab")
	}
	out, err := app.Runner.DisplayMessage("", "#{window_id}")
	if err != nil {
		return nil, fmt.Errorf("resolve current window: %w", err)
	}
	windowID := strings.TrimSpace(out)
	all, err := tabs.ListLogicalTabs(app.Runner)
	if err != nil {
		return nil, fmt.Errorf("scan tabs: %w", err)
	}
	for i := range all {
		h := &all[i]
		if h.WindowID == windowID && h.Placement == tabs.PlacementFull {
			return h, nil
		}
	}
	return nil, fmt.Errorf("current window is not a zmux tab — use --into to name the host")
}

// placementSession resolves the session scope for placement verbs: explicit
// flag, else the caller's current session.
func placementSession(app *apppkg.App, flag string) (string, error) {
	if flag != "" {
		return flag, nil
	}
	if !app.Runner.IsInsideTmux() {
		return "", fmt.Errorf("not inside tmux — use --session to specify target")
	}
	name, err := app.Runner.DisplayMessage("", "#{session_name}")
	if err != nil {
		return "", fmt.Errorf("not inside a tmux session")
	}
	return strings.TrimSpace(name), nil
}

// resolvePlacementTab resolves a name to a MANAGED logical tab for placement
// verbs. Reconcile runs first (heals drift, migrates legacy window-labeled
// tabs to pane identity); an unlabeled live-name match gets claimed by the
// mutation choke point, then picked up on a re-scan. Raw fallback targets
// are an error — placement needs a logical tab to move.
func resolvePlacementTab(app *apppkg.App, session, name string) (*tabs.LogicalTab, error) {
	if _, err := tabs.Reconcile(app.Runner); err != nil {
		debug.Log("placement: reconcile failed", "err", err)
	}
	rt, err := resolveTabTargetForMutation(app, session, name, name)
	if err != nil {
		return nil, err
	}
	if rt.Tab != nil {
		return rt.Tab, nil
	}
	if rt.Win != nil {
		// The window was just claimed (or carries a label the scan missed) —
		// one re-scan picks the fresh tab up.
		if all, lerr := tabs.ListLogicalTabs(app.Runner); lerr == nil {
			if t, rerr := tabs.Resolve(all, name, session); rerr == nil {
				return t, nil
			}
		}
	}
	return nil, fmt.Errorf("tab %q is not a zmux tab in %s", name, session)
}

// blockOnAttachedClones enforces the S6 clone predicate before any placement
// move: grouped sessions share one window set, so moving a tab would reshape
// another attached viewport underneath its user.
func blockOnAttachedClones(app *apppkg.App, session string) error {
	blocked, err := tabs.CloneBlocked(app.Runner, session)
	if err != nil {
		return err
	}
	if blocked {
		return fmt.Errorf("session %q has attached grouped viewports — detach the other clients (or close their viewports) before moving tabs", session)
	}
	return nil
}

// placementEpilogue heals state after a placement move and repaints the bar.
// Best-effort: the move already happened; a failed repair just waits for the
// next reconcile.
func placementEpilogue(app *apppkg.App) {
	if _, err := tabs.Reconcile(app.Runner); err != nil {
		debug.Log("placement: post-move reconcile failed", "err", err)
	}
	if err := app.Runner.RefreshStatus(); err != nil {
		debug.Log("placement: status refresh failed", "err", err)
	}
}
