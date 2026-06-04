// Package bar is the status-bar preview page for the uiproto harness.
// It drives real internal/bar rendering for the "classic" mode and
// draft rendering (internal/preview/bar/draft) for the new modes we
// iterate on during Phase 0.
package bar

import (
	"fmt"
	"io"
	"strings"

	"github.com/donjor/zmux/internal/bar"
	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/preview"
	"github.com/donjor/zmux/internal/preview/bar/draft"
	"github.com/donjor/zmux/internal/theme"
)

const (
	ctlLayout       preview.ControlID = "layout"
	ctlVariant      preview.ControlID = "variant"
	ctlIndicator    preview.ControlID = "indicator"
	ctlPreset       preview.ControlID = "preset"
	ctlSessionCount preview.ControlID = "sessions"
	ctlCurrentIdx   preview.ControlID = "current"
	ctlNameSet      preview.ControlID = "names"
	ctlShowClones   preview.ControlID = "show-clones"
	ctlShowWS       preview.ControlID = "seg-workspace"
	ctlShowGit      preview.ControlID = "seg-git"
	ctlShowClock    preview.ControlID = "seg-clock"
)

// layouts control structure (how many rows).
var layouts = []string{draft.LayoutSingle, draft.LayoutTwoLine, draft.LayoutSplit}

// topBarVariants control how the workspace/session top row renders.
var topBarVariants = []string{draft.TopBarMinimal, draft.TopBarTabs, draft.TopBarDots}

// indicators control what goes inside the session name pill on the
// main bar — independent of variant.
var indicators = []string{draft.IndicatorNone, draft.IndicatorNumbers, draft.IndicatorDots}

var presetNames = []string{
	"default", "minimal", "powerline", "blocks", "rounded",
	"hacker", "zen", "starship", "rpowerline",
}

// Page is the bar preview page.
type Page struct {
	ctrls []preview.Control

	// Resolver used to load palettes for each preset choice.
	resolver *theme.Resolver
}

// New builds a bar preview page with default controls.
func New() *Page {
	names := FixtureNames()

	p := &Page{
		resolver: theme.NewResolver(config.RealFS{}, "", ""),
	}
	p.ctrls = []preview.Control{
		preview.NewChoice(ctlPreset, "preset", presetNames, "rpowerline"),
		preview.NewChoice(ctlLayout, "layout", layouts, draft.LayoutTwoLine),
		preview.NewChoice(ctlVariant, "top bar", topBarVariants, draft.TopBarTabs),
		preview.NewChoice(ctlIndicator, "indicator", indicators, draft.IndicatorDots),
		preview.NewInt(ctlSessionCount, "sessions", 1, 9, 3, "total"),
		preview.NewInt(ctlCurrentIdx, "current", 1, 9, 2, "of N"),
		preview.NewChoice(ctlNameSet, "names", names, "realistic"),
		preview.NewToggle(ctlShowClones, "clones", false),
		preview.NewToggle(ctlShowWS, "show ws", true),
		preview.NewToggle(ctlShowGit, "show git", true),
		preview.NewToggle(ctlShowClock, "show clock", true),
	}
	return p
}

// ── preview.Page ──

func (p *Page) ID() string    { return "bar" }
func (p *Page) Title() string { return "Status Bar" }

func (p *Page) Controls() []preview.Control { return p.ctrls }

