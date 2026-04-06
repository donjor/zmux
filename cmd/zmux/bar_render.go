package main

import (
	"fmt"

	"github.com/donjor/zmux/internal/bar"
	"github.com/spf13/cobra"
)

var barRenderDir string

var barRenderCmd = &cobra.Command{
	Use:    "bar-render <left|right>",
	Short:  "Render a status bar segment (used by tmux #())",
	Long:   `Internal command called by tmux's #() shell execution to render dynamic status bar content.`,
	Args:   cobra.ExactArgs(1),
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		side := args[0]

		palette := loadPaletteOrDefault(app.FS)

		// Get tmux state.
		sessionName, _ := app.Runner.DisplayMessage("", "#{session_name}")
		paneCmd, _ := app.Runner.DisplayMessage("", "#{pane_current_command}")
		prefixStr, _ := app.Runner.DisplayMessage("", "#{client_prefix}")
		groupID, _ := app.Runner.DisplayMessage("", "#{session_group}")

		// Use --dir if provided (embedded in #() for per-window cache),
		// otherwise fall back to querying tmux.
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
		ctx := bar.GatherContext(sessionName, paneDir, paneCmd, prefixStr, groupID, workspace)
		ctx.WorkspacePos = wsPos
		ctx.WorkspaceCount = wsCount
		ctx.ShowWorkspace = cfg.Bar.Segments.Workspace
		ctx.ShowGit = cfg.Bar.Segments.Git
		ctx.ShowLang = cfg.Bar.Segments.Lang
		ctx.ShowClock = cfg.Bar.Segments.Clock
		ctx.ShowDirectory = cfg.Bar.Segments.Directory
		ctx.ShowProcess = cfg.Bar.Segments.Process
		ctx.ShowGroup = cfg.Bar.Segments.Group

		switch side {
		case "left":
			fmt.Print(bar.RenderLeft(palette, ctx, preset))
		case "right":
			fmt.Print(bar.RenderRight(palette, ctx, preset))
		default:
			return fmt.Errorf("unknown side %q (use 'left' or 'right')", side)
		}

		return nil
	},
}

func init() {
	barRenderCmd.Flags().StringVar(&barRenderDir, "dir", "", "pane directory (avoids tmux query, enables per-window cache)")
	rootCmd.AddCommand(barRenderCmd)

	var barRenderDebug = &cobra.Command{
		Use:   "render <left|right>",
		Short: "Render a bar segment (debug)",
		Args:  cobra.ExactArgs(1),
		RunE:  barRenderCmd.RunE,
	}
	barCmd.AddCommand(barRenderDebug)
}
