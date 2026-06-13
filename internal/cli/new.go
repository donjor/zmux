package cli

import (
	"fmt"
	"os"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/workspace"
	"github.com/spf13/cobra"
)

func newNewCmd(app *apppkg.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "new <workspace> [session...]",
		Aliases: []string{"n"},
		Short:   "Create a workspace and sessions",
		Long: `Create a new workspace with sessions and attach.

  zmux new myapp                   Create workspace 'myapp' + session 'main', attach
  zmux new myapp auth              Create workspace (if needed) + session 'auth', attach
  zmux new myapp auth server dev   Create workspace + multiple sessions, attach first

If the workspace already exists:
  zmux new myapp         → error (use zmux open myapp to attach)
  zmux new myapp <sess>  → adds session to existing workspace`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := os.Getwd()
			if err != nil {
				dir = os.Getenv("HOME")
			}

			if len(args) == 0 {
				return runNewTmp(app, dir)
			}

			wsName := args[0]
			sessionLabels := args[1:]

			// Validate workspace name.
			if err := workspace.ValidateWorkspaceName(wsName); err != nil {
				return err
			}

			return runNewInWorkspace(app, wsName, sessionLabels, dir)
		},
	}

	return cmd
}

// runNewTmp creates a tmp-N session with no workspace.
func runNewTmp(app *apppkg.App, dir string) error {
	name := session.NextTmpName(app.Runner)
	if err := session.Create(app.Runner, name, dir); err != nil {
		return err
	}
	return attachOwnedSession(app, name)
}

// runNewInWorkspace creates a workspace (if needed) and sessions within it.
func runNewInWorkspace(app *apppkg.App, wsName string, sessionLabels []string, dir string) error {
	// Check if workspace exists.
	ws, err := app.WorkspaceStore.GetWorkspace(wsName)
	if err != nil {
		return err
	}

	if len(sessionLabels) == 0 || (len(sessionLabels) == 1 && sessionLabels[0] == "") {
		// zmux new <workspace> — no session labels.
		if ws != nil && len(ws.Sessions) > 0 {
			return fmt.Errorf(
				"workspace %q already exists — use zmux %s to attach or zmux new %s <session> to add a session",
				wsName, "open "+wsName, wsName,
			)
		}
		sessionLabels = []string{session.DefaultName}
	}

	// Ensure workspace exists.
	if _, err := app.WorkspaceStore.EnsureWorkspace(wsName, dir); err != nil {
		return err
	}

	// Create each session.
	var firstSession workspace.WorkspaceSession
	for _, label := range sessionLabels {
		if label == "" {
			continue
		}
		if err := workspace.ValidateSessionLabel(label); err != nil {
			return fmt.Errorf("invalid session label %q: %w", label, err)
		}
		rec, err := createWorkspaceSession(app, wsName, label, dir)
		if err != nil {
			return err
		}
		if firstSession.TmuxName == "" {
			firstSession = rec
		}
	}

	if firstSession.TmuxName == "" {
		return fmt.Errorf("no sessions created")
	}

	_ = app.WorkspaceStore.SetLastActive(wsName, firstSession.ID)
	return attachOwnedSession(app, firstSession.TmuxName)
}

func createWorkspaceSession(app *apppkg.App, wsName, label, dir string) (workspace.WorkspaceSession, error) {
	rec, err := workspace.NewSessionRecord(wsName, label)
	if err != nil {
		return workspace.WorkspaceSession{}, err
	}
	if app.Runner.HasSession(rec.TmuxName) {
		return workspace.WorkspaceSession{}, fmt.Errorf("session %q already exists in workspace %q", label, wsName)
	}
	if err := session.Create(app.Runner, rec.TmuxName, dir); err != nil {
		return workspace.WorkspaceSession{}, err
	}
	if err := workspace.StampSessionMetadata(app.Runner, wsName, rec); err != nil {
		_ = app.Runner.KillSession(rec.TmuxName)
		return workspace.WorkspaceSession{}, err
	}
	if err := app.WorkspaceStore.AddSessionRecord(wsName, rec); err != nil {
		_ = app.Runner.KillSession(rec.TmuxName)
		return workspace.WorkspaceSession{}, err
	}
	return rec, nil
}
