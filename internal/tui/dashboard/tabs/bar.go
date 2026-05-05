package tabs

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/donjor/zmux/internal/bar"
	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/theme"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui"
	"github.com/donjor/zmux/internal/tui/dashboard"
)

// ── Messages ──

type barDataMsg struct {
	reqID     int64
	cfg       config.Config
	cfgExists bool
	palette   *theme.Palette
	err       error
}

func (m barDataMsg) TargetTab() dashboard.TabID { return dashboard.TabBar }

type barConfigSaveMsg struct {
	reqID int64
	err   error
}

func (m barConfigSaveMsg) TargetTab() dashboard.TabID { return dashboard.TabBar }

// ── Bar sections ──

type barSection int

const (
	barPresets barSection = iota
	barLayout
	barSegments
)

// ── Layout options ──

type barLayoutOption struct {
	Label   string
	Field   string
	Options []string
}

var barLayoutOptions = []barLayoutOption{
	{"Layout", "layout", []string{"single", "two-line", "split"}},
	{"Top bar", "top_bar", []string{"tabs", "dots", "minimal"}},
	{"Indicator", "indicator", []string{"none", "numbers", "dots"}},
}

// ── Segment labels ──

var barSegmentLabels = []struct {
	Label string
	Field string
}{
	{"Git branch", "git"},
	{"Workspace", "workspace"},
	{"Clock", "clock"},
	{"Language", "lang"},
	{"Directory", "directory"},
	{"Process", "process"},
	{"Group indicator", "group"},
}

// ── BarTab ──

// BarTab implements the Tab interface for status bar preset and segment management.
type BarTab struct {
	resolver *theme.Resolver
	fs       config.FS
	runner   tmux.Runner
	styles   tui.Styles

	presets    []bar.Preset
	cursor     int
	currentBar string
	segments   config.BarSegments

	// Layout settings.
	layout    string // "single", "two-line", "split"
	topBar    string // "tabs", "dots", "minimal"
	indicator string // "none", "numbers", "dots"

	// Resolved palette for bar previews.
	palette *theme.Palette

	cfg       config.Config
	cfgExists bool

	reqID int64

	vp            viewport.Model
	width, height int
}

// NewBarTab creates a new bar tab.
func NewBarTab(resolver *theme.Resolver, fs config.FS, runner tmux.Runner, styles tui.Styles) *BarTab {
	return &BarTab{
		resolver: resolver,
		fs:       fs,
		runner:   runner,
		styles:   styles,
		presets:  bar.AllPresets(),
	}
}

func (t *BarTab) ID() dashboard.TabID { return dashboard.TabBar }
func (t *BarTab) Title() string       { return "Bar" }
func (t *BarTab) Init() tea.Cmd       { return nil }

func (t *BarTab) Activate(reason dashboard.ActivateReason) tea.Cmd {
	t.reqID = dashboard.NextReqID()
	return t.fetchData(t.reqID)
}

func (t *BarTab) Deactivate() {
	t.reqID = dashboard.NextReqID()
}

func (t *BarTab) Resize(w, h int) {
	t.width = w
	t.height = h
	t.vp.Width = w
	t.vp.Height = h
}

func (t *BarTab) ShortHelp() string {
	switch t.currentSection() {
	case barLayout:
		return "h/l:change  j/k:navigate  g/G:top/bottom"
	case barSegments:
		return "enter/space:toggle  j/k:navigate  g/G:top/bottom"
	default:
		return "enter:apply  j/k:navigate  g/G:top/bottom"
	}
}

// ── Update ──

