package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/source"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui"
	"github.com/donjor/zmux/internal/tui/dashboard"
	"github.com/donjor/zmux/internal/tui/dashboard/tabs"
	"github.com/donjor/zmux/internal/tui/palette"
)

var app = NewApp()

var dashboardFlag bool
var dashboardTabFlag string
var paletteFlag bool

var rootCmd = &cobra.Command{
	Use:   "zmux",
	Short: "An opinionated, all-in-one tmux management wrapper",
	Long:  "zmux replaces tmux's sharp edges with a beautiful, interactive experience.",
	SilenceUsage:  true,
	SilenceErrors: true,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if paletteFlag {
			return runPalette()
		}

		if dashboardFlag {
			return runDashboard()
		}

		if err := checkTmuxVersion(); err != nil {
			return err
		}

		if app.Runner.ServerRunning() {
			_, _ = session.CleanupTmp(app.Runner)
		}

		// Shorthand: `zmux <name>` attaches or creates.
		if len(args) > 0 {
			name := args[0]
			if app.Runner.HasSession(name) {
				return session.Attach(app.Runner, name)
			}
			dir, _ := os.Getwd()
			if dir == "" {
				dir = os.Getenv("HOME")
			}
			if err := session.Create(app.Runner, name, dir); err != nil {
				return err
			}
			return session.Attach(app.Runner, name)
		}

		if app.Runner.IsInsideTmux() {
			return launchDashboardPopup()
		}

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

func launchPalettePopup() error {
	zmuxBin, err := os.Executable()
	if err != nil {
		zmuxBin = "zmux"
	}
	return app.Runner.DisplayPopup("-w60%", "-h50%", "-E", zmuxBin+" --palette")
}

func runDashboard() error {
	return runNewDashboard()
}

func runNewDashboard() error {
	styles := tui.DefaultStyles()

	// Build theme resolver for the themes tab.
	resolver, err := newResolver(app.FS)
	if err != nil {
		// Non-fatal: themes tab will show empty.
		resolver = nil
	}

	services := dashboard.Services{
		Runner: app.Runner,
		FS:     app.FS,
		Styles: styles,
	}

	// Build tabs.
	tabImpls := []dashboard.Tab{
		tabs.NewCurrentTab(app.Runner, styles),
		tabs.NewSessionsTab(app.Runner, styles),
		tabs.NewSettingsTab(resolver, app.FS, styles),
		tabs.NewHelpTab(styles),
	}

	// Parse initial tab.
	initialTab := dashboard.TabCurrent
	if dashboardTabFlag != "" {
		initialTab = dashboard.TabID(dashboardTabFlag)
	}

	model := dashboard.NewDashboardApp(services, tabImpls, initialTab)

	p := tea.NewProgram(model, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return fmt.Errorf("dashboard: %w", err)
	}

	dash, ok := result.(*dashboard.DashboardApp)
	if !ok {
		return nil
	}

	return handleDashboardResult(dash.Action, dash.Chosen)
}

func runPalette() error {
	styles := tui.DefaultStyles()

	// Build theme resolver for theme actions.
	resolver, err := newResolver(app.FS)
	if err != nil {
		resolver = nil
	}

	// Build registry with all providers.
	reg := palette.NewDefaultRegistry(app.Runner, resolver, app.FS)

	model := palette.NewPaletteModel(reg, styles)
	p := tea.NewProgram(model, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return fmt.Errorf("palette: %w", err)
	}

	pm, ok := result.(*palette.PaletteModel)
	if !ok || pm.Chosen == nil {
		return nil
	}

	// Execute the chosen action.
	executor := palette.NewExecutor(app.Runner, app.FS)
	post := executor.Run(*pm.Chosen)

	switch post.Kind {
	case palette.PostClose:
		return nil

	case palette.PostOpenDashboard:
		// Re-launch as dashboard popup with the specified tab.
		zmuxBin, binErr := os.Executable()
		if binErr != nil {
			zmuxBin = "zmux"
		}
		return app.Runner.DisplayPopup(
			"-w80%", "-h80%", "-E",
			zmuxBin+" --dashboard --dashboard-tab="+post.Tab,
		)

	case palette.PostError:
		if post.Err != nil {
			return post.Err
		}
		return nil
	}

	return nil
}

func handleDashboardResult(action, chosen string) error {
	switch action {
	case "switch":
		return session.Switch(app.Runner, chosen)

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

	case "overmind-connect":
		// chosen = "process\tcontrol_socket"
		parts := strings.SplitN(chosen, "\t", 2)
		if len(parts) == 2 {
			return source.Connect(parts[1], parts[0])
		}
		return fmt.Errorf("invalid overmind-connect payload: %q", chosen)

	case "external-attach":
		// chosen = "session\t-L socket_name" or "session\t-S socket_path"
		parts := strings.SplitN(chosen, "\t", 2)
		if len(parts) < 2 {
			return fmt.Errorf("invalid external-attach payload: %q", chosen)
		}
		sessionName := parts[0]
		epArgs := strings.Fields(parts[1])
		ep := tmux.DefaultEndpoint()
		if len(epArgs) == 2 {
			switch epArgs[0] {
			case "-L":
				ep = tmux.NamedEndpoint(epArgs[1])
			case "-S":
				ep = tmux.PathEndpoint(epArgs[1])
			}
		}
		return source.ConnectFallback(ep, sessionName, "")
	}

	return nil
}

func runSessionPicker() error {
	// Resolve working directory.
	dir, err := os.Getwd()
	if err != nil {
		dir = os.Getenv("HOME")
		if dir == "" {
			dir = "/"
		}
	}

	// Load templates (embedded + user dirs).
	cfg := config.DefaultConfig()
	templates, _ := session.LoadTemplates(app.FS, cfg.Templates.Paths)

	styles := tui.DefaultStyles()
	model := tui.NewPickerModel(app.Runner, styles)
	model.SetTemplates(templates)

	p := tea.NewProgram(model, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return fmt.Errorf("session picker: %w", err)
	}

	picker, ok := result.(tui.PickerModel)
	if !ok {
		return nil
	}

	res := picker.Result

	switch res.Action {
	case "attach":
		return session.Attach(app.Runner, res.Session)

	case "hijack":
		return session.AttachHijack(app.Runner, res.Session)

	case "new":
		name := res.Name
		if name == "" {
			name = session.NextTmpName(app.Runner)
		}
		if err := session.Create(app.Runner, name, dir); err != nil {
			return err
		}
		return session.Attach(app.Runner, name)

	case "template":
		tmplName := res.Template
		name := res.Name
		if name == "" {
			name = tmplName
		}
		if name == "" {
			name = session.NextTmpName(app.Runner)
		}

		// Find the matching template.
		var tmpl *session.Template
		for i := range templates {
			if templates[i].Name == tmplName {
				tmpl = &templates[i]
				break
			}
		}

		if tmpl == nil {
			// Fallback: try first template if available.
			if len(templates) == 0 {
				return fmt.Errorf("no templates available")
			}
			tmpl = &templates[0]
		}

		if err := session.CreateFromTemplate(app.Runner, *tmpl, name, dir); err != nil {
			return err
		}
		return session.Attach(app.Runner, name)

	case "overmind-connect":
		src := res.ExternalSource
		if src != nil && src.Overmind != nil {
			return source.Connect(src.Overmind.ControlSocket, res.Session)
		}
		// Fallback to direct tmux attach.
		if src != nil {
			return source.ConnectFallback(src.Endpoint, res.Session, "")
		}
		return fmt.Errorf("no source for overmind-connect")

	case "external-attach":
		src := res.ExternalSource
		if src != nil {
			return source.ConnectFallback(src.Endpoint, res.Session, "")
		}
		return fmt.Errorf("no source for external-attach")
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
	rootCmd.PersistentFlags().StringVar(&dashboardTabFlag, "dashboard-tab", "", "initial tab for dashboard (current, sessions, settings, help)")
	rootCmd.PersistentFlags().BoolVar(&paletteFlag, "palette", false, "render command palette directly (used by popup)")
	rootCmd.AddCommand(versionCmd)
}
