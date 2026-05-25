package bar

// MINIMAL — clean, barely decorated, content-first.

import (
	"fmt"
	"strings"

	"github.com/donjor/zmux/internal/theme"
)

func renderLeftMinimal(p *theme.Palette, ctx BarContext) string {
	color := p.Accent.Hex()
	if ctx.Prefix {
		color = p.Info.Hex()
	}
	out := fmt.Sprintf("#[fg=%s,bold] %s ", color, ctx.SessionLabel())
	if ctx.ViewportID != "" {
		out += fmt.Sprintf("#[fg=%s,bold]%s ", p.Info.Hex(), ctx.ViewportID)
	}
	if ctx.Workspace != "" {
		out += fmt.Sprintf("#[fg=%s]%s ", p.Dim.Hex(), ctx.WorkspaceLabel())
	}
	out += fmt.Sprintf("#[fg=%s]│ ", p.Dim.Hex())
	return out
}

func renderRightMinimal(p *theme.Palette, ctx BarContext) string {
	if ctx.Prefix {
		return fmt.Sprintf("#[fg=%s]prefix ", p.Info.Hex())
	}
	var b strings.Builder
	if ctx.GitBranch != "" {
		c := p.Dim.Hex()
		if ctx.GitDirty {
			c = p.Accent.Hex()
		}
		fmt.Fprintf(&b, "#[fg=%s]%s ", c, ctx.GitBranch)
	}
	if ctx.Time != "" {
		fmt.Fprintf(&b, "#[fg=%s]%s ", p.Dim.Hex(), ctx.Time)
	}
	return b.String()
}

func renderTopMinimal(p *theme.Palette, ctx BarContext) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#[fg=%s,bold] %s  ", p.FG.Hex(), ctx.Workspace)
	for i, sess := range ctx.WorkspaceSessions {
		if sess == ctx.Session {
			fmt.Fprintf(&b, "#[fg=%s,bold]%s#[nobold]", p.FG.Hex(), sess)
		} else {
			fmt.Fprintf(&b, "#[fg=%s]%s", p.Dim.Hex(), sess)
		}
		if i < len(ctx.WorkspaceSessions)-1 {
			b.WriteString("  ")
		}
	}
	return b.String()
}
