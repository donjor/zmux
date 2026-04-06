package tabs

import (
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"

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

var themesReqCounter atomic.Int64

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

// ============================================================================
// Bar segment labels
// ============================================================================

var themesBarSegmentLabels = []struct {
	Label string
	Field string
}{
	{"Git branch", "git"},
	{"Workspace", "workspace"},
	{"Clock", "clock"},
	{"Language", "lang"},
	{"Directory", "directory"},
	{"Process", "process"},
	{"Group indicator", "group"},
}

// ============================================================================
// Editor slot labels — UPPERCASE, clean names
// ============================================================================

type editorSlot struct {
	Label string
	Get   func(theme.Theme) theme.Color
	Set   func(*theme.Theme, theme.Color)
}

func buildEditorSlots() []editorSlot {
	slots := []editorSlot{
		{"BACKGROUND", func(t theme.Theme) theme.Color { return t.Background }, func(t *theme.Theme, c theme.Color) { t.Background = c }},
		{"FOREGROUND", func(t theme.Theme) theme.Color { return t.Foreground }, func(t *theme.Theme, c theme.Color) { t.Foreground = c }},
		{"CURSOR", func(t theme.Theme) theme.Color { return t.Cursor }, func(t *theme.Theme, c theme.Color) { t.Cursor = c }},
		{"SELECTION", func(t theme.Theme) theme.Color { return t.Selection }, func(t *theme.Theme, c theme.Color) { t.Selection = c }},
	}

	// ANSI 0-15 with clean UPPERCASE labels.
	paletteLabels := [16]string{
		"SURFACE", "ERROR", "SUCCESS", "ACCENT",
		"INFO", "SPECIAL", "META", "MUTED",
		"DIM", "BRIGHT RED", "BRIGHT GREEN", "BRIGHT YELLOW",
		"BRIGHT BLUE", "BRIGHT MAGENTA", "BRIGHT CYAN", "BRIGHT WHITE",
	}

	for i := 0; i < 16; i++ {
		idx := i
		slots = append(slots, editorSlot{
			Label: paletteLabels[idx],
			Get:   func(t theme.Theme) theme.Color { return t.Palette[idx] },
			Set:   func(t *theme.Theme, c theme.Color) { t.Palette[idx] = c },
		})
	}
	return slots
}

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
	editing      bool
	editTheme    theme.Theme
	editName     string
	editSlots    []editorSlot
	editCursor   int
	picker          views.ColorPicker
	pickerActive    bool
	pickerOrigColor theme.Color // saved on open, restored on Esc
	nameInput    textinput.Model
	namingActive bool // clone prompt active

	// Preview state (ephemeral).
	savedTheme   string
	savedPalette *theme.Palette
	savedStyles  *tui.Styles
	previewing   bool
	previewReqID int64

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
func (t *ThemesTab) Title() string        { return "Themes" }
func (t *ThemesTab) Init() tea.Cmd        { return nil }

func (t *ThemesTab) Activate(reason dashboard.ActivateReason) tea.Cmd {
	t.reqID = themesReqCounter.Add(1)
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
		if msg.reqID != t.reqID {
			return t, nil
		}
		if msg.err != nil {
			return t, func() tea.Msg {
				return dashboard.SetStatusIntent{Text: "Failed to load themes", IsError: true}
			}
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
		if msg.reqID != t.reqID {
			return t, nil
		}
		if msg.err != nil {
			return t, func() tea.Msg {
				return dashboard.SetStatusIntent{Text: fmt.Sprintf("Failed to apply: %v", msg.err), IsError: true}
			}
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
		if msg.reqID != t.reqID {
			return t, nil
		}
		if msg.err != nil {
			return t, func() tea.Msg {
				return dashboard.SetStatusIntent{Text: "Config save failed", IsError: true}
			}
		}
		return t, func() tea.Msg {
			return dashboard.SetStatusIntent{Text: "Config saved", IsError: false}
		}

	case themesSaveThemeMsg:
		if msg.reqID != t.reqID {
			return t, nil
		}
		if msg.err != nil {
			return t, func() tea.Msg {
				return dashboard.SetStatusIntent{Text: fmt.Sprintf("Save failed: %v", msg.err), IsError: true}
			}
		}
		// Exit editing, refresh theme list, and apply the saved theme.
		t.editing = false
		t.pickerActive = false
		var cmds []tea.Cmd
		cmds = append(cmds, func() tea.Msg {
			return dashboard.SetStatusIntent{Text: "Saved theme: " + msg.themeName, IsError: false}
		})
		// Refresh the theme list to include the new custom theme.
		t.reqID = themesReqCounter.Add(1)
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
// Key handling — Colors section (theme list)
// ============================================================================

func (t *ThemesTab) handleColorsKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
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
		// Apply highlighted theme (save config + hot reload).
		if t.themeCursor < len(t.filtered) {
			return t, t.applyTheme(t.filtered[t.themeCursor].Name)
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("/"))):
		t.mode = themesModeFilter
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

	case key.Matches(msg, key.NewBinding(key.WithKeys("e"))):
		// Toggle inline editing for highlighted theme.
		if t.themeCursor < len(t.filtered) && t.resolver != nil {
			ti := t.filtered[t.themeCursor]
			resolved, err := t.resolver.Resolve(ti.Name)
			if err == nil {
				t.editTheme = resolved
				t.editName = ti.Name
				t.editCursor = 0
				t.pickerActive = false
				t.editing = true
			}
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("c"))):
		// Clone highlighted theme — prompt for name.
		if t.themeCursor < len(t.filtered) && t.resolver != nil {
			ti := t.filtered[t.themeCursor]
			resolved, err := t.resolver.Resolve(ti.Name)
			if err == nil {
				t.editTheme = resolved
				t.editName = ti.Name + "-custom"
				t.editing = true
				t.namingActive = true
				t.nameInput.SetValue(t.editName)
				t.nameInput.Focus()
				t.nameInput.CursorEnd()
				return t, textinput.Blink
			}
		}
		return t, nil
	}

	return t, nil
}

