package cli

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/debug"
	"github.com/donjor/zmux/internal/tablabel"
	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/spf13/cobra"
)

func newTabHideCmd(app *apppkg.App) *cobra.Command {
	var sessionFlag string
	var paneFlag string
	var notifyFlag bool

	cmd := &cobra.Command{
		Use:   "hide [tab]",
		Short: "Park a pane-tab under its parent (keeps running, out of layout)",
		Long: `Hide a tab that lives as a pane inside another tab. The pane breaks out
into the reserved dock session, keeps running, stays addressable by pane id, and
is rendered as a parked badge under its recorded parent. Bring it back with
zmux tab show.

Full tabs are top-level workspace units and are intentionally not hideable;
join one as a pane first if it should become collapsible. With no tab argument,
the focused pane-tab is hidden.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			msg, err := func() (string, error) {
				t, err := resolveHideTab(app, sessionFlag, paneFlag, args)
				if err != nil {
					return "", err
				}
				if err := tabs.HideTab(app.Runner, t); err != nil {
					return "", err
				}
				name := tabs.DisplayName(t)
				return fmt.Sprintf("hidden: %s (show with: zmux tab show %s)", name, name), nil
			}()
			return notifyOutcome(app, cmd, notifyFlag, msg, nil, err)
		},
	}
	cmd.Flags().StringVarP(&sessionFlag, "session", "s", "", "session for tab-name targets (default: current)")
	cmd.Flags().StringVar(&paneFlag, "pane", "", "target pane id (mouse/menu path)")
	cmd.Flags().BoolVar(&notifyFlag, "notify", false, "flash the outcome as a transient tmux message and exit 0 (mouse/menu path)")
	_ = cmd.Flags().MarkHidden("pane")
	return cmd
}

func resolveHideTab(app *apppkg.App, sessionFlag, paneFlag string, args []string) (*tabs.LogicalTab, error) {
	if paneFlag != "" {
		if len(args) > 0 {
			return nil, fmt.Errorf("--pane cannot be combined with a tab argument")
		}
		return logicalTabByPane(app.Runner, paneFlag)
	}
	if len(args) == 0 {
		return tabs.CurrentTab(app.Runner)
	}
	session, err := placementSession(app, sessionFlag)
	if err != nil {
		return nil, err
	}
	return resolvePlacementTab(app, session, args[0], false)
}

func newTabShowCmd(app *apppkg.App) *cobra.Command {
	var sessionFlag string
	var paneFlag string
	var notifyFlag bool
	var focusFlag bool

	cmd := &cobra.Command{
		Use:   "show [tab]",
		Short: "Rejoin a parked pane-tab to its recorded parent",
		Long: `Show a hidden pane-tab: it rejoins the visible parent it was parked
under and clears the hidden marker. <tab> may be a name/label or the 1-based
parked index shown beside the current parent in the tab row. Promote a parked
pane to a full tab explicitly with zmux tab full.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var warnings []string
			msg, err := func() (string, error) {
				t, err := resolveShowTab(app, sessionFlag, paneFlag, args)
				if err != nil {
					return "", err
				}
				origin, err := tabs.ShowTab(app.Runner, t)
				if err != nil {
					return "", err
				}
				if focusFlag {
					warnings = appendFocusWarning(warnings, focusLogicalTabByID(app, t.ID))
				}
				return fmt.Sprintf("shown: %s → %s", tabs.DisplayName(t), origin), nil
			}()
			return notifyOutcome(app, cmd, notifyFlag, msg, warnings, err)
		},
	}
	cmd.Flags().StringVarP(&sessionFlag, "session", "s", "", "session for tab-name targets (default: current)")
	cmd.Flags().StringVar(&paneFlag, "pane", "", "target pane id (mouse/menu path)")
	cmd.Flags().BoolVar(&notifyFlag, "notify", false, "flash the outcome as a transient tmux message and exit 0 (mouse/menu path)")
	cmd.Flags().BoolVar(&focusFlag, "focus", false, "select the shown pane after rejoining it (human UI path)")
	_ = cmd.Flags().MarkHidden("pane")
	return cmd
}

