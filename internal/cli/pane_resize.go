package cli

import (
	"fmt"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/spf13/cobra"
)

func newPaneResizeCmd(app *apppkg.App) *cobra.Command {
	flags := &paneResizeFlags{}
	cmd := &cobra.Command{
		Use:   "resize <pane>",
		Short: "Resize a pane",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPaneResize(app, flags, args[0])
		},
	}
	cmd.Flags().StringVar(&flags.size, "size", "", "set pane width, e.g. 40% or 80 cells")
	cmd.Flags().StringVar(&flags.width, "width", "", "set pane width")
	cmd.Flags().StringVar(&flags.height, "height", "", "set pane height")
	return cmd
}

func runPaneResize(app *apppkg.App, flags *paneResizeFlags, selector string) error {
	target, err := resolvePaneSelector(app, selector)
	if err != nil {
		return err
	}
	axis := "width"
	size := flags.size
	selected := 0
	if flags.size != "" {
		selected++
	}
	if flags.width != "" {
		selected++
		axis = "width"
		size = flags.width
	}
	if flags.height != "" {
		selected++
		axis = "height"
		size = flags.height
	}
	if selected == 0 {
		return fmt.Errorf("pane resize requires --size, --width, or --height")
	}
	if selected > 1 {
		return fmt.Errorf("choose only one of --size, --width, or --height")
	}
	return app.Runner.ResizePane(target, axis, size)
}