func (t *BarTab) Update(msg tea.Msg) (dashboard.Tab, tea.Cmd) {
	switch msg := msg.(type) {
	case dashboard.ThemeChangedMsg:
		t.styles = msg.Styles
		p := msg.Palette
		t.palette = &p
		return t, nil

	case barDataMsg:
		if msg.reqID != t.reqID {
			return t, nil
		}
		if msg.err != nil {
			return t, func() tea.Msg {
				return dashboard.SetStatusIntent{
					Text:    fmt.Sprintf("Failed to load bar config: %v", msg.err),
					IsError: true,
				}
			}
		}
		t.cfg = msg.cfg
		t.cfgExists = msg.cfgExists
		t.currentBar = msg.cfg.Bar.Preset
		t.segments = msg.cfg.Bar.Segments
		t.layout = msg.cfg.Bar.Layout
		t.topBar = msg.cfg.Bar.TopBar
		t.indicator = msg.cfg.Bar.Indicator
		t.palette = msg.palette
		return t, nil

	case barConfigSaveMsg:
		if msg.reqID != t.reqID {
			return t, nil
		}
		if msg.err != nil {
			return t, func() tea.Msg {
				return dashboard.SetStatusIntent{
					Text:    fmt.Sprintf("Save failed: %v", msg.err),
					IsError: true,
				}
			}
		}
		return t, func() tea.Msg {
			return dashboard.SetStatusIntent{Text: "Bar config saved", IsError: false}
		}

	case tea.KeyMsg:
		return t.handleKey(msg)
	}

	return t, nil
}

// ── Key handling ──

func (t *BarTab) handleKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	total := len(t.presets) + len(barLayoutOptions) + len(barSegmentLabels)

	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
		if t.cursor > 0 {
			t.cursor--
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
		if t.cursor < total-1 {
			t.cursor++
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("left", "h"))):
		if t.currentSection() == barLayout {
			layoutIdx := t.cursor - len(t.presets)
			t.cycleLayoutValue(barLayoutOptions[layoutIdx].Field, -1)
			return t, t.saveConfig()
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("right", "l"))):
		if t.currentSection() == barLayout {
			layoutIdx := t.cursor - len(t.presets)
			t.cycleLayoutValue(barLayoutOptions[layoutIdx].Field, 1)
			return t, t.saveConfig()
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("enter", " "))):
		switch t.currentSection() {
		case barPresets:
			preset := t.presets[t.cursor]
			t.currentBar = preset.String()
			t.cfg.Bar.Preset = preset.String()
			return t, t.saveConfig()
		case barLayout:
			layoutIdx := t.cursor - len(t.presets)
			t.cycleLayoutValue(barLayoutOptions[layoutIdx].Field, 1)
			return t, t.saveConfig()
		case barSegments:
			segIdx := t.cursor - len(t.presets) - len(barLayoutOptions)
			if segIdx >= 0 && segIdx < len(barSegmentLabels) {
				t.toggleSegment(barSegmentLabels[segIdx].Field)
				t.cfg.Bar.Segments = t.segments
				return t, t.saveConfig()
			}
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("g"))):
		t.cursor = 0
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("G"))):
		t.cursor = total - 1
		return t, nil
	}

	return t, nil
}

// ── View ──

