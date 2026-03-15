package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"

	"github.com/donjor/zmux/internal/theme"
	"github.com/donjor/zmux/internal/tui/views"
)

// themePickerMode tracks the current mode of the theme picker.
type themePickerMode int

const (
	themeList themePickerMode = iota
	themeFilter
)

// themeKeymap defines keybindings for the theme picker.
var themeKeys = struct {
	Quit   key.Binding
	Enter  key.Binding
	Back   key.Binding
	Filter key.Binding
	Up     key.Binding
	Down   key.Binding
}{
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "apply"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "cancel"),
	),
	Filter: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "filter"),
	),
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("up/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("down/j", "down"),
	),
}

// ThemePickerModel is the bubbletea model for the theme picker TUI.
type ThemePickerModel struct {
	themes   []theme.ThemeInfo
	resolver *theme.Resolver
	filtered []theme.ThemeInfo
	cursor   int
	mode     themePickerMode
	filter   textinput.Model
	width    int
	height   int
	styles   Styles

	// Result state (read after quit).
	Chosen   string // theme name to apply
	Quitting bool
}

// NewThemePickerModel creates a new theme picker model.
func NewThemePickerModel(resolver *theme.Resolver, styles Styles) ThemePickerModel {
	ti := textinput.New()
	ti.Placeholder = "filter themes..."
	ti.CharLimit = 64

	themes := resolver.List()

	m := ThemePickerModel{
		themes:   themes,
		resolver: resolver,
		filtered: themes,
		styles:   styles,
		filter:   ti,
	}

	return m
}

// themesLoadedMsg signals that themes have been loaded.
type themesLoadedMsg struct {
	themes []theme.ThemeInfo
}

// Init loads themes.
func (m ThemePickerModel) Init() tea.Cmd {
	return nil
}

// Update handles messages and user input.
func (m ThemePickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Forward to text input if filtering.
	if m.mode == themeFilter {
		var cmd tea.Cmd
		m.filter, cmd = m.filter.Update(msg)
		m.applyFilter()
		return m, cmd
	}

	return m, nil
}

func (m ThemePickerModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle filter mode.
	if m.mode == themeFilter {
		switch {
		case key.Matches(msg, themeKeys.Back):
			m.mode = themeList
			m.filter.SetValue("")
			m.filter.Blur()
			m.applyFilter()
			return m, nil
		case key.Matches(msg, themeKeys.Enter):
			m.mode = themeList
			m.filter.Blur()
			return m, nil
		default:
			var cmd tea.Cmd
			m.filter, cmd = m.filter.Update(msg)
			m.applyFilter()
			return m, cmd
		}
	}

	// Normal list mode.
	switch {
	case key.Matches(msg, themeKeys.Quit):
		m.Quitting = true
		return m, tea.Quit

	case key.Matches(msg, themeKeys.Back):
		m.Quitting = true
		return m, tea.Quit

	case key.Matches(msg, themeKeys.Up):
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case key.Matches(msg, themeKeys.Down):
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}
		return m, nil

	case key.Matches(msg, themeKeys.Enter):
		if m.cursor < len(m.filtered) {
			m.Chosen = m.filtered[m.cursor].Name
		}
		return m, tea.Quit

	case key.Matches(msg, themeKeys.Filter):
		m.mode = themeFilter
		m.filter.Focus()
		return m, textinput.Blink
	}

	return m, nil
}

