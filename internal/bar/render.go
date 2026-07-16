// Package bar generates tmux status bar configurations from presets.
//
// The render layer is the public surface most callers interact with:
//
//   - [BarContext] holds the dynamic state needed to render the bar.
//   - [GatherContext] (in render_context.go) collects that state from the
//     environment (git, time, language detection).
//   - [RenderLeft], [RenderRight], [RenderTop] are the public entry points
//     that dispatch to the per-preset renderers in render_<preset>.go.
//
// Preset-specific rendering lives one file per preset (render_default.go,
// render_powerline.go, etc.). The dispatchers in this file are the only
// place that knows about the full set of presets.
package bar

import (
	"fmt"
	"strings"

	"github.com/donjor/zmux/internal/tabs"
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
	WindowPanes    int // #{window_panes}: pane count of the current window (0 if unknown)
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

	// WorkspaceSessionStates is index-aligned with WorkspaceSessions and
	// carries the attach state of each sibling. May be nil (treated as
	// all Unknown). See [AttachState] for the enum; populated by the
	// bar-render data path from the live tmux session list.
	WorkspaceSessionStates []AttachState

	// TopRowActive signals two-line mode, where the top row owns the
	// workspace/session identity. When set, RenderLeft drops all identity
	// chrome (pills, viewport, powerline chain) and renders only compact aux
	// that does not include cwd via renderLeftAux — see plan 024.
	TopRowActive bool

	// Segment visibility (from config).
	ShowWorkspace bool
	ShowGit       bool
	ShowLang      bool
	ShowClock     bool
	ShowDirectory bool
	ShowProcess   bool
	ShowGroup     bool
}

// Split reports whether the current window holds more than one pane, i.e. the
// status bar is rendering for a split tab. WindowPanes is 0 when the count
// wasn't passed (e.g. dashboard preview), which reads as not-split.
func (ctx BarContext) Split() bool {
	return ctx.WindowPanes > 1
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

// topSessionLabel returns the session name as it appears in the top status
// row. For the *current* session of a grouped clone it appends a compact
// viewport suffix (e.g. "dev·b"), so the clone letter shows exactly once on the
// top row and never on the bottom-left (plan 024, two-line row ownership).
// Gated on ShowGroup — applySegmentVisibility does not run for the top row, so
// the check is explicit here. Returns the bare name in every other case.
func topSessionLabel(ctx BarContext, sess string) string {
	if sess == ctx.Session && ctx.ViewportID != "" && ctx.ShowGroup {
		return sess + "·" + ctx.ViewportID
	}
	return sess
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
	if ctx.TopRowActive {
		return renderLeftAux(p, ctx, preset)
	}
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

// renderLeftAux renders the compact bottom-left for two-line mode
// (TopRowActive): the top row now owns workspace/session identity, so the
// bottom-left drops all identity chrome (pills, viewport, powerline chain) and
// keeps only volatile aux that cannot shift as cwd changes. Directory moved to
// the top-row overlay because this bottom-left shares the logical tabs row: when
// cwd width changed, tab cells jumped/truncated. Field policy now mirrors only
// stable-enough aux: hacker → process; starship → git; every other preset →
// empty. Plain dim text, no caps — the bottom-left is secondary chrome here
// (plan 024).
//
// Callers must run applySegmentVisibility first so segment toggles still gate
// these fields (RenderLeft does).
func renderLeftAux(p *theme.Palette, ctx BarContext, preset Preset) string {
	proc := ctx.PaneCmd
	if tabs.IsIdleShell(proc) {
		proc = ""
	}

	var parts []string
	add := func(hex, text string) {
		if text != "" {
			parts = append(parts, fmt.Sprintf("#[fg=%s]%s", hex, text))
		}
	}

	switch preset {
	case Hacker:
		add(p.Success.Hex(), proc)
	case Starship:
		add(p.Success.Hex(), formatGitText(ctx))
	default:
		return ""
	}

	if len(parts) == 0 {
		return ""
	}
	return " " + strings.Join(parts, " ") + " "
}

// EdgeBadge renders a leading status-bar tag that marks a non-default profile
// (e.g. the zzmux edge binary), so it's obvious at a glance you're not on the
// stable zmux. Glyph-free + preset-agnostic: distinctness comes from the Error
// colour, so it composes cleanly in front of any preset's first pill.
func EdgeBadge(p *theme.Palette, label string) string {
	if label == "" {
		return ""
	}
	return fmt.Sprintf("#[bg=%s,fg=%s,bold] %s #[nobold,bg=default,fg=default] ",
		p.Error.Hex(), p.BG.Hex(), label)
}

// DirectoryBadge renders the shortened current pane cwd for the top-row
// overlay. It intentionally stays out of status-left in two-line mode so cwd
// changes cannot steal width from the logical tabs row.
func DirectoryBadge(p *theme.Palette, dir string) string {
	dir = shortenDir(dir)
	if dir == "" {
		return ""
	}
	return fmt.Sprintf("#[bg=%s,fg=%s] %s #[bg=default,fg=default] ",
		p.Surface.Hex(), p.Muted.Hex(), dir)
}

// RenderTopOverlay renders right-aligned top-row chrome that should not affect
// the bottom tabs row. Today that is cwd plus the non-default profile badge.
// One align directive owns the whole cluster so cwd and zzmux never fight for
// the right edge.
func RenderTopOverlay(p *theme.Palette, ctx BarContext, profileName string) string {
	applySegmentVisibility(&ctx)

	var parts []string
	if badge := DirectoryBadge(p, ctx.PaneDir); badge != "" {
		parts = append(parts, badge)
	}
	if profileName != "" && profileName != "zmux" {
		parts = append(parts, EdgeBadge(p, profileName))
	}
	if len(parts) == 0 {
		return ""
	}
	return "#[align=right]" + strings.Join(parts, "")
}

// RenderRight generates the right side of the status bar. Pane labels/details
// live in pane-border-format for both single-pane and split windows, so this
// stays focused on volatile right-side status (prefix hints, git/lang/clock).
func RenderRight(p *theme.Palette, ctx BarContext, preset Preset) string {
	applySegmentVisibility(&ctx)
	return renderRightPreset(p, ctx, preset)
}

func renderRightPreset(p *theme.Palette, ctx BarContext, preset Preset) string {
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
// Outputs tmux format strings. Renders for a single session too (always-2-line,
// plan 024); returns empty only when there are no sessions at all, to avoid an
// orphaned workspace pill.
func RenderTop(p *theme.Palette, ctx BarContext, preset Preset) string {
	if len(ctx.WorkspaceSessions) == 0 {
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
