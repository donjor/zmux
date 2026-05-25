package cli

import (
	"fmt"
	"strings"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/spf13/cobra"
)

func newTabsCmd(app *apppkg.App) *cobra.Command {
	var tabsSessionFlag string

	cmd := &cobra.Command{
		Use:     "tabs [session]",
		Aliases: []string{"t"},
		Short:   "List tabs in a session",
		Long: `List all tabs in a session with their running commands.

If no session is specified, uses the current session (inside tmux)
or lists tabs for all sessions (outside tmux).

Examples:
  zmux tabs              # current session's tabs
  zmux tabs dev          # tabs in 'dev' session
  zmux t                 # alias`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sessionName := ""
			if len(args) > 0 {
				sessionName = args[0]
			} else if tabsSessionFlag != "" {
				sessionName = tabsSessionFlag
			} else if app.Runner.IsInsideTmux() {
				name, err := app.Runner.DisplayMessage("", "#{session_name}")
				if err != nil {
					return fmt.Errorf("could not get current session")
				}
				sessionName = name
			}

			if sessionName != "" {
				return listTabsForSession(app, sessionName)
			}

			return fmt.Errorf("specify a session: zmux tabs <session>\nlist sessions with: zmux ls")
		},
	}
	cmd.Flags().StringVarP(&tabsSessionFlag, "session", "s", "", "target session")
	return cmd
}

func listTabsForSession(app *apppkg.App, session string) error {
	windows, err := app.Runner.ListWindows(session)
	if err != nil {
		return err
	}

	panes, _ := app.Runner.ListPanes(session)
	panesByWindow := make(map[int][]string)
	for _, p := range panes {
		panesByWindow[p.WindowIndex] = append(panesByWindow[p.WindowIndex], p.Command)
	}

	for _, w := range windows {
		active := " "
		if w.Active {
			active = "*"
		}

		cmds := panesByWindow[w.Index]
		cmdStr := ""
		if len(cmds) > 0 {
			cmdStr = strings.Join(cmds, ", ")
		}

		dir := shortenPathCLI(app, w.Dir)

		fmt.Printf("  %s %d: %-14s %s  %s\n", active, w.Index, w.Name, cmdStr, dir)
	}
	return nil
}
