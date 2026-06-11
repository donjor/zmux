package cli

import (
	"fmt"
	"path/filepath"
	"strings"

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
	cmd.AddCommand(newTabFullCmd(app))
	return cmd
}

func newTabMoveCmd(app *apppkg.App) *cobra.Command {
	return &cobra.Command{
		Use:   "move <tab-name> <dest-session>",
		Short: "Move a tab to another session in the workspace",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			tabName := args[0]
			destSession := args[1]

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

			// Validate destination is in the same workspace.
			srcWS, srcOK := app.WorkspaceStore.WorkspaceFor(srcSession)
			dstWS, dstOK := app.WorkspaceStore.WorkspaceFor(destSession)
			if srcOK && dstOK && srcWS != dstWS {
				return fmt.Errorf("destination session %q is in workspace %q, not %q", destSession, dstWS, srcWS)
			}

			if !app.Runner.HasSession(destSession) {
				return fmt.Errorf("session %q not found", destSession)
			}

			// Check this isn't the source session's last tab.
			windows, err := app.Runner.ListWindows(srcSession)
			if err != nil {
				return fmt.Errorf("cannot list tabs: %w", err)
			}
			if len(windows) <= 1 {
				return fmt.Errorf("cannot move the last tab — use `zmux session kill %s` instead", srcSession)
			}

			return app.Runner.MoveWindow(src, destSession+":")
		},
	}
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
	return &cobra.Command{
		Use:   "kill <tab-name>",
		Short: "Kill a tab in the current session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tabName := args[0]

			current, err := app.Runner.DisplayMessage("", "#{session_name}")
			if err != nil {
				return fmt.Errorf("not inside tmux")
			}
			current = strings.TrimSpace(current)

			rt, err := resolveTabTarget(app, current, tabName)
			if err != nil {
				return err
			}

			// Pane-of and docked tabs kill as panes — the host window (or
			// dock) survives, minus this tab. tmux reaps a dock window when
			// its last pane dies, and the dock session when its last window
			// does.
			if rt.Tab != nil && rt.Tab.Placement != tabs.PlacementFull {
				return app.Runner.KillPane(rt.Tab.PaneID)
			}

			// Full/legacy tabs kill their window. Guard: killing the last
			// window kills the session in tmux — use `zmux session kill`
			// instead for proper workspace cleanup. The guard follows the
			// tab's own session (a unique label can resolve outside current).
			session := current
			if rt.Tab != nil {
				session = rt.Tab.Session
			}
			windows, wErr := app.Runner.ListWindows(session)
			if wErr != nil {
				return fmt.Errorf("cannot list tabs: %w", wErr)
			}
			if len(windows) <= 1 {
				return fmt.Errorf("cannot kill the last tab — use `zmux session kill %s` instead", session)
			}

			switch {
			case rt.Tab != nil:
				return app.Runner.KillWindowByID(rt.Tab.WindowID)
			case rt.Win != nil:
				return app.Runner.KillWindow(current, rt.Win.Index)
			}
			return fmt.Errorf("tab %q not found in session %q", tabName, current)
		},
	}
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
