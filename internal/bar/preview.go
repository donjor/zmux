package bar

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/donjor/zmux/internal/theme"
)

// renderer forces TrueColor output so previews contain ANSI escapes
// regardless of whether stdout is a TTY.
var renderer = newTrueColorRenderer()

func newTrueColorRenderer() *lipgloss.Renderer {
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(termenv.TrueColor)
	return r
}

// RenderPreview returns an ANSI-colored string that approximates the
// appearance of the given status bar preset. Useful for terminal previews
// when picking a bar layout.
func RenderPreview(preset Preset, palette *theme.Palette) string {
	switch preset {
	case Minimal:
		return renderMinimalPreview(palette)
	case Powerline:
		return renderPowerlinePreview(palette)
	case Blocks:
		return renderBlocksPreview(palette)
	default:
		return renderDefaultPreview(palette)
	}
}

func colorStr(c theme.Color) string {
	return c.Hex()
}

// style creates a new Style bound to our TrueColor renderer.
func style() lipgloss.Style {
	return renderer.NewStyle()
}

func renderDefaultPreview(p *theme.Palette) string {
	surfaceBG := style().Background(lipgloss.Color(colorStr(p.Surface)))

	sessionPill := style().
		Background(lipgloss.Color(colorStr(p.Accent))).
		Foreground(lipgloss.Color(colorStr(p.BG))).
		Bold(true).
		Render(" main ")

	sep := surfaceBG.Foreground(lipgloss.Color(colorStr(p.Dim))).Render("\u2502")

	inactiveWin := surfaceBG.Foreground(lipgloss.Color(colorStr(p.Dim))).Render(" 1 zsh ")
	activeWin := surfaceBG.Foreground(lipgloss.Color(colorStr(p.Accent))).Bold(true).Render(" 2 nvim ")
	inactiveWin2 := surfaceBG.Foreground(lipgloss.Color(colorStr(p.Dim))).Render(" 3 htop ")

	clock := surfaceBG.Foreground(lipgloss.Color(colorStr(p.Muted))).Render(" 02:30 PM ")

	left := sessionPill + " " + inactiveWin + sep + activeWin + sep + inactiveWin2
	right := clock

	return padBar(left, right, p.Surface)
}

func renderMinimalPreview(p *theme.Palette) string {
	surfaceBG := style().Background(lipgloss.Color(colorStr(p.Surface)))

	session := surfaceBG.
		Foreground(lipgloss.Color(colorStr(p.Accent))).
		Bold(true).
		Render(" main ")

	pipe := surfaceBG.Foreground(lipgloss.Color(colorStr(p.Dim))).Render("\u2502")

	inactiveWin := surfaceBG.Foreground(lipgloss.Color(colorStr(p.Dim))).Render(" zsh ")
	activeWin := surfaceBG.Foreground(lipgloss.Color(colorStr(p.FG))).Bold(true).Render(" nvim ")
	inactiveWin2 := surfaceBG.Foreground(lipgloss.Color(colorStr(p.Dim))).Render(" htop ")

	clock := surfaceBG.Foreground(lipgloss.Color(colorStr(p.Dim))).Render(" 14:30 ")

	left := session + pipe + " " + inactiveWin + " " + activeWin + " " + inactiveWin2
	right := clock

	return padBar(left, right, p.Surface)
}

func renderPowerlinePreview(p *theme.Palette) string {
	sessionPill := style().
		Background(lipgloss.Color(colorStr(p.Accent))).
		Foreground(lipgloss.Color(colorStr(p.BG))).
		Bold(true).
		Render(" main ")

	arrow1 := style().
		Background(lipgloss.Color(colorStr(p.Surface))).
		Foreground(lipgloss.Color(colorStr(p.Accent))).
		Render("\ue0b0")

	inactiveWin := style().
		Background(lipgloss.Color(colorStr(p.Dim))).
		Foreground(lipgloss.Color(colorStr(p.Muted))).
		Render(" 1 zsh ")

	arrowInL := style().
		Background(lipgloss.Color(colorStr(p.Dim))).
		Foreground(lipgloss.Color(colorStr(p.Surface))).
		Render("\ue0b0")

	arrowInR := style().
		Background(lipgloss.Color(colorStr(p.Surface))).
		Foreground(lipgloss.Color(colorStr(p.Dim))).
		Render("\ue0b0")

	activeWin := style().
		Background(lipgloss.Color(colorStr(p.Accent))).
		Foreground(lipgloss.Color(colorStr(p.BG))).
		Bold(true).
		Render(" 2 nvim ")

	arrowActL := style().
		Background(lipgloss.Color(colorStr(p.Accent))).
		Foreground(lipgloss.Color(colorStr(p.Surface))).
		Render("\ue0b0")

	arrowActR := style().
		Background(lipgloss.Color(colorStr(p.Surface))).
		Foreground(lipgloss.Color(colorStr(p.Accent))).
		Render("\ue0b0")

	// Right side
	rArrowTime := style().
		Background(lipgloss.Color(colorStr(p.Surface))).
		Foreground(lipgloss.Color(colorStr(p.Dim))).
		Render("\ue0b2")

	timeSeg := style().
		Background(lipgloss.Color(colorStr(p.Dim))).
		Foreground(lipgloss.Color(colorStr(p.Muted))).
		Render(" 14:30 ")

	rArrowDate := style().
		Background(lipgloss.Color(colorStr(p.Dim))).
		Foreground(lipgloss.Color(colorStr(p.Accent))).
		Render("\ue0b2")

	dateSeg := style().
		Background(lipgloss.Color(colorStr(p.Accent))).
		Foreground(lipgloss.Color(colorStr(p.BG))).
		Bold(true).
		Render(" Mar 16 ")

	left := sessionPill + arrow1 + " " + arrowInL + inactiveWin + arrowInR + arrowActL + activeWin + arrowActR
	right := rArrowTime + timeSeg + rArrowDate + dateSeg

	return padBar(left, right, p.Surface)
}

func renderBlocksPreview(p *theme.Palette) string {
	surfaceBG := style().Background(lipgloss.Color(colorStr(p.Surface)))

	session := surfaceBG.
		Foreground(lipgloss.Color(colorStr(p.Accent))).
		Bold(true).
		Render(" [main] ")

	inactiveWin := surfaceBG.
		Foreground(lipgloss.Color(colorStr(p.Dim))).
		Render(" [1:zsh] ")

	activeWin := surfaceBG.
		Foreground(lipgloss.Color(colorStr(p.Accent))).
		Bold(true).
		Render(" [2:nvim] ")

	inactiveWin2 := surfaceBG.
		Foreground(lipgloss.Color(colorStr(p.Dim))).
		Render(" [3:htop] ")

	clock := surfaceBG.
		Foreground(lipgloss.Color(colorStr(p.Dim))).
		Render(" [14:30] ")

	left := session + inactiveWin + activeWin + inactiveWin2
	right := clock

	return padBar(left, right, p.Surface)
}

// padBar creates a fixed-width bar string with left- and right-aligned content
// separated by surface-colored padding.
func padBar(left, right string, surface theme.Color) string {
	const barWidth = 60

	// Calculate visible widths by stripping ANSI codes
	leftVisible := stripANSI(left)
	rightVisible := stripANSI(right)

	padLen := barWidth - len(leftVisible) - len(rightVisible)
	if padLen < 1 {
		padLen = 1
	}

	pad := style().
		Background(lipgloss.Color(colorStr(surface))).
		Render(strings.Repeat(" ", padLen))

	return fmt.Sprintf("%s%s%s", left, pad, right)
}

// stripANSI removes ANSI escape sequences to measure visible string length.
func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
