package tabs

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/theme"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui/dashboard"
	"github.com/donjor/zmux/internal/tui/styles"
	"github.com/donjor/zmux/internal/tui/views"
)

// ============================================================================
// Messages
// ============================================================================

type themesDataMsg struct {
	reqID        int64
	themes       []theme.ThemeInfo
	currentTheme string
	err          error
}

func (m themesDataMsg) TargetTab() dashboard.TabID { return dashboard.TabThemes }

type themesApplyMsg struct {
	reqID     int64
	themeName string
	palette   *theme.Palette
	styles    *styles.Styles
	err       error
}

func (m themesApplyMsg) TargetTab() dashboard.TabID { return dashboard.TabThemes }

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
	selfBin  string // binary embedded in #(<bin> bar-render); config.SelfBin(profile)
	styles   styles.Styles

	mode themesMode

	// Theme list state.
	themes       []theme.ThemeInfo
	filtered     []theme.ThemeInfo
	currentTheme string
	themeCursor  int
	filter       textinput.Model

	// Inline editing state.
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
	savedStyles  *styles.Styles
	previewing   bool

	// Request tracking.
	reqID int64

	// Layout + viewport.
	vp            viewport.Model
	width, height int
}

// NewThemesTab creates a new themes tab. selfBin is the binary embedded in the
// generated bar's #(<bin> bar-render) content — pass config.SelfBin(profile).
func NewThemesTab(resolver *theme.Resolver, fs config.FS, runner tmux.Runner, selfBin string, styles styles.Styles) *ThemesTab {
	filterInput := textinput.New()
	filterInput.Placeholder = "filter themes..."
	filterInput.CharLimit = 64

	nameInput := textinput.New()
	nameInput.Placeholder = "theme name"
	nameInput.CharLimit = 64

	return &ThemesTab{
		resolver:  resolver,
		fs:        fs,
		runner:    runner,
		selfBin:   selfBin,
		styles:    styles,
		filter:    filterInput,
		editSlots: buildEditorSlots(),
		nameInput: nameInput,
	}
}

func (t *ThemesTab) ID() dashboard.TabID { return dashboard.TabThemes }
func (t *ThemesTab) Title() string       { return "Themes" }
func (t *ThemesTab) Init() tea.Cmd       { return nil }

// CapturesEscape reports that Esc should exit the filter input, the color
// editor, or clear a committed filter rather than close the dashboard. The
// committed-filter case matches the "esc to clear" hint the list view shows.
func (t *ThemesTab) CapturesEscape() bool {
	return t.mode != themesModeList || t.editing || t.filter.Value() != ""
}

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
	t.vp.SetWidth(w)
	t.vp.SetHeight(h)
}

func (t *ThemesTab) ShortHelp() string {
	switch t.mode {
	case themesModeFilter:
		return "enter:confirm  esc:clear"
	default:
		if t.editing {
			if t.namingActive {
				return "enter:save  esc:cancel"
			}
			if t.pickerActive {
				return "arrows:adjust  #:hex input  enter:confirm  esc:cancel"
			}
			return "j/k:navigate slots  enter:edit color  s:save  esc:back to list"
		}
		help := "j/k:navigate  enter:apply  /:filter  e:edit  c:clone"
		if t.previewing {
			help += "  q:revert"
		}
		return help
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
// Key handling
// ============================================================================

func (t *ThemesTab) handleKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	// Revert preview.
	if t.mode == themesModeList && !t.editing {
		if key.Matches(msg, key.NewBinding(key.WithKeys("q"))) && t.previewing {
			t.revertPreview()
			return t, t.emitRevert()
		}
	}

	switch t.mode {
	case themesModeFilter:
		return t.handleFilterKey(msg)
	default:
		if t.editing {
			return t.handleEditorKey(msg)
		}
		return t.handleColorsKey(msg)
	}
}

// ============================================================================
// View
// ============================================================================

func (t *ThemesTab) View() string {
	content, cursorLine := t.renderColorsContent()
	t.vp.SetContent(content)
	ensureCursorVisible(&t.vp, cursorLine)
	return renderScrollable(t.vp, t.styles)
}

// Preview helpers (revertPreview + emitRevert) live in themes_data.go
// alongside the other mutation commands.
