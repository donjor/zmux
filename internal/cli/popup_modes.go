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
	"github.com/donjor/zmux/internal/debug"
	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/source"
	tabspkg "github.com/donjor/zmux/internal/tabs"
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

// dimHostBehindPopup tints the host window's background while a blocking popup
// TUI is on screen, restoring it when the returned func runs (defer it right
// after calling). Best-effort focus polish: it reads the window's current
// window-style / window-active-style, overwrites both with the theme's BGDim
// background, and on restore puts the saved value back (or unsets it when there
// was none). Tints background only — text foreground is untouched.
//
// Called from inside the popup process (runNewDashboard / runPalette), which
// covers every popup entry point — the prefix+Space keybind, the bare `zmux`
// launch, and the palette→dashboard relaunch — with one seam, since they all
// converge on the same blocking p.Run(). A SIGKILL of the popup skips the
// restore; the next `zmux apply` / attach re-asserts the theme default.
//
// Returns a no-op when there's no palette (theme unresolved) or no tmux server
// (the sessionless dashboard runs outside tmux), so callers can defer
// unconditionally.
func dimHostBehindPopup(runner tmux.Runner, pal *theme.Palette) func() {
	if pal == nil || !runner.IsInsideTmux() {
		return func() {}
	}
	const target = "" // current window of the launching client

	prevActive, _ := runner.ShowWindowOption(target, "window-active-style")
	prevStyle, _ := runner.ShowWindowOption(target, "window-style")

	dim := "bg=" + pal.BGDim.Hex()
	_ = runner.SetWindowOption(target, "window-active-style", dim)
	_ = runner.SetWindowOption(target, "window-style", dim)

	return func() {
		restoreWindowStyle(runner, target, "window-active-style", prevActive)
		restoreWindowStyle(runner, target, "window-style", prevStyle)
	}
}

// restoreWindowStyle puts a saved window style back, or unsets the per-window
// override when there was none (falling back to the global/theme default).
func restoreWindowStyle(runner tmux.Runner, target, key, prev string) {
	if prev == "" {
		_ = runner.UnsetWindowOption(target, key)
		return
	}
	_ = runner.SetWindowOption(target, key, prev)
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

	// Parse initial tab (handle deprecated names). Outside tmux there is no
	// current session for the Session tab, so default to the global workspace
	// surface while still honoring an explicit --dashboard-tab.
	initialTab := resolveDashboardTab(dashboardTabFlag)
	if dashboardTabFlag == "" && !app.Runner.IsInsideTmux() {
		initialTab = dashboard.TabWorkspaces
	}

	model := dashboard.NewDashboardApp(services, tabImpls, initialTab)

	// Dim the host window behind the popup while the dashboard is open (no-op
	// outside tmux / without a palette). Covers the keybind + bare-launch paths.
	restoreDim := dimHostBehindPopup(app.Runner, pal)
	defer restoreDim()

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
	styles, pal, resolver := loadActiveStyles(app)

	// Build registry with all providers.
	reg := palette.NewDefaultRegistry(app.Runner, resolver, app.FS)

	model := palette.NewPaletteModel(reg, styles)

	// Dim the host window behind the palette popup while it's open.
	restoreDim := dimHostBehindPopup(app.Runner, pal)
	defer restoreDim()

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

	return applyTabPickerResult(app, sessionName, tp.Result)
}

