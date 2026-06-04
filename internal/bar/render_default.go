package bar

// DEFAULT — catppuccin-inspired: rounded pills, icons, elevated surfaces.

import (
	"fmt"
	"strings"

	"github.com/donjor/zmux/internal/theme"
)

func renderLeftDefault(p *theme.Palette, ctx BarContext) string {
	var b strings.Builder
	bg := p.Accent.Hex()
	if ctx.Prefix {
		bg = p.Info.Hex()
	}

	// Session pill with icon.
	fmt.Fprintf(&b, "#[fg=%s]\ue0b6#[bg=%s,fg=%s,bold] 󱂬 %s ",
		bg, bg, p.BG.Hex(), ctx.SessionLabel())

	// Viewport letter (attached to session pill).
	if ctx.ViewportID != "" {
		fmt.Fprintf(&b, "#[fg=%s,bg=%s]\ue0b0#[bg=%s,fg=%s,bold] %s ",
			bg, p.Info.Hex(), p.Info.Hex(), p.BG.Hex(), ctx.ViewportID)
		fmt.Fprintf(&b, "#[fg=%s,bg=default]\ue0b4 ", p.Info.Hex())
	} else {
		fmt.Fprintf(&b, "#[fg=%s,bg=default]\ue0b4 ", bg)
	}

	// Workspace pill (elevated surface).
	if ctx.Workspace != "" {
		fmt.Fprintf(&b, "#[fg=%s]\ue0b6#[bg=%s,fg=%s]  %s #[fg=%s,bg=default]\ue0b4 ",
			p.Surface.Hex(), p.Surface.Hex(), p.Meta.Hex(), ctx.WorkspaceLabel(), p.Surface.Hex())
	}

	return b.String()
}

func renderRightDefault(p *theme.Palette, ctx BarContext) string {
	if ctx.Prefix {
		return prefixHints(p)
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

	// Lang pill (surface bg).
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

func renderTopDefault(p *theme.Palette, ctx BarContext) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#[fg=%s]\ue0b6#[bg=%s,fg=%s,bold] 󱂬 %s #[fg=%s,bg=default]\ue0b4 ",
		p.Special.Hex(), p.Special.Hex(), p.BG.Hex(), ctx.Workspace, p.Special.Hex())
	for _, sess := range ctx.WorkspaceSessions {
		if sess == ctx.Session {
			fmt.Fprintf(&b, "#[fg=%s,bold] %s #[nobold,fg=%s]", p.Accent.Hex(), topSessionLabel(ctx, sess), p.Muted.Hex())
		} else {
			fmt.Fprintf(&b, "#[fg=%s] %s ", p.Dim.Hex(), sess)
		}
	}
	return b.String()
}
