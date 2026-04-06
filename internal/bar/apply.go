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
func Apply(runner tmux.Runner, preset Preset, palette *theme.Palette) error {
	zmuxBin, err := os.Executable()
	if err != nil {
		zmuxBin = ""
	}

	opts := Generate(preset, palette, zmuxBin)
	for _, opt := range opts {
		if err := runner.SetOption("-g", opt.Key, opt.Value); err != nil {
			return fmt.Errorf("set %s: %w", opt.Key, err)
		}
	}
	return nil
}
