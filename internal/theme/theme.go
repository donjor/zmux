// Package theme handles theme parsing, semantic palette mapping,
// resolution, and embedding of bundled themes.
package theme

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/donjor/zmux/internal/config"
)

// Color represents an RGB color.
type Color struct {
	R, G, B uint8
}

// Hex returns the color as a #rrggbb hex string.
func (c Color) Hex() string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

// Theme represents a parsed terminal color scheme.
type Theme struct {
	Name       string
	Background Color
	Foreground Color
	Cursor     Color
	Selection  Color
	Palette    [16]Color // ANSI colors 0-15
}

// IsDark returns true if the theme has a dark background.
// Uses the simple heuristic: sum of RGB < 384 (midpoint of 3*256).
func (t Theme) IsDark() bool {
	return int(t.Background.R)+int(t.Background.G)+int(t.Background.B) < 384
}

// ParseHexColor parses a hex color string like "#0a0e14" or "0a0e14" into a Color.
func ParseHexColor(s string) (Color, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return Color{}, fmt.Errorf("invalid hex color %q: expected 6 hex digits", s)
	}

	r, err := strconv.ParseUint(s[0:2], 16, 8)
	if err != nil {
		return Color{}, fmt.Errorf("invalid hex color %q: %w", s, err)
	}
	g, err := strconv.ParseUint(s[2:4], 16, 8)
	if err != nil {
		return Color{}, fmt.Errorf("invalid hex color %q: %w", s, err)
	}
	b, err := strconv.ParseUint(s[4:6], 16, 8)
	if err != nil {
		return Color{}, fmt.Errorf("invalid hex color %q: %w", s, err)
	}

	return Color{R: uint8(r), G: uint8(g), B: uint8(b)}, nil
}

// ParseBytes parses a theme from raw file content in the ghostty/iterm2 format.
// Expected format: key=value or key = value lines.
// Palette entries: "palette = N=#rrggbb" or "palette = N = #rrggbb"
// Named entries: "background = #rrggbb", "foreground = #rrggbb", etc.
func ParseBytes(data []byte) (Theme, error) {
	var t Theme

	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "background":
			c, err := ParseHexColor(value)
			if err != nil {
				return Theme{}, fmt.Errorf("line %d: background: %w", i+1, err)
			}
			t.Background = c

		case "foreground":
			c, err := ParseHexColor(value)
			if err != nil {
				return Theme{}, fmt.Errorf("line %d: foreground: %w", i+1, err)
			}
			t.Foreground = c

		case "cursor-color":
			c, err := ParseHexColor(value)
			if err != nil {
				return Theme{}, fmt.Errorf("line %d: cursor-color: %w", i+1, err)
			}
			t.Cursor = c

		case "cursor-text":
			// Parsed but not stored in Theme struct; useful for future use.

		case "selection-background":
			c, err := ParseHexColor(value)
			if err != nil {
				return Theme{}, fmt.Errorf("line %d: selection-background: %w", i+1, err)
			}
			t.Selection = c

		case "selection-foreground":
			// Parsed but not stored separately; selection bg is the main selection color.

		case "palette":
			// Format: "N=#rrggbb" or "N = #rrggbb"
			palParts := strings.SplitN(value, "=", 2)
			if len(palParts) != 2 {
				return Theme{}, fmt.Errorf("line %d: invalid palette entry %q", i+1, value)
			}
			idxStr := strings.TrimSpace(palParts[0])
			colorStr := strings.TrimSpace(palParts[1])

			idx, err := strconv.Atoi(idxStr)
			if err != nil || idx < 0 || idx > 15 {
				return Theme{}, fmt.Errorf("line %d: invalid palette index %q", i+1, idxStr)
			}

			c, err := ParseHexColor(colorStr)
			if err != nil {
				return Theme{}, fmt.Errorf("line %d: palette %d: %w", i+1, idx, err)
			}
			t.Palette[idx] = c

		default:
			// Unknown keys are silently ignored for forward compatibility.
		}
	}

	return t, nil
}

// ParseFile parses a theme from a file path using the given FS.
func ParseFile(fs config.FS, path string) (Theme, error) {
	data, err := fs.ReadFile(path)
	if err != nil {
		return Theme{}, fmt.Errorf("read theme file %q: %w", path, err)
	}

	t, err := ParseBytes(data)
	if err != nil {
		return Theme{}, fmt.Errorf("parse theme file %q: %w", path, err)
	}

	return t, nil
}
