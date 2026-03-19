package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/donjor/zmux/internal/bar"
	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/theme"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui/views"
)

// wizardStep identifies the current step in the init wizard.
type wizardStep int

const (
	stepWelcome wizardStep = iota
	stepDepCheck
	stepDetectTargets
	stepTheme
	stepBarPreset
	stepSyncTarget
	stepSummary
	stepWriting
	stepSuccess
)

// wizardKeymap defines keybindings for the init wizard.
var wizardKeys = struct {
	Quit    key.Binding
	Enter   key.Binding
	Back    key.Binding
	Up      key.Binding
	Down    key.Binding
	TabBack key.Binding
}{
	Quit: key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "cancel"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "next"),
	),
	Back: key.NewBinding(
		key.WithKeys("shift+tab"),
		key.WithHelp("shift+tab", "back"),
	),
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("up/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("down/j", "down"),
	),
	TabBack: key.NewBinding(
		key.WithKeys("b"),
		key.WithHelp("b", "back"),
	),
}

// WizardDeps holds detected dependency information.
type WizardDeps struct {
	TmuxVersion    string
	ClipboardTool  string
	HasGhostty     bool
	HasNvim        bool
	GhosttyPath    string
}

// WizardModel is the bubbletea model for the zmux init wizard.
type WizardModel struct {
	step     wizardStep
	styles   Styles
	width    int
	height   int
	version  string
	fs       config.FS

	// Dependency detection
	deps WizardDeps

	// Choices
	themes       []theme.ThemeInfo
	themeCursor  int
	chosenTheme  string

	presets       []bar.Preset
	presetCursor  int
	chosenPreset  string

	syncTargets  []string
	syncCursor   int
	chosenSync   string

	// Result
	Cancelled  bool
	Done       bool
	Error      error
	Copied     bool // whether the user copied the restart command

	// For rendering previews
	resolver *theme.Resolver
}

// NewWizardModel creates a new init wizard model.
func NewWizardModel(fs config.FS, resolver *theme.Resolver, ver string, styles Styles) WizardModel {
	themes := resolver.List()

	// Default to ayu-dark if available.
	themeCursor := 0
	for i, ti := range themes {
		if ti.Name == "ayu-dark" {
			themeCursor = i
			break
		}
	}

	presets := bar.AllPresets()

	return WizardModel{
		step:         stepWelcome,
		styles:       styles,
		version:      ver,
		fs:           fs,
		resolver:     resolver,
		themes:       themes,
		themeCursor:  themeCursor,
		chosenTheme:  "ayu-dark",
		presets:      presets,
		presetCursor: 0,
		chosenPreset: "default",
		syncTargets:  []string{"none"},
		syncCursor:   0,
		chosenSync:   "none",
	}
}

// detectDepsMsg carries dependency detection results.
type detectDepsMsg struct {
	deps WizardDeps
}

// configWrittenMsg signals that config was written successfully.
type configWrittenMsg struct{}

// configWriteErrMsg signals that config writing failed.
type configWriteErrMsg struct {
	err error
}

// Init starts dependency detection.
func (m WizardModel) Init() tea.Cmd {
	return nil
}

