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
// If zmuxBin is provided, uses #(zmux bar-render) for dynamic left/right content
// (git branch, workspace, prefix hints). Otherwise falls back to static tmux formats.
func Generate(preset Preset, palette *theme.Palette, zmuxBin ...string) []TmuxOption {
	opts := sharedOptions(palette)

	bin := ""
	if len(zmuxBin) > 0 {
		bin = zmuxBin[0]
	}

	// If zmux binary available, use dynamic rendering for left/right.
	if bin != "" {
		opts = append(opts, dynamicOptions(palette, bin, preset)...)
	} else {
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
	}

	return opts
}

// dynamicOptions uses #(zmux bar-render) for left/right, keeping window
// format strings as tmux-native (they're per-window and change with each tab).
func dynamicOptions(p *theme.Palette, zmuxBin string, preset Preset) []TmuxOption {
	// Status-left: native tmux formats (instant, per-client, per-session).
	// Uses simple #{?client_prefix,A,B} — no nested #[] inside conditionals.
	// Workspace via session-level ZMUX_WORKSPACE env var.
	statusLeft := nativeStatusLeft(p, preset)

	// Status-right: #() for git/lang. Embedding #{pane_current_path} in the
	// command makes tmux cache per-directory — so each window/pane gets its
	// own git status instead of sharing one stale result across the session.
	statusRight := fmt.Sprintf("#(%s bar-render right --dir '#{pane_current_path}')", zmuxBin)

	// Window formats stay tmux-native — they change per-window and are cheap.
	var windowFmt, windowCurrentFmt, windowSep string

	switch preset {
	case Minimal:
		windowFmt = fmt.Sprintf("#[fg=%s] #W ", p.Dim.Hex())
		windowCurrentFmt = fmt.Sprintf("#[fg=%s,bold] #W ", p.FG.Hex())
		windowSep = ""
	case Powerline:
		// Two-tone: [index]▸[name] with sharp powerline arrows.
		windowFmt = fmt.Sprintf(
			"#[fg=%s,bg=%s]\ue0b0#[bg=%s,fg=%s] #I "+
				"#[fg=%s,bg=%s]\ue0b0"+
				"#[bg=%s,fg=%s] #W "+
				"#[fg=%s,bg=default]\ue0b0",
			p.BG.Hex(), p.Dim.Hex(), p.Dim.Hex(), p.Surface.Hex(),
			p.Dim.Hex(), p.Surface.Hex(),
			p.Surface.Hex(), p.Muted.Hex(),
			p.Surface.Hex(),
		)
		windowCurrentFmt = fmt.Sprintf(
			"#[fg=%s,bg=%s]\ue0b0#[bg=%s,fg=%s,bold] #I "+
				"#[fg=%s,bg=%s]\ue0b0"+
				"#[bg=%s,fg=%s,bold] #W "+
				"#[nobold,fg=%s,bg=default]\ue0b0",
			p.BG.Hex(), p.Accent.Hex(), p.Accent.Hex(), p.BG.Hex(),
			p.Accent.Hex(), p.Surface.Hex(),
			p.Surface.Hex(), p.FG.Hex(),
			p.Surface.Hex(),
		)
		windowSep = ""
	case Blocks:
		windowFmt = fmt.Sprintf(
			"#{?client_prefix,#[fg=%s],#[fg=%s]}#[bold] [#I:#W] #[nobold]",
			p.Info.Hex(), p.Dim.Hex(),
		)
		windowCurrentFmt = fmt.Sprintf(
			"#{?client_prefix,#[fg=%s],#[fg=%s]}#[bold] [#I:#W] #[nobold]",
			p.Info.Hex(), p.Accent.Hex(),
		)
		windowSep = " "
	case Rounded:
		// Pill-shaped window tabs with rounded caps.
		windowFmt = fmt.Sprintf(
			"#[fg=%s]\ue0b6#[bg=%s,fg=%s] #I #W #[fg=%s,bg=default]\ue0b4",
			p.Surface.Hex(), p.Surface.Hex(), p.Dim.Hex(), p.Surface.Hex(),
		)
		windowCurrentFmt = fmt.Sprintf(
			"#[fg=%s]\ue0b6#[bg=%s,fg=%s,bold] #I #W #[fg=%s,bg=default]\ue0b4",
			p.Accent.Hex(), p.Accent.Hex(), p.BG.Hex(), p.Accent.Hex(),
		)
		windowSep = " "
	case Hacker:
		// Dense monospace, index:name.
		g := p.Success.Hex()
		d := p.Dim.Hex()
		windowFmt = fmt.Sprintf("#[fg=%s]#I:#W", d)
		windowCurrentFmt = fmt.Sprintf("#[fg=%s,bold]#I:#W", g)
		windowSep = fmt.Sprintf("#[fg=%s]|", d)
	case Zen:
		// Just the name, barely visible.
		windowFmt = fmt.Sprintf("#[fg=%s]#W", p.Dim.Hex())
		windowCurrentFmt = fmt.Sprintf("#[fg=%s]#W", p.Muted.Hex())
		windowSep = fmt.Sprintf("#[fg=%s] · ", p.Dim.Hex())
	case Starship:
		// Colorful tabs with chevrons.
		windowFmt = fmt.Sprintf("#[fg=%s] #I #W ", p.Dim.Hex())
		windowCurrentFmt = fmt.Sprintf(
			"#[fg=%s,bold] #I #W #[fg=%s]❯#[fg=default,nobold]",
			p.Accent.Hex(), p.Accent.Hex(),
		)
		windowSep = ""
	case Rpowerline:
		// Catppuccin-inspired two-tone pills: [accent index]▸[surface name]
		// Rounded caps on outer edges, powerline arrow between sections.
		windowFmt = fmt.Sprintf(
			"#[fg=%s]\ue0b6#[bg=%s,fg=%s]#I\u2009"+
				"#[fg=%s,bg=%s]\ue0b0"+
				"#[bg=%s,fg=%s] #W "+
				"#[fg=%s,bg=default]\ue0b4",
			p.Dim.Hex(), p.Dim.Hex(), p.Surface.Hex(),
			p.Dim.Hex(), p.Surface.Hex(),
			p.Surface.Hex(), p.Muted.Hex(),
			p.Surface.Hex(),
		)
		windowCurrentFmt = fmt.Sprintf(
			"#[fg=%s]\ue0b6#[bg=%s,fg=%s,bold]#I\u2009"+
				"#[fg=%s,bg=%s]\ue0b0"+
				"#[bg=%s,fg=%s,bold] #W "+
				"#[nobold,fg=%s,bg=default]\ue0b4",
			p.Accent.Hex(), p.Accent.Hex(), p.BG.Hex(),
			p.Accent.Hex(), p.Surface.Hex(),
			p.Surface.Hex(), p.FG.Hex(),
			p.Surface.Hex(),
		)
		windowSep = ""
	default: // Default
		windowFmt = fmt.Sprintf("#[fg=%s] #I #W ", p.Dim.Hex())
		windowCurrentFmt = fmt.Sprintf(
			"#{?client_prefix,#[fg=%s],#[fg=%s]}#[bold] #I #W #[fg=%s,nobold]",
			p.Info.Hex(), p.Accent.Hex(), p.Muted.Hex(),
		)
		windowSep = fmt.Sprintf("#[fg=%s]\u2502", p.Dim.Hex())
	}

	// Bar bg: rpowerline/powerline use BG (darker) so two-tone tab pills
	// are visible against the bar. Others use Surface.
	barBG := p.Surface.Hex()
	if preset == Rpowerline || preset == Powerline {
		barBG = p.BG.Hex()
	}

	return []TmuxOption{
		{"status-style", fmt.Sprintf("bg=%s,fg=%s", barBG, p.Muted.Hex())},
		{"status-left", statusLeft},
		{"status-right", statusRight},
		{"window-status-format", windowFmt},
		{"window-status-current-format", windowCurrentFmt},
		{"window-status-separator", windowSep},
		{"status-left-length", "100"},
		{"status-right-length", "80"},
	}
}

