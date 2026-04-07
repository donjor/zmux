package tabs

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/donjor/zmux/internal/bar"
	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/theme"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui"
	"github.com/donjor/zmux/internal/tui/dashboard"
	"github.com/donjor/zmux/internal/tui/views"
)

// ============================================================================
// Sections — Colors + Bar (two sections total)
// ============================================================================

type themesSection int

const (
	themesSectionColors themesSection = iota
	themesSectionBar
	themesSectionCount // sentinel
)

var themesSectionNames = [...]string{
	themesSectionColors: "Colors",
	themesSectionBar:    "Bar",
}

// ============================================================================
// Messages
// ============================================================================

type themesDataMsg struct {
	reqID        int64
	themes       []theme.ThemeInfo
	currentTheme string
	cfg          config.Config
	cfgExists    bool
	err          error
}

func (m themesDataMsg) TargetTab() dashboard.TabID { return dashboard.TabThemes }

type themesApplyMsg struct {
	reqID     int64
	themeName string
	palette   *theme.Palette
	styles    *tui.Styles
	err       error
}

func (m themesApplyMsg) TargetTab() dashboard.TabID { return dashboard.TabThemes }

type themesConfigSaveMsg struct {
	reqID int64
	err   error
}

func (m themesConfigSaveMsg) TargetTab() dashboard.TabID { return dashboard.TabThemes }

type themesSaveThemeMsg struct {
	reqID     int64
	themeName string
	err       error
}

func (m themesSaveThemeMsg) TargetTab() dashboard.TabID { return dashboard.TabThemes }

// ============================================================================
// Modes
// ============================================================================

type themesMode int

const (
	themesModeList   themesMode = iota
	themesModeFilter            // filter input active
)

// editorSlot + buildEditorSlots live in themes_editor.go.

// ============================================================================
// ThemesTab
// ============================================================================

type ThemesTab struct {
	// Dependencies.
	resolver *theme.Resolver
	fs       config.FS
	runner   tmux.Runner
	styles   tui.Styles

	// Section navigation.
	section themesSection
	mode    themesMode

	// Colors section — theme list state.
	themes       []theme.ThemeInfo
	filtered     []theme.ThemeInfo
	currentTheme string
	themeCursor  int
	filter       textinput.Model

	// Colors section — inline editing state.
	editing         bool
	editTheme       theme.Theme
	editName        string
	editSlots       []editorSlot
	editCursor      int
	picker          views.ColorPicker
	pickerActive    bool
	pickerOrigColor theme.Color // saved on open, restored on Esc
	nameInput       textinput.Model
	namingActive    bool // clone prompt active

	// Preview state (ephemeral).
	savedTheme   string
	savedPalette *theme.Palette
	savedStyles  *tui.Styles
	previewing   bool

	// Bar state.
	barPresets    []bar.Preset
	barCursor     int
	currentBar    string
	barSegments   config.BarSegments
	barInSegments bool

	// Config (for saving bar changes).
	cfg       config.Config
	cfgExists bool

	// Request tracking.
	reqID int64

	// Layout.
	width, height int
}

// NewThemesTab creates a new themes tab.
func NewThemesTab(resolver *theme.Resolver, fs config.FS, runner tmux.Runner, styles tui.Styles) *ThemesTab {
	filterInput := textinput.New()
	filterInput.Placeholder = "filter themes..."
	filterInput.CharLimit = 64

	nameInput := textinput.New()
	nameInput.Placeholder = "theme name"
	nameInput.CharLimit = 64

	return &ThemesTab{
		resolver:   resolver,
		fs:         fs,
		runner:     runner,
		styles:     styles,
		barPresets: bar.AllPresets(),
		filter:     filterInput,
		editSlots:  buildEditorSlots(),
		nameInput:  nameInput,
	}
}

func (t *ThemesTab) ID() dashboard.TabID { return dashboard.TabThemes }
func (t *ThemesTab) Title() string       { return "Themes" }
func (t *ThemesTab) Init() tea.Cmd       { return nil }

func (t *ThemesTab) Activate(reason dashboard.ActivateReason) tea.Cmd {
	t.reqID = dashboard.NextReqID()
	// Save current theme for preview revert.
	t.savedTheme = t.currentTheme
	t.savedPalette = nil
	t.savedStyles = nil
	t.previewing = false
	return t.fetchData(t.reqID)
}

func (t *ThemesTab) Deactivate() {
	// Revert preview on tab switch.
	if t.previewing {
		t.revertPreview()
	}
	t.filter.Blur()
	t.mode = themesModeList
	t.editing = false
	t.pickerActive = false
	t.namingActive = false
}

func (t *ThemesTab) Resize(w, h int) {
	t.width = w
	t.height = h
}

func (t *ThemesTab) ShortHelp() string {
	switch t.mode {
	case themesModeFilter:
		return "enter:confirm  esc:clear"
	default:
		switch t.section {
		case themesSectionColors:
			if t.editing {
				if t.namingActive {
					return "enter:save  esc:cancel"
				}
				if t.pickerActive {
					return "arrows:adjust  #:hex input  enter:confirm  esc:cancel"
				}
				return "j/k:navigate slots  enter:edit color  s:save  esc:back to list"
			}
			help := "j/k:navigate  enter:apply  /:filter  e:edit  c:clone  h/l:section"
			if t.previewing {
				help += "  q:revert"
			}
			return help
		case themesSectionBar:
			if t.barInSegments {
				return "enter/space:toggle  h/l:section  j/k:navigate"
			}
			return "enter:apply  h/l:section  j/k:navigate"
		default:
			return "h/l:section  j/k:navigate"
		}
	}
}

