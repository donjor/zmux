package cli

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/theme"
	"github.com/donjor/zmux/internal/tui/wizard"
	"github.com/spf13/cobra"
)

func newInitCmd(app *apppkg.App, version string) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize zmux with an interactive setup wizard",
		Long: `Launches an interactive wizard that guides you through first-time setup:

  - Checks dependencies (tmux, clipboard tools)
  - Detects available sync targets (ghostty, nvim)
  - Lets you pick a theme and status bar preset
  - Writes ~/.zmux.toml and ~/.tmux.conf
  - Creates user directories (~/.zmux/themes/, ~/.zmux/templates/)

Run this once to get started with zmux.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInitWizard(app, version)
		},
	}
}

func runInitWizard(app *apppkg.App, version string) error {
	if app.Runner.IsInsideTmux() {
		return fmt.Errorf("zmux init should be run outside of tmux — exit your session first")
	}

	// Warn if config already exists.
	if config.ConfigExists(app.FS) {
		fmt.Println("Note: ~/.zmux.toml already exists. The wizard will overwrite it.")
	}

	resolver, err := newResolver(app.FS)
	if err != nil {
		// Fallback: create resolver with empty dirs.
		resolver = theme.NewResolver(app.FS, "", "")
	}

	styles, _, _ := loadActiveStyles(app)
	model := wizard.NewWizardModel(app.FS, resolver, version, styles, app.Profile)

	p := tea.NewProgram(model)
	result, err := p.Run()
	if err != nil {
		return fmt.Errorf("init wizard: %w", err)
	}

	wz, ok := result.(wizard.WizardModel)
	if !ok {
		return nil
	}

	if wz.Cancelled {
		fmt.Println("Init cancelled.")
		return nil
	}

	if wz.Error != nil {
		return wz.Error
	}

	// After TUI exits, echo the restart command so user can copy/paste.
	if wz.Done && !wz.Copied {
		fmt.Println()
		fmt.Printf("  Run this to apply:\n\n")
		fmt.Printf("    %s\n\n", wizard.RestartCmd(app.Profile))
	}

	return nil
}