// nativeStatusLeft builds status-left using tmux-native format variables.
// Renders instantly per-client — no #() delay.
// RULE: keep #{?client_prefix,A,B} simple — A and B are complete #[] blocks,
// never nest #{?} inside #[] values.
// Workspace via session-level ZMUX_WORKSPACE env var.
func nativeStatusLeft(p *theme.Palette, preset Preset) string {
	// Helper: prefix-aware session pill (works for most presets).
	// Pattern: #{?client_prefix, COLOR_A_BLOCK, COLOR_B_BLOCK} then content.
	accent := p.Accent.Hex()
	info := p.Info.Hex()
	bg := p.BG.Hex()
	sf := p.Surface.Hex()
	dm := p.Dim.Hex()
	mt := p.Muted.Hex()
	sp := p.Special.Hex()
	meta := p.Meta.Hex()

	ws := fmt.Sprintf("#{?ZMUX_WORKSPACE,#[fg=%s] #{ZMUX_WORKSPACE} ,}", dm)

	switch preset {
	case Minimal:
		return fmt.Sprintf(
			"#{?client_prefix,#[fg=%s],#[fg=%s]}#[bold] #S #[nobold]%s#[fg=%s]│ ",
			info, accent, ws, dm,
		)

	case Powerline, Rpowerline:
		// For powerline: use accent bg always (no prefix color change on left — too complex).
		// Prefix state shown via hints on the right side.
		rcap := "\ue0b0" // sharp
		if preset == Rpowerline {
			rcap = "\ue0b4" // rounded
		}
		return fmt.Sprintf(
			"#{?client_prefix,#[bg=%s],#[bg=%s]}#[fg=%s,bold]  #S "+
				"#{?client_prefix,#[fg=%s],#[fg=%s]}#[bg=%s]\ue0b0"+
				"#[bg=%s,fg=%s]  #{b:pane_current_path} "+
				"#[fg=%s,bg=default]%s %s",
			info, accent, bg,
			info, accent, sf,
			sf, mt,
			sf, rcap, ws,
		)

	case Blocks:
		return fmt.Sprintf(
			"#{?client_prefix,#[fg=%s],#[fg=%s]}#[bold] [#S] #[nobold]"+
				"#{?ZMUX_WORKSPACE,#[fg=%s][#{ZMUX_WORKSPACE}] ,}",
			info, accent, meta,
		)

	case Rounded:
		return fmt.Sprintf(
			"#{?client_prefix,#[fg=%s],#[fg=%s]}\ue0b6"+
				"#{?client_prefix,#[bg=%s],#[bg=%s]}#[fg=%s,bold] 󱂬 #S "+
				"#{?client_prefix,#[fg=%s],#[fg=%s]}#[bg=default]\ue0b4 %s",
			info, accent,
			info, accent, bg,
			info, accent, ws,
		)

	case Hacker:
		return fmt.Sprintf(
			"#[fg=%s,bold]#S"+
				"#{?ZMUX_WORKSPACE,#[fg=%s]@#[fg=%s,bold]#{ZMUX_WORKSPACE}#[nobold],}"+
				"#[fg=%s] > #[fg=%s]#{b:pane_current_path} ",
			p.Success.Hex(), dm, p.Success.Hex(), dm, dm,
		)

	case Zen:
		return fmt.Sprintf(
			"#{?client_prefix,#[fg=%s],#[fg=%s]} #S"+
				"#{?ZMUX_WORKSPACE, · #{ZMUX_WORKSPACE},} ",
			accent, dm,
		)

	case Starship:
		return fmt.Sprintf(
			"#{?client_prefix,#[fg=%s],#[fg=%s]}#[bold]  #S ❯#[nobold,fg=default] "+
				"#{?ZMUX_WORKSPACE,#[fg=%s] #{ZMUX_WORKSPACE} ,}"+
				"#[fg=%s] #{b:pane_current_path} ",
			info, accent, sp, meta,
		)

	default: // Default
		return fmt.Sprintf(
			"#{?client_prefix,#[fg=%s],#[fg=%s]}\ue0b6"+
				"#{?client_prefix,#[bg=%s],#[bg=%s]}#[fg=%s,bold] 󱂬 #S "+
				"#{?client_prefix,#[fg=%s],#[fg=%s]}#[bg=default]\ue0b4 %s",
			info, accent,
			info, accent, bg,
			info, accent, ws,
		)
	}
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
