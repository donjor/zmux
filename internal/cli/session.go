package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tablabel"
	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/workspace"
	"github.com/spf13/cobra"
)

func newSessionCmd(app *apppkg.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Manage sessions",
	}
	cmd.AddCommand(newSessionKillCmd(app))
	cmd.AddCommand(newSessionRunCmd(app))
	return cmd
}

func newSessionKillCmd(app *apppkg.App) *cobra.Command {
	return &cobra.Command{
		Use:   "kill <session>",
		Short: "Kill a session and clean up workspace membership",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sessName := args[0]
			target, err := resolveSessionTarget(app, sessName)
			if err != nil {
				return err
			}

			if err := workspace.KillSession(app.Runner, app.WorkspaceStore, target); err != nil {
				return err
			}
			fmt.Printf("Killed session %q\n", sessName)
			return nil
		},
	}
}

// newSessionRunCmd implements `zmux session run` — create a detached session in
// a workspace and launch a command as its first/only tab. Unlike `zmux new`
// (attach-by-contract, births a blank shell tab), this never steals focus and
// never leaves a blank tab: the command IS window 1. It is the orchestration-
// safe primitive for spawning unattended worker sessions (report 009).
func newSessionRunCmd(app *apppkg.App) *cobra.Command {
	var tabName string
	var wsFlag string
	var cwdFlag string

	cmd := &cobra.Command{
		Use:   "run <session> -n <tab> -- <command...>",
		Short: "Create a detached session and run a command as its first tab",
		Long: `Create a new detached session and launch a command as its first/only tab.

Unlike "zmux new" (which attaches and births a blank shell tab), "session run"
never steals focus and never leaves a blank tab — the command is the first
window. Built for spawning unattended worker sessions under the current (or a
named) workspace.

  zmux session run worker-auth -n auth-worker -- codex -C /path/to/wt -s workspace-write -a on-request
  zmux session run worker-auth --workspace dev -n auth-worker -- <cli launch>

The command must follow "--" so its own flags are not parsed by zmux. The new
session is tagged into the workspace but never made the default attach target.`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// The command must come after `--`, so a worker's own flags
			// (-C, -s, -a, …) are never parsed by cobra. ArgsLenAtDash == 1
			// means exactly the session positional preceded `--`.
			if cmd.ArgsLenAtDash() != 1 {
				return fmt.Errorf("usage: zmux session run <session> -n <tab> -- <command...> (the command must follow `--`)")
			}
			sessionLabel := args[0]
			if err := workspace.ValidateSessionLabel(sessionLabel); err != nil {
				return fmt.Errorf("invalid session label %q: %w", sessionLabel, err)
			}
			if strings.TrimSpace(tabName) == "" {
				return fmt.Errorf("a tab name is required: pass -n <tab> (worker tabs are never name-derived)")
			}
			// Faithfully reconstruct the command from post-`--` argv; a raw
			// space-join would corrupt quoting (e.g. `-- bash -lc 'a; b'`).
			command := tmux.ShellCommand(args[1:])

			wsName, wsRootDir, err := resolveSessionRunWorkspace(app, wsFlag)
			if err != nil {
				return err
			}
			dir := resolveSessionRunDir(app, cwdFlag, wsRootDir)
			rec, err := workspace.NewSessionRecord(wsName, sessionLabel)
			if err != nil {
				return err
			}

			// Hard-error if the session exists — a worker must not silently
			// reuse a live session. Check before any tmux/workspace mutation.
			if app.Runner.HasSession(rec.TmuxName) {
				return fmt.Errorf("session %q already exists in workspace %q — choose a fresh name or reap it first", sessionLabel, wsName)
			}

			// Create the detached session with the command as its named first
			// window. No attach/switch — the caller's focus stays put.
			paneID, err := app.Runner.NewSessionWindow(rec.TmuxName, tabName, dir)
			if err != nil {
				return fmt.Errorf("create session: %w", err)
			}
			if err := workspace.StampSessionMetadata(app.Runner, wsName, rec); err != nil {
				_ = app.Runner.KillSession(rec.TmuxName)
				return err
			}

			// Tag into the workspace. Never SetLastActive — a background worker
			// must not become the workspace's default attach target. Roll the
			// session back if tagging fails so a half-built one never lingers.
			if err := app.WorkspaceStore.AddSessionRecord(wsName, rec); err != nil {
				_ = app.Runner.KillSession(rec.TmuxName)
				return fmt.Errorf("tag session to workspace %q: %w", wsName, err)
			}

			// Stamp identity at birth: pane-scoped id + pane-canonical label, so
			// a later `type`/`watch`/`send -s <session>` finds the logical tab.
			target := paneID
			if target == "" {
				target = fmt.Sprintf("%s:%s", rec.TmuxName, tabName)
			} else {
				if _, err := tabs.Stamp(app.Runner, paneID, paneID, tabName, tablabel.SourcePane); err != nil {
					return fmt.Errorf("stamp tab: %w", err)
				}
				// Worker sessions are agent-spawned and orchestrate-owned — the
				// tab reaper must never auto-kill them (scope=worker). (plan 038)
				_ = tabs.StampBirth(app.Runner, paneID, tabs.OriginAgent, tabs.ScopeWorker, time.Now())
			}

			// Command lifecycle glyphs are owned by the target shell's
			// shell-event hooks; session run only delivers the command.
			sendCmd := command
			if !isSimpleCommand(command) {
				scriptPath, cleanup, werr := writeCommandScript(command, 0)
				if werr != nil {
					return fmt.Errorf("write command script: %w", werr)
				}
				if cleanup != nil {
					defer cleanup()
				}
				sendCmd = fmt.Sprintf("bash %s", scriptPath)
			}
			if err := sendShellLine(app.Runner, target, sendCmd); err != nil {
				return fmt.Errorf("send to %s: %w", target, err)
			}

			fmt.Fprintf(os.Stderr, "created session %s/%s, running in %s:%s\n", wsName, sessionLabel, rec.TmuxName, tabName)
			return nil
		},
	}

	cmd.Flags().StringVarP(&tabName, "name", "n", "", "tab name for the worker (required)")
	cmd.Flags().StringVar(&wsFlag, "workspace", "", "target workspace (default: current session's workspace)")
	cmd.Flags().StringVar(&cwdFlag, "cwd", "", "working directory (default: current dir, then workspace root)")
	return cmd
}

