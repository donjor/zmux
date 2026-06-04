// Package workspacelist is the embeddable workspace-list controller. Phase 020
// landed the seam; phase 021 adopted it in the M-w workspace switcher
// (internal/tui/wspicker), which is currently its only caller.
//
// The dashboard Workspaces tab (internal/tui/dashboard/tabs) keeps its own
// outline-based implementation: it needs external-source rows, inline move,
// and two-step kill confirms that exceed this component's flat-list surface.
// Folding it in here remains a possible follow-up, not a done deal.
//
// Capability inference: each side-effect (Select / Create / Rename / Delete)
// is gated by the corresponding hook callback being non-nil. The component
// will not surface a key or footer entry for a capability whose hook is
// missing — keeping caller-side behaviour explicit rather than mode-flagged.
//
// See .dump/plans/020_*/C-SEAM.md for the full surface contract.
package workspacelist

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/sahilm/fuzzy"

	"github.com/donjor/zmux/internal/tui/outline"
	"github.com/donjor/zmux/internal/tui/styles"
	"github.com/donjor/zmux/internal/tui/workspaceview"
)

// Config bundles inputs to a new Model. All hook callbacks are optional;
// a nil hook disables the corresponding key + footer entry.
type Config struct {
	// Workspaces is the initial list. Callers refetch externally and pass
	// the new slice through the Workspaces field of a Reload message.
	Workspaces []workspaceview.WorkspaceViewModel

	// Display flags (pure render toggles, not capability gates).
	ShowEmpty bool

	// Hooks: callers supply side effects. Non-nil hook = key is active.
	OnSelect func(ws workspaceview.WorkspaceViewModel) tea.Cmd
	OnCreate func(workspaceName, sessionName string) tea.Cmd
	OnRename func(old, newName string) tea.Cmd
	OnDelete func(name string) tea.Cmd

	Styles styles.Styles
}

// ReloadMsg pushes a fresh workspace slice into the component without
// rebuilding it. Callers can publish this in response to mutations.
type ReloadMsg struct {
	Workspaces []workspaceview.WorkspaceViewModel
}

// Model is a fuzzy-filtered list of workspaces with capability-gated CRUD.
type Model struct {
	cfg            Config
	all            []workspaceview.WorkspaceViewModel
	filtered       []workspaceview.WorkspaceViewModel
	tree           *outline.Tree
	input          textinput.Model
	mode           mode
	renameOriginal string
}

type mode int

const (
	modeList mode = iota
	modeRename
)

// New constructs a Model. Init returns a no-op tea.Cmd — callers may want
// to chain their own setup commands.
func New(cfg Config) Model {
	ti := textinput.New()
	ti.Placeholder = "search workspaces..."
	ti.CharLimit = 64
	ti.Focus()

	m := Model{
		cfg:   cfg,
		all:   cfg.Workspaces,
		tree:  outline.NewTree(),
		input: ti,
	}
	m.applyFilter()
	return m
}

func (m Model) Init() tea.Cmd { return textinput.Blink }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ReloadMsg:
		m.all = msg.Workspaces
		m.applyFilter()
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if m.mode == modeList {
		m.applyFilter()
	}
	return m, cmd
}

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.mode == modeRename {
		return m.handleRenameKey(msg)
	}

	switch msg.String() {
	case "esc", "ctrl+c":
		if m.input.Value() != "" {
			m.input.SetValue("")
			m.applyFilter()
			return m, nil
		}
		// Bubble out — host decides what closing means.
		return m, nil

	case "up":
		m.tree.MoveUp()
		return m, nil
	case "down":
		m.tree.MoveDown()
		return m, nil

	case "enter":
		// `<ws> <session>` create grammar wins ahead of select — the user
		// has typed an explicit target name and expects creation, even when
		// no workspace row is focused (e.g. when the workspace does not
		// exist yet).
		raw := strings.TrimSpace(m.input.Value())
		if idx := strings.IndexByte(raw, ' '); idx >= 0 && m.cfg.OnCreate != nil {
			wsName := strings.TrimSpace(raw[:idx])
			sessName := strings.TrimSpace(raw[idx+1:])
			if wsName != "" && sessName != "" {
				return m, m.cfg.OnCreate(wsName, sessName)
			}
		}
		if ws := m.focused(); ws != nil && m.cfg.OnSelect != nil {
			return m, m.cfg.OnSelect(*ws)
		}
		return m, nil

	case "ctrl+x":
		if m.cfg.OnDelete == nil {
			return m, nil
		}
		ws := m.focused()
		if ws == nil {
			return m, nil
		}
		return m, m.cfg.OnDelete(ws.Name)

	case "ctrl+r":
		if m.cfg.OnRename == nil {
			return m, nil
		}
		ws := m.focused()
		if ws == nil {
			return m, nil
		}
		m.mode = modeRename
		m.renameOriginal = ws.Name
		m.input.SetValue(ws.Name)
		m.input.Placeholder = "rename..."
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.applyFilter()
	return m, cmd
}

