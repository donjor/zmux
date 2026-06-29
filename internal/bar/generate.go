package bar

import (
	"fmt"
	"strings"

	"github.com/donjor/zmux/internal/tablabel"
	"github.com/donjor/zmux/internal/theme"
)

// TmuxOption is a key-value pair for a tmux set-option call.
type TmuxOption struct {
	Key, Value string
}

// BarLayoutConfig holds the multi-session layout settings.
type BarLayoutConfig struct {
	Layout    string // "two-line", "split" (legacy "single" normalized to two-line)
	Indicator string // "none", "numbers", "dots"
	TopBar    string // "tabs", "dots", "minimal"
}

// barRenderArgs are the tmux format tokens passed to zmux bar-render
// subcommands. Tmux expands these per-client before executing #().
const barRenderArgs = "--session '#S'" +
	" --prefix '#{client_prefix}'" +
	" --group '#{session_group}'" +
	" --group-size '#{session_group_size}'" +
	" --pane-cmd '#{pane_current_command}'" +
	" --dir '#{pane_current_path}'" +
	" --panes '#{window_panes}'" +
	// pane_title is attacker-controllable (any program sets it via an OSC title
	// escape), and it lands inside the single-quoted #() shell arg. Strip single
	// quotes via tmux's #{s///} BEFORE embedding: inside single quotes `'` is the
	// only shell-special char, so removing it makes the arg injection-proof
	// regardless of other content (codex diff review — this is a trust boundary,
	// not the cosmetic edge --dir has). The render side filters hostname/cmd noise.
	" --pane-title '#{s/'//:pane_title}'"

// TopBarFormatCmd returns the tmux #() command string for the top bar
// status-format entry. Used by generate and by bar-render to
// dynamically restore the top row when sessions appear.
func TopBarFormatCmd(zmuxBin, topBar string) string {
	if topBar == "" {
		topBar = "tabs"
	}
	return fmt.Sprintf("#(%s bar-render top --top-bar '%s' %s)", zmuxBin, topBar, barRenderArgs)
}

// Status-format sections (tmux 3.4 default, carved up so the window-list
// middle can be swapped for the dynamic logical tabs row while status-left
// and status-right keep their native rendering).
const (
	statusFmtLeftSection  = `#[align=left range=left #{E:status-left-style}]#[push-default]#{T;=/#{status-left-length}:status-left}#[pop-default]#[norange default]`
	statusFmtWindowList   = `#[list=on align=#{status-justify}]#[list=left-marker]<#[list=right-marker]>#[list=on]#{W:#[range=window|#{window_index} #{E:window-status-style}#{?#{&&:#{window_last_flag},#{!=:#{E:window-status-last-style},default}}, #{E:window-status-last-style},}#{?#{&&:#{window_bell_flag},#{!=:#{E:window-status-bell-style},default}}, #{E:window-status-bell-style},#{?#{&&:#{||:#{window_activity_flag},#{window_silence_flag}},#{!=:#{E:window-status-activity-style},default}}, #{E:window-status-activity-style},}}]#[push-default]#{T:window-status-format}#[pop-default]#[norange default]#{?window_end_flag,,#{window-status-separator}},#[range=window|#{window_index} list=focus #{?#{!=:#{E:window-status-current-style},default},#{E:window-status-current-style},#{E:window-status-style}}#{?#{&&:#{window_last_flag},#{!=:#{E:window-status-last-style},default}}, #{E:window-status-last-style},}#{?#{&&:#{window_bell_flag},#{!=:#{E:window-status-bell-style},default}}, #{E:window-status-bell-style},#{?#{&&:#{||:#{window_activity_flag},#{window_silence_flag}},#{!=:#{E:window-status-activity-style},default}}, #{E:window-status-activity-style},}}]#[push-default]#{T:window-status-current-format}#[pop-default]#[norange list=on default]#{?window_end_flag,,#{window-status-separator}}}`
	statusFmtRightSection = `#[nolist align=right range=right #{E:status-right-style}]#[push-default]#{T;=/#{status-right-length}:status-right}#[pop-default]#[norange default]`
)

// TmuxDefaultStatusFormat is the standard tmux status-format[0] template
// that renders status-left, window tabs, and status-right. Used as
// status-format[1] in two-line layouts (so the normal bar renders on the
// bottom line) and as the binary-less fallback for the logical tabs row.
// Matches the tmux 3.4 default.
const TmuxDefaultStatusFormat = statusFmtLeftSection + statusFmtWindowList + statusFmtRightSection

