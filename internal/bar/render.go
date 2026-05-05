package bar

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/donjor/zmux/internal/theme"
)

// BarContext holds all the dynamic state needed to render the status bar.
type BarContext struct {
	Session        string
	Workspace      string
	WorkspacePos   int // 1-based position of current session in workspace
	WorkspaceCount int // total sessions in workspace
	GroupID        string
	Attached       int
	ViewportID     string // "a" (root), "b", "c" etc. — empty when not grouped
	PaneDir        string
	PaneCmd        string
	Prefix         bool
	GitBranch      string
	GitDirty       bool
	GitAhead       int
	GitBehind      int
	LangVersion    string
	LangIcon       string
	Time           string // formatted clock (e.g. "14:30") — empty when clock disabled
	Date           string // formatted date (e.g. "Apr 07") — empty when clock disabled

	// Session indicator rendered inside the session pill. When non-empty,
	// SessionLabel() appends this instead of "N/M" numbers. Callers set
	// this to compact dots (e.g. "○●○") or leave empty for numbers.
	SessionIndicator string

	// WorkspaceSessions is the list of all session names in the current
	// workspace (ordered). Used by RenderTop to render the session row.
	// Empty when the bar is single-line or the workspace has one session.
	WorkspaceSessions []string

	// Segment visibility (from config).
	ShowWorkspace bool
	ShowGit       bool
	ShowLang      bool
	ShowClock     bool
	ShowDirectory bool
	ShowProcess   bool
	ShowGroup     bool
}

// WorkspaceLabel returns the workspace name. No position indicator —
// that belongs on the session pill (see SessionLabel).
func (ctx BarContext) WorkspaceLabel() string {
	return ctx.Workspace
}

// SessionLabel returns the session name with a position indicator
// when the workspace has multiple sessions. The indicator is either
// compact dots ("main ○●○"), numbers ("main 2/3"), or nothing —
// controlled by the SessionIndicator field.
func (ctx BarContext) SessionLabel() string {
	if ctx.SessionIndicator != "" {
		return ctx.Session + " " + ctx.SessionIndicator
	}
	if ctx.WorkspaceCount > 1 {
		return fmt.Sprintf("%s %d/%d", ctx.Session, ctx.WorkspacePos, ctx.WorkspaceCount)
	}
	return ctx.Session
}

// GatherContext collects all dynamic state.
func GatherContext(sessionName, paneDir, paneCmd, prefixStr, groupID string, workspace string) BarContext {
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
		ctx.GitBranch = gitBranch(paneDir)
		if ctx.GitBranch != "" {
			ctx.GitDirty = gitDirty(paneDir)
			ctx.GitAhead, ctx.GitBehind = gitAheadBehind(paneDir)
		}
	}

	if paneDir != "" {
		ctx.LangIcon, ctx.LangVersion = detectLang(paneDir)
	}

	return ctx
}

// applySegmentVisibility clears context fields for disabled segments.
// This way render functions don't need individual Show* checks —
// disabled segments simply have no data to render.
func applySegmentVisibility(ctx *BarContext) {
	if !ctx.ShowWorkspace {
		ctx.Workspace = ""
	}
	if !ctx.ShowGit {
		ctx.GitBranch = ""
		ctx.GitDirty = false
		ctx.GitAhead = 0
		ctx.GitBehind = 0
	}
	if !ctx.ShowLang {
		ctx.LangVersion = ""
		ctx.LangIcon = ""
	}
	if !ctx.ShowClock {
		ctx.Time = ""
		ctx.Date = ""
	}
	if !ctx.ShowDirectory {
		ctx.PaneDir = ""
	}
	if !ctx.ShowProcess {
		ctx.PaneCmd = ""
	}
	if !ctx.ShowGroup {
		ctx.GroupID = ""
		ctx.Attached = 0
		ctx.ViewportID = ""
	}
}

// RenderLeft generates the left side of the status bar.
func RenderLeft(p *theme.Palette, ctx BarContext, preset Preset) string {
	applySegmentVisibility(&ctx)
	switch preset {
	case Minimal:
		return renderLeftMinimal(p, ctx)
	case Powerline:
		return renderLeftPowerline(p, ctx)
	case Blocks:
		return renderLeftBlocks(p, ctx)
	case Rounded:
		return renderLeftRounded(p, ctx)
	case Hacker:
		return renderLeftHacker(p, ctx)
	case Zen:
		return renderLeftZen(p, ctx)
	case Starship:
		return renderLeftStarship(p, ctx)
	case Rpowerline:
		return renderLeftRpowerline(p, ctx)
	default:
		return renderLeftDefault(p, ctx)
	}
}

