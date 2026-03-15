package bar

import "fmt"

// Preset identifies a built-in status bar layout.
type Preset int

const (
	Default   Preset = iota // Session pill, prefix hints, clock
	Minimal                 // Session name + pipe, minimal tabs
	Powerline               // Angled separators, filled segments
	Blocks                  // Square bracket segments
)

var presetNames = [...]string{
	Default:   "default",
	Minimal:   "minimal",
	Powerline: "powerline",
	Blocks:    "blocks",
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
		return 0, fmt.Errorf("unknown bar preset %q (valid: default, minimal, powerline, blocks)", s)
	}
	return p, nil
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
	return []Preset{Default, Minimal, Powerline, Blocks}
}
