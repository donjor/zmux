package bar

import (
	"fmt"

	"github.com/donjor/zmux/internal/theme"
)

// TmuxOption is a key-value pair for a tmux set-option call.
type TmuxOption struct {
	Key, Value string
}

// Generate produces the tmux status-line options for a given preset and palette.
// The returned slice includes both shared options (borders, messages, mode) and
// preset-specific options (status-left, status-right, window formats, etc.).
func Generate(preset Preset, palette *theme.Palette) []TmuxOption {
	opts := sharedOptions(palette)

	switch preset {
	case Minimal:
		opts = append(opts, minimalOptions(palette)...)
	case Powerline:
		opts = append(opts, powerlineOptions(palette)...)
	case Blocks:
		opts = append(opts, blocksOptions(palette)...)
	default:
		opts = append(opts, defaultOptions(palette)...)
	}

	return opts
}

// sharedOptions returns options common to all presets.
func sharedOptions(p *theme.Palette) []TmuxOption {
	return []TmuxOption{
		{"pane-border-style", fmt.Sprintf("fg=%s", p.Dim.Hex())},
		{"pane-active-border-style", fmt.Sprintf("fg=%s", p.Accent.Hex())},
		{"message-style", fmt.Sprintf("bg=%s,fg=%s", p.Surface.Hex(), p.FG.Hex())},
		{"message-command-style", fmt.Sprintf("bg=%s,fg=%s", p.Surface.Hex(), p.FG.Hex())},
		{"mode-style", fmt.Sprintf("bg=%s,fg=%s", p.Info.Hex(), p.BG.Hex())},
		{"clock-mode-colour", p.Accent.Hex()},
		{"window-active-style", fmt.Sprintf("#{?client_prefix,bg=%s,bg=default}", p.BGPrefix.Hex())},
		{"window-style", "bg=default"},
	}
}

// defaultOptions: Session pill (ACCENT bg, INFO on prefix), prefix hints, clock.
func defaultOptions(p *theme.Palette) []TmuxOption {
	statusLeft := fmt.Sprintf(
		"#{?client_prefix,#[bg=%s],#[bg=%s]}#[fg=%s,bold] #S #{?client_prefix,#[fg=%s],#[fg=%s]}#[bg=%s] ",
		p.Info.Hex(), p.Accent.Hex(), p.BG.Hex(),
		p.Info.Hex(), p.Accent.Hex(), p.Surface.Hex(),
	)

	// Prefix hints: shown when prefix is active (client_prefix).
	// Format: key in accent, description in dim.
	hi := p.Info.Hex()
	dm := p.Dim.Hex()
	prefixHint := fmt.Sprintf(
		"#[fg=%s]spc#[fg=%s]dash #[fg=%s]d#[fg=%s]etach #[fg=%s]c#[fg=%s]tab #[fg=%s]s#[fg=%s]witch #[fg=%s]v#[fg=%s]im #[fg=%s]?#[fg=%s]help ",
		hi, dm, hi, dm, hi, dm, hi, dm, hi, dm, hi, dm,
	)

	// Normal: show prefix key + time. Prefix active: show hints.
	statusRight := fmt.Sprintf(
		"#{?client_prefix,%s,#[fg=%s]ctrl+space #[fg=%s]%%I:%%M %%p }",
		prefixHint, dm, p.Muted.Hex(),
	)

	windowFmt := fmt.Sprintf("#[fg=%s] #I #W ", p.Dim.Hex())

	windowCurrentFmt := fmt.Sprintf(
		"#{?client_prefix,#[fg=%s],#[fg=%s]}#[bold] #I #W #[fg=%s,nobold]",
		p.Info.Hex(), p.Accent.Hex(), p.Muted.Hex(),
	)

	windowSep := fmt.Sprintf("#[fg=%s]\u2502", p.Dim.Hex()) // │

	return []TmuxOption{
		{"status-style", fmt.Sprintf("bg=%s,fg=%s", p.Surface.Hex(), p.Muted.Hex())},
		{"status-left", statusLeft},
		{"status-right", statusRight},
		{"window-status-format", windowFmt},
		{"window-status-current-format", windowCurrentFmt},
		{"window-status-separator", windowSep},
		{"status-left-length", "40"},
		{"status-right-length", "120"},
	}
}

