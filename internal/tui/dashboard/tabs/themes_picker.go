package tabs

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/donjor/zmux/internal/keys"
	"github.com/donjor/zmux/internal/theme"
	"github.com/donjor/zmux/internal/tui/dashboard"
	"github.com/donjor/zmux/internal/tui/filter"
	"github.com/donjor/zmux/internal/tui/outline"
	"github.com/donjor/zmux/internal/tui/views"
)

// Theme-family keys with no cross-surface analogue live here as package-level
// bindings (idiom A) — built once, matched in the Update paths. Generic
// list/confirm/cancel/filter navigation comes from the internal/keys registry
// (keys.TUI*); only the theme-specific verbs are declared locally.
var (
	themeEditKey  = key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit"))
	themeCloneKey = key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "clone"))
	themeSaveKey  = key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "save"))
)

// themeRowID returns the stable outline row ID for a theme. Matches the
// standalone themepicker's shape so both surfaces restore the cursor to the
// same theme across filter changes.
func themeRowID(name string) string { return "theme:" + name }

// themeGroupID returns the stable outline row ID for a source-group header
// (Custom / Bundled / Downloaded). Header rows are non-selectable dividers.
func themeGroupID(label string) string { return "themegroup:" + label }

// buildThemeRows turns the current filtered theme list into grouped outline
// rows: a non-selectable header per non-empty source group followed by its
// selectable theme rows. Group order is Custom → Bundled → Downloaded, the
// same order the list renders, so tree navigation walks the visible order.
func (t *ThemesTab) buildThemeRows() []outline.Row {
	type group struct {
		header string
		idxs   []int
	}
	custom := group{header: "Custom"}
	bundled := group{header: "Bundled"}
	downloaded := group{header: "Downloaded"}
	for i := range t.filtered {
		switch t.filtered[i].Source {
		case theme.SourceUser:
			custom.idxs = append(custom.idxs, i)
		case theme.SourceBundled:
			bundled.idxs = append(bundled.idxs, i)
		case theme.SourceIterm2:
			downloaded.idxs = append(downloaded.idxs, i)
		}
	}

	var rows []outline.Row
	for _, g := range []group{custom, bundled, downloaded} {
		if len(g.idxs) == 0 {
			continue
		}
		rows = append(rows, outline.Row{
			ID:         themeGroupID(g.header),
			Kind:       outline.RowExternalGroup,
			Label:      g.header,
			Selectable: false,
		})
		for _, i := range g.idxs {
			rows = append(rows, outline.Row{
				ID:         themeRowID(t.filtered[i].Name),
				Kind:       outline.RowSession,
				Depth:      1,
				ParentID:   themeGroupID(g.header),
				Label:      t.filtered[i].Name,
				Selectable: true,
				Data:       &t.filtered[i],
			})
		}
	}
	return rows
}

// currentThemeInfo returns the theme under the cursor, or nil when the cursor
// rests on a header / the list is empty.
func (t *ThemesTab) currentThemeInfo() *theme.ThemeInfo {
	row := t.tree.CurrentSelectable()
	if row == nil {
		return nil
	}
	ti, _ := outline.RowData[theme.ThemeInfo](row)
	return ti
}

// ============================================================================
// Key handling — Colors section (theme list)
// ============================================================================

func (t *ThemesTab) handleColorsKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.TUICancel):
		// Reaches the tab only when a committed filter is active (see
		// CapturesEscape); clear it. A second Esc then closes the dashboard.
		if t.filter.Value() != "" {
			t.filter.SetValue("")
			t.applyFilter()
		}
		return t, nil

	case key.Matches(msg, keys.TUIListUp):
		t.tree.MoveUp()
		return t, nil

	case key.Matches(msg, keys.TUIListDown):
		t.tree.MoveDown()
		return t, nil

	case key.Matches(msg, keys.TUIConfirm):
		// Apply highlighted theme (save config + hot reload).
		if ti := t.currentThemeInfo(); ti != nil {
			return t, t.applyTheme(ti.Name)
		}
		return t, nil

	case key.Matches(msg, keys.TUIFilter):
		t.mode = themesModeFilter
		t.filter.Focus()
		return t, textinput.Blink

	case key.Matches(msg, keys.TUIListBottom):
		t.tree.JumpBottom()
		return t, nil

	case key.Matches(msg, keys.TUIListTop):
		t.tree.JumpTop()
		return t, nil

	case key.Matches(msg, themeEditKey):
		// Toggle inline editing for highlighted theme.
		if ti := t.currentThemeInfo(); ti != nil && t.resolver != nil {
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

	case key.Matches(msg, themeCloneKey):
		// Clone highlighted theme — prompt for name.
		if ti := t.currentThemeInfo(); ti != nil && t.resolver != nil {
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
	case key.Matches(msg, keys.TUICancel):
		t.mode = themesModeList
		t.filter.SetValue("")
		t.filter.Blur()
		t.applyFilter()
		return t, nil

	case key.Matches(msg, keys.TUIConfirm):
		t.mode = themesModeList
		t.filter.Blur()
		return t, nil

	case key.Matches(msg, keys.TUIListUp):
		t.tree.MoveUp()
		return t, nil

	case key.Matches(msg, keys.TUIListDown):
		t.tree.MoveDown()
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
	if cur := t.currentThemeInfo(); !t.editing && cur != nil {
		b.WriteString("\n")
		swatch := views.ResolveSwatch(t.resolver, cur.Name, t.width)
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
// Returns the rendered content and the cursor line within it. The group order
// mirrors buildThemeRows so the highlighted row matches the tree cursor.
func (t *ThemesTab) renderThemeList() (string, int) {
	var b strings.Builder

	// Group filtered themes by source.
	type group struct {
		header string
		items  []theme.ThemeInfo
	}

	bundled := group{header: "Bundled"}
	downloaded := group{header: "Downloaded"}
	custom := group{header: "Custom"}

	for _, ti := range t.filtered {
		switch ti.Source {
		case theme.SourceBundled:
			bundled.items = append(bundled.items, ti)
		case theme.SourceIterm2:
			downloaded.items = append(downloaded.items, ti)
		case theme.SourceUser:
			custom.items = append(custom.items, ti)
		}
	}

	highlighted := ""
	if cur := t.currentThemeInfo(); cur != nil {
		highlighted = cur.Name
	}

	groups := []group{custom, bundled, downloaded}
	cursorLine := 0
	lineCount := 0

	for _, g := range groups {
		if len(g.items) > 0 {
			b.WriteString("  " + t.styles.Muted.Bold(true).Render(g.header) + "\n")
			lineCount++
			for _, ti := range g.items {
				selected := ti.Name == highlighted
				if selected {
					cursorLine = lineCount
				}
				entry := t.renderThemeEntry(ti, selected)
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

func (t *ThemesTab) renderThemeEntry(ti theme.ThemeInfo, selected bool) string {
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

// ============================================================================
// Filter helper
// ============================================================================

func (t *ThemesTab) applyFilter() {
	query := t.filter.Value()
	t.filtered = filter.Fuzzy(t.themes, query, func(ti theme.ThemeInfo) string { return ti.Name })
	// SetRows re-pins the cursor by stable theme ID when the highlighted theme
	// survives the filter, then falls through the outline restore hierarchy.
	t.tree.SetRows(t.buildThemeRows())
}