func (t *BarTab) View() string {
	var b strings.Builder
	cursorLine := 0
	lineCount := 0

	b.WriteString("\n")
	lineCount++
	currentLabel := "default"
	if t.currentBar != "" {
		currentLabel = t.currentBar
	}
	if t.layout != "" && t.layout != "single" {
		currentLabel += " (" + t.layout + ")"
	}
	b.WriteString(t.styles.Dim.Render("Current: ") + t.styles.Success.Render(currentLabel))
	b.WriteString("\n\n")
	lineCount += 2

	// ── Presets ──

	for i, preset := range t.presets {
		selected := t.currentSection() == barPresets && t.cursor == i
		isCurrent := preset.String() == t.currentBar

		if selected {
			cursorLine = lineCount
		}

		cursor := "  "
		if selected {
			cursor = t.styles.Accent.Render("| ")
		}

		nameStyle := t.styles.Normal
		if selected {
			nameStyle = t.styles.Accent.Bold(true)
		}

		currentMark := ""
		if isCurrent {
			currentMark = t.styles.Success.Render(" *")
		}

		b.WriteString("  " + cursor + nameStyle.Render(preset.String()) + currentMark + "\n")
		lineCount++

		if t.palette != nil {
			preview := t.renderPresetPreview(preset)
			for _, line := range strings.Split(preview, "\n") {
				b.WriteString("    " + line + "\n")
				lineCount++
			}
		}
		b.WriteString("\n")
		lineCount++
	}

	// ── Layout options ──

	b.WriteString("  " + t.styles.Muted.Render("Layout") + "\n\n")
	lineCount += 2

	P := len(t.presets)
	for i, opt := range barLayoutOptions {
		idx := P + i
		selected := t.cursor == idx

		if selected {
			cursorLine = lineCount
		}

		cursor := "  "
		if selected {
			cursor = t.styles.Accent.Render("| ")
		}

		value := t.layoutValue(opt.Field)
		labelStyle := t.styles.Normal
		valueStyle := t.styles.Success
		if selected {
			labelStyle = t.styles.Accent
		}

		b.WriteString(fmt.Sprintf("  %s%s  ◀ %s ▶\n",
			cursor,
			labelStyle.Render(opt.Label+":"),
			valueStyle.Render(value),
		))
		lineCount++
	}
	b.WriteString("\n")
	lineCount++

	// ── Segment toggles ──

	b.WriteString("  " + t.styles.Muted.Render("Segments") + "\n\n")
	lineCount += 2

	segBase := P + len(barLayoutOptions)
	for i, seg := range barSegmentLabels {
		idx := segBase + i
		selected := t.cursor == idx

		if selected {
			cursorLine = lineCount
		}

		cursor := "  "
		if selected {
			cursor = t.styles.Accent.Render("| ")
		}

		enabled := t.segmentEnabled(seg.Field)
		checkbox := t.styles.Dim.Render("[ ]")
		if enabled {
			checkbox = t.styles.Success.Render("[x]")
		}

		label := t.styles.Normal.Render(seg.Label)
		if selected {
			label = t.styles.Accent.Render(seg.Label)
		}

		b.WriteString("  " + cursor + checkbox + " " + label + "\n")
		lineCount++
	}

	t.vp.SetContent(b.String())
	ensureCursorVisible(&t.vp, cursorLine)
	return renderScrollable(t.vp, t.styles)
}

// ── Data commands ──

func (t *BarTab) fetchData(reqID int64) tea.Cmd {
	fs := t.fs
	resolver := t.resolver
	return func() tea.Msg {
		cfgPath, err := config.ConfigPath(fs)
		if err != nil {
			return barDataMsg{reqID: reqID, err: err}
		}
		exists := config.ConfigExists(fs)
		cfg, err := config.Load(fs, cfgPath)
		if err != nil {
			cfg = config.DefaultConfig()
		}

		var palette *theme.Palette
		if resolver != nil && cfg.Theme != "" {
			resolved, resolveErr := resolver.Resolve(cfg.Theme)
			if resolveErr == nil {
				p := resolved.SemanticPalette()
				palette = &p
			}
		}

		return barDataMsg{
			reqID:     reqID,
			cfg:       cfg,
			cfgExists: exists,
			palette:   palette,
		}
	}
}

func (t *BarTab) saveConfig() tea.Cmd {
	fs := t.fs
	runner := t.runner
	cfg := t.cfg
	reqID := t.reqID
	resolver := t.resolver
	return func() tea.Msg {
		cfgPath, err := config.ConfigPath(fs)
		if err != nil {
			return barConfigSaveMsg{reqID: reqID, err: err}
		}
		if err := config.Save(fs, cfgPath, cfg); err != nil {
			return barConfigSaveMsg{reqID: reqID, err: err}
		}
		// Apply bar live.
		if runner != nil && resolver != nil {
			preset, _ := bar.PresetFromString(cfg.Bar.Preset)
			resolved, resolveErr := resolver.Resolve(cfg.Theme)
			if resolveErr == nil {
				p := resolved.SemanticPalette()
				lc := bar.BarLayoutConfig{
					Layout:    cfg.Bar.Layout,
					Indicator: cfg.Bar.Indicator,
					TopBar:    cfg.Bar.TopBar,
				}
				_ = bar.Apply(runner, preset, &p, lc)
			}
		}
		return barConfigSaveMsg{reqID: reqID}
	}
}