func resolveShowTab(app *apppkg.App, sessionFlag, paneFlag string, args []string) (*tabs.LogicalTab, error) {
	if paneFlag != "" {
		if len(args) > 0 {
			return nil, fmt.Errorf("--pane cannot be combined with a tab argument")
		}
		return logicalTabByPane(app.Runner, paneFlag)
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("tab name required")
	}
	session, err := placementSession(app, sessionFlag)
	if err != nil {
		return nil, err
	}
	name := args[0]
	if n, ok := tabIndexArg(name); ok {
		return hiddenTabAtIndex(app.Runner, session, n)
	}
	return resolvePlacementTab(app, session, name, false)
}

func hiddenTabAtIndex(r tmux.Runner, session string, index int) (*tabs.LogicalTab, error) {
	all, err := tabs.ListLogicalTabs(r)
	if err != nil {
		return nil, fmt.Errorf("scan tabs: %w", err)
	}
	current, err := currentTabFromList(r, all)
	if err != nil {
		return nil, fmt.Errorf("hidden indexes are parent-scoped; %w", err)
	}
	anchors := []string{current.ID}
	if current.AnchorID != "" {
		anchors = append(anchors, current.AnchorID)
	}
	for _, anchor := range anchors {
		hidden := hiddenTabsForAnchor(all, session, anchor)
		if len(hidden) == 0 {
			continue
		}
		if index < 1 || index > len(hidden) {
			return nil, fmt.Errorf("no hidden pane at index %d under %s", index, tabs.DisplayName(current))
		}
		return &hidden[index-1], nil
	}
	return nil, fmt.Errorf("no hidden panes under %s", tabs.DisplayName(current))
}

func currentTabFromList(r tmux.Runner, all []tabs.LogicalTab) (*tabs.LogicalTab, error) {
	if !r.IsInsideTmux() {
		return nil, fmt.Errorf("run inside tmux or use a hidden pane name")
	}
	paneID, err := r.DisplayMessage("", "#{pane_id}")
	if err != nil {
		return nil, fmt.Errorf("resolve current pane: %w", err)
	}
	paneID = strings.TrimSpace(paneID)
	for i := range all {
		if all[i].PaneID == paneID {
			return &all[i], nil
		}
	}
	return nil, fmt.Errorf("current pane is not a zmux tab")
}

func hiddenTabsForAnchor(all []tabs.LogicalTab, session, anchor string) []tabs.LogicalTab {
	var hidden []tabs.LogicalTab
	for i := range all {
		if all[i].Placement == tabs.PlacementDock && all[i].OriginSession == session && all[i].AnchorID == anchor {
			hidden = append(hidden, all[i])
		}
	}
	return hidden
}

