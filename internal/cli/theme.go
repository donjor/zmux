package cli

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/bar"
	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/sync"
	"github.com/donjor/zmux/internal/theme"
	"github.com/donjor/zmux/internal/tui/themepicker"
	"github.com/spf13/cobra"
)

func newThemeCmd(app *apppkg.App) (*cobra.Command, *cobra.Command) {
	var themeSetCmd *cobra.Command
	var themePullCmd *cobra.Command

	themeSetCmd = &cobra.Command{
		Use:   "set <name>",
		Short: "Set theme directly (no TUI)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return setTheme(app, args[0])
		},
	}

	themeListCmd := &cobra.Command{
		Use:   "list",
		Short: "Print available themes to stdout",
		Long:  "Non-interactive, scriptable theme listing.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listThemes(app)
		},
	}

	themeSyncCmd := &cobra.Command{
		Use:   "sync",
		Short: "Pull theme from the default sync target",
		RunE: func(cmd *cobra.Command, args []string) error {
			return syncTheme(app, "")
		},
	}

	themePullCmd = &cobra.Command{
		Use:   "pull <target>",
		Short: "Pull theme from a specific target (ghostty, nvim)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return syncTheme(app, args[0])
		},
	}

	themeCmd := &cobra.Command{
		Use:   "theme",
		Short: "Launch the theme picker TUI",
		Long: `Interactive theme picker with fuzzy search and color previews.

Without subcommands, launches the interactive TUI.
Use subcommands for scriptable, non-interactive operations.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runThemePicker(app)
		},
	}

	themeCmd.AddCommand(themeSetCmd)
	themeCmd.AddCommand(themeListCmd)
	themeCmd.AddCommand(themeSyncCmd)
	themeCmd.AddCommand(themePullCmd)

	return themeCmd, themePullCmd
}

func newResolver(fs config.FS) (*theme.Resolver, error) {
	// Use the active profile's themes dir so `zzmux` reads/writes ~/.zzmux/themes
	// (bundled themes are go:embed and always available regardless of profile).
	p := config.ActiveProfile(fs)
	return theme.NewResolver(
		fs,
		p.ThemesDir,
		p.ThemesDir+"/iterm2",
	), nil
}

func runThemePicker(app *apppkg.App) error {
	resolver, err := newResolver(app.FS)
	if err != nil {
		return err
	}

	styles, _, _ := loadActiveStyles(app)
	model := themepicker.NewThemePickerModel(resolver, styles)

	p := tea.NewProgram(model)
	result, err := p.Run()
	if err != nil {
		return fmt.Errorf("theme picker: %w", err)
	}

	picker, ok := result.(themepicker.ThemePickerModel)
	if !ok || picker.Chosen == "" {
		return nil
	}

	return setTheme(app, picker.Chosen)
}

func setTheme(app *apppkg.App, name string) error {
	resolver, err := newResolver(app.FS)
	if err != nil {
		return err
	}

	t, err := resolver.Resolve(name)
	if err != nil {
		return err
	}

	cfg, err := loadConfig(app.FS)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	cfgPath, err := config.ConfigPath(app.FS)
	if err != nil {
		return fmt.Errorf("config path: %w", err)
	}

	// Apply theme if tmux is running.
	if app.Runner.ServerRunning() {
		if err := theme.Apply(app.Runner, app.FS, &cfg, t, cfgPath); err != nil {
			return fmt.Errorf("apply theme: %w", err)
		}

		// Apply bar preset separately (theme.Apply cannot import bar due to
		// circular dependency).
		palette := t.SemanticPalette()
		preset, err := bar.PresetFromString(cfg.Bar.Preset)
		if err != nil {
			return fmt.Errorf("parse bar preset: %w", err)
		}
		if err := bar.Apply(app.Runner, preset, &palette); err != nil {
			return fmt.Errorf("apply bar: %w", err)
		}

		fmt.Printf("Theme set to: %s\n", name)
	} else {
		// Just save the config.
		cfg.Theme = name
		if err := config.Save(app.FS, cfgPath, cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}
		fmt.Printf("Theme set to: %s (will apply on next tmux start)\n", name)
	}

	return nil
}

func listThemes(app *apppkg.App) error {
	resolver, err := newResolver(app.FS)
	if err != nil {
		return err
	}

	cfg, _ := loadConfig(app.FS)
	currentTheme := cfg.Theme

	themes := resolver.List()
	for _, ti := range themes {
		marker := "  "
		if ti.Name == currentTheme {
			marker = "* "
		}

		var tags []string
		tags = append(tags, string(ti.Source))
		if ti.IsDark {
			tags = append(tags, "dark")
		} else {
			tags = append(tags, "light")
		}

		fmt.Printf("%s%-30s [%s]\n", marker, ti.Name, strings.Join(tags, ", "))
	}

	return nil
}

func syncTheme(app *apppkg.App, targetName string) error {
	cfg, err := loadConfig(app.FS)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Determine target.
	if targetName == "" {
		targetName = cfg.Sync.Target
	}

	if targetName == "" || targetName == "none" {
		return fmt.Errorf("no sync target configured\nset sync.target in ~/.zmux.toml (ghostty or nvim)")
	}

	var target sync.SyncTarget
	switch targetName {
	case "ghostty":
		target = sync.NewGhosttyTarget(app.FS, cfg.Sync.GhosttyConfig)
	case "nvim":
		target = sync.NewNvimTarget(sync.RealCmdRunner{})
	default:
		return fmt.Errorf("unknown sync target: %s (valid: ghostty, nvim)", targetName)
	}

	name, err := target.Pull()
	if err != nil {
		return fmt.Errorf("could not read theme from %s: %w", targetName, err)
	}

	// Verify the theme exists in zmux.
	resolver, err := newResolver(app.FS)
	if err != nil {
		return err
	}

	if _, err := resolver.Resolve(name); err != nil {
		return fmt.Errorf("theme %q from %s not found in zmux theme library", name, targetName)
	}

	// Apply the theme.
	if err := setTheme(app, name); err != nil {
		return err
	}

	fmt.Printf("Synced theme from %s\n", targetName)
	return nil
}
