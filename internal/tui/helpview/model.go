// Package helpview is the prefix+? interactive help surface: a scrollable,
// fuzzy-filterable viewer over the shared help content (internal/help). It
// replaces the old `zmux help` text dump that overflowed its popup with no way
// to scroll or search.
package helpview

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/sahilm/fuzzy"

	"github.com/donjor/zmux/internal/help"
	"github.com/donjor/zmux/internal/tui/scroll"
	"github.com/donjor/zmux/internal/tui/styles"
)

// scopeMode selects which sections the viewer shows: all, commands only, or
// keybindings only. Cycled with Tab.
type scopeMode int

const (
	scopeAll scopeMode = iota
	scopeCommands
	scopeKeys
)

// Model is the bubbletea model for the help viewer.
type Model struct {
	sections []help.Section
	filter   textinput.Model
	vp       viewport.Model
	styles   styles.Styles
	scope    scopeMode
	width    int
	height   int
	ready    bool

	Quitting bool
}

// New builds a help viewer over the given sections.
func New(sections []help.Section, st styles.Styles) *Model {
	ti := textinput.New()
	ti.Placeholder = "type to search help..."
	ti.Prompt = "" // we render our own "  > " prompt in view()
	ti.CharLimit = 128
	ti.Focus()
	return &Model{sections: sections, filter: ti, styles: st}
}

// Init starts the filter cursor blinking.
func (m *Model) Init() tea.Cmd { return textinput.Blink }

// Update handles resize, navigation, quit, and filter input.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Body height = total minus title (2), filter (2), footer (2).
		// -2 width reserves the trailing scrollbar column (" ▐").
		m.vp.SetWidth(max(1, msg.Width-2))
		m.vp.SetHeight(max(1, msg.Height-6))
		m.filter.SetWidth(max(1, msg.Width-6)) // room for the "  > " prompt
		m.ready = true
		m.refresh()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc", "ctrl+c"))):
		m.Quitting = true
		return m, tea.Quit

	// Scroll the body. Arrows + ctrl+j/k so plain j/k stay typeable in the filter.
	case key.Matches(msg, key.NewBinding(key.WithKeys("up", "ctrl+k"))):
		m.vp.ScrollUp(1)
		return m, nil
	case key.Matches(msg, key.NewBinding(key.WithKeys("down", "ctrl+j"))):
		m.vp.ScrollDown(1)
		return m, nil
	case key.Matches(msg, key.NewBinding(key.WithKeys("pgup", "ctrl+u"))):
		m.vp.ScrollUp(m.vp.Height() / 2)
		return m, nil
	case key.Matches(msg, key.NewBinding(key.WithKeys("pgdown", "ctrl+d"))):
		m.vp.ScrollDown(m.vp.Height() / 2)
		return m, nil

	// Cycle the scope filter: all -> commands -> keys -> all.
	case key.Matches(msg, key.NewBinding(key.WithKeys("tab"))):
		m.scope = (m.scope + 1) % 3
		m.refresh()
		return m, nil
	case key.Matches(msg, key.NewBinding(key.WithKeys("shift+tab"))):
		m.scope = (m.scope + 2) % 3
		m.refresh()
		return m, nil
	}

	// Everything else edits the filter.
	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
	m.refresh()
	return m, cmd
}

// visibleSections applies the scope filter (the commands/keys/all toggle) before
// the fuzzy text filter narrows within it.
func (m *Model) visibleSections() []help.Section {
	switch m.scope {
	case scopeCommands:
		return help.FilterScope(m.sections, help.ScopeCommand)
	case scopeKeys:
		return help.FilterScope(m.sections, help.ScopeKeybinding)
	default:
		return m.sections
	}
}

// refresh re-renders the filtered content into the viewport and resets scroll.
func (m *Model) refresh() {
	if !m.ready {
		return
	}
	m.vp.SetContent(m.renderSections(FilterSections(m.visibleSections(), m.filter.Value())))
	m.vp.GotoTop()
}

// View renders the full-screen viewer.
func (m *Model) View() tea.View {
	v := tea.NewView(m.view())
	v.AltScreen = true
	return v
}

