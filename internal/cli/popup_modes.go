package cli

// Popup-mode dispatchers. zmux launches itself via tmux `display-popup -E`
// for the dashboard, command palette, and tab picker. The corresponding
// --dashboard / --palette / --tab-picker flags are intercepted in root.go
// and forwarded here.

import (
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/source"
	"github.com/donjor/zmux/internal/theme"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui/dashboard"
	"github.com/donjor/zmux/internal/tui/dashboard/tabs"
	"github.com/donjor/zmux/internal/tui/palette"
	"github.com/donjor/zmux/internal/tui/styles"
	"github.com/donjor/zmux/internal/tui/tabpicker"
	"github.com/donjor/zmux/internal/tui/workspaceview"
)

// resolveDashboardTab maps the --dashboard-tab flag value to a TabID. It
// handles the deprecated aliases "current" → TabSession and "sessions" →
// TabWorkspaces so older scripts / palette providers keep working.
// Empty input returns the default tab (TabSession).
func resolveDashboardTab(flag string) dashboard.TabID {
	if flag == "" {
		return dashboard.TabSession
	}
	switch flag {
	case "current":
		return dashboard.TabSession
	case "sessions":
		return dashboard.TabWorkspaces
	default:
		return dashboard.TabID(flag)
	}
}

func launchDashboardPopup(app *apppkg.App) error {
	// Find our own binary path.
	zmuxBin, err := os.Executable()
	if err != nil {
		zmuxBin = "zmux"
	}
	// Pass as single shell command string — tmux display-popup -E expects this.
	return app.Runner.DisplayPopup("-w80%", "-h80%", "-E", zmuxBin+" --dashboard")
}

// loadActiveStyles loads the configured theme and returns palette-aware styles.
// Falls back to DefaultStyles if theme can't be resolved.
func loadActiveStyles(app *apppkg.App) (styles.Styles, *theme.Palette, *theme.Resolver) {
	resolver, err := newResolver(app.FS)
	if err != nil {
		return styles.DefaultStyles(), nil, nil
	}
	cfg, err := loadConfig(app.FS)
	if err != nil {
		return styles.DefaultStyles(), nil, resolver
	}
	t, err := resolver.Resolve(cfg.Theme)
	if err != nil {
		return styles.DefaultStyles(), nil, resolver
	}
	p := t.SemanticPalette()
	return styles.NewStyles(&p), &p, resolver
}

func runDashboard(app *apppkg.App, dashboardTabFlag string) error {
	return runNewDashboard(app, dashboardTabFlag)
}

func runNewDashboard(app *apppkg.App, dashboardTabFlag string) error {
	styles, pal, resolver := loadActiveStyles(app)

	services := dashboard.Services{
		Runner:   app.Runner,
		FS:       app.FS,
		Styles:   styles,
		Palette:  pal,
		Resolver: resolver,
	}

	// Shared workspace data loader for Session and Workspaces tabs.
	wsLoader := func() []workspaceview.WorkspaceViewModel {
		workspaces, err := app.WorkspaceStore.ListWorkspaces()
		if err != nil {
			return nil
		}
		sessions, _ := session.ListSessions(app.Runner)
		return workspaceview.BuildWorkspaceViewModels(workspaces, sessions)
	}

	// Build tabs (order: Session, Workspaces, Themes, Bar, Settings, Help).
	tabImpls := []dashboard.Tab{
		tabs.NewCurrentTab(app.Runner, styles, wsLoader, app.WorkspaceStore),
		tabs.NewSessionsTab(app.Runner, styles, wsLoader, app.WorkspaceStore, app.Overmind),
		tabs.NewThemesTab(resolver, app.FS, app.Runner, styles),
		tabs.NewBarTab(resolver, app.FS, app.Runner, styles),
		tabs.NewSettingsTab(resolver, app.FS, app.Runner, styles),
		tabs.NewHelpTab(styles),
	}

	// Parse initial tab (handle deprecated names).
	initialTab := resolveDashboardTab(dashboardTabFlag)

	model := dashboard.NewDashboardApp(services, tabImpls, initialTab)

	p := tea.NewProgram(model)
	result, err := p.Run()
	if err != nil {
		return fmt.Errorf("dashboard: %w", err)
	}

	dash, ok := result.(*dashboard.DashboardApp)
	if !ok {
		return nil
	}

	return handleDashboardResult(app, dash.Action, dash.Chosen)
}

func runPalette(app *apppkg.App) error {
	styles, _, resolver := loadActiveStyles(app)

	// Build registry with all providers.
	reg := palette.NewDefaultRegistry(app.Runner, resolver, app.FS)

	model := palette.NewPaletteModel(reg, styles)
	p := tea.NewProgram(model)
	result, err := p.Run()
	if err != nil {
		return fmt.Errorf("palette: %w", err)
	}

	pm, ok := result.(*palette.PaletteModel)
	if !ok || pm.Chosen == nil {
		return nil
	}

	// Execute the chosen action.
	executor := palette.NewExecutor(app.Runner, app.FS, app.Overmind)
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

func runTabPicker(app *apppkg.App) error {
	// Get current session name.
	sessionName, err := app.Runner.DisplayMessage("", "#{session_name}")
	if err != nil {
		return fmt.Errorf("not inside a tmux session")
	}
	sessionName = session.RootName(sessionName)

	styles, _, _ := loadActiveStyles(app)
	model := tabpicker.NewTabPickerModel(app.Runner, sessionName, styles)

	p := tea.NewProgram(model)
	result, err := p.Run()
	if err != nil {
		return fmt.Errorf("tab picker: %w", err)
	}

	tp, ok := result.(tabpicker.TabPickerModel)
	if !ok || tp.Quitting && tp.Result.Action == "" {
		return nil
	}

	res := tp.Result
	switch res.Action {
	case "select":
		return app.Runner.SelectWindow(sessionName, res.Index)
	case "new":
		dir, _ := os.Getwd()
		return app.Runner.NewWindow(sessionName, res.Name, dir)
	case "rename":
		old := fmt.Sprintf("%d", res.Index)
		return app.Runner.RenameWindow(sessionName, old, res.Name)
	case "close":
		return app.Runner.KillWindow(sessionName, res.Index)
	case "swap":
		return app.Runner.SwapWindow(sessionName, res.Index, res.Index+res.Delta)
	}

	return nil
}

// handleDashboardResult applies the action chosen from inside the dashboard
// popup. Runs after the dashboard's bubbletea Program returns.
func handleDashboardResult(app *apppkg.App, action, chosen string) error {
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
			return app.Overmind.Connect(parts[1], parts[0])
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
