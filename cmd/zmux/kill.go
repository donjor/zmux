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

		return session.Kill(app.Runner, name)
	},
}

func init() {
	rootCmd.AddCommand(killCmd)
}
