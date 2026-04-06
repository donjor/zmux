package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/donjor/zmux/internal/session"
	"github.com/spf13/cobra"
)

var workspaceCmd = &cobra.Command{
	Use:     "workspace",
	Aliases: []string{"ws"},
	Short:   "Manage workspace tags",
	Long:    "Tag sessions to workspaces for grouped display in picker and dashboard.",
}

var wsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List workspaces with their sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cleanupWorkspaces(); err != nil {
			return err
		}

		workspaces := app.WorkspaceStore.Workspaces()
		if len(workspaces) == 0 {
			fmt.Println("No workspaces.")
			return nil
		}

		for _, ws := range workspaces {
			sessions := app.WorkspaceStore.SessionsIn(ws)
			fmt.Printf("  %s\n", ws)
			for _, s := range sessions {
				fmt.Printf("    %s\n", s)
			}
		}
		return nil
	},
}

var wsAddCmd = &cobra.Command{
	Use:   "add <workspace> <session>",
	Short: "Tag a session to a workspace",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cleanupWorkspaces(); err != nil {
			return err
		}

		ws := args[0]
		sess := args[1]
		root := session.RootName(sess)

		if err := app.WorkspaceStore.Set(root, ws); err != nil {
			return fmt.Errorf("set workspace: %w", err)
		}
		fmt.Printf("Tagged %s → %s\n", root, ws)
		return nil
	},
}

var wsRemoveCmd = &cobra.Command{
	Use:   "remove <session>",
	Short: "Remove a session from its workspace",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cleanupWorkspaces(); err != nil {
			return err
		}

		sess := args[0]
		root := session.RootName(sess)

		if err := app.WorkspaceStore.Delete(root); err != nil {
			return fmt.Errorf("remove workspace: %w", err)
		}
		fmt.Printf("Untagged %s\n", root)
		return nil
	},
}

var wsShowCmd = &cobra.Command{
	Use:   "show <workspace>",
	Short: "Show sessions in a workspace",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cleanupWorkspaces(); err != nil {
			return err
		}

		ws := args[0]
		sessions := app.WorkspaceStore.SessionsIn(ws)
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

// cleanupWorkspaces removes workspace entries for sessions that no longer
// exist in tmux. Called before every workspace command.
func cleanupWorkspaces() error {
	sessions, err := session.ListSessions(app.Runner)
	if err != nil {
		// tmux not running or error — skip cleanup to avoid data loss.
		return nil
	}

	liveRoots := make(map[string]bool, len(sessions))
	for _, s := range sessions {
		root := session.RootName(s.Name)
		liveRoots[root] = true
	}

	return app.WorkspaceStore.Cleanup(liveRoots)
}

// liveRootSet builds a set of root session names from the current tmux sessions.
// Returns nil if tmux is unavailable (callers should treat nil as "skip cleanup").
func liveRootSet() map[string]bool {
	sessions, err := session.ListSessions(app.Runner)
	if err != nil {
		return nil
	}
	roots := make(map[string]bool, len(sessions))
	for _, s := range sessions {
		roots[session.RootName(s.Name)] = true
	}
	return roots
}

var wsKillCmd = &cobra.Command{
	Use:   "kill <workspace>",
	Short: "Kill a workspace and all its sessions",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cleanupWorkspaces(); err != nil {
			return err
		}

		wsName := args[0]
		sessions := app.WorkspaceStore.SessionsIn(wsName)

		// Kill all live tmux sessions in this workspace.
		for _, sess := range sessions {
			_ = session.Kill(app.Runner, sess)
		}

		// Remove workspace from store.
		if err := app.WorkspaceStore.DeleteWorkspace(wsName); err != nil {
			return err
		}
		fmt.Printf("Killed workspace %q (%d sessions)\n", wsName, len(sessions))
		return nil
	},
}

var wsNextCmd = &cobra.Command{
	Use:    "next",
	Short:  "Switch to next session in current workspace",
	Hidden: true, // used by keybinding
	RunE: func(cmd *cobra.Command, args []string) error {
		return cycleWorkspaceSession(1)
	},
}