// resolveSessionRunWorkspace returns the target workspace name + its root dir
// for a new worker session. An explicit --workspace must already exist — no
// surprise workspace creation, which is exactly the failure report 009 fixed.
// Otherwise the current session's workspace is used, which requires being
// inside tmux.
func resolveSessionRunWorkspace(app *apppkg.App, wsFlag string) (name, rootDir string, err error) {
	if strings.TrimSpace(wsFlag) != "" {
		ws, gerr := app.WorkspaceStore.GetWorkspace(wsFlag)
		if gerr != nil {
			return "", "", gerr
		}
		if ws == nil {
			return "", "", fmt.Errorf("workspace %q does not exist (pass an existing workspace)", wsFlag)
		}
		return wsFlag, ws.RootDir, nil
	}
	if !app.Runner.IsInsideTmux() {
		return "", "", fmt.Errorf("outside tmux — pass --workspace to target a workspace")
	}
	current, _ := app.Runner.DisplayMessage("", "#{session_name}")
	root := session.RootName(strings.TrimSpace(current))
	wsName, ok := app.WorkspaceStore.WorkspaceFor(root)
	if !ok {
		return "", "", fmt.Errorf("current session %q is not in a workspace — pass --workspace", root)
	}
	ws, gerr := app.WorkspaceStore.GetWorkspace(wsName)
	if gerr != nil {
		return "", "", gerr
	}
	if ws != nil {
		rootDir = ws.RootDir
	}
	return wsName, rootDir, nil
}

// resolveSessionRunDir picks the new session's working directory: --cwd wins,
// then (inside tmux) the caller's current pane dir, then the workspace root,
// then the process cwd.
func resolveSessionRunDir(app *apppkg.App, cwdFlag, wsRootDir string) string {
	if d := strings.TrimSpace(cwdFlag); d != "" {
		return d
	}
	if app.Runner.IsInsideTmux() {
		if cur, err := app.Runner.DisplayMessage("", "#{pane_current_path}"); err == nil {
			if d := strings.TrimSpace(cur); d != "" {
				return d
			}
		}
	}
	if d := strings.TrimSpace(wsRootDir); d != "" {
		return d
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "."
}
