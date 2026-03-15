// Package views provides reusable TUI rendering components.
package views

import (
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/donjor/zmux/internal/theme"
)

// swatchRenderer forces TrueColor output for consistent rendering.
var swatchRenderer = newSwatchRenderer()

func newSwatchRenderer() *lipgloss.Renderer {
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(termenv.TrueColor)
	return r
}

// swatchStyle creates a new style bound to the TrueColor renderer.
func swatchStyle() lipgloss.Style {
	return swatchRenderer.NewStyle()
}

// swatchEntry pairs a semantic role name with its color.
type swatchEntry struct {
	Name  string
	Color theme.Color
}

// RenderSwatch renders a grid of colored blocks, one per semantic role,
// with the role name below each block. Returns a multi-line string.
func RenderSwatch(palette *theme.Palette, width int) string {
	if palette == nil {
		return ""
	}

	entries := []swatchEntry{
		{"BG", palette.BG},
		{"FG", palette.FG},
		{"Surface", palette.Surface},
		{"Error", palette.Error},
		{"Success", palette.Success},
		{"Accent", palette.Accent},
		{"Info", palette.Info},
		{"Special", palette.Special},
		{"Meta", palette.Meta},
		{"Muted", palette.Muted},
		{"Dim", palette.Dim},
		{"Highlight", palette.Highlight},
	}

	// Calculate block width. We want to fit all 12 entries in the available
	// width with 1-char spacing between them.
	cols := 12
	if width < cols*5 {
		cols = 6 // two rows of 6 if width is tight
	}
	if width < cols*5 {
		cols = 4
	}

	blockWidth := 4
	spacing := 1

	var rows []string
	for i := 0; i < len(entries); i += cols {
		end := i + cols
		if end > len(entries) {
			end = len(entries)
		}

		chunk := entries[i:end]

		// Top line: colored blocks
		var blockLine strings.Builder
		for j, e := range chunk {
			if j > 0 {
				blockLine.WriteString(strings.Repeat(" ", spacing))
			}
			block := swatchStyle().
				Background(lipgloss.Color(e.Color.Hex())).
				Render(strings.Repeat(" ", blockWidth))
			blockLine.WriteString(block)
		}

		// Label line: role names
		var labelLine strings.Builder
		for j, e := range chunk {
			if j > 0 {
				labelLine.WriteString(strings.Repeat(" ", spacing))
			}
			// Pad or truncate the label to blockWidth
			label := e.Name
			if len(label) > blockWidth {
				label = label[:blockWidth]
			}
			for len(label) < blockWidth {
				label += " "
			}
			styled := swatchStyle().
				Foreground(lipgloss.Color(e.Color.Hex())).
				Render(label)
			labelLine.WriteString(styled)
		}

		rows = append(rows, blockLine.String())
		rows = append(rows, labelLine.String())
	}

	return strings.Join(rows, "\n")
}
