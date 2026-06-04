package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/tablabel"
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

			// Validate destination is in the same workspace.
			srcWS, srcOK := app.WorkspaceStore.WorkspaceFor(current)
			dstWS, dstOK := app.WorkspaceStore.WorkspaceFor(destSession)
			if srcOK && dstOK && srcWS != dstWS {
				return fmt.Errorf("destination session %q is in workspace %q, not %q", destSession, dstWS, srcWS)
			}

			if !app.Runner.HasSession(destSession) {
				return fmt.Errorf("session %q not found", destSession)
			}

			// Check this isn't the last tab.
			windows, err := app.Runner.ListWindows(current)
			if err != nil {
				return fmt.Errorf("cannot list tabs: %w", err)
			}
			if len(windows) <= 1 {
				return fmt.Errorf("cannot move the last tab — use `zmux session kill %s` instead", current)
			}

			// Move window (tmux move-window -s session:name -t dest).
			src := current + ":" + tabName
			dst := destSession + ":"
			return app.Runner.MoveWindow(src, dst)
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

			// Resolve tab name to index, then kill.
			windows, wErr := app.Runner.ListWindows(current)
			if wErr != nil {
				return fmt.Errorf("cannot list tabs: %w", wErr)
			}

			// Guard: killing the last tab kills the session in tmux.
			// Use `zmux session kill` instead for proper workspace cleanup.
			if len(windows) <= 1 {
				return fmt.Errorf("cannot kill the last tab — use `zmux session kill %s` instead", current)
			}

			for _, w := range windows {
				if w.Name == tabName {
					return app.Runner.KillWindow(current, w.Index)
				}
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
	if clear || label == "" {
		if err := app.Runner.UnsetWindowOption(target, tablabel.Option); err != nil {
			return fmt.Errorf("clear tab label: %w", err)
		}
		if err := app.Runner.UnsetWindowOption(target, tablabel.SourceOption); err != nil {
			return fmt.Errorf("clear tab label source: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "cleared tab label")
		return nil
	}
	if err := app.Runner.SetWindowOption(target, tablabel.Option, label); err != nil {
		return fmt.Errorf("set tab label: %w", err)
	}
	if err := app.Runner.SetWindowOption(target, tablabel.SourceOption, tablabel.SourceManual); err != nil {
		return fmt.Errorf("set tab label source: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "tab label: %s\n", label)
	return nil
}
