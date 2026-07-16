package tabs

// Bar tab — status bar preset, layout, and segment management.
//
// Split across:
//   - bar.go         — messages, types, BarTab struct, lifecycle, Update, key handling
//   - bar_view.go    — View + preset preview rendering
//   - bar_helpers.go — fetchData / saveConfig + segment / section / layout helpers

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/donjor/zmux/internal/bar"
	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/keys"
	"github.com/donjor/zmux/internal/theme"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui/dashboard"
	"github.com/donjor/zmux/internal/tui/styles"
)

// Bar-specific keys with no cross-surface analogue: horizontal value cycling
// (left/right) and the enter/space toggle (space also inserts in text surfaces,
// so it is not in the shared registry). Built once as package-level bindings
// (idiom A); generic up/down/g/G come from keys.TUI*.
var (
	barCycleLeftKey  = key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "prev value"))
	barCycleRightKey = key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "next value"))
	barToggleKey     = key.NewBinding(key.WithKeys("enter", "space"), key.WithHelp("enter/space", "toggle"))
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
	{"Layout", "layout", []string{"two-line", "split"}},
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
	selfBin  string // binary embedded in #(<bin> bar-render); config.SelfBin(profile)
	styles   styles.Styles

	presets []bar.Preset
	cursor  int

	// Resolved palette for bar previews.
	palette *theme.Palette

	cfg       config.Config
	cfgExists bool

	reqID int64

	vp            viewport.Model
	width, height int
}

// NewBarTab creates a new bar tab. selfBin is the binary embedded in the
// generated bar's #(<bin> bar-render) content — pass config.SelfBin(profile).
func NewBarTab(resolver *theme.Resolver, fs config.FS, runner tmux.Runner, selfBin string, styles styles.Styles) *BarTab {
	return &BarTab{
		resolver: resolver,
		fs:       fs,
		runner:   runner,
		selfBin:  selfBin,
		styles:   styles,
		presets:  bar.AllPresets(),
	}
}

func (t *BarTab) ID() dashboard.TabID { return dashboard.TabBar }
func (t *BarTab) Title() string       { return "Bar" }
func (t *BarTab) Init() tea.Cmd       { return nil }

// CapturesEscape is always false — the Bar tab has no inline input mode, so
// Esc closes the dashboard.
func (t *BarTab) CapturesEscape() bool { return false }

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
	t.vp.SetWidth(w)
	t.vp.SetHeight(h)
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
	case key.Matches(msg, keys.TUIListUp):
		if t.cursor > 0 {
			t.cursor--
		}
		return t, nil

	case key.Matches(msg, keys.TUIListDown):
		if t.cursor < total-1 {
			t.cursor++
		}
		return t, nil

	case key.Matches(msg, barCycleLeftKey):
		if t.currentSection() == barLayout {
			layoutIdx := t.cursor - len(t.presets)
			t.cycleLayoutValue(barLayoutOptions[layoutIdx].Field, -1)
			return t, t.saveConfig()
		}
		return t, nil

	case key.Matches(msg, barCycleRightKey):
		if t.currentSection() == barLayout {
			layoutIdx := t.cursor - len(t.presets)
			t.cycleLayoutValue(barLayoutOptions[layoutIdx].Field, 1)
			return t, t.saveConfig()
		}
		return t, nil

	case key.Matches(msg, barToggleKey):
		switch t.currentSection() {
		case barPresets:
			preset := t.presets[t.cursor]
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
				return t, t.saveConfig()
			}
		}
		return t, nil

	case key.Matches(msg, keys.TUIListTop):
		t.cursor = 0
		return t, nil

	case key.Matches(msg, keys.TUIListBottom):
		t.cursor = total - 1
		return t, nil
	}

	return t, nil
}
