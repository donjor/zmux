package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/spf13/cobra"
)

// newPaneListCmd builds the pane-listing command. Registered as "list"
// under `zmux pane` and as top-level "panes" (alias "list-panes") for
// symmetry with `zmux ls`.
func newPaneListCmd(app *apppkg.App, use string, aliases ...string) *cobra.Command {
	flags := &paneListFlags{}
	cmd := &cobra.Command{
		Use:     use,
		Aliases: aliases,
		Short:   "List panes",
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
	cmd.Flags().BoolVar(&flags.joined, "joined", false, "list joined logical-tab panes in the current or target session")
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
	if flags.joined {
		panes, err := loadJoinedPanesForList(app, flags)
		if err != nil {
			return err
		}
		return printJoinedPanes(cmd, panes, flags)
	}

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

type joinedPaneListRow struct {
	TabID      string `json:"tabID"`
	TabName    string `json:"tabName"`
	PaneID     string `json:"paneID"`
	Session    string `json:"session"`
	HostName   string `json:"hostName,omitempty"`
	HostPaneID string `json:"hostPaneID,omitempty"`
	AnchorID   string `json:"anchorID,omitempty"`
	CWD        string `json:"cwd,omitempty"`
	Command    string `json:"command,omitempty"`
	Title      string `json:"title,omitempty"`
	Active     bool   `json:"active"`
	Caller     bool   `json:"caller"`
}

func loadJoinedPanesForList(app *apppkg.App, flags *paneListFlags) ([]joinedPaneListRow, error) {
	if flags.all {
		return nil, fmt.Errorf("--joined cannot be combined with --all")
	}

	// --joined is intentionally session-scoped. The default raw pane list stays
	// current-window scoped, but joined-pane reuse needs to find sibling-window
	// logical tabs inside the current or target session.
	target, err := resolvePaneListSessionTarget(app, flags.target)
	if err != nil {
		return nil, err
	}
	panes, err := app.Runner.ListPanes(target)
	if err != nil {
		return nil, err
	}
	byPane := make(map[string]tmux.Pane, len(panes))
	for _, pane := range panes {
		byPane[pane.ID] = pane
	}

	logical, err := tabs.ListLogicalTabs(app.Runner)
	if err != nil {
		return nil, err
	}
	byTabID := make(map[string]*tabs.LogicalTab, len(logical))
	for i := range logical {
		byTabID[logical[i].ID] = &logical[i]
	}

	callerPane := os.Getenv("TMUX_PANE")
	joined := make([]joinedPaneListRow, 0)
	for i := range logical {
		tab := &logical[i]
		if tab.Placement != tabs.PlacementPaneOf {
			continue
		}
		pane, ok := byPane[tab.PaneID]
		if !ok {
			continue
		}

		row := joinedPaneListRow{
			TabID:    tab.ID,
			TabName:  tabs.DisplayName(tab),
			PaneID:   tab.PaneID,
			Session:  firstNonEmpty(pane.Session, tab.Session),
			AnchorID: tab.AnchorID,
			CWD:      firstNonEmpty(pane.Dir, tab.Dir),
			Command:  firstNonEmpty(pane.Command, tab.Command),
			Title:    firstNonEmpty(pane.Title, tab.Title),
			Active:   pane.Active,
			Caller:   tab.PaneID == callerPane,
		}
		if host := byTabID[tab.AnchorID]; host != nil {
			row.HostName = tabs.DisplayName(host)
			row.HostPaneID = host.PaneID
		}
		joined = append(joined, row)
	}
	return joined, nil
}

func printJoinedPanes(cmd *cobra.Command, panes []joinedPaneListRow, flags *paneListFlags) error {
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
			fmt.Fprintln(cmd.OutOrStdout(), pane.PaneID)
		}
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TAB\tPANE\tCALLER\tACTIVE\tSESSION\tHOST\tANCHOR\tTITLE\tCMD\tCWD")
	for _, pane := range panes {
		caller := ""
		if pane.Caller {
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
		host := pane.HostName
		if host == "" {
			host = "-"
		}
		anchor := pane.AnchorID
		if anchor == "" {
			anchor = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			pane.TabName, pane.PaneID, caller, active, pane.Session, host, anchor, title, pane.Command, pane.CWD)
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
		target, err := resolvePaneListSessionTarget(app, flags.target)
		if err != nil {
			return nil, err
		}
		return app.Runner.ListPanes(target)
	}
	return app.Runner.ListWindowPanes(flags.target)
}

func resolvePaneListSessionTarget(app *apppkg.App, target string) (string, error) {
	if target == "" {
		return "", nil
	}
	resolved, err := resolveSessionTarget(app, target)
	if err == nil {
		return resolved, nil
	}
	if strings.Contains(target, "/") {
		return "", err
	}
	return target, nil
}

func runPaneCurrent(app *apppkg.App, cmd *cobra.Command, flags *paneCurrentFlags) error {
	paneID := os.Getenv("TMUX_PANE")
	if paneID == "" || !app.Runner.IsInsideTmux() {
		return fmt.Errorf("pane current requires tmux in the current profile")
	}
	if !flags.json {
		fmt.Fprintln(cmd.OutOrStdout(), paneID)
		return nil
	}
	panes, err := app.Runner.ListWindowPanes(paneID)
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
