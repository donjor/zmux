package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/workspace"
	"github.com/spf13/cobra"
)

var killCmd = &cobra.Command{
	Use:     "kill <name>",
	Aliases: []string{"k"},
	Short:   "Kill a workspace or session",
	Long: `Kill a workspace (and all its sessions) or a single session.

Checks workspace names first, then session names.

  zmux kill myapp    Kill workspace 'myapp' and all its sessions (confirms if live)
  zmux kill auth     Kill session 'auth' (cleans up workspace membership)`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Check if it's a workspace.
		ws, err := app.WorkspaceStore.GetWorkspace(name)
		if err != nil {
			return err
		}

		if ws != nil {
			return killWorkspace(ws)
		}

		// Fall back to session kill.
		if !app.Runner.HasSession(name) {
			return fmt.Errorf("%q is not a workspace or session", name)
		}

		if err := session.Kill(app.Runner, name); err != nil {
			return err
		}
		_ = app.WorkspaceStore.RemoveSession(session.RootName(name))
		return nil
	},
}

func killWorkspace(ws *workspace.Workspace) error {
	// Count live sessions.
	liveCount := 0
	for _, sessName := range ws.Sessions {
		if app.Runner.HasSession(sessName) {
			liveCount++
		}
	}

	if liveCount > 0 {
		// Confirm with user.
		plural := "session"
		if liveCount > 1 {
			plural = "sessions"
		}
		fmt.Printf("Kill %d live %s in workspace %q? (y/N) ", liveCount, plural, ws.Name)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// Kill all sessions.
	for _, sessName := range ws.Sessions {
		if app.Runner.HasSession(sessName) {
			_ = session.Kill(app.Runner, sessName)
		}
	}

	// Remove workspace.
	return app.WorkspaceStore.DeleteWorkspace(ws.Name)
}

func init() {
	rootCmd.AddCommand(killCmd)
}