func newTabPaneCmd(app *apppkg.App) *cobra.Command {
	var sessionFlag string
	var intoFlag string
	var sizeFlag string
	var notifyFlag bool
	var focusFlag bool
	var dirRight, dirLeft, dirUp, dirDown bool

	cmd := &cobra.Command{
		Use:   "pane <tab>",
		Short: "Join a tab into another tab's window as a pane",
		Long: `Relocate a tab to live as a pane beside another tab (default: the tab
under your cursor). The tab keeps its id, label, and state — send/type/watch
still reach it by name. Promote it back out with zmux tab full.

<tab> is a name/label or a 1-based tab index matching the bar's numbered cells
(a tab literally labeled with that number still wins). Direction flags place it
relative to the host pane (default: --right).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var warnings []string
			msg, err := func() (string, error) {
				dir, err := paneDirection(dirRight, dirLeft, dirUp, dirDown)
				if err != nil {
					return "", err
				}
				session, err := placementSession(app, sessionFlag)
				if err != nil {
					return "", err
				}
				t, err := resolvePlacementTab(app, session, args[0], true)
				if err != nil {
					return "", err
				}
				host, err := resolvePaneHost(app, session, intoFlag)
				if err != nil {
					return "", err
				}
				warnings, err = tabs.JoinTab(app.Runner, t, host, tabs.JoinOptions{
					Direction: dir,
					Size:      sizeFlag,
				})
				if err != nil {
					return "", err
				}
				if focusFlag {
					warnings = appendFocusWarning(warnings, focusLogicalTabByID(app, t.ID))
				}
				return fmt.Sprintf("pane: %s → beside %s (%s)",
					tabs.DisplayName(t), tabs.DisplayName(host), host.Session), nil
			}()
			return notifyOutcome(app, cmd, notifyFlag, msg, warnings, err)
		},
	}
	cmd.Flags().StringVarP(&sessionFlag, "session", "s", "", "session for tab-name targets (default: current)")
	cmd.Flags().StringVar(&intoFlag, "into", "", "host tab to join (default: the tab under your cursor)")
	cmd.Flags().StringVar(&sizeFlag, "size", "", "pane size, e.g. 40% or 80")
	cmd.Flags().BoolVar(&notifyFlag, "notify", false, "flash the outcome as a transient tmux message and exit 0 (keybind path)")
	cmd.Flags().BoolVar(&focusFlag, "focus", false, "select the joined pane after moving it (human UI path)")
	cmd.Flags().BoolVar(&dirRight, "right", false, "place right of the host pane (default)")
	cmd.Flags().BoolVar(&dirLeft, "left", false, "place left of the host pane")
	cmd.Flags().BoolVar(&dirUp, "up", false, "place above the host pane")
	cmd.Flags().BoolVar(&dirDown, "down", false, "place below the host pane")
	return cmd
}

func newTabSplitCmd(app *apppkg.App) *cobra.Command {
	var sizeFlag string
	var notifyFlag bool
	var focusFlag bool
	var dirRight, dirLeft, dirUp, dirDown bool

	cmd := &cobra.Command{
		Use:   "split",
		Short: "Create a new tab as a pane beside the current one",
		Long: `Create a managed tab in the current pane's cwd and immediately
join it beside the tab under your cursor. This is the first-class one-key
tab-to-pane path; use zmux tab pane <tab> when the tab already exists.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			var warnings []string
			msg, err := func() (string, error) {
				dir, err := paneDirection(dirRight, dirLeft, dirUp, dirDown)
				if err != nil {
					return "", err
				}
				created, host, w, err := createJoinedTab(app, dir, sizeFlag)
				warnings = w
				if err != nil {
					return "", err
				}
				if focusFlag {
					warnings = appendFocusWarning(warnings, focusLogicalTabByID(app, created.ID))
				}
				return fmt.Sprintf("split: %s → beside %s (%s)",
					tabs.DisplayName(created), tabs.DisplayName(host), host.Session), nil
			}()
			return notifyOutcome(app, cmd, notifyFlag, msg, warnings, err)
		},
	}
	cmd.Flags().StringVar(&sizeFlag, "size", "", "pane size, e.g. 40% or 80")
	cmd.Flags().BoolVar(&notifyFlag, "notify", false, "flash the outcome as a transient tmux message and exit 0 (keybind path)")
	cmd.Flags().BoolVar(&focusFlag, "focus", false, "select the created pane after joining it (human UI path)")
	cmd.Flags().BoolVar(&dirRight, "right", false, "place right of the host pane (default)")
	cmd.Flags().BoolVar(&dirLeft, "left", false, "place left of the host pane")
	cmd.Flags().BoolVar(&dirUp, "up", false, "place above the host pane")
	cmd.Flags().BoolVar(&dirDown, "down", false, "place below the host pane")
	return cmd
}

