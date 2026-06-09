package cli

import (
	"fmt"
	"time"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/bar"
	"github.com/spf13/cobra"
)

// newBarSpinnerCmd prints the running-state spinner frame for the current
// second. Called by tmux as a #() status job from the window-status state
// fragment — tmux re-runs it every status-interval and caches one result
// for all window cells. tabstate.Service drops the interval to 1s while
// any running state exists (idle baseline 5s), so the frame steps exactly
// when a spinner is on screen. Frame math lives in bar.SpinnerFrame so the
// glyph set stays beside the other state glyphs.
func newBarSpinnerCmd(_ *apppkg.App) *cobra.Command {
	return &cobra.Command{
		Use:    "bar-spinner",
		Short:  "Print the running-state spinner frame (used by tmux #())",
		Args:   cobra.NoArgs,
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprint(cmd.OutOrStdout(), bar.SpinnerFrame(time.Now()))
			return nil
		},
	}
}
