package bar

import (
	"fmt"
	"os"

	"github.com/donjor/zmux/internal/theme"
	"github.com/donjor/zmux/internal/tmux"
)

// Apply generates the tmux status-bar options for the given preset and palette,
// then calls runner.SetOption for each one (scope "-g" for global).
// Uses #(zmux bar-render) for dynamic content when zmux binary is available.
func Apply(runner tmux.Runner, preset Preset, palette *theme.Palette, layout ...BarLayoutConfig) error {
	zmuxBin, err := os.Executable()
	if err != nil {
		zmuxBin = ""
	}

	var lc BarLayoutConfig
	if len(layout) > 0 {
		lc = layout[0]
	}

	opts := GenerateWithLayout(preset, palette, lc, zmuxBin)
	for _, opt := range opts {
		if err := runner.SetOption("-g", opt.Key, opt.Value); err != nil {
			return fmt.Errorf("set %s: %w", opt.Key, err)
		}
	}
	return nil
}
