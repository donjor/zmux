package main

import (
	"fmt"

	"github.com/donjor/zmux/internal/session"
	"github.com/spf13/cobra"
)

var hijackFlag bool
var mirrorFlag bool
var groupFlag bool

var attachCmd = &cobra.Command{
	Use:     "attach <name>",
	Aliases: []string{"a"},
	Short:   "Attach to an existing session",
	Long: `Attach to an existing tmux session by name.

Modes (when session is already attached):
  (default)   Auto-group: independent viewport, shared windows (name-b)
  --mirror    Shared view: both clients see the exact same thing
  --group     Explicit group (same as default, for clarity)
  --hijack    Steal session, detaching other clients

Mirror mode is useful for agent/user shared terminals — both see
the same output, both can type.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if !app.Runner.HasSession(name) {
			return fmt.Errorf("session %q does not exist", name)
		}

		if mirrorFlag {
			return session.AttachMirror(app.Runner, name)
		}

		if hijackFlag {
			return session.AttachHijack(app.Runner, name)
		}

		// Default (and --group): auto-group.
		return session.Attach(app.Runner, name)
	},
}

func init() {
	attachCmd.Flags().BoolVarP(&hijackFlag, "hijack", "H", false, "steal session, detaching other clients")
	attachCmd.Flags().BoolVarP(&mirrorFlag, "mirror", "m", false, "shared view — both clients see the same thing")
	attachCmd.Flags().BoolVarP(&groupFlag, "group", "g", false, "grouped session (default behavior, for clarity)")
	rootCmd.AddCommand(attachCmd)
}
