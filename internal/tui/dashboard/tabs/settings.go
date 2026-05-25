package tabs

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/theme"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui/dashboard"
	"github.com/donjor/zmux/internal/tui/styles"
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
	key      string // dotted key for display, e.g. "sync.target"
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

// settingsMode tracks the current interaction mode.
type settingsMode int

const (
	settingsModeList settingsMode = iota // browsing config rows
	settingsModeEdit                     // config text input active
)

// ── Messages ──

type settingsDataMsg struct {
	reqID     int64
	cfg       config.Config
	cfgExists bool
	err       error
}

func (m settingsDataMsg) TargetTab() dashboard.TabID { return dashboard.TabSettings }

type settingsConfigSaveMsg struct {
	reqID int64
	err   error
}

func (m settingsConfigSaveMsg) TargetTab() dashboard.TabID { return dashboard.TabSettings }

// SettingsTab implements the Tab interface for general config editing.
type SettingsTab struct {
	fs     config.FS
	runner tmux.Runner
	styles styles.Styles

	// Data.
	reqID int64

	// General config.
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
// The resolver parameter is accepted for backward compatibility but unused.
func NewSettingsTab(resolver *theme.Resolver, fs config.FS, runner tmux.Runner, styles styles.Styles) *SettingsTab {
	editInput := textinput.New()
	editInput.CharLimit = 256

	t := &SettingsTab{
		fs:        fs,
		runner:    runner,
		styles:    styles,
		editInput: editInput,
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
			setValue: func(c *config.Config, v string) { c.Prefix = v },
		},
		{
			label:    "Sync Target",
			key:      "sync.target",
			kind:     configRowCycle,
			options:  []string{"none", "ghostty", "nvim"},
			getValue: func(c config.Config) string { return c.Sync.Target },
			setValue: func(c *config.Config, v string) { c.Sync.Target = v },
		},
		{
			label:    "Ghostty Config",
			key:      "sync.ghostty_config",
			kind:     configRowText,
			getValue: func(c config.Config) string { return c.Sync.GhosttyConfig },
			setValue: func(c *config.Config, v string) { c.Sync.GhosttyConfig = v },
		},
		{
			label: "Auto Cleanup Tmp",
			key:   "sessions.auto_cleanup_tmp",
			kind:  configRowToggle,
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
func (t *SettingsTab) Title() string       { return "Settings" }
func (t *SettingsTab) Init() tea.Cmd       { return nil }

func (t *SettingsTab) Activate(reason dashboard.ActivateReason) tea.Cmd {
	t.reqID = dashboard.NextReqID()
	return t.fetchData(t.reqID)
}

func (t *SettingsTab) Deactivate() {
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
	case dashboard.ThemeChangedMsg:
		t.styles = msg.Styles
		return t, nil

	case settingsDataMsg:
		if msg.reqID != t.reqID {
			return t, nil
		}
		if msg.err != nil {
			return t, func() tea.Msg {
				return dashboard.SetStatusIntent{Text: "Failed to load settings", IsError: true}
			}
		}
		t.cfg = msg.cfg
		t.originalCfg = msg.cfg
		t.cfgExists = msg.cfgExists
		return t, nil

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

	// Forward to edit input if active.
	if t.mode == settingsModeEdit {
		var cmd tea.Cmd
		t.editInput, cmd = t.editInput.Update(msg)
		return t, cmd
	}

	return t, nil
}

func (t *SettingsTab) handleKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	switch t.mode {
	case settingsModeEdit:
		return t.handleEditKey(msg)
	default:
		return t.handleGeneralKey(msg)
	}
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
	switch msg.String() {
	case "enter":
		row := t.configRows[t.cfgCursor]
		newVal := strings.TrimSpace(t.editInput.Value())
		if newVal != "" {
			row.setValue(&t.cfg, newVal)
		}
		t.mode = settingsModeList
		t.editInput.Blur()
		return t, nil

	case "esc":
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
	b.WriteString(t.viewGeneral())
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
	case settingsModeEdit:
		return "enter:confirm  esc:cancel"
	default:
		return "enter:edit  s:save  j/k:navigate"
	}
}

// ── Data fetching ──

func (t *SettingsTab) fetchData(reqID int64) tea.Cmd {
	fs := t.fs
	return func() tea.Msg {
		cfgPath, err := config.ConfigPath(fs)
		if err != nil {
			return settingsDataMsg{reqID: reqID, err: err}
		}

		exists := config.ConfigExists(fs)
		cfg, err := config.Load(fs, cfgPath)
		if err != nil {
			cfg = config.DefaultConfig()
		}

		return settingsDataMsg{
			reqID:     reqID,
			cfg:       cfg,
			cfgExists: exists,
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