// RenderRight generates the right side of the status bar.
func RenderRight(p *theme.Palette, ctx BarContext, preset Preset) string {
	applySegmentVisibility(&ctx)
	switch preset {
	case Minimal:
		return renderRightMinimal(p, ctx)
	case Powerline:
		return renderRightPowerline(p, ctx)
	case Blocks:
		return renderRightBlocks(p, ctx)
	case Rounded:
		return renderRightRounded(p, ctx)
	case Hacker:
		return renderRightHacker(p, ctx)
	case Zen:
		return renderRightZen(p, ctx)
	case Starship:
		return renderRightStarship(p, ctx)
	case Rpowerline:
		return renderRightRpowerline(p, ctx)
	default:
		return renderRightDefault(p, ctx)
	}
}

// RenderTop generates the workspace/session row for two-line status bars.
// Outputs tmux format strings. Returns empty if there's only one session
// (callers should collapse to single-line).
func RenderTop(p *theme.Palette, ctx BarContext, preset Preset) string {
	if len(ctx.WorkspaceSessions) <= 1 {
		return ""
	}
	switch preset {
	case Rpowerline:
		return renderTopRpowerline(p, ctx)
	case Powerline:
		return renderTopPowerline(p, ctx)
	case Rounded:
		return renderTopRounded(p, ctx)
	case Blocks:
		return renderTopBlocks(p, ctx)
	case Hacker:
		return renderTopHacker(p, ctx)
	case Zen:
		return renderTopZen(p, ctx)
	case Minimal:
		return renderTopMinimal(p, ctx)
	case Starship:
		return renderTopStarship(p, ctx)
	default:
		return renderTopDefault(p, ctx)
	}
}

// ── Top row renderers (per preset) ──
//
// Each mirrors the window-tab format from that preset but renders
// sessions instead of windows. The workspace pill leads, followed
// by session tabs, all in tmux format strings.

func renderTopRpowerline(p *theme.Palette, ctx BarContext) string {
	var b strings.Builder
	// Workspace pill.
	fmt.Fprintf(&b, "#[fg=%s]\ue0b6#[bg=%s,fg=%s,bold] 󱂬 %s #[nobold]",
		p.Special.Hex(), p.Special.Hex(), p.BG.Hex(), ctx.Workspace)
	// Arrow from workspace to first session.
	if len(ctx.WorkspaceSessions) > 0 {
		firstBG := p.Dim.Hex()
		if ctx.WorkspaceSessions[0] == ctx.Session {
			firstBG = p.Accent.Hex()
		}
		fmt.Fprintf(&b, "#[fg=%s,bg=%s]\ue0b0", p.Special.Hex(), firstBG)
	} else {
		fmt.Fprintf(&b, "#[fg=%s,bg=default]\ue0b4", p.Special.Hex())
	}
	// Session tabs — two-section pills matching window-tab chrome.
	for i, sess := range ctx.WorkspaceSessions {
		isCurrent := sess == ctx.Session
		idx := i + 1
		if isCurrent {
			fmt.Fprintf(&b, "#[bg=%s,fg=%s,bold] %d\u2009#[fg=%s,bg=%s]\ue0b0#[bg=%s,fg=%s,bold] %s ",
				p.Accent.Hex(), p.BG.Hex(), idx,
				p.Accent.Hex(), p.Surface.Hex(),
				p.Surface.Hex(), p.FG.Hex(), sess)
		} else {
			fmt.Fprintf(&b, "#[bg=%s,fg=%s] %d\u2009#[fg=%s,bg=%s]\ue0b0#[bg=%s,fg=%s] %s ",
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
			fmt.Fprintf(&b, "#[fg=%s,bg=%s]\ue0b0", p.Surface.Hex(), nextBG)
		} else {
			fmt.Fprintf(&b, "#[fg=%s,bg=default]\ue0b4", p.Surface.Hex())
		}
	}
	return b.String()
}

func renderTopPowerline(p *theme.Palette, ctx BarContext) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#[fg=%s,bg=%s]\ue0b0#[bg=%s,fg=%s,bold] 󱂬 %s ",
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
		fmt.Fprintf(&b, "#[fg=%s,bg=%s]\ue0b0", prevBG, bg)
		if isCurrent {
			fmt.Fprintf(&b, "#[bg=%s,fg=%s,bold] %s #[nobold]", bg, p.BG.Hex(), sess)
		} else {
			fmt.Fprintf(&b, "#[bg=%s,fg=%s] %s ", bg, p.Surface.Hex(), sess)
		}
		fmt.Fprintf(&b, "#[fg=%s,bg=%s]\ue0b0#[bg=%s,fg=%s] ",
			bg, p.Surface.Hex(), p.Surface.Hex(), p.Muted.Hex())
		if i == len(ctx.WorkspaceSessions)-1 {
			fmt.Fprintf(&b, "#[fg=%s,bg=default]\ue0b0", p.Surface.Hex())
		}
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
				p.Accent.Hex(), p.Accent.Hex(), p.BG.Hex(), sess, p.Accent.Hex())
		} else {
			fmt.Fprintf(&b, "#[fg=%s]\ue0b6#[bg=%s,fg=%s] %s #[fg=%s,bg=default]\ue0b4 ",
				p.Surface.Hex(), p.Surface.Hex(), p.Dim.Hex(), sess, p.Surface.Hex())
		}
	}
	return b.String()
}