// applyTabPickerResult executes the picker's chosen action against tmux.
// Split out of runTabPicker so the action→tmux mapping is testable without
// driving a tea program.
func applyTabPickerResult(app *apppkg.App, sessionName string, res tabpicker.TabPickerResult) error {
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
		if err := app.Runner.SelectWindow(target, res.Index); err != nil {
			return err
		}
		touchPickerMRU(app, target, res.TabID)
		return nil
	case "select-pane":
		// A rider tab: focus its host window, then the pane itself.
		if session.RootName(target) != sessionName {
			if err := session.Switch(app.Runner, target); err != nil {
				return err
			}
		}
		if err := app.Runner.SelectWindow(target, res.Index); err != nil {
			return err
		}
		if err := app.Runner.SelectPane(res.Pane); err != nil {
			return err
		}
		touchPickerMRU(app, target, res.TabID)
		return nil
	case "show":
		return showPickedTab(app, sessionName, res.TabID)
	case "new":
		dir, _ := os.Getwd()
		_, err := app.Runner.NewWindow(target, res.Name, dir)
		return err
	case "rename":
		old := fmt.Sprintf("%d", res.Index)
		return app.Runner.RenameWindow(target, old, res.Name)
	case "close":
		if err := guardNotLastTab(app.Runner, target); err != nil {
			return err
		}
		return app.Runner.KillWindow(target, res.Index)
	case "close-pane":
		return app.Runner.KillPane(res.Pane)
	case "swap":
		return app.Runner.SwapWindow(target, res.Index, res.Index+res.Delta)
	}

	return nil
}

// touchPickerMRU records a picker selection in the session's tab MRU —
// logical tabs only; raw windows have no stable id to remember.
func touchPickerMRU(app *apppkg.App, sessionName, tabID string) {
	if tabID == "" {
		return
	}
	if err := tabspkg.TouchMRU(app.Runner, sessionName, tabID); err != nil {
		debug.Log("tabpicker: mru touch failed", "err", err)
	}
}

// showPickedTab returns a hidden tab from the dock to its origin and focuses
// it: same clone-block + Show + epilogue as `zmux tab show`, then a re-scan
// finds where the window landed so the client can jump to it.
func showPickedTab(app *apppkg.App, sessionName, tabID string) error {
	all, err := tabspkg.ListLogicalTabs(app.Runner)
	if err != nil {
		return err
	}
	t := tabspkg.ByID(all, tabID)
	if t == nil {
		return fmt.Errorf("hidden tab no longer exists")
	}
	if t.Placement == tabspkg.PlacementDock {
		if err := blockOnAttachedClones(app, t.OriginSession); err != nil {
			return err
		}
	}
	origin, err := tabspkg.Show(app.Runner, t)
	if err != nil {
		return err
	}
	placementEpilogue(app)
	if session.RootName(origin) != sessionName {
		if err := session.Switch(app.Runner, origin); err != nil {
			return err
		}
	}
	if all, err := tabspkg.ListLogicalTabs(app.Runner); err == nil {
		if shown := tabspkg.ByID(all, tabID); shown != nil && shown.Session == origin {
			if err := app.Runner.SelectWindow(origin, shown.WindowIndex); err != nil {
				return err
			}
		}
	}
	touchPickerMRU(app, origin, tabID)
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
	if target.TmuxName == "" {
		return fmt.Errorf("workspace %q has no live sessions", wp.Result.Workspace)
	}
	_ = app.WorkspaceStore.SetLastActive(wp.Result.Workspace, target.ID)
	return attachOwnedSession(app, target.TmuxName)
}

// handleDashboardResult applies the action chosen from inside the dashboard
// popup or from the sessionless dashboard. Runs after the dashboard's
// bubbletea Program returns.
func handleDashboardResult(app *apppkg.App, action, chosen string) error {
	switch action {
	case "switch":
		if chosen == "" {
			return nil
		}
		if !app.Runner.IsInsideTmux() {
			return attachOwnedSession(app, chosen)
		}
		return session.Switch(app.Runner, chosen)

	case "focus":
		if chosen == "" || app.Runner.IsInsideTmux() {
			return nil
		}
		return attachOwnedSession(app, chosen)

	case "new":
		name := session.NextTmpName(app.Runner)
		dir, err := os.Getwd()
		if err != nil {
			dir = "."
		}
		if err := session.Create(app.Runner, name, dir); err != nil {
			return err
		}
		if !app.Runner.IsInsideTmux() {
			return attachOwnedSession(app, name)
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
