package tabs

import (
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"

	"github.com/donjor/zmux/internal/bar"
	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/theme"
	"github.com/donjor/zmux/internal/tui"
	"github.com/donjor/zmux/internal/tui/dashboard"
	"github.com/donjor/zmux/internal/tui/views"
)

// ── Shared config row types ──

// configRowKind defines how a config row is edited.
type configRowKind int

const (
	configRowText   configRowKind = iota // free-form text input
	configRowCycle                       // cycle through predefined options
	configRowToggle                      // true/false toggle
)

// configRow describes a single editable config field.
type configRow struct {
	label    string
	key      string // dotted key for display, e.g. "bar.preset"
	kind     configRowKind
	options  []string // for cycle kind
	getValue func(config.Config) string
	setValue func(*config.Config, string)
}

// cycleOption returns the next option in the list after current.
func cycleOption(options []string, current string) string {
	for i, opt := range options {
		if opt == current {
			return options[(i+1)%len(options)]
		}
	}
	if len(options) > 0 {
		return options[0]
	}
	return current
}

// settingsReqCounter is a monotonic counter for async correctness.
var settingsReqCounter atomic.Int64

// settingsSection identifies the active section within the settings tab.
type settingsSection int

const (
	sectionTheme   settingsSection = iota
	sectionBar
	sectionGeneral
)

var sectionNames = [...]string{
	sectionTheme:   "Theme",
	sectionBar:     "Bar",
	sectionGeneral: "General",
}

const sectionCount = 3

// settingsMode tracks the current interaction mode.
type settingsMode int

const (
	settingsModeList   settingsMode = iota // browsing within a section
	settingsModeFilter                     // theme filter input active
	settingsModeEdit                       // config text input active
)

// ── Messages ──

type settingsDataMsg struct {
	reqID        int64
	themes       []theme.ThemeInfo
	currentTheme string
	cfg          config.Config
	cfgExists    bool
	err          error
}

func (m settingsDataMsg) TargetTab() dashboard.TabID { return dashboard.TabSettings }

type settingsThemeApplyMsg struct {
	reqID     int64
	themeName string
	err       error
}

func (m settingsThemeApplyMsg) TargetTab() dashboard.TabID { return dashboard.TabSettings }

type settingsConfigSaveMsg struct {
	reqID int64
	err   error
}

func (m settingsConfigSaveMsg) TargetTab() dashboard.TabID { return dashboard.TabSettings }

// SettingsTab implements the Tab interface for merged settings.
type SettingsTab struct {
	resolver *theme.Resolver
	fs       config.FS
	styles   tui.Styles

	// Active section.
	section settingsSection

	// Data.
	reqID int64

	// Theme section.
	themes       []theme.ThemeInfo
	filtered     []theme.ThemeInfo
	currentTheme string
	themeCursor  int
	filter       textinput.Model

	// Bar section.
	barPresets   []bar.Preset
	barCursor    int
	currentBar   string

	// General section.
	cfg         config.Config
	originalCfg config.Config
	cfgExists   bool
	configRows  []configRow
	cfgCursor   int
	editInput   textinput.Model

	// Mode.
	mode settingsMode

	// Viewport.
	width  int
	height int

	// Status flash.
	statusText    string
	statusIsError bool
}

// NewSettingsTab creates a new settings tab.
func NewSettingsTab(resolver *theme.Resolver, fs config.FS, styles tui.Styles) *SettingsTab {
	filterInput := textinput.New()
	filterInput.Placeholder = "filter themes..."
	filterInput.CharLimit = 64

	editInput := textinput.New()
	editInput.CharLimit = 256

	t := &SettingsTab{
		resolver:   resolver,
		fs:         fs,
		styles:     styles,
		barPresets: bar.AllPresets(),
		filter:     filterInput,
		editInput:  editInput,
	}
	t.configRows = t.buildConfigRows()
	return t
}

