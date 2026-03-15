package bar

import (
	"fmt"

	"github.com/donjor/zmux/internal/theme"
	"github.com/donjor/zmux/internal/tmux"
)

// Apply generates the tmux status-bar options for the given preset and palette,
// then calls runner.SetOption for each one (scope "-g" for global).
func Apply(runner tmux.Runner, preset Preset, palette *theme.Palette) error {
	opts := Generate(preset, palette)
	for _, opt := range opts {
		if err := runner.SetOption("-g", opt.Key, opt.Value); err != nil {
			return fmt.Errorf("set %s: %w", opt.Key, err)
		}
	}
	return nil
}
