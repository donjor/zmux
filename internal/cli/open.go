package cli

import (
	"fmt"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/workspace"
	"github.com/spf13/cobra"
)

func newOpenCmd(app *apppkg.App) *cobra.Command {
	var openHijackFlag bool

	cmd := &cobra.Command{
		Use:     "open <workspace> [session]",
		Aliases: []string{"attach", "a"},
		Short:   "Open a workspace or attach to a session",
		Long: `Open a workspace — attach to its last-active session.

  zmux open <workspace>             Attach last-active session in workspace
  zmux open <workspace> <session>   Attach specific session in workspace

If the target session is already attached elsewhere, a clone
(independent viewport) is created automatically. Use --hijack
to take over instead.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			wsName := args[0]

			// Look up workspace.
			ws, err := app.WorkspaceStore.GetWorkspace(wsName)
			if err != nil {
				return err
			}

			if ws == nil {
				// Not a workspace — try as a session name (backward compat for `zmux attach <session>`).
				if app.Runner.HasSession(wsName) {
					return attachSession(app, openHijackFlag, wsName)
				}
				return fmt.Errorf("workspace %q not found\n  Use: zmux new %s  (create it)", wsName, wsName)
			}

			// Determine which session to attach.
			var targetSession string
			if len(args) >= 2 {
				targetSession = args[1]
				// Verify session belongs to this workspace.
				found := false
				for _, s := range ws.Sessions {
					if s == targetSession {
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("session %q is not in workspace %q", targetSession, wsName)
				}
			} else {
				targetSession = resolveLastActive(app, ws)
			}

			if targetSession == "" {
				return fmt.Errorf("workspace %q has no live sessions\n  Use: zmux new %s  (create one)", wsName, wsName)
			}

			// Verify session exists in tmux.
			if !app.Runner.HasSession(targetSession) {
				return fmt.Errorf("session %q not found in tmux", targetSession)
			}

			// Update last active.
			_ = app.WorkspaceStore.SetLastActive(wsName, targetSession)

			return attachSession(app, openHijackFlag, targetSession)
		},
	}
	cmd.Flags().BoolVar(&openHijackFlag, "hijack", false, "take over session from other client")
	return cmd
}

// resolveLastActive returns the best session to attach to in a workspace.
// Prefers last_active, falls back to first live session.
func resolveLastActive(app *apppkg.App, ws *workspace.Workspace) string {
	if ws.LastActiveSession != "" && app.Runner.HasSession(ws.LastActiveSession) {
		return ws.LastActiveSession
	}
	// Fallback: first live session.
	for _, s := range ws.Sessions {
		if app.Runner.HasSession(s) {
			return s
		}
	}
	return ""
}

// nextSessionName returns a session name that doesn't collide with an
// existing tmux session. Prefers "<base>-<workspace>" then "<base>-N".
func nextSessionName(app *apppkg.App, base, workspace string) string {
	candidate := base + "-" + workspace
	if !app.Runner.HasSession(candidate) {
		return candidate
	}
	for i := 2; i < 100; i++ {
		candidate = fmt.Sprintf("%s-%d", base, i)
		if !app.Runner.HasSession(candidate) {
			return candidate
		}
	}
	return base + "-" + session.NextTmpName(app.Runner)
}

// attachSession handles the attach logic: normal attach, auto-clone, or hijack.
func attachSession(app *apppkg.App, hijack bool, name string) error {
	if hijack {
		return session.AttachHijack(app.Runner, name)
	}
	// session.Attach already handles auto-cloning (creates grouped session
	// if already attached elsewhere).
	return session.Attach(app.Runner, name)
}
