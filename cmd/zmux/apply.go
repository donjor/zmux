package main

import (
	"fmt"

	"github.com/donjor/zmux/internal/bar"
	"github.com/donjor/zmux/internal/theme"
	"github.com/spf13/cobra"
)

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply theme and bar preset to the running tmux session",
	Long: `Non-interactive command that reads the zmux config, resolves the theme
and bar preset, and applies all tmux options. Intended for use in
tmux.conf via run-shell:

  run-shell "zmux apply"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !app.Runner.ServerRunning() {
			return fmt.Errorf("tmux server is not running")
		}

		cfg, err := loadConfig(app.FS)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		palette, err := loadPalette(app.FS)
		if err != nil {
			return fmt.Errorf("load palette: %w", err)
		}

		// Apply theme environment variables to tmux.
		resolver := theme.NewResolver(app.FS, "~/.zmux/themes", "~/.zmux/themes/iterm2")
		t, err := resolver.Resolve(cfg.Theme)
		if err != nil {
			// Fall back to applying bar only if theme resolution fails.
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not resolve theme %q: %v\n", cfg.Theme, err)
		} else {
			if err := theme.Apply(app.Runner, app.FS, &cfg, t, "~/.zmux.toml"); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not apply theme env vars: %v\n", err)
			}
		}

		// Apply bar preset.
		preset, err := bar.PresetFromString(cfg.Bar.Preset)
		if err != nil {
			return fmt.Errorf("parse preset: %w", err)
		}

		if err := bar.Apply(app.Runner, preset, palette); err != nil {
			return fmt.Errorf("apply bar: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(applyCmd)
}
