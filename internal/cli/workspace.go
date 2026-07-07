package cli

import (
	"fmt"
	"strings"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tui/workspaceview"
	workspacepkg "github.com/donjor/zmux/internal/workspace"
	"github.com/spf13/cobra"
)

func newWorkspaceCmd(app *apppkg.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "workspace",
		Aliases: []string{"ws"},
		Short:   "Manage workspace tags",
		Long:    "Tag sessions to workspaces for grouped display in picker and dashboard.",
	}
	cmd.AddCommand(newWsListCmd(app))
	cmd.AddCommand(newWsAddCmd(app))
	cmd.AddCommand(newWsRemoveCmd(app))
	cmd.AddCommand(newWsShowCmd(app))
	cmd.AddCommand(newWsKillCmd(app))
	cmd.AddCommand(newWsNextCmd(app))
	cmd.AddCommand(newWsPrevCmd(app))
	cmd.AddCommand(newWsSwitchToCmd(app))
	cmd.AddCommand(newWsNewSessionCmd(app))
	return cmd
}

func newWsListCmd(app *apppkg.App) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List workspaces with their sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cleanupWorkspaces(app); err != nil {
				return err
			}

			workspaces := app.WorkspaceStore.Workspaces()
			if len(workspaces) == 0 {
				fmt.Println("No workspaces.")
				return nil
			}

			for _, ws := range workspaces {
				sessions := app.WorkspaceStore.SessionLabelsIn(ws)
				fmt.Printf("  %s\n", ws)
				for _, s := range sessions {
					fmt.Printf("    %s\n", s)
				}
			}
			return nil
		},
	}
}

func newWsAddCmd(app *apppkg.App) *cobra.Command {
	return &cobra.Command{
		Use:   "add <workspace> <session>",
		Short: "Tag a session to a workspace",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cleanupWorkspaces(app); err != nil {
				return err
			}

			ws := args[0]
			sess := args[1]
			root := session.RootName(sess)

			if err := app.WorkspaceStore.MoveSession(root, ws); err != nil {
				return fmt.Errorf("set workspace: %w", err)
			}
			fmt.Printf("Tagged %s → %s\n", root, ws)
			return nil
		},
	}
}

func newWsRemoveCmd(app *apppkg.App) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <session>",
		Short: "Remove a session from its workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cleanupWorkspaces(app); err != nil {
				return err
			}

			sess := args[0]
			root := session.RootName(sess)

			if err := app.WorkspaceStore.RemoveSession(root); err != nil {
				return fmt.Errorf("remove workspace: %w", err)
			}
			fmt.Printf("Untagged %s\n", root)
			return nil
		},
	}
}

func newWsShowCmd(app *apppkg.App) *cobra.Command {
	return &cobra.Command{
		Use:   "show <workspace>",
		Short: "Show sessions in a workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cleanupWorkspaces(app); err != nil {
				return err
			}

			ws := args[0]
			sessions := app.WorkspaceStore.SessionLabelsIn(ws)
			if len(sessions) == 0 {
				fmt.Printf("Workspace %q has no sessions.\n", ws)
				return nil
			}
			fmt.Printf("  %s\n", ws)
			for _, s := range sessions {
				fmt.Printf("    %s\n", s)
			}
			return nil
		},
	}
}

// liveRootSet builds a set of root session names from the current tmux sessions.
// Returns nil if tmux is unavailable (callers should treat nil as "skip cleanup").
func liveRootSet(app *apppkg.App) map[string]bool {
	sessions, err := session.ListSessions(app.Runner)
	if err != nil {
		return nil
	}
	roots := make(map[string]bool, len(sessions))
	for _, s := range sessions {
		name := repairManagedSessionName(app, s)
		roots[session.RootName(name)] = true
	}
	return roots
}

