package main

import (
	"fmt"
	"strings"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tablabel"
	"github.com/spf13/cobra"
)

var tabCmd = &cobra.Command{
	Use:   "tab",
	Short: "Manage tabs within sessions",
}

var tabMoveCmd = &cobra.Command{
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

var tabLabelTarget string
var tabLabelClear bool

var tabLabelCmd = &cobra.Command{
	Use:   "label [label]",
	Short: "Set or clear a stable zmux label for the current tab",
	Long:  "Set a stable zmux label overlay for the current tab. The tmux auto-name remains visible as label [auto]. Pass an empty label or --clear to clear.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		label := ""
		if len(args) > 0 {
			label = args[0]
		}
		return setTabLabel(cmd, tabLabelTarget, label, tabLabelClear)
	},
}

var tabRefreshNamesCmd = &cobra.Command{
	Use:    "refresh-names [session]",
	Short:  "Refresh duplicate tab-name markers",
	Hidden: true,
	Args:   cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionName := ""
		if len(args) > 0 {
			sessionName = strings.TrimSpace(args[0])
		}
		return refreshDuplicateWindowNameMarkers(sessionName)
	},
}

var tabKillCmd = &cobra.Command{
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

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Manage sessions",
}

var sessionKillCmd = &cobra.Command{
	Use:   "kill <session>",
	Short: "Kill a session and clean up workspace membership",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessName := args[0]

		if err := session.Kill(app.Runner, sessName); err != nil {
			return err
		}

		root := session.RootName(sessName)

		// Only remove from workspace if we killed the root session itself
		// (not just a grouped clone like dev-b).
		isClone := sessName != root
		if !isClone {
			// Root session killed — check if it's truly gone (not just a clone detach).
			if !app.Runner.HasSession(root) {
				_ = app.WorkspaceStore.RemoveSession(root)
			}
		}

		fmt.Printf("Killed session %q\n", sessName)
		return nil
	},
}

func refreshDuplicateWindowNameMarkers(sessionName string) error {
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
		return refreshDuplicateWindowNameMarkersForSession(sessionName)
	}

	sessions, err := app.Runner.ListSessions()
	if err != nil {
		return err
	}
	for _, s := range sessions {
		_ = refreshDuplicateWindowNameMarkersForSession(s.Name)
	}
	return nil
}

func refreshDuplicateWindowNameMarkersForSession(sessionName string) error {
	windows, err := app.Runner.ListWindows(sessionName)
	if err != nil {
		// This command is primarily called from tmux hooks. Hooks must never become
		// user-visible noise if a session disappears mid-hook or a dead pane reports
		// incomplete metadata; the next successful refresh will repair markers.
		return nil
	}
	counts := make(map[string]int, len(windows))
	for _, w := range windows {
		counts[w.Name]++
	}
	for _, w := range windows {
		target := fmt.Sprintf("%s:%d", sessionName, w.Index)
		if counts[w.Name] > 1 {
			_ = app.Runner.SetWindowOption(target, tablabel.DuplicateNameOption, "1")
		} else {
			_ = app.Runner.UnsetWindowOption(target, tablabel.DuplicateNameOption)
		}
	}
	return nil
}

func setTabLabel(cmd *cobra.Command, target, label string, clear bool) error {
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

func init() {
	tabLabelCmd.Flags().StringVar(&tabLabelTarget, "target", "", "target tmux window (defaults to current)")
	tabLabelCmd.Flags().BoolVar(&tabLabelClear, "clear", false, "clear the tab label")
	tabCmd.AddCommand(tabMoveCmd)
	tabCmd.AddCommand(tabLabelCmd)
	tabCmd.AddCommand(tabRefreshNamesCmd)
	tabCmd.AddCommand(tabKillCmd)
	rootCmd.AddCommand(tabCmd)

	sessionCmd.AddCommand(sessionKillCmd)
	rootCmd.AddCommand(sessionCmd)
}