func (m *Model) view() string {
	if m.Quitting || !m.ready {
		return ""
	}
	var b strings.Builder
	title := m.styles.Accent.Bold(true).Render("zmux")
	scopeName := [...]string{"all", "commands", "keys"}[m.scope]
	b.WriteString("\n  " + title + m.styles.Dim.Render(" help") +
		m.styles.Dim.Render("   ·  showing ") + m.styles.Normal.Render(scopeName) + "\n\n")
	b.WriteString(m.styles.Accent.Render("  > ") + m.filter.View() + "\n\n")
	b.WriteString(scroll.Scrollable(m.vp, m.styles) + "\n")
	b.WriteString("  " + m.styles.Dim.Render("type:filter  ⇄ tab:scope  ↑/↓:scroll  esc:close"))
	return b.String()
}

// renderSections renders filtered sections into the scrollable body, grouped
// under scope bands (Commands / Keybindings) with each section's descriptions
// aligned into a column. An empty result shows a hint instead of a blank pane.
func (m *Model) renderSections(sections []help.Section) string {
	if len(sections) == 0 {
		return "  " + m.styles.Dim.Render("No matching help.")
	}
	var b strings.Builder
	shown := map[help.Scope]bool{}
	for i, s := range sections {
		if i > 0 {
			b.WriteString("\n")
		}
		if !shown[s.Scope] {
			b.WriteString("  " + m.bandHeader(s.Scope) + "\n\n")
			shown[s.Scope] = true
		}
		b.WriteString("  " + m.styles.Accent.Bold(true).Render(s.Title) + "\n")
		b.WriteString(m.renderEntries(s.Entries))
	}
	return b.String()
}

// bandHeader is the scope divider above a run of same-scope sections.
func (m *Model) bandHeader(scope help.Scope) string {
	head := m.styles.Dim.Bold(true).Render("── " + help.BandLabel(scope) + " ──")
	if scope == help.ScopeKeybinding {
		// ponytail: prefix is hardcoded Ctrl+Space, matching the generated conf
		// default the help has always assumed.
		head += m.styles.Dim.Render("  prefix = Ctrl+Space")
	}
	return head
}

// renderEntries aligns each entry's description into a column. The label column
// is capped so one long label can't push every description off-screen.
func (m *Model) renderEntries(entries []help.Entry) string {
	const labelCap = 30
	col := 0
	for _, e := range entries {
		if w := lipgloss.Width(e.Label); w > col {
			col = w
		}
	}
	if col > labelCap {
		col = labelCap
	}
	var b strings.Builder
	for _, e := range entries {
		pad := col - lipgloss.Width(e.Label)
		if pad < 1 {
			pad = 1
		}
		b.WriteString("    " + m.styles.Normal.Render(e.Label) +
			strings.Repeat(" ", pad+1) + m.styles.Dim.Render(e.Desc) + "\n")
	}
	return b.String()
}

// FilterSections returns the sections whose entries fuzzy-match the query,
// preserving the original section/entry order and dropping empty sections. An
// empty query returns every section. Pure, so the filter is unit-testable.
func FilterSections(sections []help.Section, query string) []help.Section {
	if strings.TrimSpace(query) == "" {
		return sections
	}

	// Flatten to one searchable string per entry, remembering its origin.
	type origin struct{ sec, ent int }
	var texts []string
	var origins []origin
	for si, s := range sections {
		for ei, e := range s.Entries {
			texts = append(texts, e.Label+" "+e.Desc)
			origins = append(origins, origin{si, ei})
		}
	}

	keep := make([]map[int]bool, len(sections))
	for _, match := range fuzzy.Find(query, texts) {
		o := origins[match.Index]
		if keep[o.sec] == nil {
			keep[o.sec] = map[int]bool{}
		}
		keep[o.sec][o.ent] = true
	}

	out := make([]help.Section, 0, len(sections))
	for si, s := range sections {
		if keep[si] == nil {
			continue
		}
		kept := make([]help.Entry, 0, len(s.Entries))
		for ei, e := range s.Entries {
			if keep[si][ei] {
				kept = append(kept, e)
			}
		}
		out = append(out, help.Section{Title: s.Title, Scope: s.Scope, Entries: kept})
	}
	return out
}
