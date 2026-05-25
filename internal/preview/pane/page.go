// Package pane is a non-production preview page for designing zmux's
// general pane visual system: headers, focus indication, split-line styling,
// and local shortcut hints. Feature-specific panes such as pi-clean-ui are
// modeled as optional examples, not as the core abstraction.
//
// Split layout (post-cleanup, 2026-05):
//
//   - page.go          — Page type, control wiring, Render entry point,
//     renderPaneOverview header, Dump for non-interactive review.
//   - page_fixtures.go — paneSpec + workload fixtures (coding/services/review/agent).
//   - page_layouts.go  — Per-layout renderers (split/grid/stacked/focus-rail),
//     plus renderPaneBlock + rich* chrome + per-pane attributes.
//   - page_util.go     — Width fitting, value extraction, clamp/min/max, dim().
package pane

import (
	"fmt"
	"io"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/donjor/zmux/internal/preview"
)

const (
	ctlLayout   preview.ControlID = "layout"
	ctlWorkload preview.ControlID = "workload"
	ctlFocus    preview.ControlID = "focus"
	ctlHeader   preview.ControlID = "header"
	ctlDivider  preview.ControlID = "divider"
	ctlHints    preview.ControlID = "hints"
	ctlCleanUI  preview.ControlID = "clean-ui"
	ctlAuxState preview.ControlID = "aux-state"
	ctlAuxWidth preview.ControlID = "aux-width"
)

const (
	layoutSplit     = "split"
	layoutGrid      = "grid"
	layoutStacked   = "stacked"
	layoutFocusRail = "focus+rail"

	workloadCoding   = "coding"
	workloadServices = "services"
	workloadReview   = "review"
	workloadAgent    = "agent"

	focusPrimary   = "primary"
	focusSecondary = "secondary"
	focusTertiary  = "tertiary"

	headerCompact = "compact"
	headerVerbose = "verbose"
	headerRibbon  = "ribbon"

	dividerSubtle  = "subtle"
	dividerStrong  = "strong"
	dividerRounded = "rounded"

	stateRunning   = "running"
	stateAttention = "attention"
	stateStale     = "stale"
)

// Page is the pane visual-system preview.
type Page struct {
	ctrls []preview.Control
}

// New builds the pane preview page.
func New() *Page {
	p := &Page{}
	p.ctrls = []preview.Control{
		preview.NewChoice(ctlLayout, "layout", []string{layoutSplit, layoutGrid, layoutStacked, layoutFocusRail}, layoutSplit),
		preview.NewChoice(ctlWorkload, "workload", []string{workloadCoding, workloadServices, workloadReview, workloadAgent}, workloadCoding),
		preview.NewChoice(ctlFocus, "focus", []string{focusPrimary, focusSecondary, focusTertiary}, focusPrimary),
		preview.NewChoice(ctlHeader, "header", []string{headerCompact, headerVerbose, headerRibbon}, headerVerbose),
		preview.NewChoice(ctlDivider, "divider", []string{dividerSubtle, dividerStrong, dividerRounded}, dividerStrong),
		preview.NewToggle(ctlHints, "hints", true),
		preview.NewToggle(ctlCleanUI, "clean-ui ex", false),
		preview.NewChoice(ctlAuxState, "aux state", []string{stateRunning, stateAttention, stateStale}, stateRunning),
		preview.NewInt(ctlAuxWidth, "aux %", 25, 50, 36, "wide"),
	}
	return p
}

func (p *Page) ID() string                  { return "pane" }
func (p *Page) Title() string               { return "Pane System" }
func (p *Page) Controls() []preview.Control { return p.ctrls }

