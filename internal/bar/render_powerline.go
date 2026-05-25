package bar

// POWERLINE + RPOWERLINE — angled separators (powerline) and rounded-cap
// variant (rpowerline). Grouped because they share the powerline glyph
// vocabulary and the same workspace → session → dir chain shape.

import (
	"fmt"
	"strings"

	"github.com/donjor/zmux/internal/theme"
)

// ── Powerline (tokyo-night inspired) ──

func renderLeftPowerline(p *theme.Palette, ctx BarContext) string {
	var b strings.Builder
	bg := p.Accent.Hex()
	if ctx.Prefix {
		bg = p.Info.Hex()
	}

	// Workspace → Session → Dir chain.
	if ctx.Workspace != "" {
		fmt.Fprintf(&b, "#[bg=%s,fg=%s,bold] 󱂬 %s ", p.Special.Hex(), p.BG.Hex(), ctx.WorkspaceLabel())
		fmt.Fprintf(&b, "#[fg=%s,bg=%s]", p.Special.Hex(), bg)
	}

	fmt.Fprintf(&b, "#[bg=%s,fg=%s,bold]  %s ", bg, p.BG.Hex(), ctx.SessionLabel())
	if ctx.ViewportID != "" {
		fmt.Fprintf(&b, "#[fg=%s,bg=%s]", bg, p.Info.Hex())
		fmt.Fprintf(&b, "#[bg=%s,fg=%s,bold] %s ", p.Info.Hex(), p.BG.Hex(), ctx.ViewportID)
		fmt.Fprintf(&b, "#[fg=%s,bg=%s]", p.Info.Hex(), p.Surface.Hex())
	} else {
		fmt.Fprintf(&b, "#[fg=%s,bg=%s]", bg, p.Surface.Hex())
	}

	dir := shortenDir(ctx.PaneDir)
	if dir != "" {
		fmt.Fprintf(&b, "#[bg=%s,fg=%s]  %s ", p.Surface.Hex(), p.Muted.Hex(), dir)
	}
	fmt.Fprintf(&b, "#[fg=%s,bg=default] ", p.Surface.Hex())

	return b.String()
}

func renderRightPowerline(p *theme.Palette, ctx BarContext) string {
	if ctx.Prefix {
		return prefixHints(p)
	}
	var b strings.Builder

	// Git.
	if ctx.GitBranch != "" {
		gitBg := p.Success.Hex()
		if ctx.GitDirty {
			gitBg = p.Accent.Hex()
		}
		fmt.Fprintf(&b, "#[fg=%s]", gitBg)
		fmt.Fprintf(&b, "#[bg=%s,fg=%s,bold] %s ", gitBg, p.BG.Hex(), formatGitText(ctx))
	}

	// Time.
	if ctx.Time != "" {
		fmt.Fprintf(&b, "#[fg=%s]", p.Surface.Hex())
		fmt.Fprintf(&b, "#[bg=%s,fg=%s] 󱑍 %s ", p.Surface.Hex(), p.Muted.Hex(), ctx.Time)
	}

	// Date accent.
	if ctx.Date != "" {
		fmt.Fprintf(&b, "#[fg=%s]", p.Accent.Hex())
		fmt.Fprintf(&b, "#[bg=%s,fg=%s,bold] %s ", p.Accent.Hex(), p.BG.Hex(), ctx.Date)
	}

	return b.String()
}

func renderTopPowerline(p *theme.Palette, ctx BarContext) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#[fg=%s,bg=%s]#[bg=%s,fg=%s,bold] 󱂬 %s ",
		p.BG.Hex(), p.Special.Hex(), p.Special.Hex(), p.BG.Hex(), ctx.Workspace)
	for i, sess := range ctx.WorkspaceSessions {
		isCurrent := sess == ctx.Session
		bg := p.Dim.Hex()
		if isCurrent {
			bg = p.Accent.Hex()
		}
		prevBG := p.Special.Hex()
		if i > 0 {
			prevBG = p.Surface.Hex()
		}
		fmt.Fprintf(&b, "#[fg=%s,bg=%s]", prevBG, bg)
		if isCurrent {
			fmt.Fprintf(&b, "#[bg=%s,fg=%s,bold] %s #[nobold]", bg, p.BG.Hex(), sess)
		} else {
			fmt.Fprintf(&b, "#[bg=%s,fg=%s] %s ", bg, p.Surface.Hex(), sess)
		}
		fmt.Fprintf(&b, "#[fg=%s,bg=%s]#[bg=%s,fg=%s] ",
			bg, p.Surface.Hex(), p.Surface.Hex(), p.Muted.Hex())
		if i == len(ctx.WorkspaceSessions)-1 {
			fmt.Fprintf(&b, "#[fg=%s,bg=default]", p.Surface.Hex())
		}
	}
	return b.String()
}

// ── Rpowerline (rounded powerline) ──