// ============================================================================
// Update
// ============================================================================

func (t *ThemesTab) Update(msg tea.Msg) (dashboard.Tab, tea.Cmd) {
	switch msg := msg.(type) {
	case dashboard.ThemeChangedMsg:
		t.styles = msg.Styles
		return t, nil

	case themesDataMsg:
		if cmd, ok := themesGuard(t.reqID, msg.reqID, msg.err, "Failed to load themes: %v"); !ok {
			return t, cmd
		}
		t.themes = msg.themes
		t.currentTheme = msg.currentTheme
		t.savedTheme = msg.currentTheme
		t.cfg = msg.cfg
		t.cfgExists = msg.cfgExists
		t.currentBar = msg.cfg.Bar.Preset
		t.barSegments = msg.cfg.Bar.Segments
		t.applyFilter()
		return t, nil

	case themesApplyMsg:
		if cmd, ok := themesGuard(t.reqID, msg.reqID, msg.err, "Failed to apply: %v"); !ok {
			return t, cmd
		}
		t.currentTheme = msg.themeName
		t.savedTheme = msg.themeName
		t.previewing = false
		var cmds []tea.Cmd
		cmds = append(cmds, func() tea.Msg {
			return dashboard.SetStatusIntent{Text: "Applied " + msg.themeName, IsError: false}
		})
		if msg.palette != nil && msg.styles != nil {
			pal := *msg.palette
			sty := *msg.styles
			t.savedPalette = msg.palette
			t.savedStyles = msg.styles
			cmds = append(cmds, func() tea.Msg {
				return dashboard.ThemeChangeIntent{Palette: pal, Styles: sty}
			})
		}
		return t, tea.Batch(cmds...)

	case themesConfigSaveMsg:
		if cmd, ok := themesGuard(t.reqID, msg.reqID, msg.err, "Config save failed: %v"); !ok {
			return t, cmd
		}
		return t, func() tea.Msg {
			return dashboard.SetStatusIntent{Text: "Config saved", IsError: false}
		}

	case themesSaveThemeMsg:
		if cmd, ok := themesGuard(t.reqID, msg.reqID, msg.err, "Save failed: %v"); !ok {
			return t, cmd
		}
		// Exit editing, refresh theme list, and apply the saved theme.
		t.editing = false
		t.pickerActive = false
		var cmds []tea.Cmd
		cmds = append(cmds, func() tea.Msg {
			return dashboard.SetStatusIntent{Text: "Saved theme: " + msg.themeName, IsError: false}
		})
		// Refresh the theme list to include the new custom theme.
		t.reqID = dashboard.NextReqID()
		cmds = append(cmds, t.fetchData(t.reqID))
		// Apply the new theme.
		if msg.themeName != "" {
			cmds = append(cmds, t.applyTheme(msg.themeName))
		}
		return t, tea.Batch(cmds...)

	case tea.KeyMsg:
		return t.handleKey(msg)
	}

	// Forward to filter input if active.
	if t.mode == themesModeFilter {
		var cmd tea.Cmd
		t.filter, cmd = t.filter.Update(msg)
		t.applyFilter()
		return t, cmd
	}

	return t, nil
}

// ============================================================================
// Key handling — top-level dispatch
// ============================================================================

func (t *ThemesTab) handleKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	// Section switch with left/right in list mode (not editing, not filtering).
	if t.mode == themesModeList && !t.editing {
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("left", "h"))):
			t.section = (t.section - 1 + themesSectionCount) % themesSectionCount
			return t, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("right", "l"))):
			t.section = (t.section + 1) % themesSectionCount
			return t, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("q"))):
			if t.previewing {
				t.revertPreview()
				return t, t.emitRevert()
			}
			return t, nil
		}
	}

	switch t.mode {
	case themesModeFilter:
		return t.handleFilterKey(msg)
	default:
		switch t.section {
		case themesSectionColors:
			if t.editing {
				return t.handleEditorKey(msg)
			}
			return t.handleColorsKey(msg)
		case themesSectionBar:
			return t.handleBarKey(msg)
		}
	}

	return t, nil
}

// ============================================================================
// View — top-level
// ============================================================================

func (t *ThemesTab) View() string {
	var b strings.Builder

	// Section header tabs: Colors | Bar
	for i := themesSection(0); i < themesSectionCount; i++ {
		label := themesSectionNames[i]
		if i == t.section {
			b.WriteString(t.styles.Accent.Bold(true).Underline(true).Render(label))
		} else {
			b.WriteString(t.styles.Dim.Render(label))
		}
		if i < themesSectionCount-1 {
			b.WriteString(t.styles.Dim.Render("  |  "))
		}
	}
	b.WriteString("\n")

	switch t.section {
	case themesSectionColors:
		b.WriteString(t.viewColors())
	case themesSectionBar:
		b.WriteString(t.viewBar())
	}

	return b.String()
}

// Preview helpers (revertPreview + emitRevert) live in themes_data.go
// alongside the other mutation commands.