func renderTopBlocks(p *theme.Palette, ctx BarContext) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#[fg=%s,bold] [󱂬 %s] ", p.Special.Hex(), ctx.Workspace)
	for _, sess := range ctx.WorkspaceSessions {
		if sess == ctx.Session {
			fmt.Fprintf(&b, "#[fg=%s,bold][%s]#[nobold] ", p.Accent.Hex(), sess)
		} else {
			fmt.Fprintf(&b, "#[fg=%s][%s] ", p.Dim.Hex(), sess)
		}
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
			fmt.Fprintf(&b, "#[fg=%s,bold]%d:%s#[nobold]", g, i+1, sess)
		} else {
			fmt.Fprintf(&b, "#[fg=%s]%d:%s", d, i+1, sess)
		}
		if i < len(ctx.WorkspaceSessions)-1 {
			fmt.Fprintf(&b, "#[fg=%s]|", d)
		}
	}
	return b.String()
}

func renderTopZen(p *theme.Palette, ctx BarContext) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#[fg=%s]%s ", p.Dim.Hex(), ctx.Workspace)
	for i, sess := range ctx.WorkspaceSessions {
		if sess == ctx.Session {
			fmt.Fprintf(&b, "#[fg=%s]%s", p.Muted.Hex(), sess)
		} else {
			fmt.Fprintf(&b, "#[fg=%s]%s", p.Dim.Hex(), sess)
		}
		if i < len(ctx.WorkspaceSessions)-1 {
			fmt.Fprintf(&b, "#[fg=%s] · ", p.Dim.Hex())
		}
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

func renderTopStarship(p *theme.Palette, ctx BarContext) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#[fg=%s,bold] 󱂬 %s  ", p.Special.Hex(), ctx.Workspace)
	for i, sess := range ctx.WorkspaceSessions {
		if sess == ctx.Session {
			fmt.Fprintf(&b, "#[fg=%s,bold]%s #[fg=%s]❯#[fg=default,nobold] ",
				p.Accent.Hex(), sess, p.Accent.Hex())
		} else {
			fmt.Fprintf(&b, "#[fg=%s]%s ", p.Dim.Hex(), sess)
		}
		if i < len(ctx.WorkspaceSessions)-1 && sess != ctx.Session {
			b.WriteString(" ")
		}
	}
	return b.String()
}

func renderTopDefault(p *theme.Palette, ctx BarContext) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#[fg=%s]\ue0b6#[bg=%s,fg=%s,bold] 󱂬 %s #[fg=%s,bg=default]\ue0b4 ",
		p.Special.Hex(), p.Special.Hex(), p.BG.Hex(), ctx.Workspace, p.Special.Hex())
	for _, sess := range ctx.WorkspaceSessions {
		if sess == ctx.Session {
			fmt.Fprintf(&b, "#[fg=%s,bold] %s #[nobold,fg=%s]", p.Accent.Hex(), sess, p.Muted.Hex())
		} else {
			fmt.Fprintf(&b, "#[fg=%s] %s ", p.Dim.Hex(), sess)
		}
	}
	return b.String()
}

// ── Prefix hints (shared) ──

func prefixHints(p *theme.Palette) string {
	hi := p.Info.Hex()
	dm := p.Dim.Hex()
	return fmt.Sprintf(
		"#[fg=%s]spc#[fg=%s]dash #[fg=%s]d#[fg=%s]etach #[fg=%s]c#[fg=%s]tab #[fg=%s]C#[fg=%s]session #[fg=%s].#[fg=%s]rename #[fg=%s]x#[fg=%s]close #[fg=%s]?#[fg=%s]help ",
		hi, dm, hi, dm, hi, dm, hi, dm, hi, dm, hi, dm, hi, dm,
	)
}

