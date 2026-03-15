package main

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/theme"
	"github.com/donjor/zmux/internal/tui"
)

var initCmd = &cobra.Command{
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
		return runInitWizard()
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInitWizard() error {


	// Warn if config already exists.
	if config.ConfigExists(app.FS) {
		fmt.Println("Note: ~/.zmux.toml already exists. The wizard will overwrite it.")
	}

	resolver, err := newResolver(app.FS)
	if err != nil {
		// Fallback: create resolver with empty dirs.
		resolver = theme.NewResolver(app.FS, "", "")
	}

	styles := tui.DefaultStyles()
	model := tui.NewWizardModel(app.FS, resolver, version, styles)

	p := tea.NewProgram(model, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return fmt.Errorf("init wizard: %w", err)
	}

	wizard, ok := result.(tui.WizardModel)
	if !ok {
		return nil
	}

	if wizard.Cancelled {
		fmt.Println("Init cancelled.")
		return nil
	}

	if wizard.Error != nil {
		return wizard.Error
	}

	return nil
}