// Update handles messages and user input.
func (m WizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case detectDepsMsg:
		m.deps = msg.deps
		// Build sync targets based on detection.
		m.syncTargets = []string{"none"}
		if m.deps.HasGhostty {
			m.syncTargets = append(m.syncTargets, "ghostty")
		}
		if m.deps.HasNvim {
			m.syncTargets = append(m.syncTargets, "nvim")
		}
		return m, nil

	case configWrittenMsg:
		m.step = stepSuccess
		m.Done = true
		return m, nil

	case configWriteErrMsg:
		m.Error = msg.err
		m.step = stepSuccess
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m WizardModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Cancel at any point.
	if key.Matches(msg, wizardKeys.Quit) {
		m.Cancelled = true
		return m, tea.Quit
	}

	// Back navigation (except on welcome and success/writing steps).
	if (key.Matches(msg, wizardKeys.Back) || key.Matches(msg, wizardKeys.TabBack)) &&
		m.step > stepWelcome && m.step < stepWriting {
		// On steps with list selection, 'b' is only back if not in a list context
		// that uses that key. We reserve shift+tab always, 'b' always.
		m.step--
		return m, nil
	}

	switch m.step {
	case stepWelcome:
		return m.handleWelcome(msg)
	case stepDepCheck:
		return m.handleDepCheck(msg)
	case stepDetectTargets:
		return m.handleDetectTargets(msg)
	case stepTheme:
		return m.handleThemeStep(msg)
	case stepBarPreset:
		return m.handleBarPreset(msg)
	case stepSyncTarget:
		return m.handleSyncTarget(msg)
	case stepSummary:
		return m.handleSummary(msg)
	case stepSuccess:
		if key.Matches(msg, wizardKeys.Enter) || msg.String() == "q" {
			return m, tea.Quit
		}
		if (msg.String() == "c" || msg.String() == "y") && !m.Copied {
			cmd := restartCmd()
			if err := copyToClipboard(cmd); err == nil {
				m.Copied = true
			}
			return m, nil
		}
	}

	return m, nil
}

func (m WizardModel) handleWelcome(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, wizardKeys.Enter) {
		m.step = stepDepCheck
		return m, m.detectDeps
	}
	return m, nil
}

func (m WizardModel) handleDepCheck(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, wizardKeys.Enter) {
		m.step = stepDetectTargets
		return m, nil
	}
	return m, nil
}

func (m WizardModel) handleDetectTargets(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, wizardKeys.Enter) {
		m.step = stepTheme
		return m, nil
	}
	return m, nil
}

func (m WizardModel) handleThemeStep(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, wizardKeys.Up):
		if m.themeCursor > 0 {
			m.themeCursor--
		}
		return m, nil
	case key.Matches(msg, wizardKeys.Down):
		if m.themeCursor < len(m.themes)-1 {
			m.themeCursor++
		}
		return m, nil
	case key.Matches(msg, wizardKeys.Enter):
		if m.themeCursor < len(m.themes) {
			m.chosenTheme = m.themes[m.themeCursor].Name
		}
		m.step = stepBarPreset
		return m, nil
	}
	return m, nil
}

func (m WizardModel) handleBarPreset(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, wizardKeys.Up):
		if m.presetCursor > 0 {
			m.presetCursor--
		}
		return m, nil
	case key.Matches(msg, wizardKeys.Down):
		if m.presetCursor < len(m.presets)-1 {
			m.presetCursor++
		}
		return m, nil
	case key.Matches(msg, wizardKeys.Enter):
		if m.presetCursor < len(m.presets) {
			m.chosenPreset = m.presets[m.presetCursor].String()
		}
		m.step = stepSyncTarget
		return m, nil
	}
	return m, nil
}

func (m WizardModel) handleSyncTarget(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, wizardKeys.Up):
		if m.syncCursor > 0 {
			m.syncCursor--
		}
		return m, nil
	case key.Matches(msg, wizardKeys.Down):
		if m.syncCursor < len(m.syncTargets)-1 {
			m.syncCursor++
		}
		return m, nil
	case key.Matches(msg, wizardKeys.Enter):
		if m.syncCursor < len(m.syncTargets) {
			m.chosenSync = m.syncTargets[m.syncCursor]
		}
		m.step = stepSummary
		return m, nil
	}
	return m, nil
}

func (m WizardModel) handleSummary(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, wizardKeys.Enter) {
		m.step = stepWriting
		return m, m.writeConfig
	}
	return m, nil
}