func repairManagedSessionName(app *apppkg.App, s session.SessionInfo) string {
	if !s.Managed || s.Workspace == "" || s.Label == "" {
		return repairLegacySessionName(app, s.Name)
	}
	expected := workspacepkg.RawSessionName(s.Workspace, s.Label)
	if s.Name == expected {
		return s.Name
	}
	if app.Runner.HasSession(expected) {
		return s.Name
	}
	if err := app.Runner.RenameSession(s.Name, expected); err != nil {
		return s.Name
	}
	rec, err := workspacepkg.NewSessionRecord(s.Workspace, s.Label)
	if err == nil {
		_ = workspacepkg.StampSessionMetadata(app.Runner, s.Workspace, rec)
	}
	return expected
}

func repairLegacySessionName(app *apppkg.App, raw string) string {
	if wsName, rec, ok := app.WorkspaceStore.SessionRecordForTmuxName(raw); ok {
		if err := workspacepkg.StampSessionMetadata(app.Runner, wsName, rec); err == nil {
			_ = app.WorkspaceStore.ClearLegacySessionName(wsName, rec.ID)
		}
		return raw
	}

	wsName, rec, ok := app.WorkspaceStore.LegacySessionRecordFor(raw)
	if !ok || rec.TmuxName == "" || rec.TmuxName == raw {
		return raw
	}
	if app.Runner.HasSession(rec.TmuxName) {
		return raw
	}
	if err := app.Runner.RenameSession(raw, rec.TmuxName); err != nil {
		return raw
	}
	if err := workspacepkg.StampSessionMetadata(app.Runner, wsName, rec); err != nil {
		return rec.TmuxName
	}
	_ = app.WorkspaceStore.ClearLegacySessionName(wsName, rec.ID)
	return rec.TmuxName
}

// workspaceViewOptions tunes loadWorkspaceView per browse surface.
type workspaceViewOptions struct {
	// Reconcile prunes dead sessions against the live tmux roots before loading.
	// The picker and workspace switcher do this; the dashboard does not (it
	// refreshes through its own fetch cycle).
	Reconcile bool
	// HidePseudo drops pseudo-workspaces (e.g. "temporary"). The workspace
	// switcher hides them since "switch to temporary" has no useful target.
	HidePseudo bool
}

// loadWorkspaceView builds the workspace view models shared by the primary
// picker, the dashboard, and the M-w workspace switcher. The three surfaces
// differ only in whether they reconcile first and whether they hide
// pseudo-workspaces — captured by workspaceViewOptions — so the load/build core
// lives here once instead of in three near-identical closures.
func loadWorkspaceView(app *apppkg.App, opts workspaceViewOptions) []workspaceview.WorkspaceViewModel {
	if opts.Reconcile {
		if roots := liveRootSet(app); roots != nil {
			_ = app.WorkspaceStore.Reconcile(roots)
		}
	}
	workspaces, err := app.WorkspaceStore.ListWorkspaces()
	if err != nil {
		return nil
	}
	sessions, _ := session.ListSessions(app.Runner)
	all := workspaceview.BuildWorkspaceViewModels(workspaces, sessions)
	if !opts.HidePseudo {
		return all
	}
	out := all[:0]
	for _, ws := range all {
		if ws.IsPseudo {
			continue
		}
		out = append(out, ws)
	}
	return out
}

// cleanupWorkspaces removes workspace entries for sessions that no longer
// exist in tmux. Called before every workspace command. No-op if tmux is
// unavailable (to avoid wiping state during outages).
func cleanupWorkspaces(app *apppkg.App) error {
	roots := liveRootSet(app)
	if roots == nil {
		return nil
	}
	return app.WorkspaceStore.Reconcile(roots)
}