// TabsRowStatusFormat is TmuxDefaultStatusFormat with the native window list
// swapped for the dynamic logical tabs row (`bar-render tabs`): pane-of tabs
// ride their host cell, hidden pane-tabs render dim under their parent, state
// glyphs come from pane-canonical state. status-left/right keep native
// rendering. The
// native window list is the fallback when no zmux binary is available.
//
// No #[range=window|N] click targets in the dynamic row (directive support
// inside #() output is unverified for ranges) — window switching stays on
// keys/tabpicker.
func TabsRowStatusFormat(zmuxBin string) string {
	return statusFmtLeftSection +
		fmt.Sprintf(`#[list=on align=#{status-justify}]#(%s bar-render tabs --session '#S' --prefix '#{client_prefix}' --group '#{session_group}')`, zmuxBin) +
		statusFmtRightSection
}

// Generate produces the tmux status-line options for a given preset and palette.
// If zmuxBin is provided, uses #(zmux bar-render) for dynamic left/right content
// (git branch, workspace, prefix hints). Otherwise falls back to static tmux formats.
func Generate(preset Preset, palette *theme.Palette, zmuxBin ...string) []TmuxOption {
	return GenerateWithLayout(preset, palette, BarLayoutConfig{Layout: "two-line"}, zmuxBin...)
}

