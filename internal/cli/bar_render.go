package cli

import (
	"fmt"
	"strconv"
	"time"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/bar"
	"github.com/donjor/zmux/internal/session"
	"github.com/spf13/cobra"
)

func newBarRenderCmd(app *apppkg.App, barCmd *cobra.Command) *cobra.Command {
	var barRenderDir string
	var barRenderSession string
	var barRenderPaneCmd string
	var barRenderPrefix string
	var barRenderGroup string
	var barRenderGroupSize string
	var barRenderTopBar string

	cmd := &cobra.Command{
		Use:    "bar-render <left|right>",
		Short:  "Render a status bar segment (used by tmux #())",
		Long:   `Internal command called by tmux's #() shell execution to render dynamic status bar content.`,
		Args:   cobra.ExactArgs(1),
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			side := args[0]

			palette := loadPaletteOrDefault(app.FS)

			// Tmux state is passed via flags (substituted per-client inside
			// #() by tmux itself). Querying tmux here would return the
			// globally-focused client's state — wrong when multiple clients
			// are attached to different sessions.
			sessionName := barRenderSession
			if sessionName == "" {
				sessionName, _ = app.Runner.DisplayMessage("", "#{session_name}")
			}
			groupID := barRenderGroup
			if groupID == "" {
				groupID, _ = app.Runner.DisplayMessage("", "#{session_group}")
			}

			if side == "tabs" {
				// Logical tab row: one scan, rendered for the RAW session (a
				// grouped clone has its own current-window pointer) with
				// docked-tab origins matched against the root name (hide
				// records the root — clone-block keeps clones away from
				// placement verbs). Preset comes from config (same per-call
				// load as left/right); prefix arrives as a flag because tmux
				// #{?} conditionals don't expand inside #() output.
				rows, err := app.Runner.ListLogicalPaneRows()
				if err != nil {
					return nil // empty row beats a broken bar
				}
				originScope := sessionName
				if groupID != "" {
					originScope = session.RootName(sessionName)
				}
				cfg, _ := loadConfig(app.FS)
				preset, _ := bar.PresetFromString(cfg.Bar.Preset)
				fmt.Print(bar.RenderTabsRow(palette, preset, sessionName, originScope, rows, barRenderPrefix == "1", time.Now()))
				return nil
			}

			// Resolve grouped session clones (e.g. "dev-b") to their root
			// name ("dev") so the bar displays the real session, not the
			// multi-viewport clone. Extract the viewport letter first.
			var viewportID string
			if groupID != "" {
				root := session.RootName(sessionName)
				if sessionName == root {
					viewportID = "a"
				} else {
					viewportID = string(sessionName[len(sessionName)-1])
				}
				sessionName = root
			}

			paneCmd := barRenderPaneCmd
			if paneCmd == "" {
				paneCmd, _ = app.Runner.DisplayMessage("", "#{pane_current_command}")
			}
			prefixStr := barRenderPrefix
			if prefixStr == "" {
				prefixStr, _ = app.Runner.DisplayMessage("", "#{client_prefix}")
			}
			paneDir := barRenderDir
			if paneDir == "" {
				paneDir, _ = app.Runner.DisplayMessage("", "#{pane_current_path}")
			}

			// Workspace lookup.
			workspace, _ := app.WorkspaceStore.WorkspaceFor(sessionName)

			// Workspace position for status bar display.
			wsPos, wsCount, _ := app.WorkspaceStore.SessionPosition(sessionName)

			// Get preset from config.
			cfg, _ := loadConfig(app.FS)
			preset, _ := bar.PresetFromString(cfg.Bar.Preset)

			// Gather context + apply segment visibility from config.
			ctx := bar.GatherContext(bar.ExecProber{}, sessionName, paneDir, paneCmd, prefixStr, groupID, workspace)
			ctx.WorkspacePos = wsPos
			ctx.WorkspaceCount = wsCount
			ctx.ShowWorkspace = cfg.Bar.Segments.Workspace
			ctx.ShowGit = cfg.Bar.Segments.Git
			ctx.ShowLang = cfg.Bar.Segments.Lang
			ctx.ShowClock = cfg.Bar.Segments.Clock
			ctx.ShowDirectory = cfg.Bar.Segments.Directory
			ctx.ShowProcess = cfg.Bar.Segments.Process
			ctx.ShowGroup = cfg.Bar.Segments.Group

			// Group: viewport letter + attached count.
			ctx.ViewportID = viewportID
			if n, err := strconv.Atoi(barRenderGroupSize); err == nil {
				ctx.Attached = n
			}

			// Apply session indicator from config.
			switch cfg.Bar.Indicator {
			case "dots":
				if workspace != "" {
					sessions := app.WorkspaceStore.SessionsIn(workspace)
					states := workspaceAttachStates(app, sessions, sessionName)
					ctx.SessionIndicator = bar.CompactDots(sessions, sessionName, states)
				}
			case "none":
				ctx.WorkspaceCount = 1 // suppress N/M in SessionLabel
				// "numbers": default behavior — SessionLabel adds N/M
			}

			switch side {
			case "left":
				// In two-line mode the top row owns workspace/session identity,
				// so the bottom-left renders compact aux only (plan 024).
				ctx.TopRowActive = cfg.Bar.Layout == "two-line" || cfg.Bar.Layout == "split"
				fmt.Print(bar.RenderLeft(palette, ctx, preset))
			case "right":
				fmt.Print(bar.RenderRight(palette, ctx, preset))
			case "top":
				// Fetch workspace sessions for the top row. Untracked
				// sessions (no workspace) still get a one-session top row
				// so always-2-line never shows a blank top (plan 024).
				if workspace != "" {
					ctx.WorkspaceSessions = app.WorkspaceStore.SessionsIn(workspace)
					ctx.WorkspaceSessionStates = workspaceAttachStates(app, ctx.WorkspaceSessions, sessionName)
				}
				if len(ctx.WorkspaceSessions) == 0 && sessionName != "" {
					ctx.WorkspaceSessions = []string{sessionName}
				}
				topVariant := barRenderTopBar
				if topVariant == "" {
					topVariant = cfg.Bar.TopBar
				}
				if topVariant == "" {
					topVariant = "tabs"
				}
				fmt.Print(bar.RenderTopRow(palette, ctx, preset, topVariant))
				// Right-aligned overlay: cwd plus non-default profile badge
				// (e.g. zzmux). Keep both off the bottom tabs row.
				fmt.Print(bar.RenderTopOverlay(palette, ctx, app.Profile.Name))
			default:
				return fmt.Errorf("unknown side %q (use 'left', 'right', 'top', or 'tabs')", side)
			}

			return nil
		},
	}

	registerFlags := func(c *cobra.Command) {
		c.Flags().StringVar(&barRenderDir, "dir", "", "pane directory (avoids tmux query, enables per-window cache)")
		c.Flags().StringVar(&barRenderSession, "session", "", "session name (passed from tmux #S, avoids global query)")
		c.Flags().StringVar(&barRenderPaneCmd, "pane-cmd", "", "current pane command (passed from tmux #{pane_current_command})")
		c.Flags().StringVar(&barRenderPrefix, "prefix", "", "client prefix state 0|1 (passed from tmux #{client_prefix})")
		c.Flags().StringVar(&barRenderGroup, "group", "", "session group id (passed from tmux #{session_group})")
		c.Flags().StringVar(&barRenderGroupSize, "group-size", "", "session group size (passed from tmux #{session_group_size})")
		c.Flags().StringVar(&barRenderTopBar, "top-bar", "", "top bar variant: tabs, dots, minimal (passed from generate.go)")
	}
	registerFlags(cmd)

	// Add bar render debug subcommand to barCmd. Same flags: outside tmux
	// the session fallback query is ambiguous (it can land on a hidden-tab
	// dock session), so QA/agent use needs --session.
	barRenderDebug := &cobra.Command{
		Use:   "render <left|right|top|tabs>",
		Short: "Render a bar segment (debug)",
		Args:  cobra.ExactArgs(1),
		RunE:  cmd.RunE,
	}
	registerFlags(barRenderDebug)
	barCmd.AddCommand(barRenderDebug)

	return cmd
}

// workspaceAttachStates returns a slice of attach states index-aligned with
// the supplied workspace session names. Sessions matching `currentSession`
// are returned as Unknown because the dots renderer marks the current pill
// with `●` independently — the state field is for siblings. Any session
// with one or more attached tmux clients (other than this one) maps to
// AttachLocal; everything else is AttachUnknown.
//
// SSH-future: when remote sessions arrive, this helper will populate
// AttachRemote for sessions sourced from a remote socket / SSH attach.
func workspaceAttachStates(app *apppkg.App, sessions []string, currentSession string) []bar.AttachState {
	if len(sessions) == 0 {
		return nil
	}
	live, err := session.ListSessions(app.Runner)
	if err != nil {
		return nil
	}
	liveByName := make(map[string]session.SessionInfo, len(live))
	for _, s := range live {
		liveByName[s.Name] = s
	}
	states := make([]bar.AttachState, len(sessions))
	for i, name := range sessions {
		if name == currentSession {
			continue // dots renderer paints `●` for current; leave Unknown
		}
		info, ok := liveByName[name]
		if !ok {
			continue
		}
		if info.Attached || info.AttachedClients > 0 {
			states[i] = bar.AttachLocal
		}
	}
	return states
}
