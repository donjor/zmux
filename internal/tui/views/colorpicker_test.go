package views

import (
	"math"
	"testing"

	"github.com/donjor/zmux/internal/theme"
)

func TestRGBToHSLRoundTrip(t *testing.T) {
	tests := []struct {
		r, g, b uint8
	}{
		{255, 0, 0},     // pure red
		{0, 255, 0},     // pure green
		{0, 0, 255},     // pure blue
		{255, 255, 255}, // white
		{0, 0, 0},       // black
		{128, 128, 128}, // gray
		{249, 175, 79},  // accent gold (#f9af4f)
		{11, 14, 20},    // dark bg
	}

	for _, tc := range tests {
		h, s, l := RGBToHSL(tc.r, tc.g, tc.b)
		rr, gg, bb := HSLToRGB(h, s, l)

		dr := int(rr) - int(tc.r)
		dg := int(gg) - int(tc.g)
		db := int(bb) - int(tc.b)

		if abs(dr) > 1 || abs(dg) > 1 || abs(db) > 1 {
			t.Errorf("round-trip failed for (%d,%d,%d): got (%d,%d,%d) via HSL(%.1f,%.1f,%.1f)",
				tc.r, tc.g, tc.b, rr, gg, bb, h, s, l)
		}
	}
}

func TestHSLToRGB(t *testing.T) {
	tests := []struct {
		h, s, l float64
		r, g, b uint8
	}{
		{0, 100, 50, 255, 0, 0},     // red
		{120, 100, 50, 0, 255, 0},   // green
		{240, 100, 50, 0, 0, 255},   // blue
		{0, 0, 0, 0, 0, 0},         // black
		{0, 0, 100, 255, 255, 255}, // white
	}

	for _, tc := range tests {
		r, g, b := HSLToRGB(tc.h, tc.s, tc.l)
		if r != tc.r || g != tc.g || b != tc.b {
			t.Errorf("HSLToRGB(%.0f,%.0f,%.0f) = (%d,%d,%d), want (%d,%d,%d)",
				tc.h, tc.s, tc.l, r, g, b, tc.r, tc.g, tc.b)
		}
	}
}

func TestColorPickerValue(t *testing.T) {
	c := theme.Color{R: 249, G: 175, B: 79}
	p := NewColorPicker("Accent", c)

	result := p.Value()
	dr := math.Abs(float64(result.R) - float64(c.R))
	dg := math.Abs(float64(result.G) - float64(c.G))
	db := math.Abs(float64(result.B) - float64(c.B))

	if dr > 1 || dg > 1 || db > 1 {
		t.Errorf("ColorPicker.Value() = %v, want close to %v", result, c)
	}
}

func TestColorPickerHex(t *testing.T) {
	c := theme.Color{R: 255, G: 0, B: 0}
	p := NewColorPicker("Error", c)

	hex := p.Hex()
	if hex != "#ff0000" {
		t.Errorf("ColorPicker.Hex() = %q, want #ff0000", hex)
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
