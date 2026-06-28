// Package main: pane subcommands.
//
// Split for clarity by feature group, not by definition-vs-runner.
// Each file owns one user-facing feature end-to-end (command def +
// flags + run impl + supporting helpers).
//
//   - pane.go         — root paneCmd, shared flag structs, init wiring
//   - pane_open.go    — `pane open` / `pane toggle` + arg parsing helpers
//   - pane_list.go    — `pane list` / `pane current` + top-level aliases
//   - pane_resize.go  — `pane resize`
//   - pane_select.go  — `pane close` / `pane focus` + selector helpers
package cli

import (
	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/spf13/cobra"
)

const paneAutoSize = "auto"

type paneOpenFlags struct {
	target   string
	cwd      string
	name     string
	size     string
	right    string
	left     string
	down     string
	up       string
	labelTab bool
}

type paneListFlags struct {
	target  string
	session bool
	all     bool
	joined  bool
	quiet   bool
	json    bool
}

type paneResizeFlags struct {
	size   string
	width  string
	height string
}

type paneCurrentFlags struct {
	json bool
}

type paneToggleFlags struct {
	paneOpenFlags
	focus   bool
	replace bool
}

func newPaneCmd(app *apppkg.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pane",
		Short: "Manage tmux panes with zmux-native commands",
	}
	cmd.AddCommand(newPaneOpenCmd(app))
	cmd.AddCommand(newPaneListCmd(app))
	cmd.AddCommand(newPaneCurrentCmd(app))
	cmd.AddCommand(newPaneToggleCmd(app))
	cmd.AddCommand(newPaneCloseCmd(app))
	cmd.AddCommand(newPaneFocusCmd(app))
	cmd.AddCommand(newPaneResizeCmd(app))
	return cmd
}