func (p *Page) Render(ctx preview.RenderContext) string {
	layout := stringValue(ctx, ctlLayout, layoutSplit)
	workload := stringValue(ctx, ctlWorkload, workloadCoding)
	focus := stringValue(ctx, ctlFocus, focusPrimary)
	header := stringValue(ctx, ctlHeader, headerVerbose)
	divider := stringValue(ctx, ctlDivider, dividerStrong)
	hints := boolValue(ctx, ctlHints, true)
	cleanUIExample := boolValue(ctx, ctlCleanUI, false)
	auxState := stringValue(ctx, ctlAuxState, stateRunning)
	auxPct := intValue(ctx, ctlAuxWidth, 36)

	frameWidth := ctx.Width - 4
	if frameWidth < 58 {
		frameWidth = ctx.Width
	}
	if frameWidth < 42 {
		frameWidth = 42
	}

	panes := fixturePanes(workload, cleanUIExample, auxState, focus)
	summary := dim(fmt.Sprintf("  layout=%s  workload=%s  header=%s  divider=%s  focus=%s  hints=%t  clean-ui-example=%t  aux=%s", layout, workload, header, divider, focus, hints, cleanUIExample, auxState))
	legend := dim("  principle: zmux owns general pane chrome; apps own what they render inside their panes")

	var body string
	if frameWidth < 96 {
		body = renderStacked(narrowPanes(layout, panes), frameWidth, header, divider, hints)
	} else {
		switch layout {
		case layoutGrid:
			body = renderGrid(panes, frameWidth, header, divider, hints, auxPct)
		case layoutStacked:
			body = renderStacked(panes, frameWidth, header, divider, hints)
		case layoutFocusRail:
			body = renderFocusRail(panes, frameWidth, header, divider, hints, auxPct)
		default:
			body = renderSplit(panes[0], panes[1], frameWidth, header, divider, hints, auxPct)
		}
	}

	overview := renderPaneOverview(layout, workload, focus, cleanUIExample, auxState, frameWidth)
	return overview + "\n\n" + summary + "\n" + legend + "\n\n" + indent(body, "  ")
}

func renderPaneOverview(layout, workload, focus string, cleanUIExample bool, auxState string, width int) string {
	badges := []string{
		preview.Badge("ZMUX PANES", preview.Gold),
		preview.Badge(strings.ToUpper(layout), preview.Blue),
	}
	if cleanUIExample {
		badges = append(badges, preview.Badge("CLEAN-UI EXAMPLE", preview.Purple))
	}
	switch auxState {
	case stateAttention:
		badges = append(badges, preview.Badge("ATTENTION", preview.Orange))
	case stateStale:
		badges = append(badges, preview.Badge("STALE", preview.Red))
	}

	trend := preview.Sparkline([]float64{0.22, 0.35, 0.31, 0.48, 0.62, 0.58, 0.72, 0.69, 0.81, 0.76, 0.84, 0.9}, preview.Teal)
	left := lipgloss.NewStyle().Foreground(preview.Gold).Bold(true).Render("pane chrome system")
	sub := lipgloss.NewStyle().Foreground(preview.Muted).Render("headers · active borders · local shortcuts · workload-aware density")
	gap := width - lipgloss.Width(left) - lipgloss.Width(trend)
	if gap < 1 {
		gap = 1
	}
	header := left + strings.Repeat(" ", gap) + trend

	cardW := max(14, min(22, (width-8)/3))
	cards := []string{
		preview.MetricCard("WORKLOAD", workload, "fixture set", preview.Blue, cardW),
		preview.MetricCard("FOCUS", focus, "active pane", preview.Gold, cardW),
		preview.MetricCard("AUX", auxState, "secondary state", preview.Teal, cardW),
	}
	cardRow := lipgloss.JoinHorizontal(lipgloss.Top, cards...)
	if width < 74 {
		cardRow = strings.Join(cards, "\n")
	}
	return "  " + strings.Join(badges, " ") + "\n" +
		"  " + header + "\n" +
		"  " + sub + "\n\n" +
		indent(cardRow, "  ")
}

// Dump writes representative pane-system variants for non-interactive review.
func Dump(w io.Writer, width int) {
	p := New()
	cases := []struct {
		layout   string
		workload string
		focus    string
		header   string
		divider  string
		cleanUI  bool
		state    string
	}{
		{layoutSplit, workloadCoding, focusPrimary, headerVerbose, dividerStrong, false, stateRunning},
		{layoutGrid, workloadServices, focusTertiary, headerCompact, dividerRounded, false, stateAttention},
		{layoutFocusRail, workloadAgent, focusSecondary, headerRibbon, dividerStrong, true, stateRunning},
		{layoutStacked, workloadReview, focusSecondary, headerVerbose, dividerSubtle, true, stateStale},
	}
	fmt.Fprintf(w, "Dumping pane visual-system variants @ width=%d\n\n", width)
	for _, c := range cases {
		ctx := preview.RenderContext{
			Width: width,
			Values: map[preview.ControlID]any{
				ctlLayout:   c.layout,
				ctlWorkload: c.workload,
				ctlFocus:    c.focus,
				ctlHeader:   c.header,
				ctlDivider:  c.divider,
				ctlHints:    true,
				ctlCleanUI:  c.cleanUI,
				ctlAuxState: c.state,
				ctlAuxWidth: 36,
			},
		}
		fmt.Fprintf(w, "─── layout=%s  workload=%s  focus=%s  header=%s  divider=%s  clean-ui=%t  state=%s ───\n%s\n\n", c.layout, c.workload, c.focus, c.header, c.divider, c.cleanUI, c.state, p.Render(ctx))
	}
}
