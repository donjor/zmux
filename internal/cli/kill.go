package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/workspace"
	"github.com/spf13/cobra"
)

func newKillCmd(app *apppkg.App) *cobra.Command {
	var assumeYes bool
	cmd := &cobra.Command{
		Use:     "kill <name>",
		Aliases: []string{"k"},
		Short:   "Kill a workspace or session",
		Long: `Kill a workspace (and all its sessions) or a single session.

Workspace names are checked first; otherwise the argument is resolved as a
session target — workspace/session, a workspace-local label, or a raw tmux name —
the same way run, send, type, and watch address sessions.

  zmux kill myapp        Kill workspace 'myapp' and all its sessions (confirms if live)
  zmux kill myapp/api    Kill the 'api' session in workspace 'myapp'
  zmux kill auth         Kill session 'auth' (cleans up workspace membership)`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			// Workspace names win over bare session labels. A workspace/session
			// target can never match a workspace name, so it falls straight
			// through to the session path below.
			ws, err := app.WorkspaceStore.GetWorkspace(name)
			if err != nil {
				return err
			}
			if ws != nil {
				return killWorkspace(app, ws, assumeYes)
			}

			// Resolve the session through the shared resolver so kill honors
			// workspace/session targets and local labels instead of only raw
			// tmux names.
			target, err := resolveSessionTarget(app, name)
			if err != nil {
				return err
			}
			if !app.Runner.HasSession(target) {
				return fmt.Errorf("session %q is not live", name)
			}
			return workspace.KillSession(app.Runner, app.WorkspaceStore, target)
		},
	}
	cmd.Flags().BoolVarP(&assumeYes, "yes", "y", false, "skip the workspace kill confirmation prompt")
	return cmd
}

func killWorkspace(app *apppkg.App, ws *workspace.Workspace, assumeYes bool) error {
	// Count live sessions.
	liveCount := 0
	for _, sess := range ws.Sessions {
		if app.Runner.HasSession(sess.TmuxName) {
			liveCount++
		}
	}

	if liveCount > 0 && !assumeYes {
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
	for _, sess := range ws.Sessions {
		if app.Runner.HasSession(sess.TmuxName) {
			_ = session.Kill(app.Runner, sess.TmuxName)
		}
	}

	// Remove workspace.
	return app.WorkspaceStore.DeleteWorkspace(ws.Name)
}
