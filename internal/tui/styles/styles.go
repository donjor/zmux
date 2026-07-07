// Package styles holds the shared lipgloss style set derived from a theme
// palette, consumed by every zmux TUI surface.
package styles

import (
	"charm.land/lipgloss/v2"

	"github.com/donjor/zmux/internal/theme"
)

// Styles holds lipgloss styles derived from a theme palette.
type Styles struct {
	Title    lipgloss.Style
	Subtitle lipgloss.Style
	Selected lipgloss.Style
	Normal   lipgloss.Style
	// Muted, Dim, and Help are distinct semantic names for the same
	// de-emphasized tone (palette.Dim / ANSI 8): muted body text, dim chrome,
	// help hints. Kept separate so a theme can later split them without touching
	// call sites; the constructors derive all three from one style value.
	Muted   lipgloss.Style
	Accent  lipgloss.Style
	Error   lipgloss.Style
	Success lipgloss.Style
	Info    lipgloss.Style
	Special lipgloss.Style
	Dim     lipgloss.Style
	Help    lipgloss.Style
}

// NewStyles creates a Styles set from a theme Palette.
func NewStyles(palette *theme.Palette) Styles {
	if palette == nil {
		return DefaultStyles()
	}

	dim := lipgloss.NewStyle().
		Foreground(lipgloss.Color(palette.Dim.Hex()))

	return Styles{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(palette.Accent.Hex())),

		Subtitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Muted.Hex())),

		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(palette.FG.Hex())).
			Background(lipgloss.Color(palette.BGDim.Hex())),

		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.FG.Hex())),

		Muted: dim,

		Accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Accent.Hex())),

		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Error.Hex())),

		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Success.Hex())),

		Info: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Info.Hex())),

		Special: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Special.Hex())),

		Dim:  dim,
		Help: dim,
	}
}

// DefaultStyles returns styles using default ANSI colors when no theme is available.
func DefaultStyles() Styles {
	dim := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8"))

	return Styles{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("3")), // yellow

		Subtitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("7")), // white

		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")). // bright white
			Background(lipgloss.Color("8")),  // bright black

		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")),

		Muted: dim,

		Accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("3")),

		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("1")),

		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("2")),

		Info: lipgloss.NewStyle().
			Foreground(lipgloss.Color("4")),

		Special: lipgloss.NewStyle().
			Foreground(lipgloss.Color("5")),

		Dim:  dim,
		Help: dim,
	}
}