// formatGitText returns the plain git status string (icon + branch + dirty
// marker + ahead/behind counts) used in preset pills. No tmux styling —
// callers wrap this in their own pill chrome.
//
// Returns "" if no branch. Space separator before ahead/behind is caller's
// choice; most presets omit it.
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

// ══════════════════════════════════════════════════════════════
// DEFAULT — catppuccin-inspired: rounded pills, icons, elevated surfaces
// ══════════════════════════════════════════════════════════════

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

// ══════════════════════════════════════════════════════════════
// MINIMAL — clean, barely decorated, content-first
// ══════════════════════════════════════════════════════════════

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

// ══════════════════════════════════════════════════════════════
// POWERLINE — tokyo-night inspired: crisp separators, cool precision
// ══════════════════════════════════════════════════════════════

func renderLeftPowerline(p *theme.Palette, ctx BarContext) string {
	var b strings.Builder
	bg := p.Accent.Hex()
	if ctx.Prefix {
		bg = p.Info.Hex()
	}

	// Workspace → Session → Dir chain.
	if ctx.Workspace != "" {
		fmt.Fprintf(&b, "#[bg=%s,fg=%s,bold] 󱂬 %s ", p.Special.Hex(), p.BG.Hex(), ctx.WorkspaceLabel())
		fmt.Fprintf(&b, "#[fg=%s,bg=%s]\ue0b0", p.Special.Hex(), bg)
	}

	fmt.Fprintf(&b, "#[bg=%s,fg=%s,bold]  %s ", bg, p.BG.Hex(), ctx.SessionLabel())
	if ctx.ViewportID != "" {
		fmt.Fprintf(&b, "#[fg=%s,bg=%s]\ue0b0", bg, p.Info.Hex())
		fmt.Fprintf(&b, "#[bg=%s,fg=%s,bold] %s ", p.Info.Hex(), p.BG.Hex(), ctx.ViewportID)
		fmt.Fprintf(&b, "#[fg=%s,bg=%s]\ue0b0", p.Info.Hex(), p.Surface.Hex())
	} else {
		fmt.Fprintf(&b, "#[fg=%s,bg=%s]\ue0b0", bg, p.Surface.Hex())
	}

	dir := shortenDir(ctx.PaneDir)
	if dir != "" {
		fmt.Fprintf(&b, "#[bg=%s,fg=%s]  %s ", p.Surface.Hex(), p.Muted.Hex(), dir)
	}
	fmt.Fprintf(&b, "#[fg=%s,bg=default]\ue0b0 ", p.Surface.Hex())

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
		fmt.Fprintf(&b, "#[fg=%s]\ue0b2", gitBg)
		fmt.Fprintf(&b, "#[bg=%s,fg=%s,bold] %s ", gitBg, p.BG.Hex(), formatGitText(ctx))
	}

	// Time.
	if ctx.Time != "" {
		fmt.Fprintf(&b, "#[fg=%s]\ue0b2", p.Surface.Hex())
		fmt.Fprintf(&b, "#[bg=%s,fg=%s] 󱑍 %s ", p.Surface.Hex(), p.Muted.Hex(), ctx.Time)
	}

	// Date accent.
	if ctx.Date != "" {
		fmt.Fprintf(&b, "#[fg=%s]\ue0b2", p.Accent.Hex())
		fmt.Fprintf(&b, "#[bg=%s,fg=%s,bold] %s ", p.Accent.Hex(), p.BG.Hex(), ctx.Date)
	}

	return b.String()
}

// ══════════════════════════════════════════════════════════════
// BLOCKS — square brackets, monospace, dense
// ══════════════════════════════════════════════════════════════

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

// ══════════════════════════════════════════════════════════════
// ROUNDED — elevated pill segments, catppuccin-style, premium feel
// ══════════════════════════════════════════════════════════════

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

// ══════════════════════════════════════════════════════════════
// HACKER — matrix-inspired, monospace, dense info, green on dark
// ══════════════════════════════════════════════════════════════

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

// ══════════════════════════════════════════════════════════════
// ZEN — ultra-minimal, barely there, content whispers
// ══════════════════════════════════════════════════════════════

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

