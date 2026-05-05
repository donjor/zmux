package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/donjor/zmux/internal/session"
	"github.com/spf13/cobra"
)

var lsSessionsFlag bool

var lsCmd = &cobra.Command{
	Use:     "ls [workspace]",
	Aliases: []string{"list"},
	Short:   "List workspaces or sessions",
	Long: `List workspaces (default) or sessions within a workspace.

  zmux ls              List all workspaces with session counts
  zmux ls myapp        List sessions within workspace 'myapp'
  zmux ls -s           List all sessions (flat, legacy mode)`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if lsSessionsFlag {
			return lsSessionsFlat()
		}

		if len(args) == 1 {
			return lsWorkspaceSessions(args[0])
		}

		return lsWorkspaces()
	},
}

func lsWorkspaces() error {
	// Reconcile first.
	if roots := liveRootSet(); roots != nil {
		_ = app.WorkspaceStore.Reconcile(roots)
	}

	workspaces, err := app.WorkspaceStore.ListWorkspaces()
	if err != nil {
		return err
	}

	if len(workspaces) == 0 {
		fmt.Println("No workspaces.")
		return nil
	}

	sessions, _ := session.ListSessions(app.Runner)
	sessionMap := make(map[string]session.SessionInfo)
	for _, s := range sessions {
		sessionMap[s.Name] = s
	}

	for _, ws := range workspaces {
		// Count live sessions; track most recent activity (max time).
		liveCount := 0
		var maxActivity time.Time
		hasAttached := false
		for _, sessName := range ws.Sessions {
			if s, ok := sessionMap[sessName]; ok {
				liveCount++
				if s.Attached {
					hasAttached = true
				}
				if s.Activity.After(maxActivity) {
					maxActivity = s.Activity
				}
			}
		}

		countStr := fmt.Sprintf("%d sessions", liveCount)
		if liveCount == 1 {
			countStr = "1 session"
		}

		lastActivity := ""
		if !maxActivity.IsZero() {
			lastActivity = session.HumanAge(maxActivity) + " ago"
		}

		dir := shortenPathCLI(ws.RootDir)

		attachedTag := ""
		if hasAttached {
			attachedTag = "  attached"
		}

		fmt.Printf("  %-16s %-12s %-20s %s%s\n", ws.Name, countStr, dir, lastActivity, attachedTag)
	}

	return nil
}

func lsWorkspaceSessions(wsName string) error {
	ws, err := app.WorkspaceStore.GetWorkspace(wsName)
	if err != nil {
		return err
	}
	if ws == nil {
		return fmt.Errorf("workspace %q not found", wsName)
	}

	if len(ws.Sessions) == 0 {
		fmt.Printf("Workspace %q has no sessions.\n", wsName)
		return nil
	}

	// Fetch all live sessions once and index by name.
	sessions, _ := session.ListSessions(app.Runner)
	sessionMap := make(map[string]*session.SessionInfo, len(sessions))
	for i := range sessions {
		sessionMap[sessions[i].Name] = &sessions[i]
	}

	// Detect current session if inside tmux.
	currentSession := ""
	currentTab := ""
	if app.Runner.IsInsideTmux() {
		currentSession, _ = app.Runner.DisplayMessage("", "#{session_name}")
		currentSession = session.RootName(currentSession)
		currentTab, _ = app.Runner.DisplayMessage("", "#{window_name}")
	}

	for _, sessName := range ws.Sessions {
		s, alive := sessionMap[sessName]
		if !alive {
			fmt.Printf("  ○ %-14s (dead)\n", sessName)
			continue
		}

		icon := "○"
		if sessName == currentSession {
			icon = "◆"
		} else if s.Attached {
			icon = "●"
		}

		// Window names.
		winStr := fmt.Sprintf("%dw", s.Windows)
		if wins, err := app.Runner.ListWindows(sessName); err == nil && len(wins) > 0 {
			names := make([]string, len(wins))
			for i, w := range wins {
				names[i] = w.Name
			}
			winStr = fmt.Sprintf("[%s]", strings.Join(names, ", "))
		}

		marker := ""
		if sessName == currentSession && currentTab != "" {
			marker = fmt.Sprintf("  ← you (%s)", currentTab)
		}

		fmt.Printf("  %s %-14s %s%s\n", icon, sessName, winStr, marker)
	}

	return nil
}

func lsSessionsFlat() error {
	sessions, err := session.ListSessions(app.Runner)
	if err != nil {
		fmt.Println("No sessions.")
		return nil
	}
	if len(sessions) == 0 {
		fmt.Println("No sessions.")
		return nil
	}

	currentSession := ""
	currentTab := ""
	if app.Runner.IsInsideTmux() {
		currentSession, _ = app.Runner.DisplayMessage("", "#{session_name}")
		currentSession = session.RootName(currentSession)
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
}

func shortenPathCLI(path string) string {
	home, _ := app.FS.UserHomeDir()
	if home != "" && len(path) > len(home) && path[:len(home)] == home {
		path = "~" + path[len(home):]
	}
	return path
}

func init() {
	lsCmd.Flags().BoolVarP(&lsSessionsFlag, "sessions", "s", false, "list all sessions (flat)")
	rootCmd.AddCommand(lsCmd)
}
