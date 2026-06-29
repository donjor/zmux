package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/tablabel"
	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/workspace"
	"github.com/spf13/cobra"
)

func newTabCmd(app *apppkg.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tab",
		Short: "Manage tabs within sessions",
	}
	cmd.AddCommand(newTabMoveCmd(app))
	cmd.AddCommand(newTabLabelCmd(app))
	cmd.AddCommand(newTabRefreshNamesCmd(app))
	cmd.AddCommand(newTabKillCmd(app))
	cmd.AddCommand(newTabStateCmd(app))
	cmd.AddCommand(newTabStateExitCmd(app))
	cmd.AddCommand(newTabHideCmd(app))
	cmd.AddCommand(newTabShowCmd(app))
	cmd.AddCommand(newTabPaneCmd(app))
	cmd.AddCommand(newTabSplitCmd(app))
	cmd.AddCommand(newTabFullCmd(app))
	cmd.AddCommand(newTabMarkAgentCmd(app))
	cmd.AddCommand(newTabAdoptCmd(app))
	return cmd
}

// newTabAdoptCmd stamps a single newly-linked window as a managed logical tab.
// Driven by the window-linked tmux hook so a window born interactively
// (prefix+c / new-window) becomes addressable — index/name resolvers and
// `tab pane` joins skip panes with no @zmux_tab_id, so without this an
// interactive window can't be joined ("no tab at index N"). Hidden — it's hook
// wiring, the symmetric completion of the first-window stamp in session.Create.
func newTabAdoptCmd(app *apppkg.App) *cobra.Command {
	return &cobra.Command{
		Use:    "adopt <window>",
		Short:  "Stamp a newly-created single-pane window as a managed tab",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return adoptWindow(app, strings.TrimSpace(args[0]))
		},
	}
}

// adoptWindow claims ONE window's pane as a logical tab. Scoped to a single
// window on purpose: a session-wide scan would let any new-window event
// retroactively adopt older raw windows the user never asked to manage, which
// would breach the 027 floor ("raw panes never auto-adopted"). Stamping only
// the just-created window is a pure stamp-at-create, precedented by the
// first-window stamp in session.Create.
//
// Best-effort throughout: it runs from a background hook, so a vanished window,
// dead pane, or any tmux hiccup returns nil rather than surfacing noise — the
// next window event re-attempts. Guards mirror Reconcile's unambiguous-claim
// rule: non-reserved session, exactly one pane, pane not already managed. The
// session is read off the pane (not passed in) so a clone/dock window is
// recognised without trusting a hook-format arg to expand.
func adoptWindow(app *apppkg.App, window string) error {
	if window == "" {
		return nil
	}
	panes, err := app.Runner.ListWindowPanes(window)
	if err != nil || len(panes) != 1 {
		return nil // gone mid-hook, or a raw split — only single-pane windows claim
	}
	if tabs.IsReservedSession(panes[0].Session) {
		return nil // dock/__zmux_ sessions are parking, never presented
	}
	paneID := panes[0].ID
	if paneID == "" {
		return nil
	}
	if id, err := app.Runner.ShowPaneOption(paneID, tabs.OptTabID); err != nil || id != "" {
		return nil // unreadable, or already a managed tab — idempotent no-op
	}
	// A legacy window-scoped @zmux_label migrates onto the pane (canonical
	// identity); MigrateWindowLabel no-ops on an empty label, so a fresh
	// window falls through to a display-neutral id-only stamp (empty label ⇒
	// the bar keeps using the live window name).
	if id, err := tabs.MigrateWindowLabel(app.Runner, window, paneID); err != nil || id != "" {
		return nil
	}
	_, _ = tabs.Stamp(app.Runner, paneID, paneID, "", "")
	return nil
}

// newTabMarkAgentCmd tags the current pane as an agent's home shell
// (origin=agent, scope=agent-shell). Driven by the zmux skill's session-start
// hook so agent-spawned tabs inherit origin=agent (short TTL) without a per-run
// env flag; the shell itself becomes a keep-scope. Hidden — it's wiring, not a
// daily verb. Fails open when not in a pane.
func newTabMarkAgentCmd(app *apppkg.App) *cobra.Command {
	return &cobra.Command{
		Use:    "mark-agent",
		Short:  "Tag the current tab as an agent shell (origin=agent, scope=agent-shell)",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			pane := os.Getenv("TMUX_PANE")
			if pane == "" {
				return nil // not in a pane — nothing to mark
			}
			return tabs.MarkAgentShell(app.Runner, pane, time.Now())
		},
	}
}