func (p *Page) Render(ctx preview.RenderContext) string {
	layout, _ := ctx.Values[ctlLayout].(string)
	variant, _ := ctx.Values[ctlVariant].(string)
	indicator, _ := ctx.Values[ctlIndicator].(string)
	presetName, _ := ctx.Values[ctlPreset].(string)
	sessCount, _ := ctx.Values[ctlSessionCount].(int)
	currentIdx, _ := ctx.Values[ctlCurrentIdx].(int)
	nameSet, _ := ctx.Values[ctlNameSet].(string)
	showClones, _ := ctx.Values[ctlShowClones].(bool)

	if currentIdx > sessCount {
		currentIdx = sessCount
	}

	palette := p.loadPalette()
	preset, err := bar.PresetFromString(presetName)
	if err != nil {
		return dim("unknown preset: " + presetName)
	}

	names := Fixtures[nameSet]
	if names == nil {
		names = Fixtures["realistic"]
	}
	sessions := buildSessions(names, sessCount, currentIdx, showClones)

	header := fmt.Sprintf("  layout=%s  top=%s  indicator=%s  preset=%s  %d session(s)  current=%s",
		layout, variant, indicator, presetName, len(sessions), safeName(sessions, currentIdx-1))
	header = dim(header)

	// Frame width: leave a couple cols of margin so the bar reads as
	// a discrete strip rather than edge-to-edge. Preserves contrast.
	frameWidth := ctx.Width - 4
	if frameWidth < 40 {
		frameWidth = ctx.Width
	}

	// Build segment visibility from controls.
	segWS, _ := ctx.Values[ctlShowWS].(bool)
	segGit, _ := ctx.Values[ctlShowGit].(bool)
	segClock, _ := ctx.Values[ctlShowClock].(bool)
	segments := config.BarSegments{
		Workspace: segWS,
		Git:       segGit,
		Lang:      true,
		Clock:     segClock,
		Directory: true,
		Process:   true,
		Group:     false,
	}

	// Build session name list for the real RenderTop.
	sessionNames := make([]string, len(sessions))
	for i, s := range sessions {
		sessionNames[i] = s.Name
	}
	currentName := safeName(sessions, currentIdx-1)

	// Bar override: apply the session indicator (dots/numbers/none)
	// inside the session pill.
	barOverride := func(bctx *bar.BarContext) {
		switch indicator {
		case draft.IndicatorDots:
			bctx.SessionIndicator = draft.RenderDotsPlain(sessions, currentIdx)
		case draft.IndicatorNone:
			bctx.WorkspaceCount = 1 // suppress N/M
		}
		// IndicatorNumbers: leave default (SessionLabel adds N/M)
	}

	switch layout {
	case draft.LayoutSingle:
		barRow := bar.RenderBarPreviewOverride(preset, palette, segments, frameWidth, barOverride)
		return header + "\n\n" +
			barRow + "\n" +
			mockEditorContent(palette, frameWidth)

	case draft.LayoutTwoLine, draft.LayoutSplit:
		// Top row: depends on variant.
		topRow := renderTopRow(variant, preset, palette, sessionNames, currentName, frameWidth)

		// Bottom row: aux-only — the top row owns identity in two-line mode
		// (plan 024), so flag TopRowActive so the preview matches live render.
		noWS := segments
		noWS.Workspace = false
		bottomRow := bar.RenderBarPreviewOverride(preset, palette, noWS, frameWidth, func(bctx *bar.BarContext) {
			bctx.TopRowActive = true
			barOverride(bctx)
		})

		if layout == draft.LayoutSplit {
			return header + "\n\n" +
				topRow + "\n" +
				mockEditorContent(palette, frameWidth) + "\n\n" +
				bottomRow
		}
		return header + "\n\n" +
			topRow + "\n" + bottomRow + "\n" +
			mockEditorContent(palette, frameWidth)

	default:
		return header + "\n\n" + "unknown layout: " + layout
	}
}

// ── helpers ──

func (p *Page) loadPalette() *theme.Palette {
	// Try the user's configured theme first (so the preview looks like
	// their real bar), fall back to the bundled default if the user
	// hasn't configured anything or resolution fails.
	cfg := config.DefaultConfig()
	candidates := []string{cfg.Theme}

	// Walk ~/.zmux.toml's theme if present.
	fs := config.RealFS{}
	if home, err := fs.UserHomeDir(); err == nil {
		userResolver := theme.NewResolver(
			fs,
			home+"/.zmux/themes",
			home+"/.zmux/themes/iterm2",
		)
		if path, err := config.ExpandHome(fs, "~/.zmux.toml"); err == nil {
			if userCfg, err := config.Load(fs, path); err == nil && userCfg.Theme != "" {
				candidates = append([]string{userCfg.Theme}, candidates...)
			}
		}
		for _, name := range candidates {
			if t, err := userResolver.Resolve(name); err == nil {
				pal := t.SemanticPalette()
				return &pal
			}
		}
	}

	// Last resort: bundled default via empty-dir resolver.
	for _, name := range candidates {
		if t, err := p.resolver.Resolve(name); err == nil {
			pal := t.SemanticPalette()
			return &pal
		}
	}
	return &theme.Palette{}
}

func buildSessions(names []string, count, currentIdx int, showClones bool) []draft.Session {
	if count > len(names) {
		count = len(names)
	}
	if count < 1 {
		count = 1
	}
	out := make([]draft.Session, 0, count)
	for i := 0; i < count; i++ {
		name := names[i]
		if !showClones && strings.HasSuffix(name, "-b") {
			// Sub a non-clone from further down the list, or fall back.
			if i+1 < len(names) {
				name = names[i+1]
			}
		}
		out = append(out, draft.Session{
			Name:    name,
			Index:   i + 1,
			Current: i+1 == currentIdx,
		})
	}
	return out
}

