package recipeup

import (
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/recipe"
)

type screen int

const (
	screenBrowser screen = iota
	screenInput
	screenDryRun
	screenResult
	screenEditor
)

type Model struct {
	app *apppkg.App

	width  int
	height int

	screen screen
	defs   []recipe.Definition
	dirs   []string
	cursor int
	plan   recipe.Plan
	lints  []recipe.LintResult
	logs   []string
	err    error

	showHelp bool
	itemsRaw string
	values   recipeFormValues
	form     *huh.Form

	editor     textarea.Model
	editorPath string
	editorName string
}

type recipeFormValues struct {
	CWD       string
	Workspace string
	Session   string
	TabMode   string
	ItemsRaw  string
}

var (
	ink     = lipgloss.Color("#D9E0EE")
	muted   = lipgloss.Color("#7F849C")
	accent  = lipgloss.Color("#89B4FA")
	green   = lipgloss.Color("#A6E3A1")
	yellow  = lipgloss.Color("#F9E2AF")
	red     = lipgloss.Color("#F38BA8")
	surface = lipgloss.Color("#313244")

	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(accent)
	mutedStyle   = lipgloss.NewStyle().Foreground(muted)
	okStyle      = lipgloss.NewStyle().Foreground(green)
	warnStyle    = lipgloss.NewStyle().Foreground(yellow)
	errorStyle   = lipgloss.NewStyle().Foreground(red)
	commandStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#CBA6F7"))
	sectionStyle = lipgloss.NewStyle().Bold(true).Foreground(ink)
	panelStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(surface).Padding(1, 2)
	activeStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#11111B")).Background(accent).Bold(true).Padding(0, 1)
	chipStyle    = lipgloss.NewStyle().Foreground(ink).Background(surface).Padding(0, 1)
)

func Run(app *apppkg.App) error {
	_, err := tea.NewProgram(New(app)).Run()
	return err
}

func RunRecipe(app *apppkg.App, name string, items []string, opts recipe.PlanOptions) error {
	m := New(app)
	if !m.selectByName(name) {
		return fmt.Errorf("recipe %q not found", name)
	}
	next, _ := m.configureSelectedWith(opts, items)
	model, ok := next.(Model)
	if !ok {
		model = m
	}
	_, err := tea.NewProgram(model).Run()
	return err
}

func Snapshot(app *apppkg.App) (string, error) {
	m := New(app)
	m.width = 110
	m.height = 32
	return m.render(), nil
}

func SnapshotPlan(app *apppkg.App, name string, items []string) (string, error) {
	m := New(app)
	def, ok := recipe.Find(m.defs, name)
	if !ok {
		return "", fmt.Errorf("recipe %q not found", name)
	}
	p, err := plan(app, def.Recipe, recipe.PlanOptions{Items: items})
	if err != nil {
		return "", err
	}
	m.width = 110
	m.height = 32
	m.screen = screenDryRun
	m.plan = p
	return m.render(), nil
}

func New(app *apppkg.App) Model {
	m := Model{app: app, width: 100, height: 30}
	m.reload()
	return m
}

