package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/bar"
	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/theme"
	"github.com/spf13/cobra"
)

func newBarCmd(app *apppkg.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bar [preset]",
		Short: "Manage status bar presets",
		Long: `List available bar presets with ANSI previews, or set a preset directly.

Without arguments, lists all presets with colored previews.
With a preset name, sets the bar to that preset immediately.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				// Listing is read-only; use a fallback palette if config is broken.
				palette := loadPaletteOrDefault(app.FS)
				return barList(app, palette)
			}

			// Setting requires a valid palette.
			palette, err := loadPalette(app.FS)
			if err != nil {
				return err
			}
			return barSet(app, args[0], palette)
		},
	}
	cmd.AddCommand(newBarShowCmd(app))
	return cmd
}

func newBarShowCmd(app *apppkg.App) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show the current bar preset from config",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := loadConfig(app.FS)
			palette := loadPaletteOrDefault(app.FS)

			preset, err := bar.PresetFromString(cfg.Bar.Preset)
			if err != nil {
				preset = bar.Default
			}

			fmt.Printf("Current preset: %s\n", preset)
			fmt.Println(bar.RenderPreview(preset, palette))
			return nil
		},
	}
}

// barList shows all presets. Inside tmux: live preview carousel (applies each
// preset to the actual status bar for 2 seconds). Outside tmux: static ANSI previews.
func barList(app *apppkg.App, palette *theme.Palette) error {
	cfg, _ := loadConfig(app.FS)
	currentPreset := cfg.Bar.Preset

	if app.Runner.IsInsideTmux() {
		return barListLive(app, palette, currentPreset)
	}

	// Outside tmux: static previews.
	for _, p := range bar.AllPresets() {
		marker := "  "
		if p.String() == currentPreset {
			marker = "* "
		}
		fmt.Printf("%s%s\n", marker, p)
		fmt.Printf("  %s\n\n", bar.RenderPreview(p, palette))
	}
	return nil
}

// barListLive cycles through presets on the actual tmux status bar.
func barListLive(app *apppkg.App, palette *theme.Palette, currentPreset string) error {
	presets := bar.AllPresets()

	fmt.Println("  Cycling through bar presets (live preview)...")
	fmt.Println("  Watch your status bar!")
	fmt.Println()

	for _, p := range presets {
		marker := "  "
		if p.String() == currentPreset {
			marker = "* "
		}
		fmt.Printf("%s%s", marker, p)

		// Apply this preset to the live bar.
		_ = bar.Apply(app.Runner, config.SelfBin(app.Profile), p, palette)

		if p.String() == currentPreset {
			fmt.Println("  (current)")
		} else {
			fmt.Println()
		}

		// Hold for 2 seconds so user can see it.
		time.Sleep(2 * time.Second)
	}

	// Restore the current preset.
	current, _ := bar.PresetFromString(currentPreset)
	_ = bar.Apply(app.Runner, config.SelfBin(app.Profile), current, palette)

	fmt.Printf("\n  Restored: %s\n", currentPreset)
	fmt.Println("  Set with: zmux bar <preset>")
	return nil
}

// barSet sets the bar preset, applies it to tmux, and saves the config.
func barSet(app *apppkg.App, name string, palette *theme.Palette) error {
	preset, err := bar.PresetFromString(name)
	if err != nil {
		// Show available presets on error
		fmt.Fprintf(os.Stderr, "Available presets: %s\n",
			strings.Join(presetNames(), ", "))
		return err
	}

	// Apply to tmux if server is running
	if app.Runner.ServerRunning() {
		if err := bar.Apply(app.Runner, config.SelfBin(app.Profile), preset, palette); err != nil {
			return fmt.Errorf("apply bar: %w", err)
		}
	}

	// Update config
	cfg, err := loadConfig(app.FS)
	if err != nil {
		return err
	}

	cfg.Bar.Preset = preset.String()
	cfgPath, err := config.ConfigPath(app.FS)
	if err != nil {
		return err
	}

	if err := config.Save(app.FS, cfgPath, cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("Bar preset set to: %s\n", preset)
	fmt.Println(bar.RenderPreview(preset, palette))
	return nil
}

// presetNames returns the string names of all presets.
func presetNames() []string {
	presets := bar.AllPresets()
	names := make([]string, len(presets))
	for i, p := range presets {
		names[i] = p.String()
	}
	return names
}

// loadConfig loads the zmux config, falling back to defaults.
func loadConfig(fs config.FS) (config.Config, error) {
	cfgPath, err := config.ConfigPath(fs)
	if err != nil {
		return config.DefaultConfig(), nil
	}

	if !config.ConfigExists(fs) {
		return config.DefaultConfig(), nil
	}

	return config.Load(fs, cfgPath)
}

// loadPalette resolves the theme from config and returns its semantic palette.
func loadPalette(fs config.FS) (*theme.Palette, error) {
	cfg, err := loadConfig(fs)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// Use the active profile's themes dir so `zzmux apply` resolves against
	// ~/.zzmux/themes (+ bundled), not the live ~/.zmux/themes.
	p := config.ActiveProfile(fs)
	resolver := theme.NewResolver(
		fs,
		p.ThemesDir,
		p.ThemesDir+"/iterm2",
	)

	t, err := resolver.Resolve(cfg.Theme)
	if err != nil {
		return nil, fmt.Errorf("resolve theme %q: %w", cfg.Theme, err)
	}

	palette := t.SemanticPalette()
	return &palette, nil
}

// loadPaletteOrDefault tries loadPalette; on any failure it falls back to the
// default bundled theme so that listing/preview commands always work.
func loadPaletteOrDefault(fs config.FS) *theme.Palette {
	p, err := loadPalette(fs)
	if err == nil {
		return p
	}

	// Fallback: resolve the default theme directly from bundled themes.
	defaults := config.DefaultConfig()
	resolver := theme.NewResolver(fs, "", "")
	t, err := resolver.Resolve(defaults.Theme)
	if err != nil {
		// Last resort: return a minimal palette so we never panic.
		return &theme.Palette{}
	}
	palette := t.SemanticPalette()
	return &palette
}