// detectDeps detects system dependencies.
func (m WizardModel) detectDeps() tea.Msg {
	var deps WizardDeps

	// Check tmux version.
	if out, err := exec.Command("tmux", "-V").Output(); err == nil {
		ver := strings.TrimSpace(string(out))
		ver = strings.TrimPrefix(ver, "tmux ")
		deps.TmuxVersion = ver
	}

	// Check clipboard.
	deps.ClipboardTool = tmux.DetectClipboard()

	// Check ghostty config.
	home, err := m.fs.UserHomeDir()
	if err == nil {
		xdgConfig := os.Getenv("XDG_CONFIG_HOME")
		if xdgConfig == "" {
			xdgConfig = filepath.Join(home, ".config")
		}
		ghosttyPath := filepath.Join(xdgConfig, "ghostty", "config")
		if _, err := m.fs.Stat(ghosttyPath); err == nil {
			deps.HasGhostty = true
			deps.GhosttyPath = ghosttyPath
		}
	}

	// Check nvim.
	if _, err := exec.LookPath("nvim"); err == nil {
		deps.HasNvim = true
	}

	return detectDepsMsg{deps: deps}
}

// writeConfig writes the config file and creates directories.
func (m WizardModel) writeConfig() tea.Msg {
	home, err := m.fs.UserHomeDir()
	if err != nil {
		return configWriteErrMsg{err: fmt.Errorf("get home dir: %w", err)}
	}

	// Create directories.
	dirs := []string{
		filepath.Join(home, ".zmux"),
		filepath.Join(home, ".zmux", "themes"),
		filepath.Join(home, ".zmux", "templates"),
	}
	for _, dir := range dirs {
		if err := m.fs.MkdirAll(dir, 0o755); err != nil {
			return configWriteErrMsg{err: fmt.Errorf("create dir %s: %w", dir, err)}
		}
	}

	// Build config.
	cfg := config.Config{
		Theme:  m.chosenTheme,
		Prefix: "C-Space",
		Bar: config.BarConfig{
			Preset: m.chosenPreset,
		},
		Sessions: config.SessionsConfig{
			AutoCleanupTmp: true,
		},
		Templates: config.TemplatesConfig{
			Paths: []string{"~/.zmux/templates"},
		},
		Sync: config.SyncConfig{
			Target:        m.chosenSync,
			GhosttyConfig: "auto",
		},
	}

	// Write config.
	cfgPath := filepath.Join(home, ".zmux.toml")
	if err := config.Save(m.fs, cfgPath, cfg); err != nil {
		return configWriteErrMsg{err: fmt.Errorf("save config: %w", err)}
	}

	// Generate and write tmux.conf.
	zmuxBin, _ := os.Executable()
	if zmuxBin == "" {
		zmuxBin = "zmux"
	}

	// Resolve theme palette for conf generation.
	t, err := m.resolver.Resolve(m.chosenTheme)
	if err == nil {
		palette := t.SemanticPalette()
		confContent := tmux.GenerateConf(&cfg, &palette, zmuxBin)
		confPath := filepath.Join(home, ".tmux.conf")
		if writeErr := tmux.WriteConf(m.fs, confPath, confContent); writeErr != nil {
			return configWriteErrMsg{err: fmt.Errorf("write tmux.conf: %w", writeErr)}
		}
	}

	return configWrittenMsg{}
}

// View renders the wizard UI.
func (m WizardModel) View() string {
	if m.Cancelled {
		return ""
	}

	var b strings.Builder

	// Step indicator.
	stepNames := []string{
		"Welcome", "Dependencies", "Sync Targets", "Theme",
		"Status Bar", "Sync Target", "Summary", "Writing...", "Done",
	}
	currentName := "Unknown"
	if int(m.step) < len(stepNames) {
		currentName = stepNames[m.step]
	}

	progress := m.styles.Dim.Render(fmt.Sprintf("  [%d/%d] %s", m.step+1, len(stepNames), currentName))
	b.WriteString(progress + "\n\n")

	switch m.step {
	case stepWelcome:
		b.WriteString(m.viewWelcome())
	case stepDepCheck:
		b.WriteString(m.viewDepCheck())
	case stepDetectTargets:
		b.WriteString(m.viewDetectTargets())
	case stepTheme:
		b.WriteString(m.viewTheme())
	case stepBarPreset:
		b.WriteString(m.viewBarPreset())
	case stepSyncTarget:
		b.WriteString(m.viewSyncTarget())
	case stepSummary:
		b.WriteString(m.viewSummary())
	case stepWriting:
		b.WriteString(m.viewWriting())
	case stepSuccess:
		b.WriteString(m.viewSuccess())
	}

	// Help bar.
	b.WriteString("\n")
	b.WriteString(m.viewHelp())

	return b.String()
}

