package views

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/donjor/zmux/internal/theme"
)

// PickerMode determines how the color picker accepts input.
type PickerMode int

const (
	PickerSlider PickerMode = iota
	PickerHex
)

// ColorPicker is a reusable HSL color picker bubbletea component.
type ColorPicker struct {
	H, S, L  float64 // H: 0-360, S: 0-100, L: 0-100
	HexInput textinput.Model
	Mode     PickerMode
	Slot     string // semantic label being edited (e.g. "Accent")
	channel  int    // 0=H, 1=S, 2=L
	width    int
}

// NewColorPicker creates a color picker initialized from an RGB color.
func NewColorPicker(slot string, c theme.Color) ColorPicker {
	h, s, l := RGBToHSL(c.R, c.G, c.B)

	hexInput := textinput.New()
	hexInput.Placeholder = "#rrggbb"
	hexInput.CharLimit = 7

	return ColorPicker{
		H:        h,
		S:        s,
		L:        l,
		HexInput: hexInput,
		Slot:     slot,
		width:    30,
	}
}

// Value returns the current color as an RGB theme.Color.
func (p *ColorPicker) Value() theme.Color {
	r, g, b := HSLToRGB(p.H, p.S, p.L)
	return theme.Color{R: r, G: g, B: b}
}

// Hex returns the current color as a hex string.
func (p *ColorPicker) Hex() string {
	return p.Value().Hex()
}

// SetColor updates the picker from an RGB color.
func (p *ColorPicker) SetColor(c theme.Color) {
	p.H, p.S, p.L = RGBToHSL(c.R, c.G, c.B)
}

// Update handles key events for the color picker.
func (p *ColorPicker) Update(msg tea.Msg) (*ColorPicker, tea.Cmd) {
	if p.Mode == PickerHex {
		return p.updateHex(msg)
	}
	return p.updateSlider(msg)
}

func (p *ColorPicker) updateSlider(msg tea.Msg) (*ColorPicker, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}

	step := 1.0
	if keyMsg.String() == "shift+left" || keyMsg.String() == "shift+right" {
		step = 10.0
	}

	switch {
	case key.Matches(keyMsg, key.NewBinding(key.WithKeys("left", "shift+left"))):
		p.adjust(-step)
	case key.Matches(keyMsg, key.NewBinding(key.WithKeys("right", "shift+right"))):
		p.adjust(step)
	case key.Matches(keyMsg, key.NewBinding(key.WithKeys("up", "k"))):
		p.channel = (p.channel - 1 + 3) % 3
	case key.Matches(keyMsg, key.NewBinding(key.WithKeys("down", "j"))):
		p.channel = (p.channel + 1) % 3
	case key.Matches(keyMsg, key.NewBinding(key.WithKeys("#"))):
		p.Mode = PickerHex
		p.HexInput.SetValue(p.Hex())
		p.HexInput.Focus()
		p.HexInput.CursorEnd()
		return p, textinput.Blink
	}

	return p, nil
}

func (p *ColorPicker) updateHex(msg tea.Msg) (*ColorPicker, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.Type {
		case tea.KeyEnter:
			hex := strings.TrimSpace(p.HexInput.Value())
			c, err := theme.ParseHexColor(hex)
			if err == nil {
				p.SetColor(c)
			}
			p.Mode = PickerSlider
			p.HexInput.Blur()
			return p, nil
		case tea.KeyEscape:
			p.Mode = PickerSlider
			p.HexInput.Blur()
			return p, nil
		}
	}

	var cmd tea.Cmd
	p.HexInput, cmd = p.HexInput.Update(msg)
	return p, cmd
}

func (p *ColorPicker) adjust(delta float64) {
	switch p.channel {
	case 0:
		p.H = math.Mod(p.H+delta+360, 360)
	case 1:
		p.S = clamp(p.S+delta, 0, 100)
	case 2:
		p.L = clamp(p.L+delta, 0, 100)
	}
}