func (m *ThemePickerModel) applyFilter() {
	query := m.filter.Value()
	if query == "" {
		m.filtered = m.themes
	} else {
		names := make([]string, len(m.themes))
		for i, ti := range m.themes {
			names[i] = ti.Name
		}
		matches := fuzzy.Find(query, names)
		m.filtered = make([]theme.ThemeInfo, len(matches))
		for i, match := range matches {
			m.filtered[i] = m.themes[match.Index]
		}
	}

	// Clamp cursor.
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

// View renders the theme picker UI.
func (m ThemePickerModel) View() string {
	if m.Quitting && m.Chosen == "" {
		return ""
	}

	var b strings.Builder

	// Header.
	title := m.styles.Title.Render("zmux")
	subtitle := m.styles.Muted.Render(" theme picker")
	b.WriteString(title + subtitle + "\n\n")

	// Filter bar.
	if m.mode == themeFilter {
		b.WriteString(m.filter.View() + "\n\n")
	} else if m.filter.Value() != "" {
		b.WriteString(m.styles.Dim.Render("filter: "+m.filter.Value()) + "\n\n")
	}

	// Calculate available height for the list.
	headerLines := 2 // title + blank
	helpLines := 2   // blank + help
	swatchLines := 4 // swatch preview area
	filterLines := 0
	if m.mode == themeFilter || m.filter.Value() != "" {
		filterLines = 2
	}
	availableHeight := m.height - headerLines - helpLines - swatchLines - filterLines
	if availableHeight < 5 {
		availableHeight = 10 // minimum
	}

	// Theme list with scrolling.
	if len(m.filtered) == 0 {
		b.WriteString(m.styles.Muted.Render("  No themes found.") + "\n")
	} else {
		// Calculate visible window.
		start := 0
		if m.cursor >= availableHeight {
			start = m.cursor - availableHeight + 1
		}
		end := start + availableHeight
		if end > len(m.filtered) {
			end = len(m.filtered)
		}

		for i := start; i < end; i++ {
			b.WriteString(m.renderThemeEntry(i, m.filtered[i]))
		}

		// Scroll indicators.
		if start > 0 {
			b.WriteString(m.styles.Dim.Render("  ... more above") + "\n")
		}
		if end < len(m.filtered) {
			b.WriteString(m.styles.Dim.Render("  ... more below") + "\n")
		}
	}

	// Color swatch for the selected theme.
	if m.cursor < len(m.filtered) {
		b.WriteString("\n")
		swatch := m.renderSwatch(m.filtered[m.cursor])
		if swatch != "" {
			b.WriteString(swatch + "\n")
		}
	}

	// Help bar.
	b.WriteString("\n")
	b.WriteString(m.renderThemeHelp())

	return b.String()
}

func (m ThemePickerModel) renderThemeEntry(idx int, ti theme.ThemeInfo) string {
	cursor := "  "
	if idx == m.cursor {
		cursor = m.styles.Accent.Render("> ")
	}

	// Name.
	nameStyle := m.styles.Normal
	if idx == m.cursor {
		nameStyle = m.styles.Selected
	}
	name := nameStyle.Render(ti.Name)

	// Source tag.
	var sourceTag string
	switch ti.Source {
	case theme.SourceBundled:
		sourceTag = m.styles.Info.Render(" [bundled]")
	case theme.SourceUser:
		sourceTag = m.styles.Success.Render(" [user]")
	case theme.SourceIterm2:
		sourceTag = m.styles.Special.Render(" [iterm2]")
	}

	// Dark/light tag.
	var modeTag string
	if ti.IsDark {
		modeTag = m.styles.Dim.Render(" dark")
	} else {
		modeTag = m.styles.Accent.Render(" light")
	}

	return cursor + name + sourceTag + modeTag + "\n"
}

func (m ThemePickerModel) renderSwatch(ti theme.ThemeInfo) string {
	t, err := m.resolver.Resolve(ti.Name)
	if err != nil {
		return ""
	}
	palette := t.SemanticPalette()

	width := m.width
	if width <= 0 {
		width = 80
	}
	return views.RenderSwatch(&palette, width)
}

func (m ThemePickerModel) renderThemeHelp() string {
	parts := []string{
		"enter:apply",
		"/:filter",
		"esc:cancel",
		"q:quit",
	}
	return m.styles.Help.Render("  " + strings.Join(parts, "  "))
}
