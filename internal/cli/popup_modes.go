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
	"github.com/donjor/zmux/internal/config"
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
	"github.com/donjor/zmux/internal/tui/wspicker"
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
	// Find our own binary path (profile-correct; never a hardcoded "zmux").
	zmuxBin := config.SelfBin(app.Profile)
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

	// Shared workspace data loader for Session and Workspaces tabs. The
	// dashboard does not reconcile here — it refreshes via its own fetch cycle.
	wsLoader := func() []workspaceview.WorkspaceViewModel {
		return loadWorkspaceView(app, workspaceViewOptions{})
	}

	// Build tabs (order: Session, Workspaces, Themes, Bar, Settings, Help).
	// selfBin is the active profile's binary, embedded in the bar's
	// #(<bin> bar-render) when the Themes/Bar tabs hot-reload the live bar.
	selfBin := config.SelfBin(app.Profile)
	tabImpls := []dashboard.Tab{
		tabs.NewCurrentTab(app.Runner, styles, wsLoader, app.WorkspaceStore),
		tabs.NewSessionsTab(app.Runner, styles, wsLoader, app.WorkspaceStore, app.Overmind),
		tabs.NewThemesTab(resolver, app.FS, app.Runner, selfBin, styles),
		tabs.NewBarTab(resolver, app.FS, app.Runner, selfBin, styles),
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
	executor := palette.NewExecutor(app.Runner, app.FS, app.Overmind, app.WorkspaceStore)
	post := executor.Run(*pm.Chosen)

	switch post.Kind {
	case palette.PostClose:
		return nil

	case palette.PostOpenDashboard:
		// Re-launch as dashboard popup with the specified tab.
		zmuxBin := config.SelfBin(app.Profile)
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
	// Current session name (root — clone suffixes stripped).
	sessionName, err := app.Runner.DisplayMessage("", "#{session_name}")
	if err != nil {
		return fmt.Errorf("not inside a tmux session")
	}
	sessionName = session.RootName(strings.TrimSpace(sessionName))

	// Resolve the workspace + its live sessions. The switcher is scoped to
	// the current workspace; if the session isn't claimed by one we fall
	// back to a single-session list so the tab ops still work.
	wsName := sessionName
	var infos []session.SessionInfo
	if app.WorkspaceStore != nil {
		if name, ok := app.WorkspaceStore.WorkspaceFor(sessionName); ok {
			wsName = name
		}
	}
	for _, vm := range loadWorkspaceView(app, workspaceViewOptions{}) {
		if vm.Name == wsName {
			infos = vm.LiveSessions
			break
		}
	}
	if len(infos) == 0 {
		// Not in a tracked workspace (or none live): just the current one.
		infos = []session.SessionInfo{{Name: sessionName, Attached: true}}
	}

	styles, _, _ := loadActiveStyles(app)
	model := tabpicker.NewTabPickerModel(app.Runner, wsName, sessionName, infos, styles)

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
	target := res.Session
	if target == "" {
		target = sessionName
	}
	switch res.Action {
	case "switch":
		return session.Switch(app.Runner, target)
	case "select":
		// Switch to the owning session first if it isn't the current one,
		// then select the window there.
		if session.RootName(target) != sessionName {
			if err := session.Switch(app.Runner, target); err != nil {
				return err
			}
		}
		return app.Runner.SelectWindow(target, res.Index)
	case "new":
		dir, _ := os.Getwd()
		return app.Runner.NewWindow(target, res.Name, dir)
	case "rename":
		old := fmt.Sprintf("%d", res.Index)
		return app.Runner.RenameWindow(target, old, res.Name)
	case "close":
		return app.Runner.KillWindow(target, res.Index)
	case "swap":
		return app.Runner.SwapWindow(target, res.Index, res.Index+res.Delta)
	}

	return nil
}

func runWorkspacePicker(app *apppkg.App) error {
	styles, _, _ := loadActiveStyles(app)

	loader := func() []workspaceview.WorkspaceViewModel {
		return loadWorkspaceView(app, workspaceViewOptions{Reconcile: true, HidePseudo: true})
	}

	model := wspicker.NewModel(loader, styles)
	p := tea.NewProgram(model)
	result, err := p.Run()
	if err != nil {
		return fmt.Errorf("workspace picker: %w", err)
	}

	wp, ok := result.(wspicker.Model)
	if !ok || wp.Result.Action != "switch" {
		return nil
	}

	ws, err := app.WorkspaceStore.GetWorkspace(wp.Result.Workspace)
	if err != nil || ws == nil {
		return fmt.Errorf("workspace %q not found", wp.Result.Workspace)
	}
	target := resolveLastActive(app, ws)
	if target == "" {
		return fmt.Errorf("workspace %q has no live sessions", wp.Result.Workspace)
	}
	_ = app.WorkspaceStore.SetLastActive(wp.Result.Workspace, target)
	return session.Attach(app.Runner, target)
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
