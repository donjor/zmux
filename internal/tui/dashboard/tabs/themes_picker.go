package tabs

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/sahilm/fuzzy"

	"github.com/donjor/zmux/internal/theme"
	"github.com/donjor/zmux/internal/tui/dashboard"
	"github.com/donjor/zmux/internal/tui/views"
)

// ============================================================================
// Key handling — Colors section (theme list)
// ============================================================================

func (t *ThemesTab) handleColorsKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		// Reaches the tab only when a committed filter is active (see
		// CapturesEscape); clear it. A second Esc then closes the dashboard.
		if t.filter.Value() != "" {
			t.filter.SetValue("")
			t.applyFilter()
		}
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
// View — Colors section
// ============================================================================

// renderColorsContent returns the full colors section content and
// the cursor line within it (for viewport scrolling).
func (t *ThemesTab) renderColorsContent() (string, int) {
	var b strings.Builder
	cursorLine := 0
	lineCount := 0

	b.WriteString("\n")
	lineCount++
	currentLabel := "none"
	if t.currentTheme != "" {
		currentLabel = t.currentTheme
	}
	b.WriteString(t.styles.Dim.Render("Current: ") + t.styles.Success.Render(currentLabel))
	b.WriteString("  " + t.styles.Dim.Render(fmt.Sprintf("%d themes", len(t.themes))))
	b.WriteString("\n\n")
	lineCount += 2

	// Filter bar.
	if t.mode == themesModeFilter {
		prompt := t.styles.Accent.Render("  / ")
		b.WriteString(prompt + t.filter.View() + "\n\n")
		lineCount += 2
	} else if t.filter.Value() != "" {
		b.WriteString(t.styles.Dim.Render("  filter: "+t.filter.Value()) + "\n\n")
		lineCount += 2
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
		listContent, listCursor := t.renderThemeList()
		cursorLine = lineCount + listCursor
		b.WriteString(listContent)
		lineCount += strings.Count(listContent, "\n")
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
		lineCount++
		cursorLine = lineCount // scroll to editor area
		b.WriteString(t.viewInlineEditor())
	}

	return b.String(), cursorLine
}

// renderThemeList renders themes grouped by source with section headers.
// Returns the rendered content and the cursor line within it.
func (t *ThemesTab) renderThemeList() (string, int) {
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

	groups := []group{custom, bundled, downloaded}
	cursorLine := 0
	lineCount := 0

	for _, g := range groups {
		if len(g.items) > 0 {
			b.WriteString("  " + t.styles.Muted.Bold(true).Render(g.header) + "\n")
			lineCount++
			for _, it := range g.items {
				if it.globalIdx == t.themeCursor {
					cursorLine = lineCount
				}
				entry := t.renderThemeEntry(it.globalIdx, it.info)
				b.WriteString(entry)
				lineCount += strings.Count(entry, "\n")
			}
		}
	}

	// If no downloaded themes exist, show hint.
	if len(downloaded.items) == 0 {
		b.WriteString("  " + t.styles.Muted.Bold(true).Render("Downloaded") + "\n")
		b.WriteString("    " + t.styles.Dim.Render("Press d to download 300+ themes from iterm2-color-schemes") + "\n")
	}

	return b.String(), cursorLine
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
// Filter helper
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
