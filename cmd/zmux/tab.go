package main

import (
	"fmt"
	"strings"

	"github.com/donjor/zmux/internal/session"
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

func init() {
	tabCmd.AddCommand(tabMoveCmd)
	tabCmd.AddCommand(tabKillCmd)
	rootCmd.AddCommand(tabCmd)

	sessionCmd.AddCommand(sessionKillCmd)
	rootCmd.AddCommand(sessionCmd)
}
