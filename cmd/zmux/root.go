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
	"github.com/donjor/zmux/internal/theme"
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
var tabPickerFlag bool

var rootCmd = &cobra.Command{
	Use:   "zmux",
	Short: "An opinionated, all-in-one tmux management wrapper",
	Long:  "zmux replaces tmux's sharp edges with a beautiful, interactive experience.",
	SilenceUsage:  true,
	SilenceErrors: true,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if tabPickerFlag {
			return runTabPicker()
		}

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

		if len(args) > 0 {
			return resolveShorthand(args)
		}

		if app.Runner.IsInsideTmux() {
			return launchDashboardPopup()
		}

		return runSessionPicker()
	},
}

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

// resolveShorthand handles `zmux <name>` and `zmux <ws> <session>` dispatch.
//
// Two-arg form is strict: the workspace must exist and contain the session,
// otherwise it errors with a helpful hint.
//
// Single-arg form is workspace-first: checks if <name> is a workspace and
// attaches to its last-active session. Falls back to session attach/create
// if no matching workspace.
func resolveShorthand(args []string) error {
	if len(args) == 2 {
		wsName := args[0]
		sessName := args[1]
		ws, _ := app.WorkspaceStore.GetWorkspace(wsName)
		if ws == nil {
			return fmt.Errorf("workspace %q not found — use zmux new %s %s to create", wsName, wsName, sessName)
		}
		if app.Runner.HasSession(sessName) {
			_ = app.WorkspaceStore.SetLastActive(wsName, sessName)
			return session.Attach(app.Runner, sessName)
		}
		return fmt.Errorf("session %q not found in workspace %q", sessName, wsName)
	}

	// Single arg: workspace-first, then session fallback.
	name := args[0]
	if ws, _ := app.WorkspaceStore.GetWorkspace(name); ws != nil {
		if target := resolveLastActive(ws); target != "" {
			_ = app.WorkspaceStore.SetLastActive(name, target)
			return session.Attach(app.Runner, target)
		}
		// Workspace exists but no live sessions — fall through to create.
	}

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

func launchDashboardPopup() error {
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
func loadActiveStyles() (tui.Styles, *theme.Palette, *theme.Resolver) {
	resolver, err := newResolver(app.FS)
	if err != nil {
		return tui.DefaultStyles(), nil, nil
	}
	cfg, err := loadConfig(app.FS)
	if err != nil {
		return tui.DefaultStyles(), nil, resolver
	}
	t, err := resolver.Resolve(cfg.Theme)
	if err != nil {
		return tui.DefaultStyles(), nil, resolver
	}
	p := t.SemanticPalette()
	return tui.NewStyles(&p), &p, resolver
}

func runDashboard() error {
	return runNewDashboard()
}

func runNewDashboard() error {
	styles, pal, resolver := loadActiveStyles()

	services := dashboard.Services{
		Runner:   app.Runner,
		FS:       app.FS,
		Styles:   styles,
		Palette:  pal,
		Resolver: resolver,
	}

	// Shared workspace data loader for Session and Workspaces tabs.
	wsLoader := func() []tui.WorkspaceViewModel {
		workspaces, err := app.WorkspaceStore.ListWorkspaces()
		if err != nil {
			return nil
		}
		sessions, _ := session.ListSessions(app.Runner)
		return tui.BuildWorkspaceViewModels(workspaces, sessions)
	}

	// Build tabs (order: Session, Workspaces, Themes, Settings, Help).
	tabImpls := []dashboard.Tab{
		tabs.NewCurrentTab(app.Runner, styles, wsLoader, app.WorkspaceStore),
		tabs.NewSessionsTab(app.Runner, styles, wsLoader, app.WorkspaceStore),
		tabs.NewThemesTab(resolver, app.FS, app.Runner, styles),
		tabs.NewSettingsTab(resolver, app.FS, app.Runner, styles),
		tabs.NewHelpTab(styles),
	}

	// Parse initial tab (handle deprecated names).
	initialTab := resolveDashboardTab(dashboardTabFlag)

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
	styles, _, resolver := loadActiveStyles()

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

func runTabPicker() error {
	// Get current session name.
	sessionName, err := app.Runner.DisplayMessage("", "#{session_name}")
	if err != nil {
		return fmt.Errorf("not inside a tmux session")
	}

	styles, _, _ := loadActiveStyles()
	model := tui.NewTabPickerModel(app.Runner, sessionName, styles)

	p := tea.NewProgram(model, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return fmt.Errorf("tab picker: %w", err)
	}

	tp, ok := result.(tui.TabPickerModel)
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

	styles, _, _ := loadActiveStyles()
	model := tui.NewPickerModel(app.Runner, styles)
	model.SetTemplates(templates)
	model.SetWorkspaceStore(app.WorkspaceStore)
	model.SetWorkspaceDataLoader(func() []tui.WorkspaceViewModel {
		// Reconcile before loading state.
		if roots := liveRootSet(); roots != nil {
			_ = app.WorkspaceStore.Reconcile(roots)
		}
		workspaces, err := app.WorkspaceStore.ListWorkspaces()
		if err != nil {
			return nil
		}
		sessions, _ := session.ListSessions(app.Runner)
		return tui.BuildWorkspaceViewModels(workspaces, sessions)
	})

	p := tea.NewProgram(model, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return fmt.Errorf("workspace picker: %w", err)
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
		// If the session already exists (e.g. "main" across workspaces),
		// pick the next available name based on the workspace.
		if app.Runner.HasSession(name) {
			if res.Workspace != "" {
				name = nextSessionName(name, res.Workspace)
			} else {
				name = session.NextTmpName(app.Runner)
			}
		}
		if err := session.Create(app.Runner, name, dir); err != nil {
			return err
		}
		// Auto-assign to workspace if specified.
		if res.Workspace != "" {
			_ = app.WorkspaceStore.AddSession(res.Workspace, name)
			_ = app.WorkspaceStore.SetLastActive(res.Workspace, name)
		}
		return session.Attach(app.Runner, name)

	case "workspace-create":
		wsName := res.Workspace
		if err := app.WorkspaceStore.CreateWorkspace(wsName, dir); err != nil {
			return err
		}
		// Create "main" session in the new workspace. If "main" is taken,
		// use workspace-qualified fallback.
		sessName := "main"
		if app.Runner.HasSession(sessName) {
			sessName = nextSessionName(sessName, wsName)
		}
		if err := session.Create(app.Runner, sessName, dir); err != nil {
			return err
		}
		_ = app.WorkspaceStore.AddSession(wsName, sessName)
		_ = app.WorkspaceStore.SetLastActive(wsName, sessName)
		return session.Attach(app.Runner, sessName)

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
			if len(templates) == 0 {
				return fmt.Errorf("no templates available")
			}
			tmpl = &templates[0]
		}

		if err := session.CreateFromTemplate(app.Runner, *tmpl, name, dir); err != nil {
			return err
		}
		// Auto-assign to workspace if specified.
		if res.Workspace != "" {
			_ = app.WorkspaceStore.AddSession(res.Workspace, name)
		}
		return session.Attach(app.Runner, name)

	case "overmind-connect":
		src := res.ExternalSource
		if src != nil && src.Overmind != nil {
			return source.Connect(src.Overmind.ControlSocket, res.Session)
		}
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

	case "workspace-focus":
		ws, err := app.WorkspaceStore.GetWorkspace(res.Workspace)
		if err != nil || ws == nil {
			return fmt.Errorf("workspace %q not found", res.Workspace)
		}
		target := resolveLastActive(ws)
		if target == "" {
			return fmt.Errorf("workspace %q has no live sessions", res.Workspace)
		}
		_ = app.WorkspaceStore.SetLastActive(res.Workspace, target)
		return session.Attach(app.Runner, target)
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
	rootCmd.PersistentFlags().BoolVar(&tabPickerFlag, "tab-picker", false, "render tab picker directly (used by Alt+`)")
	rootCmd.AddCommand(versionCmd)
}