func (t *SettingsTab) buildConfigRows() []configRow {
	return []configRow{
		{
			label:    "Prefix",
			key:      "prefix",
			kind:     configRowText,
			getValue: func(c config.Config) string { return c.Prefix },
			setValue:  func(c *config.Config, v string) { c.Prefix = v },
		},
		{
			label:    "Sync Target",
			key:      "sync.target",
			kind:     configRowCycle,
			options:  []string{"none", "ghostty", "nvim"},
			getValue: func(c config.Config) string { return c.Sync.Target },
			setValue:  func(c *config.Config, v string) { c.Sync.Target = v },
		},
		{
			label:    "Ghostty Config",
			key:      "sync.ghostty_config",
			kind:     configRowText,
			getValue: func(c config.Config) string { return c.Sync.GhosttyConfig },
			setValue:  func(c *config.Config, v string) { c.Sync.GhosttyConfig = v },
		},
		{
			label:   "Auto Cleanup Tmp",
			key:     "sessions.auto_cleanup_tmp",
			kind:    configRowToggle,
			getValue: func(c config.Config) string {
				if c.Sessions.AutoCleanupTmp {
					return "true"
				}
				return "false"
			},
			setValue: func(c *config.Config, v string) {
				c.Sessions.AutoCleanupTmp = v == "true"
			},
		},
	}
}

func (t *SettingsTab) ID() dashboard.TabID { return dashboard.TabSettings }
func (t *SettingsTab) Title() string        { return "Settings" }
func (t *SettingsTab) Init() tea.Cmd        { return nil }

func (t *SettingsTab) Activate(reason dashboard.ActivateReason) tea.Cmd {
	t.reqID = settingsReqCounter.Add(1)
	return t.fetchData(t.reqID)
}

func (t *SettingsTab) Deactivate() {
	if t.mode == settingsModeFilter {
		t.filter.Blur()
		t.mode = settingsModeList
	}
	if t.mode == settingsModeEdit {
		t.editInput.Blur()
		t.mode = settingsModeList
	}
}

func (t *SettingsTab) Resize(width, height int) {
	t.width = width
	t.height = height
}

// Update processes messages for the settings tab.
func (t *SettingsTab) Update(msg tea.Msg) (dashboard.Tab, tea.Cmd) {
	switch msg := msg.(type) {
	case settingsDataMsg:
		if msg.reqID != t.reqID {
			return t, nil
		}
		if msg.err != nil {
			return t, func() tea.Msg {
				return dashboard.SetStatusIntent{Text: "Failed to load settings", IsError: true}
			}
		}
		t.themes = msg.themes
		t.currentTheme = msg.currentTheme
		t.cfg = msg.cfg
		t.originalCfg = msg.cfg
		t.cfgExists = msg.cfgExists
		t.currentBar = msg.cfg.Bar.Preset
		t.applyFilter()
		// Sync bar cursor with current preset.
		for i, p := range t.barPresets {
			if p.String() == t.currentBar {
				t.barCursor = i
				break
			}
		}
		return t, nil

	case settingsThemeApplyMsg:
		if msg.reqID != t.reqID {
			return t, nil
		}
		if msg.err != nil {
			return t, func() tea.Msg {
				return dashboard.SetStatusIntent{Text: fmt.Sprintf("Failed to apply: %v", msg.err), IsError: true}
			}
		}
		t.currentTheme = msg.themeName
		return t, func() tea.Msg {
			return dashboard.SetStatusIntent{Text: "Applied " + msg.themeName, IsError: false}
		}

	case settingsConfigSaveMsg:
		if msg.reqID != t.reqID {
			return t, nil
		}
		if msg.err != nil {
			t.statusText = fmt.Sprintf("Save failed: %v", msg.err)
			t.statusIsError = true
			return t, func() tea.Msg {
				return dashboard.SetStatusIntent{Text: "Config save failed", IsError: true}
			}
		}
		t.originalCfg = t.cfg
		t.statusText = "Saved"
		t.statusIsError = false
		return t, func() tea.Msg {
			return dashboard.SetStatusIntent{Text: "Config saved", IsError: false}
		}

	case tea.KeyMsg:
		return t.handleKey(msg)
	}

	// Forward to filter input if active.
	if t.mode == settingsModeFilter {
		var cmd tea.Cmd
		t.filter, cmd = t.filter.Update(msg)
		t.applyFilter()
		return t, cmd
	}

	// Forward to edit input if active.
	if t.mode == settingsModeEdit {
		var cmd tea.Cmd
		t.editInput, cmd = t.editInput.Update(msg)
		return t, cmd
	}

	return t, nil
}

