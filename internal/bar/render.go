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
	PaneDir        string
	PaneCmd        string
	Prefix         bool
	GitBranch      string
	GitDirty       bool
	GitAhead       int
	GitBehind      int
	LangVersion    string
	LangIcon       string
	Time           string

	// Segment visibility (from config).
	ShowWorkspace bool
	ShowGit       bool
	ShowLang      bool
	ShowClock     bool
	ShowDirectory bool
	ShowProcess   bool
	ShowGroup     bool
}

// WorkspaceLabel returns the formatted workspace display string.
// e.g., "myapp 2/4" for multi-session, "myapp" for single-session.
func (ctx BarContext) WorkspaceLabel() string {
	if ctx.Workspace == "" {
		return ""
	}
	if ctx.WorkspaceCount > 1 {
		return fmt.Sprintf("%s %d/%d", ctx.Workspace, ctx.WorkspacePos, ctx.WorkspaceCount)
	}
	return ctx.Workspace
}

// GatherContext collects all dynamic state.
func GatherContext(sessionName, paneDir, paneCmd, prefixStr, groupID string, workspace string) BarContext {
	ctx := BarContext{
		Session:   sessionName,
		Workspace: workspace,
		GroupID:   groupID,
		PaneDir:   paneDir,
		PaneCmd:   paneCmd,
		Prefix:    prefixStr == "1",
		Time:      time.Now().Format("15:04"),
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

// ── Prefix hints (shared) ──

func prefixHints(p *theme.Palette) string {
	hi := p.Info.Hex()
	dm := p.Dim.Hex()
	return fmt.Sprintf(
		"#[fg=%s]spc#[fg=%s]dash #[fg=%s]d#[fg=%s]etach #[fg=%s]c#[fg=%s]tab #[fg=%s]x#[fg=%s]close #[fg=%s]?#[fg=%s]help ",
		hi, dm, hi, dm, hi, dm, hi, dm, hi, dm,
	)
}

// ── Git segment builder (shared) ──

func gitSegment(p *theme.Palette, ctx BarContext) string {
	if ctx.GitBranch == "" {
		return ""
	}
	var b strings.Builder
	branchColor := p.Success.Hex()
	if ctx.GitDirty {
		branchColor = p.Accent.Hex()
	}
	fmt.Fprintf(&b, "#[fg=%s] %s", branchColor, ctx.GitBranch)
	if ctx.GitDirty {
		fmt.Fprintf(&b, "#[fg=%s]*", p.Accent.Hex())
	}
	if ctx.GitAhead > 0 {
		fmt.Fprintf(&b, "#[fg=%s]↑%d", p.Success.Hex(), ctx.GitAhead)
	}
	if ctx.GitBehind > 0 {
		fmt.Fprintf(&b, "#[fg=%s]↓%d", p.Error.Hex(), ctx.GitBehind)
	}
	return b.String()
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
	fmt.Fprintf(&b, "#[fg=%s]\ue0b6#[bg=%s,fg=%s,bold] 󱂬 %s #[fg=%s,bg=default]\ue0b4 ",
		bg, bg, p.BG.Hex(), ctx.Session, bg)

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
		gitText := fmt.Sprintf("󰘬 %s", ctx.GitBranch)
		if ctx.GitDirty {
			gitText += "*"
		}
		if ctx.GitAhead > 0 {
			gitText += fmt.Sprintf("↑%d", ctx.GitAhead)
		}
		fmt.Fprintf(&b, "#[fg=%s]\ue0b6#[bg=%s,fg=%s,bold] %s #[fg=%s,bg=default]\ue0b4 ",
			gitBg, gitBg, p.BG.Hex(), gitText, gitBg)
	}

	// Lang pill (surface bg).
	if ctx.LangVersion != "" {
		fmt.Fprintf(&b, "#[fg=%s]\ue0b6#[bg=%s,fg=%s] %s%s #[fg=%s,bg=default]\ue0b4 ",
			sf, sf, p.Muted.Hex(), ctx.LangIcon, ctx.LangVersion, sf)
	}

	// Time pill.
	fmt.Fprintf(&b, "#[fg=%s]\ue0b6#[bg=%s,fg=%s] 󱑍 %s #[fg=%s,bg=default]\ue0b4",
		sf, sf, p.Muted.Hex(), ctx.Time, sf)

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
	out := fmt.Sprintf("#[fg=%s,bold] %s ", color, ctx.Session)
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
	fmt.Fprintf(&b, "#[fg=%s]%s ", p.Dim.Hex(), ctx.Time)
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

	fmt.Fprintf(&b, "#[bg=%s,fg=%s,bold]  %s ", bg, p.BG.Hex(), ctx.Session)
	fmt.Fprintf(&b, "#[fg=%s,bg=%s]\ue0b0", bg, p.Surface.Hex())

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
		gitText := fmt.Sprintf("󰘬 %s", ctx.GitBranch)
		if ctx.GitDirty {
			gitText += "*"
		}
		if ctx.GitAhead > 0 {
			gitText += fmt.Sprintf(" ↑%d", ctx.GitAhead)
		}
		fmt.Fprintf(&b, "#[bg=%s,fg=%s,bold] %s ", gitBg, p.BG.Hex(), gitText)
	}

	// Time.
	fmt.Fprintf(&b, "#[fg=%s]\ue0b2", p.Surface.Hex())
	fmt.Fprintf(&b, "#[bg=%s,fg=%s] 󱑍 %s ", p.Surface.Hex(), p.Muted.Hex(), ctx.Time)

	// Date accent.
	date := time.Now().Format("Jan 02")
	fmt.Fprintf(&b, "#[fg=%s]\ue0b2", p.Accent.Hex())
	fmt.Fprintf(&b, "#[bg=%s,fg=%s,bold] %s ", p.Accent.Hex(), p.BG.Hex(), date)

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
	fmt.Fprintf(&b, "#[fg=%s,bold] [%s] ", color, ctx.Session)
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
	fmt.Fprintf(&b, "#[fg=%s][%s] ", p.Muted.Hex(), ctx.Time)
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

	// Session pill.
	fmt.Fprintf(&b, "#[fg=%s]\ue0b6#[bg=%s,fg=%s,bold] 󱂬 %s #[fg=%s,bg=default]\ue0b4 ",
		bg, bg, p.BG.Hex(), ctx.Session, bg)

	// Workspace pill on surface.
	if ctx.Workspace != "" {
		fmt.Fprintf(&b, "#[fg=%s]\ue0b6#[bg=%s,fg=%s]  %s #[fg=%s,bg=default]\ue0b4 ",
			sf, sf, p.Special.Hex(), ctx.WorkspaceLabel(), sf)
	}

	// Group indicator pill.
	if ctx.GroupID != "" && ctx.Attached > 1 {
		fmt.Fprintf(&b, "#[fg=%s]\ue0b6#[bg=%s,fg=%s] %d◉ #[fg=%s,bg=default]\ue0b4 ",
			sf, sf, p.Dim.Hex(), ctx.Attached, sf)
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
		gitText := fmt.Sprintf("󰘬 %s", ctx.GitBranch)
		if ctx.GitDirty {
			gitText += "*"
		}
		if ctx.GitAhead > 0 {
			gitText += fmt.Sprintf("↑%d", ctx.GitAhead)
		}
		fmt.Fprintf(&b, "#[fg=%s]\ue0b6#[bg=%s,fg=%s,bold] %s #[fg=%s,bg=default]\ue0b4 ",
			gitBg, gitBg, p.BG.Hex(), gitText, gitBg)
	}

	// Lang pill.
	if ctx.LangVersion != "" {
		fmt.Fprintf(&b, "#[fg=%s]\ue0b6#[bg=%s,fg=%s] %s%s #[fg=%s,bg=default]\ue0b4 ",
			sf, sf, p.Muted.Hex(), ctx.LangIcon, ctx.LangVersion, sf)
	}

	// Time pill.
	fmt.Fprintf(&b, "#[fg=%s]\ue0b6#[bg=%s,fg=%s] 󱑍 %s #[fg=%s,bg=default]\ue0b4",
		sf, sf, p.Muted.Hex(), ctx.Time, sf)

	return b.String()
}

