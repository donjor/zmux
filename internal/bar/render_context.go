package bar

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/donjor/zmux/internal/keys"
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
//
// Keys are sourced from the keys registry so changing a bind in one place
// updates the hint surface too — the previous hardcoded "." for rename
// pointed at label-tab (the real rename key is ",").
func prefixHints(p *theme.Palette) string {
	hi := p.Info.Hex()
	dm := p.Dim.Hex()
	pairs := []struct{ key, label string }{
		{shortPrefixKey(keys.Dashboard.Key), "dash"},
		{"d", "etach"}, // tmux built-in, not in our registry
		{keys.NewTab.Key, "tab"},
		{keys.NewSession.Key, "session"},
		{keys.RenameSession.Key, "rename"},
		{keys.LabelTab.Key, "label"},
		{keys.TabKill.Key, "close"},
		{keys.Help.Key, "help"},
	}
	var b strings.Builder
	for _, p := range pairs {
		fmt.Fprintf(&b, "#[fg=%s]%s#[fg=%s]%s ", hi, p.key, dm, p.label)
	}
	return b.String()
}

// shortPrefixKey collapses verbose key names that don't fit the compact hint
// format (e.g. "Space" → "spc"). One-off keys keep their literal spelling.
func shortPrefixKey(k string) string {
	if k == "Space" {
		return "spc"
	}
	return k
}