func (m WizardModel) viewWelcome() string {
	var b strings.Builder

	title := m.styles.Title.Render("zmux init")
	ver := m.styles.Muted.Render(fmt.Sprintf(" v%s", m.version))
	b.WriteString("  " + title + ver + "\n\n")

	b.WriteString(m.styles.Normal.Render("  Welcome to zmux! This wizard will help you set up:") + "\n\n")
	b.WriteString(m.styles.Accent.Render("    > ") + m.styles.Normal.Render("Config file (~/.zmux.toml)") + "\n")
	b.WriteString(m.styles.Accent.Render("    > ") + m.styles.Normal.Render("tmux configuration (~/.tmux.conf)") + "\n")
	b.WriteString(m.styles.Accent.Render("    > ") + m.styles.Normal.Render("Theme selection") + "\n")
	b.WriteString(m.styles.Accent.Render("    > ") + m.styles.Normal.Render("Status bar preset") + "\n")
	b.WriteString(m.styles.Accent.Render("    > ") + m.styles.Normal.Render("Sync target configuration") + "\n")
	b.WriteString(m.styles.Accent.Render("    > ") + m.styles.Normal.Render("User directories (~/.zmux/themes/, ~/.zmux/templates/)") + "\n")

	b.WriteString("\n")
	b.WriteString(m.styles.Muted.Render("  Press Enter to begin.") + "\n")

	return b.String()
}

func (m WizardModel) viewDepCheck() string {
	var b strings.Builder

	b.WriteString("  " + m.styles.Title.Render("Dependency Check") + "\n\n")

	depStyles := views.DepCheckStyles{
		Success: m.styles.Success,
		Error:   m.styles.Error,
		Normal:  m.styles.Normal,
		Muted:   m.styles.Muted,
	}
	b.WriteString(views.RenderDepCheck(m.deps.TmuxVersion, m.deps.ClipboardTool, depStyles))

	if m.deps.TmuxVersion == "" {
		b.WriteString("\n")
		b.WriteString(m.styles.Error.Render("  tmux is required. Please install tmux >= 3.2") + "\n")
	}

	b.WriteString("\n")
	b.WriteString(m.styles.Muted.Render("  Press Enter to continue.") + "\n")

	return b.String()
}

func (m WizardModel) viewDetectTargets() string {
	var b strings.Builder

	b.WriteString("  " + m.styles.Title.Render("Detected Sync Targets") + "\n\n")

	if m.deps.HasGhostty {
		b.WriteString(m.styles.Success.Render("  [ok]"))
		b.WriteString(m.styles.Normal.Render("  Ghostty config found") + "\n")
	} else {
		b.WriteString(m.styles.Muted.Render("  [--]"))
		b.WriteString(m.styles.Muted.Render("  Ghostty config not found") + "\n")
	}

	if m.deps.HasNvim {
		b.WriteString(m.styles.Success.Render("  [ok]"))
		b.WriteString(m.styles.Normal.Render("  Neovim found") + "\n")
	} else {
		b.WriteString(m.styles.Muted.Render("  [--]"))
		b.WriteString(m.styles.Muted.Render("  Neovim not found") + "\n")
	}

	b.WriteString("\n")
	b.WriteString(m.styles.Muted.Render("  Press Enter to continue.") + "\n")

	return b.String()
}