func (t *SettingsTab) handleKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	// Section switch with left/right takes priority (in list mode only).
	if t.mode == settingsModeList {
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("left", "h"))):
			if t.section > 0 {
				t.section--
			}
			return t, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("right", "l"))):
			if t.section < sectionCount-1 {
				t.section++
			}
			return t, nil
		}
	}

	switch t.mode {
	case settingsModeFilter:
		return t.handleFilterKey(msg)
	case settingsModeEdit:
		return t.handleEditKey(msg)
	default:
		switch t.section {
		case sectionTheme:
			return t.handleThemeKey(msg)
		case sectionBar:
			return t.handleBarKey(msg)
		case sectionGeneral:
			return t.handleGeneralKey(msg)
		}
	}

	return t, nil
}

// ── Theme section key handling ──

func (t *SettingsTab) handleThemeKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
		if t.themeCursor > 0 {
			t.themeCursor--
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
		if t.themeCursor < len(t.filtered)-1 {
			t.themeCursor++
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		if t.themeCursor < len(t.filtered) {
			return t, t.applyTheme(t.filtered[t.themeCursor].Name)
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("/"))):
		t.mode = settingsModeFilter
		t.filter.Focus()
		return t, textinput.Blink

	case key.Matches(msg, key.NewBinding(key.WithKeys("G"))):
		if len(t.filtered) > 0 {
			t.themeCursor = len(t.filtered) - 1
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("g"))):
		t.themeCursor = 0
		return t, nil
	}

	return t, nil
}

func (t *SettingsTab) handleFilterKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		t.mode = settingsModeList
		t.filter.SetValue("")
		t.filter.Blur()
		t.applyFilter()
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		t.mode = settingsModeList
		t.filter.Blur()
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("up"))):
		if t.themeCursor > 0 {
			t.themeCursor--
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("down"))):
		if t.themeCursor < len(t.filtered)-1 {
			t.themeCursor++
		}
		return t, nil
	}

	var cmd tea.Cmd
	t.filter, cmd = t.filter.Update(msg)
	t.applyFilter()
	return t, cmd
}

// ── Bar section key handling ──

func (t *SettingsTab) handleBarKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
		if t.barCursor > 0 {
			t.barCursor--
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
		if t.barCursor < len(t.barPresets)-1 {
			t.barCursor++
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		if t.barCursor < len(t.barPresets) {
			preset := t.barPresets[t.barCursor]
			t.currentBar = preset.String()
			t.cfg.Bar.Preset = preset.String()
			return t, t.saveConfig()
		}
		return t, nil
	}

	return t, nil
}

// ── General section key handling ──

func (t *SettingsTab) handleGeneralKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
		if t.cfgCursor > 0 {
			t.cfgCursor--
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
		if t.cfgCursor < len(t.configRows)-1 {
			t.cfgCursor++
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("enter", " "))):
		return t.activateConfigRow()

	case key.Matches(msg, key.NewBinding(key.WithKeys("s"))):
		return t, t.saveConfig()

	case key.Matches(msg, key.NewBinding(key.WithKeys("G"))):
		t.cfgCursor = len(t.configRows) - 1
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("g"))):
		t.cfgCursor = 0
		return t, nil
	}

	return t, nil
}

func (t *SettingsTab) handleEditKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		row := t.configRows[t.cfgCursor]
		newVal := strings.TrimSpace(t.editInput.Value())
		if newVal != "" {
			row.setValue(&t.cfg, newVal)
		}
		t.mode = settingsModeList
		t.editInput.Blur()
		return t, nil

	case tea.KeyEscape:
		t.mode = settingsModeList
		t.editInput.Blur()
		return t, nil
	}

	var cmd tea.Cmd
	t.editInput, cmd = t.editInput.Update(msg)
	return t, cmd
}

func (t *SettingsTab) activateConfigRow() (dashboard.Tab, tea.Cmd) {
	if t.cfgCursor >= len(t.configRows) {
		return t, nil
	}

	row := t.configRows[t.cfgCursor]

	switch row.kind {
	case configRowText:
		t.mode = settingsModeEdit
		t.editInput.Placeholder = row.key
		t.editInput.SetValue(row.getValue(t.cfg))
		t.editInput.Focus()
		t.editInput.CursorEnd()
		return t, textinput.Blink

	case configRowCycle:
		current := row.getValue(t.cfg)
		next := cycleOption(row.options, current)
		row.setValue(&t.cfg, next)
		return t, nil

	case configRowToggle:
		current := row.getValue(t.cfg)
		if current == "true" {
			row.setValue(&t.cfg, "false")
		} else {
			row.setValue(&t.cfg, "true")
		}
		return t, nil
	}

	return t, nil
}