// GenerateWithLayout is Generate with explicit layout configuration.
func GenerateWithLayout(preset Preset, palette *theme.Palette, layout BarLayoutConfig, zmuxBin ...string) []TmuxOption {
	opts := sharedOptions(palette)

	bin := ""
	if len(zmuxBin) > 0 {
		bin = zmuxBin[0]
	}

	// If zmux binary available, use dynamic rendering for left/right.
	if bin != "" {
		opts = append(opts, dynamicOptions(palette, bin, preset, layout)...)
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

	// State first (targets the single raw #W), labels second (#W → label
	// conditional, which itself contains #W tokens) — see withTabStateFormats.
	return withTabLabelFormats(withTabStateFormats(opts, palette, bin), palette)
}

func withTabLabelFormats(opts []TmuxOption, palette *theme.Palette) []TmuxOption {
	name := tablabel.Format(palette.Dim.Hex())
	for i := range opts {
		switch opts[i].Key {
		case "window-status-format", "window-status-current-format":
			opts[i].Value = strings.ReplaceAll(opts[i].Value, "#W", name)
		}
	}
	return opts
}

// dynamicOptions uses #(zmux bar-render) for left/right, keeping window
// format strings as tmux-native (they're per-window and change with each tab).
func dynamicOptions(p *theme.Palette, zmuxBin string, preset Preset, layout BarLayoutConfig) []TmuxOption {
	// Status-left / status-right: #(zmux bar-render ...) so the live bar
	// shares the Go render code path with the dashboard preview.
	//
	// All tmux state is passed in as flags — tmux substitutes these format
	// tokens per-client before executing the shell command. Querying tmux
	// from inside bar-render (via display-message) would return the
	// globally-focused client's state, which is wrong when multiple clients
	// are attached to different sessions (the workspace/session pill would
	// be stuck on whichever session was last focused globally).
	//
	// The expanded arguments also participate in tmux's #() cache key, so
	// each distinct (session, prefix, dir, ...) tuple gets its own cached
	// output and refreshes correctly per-client.
	statusLeft := fmt.Sprintf("#(%s bar-render left %s)", zmuxBin, barRenderArgs)
	statusRight := fmt.Sprintf("#(%s bar-render right %s)", zmuxBin, barRenderArgs)

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

	opts := []TmuxOption{
		{"status-style", fmt.Sprintf("bg=%s,fg=%s", barBG, p.Muted.Hex())},
		{"status-left", statusLeft},
		{"status-right", statusRight},
		{"window-status-format", windowFmt},
		{"window-status-current-format", windowCurrentFmt},
		{"window-status-separator", windowSep},
		{"status-left-length", "100"},
		{"status-right-length", "80"},
	}

	// Multi-line layouts: add a top row with workspace + session info.
	// tmux's `status 2` gives two rows sharing the same status-style bg.
	// always-2-line (plan 024): the top row renders for a single session too,
	// so two-line/split is a stable two rows that never reflows on session
	// add/remove. Single-session collapse was removed.
	if zmuxBin != "" && (layout.Layout == "two-line" || layout.Layout == "split") {
		opts = append(
			opts,
			TmuxOption{"status", "2"},
			TmuxOption{"status-format[0]", TopBarFormatCmd(zmuxBin, layout.TopBar)},
			TmuxOption{"status-format[1]", TabsRowStatusFormat(zmuxBin)},
		)
	} else if zmuxBin != "" {
		// Single layout: restore in case we're switching from two-line.
		opts = append(
			opts,
			TmuxOption{"status", "on"},
			TmuxOption{"status-format[0]", TabsRowStatusFormat(zmuxBin)},
		)
	}

	return opts
}

// sharedOptions returns options common to all presets.
func sharedOptions(p *theme.Palette) []TmuxOption {
	return []TmuxOption{
		{"pane-border-status", "top"},
		{"pane-border-lines", "single"},
		{"pane-border-indicators", "both"},
		{"pane-border-style", fmt.Sprintf("fg=%s,bg=default", p.Dim.Hex())},
		{"pane-active-border-style", fmt.Sprintf("fg=%s,bg=default,bold", p.Accent.Hex())},
		{"pane-border-format", paneBorderFormat(p)},
		{"message-style", fmt.Sprintf("bg=%s,fg=%s", p.Surface.Hex(), p.FG.Hex())},
		{"message-command-style", fmt.Sprintf("bg=%s,fg=%s", p.Surface.Hex(), p.FG.Hex())},
		{"mode-style", fmt.Sprintf("bg=%s,fg=%s", p.Info.Hex(), p.BG.Hex())},
		{"clock-mode-colour", p.Accent.Hex()},
		{"window-active-style", fmt.Sprintf("#{?client_prefix,bg=%s,bg=default}", p.BGPrefix.Hex())},
		{"window-style", "bg=default"},
	}
}

func paneBorderFormat(p *theme.Palette) string {
	// Only a split window gets per-pane headers; a lone pane shows none.
	// Active and inactive panes render the SAME shape — "<N> <name> <detail>" —
	// so the line doesn't reflow on focus change. The old format swapped in
	// pane_id + WxH only for the active pane, which read as the panes
	// renumbering every time focus moved.
	//   N      = pane_index — stable per slot, not the raw %id.
	//   name   = the tab's @zmux_label, but ONLY when the pane is zmux-managed
	//            (@zmux_tab_id is pane-exact). An unmanaged raw split shows its
	//            command instead, so a window-level label can't leak onto it.
	//   detail = pane_title (e.g. an agent's task line).
	//
	// ponytail: a managed-but-unlabeled pane in a joined window can still
	// merge-read the host window's @zmux_label — tmux formats can't do a
	// pane-exact option read. Rare and cosmetic; add a pane-exact label option
	// if it ever bites.
	name := "#{?#{@zmux_tab_id}," +
		"#{?#{@zmux_label},#{@zmux_label},#{pane_current_command}}," +
		"#{pane_current_command}}"
	active := fmt.Sprintf(
		"#[fg=%s]#[bold] ● #{pane_index} #[fg=%s]%s #[nobold]#[fg=%s]#{pane_title} ",
		p.Accent.Hex(), p.FG.Hex(), name, p.Muted.Hex(),
	)
	inactive := fmt.Sprintf(
		"#[fg=%s] ○ #{pane_index} #[fg=%s]%s #[fg=%s]#{pane_title} ",
		p.Dim.Hex(), p.Muted.Hex(), name, p.Dim.Hex(),
	)
	return fmt.Sprintf("#{?#{>:#{window_panes},1},#{?pane_active,%s,%s},}", active, inactive)
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
	// Static fallback hint (Phase 1.5 makes the split-pane variant context-aware).
	// `s` now toggles split orientation, so the session picker hint points at its
	// canonical key `w` instead.
	prefixHint := fmt.Sprintf(
		"#[fg=%s]spc#[fg=%s]dash #[fg=%s]d#[fg=%s]etach #[fg=%s]c#[fg=%s]tab #[fg=%s]w#[fg=%s]switch #[fg=%s]v#[fg=%s]im #[fg=%s]?#[fg=%s]help ",
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