func (m Model) handleRenameKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		newName := strings.TrimSpace(m.input.Value())
		old := m.renameOriginal
		m.mode = modeList
		m.input.SetValue("")
		m.input.Placeholder = "search workspaces..."
		m.applyFilter()
		if newName == "" || newName == old || m.cfg.OnRename == nil {
			return m, nil
		}
		return m, m.cfg.OnRename(old, newName)
	case "esc":
		m.mode = modeList
		m.input.SetValue("")
		m.input.Placeholder = "search workspaces..."
		m.applyFilter()
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) focused() *workspaceview.WorkspaceViewModel {
	row := m.tree.CurrentSelectable()
	if row == nil {
		return nil
	}
	ws, _ := outline.RowData[workspaceview.WorkspaceViewModel](row)
	return ws
}

// FilterText returns the current search input. Hosts may surface this in a
// ghost-prompt or breadcrumb strip outside the component.
func (m Model) FilterText() string { return m.input.Value() }

// Focused returns the workspace under the cursor, or nil.
func (m Model) Focused() *workspaceview.WorkspaceViewModel { return m.focused() }

func (m *Model) applyFilter() {
	query := strings.TrimSpace(m.input.Value())
	// In create grammar, the part before the first space is the workspace
	// filter; the part after is the session name. Filter on the workspace
	// part only. This split only applies when the consumer enabled creation
	// (OnCreate != nil); a select-only consumer (e.g. the wspicker switcher)
	// has no create grammar, so it filters on the full query rather than
	// silently dropping everything after a space.
	if m.cfg.OnCreate != nil {
		if idx := strings.IndexByte(query, ' '); idx >= 0 {
			query = query[:idx]
		}
	}

	source := m.all
	if !m.cfg.ShowEmpty {
		// Hide workspaces with no live sessions when the caller has not
		// opted into empty-state. Callers that want every workspace
		// regardless (e.g. dashboard CRUD) set ShowEmpty.
		filtered := source[:0:0]
		for _, ws := range m.all {
			if len(ws.LiveSessions) == 0 {
				continue
			}
			filtered = append(filtered, ws)
		}
		source = filtered
	}

	if query == "" {
		m.filtered = source
	} else {
		names := make([]string, len(source))
		for i, ws := range source {
			names[i] = ws.Name
		}
		matches := fuzzy.Find(query, names)
		m.filtered = make([]workspaceview.WorkspaceViewModel, len(matches))
		for i, match := range matches {
			m.filtered[i] = source[match.Index]
		}
	}
	m.tree.SetRows(m.buildRows())
}

func (m *Model) buildRows() []outline.Row {
	rows := make([]outline.Row, len(m.filtered))
	for i := range m.filtered {
		ws := &m.filtered[i]
		rows[i] = outline.Row{
			ID:         outline.WorkspaceID(ws.Name),
			Kind:       outline.RowWorkspaceHeader,
			Label:      ws.Name,
			Selectable: true,
			Data:       ws,
		}
	}
	return rows
}

// View renders the list. Phase 020 ships a deliberately simple view so the
// surface is stable; adopters in plan 021+ may either reuse it or render
// their own chrome around the same Model state.
func (m Model) View() string {
	var b strings.Builder
	switch m.mode {
	case modeRename:
		b.WriteString("  " + m.cfg.Styles.Accent.Render("rename ▸ ") + m.input.View() + "\n\n")
	default:
		b.WriteString("  " + m.cfg.Styles.Accent.Render("▸ ") + m.input.View() + "\n\n")
	}

	if len(m.filtered) == 0 {
		b.WriteString(m.cfg.Styles.Muted.Render("  no workspaces") + "\n")
	} else {
		for i, ws := range m.filtered {
			selected := i == m.tree.Cursor
			cursor := "  "
			if selected {
				cursor = m.cfg.Styles.Accent.Render("▸ ")
			}
			nameStyle := m.cfg.Styles.Normal.Bold(true)
			if selected {
				nameStyle = m.cfg.Styles.Accent.Bold(true)
			}
			name := nameStyle.Render(ws.Name)
			meta := m.cfg.Styles.Dim.Render(outline.FormatSessionCount(len(ws.LiveSessions)))
			b.WriteString("  " + cursor + name + "  " + meta + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(m.cfg.Styles.Help.Render("  " + m.helpFooter()))
	b.WriteString("\n")
	return b.String()
}

// helpFooter is capability-gated — entries only appear when their hook is set.
func (m Model) helpFooter() string {
	parts := []string{}
	if m.cfg.OnSelect != nil {
		parts = append(parts, "enter:select")
	}
	if m.cfg.OnCreate != nil {
		parts = append(parts, "space+name:new")
	}
	if m.cfg.OnRename != nil {
		parts = append(parts, "ctrl+r:rename")
	}
	if m.cfg.OnDelete != nil {
		parts = append(parts, "ctrl+x:delete")
	}
	parts = append(parts, "esc:quit")
	return strings.Join(parts, "  ")
}