func newTabMoveCmd(app *apppkg.App) *cobra.Command {
	var moveForce bool
	cmd := &cobra.Command{
		Use:   "move <tab-name> <dest-session>",
		Short: "Move a tab to another session",
		Long: `Move a full tab to another session.

The destination must be in the same workspace as the source by default — moving
a tab across workspaces mixes project contexts and is usually a mistake. Pass
-f/--force to move it anyway (e.g. to recover a peer tab spawned in the wrong
session).`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			tabName := args[0]
			destInput := args[1]

			// Get current session.
			current, err := app.Runner.DisplayMessage("", "#{session_name}")
			if err != nil {
				return fmt.Errorf("not inside tmux")
			}
			current = strings.TrimSpace(current)

			// Resolve label-aware: logical tab → legacy window → raw name.
			rt, err := resolveTabTarget(app, current, tabName)
			if err != nil {
				return err
			}

			// Only full tabs move as windows; a pane-of or docked tab has no
			// window of its own to move.
			srcSession := current
			src := current + ":" + tabName // raw fallback — tmux errors if missing
			switch {
			case rt.Tab != nil && rt.Tab.Placement == tabs.PlacementFull:
				srcSession = rt.Tab.Session
				src = rt.Tab.WindowID
			case rt.Tab != nil:
				return fmt.Errorf("tab %q is %s — only full tabs move between sessions", tabName, rt.Tab.Placement)
			case rt.Win != nil:
				src = fmt.Sprintf("%s:%d", current, rt.Win.Index)
			}

			destSession, err := resolveSessionTarget(app, destInput)
			if err != nil {
				return fmt.Errorf("move tab %q (session %q) -> %q: %w", tabName, srcSession, destInput, err)
			}

			// Cross-workspace moves mix project contexts; gate behind -f. The
			// refusal names both ends so the operator sees exactly what was
			// resolved and why it stopped (the recovery path for a peer tab that
			// landed in the wrong session).
			srcWS, srcOK := app.WorkspaceStore.WorkspaceFor(srcSession)
			dstWS, dstOK := app.WorkspaceStore.WorkspaceFor(destSession)
			if srcOK && dstOK && srcWS != dstWS {
				if !moveForce {
					return fmt.Errorf("refused to move tab %q across workspaces: source session %q is in %q, destination %q is in %q — pass -f/--force to move anyway", tabName, srcSession, srcWS, destSession, dstWS)
				}
				fmt.Fprintf(os.Stderr, "moving tab %q across workspaces (%s → %s)\n", tabName, srcWS, dstWS)
			}

			// Check this isn't the source session's last tab.
			windows, err := app.Runner.ListWindows(srcSession)
			if err != nil {
				return fmt.Errorf("cannot list tabs: %w", err)
			}
			if len(windows) <= 1 {
				return fmt.Errorf("cannot move the last tab — use `zmux session kill %s` instead", srcSession)
			}

			if err := app.Runner.MoveWindow(src, destSession+":"); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "moved tab %q: %s → %s\n", tabName, srcSession, destSession)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&moveForce, "force", "f", false, "move even if the destination session is in another workspace")
	return cmd
}

func newTabLabelCmd(app *apppkg.App) *cobra.Command {
	var tabLabelTarget string
	var tabLabelClear bool

	cmd := &cobra.Command{
		Use:   "label [label]",
		Short: "Set or clear a stable zmux label for the current tab",
		Long:  "Set a stable zmux label overlay for the current tab. The tmux auto-name remains visible as label [auto]. Pass an empty label or --clear to clear.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			label := ""
			if len(args) > 0 {
				label = args[0]
			}
			return setTabLabel(app, cmd, tabLabelTarget, label, tabLabelClear)
		},
	}
	cmd.Flags().StringVar(&tabLabelTarget, "target", "", "target tmux window (defaults to current)")
	cmd.Flags().BoolVar(&tabLabelClear, "clear", false, "clear the tab label")
	return cmd
}

func newTabRefreshNamesCmd(app *apppkg.App) *cobra.Command {
	return &cobra.Command{
		Use:    "refresh-names [session]",
		Short:  "Refresh duplicate tab-name markers",
		Hidden: true,
		Args:   cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sessionName := ""
			if len(args) > 0 {
				sessionName = strings.TrimSpace(args[0])
			}
			return refreshDuplicateWindowNameMarkers(app, sessionName)
		},
	}
}

