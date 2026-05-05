package bar

import "fmt"

// Preset identifies a built-in status bar layout.
type Preset int

const (
	Default    Preset = iota // Session pill, prefix hints, clock
	Minimal                  // Session name + pipe, minimal tabs
	Powerline                // Angled separators, filled segments
	Blocks                   // Square bracket segments
	Rounded                  // Pill-shaped segments with rounded ends
	Hacker                   // Matrix-inspired, monospace, dense info
	Zen                      // Ultra-minimal, just the essentials
	Starship                 // Inspired by starship prompt — colorful segments
	Rpowerline               // Rounded powerline — angled fills with rounded caps
)

var presetNames = [...]string{
	Default:    "default",
	Minimal:    "minimal",
	Powerline:  "powerline",
	Blocks:     "blocks",
	Rounded:    "rounded",
	Hacker:     "hacker",
	Zen:        "zen",
	Starship:   "starship",
	Rpowerline: "rpowerline",
}

var presetsByName map[string]Preset

func init() {
	presetsByName = make(map[string]Preset, len(presetNames))
	for p, name := range presetNames {
		presetsByName[name] = Preset(p)
	}
}

// PresetFromString parses a preset name (case-sensitive).
func PresetFromString(s string) (Preset, error) {
	p, ok := presetsByName[s]
	if !ok {
		names := make([]string, len(presetNames))
		copy(names, presetNames[:])
		return 0, fmt.Errorf("unknown bar preset %q (valid: %s)", s, join(names))
	}
	return p, nil
}

func join(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}

// String returns the preset's canonical name.
func (p Preset) String() string {
	if int(p) < len(presetNames) {
		return presetNames[p]
	}
	return fmt.Sprintf("Preset(%d)", p)
}

// AllPresets returns all built-in presets in definition order.
func AllPresets() []Preset {
	all := make([]Preset, len(presetNames))
	for i := range presetNames {
		all[i] = Preset(i)
	}
	return all
}
