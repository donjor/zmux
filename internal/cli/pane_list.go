package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/spf13/cobra"
)

func newPaneListCmd(app *apppkg.App) *cobra.Command {
	flags := &paneListFlags{}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List panes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPaneList(app, cmd, flags)
		},
	}
	addPaneListFlags(cmd, flags)
	return cmd
}

// newTopLevelPaneListCmd is the same as `pane list` but registered as
// a top-level command (e.g. `zmux panes`) for symmetry with `zmux ls`.
func newTopLevelPaneListCmd(app *apppkg.App, use string) *cobra.Command {
	flags := &paneListFlags{}
	cmd := &cobra.Command{
		Use:   use,
		Short: "List panes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPaneList(app, cmd, flags)
		},
	}
	addPaneListFlags(cmd, flags)
	return cmd
}

func addPaneListFlags(cmd *cobra.Command, flags *paneListFlags) {
	cmd.Flags().StringVar(&flags.target, "target", "", "target session/window/pane")
	cmd.Flags().BoolVar(&flags.session, "session", false, "list all panes in the current or target session")
	cmd.Flags().BoolVar(&flags.all, "all", false, "list panes across all sessions")
	cmd.Flags().BoolVarP(&flags.quiet, "quiet", "q", false, "print only pane ids")
	cmd.Flags().BoolVar(&flags.json, "json", false, "print pane data as JSON")
}

func newPaneCurrentCmd(app *apppkg.App) *cobra.Command {
	flags := &paneCurrentFlags{}
	cmd := &cobra.Command{
		Use:   "current",
		Short: "Print the current pane id",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPaneCurrent(app, cmd, flags)
		},
	}
	cmd.Flags().BoolVar(&flags.json, "json", false, "print current pane data as JSON")
	return cmd
}

func runPaneList(app *apppkg.App, cmd *cobra.Command, flags *paneListFlags) error {
	panes, err := loadPanesForList(app, flags)
	if err != nil {
		return err
	}
	if flags.json {
		encoded, err := json.MarshalIndent(panes, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
		return nil
	}
	if flags.quiet {
		for _, pane := range panes {
			fmt.Fprintln(cmd.OutOrStdout(), pane.ID)
		}
		return nil
	}

	callerPane := os.Getenv("TMUX_PANE")
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tCALLER\tACTIVE\tSESSION\tWIN\tIDX\tTITLE\tCMD\tSIZE\tCWD")
	for _, pane := range panes {
		caller := ""
		if pane.ID == callerPane {
			caller = "you"
		}
		active := ""
		if pane.Active {
			active = "*"
		}
		title := pane.Title
		if title == "" {
			title = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%d\t%s\t%s\t%dx%d\t%s\n",
			pane.ID, caller, active, pane.Session, pane.WindowIndex, pane.Index, title, pane.Command, pane.Width, pane.Height, pane.Dir)
	}
	return w.Flush()
}

func loadPanesForList(app *apppkg.App, flags *paneListFlags) ([]tmux.Pane, error) {
	if flags.all && flags.session {
		return nil, fmt.Errorf("choose only one of --all or --session")
	}
	if flags.all && flags.target != "" {
		return nil, fmt.Errorf("--all cannot be combined with --target")
	}
	if flags.all {
		return app.Runner.ListAllPanes()
	}
	if flags.session {
		return app.Runner.ListPanes(flags.target)
	}
	return app.Runner.ListWindowPanes(flags.target)
}

func runPaneCurrent(app *apppkg.App, cmd *cobra.Command, flags *paneCurrentFlags) error {
	paneID := os.Getenv("TMUX_PANE")
	if paneID == "" {
		return fmt.Errorf("pane current requires tmux")
	}
	if !flags.json {
		fmt.Fprintln(cmd.OutOrStdout(), paneID)
		return nil
	}
	panes, err := app.Runner.ListWindowPanes("")
	if err != nil {
		return err
	}
	for _, pane := range panes {
		if pane.ID == paneID {
			encoded, err := json.MarshalIndent(pane, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
			return nil
		}
	}
	encoded, err := json.MarshalIndent(tmux.Pane{ID: paneID}, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
	return nil
}
