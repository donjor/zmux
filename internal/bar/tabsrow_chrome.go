package bar

// TABS ROW CHROME — per-preset cell decoration for the logical tabs row.
//
// The dynamic row (`bar-render tabs`) replaced tmux's native window list, so
// tmux never expands window-status-format when the binary is present — the
// preset pill chrome has to render here. Specs are 1:1 ports of the
// windowFmt/windowCurrentFmt strings in dynamicOptions (generate.go), with
// one addition: explicit `nobold` resets where the native list relied on
// #[push-default]/#[pop-default] per cell — in #() output styles leak across
// cells unless each cell cleans up after itself.
//
// `content` arrives pre-rendered (name/label + state glyph + riders) carrying
// no bg directives, so it inherits the cell's background — same in-pill
// rationale as the old withTabStateFormats injection.

import (
	"fmt"

	"github.com/donjor/zmux/internal/theme"
)

// Powerline glyph vocabulary (PUA codepoints, written as \u escapes — see
// bar glyph gotchas: literal icon glyphs break grep/Edit tooling).
const (
	tabArrow    = "\ue0b0" // powerline right arrow
	tabCapLeft  = "\ue0b6" // rounded left cap
	tabCapRight = "\ue0b4" // rounded right cap
	tabThinSp   = "\u2009" // thin space (rpowerline index padding)
)

// renderTabCell wraps one logical tab cell in preset chrome. index is the
// tmux window index; content is the cell body (name, glyph, riders); prefix
// mirrors #{client_prefix} — Blocks/Default tint on it, same as their tmux
// format conditionals did (which can't expand inside #() output).
func renderTabCell(p *theme.Palette, preset Preset, index int, content string, active, prefix bool) string {
	switch preset {
	case Minimal:
		if active {
			return fmt.Sprintf("#[fg=%s,bold] %s #[nobold]", p.FG.Hex(), content)
		}
		return fmt.Sprintf("#[fg=%s] %s ", p.Dim.Hex(), content)

	case Powerline:
		// Two-tone: [index]▸[name] with sharp powerline arrows.
		if active {
			return fmt.Sprintf(
				"#[fg=%s,bg=%s]"+tabArrow+"#[bg=%s,fg=%s,bold] %d "+
					"#[fg=%s,bg=%s]"+tabArrow+
					"#[bg=%s,fg=%s,bold] %s "+
					"#[nobold,fg=%s,bg=default]"+tabArrow,
				p.BG.Hex(), p.Accent.Hex(), p.Accent.Hex(), p.BG.Hex(), index,
				p.Accent.Hex(), p.Surface.Hex(),
				p.Surface.Hex(), p.FG.Hex(), content,
				p.Surface.Hex(),
			)
		}
		return fmt.Sprintf(
			"#[fg=%s,bg=%s]"+tabArrow+"#[bg=%s,fg=%s] %d "+
				"#[fg=%s,bg=%s]"+tabArrow+
				"#[bg=%s,fg=%s] %s "+
				"#[fg=%s,bg=default]"+tabArrow,
			p.BG.Hex(), p.Dim.Hex(), p.Dim.Hex(), p.Surface.Hex(), index,
			p.Dim.Hex(), p.Surface.Hex(),
			p.Surface.Hex(), p.Muted.Hex(), content,
			p.Surface.Hex(),
		)

	case Blocks:
		tint := p.Dim.Hex()
		if active {
			tint = p.Accent.Hex()
		}
		if prefix {
			tint = p.Info.Hex()
		}
		return fmt.Sprintf("#[fg=%s,bold] [%d:%s] #[nobold]", tint, index, content)

	case Rounded:
		if active {
			return fmt.Sprintf(
				"#[fg=%s]"+tabCapLeft+"#[bg=%s,fg=%s,bold] %d %s #[nobold,fg=%s,bg=default]"+tabCapRight,
				p.Accent.Hex(), p.Accent.Hex(), p.BG.Hex(), index, content, p.Accent.Hex(),
			)
		}
		return fmt.Sprintf(
			"#[fg=%s]"+tabCapLeft+"#[bg=%s,fg=%s] %d %s #[fg=%s,bg=default]"+tabCapRight,
			p.Surface.Hex(), p.Surface.Hex(), p.Dim.Hex(), index, content, p.Surface.Hex(),
		)

	case Hacker:
		if active {
			return fmt.Sprintf("#[fg=%s,bold]%d:%s#[nobold]", p.Success.Hex(), index, content)
		}
		return fmt.Sprintf("#[fg=%s]%d:%s", p.Dim.Hex(), index, content)

	case Zen:
		// Just the name, barely visible — no index.
		fg := p.Dim.Hex()
		if active {
			fg = p.Muted.Hex()
		}
		return fmt.Sprintf("#[fg=%s]%s", fg, content)

	case Starship:
		if active {
			return fmt.Sprintf(
				"#[fg=%s,bold] %d %s #[fg=%s]❯#[fg=default,nobold]",
				p.Accent.Hex(), index, content, p.Accent.Hex(),
			)
		}
		return fmt.Sprintf("#[fg=%s] %d %s ", p.Dim.Hex(), index, content)

	case Rpowerline:
		// Catppuccin-inspired two-tone pills: [accent index]▸[surface name],
		// rounded caps on outer edges, powerline arrow between sections.
		if active {
			return fmt.Sprintf(
				"#[fg=%s]"+tabCapLeft+"#[bg=%s,fg=%s,bold]%d"+tabThinSp+
					"#[fg=%s,bg=%s]"+tabArrow+
					"#[bg=%s,fg=%s,bold] %s "+
					"#[nobold,fg=%s,bg=default]"+tabCapRight,
				p.Accent.Hex(), p.Accent.Hex(), p.BG.Hex(), index,
				p.Accent.Hex(), p.Surface.Hex(),
				p.Surface.Hex(), p.FG.Hex(), content,
				p.Surface.Hex(),
			)
		}
		return fmt.Sprintf(
			"#[fg=%s]"+tabCapLeft+"#[bg=%s,fg=%s]%d"+tabThinSp+
				"#[fg=%s,bg=%s]"+tabArrow+
				"#[bg=%s,fg=%s] %s "+
				"#[fg=%s,bg=default]"+tabCapRight,
			p.Dim.Hex(), p.Dim.Hex(), p.Surface.Hex(), index,
			p.Dim.Hex(), p.Surface.Hex(),
			p.Surface.Hex(), p.Muted.Hex(), content,
			p.Surface.Hex(),
		)

	default: // Default
		if active {
			tint := p.Accent.Hex()
			if prefix {
				tint = p.Info.Hex()
			}
			return fmt.Sprintf("#[fg=%s,bold] %d %s #[fg=%s,nobold]", tint, index, content, p.Muted.Hex())
		}
		return fmt.Sprintf("#[fg=%s] %d %s ", p.Dim.Hex(), index, content)
	}
}

// tabCellSep separates two cells — the window-status-separator port.
func tabCellSep(p *theme.Palette, preset Preset) string {
	switch preset {
	case Hacker:
		return fmt.Sprintf("#[fg=%s,nobold]|", p.Dim.Hex())
	case Zen:
		return fmt.Sprintf("#[fg=%s,nobold] · ", p.Dim.Hex())
	case Blocks, Rounded:
		return " "
	case Minimal, Powerline, Rpowerline, Starship:
		return ""
	default:
		return fmt.Sprintf("#[fg=%s,nobold]│", p.Dim.Hex())
	}
}
