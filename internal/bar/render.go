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