func newTabKillCmd(app *apppkg.App) *cobra.Command {
	var paneFlag string
	var notifyFlag bool

	cmd := &cobra.Command{
		Use:   "kill [tab-name]",
		Short: "Kill a tab in the current session",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			msg, err := func() (string, error) {
				if paneFlag != "" {
					if len(args) > 0 {
						return "", fmt.Errorf("--pane cannot be combined with a tab argument")
					}
					t, err := logicalTabByPane(app.Runner, paneFlag)
					if err != nil {
						return "", err
					}
					return killLogicalTab(app, t)
				}
				if len(args) == 0 {
					return "", fmt.Errorf("tab name required")
				}
				return killNamedTab(app, args[0])
			}()
			return notifyOutcome(app, cmd, notifyFlag, msg, nil, err)
		},
	}
	cmd.Flags().StringVar(&paneFlag, "pane", "", "target pane id (mouse/menu path)")
	cmd.Flags().BoolVar(&notifyFlag, "notify", false, "flash the outcome as a transient tmux message and exit 0 (mouse/menu path)")
	_ = cmd.Flags().MarkHidden("pane")
	return cmd
}

func killNamedTab(app *apppkg.App, tabName string) (string, error) {
	current, err := app.Runner.DisplayMessage("", "#{session_name}")
	if err != nil {
		return "", fmt.Errorf("not inside tmux")
	}
	current = strings.TrimSpace(current)

	// kill mutates (destroys a tab) — never reach across sessions by a
	// bare name (report 039). Cross-session kill must be explicit.
	rt, err := resolveTabTargetScoped(app, current, tabName, scopeSessionOnly)
	if err != nil {
		return "", err
	}
	if rt.Tab != nil {
		return killLogicalTab(app, rt.Tab)
	}

	// Legacy/raw window fallback kills by window index. Guard: killing the last
	// window kills the session in tmux — use `zmux session kill` instead for
	// proper workspace cleanup.
	if rt.Win != nil {
		if err := guardNotLastTab(app.Runner, current); err != nil {
			return "", err
		}
		if err := app.Runner.KillWindow(current, rt.Win.Index); err != nil {
			return "", err
		}
		return fmt.Sprintf("killed: %s", tabName), nil
	}
	return "", fmt.Errorf("tab %q not found in session %q", tabName, current)
}

func killLogicalTab(app *apppkg.App, t *tabs.LogicalTab) (string, error) {
	// Pane-of and docked tabs kill as panes — the host window (or dock)
	// survives, minus this tab. tmux reaps a dock window when its last pane dies,
	// and the dock session when its last window does.
	if t.Placement != tabs.PlacementFull {
		if err := app.Runner.KillPane(t.PaneID); err != nil {
			return "", err
		}
		return fmt.Sprintf("killed: %s", tabs.DisplayName(t)), nil
	}

	// Full tabs kill their window. Guard against killing a session's last window.
	if err := guardNotLastTab(app.Runner, t.Session); err != nil {
		return "", err
	}
	if err := app.Runner.KillWindowByID(t.WindowID); err != nil {
		return "", err
	}
	return fmt.Sprintf("killed: %s", tabs.DisplayName(t)), nil
}

func guardNotLastTab(runner tmux.Runner, sessionName string) error {
	windows, err := runner.ListWindows(sessionName)
	if err != nil {
		return fmt.Errorf("cannot list tabs: %w", err)
	}
	if len(windows) <= 1 {
		return fmt.Errorf("cannot kill the last tab — use `zmux session kill %s` instead", sessionName)
	}
	return nil
}

func newSessionCmd(app *apppkg.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Manage sessions",
	}
	cmd.AddCommand(newSessionKillCmd(app))
	cmd.AddCommand(newSessionRunCmd(app))
	return cmd
}

func newSessionKillCmd(app *apppkg.App) *cobra.Command {
	return &cobra.Command{
		Use:   "kill <session>",
		Short: "Kill a session and clean up workspace membership",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sessName := args[0]

			if err := workspace.KillSession(app.Runner, app.WorkspaceStore, sessName); err != nil {
				return err
			}
			fmt.Printf("Killed session %q\n", sessName)
			return nil
		},
	}
}

func refreshDuplicateWindowNameMarkers(app *apppkg.App, sessionName string) error {
	if sessionName == "" {
		if app.Runner.IsInsideTmux() {
			name, err := app.Runner.DisplayMessage("", "#{session_name}")
			if err != nil {
				return fmt.Errorf("current session: %w", err)
			}
			sessionName = strings.TrimSpace(name)
		}
	}
	if sessionName != "" {
		return refreshDuplicateWindowNameMarkersForSession(app, sessionName)
	}

	sessions, err := app.Runner.ListSessions()
	if err != nil {
		return err
	}
	for _, s := range sessions {
		if tabs.IsReservedSession(s.Name) {
			continue // dock windows aren't presented — no markers to manage
		}
		_ = refreshDuplicateWindowNameMarkersForSession(app, s.Name)
	}
	return nil
}

