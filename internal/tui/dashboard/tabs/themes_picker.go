package tabs

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
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