// ============================================================================
// Key handling — Filter mode
// ============================================================================

func (t *ThemesTab) handleFilterKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		t.mode = themesModeList
		t.filter.SetValue("")
		t.filter.Blur()
		t.applyFilter()
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		t.mode = themesModeList
		t.filter.Blur()
		return t, nil

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
	}

	var cmd tea.Cmd
	t.filter, cmd = t.filter.Update(msg)
	t.applyFilter()
	return t, cmd
}

// ============================================================================
// Key handling — Bar section (unchanged)
// ============================================================================

func (t *ThemesTab) handleBarKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	totalPresets := len(t.barPresets)
	totalSegments := len(themesBarSegmentLabels)

	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
		if t.barInSegments {
			segIdx := t.barCursor - totalPresets
			if segIdx <= 0 {
				t.barInSegments = false
				t.barCursor = totalPresets - 1
			} else {
				t.barCursor--
			}
		} else {
			if t.barCursor > 0 {
				t.barCursor--
			}
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
		if t.barInSegments {
			segIdx := t.barCursor - totalPresets
			if segIdx < totalSegments-1 {
				t.barCursor++
			}
		} else {
			if t.barCursor < totalPresets-1 {
				t.barCursor++
			} else {
				t.barInSegments = true
				t.barCursor = totalPresets
			}
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("enter", " "))):
		if t.barInSegments {
			segIdx := t.barCursor - totalPresets
			if segIdx >= 0 && segIdx < totalSegments {
				t.toggleSegment(themesBarSegmentLabels[segIdx].Field)
				t.cfg.Bar.Segments = t.barSegments
				return t, t.saveConfig()
			}
		} else if t.barCursor < totalPresets {
			preset := t.barPresets[t.barCursor]
			t.currentBar = preset.String()
			t.cfg.Bar.Preset = preset.String()
			return t, t.saveConfig()
		}
		return t, nil
	}

	return t, nil
}

// ============================================================================
// Key handling — Inline editor (within Colors section)
// ============================================================================

