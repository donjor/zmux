package bar

// HACKER — matrix-inspired, monospace, dense info, green on dark.

import (
	"fmt"
	"strings"

	"github.com/donjor/zmux/internal/theme"
)

func renderLeftHacker(p *theme.Palette, ctx BarContext) string {
	var b strings.Builder
	g := p.Success.Hex()
	d := p.Dim.Hex()

	fmt.Fprintf(&b, "#[fg=%s,bold]%s", g, ctx.SessionLabel())
	if ctx.ViewportID != "" {
		fmt.Fprintf(&b, "#[fg=%s]:#[fg=%s]%s", d, p.Info.Hex(), ctx.ViewportID)
	}
	if ctx.Workspace != "" {
		fmt.Fprintf(&b, "#[fg=%s]@#[fg=%s]%s", d, g, ctx.WorkspaceLabel())
	}
	fmt.Fprintf(&b, "#[fg=%s] > ", d)

	// Active process.
	if ctx.PaneCmd != "" && ctx.PaneCmd != "bash" && ctx.PaneCmd != "zsh" && ctx.PaneCmd != "fish" {
		fmt.Fprintf(&b, "#[fg=%s]%s ", g, ctx.PaneCmd)
	}

	dir := shortenDir(ctx.PaneDir)
	if dir != "" {
		fmt.Fprintf(&b, "#[fg=%s]%s ", d, dir)
	}

	return b.String()
}

func renderRightHacker(p *theme.Palette, ctx BarContext) string {
	var b strings.Builder
	g := p.Success.Hex()
	d := p.Dim.Hex()

	if ctx.Prefix {
		fmt.Fprintf(&b, "#[fg=%s,blink]PREFIX#[noblink] ", g)
		fmt.Fprintf(&b, "#[fg=%s]spc:dash d:detach c:tab x:close ?:help ", d)
		return b.String()
	}

	if ctx.GitBranch != "" {
		fmt.Fprintf(&b, "#[fg=%s]git:", d)
		fmt.Fprintf(&b, "#[fg=%s]%s", g, ctx.GitBranch)
		if ctx.GitDirty {
			fmt.Fprintf(&b, "#[fg=%s]+", p.Accent.Hex())
		}
		if ctx.GitAhead > 0 || ctx.GitBehind > 0 {
			fmt.Fprintf(&b, "#[fg=%s](%d/%d)", d, ctx.GitAhead, ctx.GitBehind)
		}
		b.WriteString(" ")
	}

	if ctx.LangVersion != "" {
		fmt.Fprintf(&b, "#[fg=%s]%s%s ", d, ctx.LangIcon, ctx.LangVersion)
	}

	if ctx.Time != "" {
		fmt.Fprintf(&b, "#[fg=%s]%s ", g, ctx.Time)
	}

	return b.String()
}

func renderTopHacker(p *theme.Palette, ctx BarContext) string {
	var b strings.Builder
	g := p.Success.Hex()
	d := p.Dim.Hex()
	fmt.Fprintf(&b, "#[fg=%s]%s", g, ctx.Workspace)
	for i, sess := range ctx.WorkspaceSessions {
		if i == 0 {
			fmt.Fprintf(&b, "#[fg=%s]>", d)
		}
		if sess == ctx.Session {
			fmt.Fprintf(&b, "#[fg=%s,bold]%d:%s#[nobold]", g, i+1, topSessionLabel(ctx, sess))
		} else {
			fmt.Fprintf(&b, "#[fg=%s]%d:%s", d, i+1, sess)
		}
		if i < len(ctx.WorkspaceSessions)-1 {
			fmt.Fprintf(&b, "#[fg=%s]|", d)
		}
	}
	return b.String()
}
