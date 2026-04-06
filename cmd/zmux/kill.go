package main

import (
	"fmt"

	"github.com/donjor/zmux/internal/session"
	"github.com/spf13/cobra"
)

var killCmd = &cobra.Command{
	Use:     "kill <name>",
	Aliases: []string{"k"},
	Short:   "Kill a session",
	Long:    `Kill a tmux session by name.`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if !app.Runner.HasSession(name) {
			return fmt.Errorf("session %q does not exist", name)
		}

		if err := session.Kill(app.Runner, name); err != nil {
			return err
		}

		// Remove from workspace tracking.
		_ = app.WorkspaceStore.Delete(session.RootName(name))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(killCmd)
}