func (t *ThemesTab) handleEditorKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	// Clone/save-as name input.
	if t.namingActive {
		switch msg.Type {
		case tea.KeyEnter:
			name := strings.TrimSpace(t.nameInput.Value())
			if name != "" {
				t.editName = name
				t.editTheme.Name = name
				t.namingActive = false
				t.nameInput.Blur()
				return t, t.saveThemeFile()
			}
			t.namingActive = false
			t.nameInput.Blur()
			return t, nil
		case tea.KeyEscape:
			t.namingActive = false
			t.nameInput.Blur()
			return t, nil
		}
		var cmd tea.Cmd
		t.nameInput, cmd = t.nameInput.Update(msg)
		return t, cmd
	}

	// Color picker active.
	if t.pickerActive {
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			// Confirm color.
			if t.editCursor < len(t.editSlots) {
				t.editSlots[t.editCursor].Set(&t.editTheme, t.picker.Value())
			}
			t.pickerActive = false
			return t, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			// Cancel — restore original color.
			if t.editCursor < len(t.editSlots) {
				t.editSlots[t.editCursor].Set(&t.editTheme, t.pickerOrigColor)
			}
			t.pickerActive = false
			return t, nil
		default:
			picker, cmd := t.picker.Update(msg)
			t.picker = *picker
			// Live preview: update the slot as user adjusts.
			if t.editCursor < len(t.editSlots) {
				t.editSlots[t.editCursor].Set(&t.editTheme, t.picker.Value())
			}
			return t, cmd
		}
	}

	// Slot navigation and editor commands.
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
		if t.editCursor > 0 {
			t.editCursor--
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
		if t.editCursor < len(t.editSlots)-1 {
			t.editCursor++
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		// Open HSL picker for current slot.
		if t.editCursor < len(t.editSlots) {
			slot := t.editSlots[t.editCursor]
			c := slot.Get(t.editTheme)
			t.pickerOrigColor = c // save for Esc revert
			t.picker = views.NewColorPicker(slot.Label, c)
			t.picker.Resize(t.width - 10)
			t.pickerActive = true
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("s"))):
		// Save edited colors as custom theme (prompt for name).
		t.namingActive = true
		t.nameInput.SetValue(t.editName)
		t.nameInput.Focus()
		t.nameInput.CursorEnd()
		return t, textinput.Blink

	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		// Exit editing mode back to theme list.
		t.editing = false
		t.pickerActive = false
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("G"))):
		t.editCursor = len(t.editSlots) - 1
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("g"))):
		t.editCursor = 0
		return t, nil
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

// ============================================================================
// View — Colors section
// ============================================================================

func (t *ThemesTab) viewColors() string {
	var b strings.Builder

	b.WriteString("\n")
	currentLabel := "none"
	if t.currentTheme != "" {
		currentLabel = t.currentTheme
	}
	b.WriteString(t.styles.Dim.Render("Current: ") + t.styles.Success.Render(currentLabel))
	b.WriteString("  " + t.styles.Dim.Render(fmt.Sprintf("%d themes", len(t.themes))))
	b.WriteString("\n\n")

	// Filter bar.
	if t.mode == themesModeFilter {
		prompt := t.styles.Accent.Render("  / ")
		b.WriteString(prompt + t.filter.View() + "\n\n")
	} else if t.filter.Value() != "" {
		b.WriteString(t.styles.Dim.Render("  filter: "+t.filter.Value()) + "\n\n")
	}

	// Theme list grouped by source.
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
		b.WriteString(t.viewGroupedThemeList())
	}

	// Color strip for highlighted theme (always visible when not editing).
	if !t.editing && t.themeCursor < len(t.filtered) && t.resolver != nil {
		b.WriteString("\n")
		swatch := t.renderSwatch(t.filtered[t.themeCursor])
		if swatch != "" {
			b.WriteString("  " + swatch + "\n")
		}
	}

	// Inline editor below the list when editing.
	if t.editing {
		b.WriteString("\n")
		b.WriteString(t.viewInlineEditor())
	}

	return b.String()
}