func (m Model) Init() tea.Cmd {
	if m.screen == screenInput && m.form != nil {
		return tea.Batch(tea.RequestWindowSize, m.form.Init())
	}
	return tea.RequestWindowSize
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = max(72, msg.Width)
		m.height = max(24, msg.Height)
		if m.screen == screenEditor {
			m.editor.SetWidth(max(40, m.width-8))
			m.editor.SetHeight(max(8, m.height-10))
		}
	case tea.KeyPressMsg:
		if m.screen == screenInput && m.form != nil {
			if msg.String() == "esc" {
				m.screen = screenBrowser
				m.err = nil
				return m, nil
			}
			updated, cmd := m.form.Update(msg)
			if f, ok := updated.(*huh.Form); ok {
				m.form = f
			}
			switch m.form.State {
			case huh.StateCompleted:
				if err := m.buildPlanFromValues(); err != nil {
					m.err = err
					m.screen = screenResult
				} else {
					m.screen = screenDryRun
				}
				return m, nil
			case huh.StateAborted:
				m.screen = screenBrowser
				return m, nil
			}
			return m, cmd
		}
		if m.screen == screenEditor {
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "esc":
				m.screen = screenBrowser
				m.err = nil
				return m, nil
			case "ctrl+s":
				m.saveEditor()
				return m, nil
			}
			var cmd tea.Cmd
			m.editor, cmd = m.editor.Update(msg)
			return m, cmd
		}
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			if m.screen == screenBrowser {
				return m, tea.Quit
			}
			m.screen = screenBrowser
			m.err = nil
		case "?", "h":
			m.showHelp = !m.showHelp
		case "up", "k":
			if m.screen == screenBrowser && m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.screen == screenBrowser && m.cursor < len(m.defs)-1 {
				m.cursor++
			}
		case "r":
			if m.screen == screenBrowser {
				m.reload()
			}
		case "l":
			if m.screen == screenBrowser {
				m.lintSelected()
			}
		case "f":
			if m.screen == screenBrowser {
				m.forkSelected()
			}
		case "n":
			if m.screen == screenBrowser {
				m.createStarter()
			}
		case "e":
			if m.screen == screenBrowser {
				m.editSelected()
			}
		case "enter":
			switch m.screen {
			case screenBrowser:
				return m.configureSelected()
			case screenDryRun:
				m.executePlan()
				m.screen = screenResult
			case screenResult:
				m.screen = screenBrowser
			}
		case "y":
			if m.screen == screenDryRun {
				m.executePlan()
				m.screen = screenResult
			}
		}
	}
	return m, nil
}

func (m Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.WindowTitle = "zmux recipes"
	return v
}

func (m *Model) reload() {
	cfg, err := loadConfig(m.app)
	if err != nil {
		cfg = config.DefaultConfig()
	}
	dirs := recipe.ConfiguredDirs(m.app.FS, m.app.Profile, cfg)
	defs, err := recipe.Load(m.app.FS, dirs, cfg.Recipes.Disabled)
	if err != nil {
		m.err = err
		return
	}
	m.dirs = dirs
	m.defs = defs
	m.lints = recipe.Lint(m.app.FS, dirs, nil)
	if m.cursor >= len(m.defs) {
		m.cursor = max(0, len(m.defs)-1)
	}
	m.logs = append(m.logs, fmt.Sprintf("loaded %d recipe(s)", len(defs)))
}

func (m *Model) selected() (recipe.Definition, bool) {
	if len(m.defs) == 0 || m.cursor < 0 || m.cursor >= len(m.defs) {
		return recipe.Definition{}, false
	}
	return m.defs[m.cursor], true
}

func (m *Model) selectByName(name string) bool {
	for i, def := range m.defs {
		if def.Recipe.Name == name {
			m.cursor = i
			return true
		}
	}
	return false
}

func (m Model) configureSelected() (tea.Model, tea.Cmd) {
	return m.configureSelectedWith(recipe.PlanOptions{}, nil)
}

func (m Model) configureSelectedWith(opts recipe.PlanOptions, items []string) (tea.Model, tea.Cmd) {
	def, ok := m.selected()
	if !ok {
		m.err = fmt.Errorf("no recipe selected")
		return m, nil
	}
	m.values = defaultFormValues(def.Recipe, opts, items)
	m.itemsRaw = m.values.ItemsRaw
	m.form = buildRecipeForm(def.Recipe, &m.values, m.width, m.height)
	m.screen = screenInput
	return m, m.form.Init()
}

func (m *Model) buildPlanFromValues() error {
	return m.buildPlanWithOptions(recipe.PlanOptions{
		CWD:       strings.TrimSpace(m.values.CWD),
		Workspace: strings.TrimSpace(m.values.Workspace),
		Session:   strings.TrimSpace(m.values.Session),
		TabMode:   strings.TrimSpace(m.values.TabMode),
		Items:     strings.Fields(m.values.ItemsRaw),
	})
}

