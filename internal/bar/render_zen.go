package bar

// ZEN — ultra-minimal, barely there, content whispers.

import (
	"fmt"
	"strings"

	"github.com/donjor/zmux/internal/theme"
)

func renderLeftZen(p *theme.Palette, ctx BarContext) string {
	color := p.Dim.Hex()
	if ctx.Prefix {
		color = p.Accent.Hex()
	}
	out := fmt.Sprintf("#[fg=%s] %s", color, ctx.SessionLabel())
	if ctx.ViewportID != "" {
		out += fmt.Sprintf("#[fg=%s] %s", p.Info.Hex(), ctx.ViewportID)
	}
	if ctx.Workspace != "" {
		out += fmt.Sprintf(" · %s", ctx.WorkspaceLabel())
	}
	out += " "
	return out
}

func renderRightZen(p *theme.Palette, ctx BarContext) string {
	if ctx.Prefix {
		return fmt.Sprintf("#[fg=%s]· ", p.Accent.Hex())
	}
	d := p.Dim.Hex()
	var parts []string
	if ctx.GitBranch != "" {
		g := ctx.GitBranch
		if ctx.GitDirty {
			g += "·"
		}
		parts = append(parts, g)
	}
	if ctx.Time != "" {
		parts = append(parts, ctx.Time)
	}
	if len(parts) == 0 {
		return ""
	}
	return fmt.Sprintf("#[fg=%s]%s ", d, strings.Join(parts, "  "))
}

func renderTopZen(p *theme.Palette, ctx BarContext) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#[fg=%s]%s ", p.Dim.Hex(), ctx.Workspace)
	for i, sess := range ctx.WorkspaceSessions {
		if sess == ctx.Session {
			fmt.Fprintf(&b, "#[fg=%s]%s", p.Muted.Hex(), topSessionLabel(ctx, sess))
		} else {
			fmt.Fprintf(&b, "#[fg=%s]%s", p.Dim.Hex(), sess)
		}
		if i < len(ctx.WorkspaceSessions)-1 {
			fmt.Fprintf(&b, "#[fg=%s] · ", p.Dim.Hex())
		}
	}
	return b.String()
}
