package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tui"
)

var app = NewApp()

var dashboardFlag bool

var rootCmd = &cobra.Command{
	Use:   "zmux",
	Short: "An opinionated, all-in-one tmux management wrapper",
	Long:  "zmux replaces tmux's sharp edges with a beautiful, interactive experience.",
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if dashboardFlag {
			// --dashboard: render dashboard directly (called from popup).
			return runDashboard()
		}

		// Check tmux version floor (>= 3.2 required for display-popup).
		if err := checkTmuxVersion(); err != nil {
			return err
		}

		// Cleanup stale tmp sessions on every zmux start (if tmux is running).
		if app.Runner.ServerRunning() {
			_, _ = session.CleanupTmp(app.Runner)
		}

		if app.Runner.IsInsideTmux() {
			// Inside tmux without --dashboard: launch popup.
			return launchDashboardPopup()
		}

		// Outside tmux: launch the session picker.
		return runSessionPicker()
	},
}

func launchDashboardPopup() error {
	// Find our own binary path.
	zmuxBin, err := os.Executable()
	if err != nil {
		zmuxBin = "zmux"
	}
	// Pass as single shell command string — tmux display-popup -E expects this.
	return app.Runner.DisplayPopup("-w80%", "-h80%", "-E", zmuxBin+" --dashboard")
}

func runDashboard() error {
	styles := tui.DefaultStyles()
	model := tui.NewDashboardModel(app.Runner, styles)

	p := tea.NewProgram(model, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return fmt.Errorf("dashboard: %w", err)
	}

	dash, ok := result.(tui.DashboardModel)
	if !ok {
		return nil
	}

	switch dash.Action {
	case "switch":
		return session.Switch(app.Runner, dash.Chosen)

	case "new":
		name := session.NextTmpName(app.Runner)
		if err := session.Create(app.Runner, name, "."); err != nil {
			return err
		}
		return session.Switch(app.Runner, name)

	case "template":
		templates := session.EmbeddedTemplates()
		if len(templates) == 0 {
			return fmt.Errorf("no templates available")
		}
		name := session.NextTmpName(app.Runner)
		if err := session.CreateFromTemplate(app.Runner, templates[0], name, "."); err != nil {
			return err
		}
		return session.Switch(app.Runner, name)

	case "theme":
		// Re-launch with theme subcommand.
		// For now, just return nil; the theme command exists separately.
		return nil
	}

	return nil
}

func runSessionPicker() error {
	styles := tui.DefaultStyles()
	model := tui.NewPickerModel(app.Runner, styles)

	p := tea.NewProgram(model, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return fmt.Errorf("session picker: %w", err)
	}

	picker, ok := result.(tui.PickerModel)
	if !ok {
		return nil
	}

	switch picker.Action {
	case "attach":
		return session.Attach(app.Runner, picker.Chosen)

	case "new":
		name := session.NextTmpName(app.Runner)
		if err := session.Create(app.Runner, name, "."); err != nil {
			return err
		}
		return session.Attach(app.Runner, name)

	case "template":
		templates := session.EmbeddedTemplates()
		if len(templates) == 0 {
			return fmt.Errorf("no templates available")
		}
		name := session.NextTmpName(app.Runner)
		if err := session.CreateFromTemplate(app.Runner, templates[0], name, "."); err != nil {
			return err
		}
		return session.Attach(app.Runner, name)
	}

	return nil
}

func checkTmuxVersion() error {
	ver, err := app.Runner.Version()
	if err != nil {
		return fmt.Errorf("tmux not found — install tmux >= 3.2 to use zmux")
	}

	// Parse major.minor from version string like "3.4" or "3.2a".
	parts := strings.SplitN(ver, ".", 2)
	if len(parts) < 2 {
		return nil // can't parse, let it through
	}
	major := 0
	minor := 0
	fmt.Sscanf(parts[0], "%d", &major)
	// Minor may have trailing letters like "2a".
	fmt.Sscanf(parts[1], "%d", &minor)

	if major < 3 || (major == 3 && minor < 2) {
		return fmt.Errorf("tmux %s found, but zmux requires >= 3.2 (for popup support)", ver)
	}
	return nil
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&dashboardFlag, "dashboard", false, "render dashboard TUI directly (used by popup)")
	rootCmd.AddCommand(versionCmd)
}
