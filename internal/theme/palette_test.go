package theme

import "testing"

func TestSemanticPalette_AyuDark(t *testing.T) {
	th, err := ParseBytes([]byte(ayuDarkTheme))
	if err != nil {
		t.Fatalf("ParseBytes failed: %v", err)
	}

	p := th.SemanticPalette()

	// BG and FG from theme
	assertColor(t, "BG", p.BG, th.Background)
	assertColor(t, "FG", p.FG, th.Foreground)

	// Semantic roles mapped from ANSI indices per vision doc
	assertColor(t, "Surface (ANSI 0)", p.Surface, th.Palette[0])
	assertColor(t, "Error (ANSI 1)", p.Error, th.Palette[1])
	assertColor(t, "Success (ANSI 2)", p.Success, th.Palette[2])
	assertColor(t, "Accent (ANSI 3)", p.Accent, th.Palette[3])
	assertColor(t, "Info (ANSI 4)", p.Info, th.Palette[4])
	assertColor(t, "Special (ANSI 5)", p.Special, th.Palette[5])
	assertColor(t, "Meta (ANSI 6)", p.Meta, th.Palette[6])
	assertColor(t, "Muted (ANSI 7)", p.Muted, th.Palette[7])
	assertColor(t, "Dim (ANSI 8)", p.Dim, th.Palette[8])

	// Highlight from cursor color
	assertColor(t, "Highlight", p.Highlight, th.Cursor)
}

func TestSemanticPalette_DerivedColors_DarkTheme(t *testing.T) {
	th := Theme{
		Background: Color{R: 10, G: 14, B: 20}, // dark
		Foreground: Color{R: 200, G: 200, B: 200},
		Cursor:     Color{R: 230, G: 180, B: 80},
	}

	p := th.SemanticPalette()

	// BGDim should be lighter than BG for dark themes (+15)
	assertColor(t, "BGDim", p.BGDim, Color{R: 25, G: 29, B: 35})

	// BGPrefix should be even lighter (+25)
	assertColor(t, "BGPrefix", p.BGPrefix, Color{R: 35, G: 39, B: 45})
}

func TestSemanticPalette_DerivedColors_LightTheme(t *testing.T) {
	th := Theme{
		Background: Color{R: 240, G: 240, B: 240}, // light
		Foreground: Color{R: 30, G: 30, B: 30},
		Cursor:     Color{R: 0, G: 0, B: 0},
	}

	p := th.SemanticPalette()

	// BGDim should be darker than BG for light themes (-15)
	assertColor(t, "BGDim", p.BGDim, Color{R: 225, G: 225, B: 225})

	// BGPrefix should be even darker (-25)
	assertColor(t, "BGPrefix", p.BGPrefix, Color{R: 215, G: 215, B: 215})
}

func TestShiftColor_Clamping(t *testing.T) {
	// Test upper clamp
	c := shiftColor(Color{R: 250, G: 250, B: 250}, 20)
	assertColor(t, "upper clamp", c, Color{R: 255, G: 255, B: 255})

	// Test lower clamp
	c = shiftColor(Color{R: 5, G: 5, B: 5}, -20)
	assertColor(t, "lower clamp", c, Color{R: 0, G: 0, B: 0})
}

func TestClampUint8(t *testing.T) {
	tests := []struct {
		input int
		want  uint8
	}{
		{-10, 0},
		{0, 0},
		{128, 128},
		{255, 255},
		{300, 255},
	}

	for _, tt := range tests {
		got := clampUint8(tt.input)
		if got != tt.want {
			t.Errorf("clampUint8(%d) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