// minimalOptions: Session name + pipe, minimal tabs, optional time.
func minimalOptions(p *theme.Palette) []TmuxOption {
	statusLeft := fmt.Sprintf(
		"#{?client_prefix,#[fg=%s],#[fg=%s]}#[bold] #S #[fg=%s,nobold]\u2502 ",
		p.Info.Hex(), p.Accent.Hex(), p.Dim.Hex(),
	)

	statusRight := fmt.Sprintf("#[fg=%s]%%H:%%M ", p.Dim.Hex())

	windowFmt := fmt.Sprintf("#[fg=%s] #W ", p.Dim.Hex())

	windowCurrentFmt := fmt.Sprintf("#[fg=%s,bold] #W ", p.FG.Hex())

	return []TmuxOption{
		{"status-style", fmt.Sprintf("bg=%s,fg=%s", p.Surface.Hex(), p.Muted.Hex())},
		{"status-left", statusLeft},
		{"status-right", statusRight},
		{"window-status-format", windowFmt},
		{"window-status-current-format", windowCurrentFmt},
		{"window-status-separator", " "},
		{"status-left-length", "40"},
		{"status-right-length", "40"},
	}
}

// powerlineOptions: Angled separators, filled segments.
func powerlineOptions(p *theme.Palette) []TmuxOption {
	statusLeft := fmt.Sprintf(
		"#{?client_prefix,#[bg=%s],#[bg=%s]}#[fg=%s,bold] #S #{?client_prefix,#[fg=%s],#[fg=%s]}#[bg=%s]\ue0b0 ",
		p.Info.Hex(), p.Accent.Hex(), p.BG.Hex(),
		p.Info.Hex(), p.Accent.Hex(), p.Surface.Hex(),
	)

	statusRight := fmt.Sprintf(
		"#[fg=%s]\ue0b2#[bg=%s,fg=%s] %%H:%%M #[fg=%s]\ue0b2#[bg=%s,fg=%s,bold] %%b %%d ",
		p.Dim.Hex(),
		p.Dim.Hex(), p.Muted.Hex(),
		p.Accent.Hex(),
		p.Accent.Hex(), p.BG.Hex(),
	)

	windowFmt := fmt.Sprintf(
		"#[fg=%s,bg=%s]\ue0b0#[fg=%s] #I #W #[fg=%s,bg=%s]\ue0b0",
		p.Surface.Hex(), p.Dim.Hex(), p.Muted.Hex(),
		p.Dim.Hex(), p.Surface.Hex(),
	)

	windowCurrentFmt := fmt.Sprintf(
		"#{?client_prefix,#[fg=%s]#[bg=%s]\ue0b0#[fg=%s]#[bold] #I #W #[fg=%s]#[bg=%s]\ue0b0,#[fg=%s]#[bg=%s]\ue0b0#[fg=%s]#[bold] #I #W #[fg=%s]#[bg=%s]\ue0b0}",
		p.Surface.Hex(), p.Info.Hex(), p.BG.Hex(), p.Info.Hex(), p.Surface.Hex(),
		p.Surface.Hex(), p.Accent.Hex(), p.BG.Hex(), p.Accent.Hex(), p.Surface.Hex(),
	)

	return []TmuxOption{
		{"status-style", fmt.Sprintf("bg=%s,fg=%s", p.Surface.Hex(), p.Muted.Hex())},
		{"status-left", statusLeft},
		{"status-right", statusRight},
		{"window-status-format", windowFmt},
		{"window-status-current-format", windowCurrentFmt},
		{"window-status-separator", ""},
		{"status-left-length", "40"},
		{"status-right-length", "60"},
	}
}

// blocksOptions: Square bracket segments, monospace aesthetic.
func blocksOptions(p *theme.Palette) []TmuxOption {
	statusLeft := fmt.Sprintf(
		"#{?client_prefix,#[fg=%s],#[fg=%s]}#[bold] [#S] #[fg=%s,nobold]",
		p.Info.Hex(), p.Accent.Hex(), p.Dim.Hex(),
	)

	statusRight := fmt.Sprintf("#[fg=%s][%%H:%%M] ", p.Dim.Hex())

	windowFmt := fmt.Sprintf("#[fg=%s] [#I:#W] ", p.Dim.Hex())

	windowCurrentFmt := fmt.Sprintf(
		"#{?client_prefix,#[fg=%s],#[fg=%s]}#[bold] [#I:#W] #[nobold]",
		p.Info.Hex(), p.Accent.Hex(),
	)

	return []TmuxOption{
		{"status-style", fmt.Sprintf("bg=%s,fg=%s", p.Surface.Hex(), p.Muted.Hex())},
		{"status-left", statusLeft},
		{"status-right", statusRight},
		{"window-status-format", windowFmt},
		{"window-status-current-format", windowCurrentFmt},
		{"window-status-separator", ""},
		{"status-left-length", "40"},
		{"status-right-length", "40"},
	}
}