// ── View rendering ──

func (t *SettingsTab) View() string {
	var b strings.Builder

	b.WriteString("\n")

	// Section tabs.
	for i := 0; i < sectionCount; i++ {
		label := sectionNames[i]
		if settingsSection(i) == t.section {
			b.WriteString(t.styles.Accent.Bold(true).Underline(true).Render(label))
		} else {
			b.WriteString(t.styles.Dim.Render(label))
		}
		if i < sectionCount-1 {
			b.WriteString("    ")
		}
	}
	b.WriteString("\n")

	lineWidth := t.width - 4
	if lineWidth < 20 {
		lineWidth = 20
	}
	if lineWidth > 60 {
		lineWidth = 60
	}
	b.WriteString(t.styles.Dim.Render(strings.Repeat("─", lineWidth)) + "\n")

	switch t.section {
	case sectionTheme:
		b.WriteString(t.viewTheme())
	case sectionBar:
		b.WriteString(t.viewBar())
	case sectionGeneral:
		b.WriteString(t.viewGeneral())
	}

	return b.String()
}

func (t *SettingsTab) viewTheme() string {
	var b strings.Builder

	// Header.
	b.WriteString("\n")
	currentLabel := "none"
	if t.currentTheme != "" {
		currentLabel = t.currentTheme
	}
	b.WriteString(t.styles.Dim.Render("Current: ") + t.styles.Success.Render(currentLabel))
	b.WriteString("  " + t.styles.Dim.Render(fmt.Sprintf("%d themes", len(t.themes))))
	b.WriteString("\n\n")

	// Filter bar.
	if t.mode == settingsModeFilter {
		prompt := t.styles.Accent.Render("  / ")
		b.WriteString(prompt + t.filter.View() + "\n\n")
	} else if t.filter.Value() != "" {
		b.WriteString(t.styles.Dim.Render("  filter: "+t.filter.Value()) + "\n\n")
	}

	// Theme list.
	if len(t.filtered) == 0 {
		if t.filter.Value() != "" {
			b.WriteString(views.RenderEmptyState(
				"No themes match your filter.",
				"Press / to search or esc to clear.",
				t.styles.Dim,
			))
		} else {
			b.WriteString(views.RenderEmptyState(
				"No themes available.",
				"",
				t.styles.Dim,
			))
		}
	} else {
		availableHeight := t.height - 12
		if availableHeight < 5 {
			availableHeight = 10
		}

		start := 0
		if t.themeCursor >= availableHeight {
			start = t.themeCursor - availableHeight + 1
		}
		end := start + availableHeight
		if end > len(t.filtered) {
			end = len(t.filtered)
		}

		if start > 0 {
			b.WriteString(t.styles.Dim.Render("  ↑ " + fmt.Sprintf("%d more", start)) + "\n")
		}

		for i := start; i < end; i++ {
			b.WriteString(t.renderThemeEntry(i, t.filtered[i]))
		}

		if end < len(t.filtered) {
			b.WriteString(t.styles.Dim.Render("  ↓ " + fmt.Sprintf("%d more", len(t.filtered)-end)) + "\n")
		}

		// Color swatch.
		if t.themeCursor < len(t.filtered) && t.resolver != nil {
			b.WriteString("\n")
			swatch := t.renderSwatch(t.filtered[t.themeCursor])
			if swatch != "" {
				b.WriteString("  " + swatch + "\n")
			}
		}
	}

	return b.String()
}

