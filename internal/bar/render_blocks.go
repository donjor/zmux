package bar

// BLOCKS — square brackets, monospace, dense.

import (
	"fmt"
	"strings"

	"github.com/donjor/zmux/internal/theme"
)

func renderLeftBlocks(p *theme.Palette, ctx BarContext) string {
	color := p.Accent.Hex()
	if ctx.Prefix {
		color = p.Info.Hex()
	}
	var b strings.Builder
	label := ctx.SessionLabel()
	if ctx.ViewportID != "" {
		label += fmt.Sprintf("#[fg=%s]:%s", p.Info.Hex(), ctx.ViewportID)
	}
	fmt.Fprintf(&b, "#[fg=%s,bold] [%s] ", color, label)
	if ctx.Workspace != "" {
		fmt.Fprintf(&b, "#[fg=%s][%s] ", p.Meta.Hex(), ctx.WorkspaceLabel())
	}
	return b.String()
}

func renderRightBlocks(p *theme.Palette, ctx BarContext) string {
	if ctx.Prefix {
		return fmt.Sprintf("#[fg=%s][spc]dash [d]etach [c]tab [?]help ", p.Info.Hex())
	}
	var b strings.Builder
	if ctx.GitBranch != "" {
		color := p.Success.Hex()
		if ctx.GitDirty {
			color = p.Accent.Hex()
		}
		fmt.Fprintf(&b, "#[fg=%s][ %s", color, ctx.GitBranch)
		if ctx.GitDirty {
			b.WriteString("*")
		}
		b.WriteString("] ")
	}
	if ctx.LangVersion != "" {
		fmt.Fprintf(&b, "#[fg=%s][%s%s] ", p.Dim.Hex(), ctx.LangIcon, ctx.LangVersion)
	}
	if ctx.Time != "" {
		fmt.Fprintf(&b, "#[fg=%s][%s] ", p.Muted.Hex(), ctx.Time)
	}
	return b.String()
}

func renderTopBlocks(p *theme.Palette, ctx BarContext) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#[fg=%s,bold] [󱂬 %s] ", p.Special.Hex(), ctx.Workspace)
	for _, sess := range ctx.WorkspaceSessions {
		if sess == ctx.Session {
			fmt.Fprintf(&b, "#[fg=%s,bold][%s]#[nobold] ", p.Accent.Hex(), topSessionLabel(ctx, sess))
		} else {
			fmt.Fprintf(&b, "#[fg=%s][%s] ", p.Dim.Hex(), sess)
		}
	}
	return b.String()
}