// ══════════════════════════════════════════════════════════════
// STARSHIP — colorful prompt-inspired, each segment its own color
// ══════════════════════════════════════════════════════════════

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
	if ctx.PaneCmd != "" && ctx.PaneCmd != "bash" && ctx.PaneCmd != "zsh" && ctx.PaneCmd != "fish" {
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

// ══════════════════════════════════════════════════════════════
// RPOWERLINE — rounded powerline: filled segments with rounded caps
// ══════════════════════════════════════════════════════════════

func renderLeftRpowerline(p *theme.Palette, ctx BarContext) string {
	var b strings.Builder
	bg := p.Accent.Hex()
	if ctx.Prefix {
		bg = p.Info.Hex()
	}

	// Workspace → Session → Dir chain with rounded caps.
	if ctx.Workspace != "" {
		fmt.Fprintf(&b, "#[fg=%s]\ue0b6#[bg=%s,fg=%s,bold] 󱂬 %s ", p.Special.Hex(), p.Special.Hex(), p.BG.Hex(), ctx.WorkspaceLabel())
		fmt.Fprintf(&b, "#[fg=%s,bg=%s]\ue0b0", p.Special.Hex(), bg)
	} else {
		fmt.Fprintf(&b, "#[fg=%s]\ue0b6", bg)
	}

	fmt.Fprintf(&b, "#[bg=%s,fg=%s,bold]  %s ", bg, p.BG.Hex(), ctx.SessionLabel())
	if ctx.ViewportID != "" {
		fmt.Fprintf(&b, "#[fg=%s,bg=%s]\ue0b0", bg, p.Info.Hex())
		fmt.Fprintf(&b, "#[bg=%s,fg=%s,bold] %s ", p.Info.Hex(), p.BG.Hex(), ctx.ViewportID)
		fmt.Fprintf(&b, "#[fg=%s,bg=%s]\ue0b0", p.Info.Hex(), p.Surface.Hex())
	} else {
		fmt.Fprintf(&b, "#[fg=%s,bg=%s]\ue0b0", bg, p.Surface.Hex())
	}

	dir := shortenDir(ctx.PaneDir)
	if dir != "" {
		fmt.Fprintf(&b, "#[bg=%s,fg=%s]  %s ", p.Surface.Hex(), p.Muted.Hex(), dir)
	}
	fmt.Fprintf(&b, "#[fg=%s,bg=default]\ue0b4 ", p.Surface.Hex())

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
			fmt.Fprintf(&b, "#[fg=%s]\ue0b6", s.bg)
		}
		// Pill body.
		boldTag := ""
		if s.bold {
			boldTag = ",bold"
		}
		fmt.Fprintf(&b, "#[bg=%s,fg=%s%s] %s ", s.bg, p.BG.Hex(), boldTag, s.text)
		// Transition: pointed arrow into next segment, or rounded cap to end.
		if i+1 < len(segs) {
			fmt.Fprintf(&b, "#[fg=%s,bg=%s]\ue0b0", s.bg, segs[i+1].bg)
		} else {
			fmt.Fprintf(&b, "#[fg=%s,bg=default]\ue0b4", s.bg)
		}
	}

	return b.String()
}

// ══════════════════════════════════════════════════════════════
// Helpers
// ══════════════════════════════════════════════════════════════

func gitBranch(dir string) string {
	out, err := exec.Command("git", "-C", dir, "branch", "--show-current").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func gitDirty(dir string) bool {
	out, err := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

func gitAheadBehind(dir string) (ahead, behind int) {
	out, err := exec.Command("git", "-C", dir, "rev-list", "--left-right", "--count", "HEAD...@{upstream}").Output()
	if err != nil {
		return 0, 0
	}
	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) == 2 {
		fmt.Sscanf(parts[0], "%d", &ahead)
		fmt.Sscanf(parts[1], "%d", &behind)
	}
	return
}

func detectLang(dir string) (icon, version string) {
	if exists(filepath.Join(dir, "go.mod")) {
		out, err := exec.Command("go", "version").Output()
		if err == nil {
			parts := strings.Fields(string(out))
			if len(parts) >= 3 {
				return " ", strings.TrimPrefix(parts[2], "go")
			}
		}
	}
	if exists(filepath.Join(dir, "package.json")) {
		out, err := exec.Command("node", "-v").Output()
		if err == nil {
			return " ", strings.TrimSpace(strings.TrimPrefix(string(out), "v"))
		}
	}
	if exists(filepath.Join(dir, "Cargo.toml")) {
		out, err := exec.Command("rustc", "--version").Output()
		if err == nil {
			parts := strings.Fields(string(out))
			if len(parts) >= 2 {
				return " ", parts[1]
			}
		}
	}
	if exists(filepath.Join(dir, "requirements.txt")) || exists(filepath.Join(dir, "pyproject.toml")) {
		out, err := exec.Command("python3", "--version").Output()
		if err == nil {
			parts := strings.Fields(string(out))
			if len(parts) >= 2 {
				return " ", parts[1]
			}
		}
	}
	return "", ""
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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