// ── Segment helpers ──

func (t *BarTab) toggleSegment(field string) {
	switch field {
	case "git":
		t.segments.Git = !t.segments.Git
	case "workspace":
		t.segments.Workspace = !t.segments.Workspace
	case "clock":
		t.segments.Clock = !t.segments.Clock
	case "lang":
		t.segments.Lang = !t.segments.Lang
	case "directory":
		t.segments.Directory = !t.segments.Directory
	case "process":
		t.segments.Process = !t.segments.Process
	case "group":
		t.segments.Group = !t.segments.Group
	}
}

// ── Section helpers ──

func (t *BarTab) currentSection() barSection {
	P := len(t.presets)
	L := len(barLayoutOptions)
	if t.cursor < P {
		return barPresets
	}
	if t.cursor < P+L {
		return barLayout
	}
	return barSegments
}

// ── Layout helpers ──

func (t *BarTab) layoutValue(field string) string {
	// Defaults match config.DefaultConfig().
	switch field {
	case "layout":
		if t.layout == "" {
			return "two-line"
		}
		return t.layout
	case "top_bar":
		if t.topBar == "" {
			return "tabs"
		}
		return t.topBar
	case "indicator":
		if t.indicator == "" {
			return "dots"
		}
		return t.indicator
	}
	return ""
}

func (t *BarTab) cycleLayoutValue(field string, delta int) {
	for _, opt := range barLayoutOptions {
		if opt.Field != field {
			continue
		}
		current := t.layoutValue(field)
		idx := 0
		for i, v := range opt.Options {
			if v == current {
				idx = i
				break
			}
		}
		idx = (idx + delta + len(opt.Options)) % len(opt.Options)
		value := opt.Options[idx]
		switch field {
		case "layout":
			t.layout = value
			t.cfg.Bar.Layout = value
		case "top_bar":
			t.topBar = value
			t.cfg.Bar.TopBar = value
		case "indicator":
			t.indicator = value
			t.cfg.Bar.Indicator = value
		}
		return
	}
}

// ── Preview helpers ──

// previewSessions are mock session names for the two-line preview.
var previewSessions = []string{"main", "api", "tests"}

func (t *BarTab) renderPresetPreview(preset bar.Preset) string {
	width := t.width - 8
	if width < 40 {
		width = 60
	}

	switch t.layout {
	case "two-line", "split":
		topRow := bar.RenderTopPreviewVariant(
			preset, t.palette, previewSessions, previewSessions[0], width, t.topBar)
		if topRow == "" {
			return bar.RenderPreviewWithSegments(preset, t.palette, t.segments)
		}
		noWS := t.segments
		noWS.Workspace = false
		bottomRow := bar.RenderBarPreviewOverride(preset, t.palette, noWS, width,
			func(bctx *bar.BarContext) {
				switch t.indicator {
				case "dots":
					bctx.SessionIndicator = bar.CompactDots(previewSessions, previewSessions[0])
				case "none":
					bctx.WorkspaceCount = 1
				}
			})
		return topRow + "\n" + bottomRow
	default:
		return bar.RenderPreviewWithSegments(preset, t.palette, t.segments)
	}
}

func (t *BarTab) segmentEnabled(field string) bool {
	switch field {
	case "git":
		return t.segments.Git
	case "workspace":
		return t.segments.Workspace
	case "clock":
		return t.segments.Clock
	case "lang":
		return t.segments.Lang
	case "directory":
		return t.segments.Directory
	case "process":
		return t.segments.Process
	case "group":
		return t.segments.Group
	}
	return false
}
