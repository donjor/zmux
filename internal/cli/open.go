package cli

import (
	"fmt"
	"strings"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/workspace"
	"github.com/spf13/cobra"
)

func newOpenCmd(app *apppkg.App) *cobra.Command {
	var openHijackFlag bool
	var openPinViewFlag bool

	cmd := &cobra.Command{
		Use:     "open <workspace> [session]",
		Aliases: []string{"attach", "a"},
		Short:   "Open a workspace or attach to a session",
		Long: `Open a workspace — attach to its last-active session.

  zmux open <workspace>             Attach last-active session in workspace
  zmux open <workspace> <session>   Attach specific session in workspace

If the target session is already attached elsewhere, a clone
(independent viewport) is created automatically. Use --hijack
to take over instead, or --pin-view to create a persistent grouped viewport.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if openHijackFlag && openPinViewFlag {
				return fmt.Errorf("--hijack and --pin-view cannot be used together")
			}
			wsName, requestedLabel, err := parseWorkspaceSessionArgs(args)
			if err != nil {
				return err
			}

			// Look up workspace.
			ws, err := app.WorkspaceStore.GetWorkspace(wsName)
			if err != nil {
				return err
			}

			if ws == nil {
				// Not a workspace — try as a session name (backward compat for `zmux attach <session>`).
				if app.Runner.HasSession(wsName) {
					return attachSession(app, openHijackFlag, openPinViewFlag, wsName)
				}
				return fmt.Errorf("workspace %q not found\n  Use: zmux new %s  (create it)", wsName, wsName)
			}

			// Determine which session to attach.
			var targetSession workspace.WorkspaceSession
			if requestedLabel != "" {
				var found bool
				targetSession, found = app.WorkspaceStore.SessionRecord(wsName, requestedLabel)
				if !found {
					return fmt.Errorf("session %q is not in workspace %q", requestedLabel, wsName)
				}
			} else {
				targetSession = resolveLastActive(app, ws)
			}

			if targetSession.TmuxName == "" {
				return fmt.Errorf("workspace %q has no live sessions\n  Use: zmux new %s  (create one)", wsName, wsName)
			}

			targetName := liveWorkspaceSessionTarget(app, wsName, targetSession)
			// Verify session exists in tmux.
			if !app.Runner.HasSession(targetName) {
				return fmt.Errorf("session %q not found in tmux", targetSession.Label)
			}

			// Update last active.
			_ = app.WorkspaceStore.SetLastActive(wsName, targetSession.ID)

			return attachSession(app, openHijackFlag, openPinViewFlag, targetName)
		},
	}
	cmd.Flags().BoolVar(&openHijackFlag, "hijack", false, "take over session from other client")
	cmd.Flags().BoolVar(&openPinViewFlag, "pin-view", false, "create a persistent grouped viewport instead of sharing the root view")
	return cmd
}

// resolveLastActive returns the best session to attach to in a workspace.
// Prefers last_active, falls back to first live session.
func resolveLastActive(app *apppkg.App, ws *workspace.Workspace) workspace.WorkspaceSession {
	if ws.LastActiveSessionID != "" {
		if rec, ok := app.WorkspaceStore.SessionRecord(ws.Name, ws.LastActiveSessionID); ok && app.Runner.HasSession(liveWorkspaceSessionTarget(app, ws.Name, rec)) {
			return rec
		}
	}
	// Fallback: first live session.
	for _, s := range ws.Sessions {
		if app.Runner.HasSession(liveWorkspaceSessionTarget(app, ws.Name, s)) {
			return s
		}
	}
	return workspace.WorkspaceSession{}
}

func liveWorkspaceSessionTarget(app *apppkg.App, wsName string, rec workspace.WorkspaceSession) string {
	if rec.TmuxName != "" && app.Runner.HasSession(rec.TmuxName) {
		return rec.TmuxName
	}
	if rec.LegacyTmuxName == "" || !app.Runner.HasSession(rec.LegacyTmuxName) {
		return rec.TmuxName
	}
	if rec.TmuxName == "" {
		return rec.LegacyTmuxName
	}
	if err := app.Runner.RenameSession(rec.LegacyTmuxName, rec.TmuxName); err != nil {
		return rec.LegacyTmuxName
	}
	if err := workspace.StampSessionMetadata(app.Runner, wsName, rec); err != nil {
		return rec.TmuxName
	}
	_ = app.WorkspaceStore.ClearLegacySessionName(wsName, rec.ID)
	return rec.TmuxName
}

// attachSession handles the attach logic: normal attach, auto-clone, pinned view, or hijack.
func attachSession(app *apppkg.App, hijack, pinView bool, name string) error {
	if pinView {
		return attachOwnedSessionWith(app, name, func(r tmux.Runner, target string) error {
			_, err := session.AttachPinnedView(r, target)
			return err
		})
	}
	if hijack {
		return attachOwnedSessionWith(app, name, session.AttachHijack)
	}
	// session.Attach already handles auto-cloning (creates grouped session
	// if already attached elsewhere).
	return attachOwnedSession(app, name)
}

func parseWorkspaceSessionArgs(args []string) (workspaceName, sessionLabel string, err error) {
	workspaceName = args[0]
	if strings.Count(workspaceName, "/") > 1 {
		return "", "", fmt.Errorf("target must be workspace/session")
	}
	if strings.Contains(workspaceName, "/") {
		parts := strings.SplitN(workspaceName, "/", 2)
		workspaceName, sessionLabel = parts[0], parts[1]
	}
	if len(args) == 2 {
		if sessionLabel != "" {
			return "", "", fmt.Errorf("pass either workspace/session or workspace session, not both")
		}
		sessionLabel = args[1]
	}
	if err := workspace.ValidateWorkspaceName(workspaceName); err != nil {
		return "", "", err
	}
	if sessionLabel != "" {
		if err := workspace.ValidateSessionLabel(sessionLabel); err != nil {
			return "", "", err
		}
	}
	return workspaceName, sessionLabel, nil
}
