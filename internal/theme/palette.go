package theme

// Palette maps ANSI palette indices to semantic UI roles.
// These names describe purpose, not color — a theme's "Error" may be
// red, pink, or orange depending on the palette.
type Palette struct {
	BG        Color // background
	FG        Color // foreground / primary text
	Surface   Color // cards, elevated panels (ANSI 0 / black)
	Error     Color // destructive actions, errors (ANSI 1 / red)
	Success   Color // confirmations, active status (ANSI 2 / green)
	Accent    Color // primary accent, branding (ANSI 3 / yellow)
	Info      Color // links, secondary accent (ANSI 4 / blue)
	Special   Color // unique items, templates (ANSI 5 / magenta)
	Meta      Color // tags, metadata (ANSI 6 / cyan)
	Muted     Color // secondary text, labels (ANSI 7 / white)
	Dim       Color // borders, separators, faint (ANSI 8 / bright black)
	Highlight Color // focus indicators, cursor (cursor-color)
	BGDim     Color // slightly lighter/darker than BG (derived)
	BGPrefix  Color // prefix-active background indicator (derived)
}

// SemanticPalette maps ANSI palette indices to semantic roles per the vision doc.
func (t Theme) SemanticPalette() Palette {
	p := Palette{
		BG:        t.Background,
		FG:        t.Foreground,
		Surface:   t.Palette[0],  // black
		Error:     t.Palette[1],  // red
		Success:   t.Palette[2],  // green
		Accent:    t.Palette[3],  // yellow
		Info:      t.Palette[4],  // blue
		Special:   t.Palette[5],  // magenta
		Meta:      t.Palette[6],  // cyan
		Muted:     t.Palette[7],  // white
		Dim:       t.Palette[8],  // bright black
		Highlight: t.Cursor,
	}

	p.BGDim = deriveBGDim(t.Background, t.IsDark())
	p.BGPrefix = deriveBGPrefix(t.Background, t.IsDark())

	return p
}

// deriveBGDim creates a color slightly shifted from the background.
// For dark themes: slightly lighter. For light themes: slightly darker.
func deriveBGDim(bg Color, isDark bool) Color {
	if isDark {
		return shiftColor(bg, 15)
	}
	return shiftColor(bg, -15)
}

// deriveBGPrefix creates a prefix-active background indicator.
// More pronounced shift than BGDim to be noticeable.
func deriveBGPrefix(bg Color, isDark bool) Color {
	if isDark {
		return shiftColor(bg, 25)
	}
	return shiftColor(bg, -25)
}

// shiftColor shifts each RGB component by delta, clamping to [0, 255].
func shiftColor(c Color, delta int) Color {
	return Color{
		R: clampUint8(int(c.R) + delta),
		G: clampUint8(int(c.G) + delta),
		B: clampUint8(int(c.B) + delta),
	}
}

// clampUint8 clamps an int to the [0, 255] range and returns it as uint8.
func clampUint8(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}