func (t *SettingsTab) renderThemeEntry(idx int, ti theme.ThemeInfo) string {
	selected := idx == t.themeCursor
	isCurrent := ti.Name == t.currentTheme

	cursor := "  "
	if selected {
		cursor = t.styles.Accent.Render("▸ ")
	}

	nameStyle := t.styles.Normal
	if selected {
		nameStyle = t.styles.Accent.Bold(true)
	}
	name := nameStyle.Render(ti.Name)

	currentMark := ""
	if isCurrent {
		currentMark = t.styles.Success.Render(" ●")
	}

	var sourceTag string
	switch ti.Source {
	case theme.SourceBundled:
		sourceTag = t.styles.Dim.Render(" bundled")
	case theme.SourceUser:
		sourceTag = t.styles.Special.Render(" user")
	case theme.SourceIterm2:
		sourceTag = t.styles.Info.Render(" iterm2")
	}

	var modeTag string
	if ti.IsDark {
		modeTag = t.styles.Dim.Render(" dark")
	} else {
		modeTag = t.styles.Accent.Render(" light")
	}

	return "  " + cursor + name + currentMark + sourceTag + modeTag + "\n"
}

func (t *SettingsTab) renderSwatch(ti theme.ThemeInfo) string {
	if t.resolver == nil {
		return ""
	}
	resolved, err := t.resolver.Resolve(ti.Name)
	if err != nil {
		return ""
	}
	palette := resolved.SemanticPalette()

	width := t.width
	if width <= 0 {
		width = 80
	}
	return views.RenderSwatch(&palette, width)
}