func (m *Model) buildPlanWithOptions(opts recipe.PlanOptions) error {
	def, ok := m.selected()
	if !ok {
		return fmt.Errorf("no recipe selected")
	}
	p, err := plan(m.app, def.Recipe, opts)
	if err != nil {
		return err
	}
	m.plan = p
	return nil
}

func (m *Model) executePlan() {
	m.err = nil
	if err := recipe.Execute(m.app.Runner, m.app.WorkspaceStore, m.plan); err != nil {
		m.err = err
		m.logs = append(m.logs, "execution failed: "+err.Error())
		return
	}
	m.logs = append(m.logs, "executed "+m.plan.RecipeName)
}

func (m *Model) lintSelected() {
	def, ok := m.selected()
	if !ok {
		return
	}
	if result, ok := m.lintFor(def.Recipe.Name); ok && result.Err != nil {
		m.err = result.Err
		m.logs = append(m.logs, "lint failed: "+result.Err.Error())
		return
	}
	if err := recipe.Validate(def.Recipe); err != nil {
		m.err = err
		m.logs = append(m.logs, "lint failed: "+err.Error())
		return
	}
	m.err = nil
	m.logs = append(m.logs, "lint ok: "+def.Recipe.Name)
}

func (m *Model) forkSelected() {
	def, ok := m.selected()
	if !ok {
		return
	}
	path, err := recipe.Fork(m.app.FS, m.app.Profile, def, false)
	if err != nil {
		m.err = err
		m.logs = append(m.logs, "fork failed: "+err.Error())
		return
	}
	m.err = nil
	m.logs = append(m.logs, "forked to "+path)
	m.reload()
}

func (m Model) lintFor(name string) (recipe.LintResult, bool) {
	var okResult recipe.LintResult
	found := false
	for _, result := range m.lints {
		if result.Name == name {
			if result.Err != nil {
				return result, true
			}
			okResult = result
			found = true
		}
	}
	return okResult, found
}

func (m Model) lintErrorCount() int {
	count := 0
	for _, result := range m.lints {
		if result.Err != nil {
			count++
		}
	}
	return count
}

func (m *Model) editSelected() {
	def, ok := m.selected()
	if !ok {
		return
	}
	if def.Source == recipe.SourceBundled {
		path, err := recipe.Fork(m.app.FS, m.app.Profile, def, false)
		if err != nil {
			m.err = err
			return
		}
		m.logs = append(m.logs, "forked to "+path)
		m.reload()
		if !m.selectByName(def.Recipe.Name) {
			m.err = fmt.Errorf("forked recipe %q was not reloaded", def.Recipe.Name)
			return
		}
		def, _ = m.selected()
	}
	data := def.Raw
	if len(data) == 0 && def.Path != "" {
		raw, err := m.app.FS.ReadFile(def.Path)
		if err != nil {
			m.err = err
			return
		}
		data = raw
	}
	editor := textarea.New()
	editor.Prompt = "  "
	editor.ShowLineNumbers = true
	editor.SetWidth(max(40, m.width-8))
	editor.SetHeight(max(8, m.height-10))
	editor.SetValue(string(data))
	_ = editor.Focus()
	m.editor = editor
	m.editorPath = def.Path
	m.editorName = def.Recipe.Name
	m.err = nil
	m.screen = screenEditor
}

func (m *Model) saveEditor() {
	if m.editorPath == "" {
		m.err = fmt.Errorf("no editor path")
		return
	}
	data := []byte(m.editor.Value())
	if _, err := recipe.Parse(data); err != nil {
		m.err = err
		m.logs = append(m.logs, "save blocked: "+err.Error())
		return
	}
	if err := m.app.FS.WriteFile(m.editorPath, data, 0o644); err != nil {
		m.err = err
		m.logs = append(m.logs, "save failed: "+err.Error())
		return
	}
	m.err = nil
	m.logs = append(m.logs, "saved "+m.editorPath)
	m.reload()
}

