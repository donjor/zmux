package cli

import (
	"fmt"
	"strings"
	"time"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/debug"
	"github.com/donjor/zmux/internal/tabs"
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
			MaybeReap(app, time.Now())
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
				target, err := resolveSessionTarget(app, sessionName)
				if err != nil {
					return err
				}
				return listTabsForSession(app, target)
			}

			return fmt.Errorf("specify a session: zmux tabs <session>\nlist sessions with: zmux ls")
		},
	}
	cmd.Flags().StringVarP(&tabsSessionFlag, "session", "s", "", "target session")
	return cmd
}

func listTabsForSession(app *apppkg.App, session string) error {
	// Heal logical/physical drift before presenting — manual joins/breaks,
	// killed windows, dead dock. Best-effort: listing still works when the
	// reconciler can't (and the scan below degrades to plain windows).
	if _, err := tabs.Reconcile(app.Runner); err != nil {
		debug.Log("tabs: reconcile before listing failed", "err", err)
	}
	logical, err := tabs.ListLogicalTabs(app.Runner)
	if err != nil {
		logical = nil
	}

	windows, err := app.Runner.ListWindows(session)
	if err != nil {
		return err
	}

	panes, _ := app.Runner.ListPanes(session)
	panesByWindow := make(map[int][]string)
	for _, p := range panes {
		panesByWindow[p.WindowIndex] = append(panesByWindow[p.WindowIndex], p.Command)
	}

	// Index logical tabs for this session: the full tab per window (its
	// label is the addressable name), pane-of riders per window, and docked
	// tabs whose recorded origin is this session.
	fullByIndex := make(map[int]*tabs.LogicalTab)
	ridersByIndex := make(map[int][]*tabs.LogicalTab)
	var hidden []*tabs.LogicalTab
	for i := range logical {
		t := &logical[i]
		switch {
		case t.Placement == tabs.PlacementDock:
			if t.OriginSession == session {
				hidden = append(hidden, t)
			}
		case t.Session != session:
		case t.Placement == tabs.PlacementFull:
			fullByIndex[t.WindowIndex] = t
		default:
			ridersByIndex[t.WindowIndex] = append(ridersByIndex[t.WindowIndex], t)
		}
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

		// Show the addressable name: the tab/window label when set (it is
		// what send/watch/run resolve), with the auto-renamed live name in
		// brackets when they differ — mirroring the bar's `label [auto]`.
		name := w.Name
		label := w.Label
		if full := fullByIndex[w.Index]; full != nil && full.Label != "" {
			label = full.Label
		}
		if label != "" && label != w.Name {
			name = fmt.Sprintf("%s [%s]", label, w.Name)
		}

		fmt.Printf("  %s %d: %-14s %s  %s\n", active, w.Index, name, cmdStr, dir)

		// Pane-of tabs ride inside this window but stay addressable tabs.
		for _, r := range ridersByIndex[w.Index] {
			fmt.Printf("      └ %-12s %s  %s\n", tabs.DisplayName(r), r.Command, r.ID)
		}
	}

	// Docked tabs are out of sight, never out of the list.
	if len(hidden) > 0 {
		fmt.Printf("  hidden:\n")
		for _, h := range hidden {
			fmt.Printf("    ~ %-14s %s  %s\n", tabs.DisplayName(h), h.Command, h.ID)
		}
	}
	return nil
}