func (t *SettingsTab) viewBar() string {
	var b strings.Builder

	b.WriteString("\n")
	currentLabel := "default"
	if t.currentBar != "" {
		currentLabel = t.currentBar
	}
	b.WriteString(t.styles.Dim.Render("Current: ") + t.styles.Success.Render(currentLabel))
	b.WriteString("\n\n")

	// Resolve palette for previews.
	var palette *theme.Palette
	if t.resolver != nil && t.currentTheme != "" {
		resolved, err := t.resolver.Resolve(t.currentTheme)
		if err == nil {
			p := resolved.SemanticPalette()
			palette = &p
		}
	}

	for i, preset := range t.barPresets {
		selected := i == t.barCursor
		isCurrent := preset.String() == t.currentBar

		cursor := "  "
		if selected {
			cursor = t.styles.Accent.Render("▸ ")
		}

		nameStyle := t.styles.Normal
		if selected {
			nameStyle = t.styles.Accent.Bold(true)
		}

		currentMark := ""
		if isCurrent {
			currentMark = t.styles.Success.Render(" ●")
		}

		b.WriteString("  " + cursor + nameStyle.Render(preset.String()) + currentMark + "\n")

		// ANSI preview.
		if palette != nil {
			preview := bar.RenderPreview(preset, palette)
			b.WriteString("    " + preview + "\n")
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (t *SettingsTab) viewGeneral() string {
	var b strings.Builder

	b.WriteString("\n")
	if t.cfgExists {
		b.WriteString(t.styles.Dim.Render("~/.zmux.toml"))
	} else {
		b.WriteString(t.styles.Dim.Render("~/.zmux.toml") + "  " + t.styles.Info.Render("(using defaults)"))
	}

	if t.statusText != "" {
		style := t.styles.Success
		if t.statusIsError {
			style = t.styles.Error
		}
		b.WriteString("  " + style.Render(t.statusText))
	}
	b.WriteString("\n\n")

	// Edit overlay.
	if t.mode == settingsModeEdit && t.cfgCursor < len(t.configRows) {
		row := t.configRows[t.cfgCursor]
		prompt := t.styles.Accent.Render("  " + row.label + " ▸ ")
		b.WriteString(prompt + t.editInput.View() + "\n\n")
	}

	for i, row := range t.configRows {
		b.WriteString(t.renderConfigRow(i, row))
	}

	b.WriteString("\n")
	if t.hasConfigChanges() {
		b.WriteString("  " + t.styles.Info.Render("Unsaved changes") + "  " +
			t.styles.Dim.Render("press s to save") + "\n")
	}

	return b.String()
}

func (t *SettingsTab) renderConfigRow(idx int, row configRow) string {
	selected := idx == t.cfgCursor
	modified := row.getValue(t.cfg) != row.getValue(t.originalCfg)

	cursor := "  "
	if selected {
		cursor = t.styles.Accent.Render("▸ ")
	}

	labelStyle := t.styles.Normal
	if selected {
		labelStyle = t.styles.Accent.Bold(true)
	}

	label := fmt.Sprintf("%-22s", row.label)
	labelStr := labelStyle.Render(label)

	value := row.getValue(t.cfg)
	valueStyle := t.styles.Normal
	switch row.kind {
	case configRowCycle:
		valueStyle = t.styles.Info
	case configRowToggle:
		if value == "true" {
			valueStyle = t.styles.Success
		} else {
			valueStyle = t.styles.Dim
		}
	}
	valueStr := valueStyle.Render(value)

	hint := ""
	if selected {
		switch row.kind {
		case configRowCycle:
			hint = t.styles.Dim.Render("  [enter to cycle]")
		case configRowToggle:
			hint = t.styles.Dim.Render("  [enter to toggle]")
		case configRowText:
			hint = t.styles.Dim.Render("  [enter to edit]")
		}
	}

	modStr := ""
	if modified {
		modStr = t.styles.Info.Render("  (modified)")
	}

	return "  " + cursor + labelStr + "  " + valueStr + modStr + hint + "\n"
}

func (t *SettingsTab) hasConfigChanges() bool {
	for _, row := range t.configRows {
		if row.getValue(t.cfg) != row.getValue(t.originalCfg) {
			return true
		}
	}
	return false
}

func (t *SettingsTab) ShortHelp() string {
	switch t.mode {
	case settingsModeFilter:
		return "enter:confirm  esc:clear"
	case settingsModeEdit:
		return "enter:confirm  esc:cancel"
	default:
		switch t.section {
		case sectionTheme:
			return "enter:apply  /:filter  h/l:section  j/k:navigate"
		case sectionBar:
			return "enter:apply  h/l:section  j/k:navigate"
		case sectionGeneral:
			return "enter:edit  s:save  h/l:section  j/k:navigate"
		default:
			return "h/l:section  j/k:navigate"
		}
	}
}

// ── Data fetching ──

func (t *SettingsTab) fetchData(reqID int64) tea.Cmd {
	resolver := t.resolver
	fs := t.fs
	return func() tea.Msg {
		var themes []theme.ThemeInfo
		if resolver != nil {
			themes = resolver.List()
		}

		currentTheme := ""
		cfgPath, err := config.ConfigPath(fs)
		if err != nil {
			return settingsDataMsg{reqID: reqID, err: err}
		}

		exists := config.ConfigExists(fs)
		cfg, err := config.Load(fs, cfgPath)
		if err != nil {
			cfg = config.DefaultConfig()
		} else {
			currentTheme = cfg.Theme
		}

		return settingsDataMsg{
			reqID:        reqID,
			themes:       themes,
			currentTheme: currentTheme,
			cfg:          cfg,
			cfgExists:    exists,
		}
	}
}

func (t *SettingsTab) applyTheme(name string) tea.Cmd {
	fs := t.fs
	reqID := t.reqID
	return func() tea.Msg {
		cfgPath, err := config.ConfigPath(fs)
		if err != nil {
			return settingsThemeApplyMsg{reqID: reqID, err: err}
		}

		cfg, err := config.Load(fs, cfgPath)
		if err != nil {
			cfg = config.DefaultConfig()
		}

		cfg.Theme = name
		if err := config.Save(fs, cfgPath, cfg); err != nil {
			return settingsThemeApplyMsg{reqID: reqID, err: err}
		}

		return settingsThemeApplyMsg{
			reqID:     reqID,
			themeName: name,
		}
	}
}

func (t *SettingsTab) saveConfig() tea.Cmd {
	fs := t.fs
	cfg := t.cfg
	reqID := t.reqID
	return func() tea.Msg {
		cfgPath, err := config.ConfigPath(fs)
		if err != nil {
			return settingsConfigSaveMsg{reqID: reqID, err: err}
		}

		if err := config.Save(fs, cfgPath, cfg); err != nil {
			return settingsConfigSaveMsg{reqID: reqID, err: err}
		}

		return settingsConfigSaveMsg{reqID: reqID}
	}
}

func (t *SettingsTab) applyFilter() {
	query := t.filter.Value()
	if query == "" {
		t.filtered = t.themes
	} else {
		names := make([]string, len(t.themes))
		for i, ti := range t.themes {
			names[i] = ti.Name
		}
		matches := fuzzy.Find(query, names)
		t.filtered = make([]theme.ThemeInfo, len(matches))
		for i, match := range matches {
			t.filtered[i] = t.themes[match.Index]
		}
	}

	if t.themeCursor >= len(t.filtered) {
		t.themeCursor = max(0, len(t.filtered)-1)
	}
}
