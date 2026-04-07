package tabs

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/donjor/zmux/internal/theme"
	"github.com/donjor/zmux/internal/tui/dashboard"
	"github.com/donjor/zmux/internal/tui/views"
)

// ============================================================================
// Editor slot labels — UPPERCASE, clean names
// ============================================================================

// editorSlot captures a single editable field on a theme.Theme — four
// named colors (background/foreground/cursor/selection) and the 16 ANSI
// palette entries. Get and Set are closures rather than a field index
// so the picker UI can work with any theme field uniformly.
type editorSlot struct {
	Label string
	Get   func(theme.Theme) theme.Color
	Set   func(*theme.Theme, theme.Color)
}

// buildEditorSlots returns the 20 slots in display order: 4 named
// colors followed by 16 palette entries with UPPERCASE labels.
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