// viewGroupedThemeList renders themes grouped by source with section headers.
func (t *ThemesTab) viewGroupedThemeList() string {
	var b strings.Builder

	// Group filtered themes by source.
	type group struct {
		header string
		items  []indexedTheme
	}

	bundled := group{header: "Bundled"}
	downloaded := group{header: "Downloaded"}
	custom := group{header: "Custom"}

	for i, ti := range t.filtered {
		it := indexedTheme{globalIdx: i, info: ti}
		switch ti.Source {
		case theme.SourceBundled:
			bundled.items = append(bundled.items, it)
		case theme.SourceIterm2:
			downloaded.items = append(downloaded.items, it)
		case theme.SourceUser:
			custom.items = append(custom.items, it)
		}
	}

	// Determine available height for scrollable list.
	availableHeight := t.height - 14
	if t.editing {
		availableHeight -= 12
	}
	if availableHeight < 5 {
		availableHeight = 10
	}

	// Build flat list of renderable items (headers + entries) with scroll window.
	type listItem struct {
		isHeader bool
		text     string
	}
	var allItems []listItem

	groups := []group{custom, bundled, downloaded}
	for _, g := range groups {
		if len(g.items) > 0 {
			allItems = append(allItems, listItem{isHeader: true, text: g.header})
			for _, it := range g.items {
				allItems = append(allItems, listItem{isHeader: false, text: t.renderThemeEntry(it.globalIdx, it.info)})
			}
		}
	}

	// If no downloaded themes exist, show hint.
	hasDownloaded := len(downloaded.items) > 0
	if !hasDownloaded {
		allItems = append(allItems, listItem{isHeader: true, text: "Downloaded"})
		allItems = append(allItems, listItem{isHeader: false, text: "    " + t.styles.Dim.Render("Press d to download 300+ themes from iterm2-color-schemes") + "\n"})
	}

	// Find which line index corresponds to the cursor for scroll anchoring.
	cursorLineIdx := 0
	lineIdx := 0
	for _, g := range groups {
		if len(g.items) > 0 {
			lineIdx++ // header
			for _, it := range g.items {
				if it.globalIdx == t.themeCursor {
					cursorLineIdx = lineIdx
				}
				lineIdx++
			}
		}
	}
	if !hasDownloaded {
		// Account for the hint lines.
		lineIdx += 2
	}

	// Apply scroll window.
	start := 0
	if cursorLineIdx >= availableHeight {
		start = cursorLineIdx - availableHeight + 1
	}
	end := start + availableHeight
	if end > len(allItems) {
		end = len(allItems)
	}

	if start > 0 {
		b.WriteString(t.styles.Dim.Render("  ^ " + fmt.Sprintf("%d more", start)) + "\n")
	}

	for i := start; i < end; i++ {
		item := allItems[i]
		if item.isHeader {
			b.WriteString("  " + t.styles.Muted.Bold(true).Render(item.text) + "\n")
		} else {
			b.WriteString(item.text)
		}
	}

	if end < len(allItems) {
		b.WriteString(t.styles.Dim.Render("  v " + fmt.Sprintf("%d more", len(allItems)-end)) + "\n")
	}

	return b.String()
}

type indexedTheme struct {
	globalIdx int
	info      theme.ThemeInfo
}

func (t *ThemesTab) renderThemeEntry(idx int, ti theme.ThemeInfo) string {
	selected := idx == t.themeCursor
	isCurrent := ti.Name == t.currentTheme

	cursor := "  "
	if selected {
		cursor = t.styles.Accent.Render("| ")
	}

	nameStyle := t.styles.Normal
	if selected {
		nameStyle = t.styles.Accent.Bold(true)
	}
	name := nameStyle.Render(ti.Name)

	currentMark := ""
	if isCurrent {
		currentMark = t.styles.Success.Render(" *")
	}

	var modeTag string
	if ti.IsDark {
		modeTag = t.styles.Dim.Render(" dark")
	} else {
		modeTag = t.styles.Accent.Render(" light")
	}

	return "  " + cursor + name + currentMark + modeTag + "\n"
}