func renderLeftRpowerline(p *theme.Palette, ctx BarContext) string {
	var b strings.Builder
	bg := p.Accent.Hex()
	if ctx.Prefix {
		bg = p.Info.Hex()
	}

	// Workspace → Session → Dir chain with rounded caps.
	if ctx.Workspace != "" {
		fmt.Fprintf(&b, "#[fg=%s]#[bg=%s,fg=%s,bold] 󱂬 %s ", p.Special.Hex(), p.Special.Hex(), p.BG.Hex(), ctx.WorkspaceLabel())
		fmt.Fprintf(&b, "#[fg=%s,bg=%s]", p.Special.Hex(), bg)
	} else {
		fmt.Fprintf(&b, "#[fg=%s]", bg)
	}

	fmt.Fprintf(&b, "#[bg=%s,fg=%s,bold]  %s ", bg, p.BG.Hex(), ctx.SessionLabel())
	if ctx.ViewportID != "" {
		fmt.Fprintf(&b, "#[fg=%s,bg=%s]", bg, p.Info.Hex())
		fmt.Fprintf(&b, "#[bg=%s,fg=%s,bold] %s ", p.Info.Hex(), p.BG.Hex(), ctx.ViewportID)
		fmt.Fprintf(&b, "#[fg=%s,bg=%s]", p.Info.Hex(), p.Surface.Hex())
	} else {
		fmt.Fprintf(&b, "#[fg=%s,bg=%s]", bg, p.Surface.Hex())
	}

	dir := shortenDir(ctx.PaneDir)
	if dir != "" {
		fmt.Fprintf(&b, "#[bg=%s,fg=%s]  %s ", p.Surface.Hex(), p.Muted.Hex(), dir)
	}
	fmt.Fprintf(&b, "#[fg=%s,bg=default] ", p.Surface.Hex())

	return b.String()
}

func renderRightRpowerline(p *theme.Palette, ctx BarContext) string {
	if ctx.Prefix {
		return prefixHints(p)
	}

	// Build the segments we want to render. Each segment has its own bg.
	type seg struct {
		bg   string
		text string
		bold bool
	}
	var segs []seg

	if ctx.GitBranch != "" {
		gitBg := p.Success.Hex()
		if ctx.GitDirty {
			gitBg = p.Accent.Hex()
		}
		segs = append(segs, seg{bg: gitBg, text: formatGitText(ctx), bold: true})
	}
	if ctx.Time != "" {
		segs = append(segs, seg{bg: p.Surface.Hex(), text: "󱑍 " + ctx.Time})
	}
	if ctx.Date != "" {
		segs = append(segs, seg{bg: p.Accent.Hex(), text: ctx.Date, bold: true})
	}

	if len(segs) == 0 {
		return ""
	}

	var b strings.Builder
	for i, s := range segs {
		// Lead-in: rounded left cap on first segment.
		if i == 0 {
			fmt.Fprintf(&b, "#[fg=%s]", s.bg)
		}
		// Pill body.
		boldTag := ""
		if s.bold {
			boldTag = ",bold"
		}
		fmt.Fprintf(&b, "#[bg=%s,fg=%s%s] %s ", s.bg, p.BG.Hex(), boldTag, s.text)
		// Transition: pointed arrow into next segment, or rounded cap to end.
		if i+1 < len(segs) {
			fmt.Fprintf(&b, "#[fg=%s,bg=%s]", s.bg, segs[i+1].bg)
		} else {
			fmt.Fprintf(&b, "#[fg=%s,bg=default]", s.bg)
		}
	}

	return b.String()
}

func renderTopRpowerline(p *theme.Palette, ctx BarContext) string {
	var b strings.Builder
	// Workspace pill.
	fmt.Fprintf(&b, "#[fg=%s]#[bg=%s,fg=%s,bold] 󱂬 %s #[nobold]",
		p.Special.Hex(), p.Special.Hex(), p.BG.Hex(), ctx.Workspace)
	// Arrow from workspace to first session.
	if len(ctx.WorkspaceSessions) > 0 {
		firstBG := p.Dim.Hex()
		if ctx.WorkspaceSessions[0] == ctx.Session {
			firstBG = p.Accent.Hex()
		}
		fmt.Fprintf(&b, "#[fg=%s,bg=%s]", p.Special.Hex(), firstBG)
	} else {
		fmt.Fprintf(&b, "#[fg=%s,bg=default]", p.Special.Hex())
	}
	// Session tabs — two-section pills matching window-tab chrome.
	for i, sess := range ctx.WorkspaceSessions {
		isCurrent := sess == ctx.Session
		idx := i + 1
		if isCurrent {
			fmt.Fprintf(&b, "#[bg=%s,fg=%s,bold] %d #[fg=%s,bg=%s]#[bg=%s,fg=%s,bold] %s ",
				p.Accent.Hex(), p.BG.Hex(), idx,
				p.Accent.Hex(), p.Surface.Hex(),
				p.Surface.Hex(), p.FG.Hex(), sess)
		} else {
			fmt.Fprintf(&b, "#[bg=%s,fg=%s] %d #[fg=%s,bg=%s]#[bg=%s,fg=%s] %s ",
				p.Dim.Hex(), p.Surface.Hex(), idx,
				p.Dim.Hex(), p.Surface.Hex(),
				p.Surface.Hex(), p.Muted.Hex(), sess)
		}
		// Transition to next or cap.
		if i < len(ctx.WorkspaceSessions)-1 {
			nextBG := p.Dim.Hex()
			if ctx.WorkspaceSessions[i+1] == ctx.Session {
				nextBG = p.Accent.Hex()
			}
			fmt.Fprintf(&b, "#[fg=%s,bg=%s]", p.Surface.Hex(), nextBG)
		} else {
			fmt.Fprintf(&b, "#[fg=%s,bg=default]", p.Surface.Hex())
		}
	}
	return b.String()
}
