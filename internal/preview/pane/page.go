// Package pane is a non-production preview page for designing zmux's
// general pane visual system: headers, focus indication, split-line styling,
// and local shortcut hints. Feature-specific panes such as pi-clean-ui are
// modeled as optional examples, not as the core abstraction.
package pane

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"

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

const (
	ansiReset = "\033[0m"
	ansiBold  = "\033[1m"
	ansiDim   = "\033[2m"
	fgCyan    = "\033[36m"
	fgBlue    = "\033[34m"
	fgGreen   = "\033[32m"
	fgYellow  = "\033[33m"
	fgMagenta = "\033[35m"
	fgRed     = "\033[31m"
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
	if auxState == stateAttention {
		badges = append(badges, preview.Badge("ATTENTION", preview.Orange))
	} else if auxState == stateStale {
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

type paneSpec struct {
	Slot    string
	ID      string
	Title   string
	Command string
	CWD     string
	Size    string
	Focused bool
	State   string
	Lines   []string
}

func fixturePanes(workload string, cleanUIExample bool, auxState string, focus string) []paneSpec {
	primary := paneSpec{
		Slot:    focusPrimary,
		ID:      "%11",
		Title:   "editor",
		Command: "nvim",
		CWD:     "~/donjor/zmux",
		Size:    "142×52",
		Focused: focus == focusPrimary,
		State:   stateRunning,
		Lines: []string{
			"internal/tmux/conf.go",
			"internal/preview/pane/page.go",
			"cmd/zmux/pane.go",
			"",
			"// Pane chrome should make focus and purpose obvious",
			"// before the user reads pane contents.",
		},
	}
	secondary := paneSpec{
		Slot:    focusSecondary,
		ID:      "%12",
		Title:   "server",
		Command: "go run ./cmd/api",
		CWD:     "~/donjor/myapp",
		Size:    "86×52",
		Focused: focus == focusSecondary,
		State:   auxState,
		Lines: []string{
			"listening on :8080",
			"GET /health 200 2ms",
			"GET /api/sessions 200 8ms",
			"",
			"hot reload: ready",
		},
	}
	tertiary := paneSpec{
		Slot:    focusTertiary,
		ID:      "%13",
		Title:   "tests",
		Command: "go test ./...",
		CWD:     "~/donjor/zmux",
		Size:    "86×18",
		Focused: focus == focusTertiary,
		State:   stateRunning,
		Lines: []string{
			"ok  github.com/donjor/zmux/cmd/zmux",
			"ok  github.com/donjor/zmux/internal/tmux",
			"ok  github.com/donjor/zmux/internal/tui/dashboard/tabs",
			"",
			"watching for changes…",
		},
	}

	switch workload {
	case workloadServices:
		primary.Title, primary.Command = "api", "air ./cmd/api"
		primary.Lines = []string{"POST /v1/chat 200 418ms", "GET /v1/tasks 200 12ms", "worker queue depth: 3", "cache hit rate: 94%"}
		secondary.Title, secondary.Command = "worker", "npm run worker"
		secondary.CWD = "~/donjor/myapp/services/worker"
		secondary.Lines = []string{"job sync_catalog done", "job refresh_cache running", "job emit_metrics pending", "", "press pfx+q to reveal pane ids"}
		tertiary.Title, tertiary.Command = "logs", "tail -f app.log"
		tertiary.Lines = []string{"INFO deploy sha=9f32", "WARN retry payment-webhook", "INFO queue drained"}
	case workloadReview:
		primary.Title, primary.Command = "diff", "git diff"
		primary.Lines = []string{"diff --git a/internal/tmux/conf.go b/internal/tmux/conf.go", "+ set -g pane-border-status top", "+ set -g pane-active-border-style fg=cyan,bold", "", "Reviewing whether this should graduate from proto."}
		secondary.Title, secondary.Command = "notes", "vim docs/review.md"
		secondary.Lines = []string{"Review checklist", "✓ current pane command", "✓ toggle command", "→ pane border design", "", "Question: compact or verbose header?"}
		tertiary.Title, tertiary.Command = "shell", "git status"
		tertiary.Lines = []string{"## master...origin/master [ahead 45]", " M internal/preview/pane/page.go"}
	case workloadAgent:
		primary.Title, primary.Command = "pi", "pi"
		primary.Lines = []string{"Mission: general pane visual system", "", "User: panes in a general sense; sidecar is just a feature", "", "Assistant: updating prototype boundaries."}
		secondary.Title, secondary.Command = "captain", "zmux watch captain"
		secondary.Lines = []string{"Tasks 2/4 done · 1 active", "→ implement generalized pane prototype", "", "No global status clutter."}
		tertiary.Title, tertiary.Command = "scratch", "rg pane"
		tertiary.Lines = []string{"cmd/zmux/pane.go", "internal/tui/dashboard/tabs/current_tree.go", "internal/preview/pane/page.go"}
	}

	if cleanUIExample {
		secondary = cleanUIPane(auxState, focus == focusSecondary)
	}
	return []paneSpec{primary, secondary, tertiary}
}

func cleanUIPane(state string, focused bool) paneSpec {
	mode := "live"
	if state == stateStale {
		mode = "degraded"
	}
	return paneSpec{
		Slot:    focusSecondary,
		ID:      "%73",
		Title:   "clean-ui",
		Command: "clean-ui-sidecar",
		CWD:     "~/pi-extensions/pi-clean-ui",
		Size:    "92×52",
		Focused: focused,
		State:   state,
		Lines: []string{
			"clean-ui sidecar  operations cockpit      seq 42 · gen 3",
			"~/donjor/zmux",
			"",
			"▸ watch   tasks   artifacts",
			"━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━",
			"WATCH  7   TASKS  3/4   ARTIFACTS  2",
			"",
			"✓ edit internal/preview/pane/page.go · +142/-88",
			"→ task: generalize pane prototype",
			"",
			fmt.Sprintf("j/k select · h/l tabs · q quit          %s", mode),
		},
	}
}

func narrowPanes(layout string, panes []paneSpec) []paneSpec {
	if layout == layoutSplit && len(panes) > 2 {
		return panes[:2]
	}
	if layout == layoutFocusRail {
		ordered := make([]paneSpec, 0, len(panes))
		for _, pane := range panes {
			if pane.Focused {
				ordered = append(ordered, pane)
			}
		}
		for _, pane := range panes {
			if !pane.Focused {
				ordered = append(ordered, pane)
			}
		}
		if len(ordered) > 0 {
			return ordered
		}
	}
	return panes
}

func renderSplit(primary, secondary paneSpec, width int, headerMode, dividerMode string, hints bool, auxPct int) string {
	rightW := clamp(width*auxPct/100, 28, width-32)
	leftW := width - rightW - 1
	if leftW < 32 {
		leftW = 32
		rightW = width - leftW - 1
	}
	gap := dividerGap(dividerMode, primary.Focused || secondary.Focused)
	return lipgloss.JoinHorizontal(lipgloss.Top,
		renderPaneBlock(primary, leftW, 10, headerMode, hints),
		gap,
		renderPaneBlock(secondary, rightW, 10, headerMode, hints),
	)
}

func renderGrid(panes []paneSpec, width int, headerMode, dividerMode string, hints bool, auxPct int) string {
	rightW := clamp(width*auxPct/100, 26, width-26)
	leftW := width - rightW - 1
	if leftW < 26 {
		leftW = 26
		rightW = width - leftW - 1
	}
	left := renderPaneBlock(panes[0], leftW, 16, headerMode, hints)
	upper := renderPaneBlock(panes[1], rightW, 6, headerMode, hints)
	lower := renderPaneBlock(panes[2], rightW, 6, headerMode, hints)
	right := upper + "\n" + lower
	return lipgloss.JoinHorizontal(lipgloss.Top, left, dividerGap(dividerMode, anyFocused(panes)), right)
}

func renderStacked(panes []paneSpec, width int, headerMode, dividerMode string, hints bool) string {
	var blocks []string
	for _, pane := range panes {
		blocks = append(blocks, renderPaneBlock(pane, width, 5, headerMode, hints))
	}
	return strings.Join(blocks, "\n")
}

func renderFocusRail(panes []paneSpec, width int, headerMode, dividerMode string, hints bool, railPct int) string {
	focused := panes[0]
	for _, pane := range panes {
		if pane.Focused {
			focused = pane
			break
		}
	}
	rail := make([]paneSpec, 0, len(panes)-1)
	for _, pane := range panes {
		if pane.ID != focused.ID {
			rail = append(rail, pane)
		}
	}
	rightW := clamp(width*railPct/100, 28, width-34)
	leftW := width - rightW - 1
	left := renderPaneBlock(focused, leftW, 14, headerMode, hints)
	var rightBlocks []string
	for _, pane := range rail {
		rightBlocks = append(rightBlocks, renderPaneBlock(pane, rightW, 5, headerMode, hints))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, left, dividerGap(dividerMode, true), strings.Join(rightBlocks, "\n"))
}

func renderColumns(panes []paneSpec, widths []int, headerMode, dividerMode string, hints bool, bodyHeight int) string {
	var rows []string
	for i, pane := range panes {
		rows = append(rows, paneHeader(pane, widths[i], headerMode, hints))
	}
	var b strings.Builder
	b.WriteString(strings.Join(rows, dividerVertical(dividerMode, anyFocused(panes))))
	b.WriteString("\n")
	for line := 0; line < bodyHeight; line++ {
		rows = rows[:0]
		for i, pane := range panes {
			rows = append(rows, paneBodyLine(pane, widths[i], line))
		}
		b.WriteString(strings.Join(rows, dividerVertical(dividerMode, anyFocused(panes))))
		if line < bodyHeight-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func renderPaneBlock(p paneSpec, width, bodyHeight int, headerMode string, hints bool) string {
	innerW := max(12, width-2)
	accent := paneAccent(p)
	border := preview.Dim
	if p.Focused {
		border = preview.Gold
	} else if p.State == stateAttention {
		border = preview.Orange
	} else if p.State == stateStale {
		border = preview.Red
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Width(innerW)
	if p.Focused {
		style = style.Bold(true)
	}

	header := richPaneHeader(p, innerW, headerMode, hints, accent)
	metrics := richPaneMetrics(p, innerW, accent)
	body := richPaneBody(p, innerW, bodyHeight)
	return style.Render(header + "\n" + metrics + "\n" + body)
}

func richPaneHeader(p paneSpec, width int, mode string, hints bool, accent lipgloss.Color) string {
	mark := "○"
	if p.Focused {
		mark = "●"
	}
	state := stateLabel(p)
	var raw string
	if mode == headerCompact {
		raw = mark + " " + p.ID + " " + p.Title
		if state != "" {
			raw += "  " + state
		}
		return fitVisual(lipgloss.NewStyle().Foreground(accent).Bold(p.Focused).Render(fit(raw, width)), width)
	}
	if mode == headerRibbon {
		raw = mark + " " + p.Title
		if hints {
			raw += "  " + localHints(p, hints)
		}
		return fitVisual(lipgloss.NewStyle().Foreground(preview.BGDark).Background(accent).Bold(true).Render(fit(" "+raw+" ", width)), width)
	}
	raw = mark + " " + p.ID + " " + p.Title + "  " + p.Command
	if state != "" {
		raw += "  " + state
	}
	return fitVisual(lipgloss.NewStyle().Foreground(accent).Bold(p.Focused).Render(fit(raw, width)), width)
}

func richPaneMetrics(p paneSpec, width int, accent lipgloss.Color) string {
	trend := preview.Sparkline(paneTrend(p), accent)
	trendW := lipgloss.Width(trend)
	metaW := max(1, width-trendW-1)
	meta := preview.MuteStyle.Render(fit(p.CWD+" · "+p.Size, metaW))
	gap := width - lipgloss.Width(meta) - trendW
	if gap < 1 {
		gap = 1
	}
	return fitVisual(meta+strings.Repeat(" ", gap)+trend, width)
}

func richPaneBody(p paneSpec, width, bodyHeight int) string {
	var lines []string
	for i := 0; i < bodyHeight; i++ {
		content := ""
		if i < len(p.Lines) {
			content = p.Lines[i]
		}
		if i == len(p.Lines)+1 && !p.Focused {
			content = "Alt+Shift+Arrow focuses this pane"
		}
		prefix := preview.DimStyle.Render("│ ")
		if p.Focused {
			prefix = lipgloss.NewStyle().Foreground(preview.Gold).Bold(true).Render("▌ ")
		} else if p.State == stateAttention {
			prefix = lipgloss.NewStyle().Foreground(preview.Orange).Render("│ ")
		} else if p.State == stateStale {
			prefix = lipgloss.NewStyle().Foreground(preview.Red).Render("│ ")
		}
		text := preview.FGStyle.Render(fit(content, max(1, width-2)))
		lines = append(lines, prefix+fitVisual(text, max(1, width-2)))
	}
	return strings.Join(lines, "\n")
}

func stateChip(label, state string) string {
	color := preview.Green
	if state == stateAttention {
		color = preview.Orange
	} else if state == stateStale {
		color = preview.Red
	}
	return lipgloss.NewStyle().Foreground(color).Bold(true).Render(label)
}

func paneAccent(p paneSpec) lipgloss.Color {
	if p.Focused {
		return preview.Gold
	}
	switch p.Slot {
	case focusSecondary:
		return preview.Blue
	case focusTertiary:
		return preview.Purple
	default:
		return preview.Teal
	}
}

func paneTrend(p paneSpec) []float64 {
	switch p.State {
	case stateAttention:
		return []float64{0.32, 0.44, 0.58, 0.73, 0.88, 0.79, 0.9, 0.82}
	case stateStale:
		return []float64{0.7, 0.62, 0.52, 0.44, 0.38, 0.3, 0.24, 0.2}
	default:
		return []float64{0.2, 0.34, 0.31, 0.45, 0.58, 0.52, 0.64, 0.72}
	}
}

func dividerGap(mode string, active bool) string {
	glyph := " "
	if mode == dividerStrong {
		glyph = "┃"
	} else if mode == dividerRounded {
		glyph = "┆"
	}
	if active && glyph != " " {
		return lipgloss.NewStyle().Foreground(preview.Gold).Bold(true).Render(glyph)
	}
	return preview.DimStyle.Render(glyph)
}

func paneHeader(p paneSpec, width int, mode string, hints bool) string {
	if width < 8 {
		return strings.Repeat(" ", max(0, width))
	}
	focusMark := "○"
	if p.Focused {
		focusMark = "●"
	}
	state := stateLabel(p)
	var text string
	switch mode {
	case headerCompact:
		text = fmt.Sprintf(" %s %s %s %s", focusMark, p.ID, p.Title, state)
	case headerRibbon:
		text = fmt.Sprintf(" %s %s  %s", focusMark, p.Title, localHints(p, hints))
	default:
		text = fmt.Sprintf(" %s %s %s  %s  %s  %s", focusMark, p.ID, p.Title, p.Command, p.CWD, p.Size)
		if state != "" {
			text += "  " + state
		}
		if hints {
			text += "  " + localHints(p, hints)
		}
	}
	text = fit(text, width)
	if mode == headerRibbon {
		text = "▰" + fit(strings.TrimSpace(text), width-2) + "▰"
	}
	if p.Focused {
		return ansiBold + fgCyan + text + ansiReset
	}
	if p.State == stateAttention {
		return ansiBold + fgYellow + text + ansiReset
	}
	if p.State == stateStale {
		return ansiBold + fgRed + text + ansiReset
	}
	return ansiDim + text + ansiReset
}

func paneBodyLine(p paneSpec, width, line int) string {
	if width < 4 {
		return strings.Repeat(" ", max(0, width))
	}
	gutter := "│"
	style := ansiDim
	if p.Focused {
		gutter = "▌"
		style = fgCyan
	} else if p.State == stateAttention {
		style = fgYellow
	} else if p.State == stateStale {
		style = fgRed
	} else if p.Slot == focusSecondary {
		style = fgBlue
	} else if p.Slot == focusTertiary {
		style = fgMagenta
	}
	content := ""
	if line < len(p.Lines) {
		content = p.Lines[line]
	}
	if line == len(p.Lines)+1 && !p.Focused {
		content = "inactive — Alt+Shift+Arrow to focus"
	}
	inner := fit(" "+content, width-1)
	return style + gutter + ansiReset + inner
}

func dividerVertical(mode string, active bool) string {
	s := "│"
	switch mode {
	case dividerStrong:
		s = "┃"
	case dividerRounded:
		s = "┆"
	}
	if active {
		return ansiBold + fgCyan + s + ansiReset
	}
	return ansiDim + s + ansiReset
}

func dividerHorizontal(width int, mode string, active bool) string {
	s := "─"
	switch mode {
	case dividerStrong:
		s = "━"
	case dividerRounded:
		s = "╌"
	}
	line := strings.Repeat(s, max(0, width))
	if active {
		return ansiBold + fgCyan + line + ansiReset
	}
	return ansiDim + line + ansiReset
}

func stateLabel(p paneSpec) string {
	switch p.State {
	case stateAttention:
		return "◆ attention"
	case stateStale:
		return "△ stale"
	default:
		return ""
	}
}

func localHints(p paneSpec, enabled bool) string {
	if !enabled {
		return ""
	}
	switch p.Slot {
	case focusSecondary:
		if p.Title == "clean-ui" {
			return "A-S-← main · sidecar-focus · sidecar-close"
		}
		return "A-S-←/→ focus · pfx+z zoom · pfx+q ids"
	case focusTertiary:
		return "A-S-↑/↓ focus · x close"
	default:
		return "A-S-→ next pane · pfx+q ids"
	}
}

func anyFocused(panes []paneSpec) bool {
	for _, p := range panes {
		if p.Focused {
			return true
		}
	}
	return false
}

func zipBlocks(left, right string, leftWidth int, divider string) string {
	lLines := strings.Split(left, "\n")
	rLines := strings.Split(right, "\n")
	n := max(len(lLines), len(rLines))
	var out []string
	for i := 0; i < n; i++ {
		l, r := "", ""
		if i < len(lLines) {
			l = lLines[i]
		} else {
			l = strings.Repeat(" ", max(0, leftWidth))
		}
		if i < len(rLines) {
			r = rLines[i]
		}
		out = append(out, l+divider+r)
	}
	return strings.Join(out, "\n")
}

func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n")
}

func stringValue(ctx preview.RenderContext, id preview.ControlID, fallback string) string {
	if v, ok := ctx.Values[id].(string); ok && v != "" {
		return v
	}
	return fallback
}

func boolValue(ctx preview.RenderContext, id preview.ControlID, fallback bool) bool {
	if v, ok := ctx.Values[id].(bool); ok {
		return v
	}
	return fallback
}

func intValue(ctx preview.RenderContext, id preview.ControlID, fallback int) int {
	if v, ok := ctx.Values[id].(int); ok {
		return v
	}
	return fallback
}

func fitVisual(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

func fit(s string, width int) string {
	if width <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) > width {
		if width == 1 {
			return "…"
		}
		return string(r[:width-1]) + "…"
	}
	return s + strings.Repeat(" ", width-len(r))
}

func dim(s string) string { return ansiDim + s + ansiReset }

func clamp(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