func safeName(sessions []draft.Session, idx int) string {
	if idx < 0 || idx >= len(sessions) {
		return "?"
	}
	return sessions[idx].Name
}

// renderTopRow renders the top row based on the variant selection.
// tabs: uses the real preset-styled RenderTopPreview (session tabs
//
//	matching window tab chrome).
//
// dots: enriched dots with session names, on bar bg.
// minimal: plain session names, on bar bg.
func renderTopRow(variant string, preset bar.Preset, palette *theme.Palette, sessions []string, current string, width int) string {
	switch variant {
	case draft.TopBarTabs:
		return bar.RenderTopPreview(preset, palette, sessions, current, width)
	case draft.TopBarDots, draft.TopBarMinimal:
		// Use draft renderers (ANSI) wrapped in bar bg.
		draftSessions := make([]draft.Session, len(sessions))
		currentIdx := 1
		for i, s := range sessions {
			isCurrent := s == current
			if isCurrent {
				currentIdx = i + 1
			}
			draftSessions[i] = draft.Session{Name: s, Index: i + 1, Current: isCurrent}
		}

		bg := bar.BarBGColor(palette, preset)
		bgAnsi := fmt.Sprintf("\033[48;2;%d;%d;%dm", bg.R, bg.G, bg.B)

		// Workspace pill (draft version).
		wsPill := draft.RenderWorkspacePill(palette, "myapp", preset)
		var content string
		if variant == draft.TopBarDots {
			content = draft.RenderDotsEnrichedStr(palette, draftSessions, currentIdx)
		} else {
			content = draft.RenderMinimalSessionList(palette, draftSessions, currentIdx)
		}

		row := bgAnsi + wsPill + "  " + content
		// Pad to width.
		visible := draft.VisualLen(row)
		pad := width - visible
		if pad < 0 {
			pad = 0
		}
		return row + strings.Repeat(" ", pad) + "\033[0m"
	default:
		return bar.RenderTopPreview(preset, palette, sessions, current, width)
	}
}

func dim(s string) string {
	return "\033[2m" + s + "\033[0m"
}

// mockEditorContent draws a faint "editor" block so the bar is seen
// in context. For split mode, this sits BETWEEN the top and bottom bars.
func mockEditorContent(pal *theme.Palette, width int) string {
	fgDim := fmt.Sprintf("\033[38;2;%d;%d;%dm", pal.Dim.R, pal.Dim.G, pal.Dim.B)
	reset := "\033[0m"

	lines := []string{
		"  ~ " + fgDim + "// editor content — your bar sits above/below this" + reset,
		"  " + fgDim + "func main() {" + reset,
		"  " + fgDim + "    fmt.Println(\"hello\")" + reset,
		"  " + fgDim + "}" + reset,
	}
	return strings.Join(lines, "\n")
}

// Dump renders one frame of a representative mode×variant grid to w
// (no TUI). Useful for quick ANSI inspection / side-by-side compares.
func Dump(w io.Writer, width int) {
	p := New()
	fmt.Fprintf(w, "Dumping bar mode × variant @ width=%d (default preset, 3 realistic sessions, current=2)\n\n", width)

	combos := []struct{ layout, topBar, indicator string }{
		{draft.LayoutSingle, "", draft.IndicatorNone},
		{draft.LayoutSingle, "", draft.IndicatorNumbers},
		{draft.LayoutSingle, "", draft.IndicatorDots},
		{draft.LayoutTwoLine, draft.TopBarTabs, draft.IndicatorDots},
		{draft.LayoutTwoLine, draft.TopBarMinimal, draft.IndicatorDots},
		{draft.LayoutTwoLine, draft.TopBarDots, draft.IndicatorDots},
		{draft.LayoutSplit, draft.TopBarTabs, draft.IndicatorDots},
	}

	for _, c := range combos {
		vals := map[preview.ControlID]any{
			ctlLayout:       c.layout,
			ctlVariant:      c.topBar,
			ctlIndicator:    c.indicator,
			ctlPreset:       "rpowerline",
			ctlSessionCount: 3,
			ctlCurrentIdx:   2,
			ctlNameSet:      "realistic",
			ctlShowClones:   false,
			ctlShowWS:       true,
			ctlShowGit:      true,
			ctlShowClock:    true,
		}
		ctx := preview.RenderContext{Width: width, Height: 8, Values: vals}
		fmt.Fprintf(w, "─── layout=%s  top=%s  indicator=%s ───\n%s\n\n", c.layout, c.topBar, c.indicator, p.Render(ctx))
	}
}