var wsPrevCmd = &cobra.Command{
	Use:    "prev",
	Short:  "Switch to previous session in current workspace",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cycleWorkspaceSession(-1)
	},
}

var wsSwitchToCmd = &cobra.Command{
	Use:    "switch-to <position>",
	Short:  "Switch to session at position in current workspace",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pos := 0
		fmt.Sscanf(args[0], "%d", &pos)
		return switchToWorkspacePosition(pos)
	},
}

func cycleWorkspaceSession(direction int) error {
	current, err := app.Runner.DisplayMessage("", "#{session_name}")
	if err != nil {
		return fmt.Errorf("not inside tmux")
	}
	current = strings.TrimSpace(current)
	root := session.RootName(current)

	wsName, ok := app.WorkspaceStore.WorkspaceFor(root)
	if !ok {
		return nil // not in a workspace
	}

	sessions := app.WorkspaceStore.SessionsIn(wsName)
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
	return app.Runner.SwitchClient(target)
}

func switchToWorkspacePosition(pos int) error {
	current, err := app.Runner.DisplayMessage("", "#{session_name}")
	if err != nil {
		return fmt.Errorf("not inside tmux")
	}
	root := session.RootName(strings.TrimSpace(current))

	wsName, ok := app.WorkspaceStore.WorkspaceFor(root)
	if !ok {
		return nil
	}

	sessions := app.WorkspaceStore.SessionsIn(wsName)
	idx := pos - 1 // 1-based to 0-based
	if idx < 0 || idx >= len(sessions) {
		return nil
	}

	target := sessions[idx]
	_ = app.WorkspaceStore.SetLastActive(wsName, target)
	return app.Runner.SwitchClient(target)
}

func init() {
	workspaceCmd.AddCommand(wsListCmd)
	workspaceCmd.AddCommand(wsAddCmd)
	workspaceCmd.AddCommand(wsRemoveCmd)
	workspaceCmd.AddCommand(wsShowCmd)
	workspaceCmd.AddCommand(wsKillCmd)
	workspaceCmd.AddCommand(wsNextCmd)
	workspaceCmd.AddCommand(wsPrevCmd)
	workspaceCmd.AddCommand(wsSwitchToCmd)
	rootCmd.AddCommand(workspaceCmd)
}

// ── Helpers used by picker/dashboard for workspace grouping ──

// workspaceGroups partitions sessions by workspace for display. Returns:
//   - groups: workspace name → sessions (sorted by name)
//   - untagged: sessions not in any workspace (non-tmp)
//   - tmp: temporary sessions
//
// Grouped sessions (dev-b) inherit their root's workspace.
func workspaceGroups(
	sessions []session.SessionInfo,
	wsState map[string]string,
) (groups map[string][]session.SessionInfo, order []string, untagged, tmp []session.SessionInfo) {
	groups = make(map[string][]session.SessionInfo)
	seen := make(map[string]bool)

	for _, s := range sessions {
		if s.IsTmp {
			tmp = append(tmp, s)
			continue
		}

		root := session.RootName(s.Name)
		ws, ok := wsState[root]
		if ok {
			groups[ws] = append(groups[ws], s)
			if !seen[ws] {
				seen[ws] = true
				order = append(order, ws)
			}
		} else {
			untagged = append(untagged, s)
		}
	}

	sort.Strings(order)

	// Sort sessions within each group by name.
	for ws := range groups {
		g := groups[ws]
		sort.Slice(g, func(i, j int) bool { return g[i].Name < g[j].Name })
		groups[ws] = g
	}

	// Sort untagged by name.
	sort.Slice(untagged, func(i, j int) bool { return untagged[i].Name < untagged[j].Name })

	return groups, order, untagged, tmp
}

// FormatWorkspaceList produces a simple text listing of workspaces for CLI output.
func FormatWorkspaceList(groups map[string][]session.SessionInfo, order []string) string {
	var b strings.Builder
	for _, ws := range order {
		sessions := groups[ws]
		b.WriteString(fmt.Sprintf("  %s\n", ws))
		for _, s := range sessions {
			b.WriteString(fmt.Sprintf("    %s\n", s.Name))
		}
	}
	return b.String()
}
