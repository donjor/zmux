package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Print current zmux configuration summary",
	Long:  `Shows the current zmux configuration including theme, bar preset, prefix key, sync target, and system info.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runStatus()
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus() error {

	styles := tui.DefaultStyles()

	cfg, err := loadConfig(app.FS)
	if err != nil {
		cfg = config.DefaultConfig()
	}

	cfgPath, _ := config.ConfigPath(app.FS)
	configExists := config.ConfigExists(app.FS)

	// Detect tmux version.
	tmuxVersion := "not found"
	if ver, err := app.Runner.Version(); err == nil {
		tmuxVersion = ver
	}

	// Detect clipboard.
	clipboard := tmux.DetectClipboard()
	if clipboard == "" {
		clipboard = "none"
	}

	// Count sessions.
	sessionCount := 0
	if sessions, err := app.Runner.ListSessions(); err == nil {
		sessionCount = len(sessions)
	}

	// Determine theme source.
	themeSource := "default"
	if configExists {
		themeSource = "config"
	}

	// Render.
	var b strings.Builder

	title := styles.Title.Render("zmux status")
	b.WriteString(title + "\n\n")

	labelStyle := lipgloss.NewStyle().Width(16).Foreground(lipgloss.Color("3"))
	valueStyle := styles.Normal

	rows := []struct {
		label string
		value string
	}{
		{"Theme", cfg.Theme + " (" + themeSource + ")"},
		{"Bar preset", cfg.Bar.Preset},
		{"Prefix", cfg.Prefix},
		{"Sync target", cfg.Sync.Target},
		{"Config", cfgPath},
		{"tmux", tmuxVersion},
		{"Clipboard", clipboard},
		{"Sessions", fmt.Sprintf("%d", sessionCount)},
	}

	for _, row := range rows {
		label := labelStyle.Render(row.label + ":")
		value := valueStyle.Render(row.value)
		b.WriteString("  " + label + " " + value + "\n")
	}

	if !configExists {
		b.WriteString("\n")
		b.WriteString(styles.Muted.Render("  No config file found. Run 'zmux init' to get started.") + "\n")
	}

	fmt.Print(b.String())
	return nil
}
