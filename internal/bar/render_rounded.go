package bar

// ROUNDED — elevated pill segments, catppuccin-style, premium feel.

import (
	"fmt"
	"strings"

	"github.com/donjor/zmux/internal/theme"
)

func renderLeftRounded(p *theme.Palette, ctx BarContext) string {
	var b strings.Builder
	bg := p.Accent.Hex()
	if ctx.Prefix {
		bg = p.Info.Hex()
	}
	sf := p.Surface.Hex()

	// Session pill (with viewport letter attached).
	fmt.Fprintf(&b, "#[fg=%s]\ue0b6#[bg=%s,fg=%s,bold] 󱂬 %s ",
		bg, bg, p.BG.Hex(), ctx.SessionLabel())
	if ctx.ViewportID != "" {
		fmt.Fprintf(&b, "#[fg=%s,bg=%s]\ue0b0#[bg=%s,fg=%s,bold] %s #[fg=%s,bg=default]\ue0b4 ",
			bg, p.Info.Hex(), p.Info.Hex(), p.BG.Hex(), ctx.ViewportID, p.Info.Hex())
	} else {
		fmt.Fprintf(&b, "#[fg=%s,bg=default]\ue0b4 ", bg)
	}

	// Workspace pill on surface.
	if ctx.Workspace != "" {
		fmt.Fprintf(&b, "#[fg=%s]\ue0b6#[bg=%s,fg=%s]  %s #[fg=%s,bg=default]\ue0b4 ",
			sf, sf, p.Special.Hex(), ctx.WorkspaceLabel(), sf)
	}

	return b.String()
}

func renderRightRounded(p *theme.Palette, ctx BarContext) string {
	if ctx.Prefix {
		sf := p.Surface.Hex()
		return fmt.Sprintf(
			"#[fg=%s]\ue0b6#[bg=%s,fg=%s] spc·dash  d·etach  ?·help #[fg=%s,bg=default]\ue0b4 ",
			sf, sf, p.Info.Hex(), sf,
		)
	}
	var b strings.Builder
	sf := p.Surface.Hex()

	// Git pill.
	if ctx.GitBranch != "" {
		gitBg := p.Success.Hex()
		if ctx.GitDirty {
			gitBg = p.Accent.Hex()
		}
		fmt.Fprintf(&b, "#[fg=%s]\ue0b6#[bg=%s,fg=%s,bold] %s #[fg=%s,bg=default]\ue0b4 ",
			gitBg, gitBg, p.BG.Hex(), formatGitText(ctx), gitBg)
	}

	// Lang pill.
	if ctx.LangVersion != "" {
		fmt.Fprintf(&b, "#[fg=%s]\ue0b6#[bg=%s,fg=%s] %s%s #[fg=%s,bg=default]\ue0b4 ",
			sf, sf, p.Muted.Hex(), ctx.LangIcon, ctx.LangVersion, sf)
	}

	// Time pill.
	if ctx.Time != "" {
		fmt.Fprintf(&b, "#[fg=%s]\ue0b6#[bg=%s,fg=%s] 󱑍 %s #[fg=%s,bg=default]\ue0b4",
			sf, sf, p.Muted.Hex(), ctx.Time, sf)
	}

	return b.String()
}

func renderTopRounded(p *theme.Palette, ctx BarContext) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#[fg=%s]\ue0b6#[bg=%s,fg=%s,bold] 󱂬 %s #[fg=%s,bg=default]\ue0b4 ",
		p.Special.Hex(), p.Special.Hex(), p.BG.Hex(), ctx.Workspace, p.Special.Hex())
	for _, sess := range ctx.WorkspaceSessions {
		if sess == ctx.Session {
			fmt.Fprintf(&b, "#[fg=%s]\ue0b6#[bg=%s,fg=%s,bold] %s #[fg=%s,bg=default]\ue0b4 ",
				p.Accent.Hex(), p.Accent.Hex(), p.BG.Hex(), topSessionLabel(ctx, sess), p.Accent.Hex())
		} else {
			fmt.Fprintf(&b, "#[fg=%s]\ue0b6#[bg=%s,fg=%s] %s #[fg=%s,bg=default]\ue0b4 ",
				p.Surface.Hex(), p.Surface.Hex(), p.Dim.Hex(), sess, p.Surface.Hex())
		}
	}
	return b.String()
}
