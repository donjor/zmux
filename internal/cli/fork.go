package cli

import (
	"fmt"
	"os"
	"strings"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/workspace"
	"github.com/spf13/cobra"
)

func newForkCmd(app *apppkg.App) *cobra.Command {
	var dirFlag string

	cmd := &cobra.Command{
		Use:   "fork <new-session-label>",
		Short: "Fork the current workspace session layout into a new session",
		Long: `Fork the current session into a new managed session in the same workspace.

The positional argument is the new workspace-local session label, not the
source session. The source is always the current session. Fork copies tab names
and order only; it does not replay commands, copy running processes, create a
worktrunk branch, or persist pane layouts.

  zmux fork feature-auth
  zmux fork feature-auth --dir /path/to/worktree`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !app.Runner.IsInsideTmux() {
				return fmt.Errorf("zmux fork must run inside the source session")
			}
			destLabel := strings.TrimSpace(args[0])
			if err := workspace.ValidateSessionLabel(destLabel); err != nil {
				return fmt.Errorf("invalid session label %q: %w", destLabel, err)
			}

			sourceRoot, err := currentSessionName(app)
			if err != nil {
				return fmt.Errorf("resolve current session: %w", err)
			}

			wsName, ok := app.WorkspaceStore.WorkspaceFor(sourceRoot)
			if !ok {
				return fmt.Errorf("current session %q is not in a workspace", sourceRoot)
			}

			dir := strings.TrimSpace(dirFlag)
			if dir == "" {
				if cwd, err := os.Getwd(); err == nil {
					dir = cwd
				}
			}

			rec, err := workspace.ForkSession(app.Runner, app.WorkspaceStore, wsName, sourceRoot, destLabel, dir)
			if err != nil {
				return err
			}
			_ = app.WorkspaceStore.SetLastActive(wsName, rec.ID)
			return attachOwnedSession(app, rec.TmuxName)
		},
	}
	cmd.Flags().StringVar(&dirFlag, "dir", "", "working directory for every forked tab (default: current directory)")
	return cmd
}
