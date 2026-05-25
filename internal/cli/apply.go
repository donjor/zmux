package cli

import (
	"fmt"
	"os"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/bar"
	"github.com/donjor/zmux/internal/theme"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/spf13/cobra"
)

func newApplyCmd(app *apppkg.App) *cobra.Command {
	var applyBootstrap bool
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Regenerate tmux.conf and apply theme + bar to running tmux",
		Long: `Reads ~/.zmux.toml, regenerates ~/.tmux.conf, and applies all theme
and bar options to the running tmux server.

Used by:
  - prefix+r (reload keybind) — sources conf for keybinding changes
  - run-shell in tmux.conf (bootstrap) — uses --bootstrap to skip source`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runApply(app, applyBootstrap)
		},
	}
	cmd.Flags().BoolVar(&applyBootstrap, "bootstrap", false,
		"skip source-file (used by tmux.conf bootstrap run-shell)")
	return cmd
}

func runApply(app *apppkg.App, skipSource bool) error {
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

	// Step 1: Regenerate tmux.conf at the active profile's conf path
	// (~/.tmux.conf for zmux, ~/.zzmux.conf for the zzmux edge profile).
	confContent := tmux.GenerateConf(&cfg, palette, zmuxBin)
	confPath := app.Profile.ConfFile
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
			_ = theme.Apply(app.Runner, app.FS, &cfg, t, app.ConfigPath)
		}
	}

	// Step 3: Set workspace env var for native tmux format access.
	// Use the endpoint-aware IsInsideTmux only — a raw $TMUX check would let a
	// zzmux run nested in the live default tmux act on the wrong server.
	if app.Runner.IsInsideTmux() {
		sessionName, _ := app.Runner.DisplayMessage("", "#{session_name}")
		if sessionName != "" {
			ws, _ := app.WorkspaceStore.WorkspaceFor(sessionName)
			_ = app.Runner.SetEnvironment("ZMUX_WORKSPACE", ws)
		}
	}

	// Step 4: Apply bar preset + layout (live set-option calls).
	preset, _ := bar.PresetFromString(cfg.Bar.Preset)
	layoutCfg := bar.BarLayoutConfig{
		Layout:    cfg.Bar.Layout,
		Indicator: cfg.Bar.Indicator,
		TopBar:    cfg.Bar.TopBar,
	}
	_ = bar.Apply(app.Runner, preset, palette, layoutCfg)

	// Step 4b: Per-session status line count for two-line layouts.
	if (cfg.Bar.Layout == "two-line" || cfg.Bar.Layout == "split") && zmuxBin != "" {
		adjustBarStatusLines(app.Runner, app.WorkspaceStore, cfg.Bar.TopBar, zmuxBin)
	}

	// Step 5: Source conf for keybinding changes.
	// Skipped during bootstrap (--bootstrap) to prevent an infinite
	// loop: source → bootstrap run-shell "zmux apply --bootstrap" →
	// source → … The TMUX_PANE heuristic was unreliable — TMUX_PANE
	// CAN be set in run-shell context (e.g. start-server ; ... ;
	// kill-server), so we use an explicit flag instead.
	if !skipSource {
		_ = app.Runner.SourceFile(confPath)
	}

	// Step 6: Refresh duplicate-name markers used by label-aware window formats.
	_ = refreshDuplicateWindowNameMarkers(app, "")

	return nil
}