func (m WizardModel) viewTheme() string {
	var b strings.Builder

	b.WriteString("  " + m.styles.Title.Render("Choose a Theme") + "\n\n")

	if len(m.themes) == 0 {
		b.WriteString(m.styles.Muted.Render("  No themes available.") + "\n")
		return b.String()
	}

	// Calculate visible window.
	availableHeight := m.height - 12
	if availableHeight < 5 {
		availableHeight = 10
	}

	start := 0
	if m.themeCursor >= availableHeight {
		start = m.themeCursor - availableHeight + 1
	}
	end := start + availableHeight
	if end > len(m.themes) {
		end = len(m.themes)
	}

	for i := start; i < end; i++ {
		cursor := "  "
		if i == m.themeCursor {
			cursor = m.styles.Accent.Render("> ")
		}

		nameStyle := m.styles.Normal
		if i == m.themeCursor {
			nameStyle = m.styles.Selected
		}
		name := nameStyle.Render(m.themes[i].Name)

		var sourceTag string
		switch m.themes[i].Source {
		case theme.SourceBundled:
			sourceTag = m.styles.Info.Render(" [bundled]")
		case theme.SourceUser:
			sourceTag = m.styles.Success.Render(" [user]")
		}

		b.WriteString("  " + cursor + name + sourceTag + "\n")
	}

	if start > 0 {
		b.WriteString(m.styles.Dim.Render("    ... more above") + "\n")
	}
	if end < len(m.themes) {
		b.WriteString(m.styles.Dim.Render("    ... more below") + "\n")
	}

	// Show swatch for selected theme.
	if m.themeCursor < len(m.themes) {
		t, err := m.resolver.Resolve(m.themes[m.themeCursor].Name)
		if err == nil {
			palette := t.SemanticPalette()
			width := m.width
			if width <= 0 {
				width = 80
			}
			swatch := views.RenderSwatch(&palette, width)
			if swatch != "" {
				b.WriteString("\n" + swatch + "\n")
			}
		}
	}

	return b.String()
}