func (t *ThemesTab) renderSwatch(ti theme.ThemeInfo) string {
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

// ============================================================================
// View — Inline editor (slots below theme list in Colors section)
// ============================================================================

func (t *ThemesTab) viewInlineEditor() string {
	var b strings.Builder

	nameLabel := t.editName
	if nameLabel == "" {
		nameLabel = "(new theme)"
	}
	b.WriteString("  " + t.styles.Dim.Render("Editing: ") + t.styles.Accent.Render(nameLabel))
	b.WriteString("\n\n")

	// Clone/save-as name input.
	if t.namingActive {
		b.WriteString("  " + t.styles.Accent.Render("Save as: ") + t.nameInput.View() + "\n\n")
	}

	// Slot list with scrolling.
	availableHeight := t.height / 3
	if t.pickerActive {
		availableHeight -= 7 // space for picker
	}
	if availableHeight < 5 {
		availableHeight = 8
	}

	start := 0
	if t.editCursor >= availableHeight {
		start = t.editCursor - availableHeight + 1
	}
	end := start + availableHeight
	if end > len(t.editSlots) {
		end = len(t.editSlots)
	}

	if start > 0 {
		b.WriteString(t.styles.Dim.Render("  ^ " + fmt.Sprintf("%d more", start)) + "\n")
	}

	for i := start; i < end; i++ {
		slot := t.editSlots[i]
		selected := i == t.editCursor
		color := slot.Get(t.editTheme)

		cursor := "  "
		if selected {
			cursor = t.styles.Accent.Render("| ")
		}

		// Color swatch block.
		swatchStyle := lipgloss.NewStyle().
			Background(lipgloss.Color(color.Hex())).
			Foreground(lipgloss.Color(color.Hex()))
		swatch := swatchStyle.Render("  ")

		labelStyle := t.styles.Normal
		if selected {
			labelStyle = t.styles.Accent.Bold(true)
		}

		hexStr := t.styles.Dim.Render(color.Hex())
		b.WriteString("  " + cursor + swatch + " " + labelStyle.Render(slot.Label) + "  " + hexStr + "\n")
	}

	if end < len(t.editSlots) {
		b.WriteString(t.styles.Dim.Render("  v " + fmt.Sprintf("%d more", len(t.editSlots)-end)) + "\n")
	}

	// Color picker (inline below the slots when active).
	if t.pickerActive {
		b.WriteString("\n")
		b.WriteString(t.picker.View())
	}

	return b.String()
}

// ============================================================================
// View — Bar section (unchanged)
// ============================================================================

func (t *ThemesTab) viewBar() string {
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
		selected := i == t.barCursor && !t.barInSegments
		isCurrent := preset.String() == t.currentBar

		cursor := "  "
		if selected {
			cursor = t.styles.Accent.Render("| ")
		}

		nameStyle := t.styles.Normal
		if selected {
			nameStyle = t.styles.Accent.Bold(true)
		}

		currentMark := ""
		if isCurrent {
			currentMark = t.styles.Success.Render(" *")
		}

		b.WriteString("  " + cursor + nameStyle.Render(preset.String()) + currentMark + "\n")

		if palette != nil {
			preview := bar.RenderPreviewWithSegments(preset, palette, t.barSegments)
			b.WriteString("    " + preview + "\n")
		}
		b.WriteString("\n")
	}

	// Segment toggles.
	b.WriteString("  " + t.styles.Muted.Render("Segments") + "\n\n")

	totalPresets := len(t.barPresets)
	for i, seg := range themesBarSegmentLabels {
		idx := totalPresets + i
		selected := t.barInSegments && t.barCursor == idx

		cursor := "  "
		if selected {
			cursor = t.styles.Accent.Render("| ")
		}

		enabled := t.segmentEnabled(seg.Field)
		checkbox := t.styles.Dim.Render("[ ]")
		if enabled {
			checkbox = t.styles.Success.Render("[x]")
		}

		label := t.styles.Normal.Render(seg.Label)
		if selected {
			label = t.styles.Accent.Render(seg.Label)
		}

		b.WriteString("  " + cursor + checkbox + " " + label + "\n")
	}

	return b.String()
}

// ============================================================================
// Data + Commands
// ============================================================================

func (t *ThemesTab) fetchData(reqID int64) tea.Cmd {
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
			return themesDataMsg{reqID: reqID, err: err}
		}

		exists := config.ConfigExists(fs)
		cfg, err := config.Load(fs, cfgPath)
		if err != nil {
			cfg = config.DefaultConfig()
		} else {
			currentTheme = cfg.Theme
		}

		return themesDataMsg{
			reqID:        reqID,
			themes:       themes,
			currentTheme: currentTheme,
			cfg:          cfg,
			cfgExists:    exists,
		}
	}
}

