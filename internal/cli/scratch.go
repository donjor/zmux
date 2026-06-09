package cli

import (
	"fmt"
	"os"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/spf13/cobra"
)

// newScratchCmd builds the `zmux scratch` subcommand tree. The only verb today
// is `extract` — invoked from inside the scratch shell popup to promote the
// throwaway shell's cwd into a real tab in the parent session.
func newScratchCmd(app *apppkg.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scratch",
		Short: "Scratch-shell popup helpers",
		Long: `Helpers for the scratch shell (prefix+!). Run ` + "`zmux scratch extract`" + `
from inside the popup to promote the current cwd into a new tab in
the parent session and close the popup.`,
	}
	cmd.AddCommand(newScratchExtractCmd(app))
	return cmd
}

func newScratchExtractCmd(app *apppkg.App) *cobra.Command {
	return &cobra.Command{
		Use:   "extract",
		Short: "Promote scratch shell cwd to a new tab in the parent session",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !app.Runner.IsInsideTmux() {
				return fmt.Errorf("zmux scratch extract must run inside the scratch popup")
			}

			session, err := app.Runner.DisplayMessage("", "#S")
			if err != nil || session == "" {
				return fmt.Errorf("could not resolve parent session: %w", err)
			}

			dir, err := os.Getwd()
			if err != nil || dir == "" {
				dir = os.Getenv("HOME")
			}

			if _, err := app.Runner.NewWindow(session, "", dir); err != nil {
				return fmt.Errorf("create tab in %s: %w", session, err)
			}

			// Close the popup we are currently inside. -C is best-effort —
			// older tmux returns an error but the new tab is already there,
			// so swallow it and let the user close the popup manually.
			_ = app.Runner.DisplayPopup("-C")
			return nil
		},
	}
}
