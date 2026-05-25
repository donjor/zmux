package preview

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Ayu-inspired palette borrowed from the cli-showcase visual lab. Keeping it
// here gives every preview page a stronger default vocabulary without forcing
// production UI surfaces to share prototype-only styles.
var (
	Gold   = lipgloss.Color("#e6b450")
	Blue   = lipgloss.Color("#53bdfa")
	Green  = lipgloss.Color("#7fd962")
	Red    = lipgloss.Color("#ea6c73")
	Purple = lipgloss.Color("#cda1fa")
	Teal   = lipgloss.Color("#90e1c6")
	Orange = lipgloss.Color("#ffb454")
	FG     = lipgloss.Color("#bfbdb6")
	Muted  = lipgloss.Color("#5a6378")
	Dim    = lipgloss.Color("#3e4452")
	BGDark = lipgloss.Color("#0b0e14")
	BGCard = lipgloss.Color("#11151c")
)

var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Gold)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(Muted)

	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Dim).
			Padding(1, 2)

	PanelActiveStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(Gold).
				Padding(1, 2)

	HeroStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(Gold).
			Foreground(FG).
			Padding(0, 2)

	TabActiveStyle = lipgloss.NewStyle().
			Foreground(Gold).
			Bold(true).
			Padding(0, 2)

	TabInactiveStyle = lipgloss.NewStyle().
				Foreground(Dim).
				Padding(0, 2)

	HelpStyle = lipgloss.NewStyle().Foreground(Dim)
	DimStyle  = lipgloss.NewStyle().Foreground(Dim)
	MuteStyle = lipgloss.NewStyle().Foreground(Muted)
	FGStyle   = lipgloss.NewStyle().Foreground(FG)
)

func Badge(label string, col color.Color) string {
	return lipgloss.NewStyle().
		Foreground(BGDark).
		Background(col).
		Bold(true).
		Padding(0, 1).
		Render(" " + label + " ")
}

func MetricCard(label, value, detail string, col color.Color, width int) string {
	if width < 12 {
		width = 12
	}
	content := lipgloss.NewStyle().Foreground(Muted).Bold(true).Render(label) + "\n" +
		lipgloss.NewStyle().Foreground(col).Bold(true).Render(value) + "\n" +
		lipgloss.NewStyle().Foreground(Muted).Render(detail)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Dim).
		Background(BGCard).
		Padding(0, 1).
		Width(width).
		Render(content)
}

func Sparkline(vals []float64, col color.Color) string {
	chars := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	style := lipgloss.NewStyle().Foreground(col)
	out := make([]rune, 0, len(vals))
	for _, v := range vals {
		if v < 0 {
			v = 0
		}
		if v > 1 {
			v = 1
		}
		idx := int(v * float64(len(chars)-1))
		out = append(out, chars[idx])
	}
	return style.Render(string(out))
}
