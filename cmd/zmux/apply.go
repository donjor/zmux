package main

import (
	"fmt"
	"os"

	"github.com/donjor/zmux/internal/bar"
	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/theme"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/spf13/cobra"
)

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Regenerate tmux.conf and apply theme + bar to running tmux",
	Long: `Reads ~/.zmux.toml, regenerates ~/.tmux.conf, and applies all theme
and bar options to the running tmux server.

Used by:
  - prefix+r (reload keybind)
  - After changing theme/bar/config manually
  - run-shell in tmux.conf (bootstrap — skips source to avoid loop)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig(app.FS)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		palette, err := loadPalette(app.FS)
		if err != nil {
			return fmt.Errorf("load palette: %w", err)
		}

		zmuxBin, err := os.Executable()
		if err != nil {
			zmuxBin = "zmux"
		}

		// Step 1: Regenerate tmux.conf.
		confContent := tmux.GenerateConf(&cfg, palette, zmuxBin)
		confPath, _ := config.ExpandHome(app.FS, "~/.tmux.conf")
		if confPath == "" {
			confPath = "~/.tmux.conf"
		}
		if err := tmux.WriteConf(app.FS, confPath, confContent); err != nil {
			return fmt.Errorf("write tmux.conf: %w", err)
		}

		if !app.Runner.ServerRunning() {
			return nil
		}

		// Step 2: Apply theme env vars.
		resolver, err := newResolver(app.FS)
		if err == nil {
			t, resolveErr := resolver.Resolve(cfg.Theme)
			if resolveErr == nil {
				cfgPath, _ := config.ConfigPath(app.FS)
				_ = theme.Apply(app.Runner, app.FS, &cfg, t, cfgPath)
			}
		}

		// Step 3: Apply bar preset (live set-option calls).
		preset, _ := bar.PresetFromString(cfg.Bar.Preset)
		_ = bar.Apply(app.Runner, preset, palette)

		// Step 4: Source conf ONLY when called interactively (prefix+r).
		// The bootstrap run-shell in tmux.conf must NOT re-source, or it loops.
		// We detect this: if TMUX_PANE is not set, we're in run-shell context.
		if os.Getenv("TMUX_PANE") != "" {
			// Interactive — source to pick up keybind/setting changes.
			_ = app.Runner.SourceFile(confPath)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(applyCmd)
}
