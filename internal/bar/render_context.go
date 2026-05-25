package bar

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/donjor/zmux/internal/theme"
)

// GatherContext collects all dynamic state for the status bar. The prober
// supplies VCS/language state behind an interface so the side-effects are
// injectable (pass bar.ExecProber{} in production).
func GatherContext(prober Prober, sessionName, paneDir, paneCmd, prefixStr, groupID string, workspace string) BarContext {
	now := time.Now()
	ctx := BarContext{
		Session:   sessionName,
		Workspace: workspace,
		GroupID:   groupID,
		PaneDir:   paneDir,
		PaneCmd:   paneCmd,
		Prefix:    prefixStr == "1",
		Time:      now.Format("15:04"),
		Date:      now.Format("Jan 02"),
	}

	if paneDir != "" {
		ctx.GitBranch = prober.GitBranch(paneDir)
		if ctx.GitBranch != "" {
			ctx.GitDirty = prober.GitDirty(paneDir)
			ctx.GitAhead, ctx.GitBehind = prober.GitAheadBehind(paneDir)
		}
	}

	if paneDir != "" {
		ctx.LangIcon, ctx.LangVersion = prober.DetectLang(paneDir)
	}

	return ctx
}

// formatGitText returns the plain git status string (icon + branch + dirty
// marker + ahead/behind counts) used in preset pills. No tmux styling —
// callers wrap this in their own pill chrome.
func formatGitText(ctx BarContext) string {
	if ctx.GitBranch == "" {
		return ""
	}
	text := "󰘬 " + ctx.GitBranch
	if ctx.GitDirty {
		text += "*"
	}
	if ctx.GitAhead > 0 {
		text += fmt.Sprintf(" ↑%d", ctx.GitAhead)
	}
	if ctx.GitBehind > 0 {
		text += fmt.Sprintf(" ↓%d", ctx.GitBehind)
	}
	return text
}

func shortenDir(dir string) string {
	if dir == "" {
		return ""
	}
	home, _ := os.UserHomeDir()
	if home != "" && strings.HasPrefix(dir, home) {
		dir = "~" + dir[len(home):]
	}
	parts := strings.Split(dir, "/")
	if len(parts) > 3 {
		dir = "…/" + strings.Join(parts[len(parts)-2:], "/")
	}
	return dir
}

// prefixHints renders the shared prefix-active hint string used by several presets.
func prefixHints(p *theme.Palette) string {
	hi := p.Info.Hex()
	dm := p.Dim.Hex()
	return fmt.Sprintf(
		"#[fg=%s]spc#[fg=%s]dash #[fg=%s]d#[fg=%s]etach #[fg=%s]c#[fg=%s]tab #[fg=%s]C#[fg=%s]session #[fg=%s].#[fg=%s]rename #[fg=%s]x#[fg=%s]close #[fg=%s]?#[fg=%s]help ",
		hi, dm, hi, dm, hi, dm, hi, dm, hi, dm, hi, dm, hi, dm,
	)
}
