package main

import (
	"fmt"
	"strings"

	"github.com/donjor/zmux/internal/session"
	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:     "ls",
	Aliases: []string{"list"},
	Short:   "List sessions",
	Long:    `List all tmux sessions with metadata.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		sessions, err := session.ListSessions(app.Runner)
		if err != nil {
			fmt.Println("No sessions.")
			return nil
		}

		if len(sessions) == 0 {
			fmt.Println("No sessions.")
			return nil
		}

		// Detect current session + tab if inside tmux.
		currentSession := ""
		currentTab := ""
		if app.Runner.IsInsideTmux() {
			currentSession, _ = app.Runner.DisplayMessage("", "#{session_name}")
			currentTab, _ = app.Runner.DisplayMessage("", "#{window_name}")
		}

		for _, s := range sessions {
			icon := "○"
			if s.Name == currentSession {
				icon = "◆"
			} else if s.Attached {
				icon = "●"
			}

			age := ""
			if !s.Activity.IsZero() {
				age = session.HumanAge(s.Activity)
			}

			// Get window names.
			winStr := fmt.Sprintf("%dw", s.Windows)
			if wins, err := app.Runner.ListWindows(s.Name); err == nil && len(wins) > 0 {
				names := make([]string, len(wins))
				for i, w := range wins {
					names[i] = w.Name
				}
				winStr = fmt.Sprintf("[%s]", strings.Join(names, ", "))
			}

			dir := shortenPathCLI(s.Dir)

			marker := ""
			if s.Name == currentSession && currentTab != "" {
				marker = fmt.Sprintf("  ← you (%s)", currentTab)
			}
			fmt.Printf("  %s %-14s %s  %s  %s%s\n", icon, s.Name, winStr, dir, age, marker)
		}

		return nil
	},
}

func shortenPathCLI(path string) string {
	home, _ := app.FS.UserHomeDir()
	if home != "" && len(path) > len(home) && path[:len(home)] == home {
		path = "~" + path[len(home):]
	}
	return path
}

func init() {
	rootCmd.AddCommand(lsCmd)
}