// View renders the color picker.
func (p *ColorPicker) View() string {
	var b strings.Builder

	color := p.Value()

	// Title line: slot name + hex value.
	b.WriteString(fmt.Sprintf("  %s (%s)\n", p.Slot, color.Hex()))

	if p.Mode == PickerHex {
		b.WriteString("  Hex: " + p.HexInput.View() + "\n")
		return b.String()
	}

	// Slider bars.
	barWidth := p.width
	if barWidth < 10 {
		barWidth = 20
	}

	channels := []struct {
		label  string
		value  float64
		maxVal float64
		unit   string
	}{
		{"H", p.H, 360, "\u00b0"},
		{"S", p.S, 100, "%"},
		{"L", p.L, 100, "%"},
	}

	for i, ch := range channels {
		cursor := "  "
		if i == p.channel {
			cursor = "\u25b8 "
		}

		// Render gradient bar.
		var bar strings.Builder
		for j := 0; j < barWidth; j++ {
			frac := float64(j) / float64(barWidth)
			var cr, cg, cb uint8
			switch i {
			case 0: // Hue gradient.
				cr, cg, cb = HSLToRGB(frac*360, p.S, p.L)
			case 1: // Saturation gradient.
				cr, cg, cb = HSLToRGB(p.H, frac*100, p.L)
			case 2: // Lightness gradient.
				cr, cg, cb = HSLToRGB(p.H, p.S, frac*100)
			}
			c := termenv.RGBColor(fmt.Sprintf("#%02x%02x%02x", cr, cg, cb))
			bar.WriteString(termenv.String("\u2588").Foreground(c).String())
		}

		valueStr := fmt.Sprintf(" %3.0f%s", ch.value, ch.unit)
		b.WriteString(fmt.Sprintf("  %s%s %s%s\n", cursor, ch.label, bar.String(), valueStr))
	}

	// Preview swatch.
	hexColor := color.Hex()
	swatchStyle := lipgloss.NewStyle().
		Background(lipgloss.Color(hexColor)).
		Foreground(lipgloss.Color(hexColor))
	swatch := swatchStyle.Render("      ")
	b.WriteString(fmt.Sprintf("  [%s] preview\n", swatch))

	return b.String()
}

// Resize updates the picker width.
func (p *ColorPicker) Resize(width int) {
	p.width = width
	if p.width > 40 {
		p.width = 40
	}
	if p.width < 10 {
		p.width = 10
	}
}

// --- HSL <-> RGB conversion ---

// RGBToHSL converts RGB (0-255 each) to HSL (H: 0-360, S: 0-100, L: 0-100).
func RGBToHSL(r, g, b uint8) (h, s, l float64) {
	rf := float64(r) / 255.0
	gf := float64(g) / 255.0
	bf := float64(b) / 255.0

	maxC := math.Max(rf, math.Max(gf, bf))
	minC := math.Min(rf, math.Min(gf, bf))
	delta := maxC - minC

	l = (maxC + minC) / 2.0

	if delta == 0 {
		return 0, 0, l * 100
	}

	if l < 0.5 {
		s = delta / (maxC + minC)
	} else {
		s = delta / (2.0 - maxC - minC)
	}

	switch maxC {
	case rf:
		h = (gf - bf) / delta
		if gf < bf {
			h += 6
		}
	case gf:
		h = (bf-rf)/delta + 2
	case bf:
		h = (rf-gf)/delta + 4
	}
	h *= 60

	return h, s * 100, l * 100
}

// HSLToRGB converts HSL (H: 0-360, S: 0-100, L: 0-100) to RGB (0-255 each).
func HSLToRGB(h, s, l float64) (uint8, uint8, uint8) {
	s /= 100
	l /= 100

	if s == 0 {
		v := uint8(math.Round(l * 255))
		return v, v, v
	}

	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}

	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q

	hNorm := h / 360.0

	r := hueToRGB(p, q, hNorm+1.0/3.0)
	g := hueToRGB(p, q, hNorm)
	b := hueToRGB(p, q, hNorm-1.0/3.0)

	return uint8(math.Round(r * 255)), uint8(math.Round(g * 255)), uint8(math.Round(b * 255))
}

func hueToRGB(p, q, t float64) float64 {
	if t < 0 {
		t++
	}
	if t > 1 {
		t--
	}
	switch {
	case t < 1.0/6.0:
		return p + (q-p)*6*t
	case t < 1.0/2.0:
		return q
	case t < 2.0/3.0:
		return p + (q-p)*(2.0/3.0-t)*6
	}
	return p
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