func (t *ThemesTab) applyTheme(name string) tea.Cmd {
	fs := t.fs
	runner := t.runner
	resolver := t.resolver
	reqID := t.reqID
	return func() tea.Msg {
		cfgPath, err := config.ConfigPath(fs)
		if err != nil {
			return themesApplyMsg{reqID: reqID, err: err}
		}

		cfg, err := config.Load(fs, cfgPath)
		if err != nil {
			cfg = config.DefaultConfig()
		}

		cfg.Theme = name
		if err := config.Save(fs, cfgPath, cfg); err != nil {
			return themesApplyMsg{reqID: reqID, err: err}
		}

		// Hot-reload: apply theme env vars + bar colors.
		var pal *theme.Palette
		var sty *tui.Styles
		if runner != nil && resolver != nil {
			resolved, resolveErr := resolver.Resolve(name)
			if resolveErr == nil {
				_ = theme.Apply(runner, fs, &cfg, resolved, cfgPath)
				p := resolved.SemanticPalette()
				preset, _ := bar.PresetFromString(cfg.Bar.Preset)
				_ = bar.Apply(runner, preset, &p)
				pal = &p
				s := tui.NewStyles(&p)
				sty = &s
			}
		}

		return themesApplyMsg{
			reqID:     reqID,
			themeName: name,
			palette:   pal,
			styles:    sty,
		}
	}
}

func (t *ThemesTab) saveConfig() tea.Cmd {
	fs := t.fs
	runner := t.runner
	cfg := t.cfg
	reqID := t.reqID
	resolver := t.resolver
	return func() tea.Msg {
		cfgPath, err := config.ConfigPath(fs)
		if err != nil {
			return themesConfigSaveMsg{reqID: reqID, err: err}
		}

		if err := config.Save(fs, cfgPath, cfg); err != nil {
			return themesConfigSaveMsg{reqID: reqID, err: err}
		}

		// Apply bar live.
		if runner != nil && resolver != nil {
			preset, _ := bar.PresetFromString(cfg.Bar.Preset)
			resolved, resolveErr := resolver.Resolve(cfg.Theme)
			if resolveErr == nil {
				p := resolved.SemanticPalette()
				_ = bar.Apply(runner, preset, &p)
			}
		}

		return themesConfigSaveMsg{reqID: reqID}
	}
}

func (t *ThemesTab) saveThemeFile() tea.Cmd {
	fs := t.fs
	editTheme := t.editTheme
	editName := t.editName
	reqID := t.reqID
	return func() tea.Msg {
		home, err := fs.UserHomeDir()
		if err != nil {
			return themesSaveThemeMsg{reqID: reqID, err: err}
		}

		dir := home + "/.zmux/themes"
		_ = fs.MkdirAll(dir, 0755)

		path := dir + "/" + editName
		if err := theme.WriteFile(fs, path, editTheme); err != nil {
			return themesSaveThemeMsg{reqID: reqID, err: fmt.Errorf("save theme: %w", err)}
		}

		return themesSaveThemeMsg{reqID: reqID, themeName: editName}
	}
}

// ============================================================================
// Preview / Revert
// ============================================================================

func (t *ThemesTab) revertPreview() {
	t.previewing = false
}

func (t *ThemesTab) emitRevert() tea.Cmd {
	if t.savedPalette == nil || t.savedStyles == nil {
		return nil
	}
	pal := *t.savedPalette
	sty := *t.savedStyles
	return func() tea.Msg {
		return dashboard.ThemeChangeIntent{Palette: pal, Styles: sty}
	}
}

// ============================================================================
// Helpers
// ============================================================================

func (t *ThemesTab) applyFilter() {
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

func (t *ThemesTab) toggleSegment(field string) {
	switch field {
	case "git":
		t.barSegments.Git = !t.barSegments.Git
	case "workspace":
		t.barSegments.Workspace = !t.barSegments.Workspace
	case "clock":
		t.barSegments.Clock = !t.barSegments.Clock
	case "lang":
		t.barSegments.Lang = !t.barSegments.Lang
	case "directory":
		t.barSegments.Directory = !t.barSegments.Directory
	case "process":
		t.barSegments.Process = !t.barSegments.Process
	case "group":
		t.barSegments.Group = !t.barSegments.Group
	}
}

func (t *ThemesTab) segmentEnabled(field string) bool {
	switch field {
	case "git":
		return t.barSegments.Git
	case "workspace":
		return t.barSegments.Workspace
	case "clock":
		return t.barSegments.Clock
	case "lang":
		return t.barSegments.Lang
	case "directory":
		return t.barSegments.Directory
	case "process":
		return t.barSegments.Process
	case "group":
		return t.barSegments.Group
	}
	return false
}