func (m WizardModel) viewBarPreset() string {
	var b strings.Builder

	b.WriteString("  " + m.styles.Title.Render("Choose a Status Bar Preset") + "\n\n")

	// Get palette for previews.
	var palette *theme.Palette
	if m.chosenTheme != "" {
		t, err := m.resolver.Resolve(m.chosenTheme)
		if err == nil {
			p := t.SemanticPalette()
			palette = &p
		}
	}

	for i, p := range m.presets {
		cursor := "  "
		if i == m.presetCursor {
			cursor = m.styles.Accent.Render("> ")
		}

		nameStyle := m.styles.Normal
		if i == m.presetCursor {
			nameStyle = m.styles.Selected
		}
		name := nameStyle.Render(p.String())
		b.WriteString("  " + cursor + name + "\n")

		// ANSI preview.
		if palette != nil {
			preview := bar.RenderPreview(p, palette)
			b.WriteString("    " + preview + "\n")
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (m WizardModel) viewSyncTarget() string {
	var b strings.Builder

	b.WriteString("  " + m.styles.Title.Render("Choose a Sync Target") + "\n\n")
	b.WriteString(m.styles.Muted.Render("  Sync pulls your theme from another app.") + "\n\n")

	for i, target := range m.syncTargets {
		cursor := "  "
		if i == m.syncCursor {
			cursor = m.styles.Accent.Render("> ")
		}

		nameStyle := m.styles.Normal
		if i == m.syncCursor {
			nameStyle = m.styles.Selected
		}

		desc := ""
		switch target {
		case "none":
			desc = m.styles.Muted.Render(" (manual theme management)")
		case "ghostty":
			desc = m.styles.Muted.Render(" (sync from Ghostty terminal)")
		case "nvim":
			desc = m.styles.Muted.Render(" (sync from Neovim colorscheme)")
		}

		b.WriteString("  " + cursor + nameStyle.Render(target) + desc + "\n")
	}

	return b.String()
}

func (m WizardModel) viewSummary() string {
	var b strings.Builder

	b.WriteString("  " + m.styles.Title.Render("Configuration Summary") + "\n\n")

	b.WriteString(m.styles.Accent.Render("  Theme:      ") + m.styles.Normal.Render(m.chosenTheme) + "\n")
	b.WriteString(m.styles.Accent.Render("  Bar preset: ") + m.styles.Normal.Render(m.chosenPreset) + "\n")
	b.WriteString(m.styles.Accent.Render("  Prefix:     ") + m.styles.Normal.Render("Ctrl+Space") + "\n")
	b.WriteString(m.styles.Accent.Render("  Sync:       ") + m.styles.Normal.Render(m.chosenSync) + "\n")

	b.WriteString("\n")
	b.WriteString(m.styles.Normal.Render("  This will create:") + "\n")
	b.WriteString(m.styles.Muted.Render("    ~/.zmux.toml") + "\n")
	b.WriteString(m.styles.Muted.Render("    ~/.tmux.conf") + "\n")
	b.WriteString(m.styles.Muted.Render("    ~/.zmux/themes/") + "\n")
	b.WriteString(m.styles.Muted.Render("    ~/.zmux/templates/") + "\n")

	b.WriteString("\n")
	b.WriteString(m.styles.Normal.Render("  Press Enter to write configuration.") + "\n")

	return b.String()
}

func (m WizardModel) viewWriting() string {
	return "  " + m.styles.Accent.Render("Writing configuration...") + "\n"
}

func (m WizardModel) viewSuccess() string {
	var b strings.Builder

	if m.Error != nil {
		b.WriteString("  " + m.styles.Error.Render("Error writing config:") + "\n")
		b.WriteString("  " + m.styles.Normal.Render(m.Error.Error()) + "\n")
		b.WriteString("\n")
		b.WriteString(m.styles.Muted.Render("  Press Enter or q to exit.") + "\n")
		return b.String()
	}

	b.WriteString("  " + m.styles.Success.Render("Configuration written successfully!") + "\n\n")

	cmd := restartCmd()
	b.WriteString(m.styles.Normal.Render("  Run this to apply:") + "\n\n")
	b.WriteString("    " + m.styles.Accent.Render(cmd) + "\n\n")

	if m.Copied {
		b.WriteString("  " + m.styles.Success.Render("Copied to clipboard!") + "\n")
	} else {
		b.WriteString(m.styles.Muted.Render("  c/y:copy  enter:exit") + "\n")
	}

	return b.String()
}

func (m WizardModel) viewHelp() string {
	parts := []string{"enter:next"}

	if m.step > stepWelcome && m.step < stepWriting {
		parts = append(parts, "shift+tab:back")
	}
	parts = append(parts, "ctrl+c:cancel")

	if m.step == stepTheme || m.step == stepBarPreset || m.step == stepSyncTarget {
		parts = append([]string{"j/k:navigate"}, parts...)
	}

	return m.styles.Help.Render("  " + strings.Join(parts, "  "))
}

// Step returns the current wizard step (for testing).
func (m WizardModel) Step() wizardStep { return m.step }

// RestartCmd returns the command the user should run after init.
// Exported so the caller can echo it after the TUI exits.
func RestartCmd() string { return restartCmd() }

func restartCmd() string {
	return "tmux source-file ~/.tmux.conf 2>/dev/null; exec $SHELL"
}

func copyToClipboard(text string) error {
	tools := []string{"wl-copy", "xclip", "pbcopy"}
	for _, tool := range tools {
		path, err := exec.LookPath(tool)
		if err != nil {
			continue
		}
		var cmd *exec.Cmd
		switch tool {
		case "xclip":
			cmd = exec.Command(path, "-selection", "clipboard")
		default:
			cmd = exec.Command(path)
		}
		cmd.Stdin = strings.NewReader(text)
		return cmd.Run()
	}
	return fmt.Errorf("no clipboard tool found")
}