func (m *Model) createStarter() {
	name := "local-recipe"
	path, err := recipe.CreateStarter(m.app.FS, m.app.Profile, name)
	if err != nil {
		m.err = err
		m.logs = append(m.logs, "create failed: "+err.Error())
		return
	}
	m.err = nil
	m.logs = append(m.logs, "created "+path)
	m.reload()
}

func defaultFormValues(r recipe.Recipe, opts recipe.PlanOptions, items []string) recipeFormValues {
	cwd := opts.CWD
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			cwd = os.Getenv("HOME")
		}
	}
	defaults := recipe.DefaultOptions(r, cwd)
	values := recipeFormValues{
		CWD:       defaults.CWD,
		Workspace: defaults.Workspace,
		Session:   defaults.Session,
		TabMode:   defaults.TabMode,
		ItemsRaw:  strings.Join(items, " "),
	}
	if opts.CWD != "" {
		values.CWD = opts.CWD
	}
	if opts.Workspace != "" {
		values.Workspace = opts.Workspace
	}
	if opts.Session != "" {
		values.Session = opts.Session
	}
	if opts.TabMode != "" {
		values.TabMode = opts.TabMode
	}
	if len(opts.Items) > 0 {
		values.ItemsRaw = strings.Join(opts.Items, " ")
	}
	return values
}

func buildRecipeForm(r recipe.Recipe, values *recipeFormValues, width int, height int) *huh.Form {
	fields := []huh.Field{
		huh.NewText().
			Title("Working directory").
			Description("Root used for relative commands and tab cwd.").
			Placeholder("~/donjor/vone").
			CharLimit(800).
			Value(&values.CWD),
		huh.NewText().
			Title("Workspace").
			Description("Workspace tag to create or reuse.").
			Placeholder("vone").
			CharLimit(120).
			Value(&values.Workspace),
		huh.NewText().
			Title("Session").
			Description("Primary session name for this run.").
			Placeholder("main").
			CharLimit(120).
			Value(&values.Session),
		huh.NewSelect[string]().
			Title("Tabs").
			Description("Run commands, type them ready at the prompt, or create empty tabs.").
			Options(
				huh.NewOption("run commands", recipe.TabModeRun),
				huh.NewOption("ready at prompt", recipe.TabModeReady),
				huh.NewOption("empty tabs", recipe.TabModeEmpty),
			).
			Value(&values.TabMode),
	}
	if needsItems(r) {
		title := r.Inputs.Prompt
		if title == "" {
			title = "Items"
		}
		fields = append(fields, huh.NewText().
			Title(title).
			Description("Space-separated positional items for fanout.").
			Placeholder("api web docs").
			CharLimit(600).
			Value(&values.ItemsRaw))
	}
	return huh.NewForm(
		huh.NewGroup(fields...),
	).WithTheme(huh.ThemeFunc(huh.ThemeCharm)).
		WithWidth(min(max(40, width-10), 78)).
		WithHeight(min(max(14, height-10), 20)).
		WithShowHelp(true)
}

func plan(app *apppkg.App, r recipe.Recipe, opts recipe.PlanOptions) (recipe.Plan, error) {
	dir, err := os.Getwd()
	if err != nil {
		dir = os.Getenv("HOME")
	}
	if opts.CWD == "" {
		opts.CWD = dir
	}
	opts.Bin = app.Profile.Name
	opts.InsideZmux = app.Runner.IsInsideTmux()
	state := recipe.State{Sessions: map[string]recipe.SessionState{}, Workspaces: map[string]recipe.WorkspaceState{}}
	if app.Runner.ServerRunning() {
		state, err = recipe.BuildState(app.Runner, app.WorkspaceStore)
		if err != nil {
			return recipe.Plan{}, err
		}
	}
	return recipe.PlanRecipe(r, opts, state)
}

func loadConfig(app *apppkg.App) (config.Config, error) {
	if !config.ConfigExists(app.FS) {
		return config.DefaultConfig(), nil
	}
	path := app.ConfigPath
	if path == "" {
		var err error
		path, err = config.ConfigPath(app.FS)
		if err != nil {
			return config.DefaultConfig(), err
		}
	}
	return config.Load(app.FS, path)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
