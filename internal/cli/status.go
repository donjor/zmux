package cli

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/spf13/cobra"
)

func newStatusCmd(app *apppkg.App) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Print current zmux configuration summary",
		Long:  `Shows the current zmux configuration including theme, bar preset, prefix key, sync target, and system info.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(app)
		},
	}
}

func runStatus(app *apppkg.App) error {
	styles, _, _ := loadActiveStyles(app)

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

	// Count sessions (reserved zmux-internal ones don't count).
	sessionCount := 0
	if sessions, err := app.Runner.ListSessions(); err == nil {
		for _, s := range sessions {
			if !tabs.IsReservedSession(s.Name) {
				sessionCount++
			}
		}
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

	labelStyle := styles.Accent.Width(16)
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

	// lipgloss v2 renders full-fidelity; the package Writer downsamples to the
	// terminal's color profile (v1 did this implicitly via the global renderer).
	lipgloss.Print(b.String())
	return nil
}