func newWsKillCmd(app *apppkg.App) *cobra.Command {
	var assumeYes bool
	cmd := &cobra.Command{
		Use:   "kill <workspace>",
		Short: "Kill a workspace and all its sessions",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cleanupWorkspaces(app); err != nil {
				return err
			}

			ws, err := app.WorkspaceStore.GetWorkspace(args[0])
			if err != nil {
				return err
			}
			if ws == nil {
				return fmt.Errorf("workspace %q not found", args[0])
			}
			return killWorkspace(app, ws, assumeYes)
		},
	}
	cmd.Flags().BoolVarP(&assumeYes, "yes", "y", false, "skip the kill confirmation prompt")
	return cmd
}

func newWsNextCmd(app *apppkg.App) *cobra.Command {
	return &cobra.Command{
		Use:    "next",
		Short:  "Switch to next session in current workspace",
		Hidden: true, // used by keybinding
		RunE: func(cmd *cobra.Command, args []string) error {
			return cycleWorkspaceSession(app, 1)
		},
	}
}

func newWsPrevCmd(app *apppkg.App) *cobra.Command {
	return &cobra.Command{
		Use:    "prev",
		Short:  "Switch to previous session in current workspace",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cycleWorkspaceSession(app, -1)
		},
	}
}

func newWsSwitchToCmd(app *apppkg.App) *cobra.Command {
	return &cobra.Command{
		Use:    "switch-to <position>",
		Short:  "Switch to session at position in current workspace",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pos := 0
			_, _ = fmt.Sscanf(args[0], "%d", &pos)
			return switchToWorkspacePosition(app, pos)
		},
	}
}

func cycleWorkspaceSession(app *apppkg.App, direction int) error {
	root, err := currentSessionName(app)
	if err != nil {
		return err
	}

	wsName, ok := app.WorkspaceStore.WorkspaceFor(root)
	if !ok {
		return nil // not in a workspace
	}

	sessions := app.WorkspaceStore.SessionTargetsIn(wsName)
	if len(sessions) <= 1 {
		return nil
	}

	idx := -1
	for i, s := range sessions {
		if s == root {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil
	}

	next := (idx + direction + len(sessions)) % len(sessions)
	target := sessions[next]

	_ = app.WorkspaceStore.SetLastActive(wsName, target)
	_, serr := session.SwitchView(app.Runner, target)
	return serr
}

func switchToWorkspacePosition(app *apppkg.App, pos int) error {
	root, err := currentSessionName(app)
	if err != nil {
		return err
	}

	wsName, ok := app.WorkspaceStore.WorkspaceFor(root)
	if !ok {
		return nil
	}

	sessions := app.WorkspaceStore.SessionTargetsIn(wsName)
	idx := pos - 1 // 1-based to 0-based
	if idx < 0 || idx >= len(sessions) {
		return nil
	}

	target := sessions[idx]
	_ = app.WorkspaceStore.SetLastActive(wsName, target)
	_, serr := session.SwitchView(app.Runner, target)
	return serr
}

func newWsNewSessionCmd(app *apppkg.App) *cobra.Command {
	return &cobra.Command{
		Use:    "new-session <name>",
		Short:  "Create a new session in the current workspace",
		Hidden: true, // used by prefix+C keybinding
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sessName := strings.TrimSpace(args[0])
			if err := session.ValidateName(sessName); err != nil {
				return err
			}
			if sessName == "" {
				return fmt.Errorf("session name cannot be empty")
			}

			root, err := currentSessionName(app)
			if err != nil {
				return err
			}

			wsName, ok := app.WorkspaceStore.WorkspaceFor(root)
			if !ok {
				return fmt.Errorf("current session is not in a workspace")
			}

			dir, _ := app.Runner.DisplayMessage("", "#{pane_current_path}")
			dir = strings.TrimSpace(dir)
			if dir == "" {
				dir = "."
			}

			rec, err := createWorkspaceSession(app, wsName, sessName, dir)
			if err != nil {
				return err
			}
			_ = app.WorkspaceStore.SetLastActive(wsName, rec.ID)
			return app.Runner.SwitchClient(rec.TmuxName)
		},
	}
}
