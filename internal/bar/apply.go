package bar

import (
	"fmt"

	"github.com/donjor/zmux/internal/theme"
	"github.com/donjor/zmux/internal/tmux"
)

// Apply generates the tmux status-bar options for the given preset and palette,
// then calls runner.SetOption for each one (scope "-g" for global).
//
// selfBin is the binary embedded in #(<bin> bar-render) dynamic content. Pass
// config.SelfBin(profile) — never os.Executable() directly: a symlinked or
// misresolved binary makes the edge (zzmux) profile bake the live zmux binary
// into its bar, which then resolves the wrong workspace store and renders an
// empty workspace pill. SelfBin also falls back to the profile name rather
// than "" (a "" here produces a broken `#( bar-render …)` status string).
func Apply(runner tmux.Runner, selfBin string, preset Preset, palette *theme.Palette, layout BarLayoutConfig) error {
	opts := GenerateWithLayout(preset, palette, layout, selfBin)
	for _, opt := range opts {
		if err := runner.SetOption("-g", opt.Key, opt.Value); err != nil {
			return fmt.Errorf("set %s: %w", opt.Key, err)
		}
	}
	return nil
}
