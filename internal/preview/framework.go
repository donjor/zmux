// Package preview is a reusable TUI harness for iterating on UI visuals
// outside the production rendering paths. Individual UI surfaces (status
// bar, dashboard rows, picker chrome) register as Pages; the framework
// hosts a page switcher + per-page controls panel + live preview area.
//
// The goal is short-loop visual iteration: change a control, see the
// result rendered at the terminal's actual width. Pages are expected
// to call real rendering code where possible so what the proto shows
// is what the real UI will show.
package preview

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Page is one previewable UI surface.
type Page interface {
	ID() string
	Title() string
	Controls() []Control
	Render(ctx RenderContext) string
}

// RenderContext is everything a Page needs to paint itself. Values is
// keyed by ControlID — pages look up their own controls by ID.
type RenderContext struct {
	Width  int
	Height int
	Values map[ControlID]any
}

// Control is an adjustable input. Implementations live in controls.go.
type Control interface {
	ID() ControlID
	Label() string
	View(focused bool) string
	Handle(key tea.KeyMsg) bool
	Value() any
}

// ControlID is a stable identifier a Page uses to find a specific
// control's value in RenderContext.Values.
type ControlID string

// App is the bubbletea model — owns page list, active page, control
// focus, and per-page values.
type App struct {
	pages  []Page
	active int
	focus  int // -1 = preview area, 0..N-1 = control index

	// values[page.ID()][control.ID()] = current value.
	values map[string]map[ControlID]any

	width  int
	height int
}

// NewApp builds the host TUI model registered with the given pages.
func NewApp(pages ...Page) *App {
	app := &App{
		pages:  pages,
		focus:  -1,
		values: make(map[string]map[ControlID]any),
	}
	// Seed each page's value map from its controls' initial values.
	for _, p := range pages {
		vals := make(map[ControlID]any)
		for _, c := range p.Controls() {
			vals[c.ID()] = c.Value()
		}
		app.values[p.ID()] = vals
	}
	return app
}

// Init implements tea.Model.
func (a *App) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = m.Width
		a.height = m.Height
		return a, nil
	case tea.KeyMsg:
		return a.handleKey(m)
	}
	return a, nil
}

func (a *App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global quits first.
	switch msg.String() {
	case "q", "ctrl+c", "esc":
		return a, tea.Quit
	case "tab":
		a.active = (a.active + 1) % len(a.pages)
		a.focus = -1
		return a, nil
	case "shift+tab":
		a.active = (a.active - 1 + len(a.pages)) % len(a.pages)
		a.focus = -1
		return a, nil
	}

	// If focused on a control, let it handle first.
	ctrls := a.pages[a.active].Controls()
	if a.focus >= 0 && a.focus < len(ctrls) {
		if ctrls[a.focus].Handle(msg) {
			// Control consumed the key — sync its value back into the map.
			a.values[a.pages[a.active].ID()][ctrls[a.focus].ID()] = ctrls[a.focus].Value()
			return a, nil
		}
	}

	// Navigation between controls.
	switch msg.String() {
	case "up", "k":
		if a.focus > 0 {
			a.focus--
		} else if a.focus == -1 {
			a.focus = len(ctrls) - 1
		}
	case "down", "j":
		if a.focus < len(ctrls)-1 {
			a.focus++
		}
	}
	return a, nil
}

// View implements tea.Model.
func (a *App) View() tea.View {
	v := tea.NewView(a.view())
	v.AltScreen = true
	return v
}

func (a *App) view() string {
	if len(a.pages) == 0 {
		return "no pages registered"
	}
	if a.width < 40 || a.height < 12 {
		return fmt.Sprintf("terminal too small (%dx%d) — resize to at least 60x16", a.width, a.height)
	}

	var b strings.Builder
	page := a.pages[a.active]

	b.WriteString(renderHero(a.width))
	b.WriteString("\n")
	b.WriteString(renderPageTabs(a.pages, a.active))
	b.WriteString("\n")
	b.WriteString(renderDivider(a.width))
	b.WriteString("\n")

	availableH := a.height - 9
	if availableH < 10 {
		availableH = 10
	}

	controlW := 42
	previewW := a.width - controlW - 7
	wideEnoughForSideBySide := a.width >= 150 && previewW >= 96 && availableH >= 20
	if wideEnoughForSideBySide {
		ctx := RenderContext{
			Width:  previewW - 4,
			Height: availableH,
			Values: a.values[page.ID()],
		}
		left := renderControlPanel(page.Controls(), a.focus, controlW)
		right := renderPreviewPanel(page.Title(), page.Render(ctx), previewW, availableH, a.focus == -1)
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right))
		b.WriteString("\n")
	} else {
		b.WriteString(renderControlPanel(page.Controls(), a.focus, a.width-4))
		b.WriteString("\n")
		ctx := RenderContext{
			Width:  a.width - 8,
			Height: availableH,
			Values: a.values[page.ID()],
		}
		b.WriteString(renderPreviewPanel(page.Title(), page.Render(ctx), a.width-4, availableH, a.focus == -1))
		b.WriteString("\n")
	}

	b.WriteString(renderFooter())
	return b.String()
}
