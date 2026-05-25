package cli

import (
	"fmt"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/spf13/cobra"
)

func newRefreshCmd(app *apppkg.App) *cobra.Command {
	var refreshClient bool

	cmd := &cobra.Command{
		Use:   "refresh",
		Short: "Apply zmux config and refresh the current tmux client",
		Long:  "Regenerate and source zmux's tmux config, then reattach the current tmux client so terminal capabilities such as RGB truecolor are re-resolved. Outside tmux, this behaves like zmux apply.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := runApply(app, false); err != nil {
				return err
			}
			if !refreshClient || !app.Runner.IsInsideTmux() {
				fmt.Fprintln(cmd.OutOrStdout(), "refreshed zmux config")
				return nil
			}
			return runTerminalRefresh(app, cmd, &terminalRefreshFlags{})
		},
	}
	cmd.Flags().BoolVar(&refreshClient, "client", true, "reattach current tmux client to refresh terminal capabilities")
	return cmd
}