func createJoinedTab(app *apppkg.App, dir tmux.SplitDirection, size string) (*tabs.LogicalTab, *tabs.LogicalTab, []string, error) {
	// The order is load-bearing: NewWindow without -d steals focus, so resolve
	// the host before creating the tab and always create detached.
	host, err := tabs.CurrentHost(app.Runner)
	if err != nil {
		return nil, nil, nil, err
	}
	cwd, err := currentPaneCWD(app)
	if err != nil {
		return nil, nil, nil, err
	}
	paneID, err := app.Runner.NewWindow(host.Session, "", cwd, tmux.Detached())
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create tab: %w", err)
	}
	if paneID == "" {
		return nil, nil, nil, fmt.Errorf("create tab: tmux did not report the new pane id")
	}
	if _, err := tabs.Stamp(app.Runner, paneID, paneID, "", tablabel.SourcePane); err != nil {
		return nil, nil, nil, fmt.Errorf("stamp tab: %w", err)
	}
	created, err := logicalTabByPane(app.Runner, paneID)
	if err != nil {
		return nil, nil, nil, err
	}
	warnings, err := tabs.JoinTab(app.Runner, created, host, tabs.JoinOptions{
		Direction: dir,
		Size:      size,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	return created, host, warnings, nil
}

func currentPaneCWD(app *apppkg.App) (string, error) {
	cwd, err := app.Runner.DisplayMessage("", "#{pane_current_path}")
	if err != nil {
		return "", fmt.Errorf("resolve current pane cwd: %w", err)
	}
	cwd = strings.TrimSpace(cwd)
	if cwd != "" {
		return cwd, nil
	}
	if fallback, ferr := os.Getwd(); ferr == nil {
		return fallback, nil
	}
	return "", fmt.Errorf("resolve current pane cwd: empty pane_current_path")
}

func logicalTabByPane(r tmux.Runner, paneID string) (*tabs.LogicalTab, error) {
	all, err := tabs.ListLogicalTabs(r)
	if err != nil {
		return nil, fmt.Errorf("scan tabs: %w", err)
	}
	for i := range all {
		if all[i].PaneID == paneID {
			return &all[i], nil
		}
	}
	return nil, fmt.Errorf("pane %s is not a zmux tab", paneID)
}

func newTabFullCmd(app *apppkg.App) *cobra.Command {
	var sessionFlag string
	var paneFlag string
	var afterFlag bool
	var notifyFlag bool

	cmd := &cobra.Command{
		Use:   "full [tab]",
		Short: "Promote a visible or hidden pane-tab to a full window",
		Long: `Break a tab living as a pane inside another tab out into its own full
window, appended after the session's existing tabs (S3: indexes are never
persisted). Hidden panes are returned to their origin session as full tabs.
With no tab argument, the focused pane-tab is promoted. --after inserts a
visible pane directly after its old host window instead.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var warnings []string
			msg, err := func() (string, error) {
				session, err := placementSession(app, sessionFlag)
				if err != nil {
					return "", err
				}
				var t *tabs.LogicalTab
				if paneFlag != "" {
					if len(args) > 0 {
						return "", fmt.Errorf("--pane cannot be combined with a tab argument")
					}
					t, err = logicalTabByPane(app.Runner, paneFlag)
					if err != nil {
						return "", err
					}
				} else if len(args) == 0 {
					t, err = tabs.CurrentTab(app.Runner)
					if err != nil {
						return "", err
					}
				} else {
					t, err = resolvePlacementTab(app, session, args[0], false)
					if err != nil {
						return "", err
					}
				}
				windowID, w, err := tabs.PromoteTab(app.Runner, t, afterFlag)
				if err != nil {
					return "", err
				}
				warnings = w
				return fmt.Sprintf("full: %s → %s (%s)",
					tabs.DisplayName(t), t.Session, windowID), nil
			}()
			return notifyOutcome(app, cmd, notifyFlag, msg, warnings, err)
		},
	}
	cmd.Flags().StringVarP(&sessionFlag, "session", "s", "", "session for tab-name targets (default: current)")
	cmd.Flags().StringVar(&paneFlag, "pane", "", "target pane id (mouse/menu path)")
	cmd.Flags().BoolVar(&afterFlag, "after", false, "insert directly after the old host window instead of appending")
	cmd.Flags().BoolVar(&notifyFlag, "notify", false, "flash the outcome as a transient tmux message and exit 0 (keybind path)")
	_ = cmd.Flags().MarkHidden("pane")
	return cmd
}

func focusLogicalTabByID(app *apppkg.App, tabID string) error {
	all, err := tabs.ListLogicalTabs(app.Runner)
	if err != nil {
		return fmt.Errorf("focus pane: scan tabs: %w", err)
	}
	t := tabs.ByID(all, tabID)
	if t == nil {
		return fmt.Errorf("focus pane: tab no longer exists")
	}
	if t.Placement == tabs.PlacementDock {
		return fmt.Errorf("focus pane: tab %q is hidden", tabs.DisplayName(t))
	}
	if t.Session != "" {
		if err := app.Runner.SelectWindow(t.Session, t.WindowIndex); err != nil {
			return fmt.Errorf("focus pane: select window: %w", err)
		}
	}
	if t.PaneID == "" {
		return fmt.Errorf("focus pane: tab %q has no pane id", tabs.DisplayName(t))
	}
	if err := app.Runner.SelectPane(t.PaneID); err != nil {
		return fmt.Errorf("focus pane: select pane: %w", err)
	}
	return nil
}

func appendFocusWarning(warnings []string, err error) []string {
	if err != nil {
		return append(warnings, err.Error())
	}
	return warnings
}

// notifyOutcome reports a placement command's result. On the keybind path
// (--notify, used by the prefix+J/prefix+F run-shell bindings) it flashes a
// transient tmux status message and swallows the error, so the binding never
// triggers tmux's view-mode takeover — a non-zero exit shows "<cmd> returned N"
// and stdout shows a sticky dump, both needing a keypress to clear. On the
// direct CLI path it prints success to stdout and returns the error with its
// exit code intact.
func notifyOutcome(app *apppkg.App, cmd *cobra.Command, notify bool, success string, warnings []string, err error) error {
	if notify {
		msg := success
		switch {
		case err != nil:
			msg = err.Error()
		case len(warnings) > 0:
			// Non-fatal warnings would otherwise vanish on the keybind path —
			// fold them into the single flash so the signal survives.
			msg += " — warning: " + strings.Join(warnings, "; ")
		}
		if msg != "" {
			if derr := app.Runner.ShowMessage(msg); derr != nil {
				debug.Log("placement: notify failed", "err", derr)
			}
		}
		return nil
	}
	for _, w := range warnings {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", w)
	}
	if err != nil {
		return err
	}
	if success != "" {
		fmt.Fprintln(cmd.OutOrStdout(), success)
	}
	return nil
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
// the caller's current pane mapped to its logical tab.
func resolvePaneHost(app *apppkg.App, session, into string) (*tabs.LogicalTab, error) {
	if into != "" {
		// Host is name-only: index addressing is for the tab being joined, not
		// the destination (which already defaults to the cursor tab).
		return resolvePlacementTab(app, session, into, false)
	}
	host, err := tabs.CurrentHost(app.Runner)
	if err != nil {
		return nil, fmt.Errorf("%w (use --into to name the host)", err)
	}
	return host, nil
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

// tabIndexArg reports whether name is a bare 1-based index (pure digits, ≥ 1).
// strconv.Atoi alone is too lenient ("+2", " 2" parse), so the digit check
// guards numeric-label precedence — only a truly numeric arg can shadow a tab
// labeled with that number.
func tabIndexArg(name string) (int, bool) {
	if name == "" {
		return 0, false
	}
	for _, r := range name {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(name)
	if err != nil || n < 1 {
		return 0, false
	}
	return n, true
}

// resolvePlacementTab resolves a name to a MANAGED logical tab for placement
// verbs. Reconcile runs first (heals drift, migrates legacy window-labeled
// tabs to pane identity); an unlabeled live-name match gets claimed by the
// mutation choke point, then picked up on a re-scan. Raw fallback targets
// are an error — placement needs a logical tab to move. allowIndex enables the
// opt-in join-by-index path (only `tab pane` passes it).
func resolvePlacementTab(app *apppkg.App, session, name string, allowIndex bool) (*tabs.LogicalTab, error) {
	if _, err := tabs.Reconcile(app.Runner); err != nil {
		debug.Log("placement: reconcile failed", "err", err)
	}
	// Opt-in, placement-only join-by-index. A bare 1-based index addresses the
	// Nth full tab, matching the numbered bar cells. Handled on its own
	// read-only path BEFORE the mutation/claim resolver: routing a numeric name
	// through resolveTabTargetForMutation would stamp the label "2" onto a raw
	// window. id/label still wins first, so a tab literally named "2" beats
	// index 2 (numeric-label precedence).
	if n, ok := tabIndexArg(name); ok && allowIndex {
		all, err := tabs.ListLogicalTabs(app.Runner)
		if err != nil {
			return nil, fmt.Errorf("scan tabs: %w", err)
		}
		switch t, rerr := tabs.Resolve(all, name, session); {
		case rerr == nil:
			// A tab literally labeled/ided this number wins — but ONLY in scope.
			// tabs.Resolve has a unique-server-wide convenience fallback; for
			// index addressing that would let `tab pane 2` in session A grab a
			// tab labeled "2" in session B. Refuse the cross-session match and
			// fall through to this session's index N (codex diff review).
			if t.InScope(session) {
				return t, nil
			}
		case !errors.Is(rerr, tabs.ErrNotFound):
			return nil, rerr // ambiguous numeric label — never guess
		}
		if t := tabs.TabAtIndex(all, session, n); t != nil {
			return t, nil
		}
		return nil, fmt.Errorf("no tab at index %d in %s", n, session)
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