// ══════════════════════════════════════════════════════════════
// HACKER — matrix-inspired, monospace, dense info, green on dark
// ══════════════════════════════════════════════════════════════

func renderLeftHacker(p *theme.Palette, ctx BarContext) string {
	var b strings.Builder
	g := p.Success.Hex()
	d := p.Dim.Hex()

	fmt.Fprintf(&b, "#[fg=%s,bold]%s", g, ctx.Session)
	if ctx.Workspace != "" {
		fmt.Fprintf(&b, "#[fg=%s]@#[fg=%s]%s", d, g, ctx.WorkspaceLabel())
	}
	if ctx.GroupID != "" {
		fmt.Fprintf(&b, "#[fg=%s]:%d", d, ctx.Attached)
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

	fmt.Fprintf(&b, "#[fg=%s]%s ", g, time.Now().Format("15:04:05"))

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
	out := fmt.Sprintf("#[fg=%s] %s", color, ctx.Session)
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
	parts = append(parts, ctx.Time)
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

	// Session with chevron.
	fmt.Fprintf(&b, "#[fg=%s,bold]  %s #[fg=%s]❯#[fg=default] ", bg, ctx.Session, bg)

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
	fmt.Fprintf(&b, "#[fg=%s]󱑍 %s ", p.Muted.Hex(), ctx.Time)

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

	fmt.Fprintf(&b, "#[bg=%s,fg=%s,bold]  %s ", bg, p.BG.Hex(), ctx.Session)
	fmt.Fprintf(&b, "#[fg=%s,bg=%s]\ue0b0", bg, p.Surface.Hex())

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
	var b strings.Builder

	// Git.
	if ctx.GitBranch != "" {
		gitBg := p.Success.Hex()
		if ctx.GitDirty {
			gitBg = p.Accent.Hex()
		}
		fmt.Fprintf(&b, "#[fg=%s]\ue0b6", gitBg)
		gitText := fmt.Sprintf("󰘬 %s", ctx.GitBranch)
		if ctx.GitDirty {
			gitText += "*"
		}
		if ctx.GitAhead > 0 {
			gitText += fmt.Sprintf(" ↑%d", ctx.GitAhead)
		}
		fmt.Fprintf(&b, "#[bg=%s,fg=%s,bold] %s ", gitBg, p.BG.Hex(), gitText)
		fmt.Fprintf(&b, "#[fg=%s,bg=%s]\ue0b0", gitBg, p.Surface.Hex())
	} else {
		fmt.Fprintf(&b, "#[fg=%s]\ue0b6", p.Surface.Hex())
	}

	// Time.
	fmt.Fprintf(&b, "#[bg=%s,fg=%s] 󱑍 %s ", p.Surface.Hex(), p.Muted.Hex(), ctx.Time)
	fmt.Fprintf(&b, "#[fg=%s,bg=%s]\ue0b0", p.Surface.Hex(), p.Accent.Hex())

	// Date.
	date := time.Now().Format("Jan 02")
	fmt.Fprintf(&b, "#[bg=%s,fg=%s,bold] %s ", p.Accent.Hex(), p.BG.Hex(), date)
	fmt.Fprintf(&b, "#[fg=%s,bg=default]\ue0b4", p.Accent.Hex())

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
