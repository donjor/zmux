package tabs

// Data loading, save, segment + section + layout helpers for the Bar tab.

import (
	tea "charm.land/bubbletea/v2"

	"github.com/donjor/zmux/internal/bar"
	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/theme"
)

// ── Data commands ──

func (t *BarTab) fetchData(reqID int64) tea.Cmd {
	fs := t.fs
	resolver := t.resolver
	return func() tea.Msg {
		cfgPath, err := config.ConfigPath(fs)
		if err != nil {
			return barDataMsg{reqID: reqID, err: err}
		}
		exists := config.ConfigExists(fs)
		cfg, err := config.Load(fs, cfgPath)
		if err != nil {
			cfg = config.DefaultConfig()
		}

		var palette *theme.Palette
		if resolver != nil && cfg.Theme != "" {
			resolved, resolveErr := resolver.Resolve(cfg.Theme)
			if resolveErr == nil {
				p := resolved.SemanticPalette()
				palette = &p
			}
		}

		return barDataMsg{
			reqID:     reqID,
			cfg:       cfg,
			cfgExists: exists,
			palette:   palette,
		}
	}
}

func (t *BarTab) saveConfig() tea.Cmd {
	fs := t.fs
	runner := t.runner
	selfBin := t.selfBin
	cfg := t.cfg
	reqID := t.reqID
	resolver := t.resolver
	return func() tea.Msg {
		cfgPath, err := config.ConfigPath(fs)
		if err != nil {
			return barConfigSaveMsg{reqID: reqID, err: err}
		}
		if err := config.Save(fs, cfgPath, cfg); err != nil {
			return barConfigSaveMsg{reqID: reqID, err: err}
		}
		// Apply bar live.
		if runner != nil && resolver != nil {
			preset, _ := bar.PresetFromString(cfg.Bar.Preset)
			resolved, resolveErr := resolver.Resolve(cfg.Theme)
			if resolveErr == nil {
				p := resolved.SemanticPalette()
				lc := bar.BarLayoutConfig{
					Layout:    cfg.Bar.Layout,
					Indicator: cfg.Bar.Indicator,
					TopBar:    cfg.Bar.TopBar,
				}
				_ = bar.Apply(runner, selfBin, preset, &p, lc)
			}
		}
		return barConfigSaveMsg{reqID: reqID}
	}
}

// ── Segment helpers ──

func (t *BarTab) toggleSegment(field string) {
	switch field {
	case "git":
		t.segments.Git = !t.segments.Git
	case "workspace":
		t.segments.Workspace = !t.segments.Workspace
	case "clock":
		t.segments.Clock = !t.segments.Clock
	case "lang":
		t.segments.Lang = !t.segments.Lang
	case "directory":
		t.segments.Directory = !t.segments.Directory
	case "process":
		t.segments.Process = !t.segments.Process
	case "group":
		t.segments.Group = !t.segments.Group
	}
}

func (t *BarTab) segmentEnabled(field string) bool {
	switch field {
	case "git":
		return t.segments.Git
	case "workspace":
		return t.segments.Workspace
	case "clock":
		return t.segments.Clock
	case "lang":
		return t.segments.Lang
	case "directory":
		return t.segments.Directory
	case "process":
		return t.segments.Process
	case "group":
		return t.segments.Group
	}
	return false
}

// ── Section helpers ──

func (t *BarTab) currentSection() barSection {
	P := len(t.presets)
	L := len(barLayoutOptions)
	if t.cursor < P {
		return barPresets
	}
	if t.cursor < P+L {
		return barLayout
	}
	return barSegments
}

// ── Layout helpers ──

// layoutHint is the one-line tradeoff note shown under the Layout selector, so
// the cost/benefit of each layout is legible at the point of choice (plan 024).
func layoutHint(layout string) string {
	switch layout {
	case "split":
		return "split · two rows, top bar separated from the status row"
	default: // two-line
		return "two-line · stable two rows, no reflow (default)"
	}
}

func (t *BarTab) layoutValue(field string) string {
	// Defaults match config.DefaultConfig().
	switch field {
	case "layout":
		if t.layout == "" {
			return "two-line"
		}
		return t.layout
	case "top_bar":
		if t.topBar == "" {
			return "tabs"
		}
		return t.topBar
	case "indicator":
		if t.indicator == "" {
			return "dots"
		}
		return t.indicator
	}
	return ""
}

func (t *BarTab) cycleLayoutValue(field string, delta int) {
	for _, opt := range barLayoutOptions {
		if opt.Field != field {
			continue
		}
		current := t.layoutValue(field)
		idx := 0
		for i, v := range opt.Options {
			if v == current {
				idx = i
				break
			}
		}
		idx = (idx + delta + len(opt.Options)) % len(opt.Options)
		value := opt.Options[idx]
		switch field {
		case "layout":
			t.layout = value
			t.cfg.Bar.Layout = value
		case "top_bar":
			t.topBar = value
			t.cfg.Bar.TopBar = value
		case "indicator":
			t.indicator = value
			t.cfg.Bar.Indicator = value
		}
		return
	}
}
