package bar

// STARSHIP — colorful prompt-inspired, each segment its own color.

import (
	"fmt"
	"strings"

	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/theme"
)

func renderLeftStarship(p *theme.Palette, ctx BarContext) string {
	var b strings.Builder
	bg := p.Accent.Hex()
	if ctx.Prefix {
		bg = p.Info.Hex()
	}

	// Session with chevron + viewport letter.
	fmt.Fprintf(&b, "#[fg=%s,bold]  %s ", bg, ctx.SessionLabel())
	if ctx.ViewportID != "" {
		fmt.Fprintf(&b, "#[fg=%s]%s ", p.Info.Hex(), ctx.ViewportID)
	}
	fmt.Fprintf(&b, "#[fg=%s]❯#[fg=default] ", bg)

	// Workspace.
	if ctx.Workspace != "" {
		fmt.Fprintf(&b, "#[fg=%s] %s ", p.Special.Hex(), ctx.WorkspaceLabel())
	}

	// Directory.
	dir := shortenDir(ctx.PaneDir)
	if dir != "" {
		fmt.Fprintf(&b, "#[fg=%s] %s ", p.Meta.Hex(), dir)
	}

	// Git inline (starship puts it on left).
	if ctx.GitBranch != "" {
		bc := p.Success.Hex()
		if ctx.GitDirty {
			bc = p.Accent.Hex()
		}
		fmt.Fprintf(&b, "#[fg=%s]󰘬 %s", bc, ctx.GitBranch)
		if ctx.GitDirty {
			fmt.Fprintf(&b, " #[fg=%s]✎", p.Accent.Hex())
		}
		if ctx.GitAhead > 0 {
			fmt.Fprintf(&b, " #[fg=%s]⇡%d", p.Success.Hex(), ctx.GitAhead)
		}
		if ctx.GitBehind > 0 {
			fmt.Fprintf(&b, " #[fg=%s]⇣%d", p.Error.Hex(), ctx.GitBehind)
		}
		b.WriteString(" ")
	}

	return b.String()
}

func renderRightStarship(p *theme.Palette, ctx BarContext) string {
	if ctx.Prefix {
		return fmt.Sprintf("#[fg=%s]⌨ prefix ", p.Info.Hex())
	}
	var b strings.Builder

	// Process.
	if ctx.PaneCmd != "" && !tabs.IsIdleShell(ctx.PaneCmd) {
		fmt.Fprintf(&b, "#[fg=%s] %s ", p.Info.Hex(), ctx.PaneCmd)
	}

	// Lang.
	if ctx.LangVersion != "" {
		fmt.Fprintf(&b, "#[fg=%s]%s%s ", p.Meta.Hex(), ctx.LangIcon, ctx.LangVersion)
	}

	// Group.
	if ctx.GroupID != "" && ctx.Attached > 1 {
		fmt.Fprintf(&b, "#[fg=%s]◉%d ", p.Special.Hex(), ctx.Attached)
	}

	// Time.
	if ctx.Time != "" {
		fmt.Fprintf(&b, "#[fg=%s]󱑍 %s ", p.Muted.Hex(), ctx.Time)
	}

	return b.String()
}

func renderTopStarship(p *theme.Palette, ctx BarContext) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#[fg=%s,bold] 󱂬 %s  ", p.Special.Hex(), ctx.Workspace)
	for i, sess := range ctx.WorkspaceSessions {
		if sess == ctx.Session {
			fmt.Fprintf(&b, "#[fg=%s,bold]%s #[fg=%s]❯#[fg=default,nobold] ",
				p.Accent.Hex(), topSessionLabel(ctx, sess), p.Accent.Hex())
		} else {
			fmt.Fprintf(&b, "#[fg=%s]%s ", p.Dim.Hex(), sess)
		}
		if i < len(ctx.WorkspaceSessions)-1 && sess != ctx.Session {
			b.WriteString(" ")
		}
	}
	return b.String()
}