func refreshDuplicateWindowNameMarkersForSession(app *apppkg.App, sessionName string) error {
	windows, err := app.Runner.ListWindows(sessionName)
	if err != nil {
		// This command is primarily called from tmux hooks. Hooks must never become
		// user-visible noise if a session disappears mid-hook or a dead pane reports
		// incomplete metadata; the next successful refresh will repair markers.
		return nil
	}
	// The [cwd-basename] disambiguator only earns its place when a window's
	// name is duplicated AND its basename is unique among the same-named
	// windows. If two same-named tabs share a cwd basename (the common
	// same-worktree case — e.g. two "claude" tabs both in skills/, two "bun"
	// tabs in one feature branch) the bracket renders identically on both and
	// differentiates nothing the window index doesn't already resolve, so we
	// suppress it rather than add noise. Basename, not full path, because the
	// bracket itself shows #{b:pane_current_path}.
	//
	// The decision reads each window's cwd at refresh time; the rendered
	// bracket uses the live #{b:pane_current_path}. A cd after this hook fired
	// can briefly desync the two until the next window event re-refreshes —
	// acceptable for a dimmed cosmetic marker.
	nameCounts := make(map[string]int, len(windows))
	nameBaseCounts := make(map[string]int, len(windows))
	bases := make([]string, len(windows))
	for i, w := range windows {
		bases[i] = filepath.Base(w.Dir)
		nameCounts[w.Name]++
		nameBaseCounts[w.Name+"\x00"+bases[i]]++
	}
	for i, w := range windows {
		target := fmt.Sprintf("%s:%d", sessionName, w.Index)
		disambiguates := nameCounts[w.Name] > 1 && nameBaseCounts[w.Name+"\x00"+bases[i]] == 1
		if disambiguates {
			_ = app.Runner.SetWindowOption(target, tablabel.DuplicateNameOption, "1")
		} else {
			_ = app.Runner.UnsetWindowOption(target, tablabel.DuplicateNameOption)
		}
	}
	return nil
}

func setTabLabel(app *apppkg.App, cmd *cobra.Command, target, label string, clear bool) error {
	label = strings.TrimSpace(label)

	// Resolve the target window and its active pane in one round-trip — an
	// empty target keeps tmux's "client's current window" semantics.
	out, err := app.Runner.DisplayMessage(target, "#{pane_id}\t#{session_name}:#{window_index}")
	if err != nil {
		return fmt.Errorf("resolve tab label target: %w", err)
	}
	parts := strings.SplitN(strings.TrimSpace(out), "\t", 2)
	if len(parts) != 2 || parts[0] == "" {
		return fmt.Errorf("resolve tab label target: unexpected display output %q", out)
	}
	paneID, window := parts[0], parts[1]

	// The label belongs to the window's owning TAB when one exists — write
	// its canonical pane, not whichever pane happens to be active (a raw
	// split beside the tab must never get stamped into a second tab).
	if logical, lerr := tabs.ListLogicalTabs(app.Runner); lerr == nil {
		for i := range logical {
			t := &logical[i]
			if t.Placement == tabs.PlacementFull && fmt.Sprintf("%s:%d", t.Session, t.WindowIndex) == window {
				paneID = t.PaneID
				break
			}
		}
	}

	if clear || label == "" {
		// Clearing the label keeps the tab managed (id stays) — it just
		// reverts to presenting under its live window name.
		writes := []tmux.OptionWrite{
			{Scope: tmux.ScopePane, Target: paneID, Key: tablabel.Option, Unset: true},
			{Scope: tmux.ScopePane, Target: paneID, Key: tablabel.SourceOption, Unset: true},
			{Scope: tmux.ScopeWindow, Target: window, Key: tablabel.Option, Unset: true},
			{Scope: tmux.ScopeWindow, Target: window, Key: tablabel.SourceOption, Unset: true},
		}
		if err := app.Runner.ApplyOptions(writes); err != nil {
			return fmt.Errorf("clear tab label: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "cleared tab label")
		return nil
	}

	// Stamp claims unmanaged windows as logical tabs (pane id + canonical
	// label) and relabels managed ones idempotently; the window mirror rides
	// along in the same batch.
	if _, err := tabs.Stamp(app.Runner, paneID, window, label, tablabel.SourceManual); err != nil {
		return fmt.Errorf("set tab label: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "tab label: %s\n", label)
	return nil
}
