// Package tabpicker is the light Alt+` switcher: a two-level
// session-and-tab picker scoped to the current workspace. Session level
// hops between sibling sessions (the cursor session previews its tabs
// inline); drilling with l/→ descends into that session's tabs where the
// per-tab ops (new / rename / close / reorder) live.
//
// It is deliberately a "light" cousin of the dashboard's Session tab:
// no viewport/process-stats/mutation machinery — it emits a single
// TabPickerResult that the CLI layer applies after the popup closes.
package tabpicker

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/donjor/zmux/internal/bar"
	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui/filter"
	"github.com/donjor/zmux/internal/tui/outline"
	"github.com/donjor/zmux/internal/tui/styles"
)

// tpMode tracks the textinput-driven sub-modes layered over list browsing.
type tpMode int

const (
	tpModeList   tpMode = iota
	tpModeNew           // text input for new tab name
	tpModeRename        // text input for rename
)

// navLevel is the two-level cursor scope. In session level the cursor hops
// session-to-session; in tab level it navigates the drilled session's tabs.
type navLevel int

const (
	navSession navLevel = iota
	navTab
)

// TabPickerResult holds the outcome applied by the CLI layer once the
// popup closes. Session names the target session for every action — tab
// ops can target a sibling session now, not just the current one.
type TabPickerResult struct {
	Action  string // "switch", "select", "select-pane", "show", "new", "rename", "close", "close-pane", "swap", ""
	Session string // target session name
	Index   int    // window index (tab actions)
	Name    string // tab name (new / rename)
	Delta   int    // swap direction (-1 / +1)
	Pane    string // pane id (select-pane / close-pane)
	TabID   string // logical tab id (MRU touch / show)
}

// tabEntryKind says what a picker row actually is: a window slot (raw
// window or full tab), a pane-of tab riding inside a host window, or a
// hidden pane-tab whose origin is this session.
type tabEntryKind int

const (
	teWindow tabEntryKind = iota
	teRider
	teHidden
)

// tabEntry is one picker row. Window is the host window for teWindow and
// teRider (selection needs its index); zero for teHidden. Tab carries the
// logical identity when the row is zmux-managed (nil for raw windows).
type tabEntry struct {
	tmux.Window
	Command string
	Kind    tabEntryKind
	Tab     *tabs.LogicalTab
}

// displayName is the row label: the logical label when one exists, else
// the window name (or the tab id for unlabeled riders/hidden panes).
func (t *tabEntry) displayName() string {
	if t.Tab != nil && (t.Kind != teWindow || t.Tab.Label != "") {
		return tabs.DisplayName(t.Tab)
	}
	return t.Name
}

// rowActive reports whether the row carries the active dot.
func (t *tabEntry) rowActive() bool {
	switch t.Kind {
	case teRider:
		return t.Tab != nil && t.Tab.Active
	case teHidden:
		return false
	}
	return t.Active
}

// stateGlyph returns the lifecycle glyph for the row's logical state
// (shared vocabulary with the bar); empty when stateless or raw.
func (t *tabEntry) stateGlyph() string {
	if t.Tab == nil {
		return ""
	}
	st, ok := t.Tab.StateOf()
	if !ok {
		return ""
	}
	return bar.StateGlyph(st)
}

// sessionEntry is one session plus its loaded tabs.
type sessionEntry struct {
	Info    session.SessionInfo
	Windows []tabEntry
}

// TabPickerModel is the two-level session+tab switcher.
type TabPickerModel struct {
	runner  tmux.Runner
	wsName  string
	current string                // current session name (root)
	infos   []session.SessionInfo // input session list, current-first

	entries  []sessionEntry // loaded (windows filled)
	filtered []sessionEntry // session-level filter applied

	tree  *outline.Tree
	input textinput.Model
	mode  tpMode
	nav   navLevel

	focused string // focused session (session level)
	drilled string // drilled session (tab level)

	// rename target (within drilled session)
	renameIdx int

	width  int
	height int
	styles styles.Styles

	Result   TabPickerResult
	Quitting bool
}

// NewTabPickerModel builds the switcher. infos is the workspace's live
// sessions; the model reorders them current-first and loads each one's
// windows on Init. wsName is shown in the header.
func NewTabPickerModel(runner tmux.Runner, wsName, current string, infos []session.SessionInfo, sty styles.Styles) TabPickerModel {
	ti := textinput.New()
	ti.Placeholder = "search sessions..."
	ti.CharLimit = 64
	ti.Focus()

	return TabPickerModel{
		runner:  runner,
		wsName:  wsName,
		current: current,
		infos:   orderCurrentFirst(infos, current),
		styles:  sty,
		input:   ti,
		focused: current,
		tree:    outline.NewTree(),
	}
}

// orderCurrentFirst returns infos with the current session moved to the
// front, preserving the relative order of the rest.
func orderCurrentFirst(infos []session.SessionInfo, current string) []session.SessionInfo {
	out := make([]session.SessionInfo, 0, len(infos))
	for _, s := range infos {
		if session.RootName(s.Name) == current {
			out = append(out, s)
		}
	}
	for _, s := range infos {
		if session.RootName(s.Name) != current {
			out = append(out, s)
		}
	}
	return out
}

type sessionsLoadedMsg struct {
	entries []sessionEntry
}

func (m TabPickerModel) Init() tea.Cmd {
	return tea.Batch(m.loadSessions(), textinput.Blink)
}

// loadSessions fetches windows + active-pane commands for every session,
// then decorates them from one whole-server logical scan: full tabs get
// their label/state, pane-of riders slot in under their host window, and
// docked tabs surface at the end of their origin session's list.
func (m TabPickerModel) loadSessions() tea.Cmd {
	runner := m.runner
	infos := m.infos
	return func() tea.Msg {
		var logical []tabs.LogicalTab
		if rows, err := runner.ListLogicalPaneRows(); err == nil {
			logical = tabs.FromRows(rows)
		}
		entries := make([]sessionEntry, 0, len(infos))
		for _, info := range infos {
			windows, _ := runner.ListWindows(info.Name)
			panes, _ := runner.ListPanes(info.Name)
			cmdByWin := make(map[int]string, len(panes))
			for _, p := range panes {
				if p.Active {
					cmdByWin[p.WindowIndex] = p.Command
				}
			}
			fullByIdx := make(map[int]*tabs.LogicalTab)
			ridersByIdx := make(map[int][]*tabs.LogicalTab)
			var hidden []*tabs.LogicalTab
			for i := range logical {
				t := &logical[i]
				switch {
				case t.Placement == tabs.PlacementDock:
					if t.OriginSession == info.Name {
						hidden = append(hidden, t)
					}
				case t.Session != info.Name:
				case t.Placement == tabs.PlacementFull:
					fullByIdx[t.WindowIndex] = t
				case t.Placement == tabs.PlacementPaneOf:
					ridersByIdx[t.WindowIndex] = append(ridersByIdx[t.WindowIndex], t)
				}
			}
			rows := make([]tabEntry, 0, len(windows))
			for _, w := range windows {
				rows = append(rows, tabEntry{Window: w, Command: cmdByWin[w.Index], Tab: fullByIdx[w.Index]})
				for _, r := range ridersByIdx[w.Index] {
					rows = append(rows, tabEntry{Window: w, Command: r.Command, Kind: teRider, Tab: r})
				}
			}
			for _, h := range hidden {
				rows = append(rows, tabEntry{Command: h.Command, Kind: teHidden, Tab: h})
			}
			entries = append(entries, sessionEntry{Info: info, Windows: rows})
		}
		return sessionsLoadedMsg{entries: entries}
	}
}

func (m TabPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case sessionsLoadedMsg:
		m.entries = msg.entries
		m.applyFilter()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	if m.mode == tpModeList {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.applyFilter()
		return m, cmd
	}
	return m, nil
}

func (m TabPickerModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.mode == tpModeNew || m.mode == tpModeRename {
		return m.handleInputMode(msg)
	}
	if m.nav == navTab {
		return m.handleTabLevel(msg)
	}
	return m.handleSessionLevel(msg)
}

// handleInputMode handles the new-tab / rename text-entry sub-modes.
func (m TabPickerModel) handleInputMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		name := strings.TrimSpace(m.input.Value())
		if m.mode == tpModeNew {
			m.Result = TabPickerResult{Action: "new", Session: m.drilled, Name: name}
		} else {
			m.Result = TabPickerResult{Action: "rename", Session: m.drilled, Index: m.renameIdx, Name: name}
		}
		m.Quitting = true
		return m, tea.Quit
	case "esc":
		m.mode = tpModeList
		m.input.SetValue("")
		m.input.Placeholder = "search tabs..."
		m.applyFilter()
		return m, nil
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
}

// handleSessionLevel handles keys while the cursor hops sessions.
func (m TabPickerModel) handleSessionLevel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.Quitting = true
		return m, tea.Quit

	case "esc":
		if m.input.Value() != "" {
			m.input.SetValue("")
			m.applyFilter()
			return m, nil
		}
		m.Quitting = true
		return m, tea.Quit

	case "up", "ctrl+p":
		m.moveFocus(-1)
		return m, nil

	case "down", "ctrl+n":
		m.moveFocus(+1)
		return m, nil

	case "enter":
		if m.focused != "" {
			m.Result = TabPickerResult{Action: "switch", Session: m.focused}
			m.Quitting = true
			return m, tea.Quit
		}
		return m, nil

	case "right", "l", "tab":
		// Drill into the focused session's tabs (if it has any).
		if e := m.entryByName(m.focused); e != nil && len(e.Windows) > 0 {
			m.nav = navTab
			m.drilled = m.focused
			m.input.SetValue("")
			m.input.Placeholder = "search tabs..."
			m.rebuild()
			m.tree.JumpTop()
		}
		return m, nil
	}

	// Anything else feeds the session filter.
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.applyFilter()
	return m, cmd
}

// handleTabLevel handles keys while the cursor navigates a session's tabs.
func (m TabPickerModel) handleTabLevel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.Quitting = true
		return m, tea.Quit

	case "esc":
		if m.input.Value() != "" {
			m.input.SetValue("")
			m.applyFilter()
			return m, nil
		}
		m.backToSessions()
		return m, nil

	case "left", "h":
		if m.input.Value() == "" {
			m.backToSessions()
		}
		return m, nil

	case "up":
		m.tree.MoveUp()
		return m, nil

	case "down":
		m.tree.MoveDown()
		return m, nil

	case "enter":
		if t := m.currentTab(); t != nil {
			m.Result = m.selectResult(t)
			m.Quitting = true
			return m, tea.Quit
		}
		return m, nil

	case "ctrl+n":
		m.mode = tpModeNew
		m.input.SetValue("")
		m.input.Placeholder = "new tab name (blank for default)..."
		return m, nil

	case "ctrl+r":
		// Rename renames the WINDOW — riders and hidden tabs have no window
		// of their own (zmux tab label owns logical labels).
		if t := m.currentTab(); t != nil && t.Kind == teWindow {
			m.renameIdx = t.Index
			m.mode = tpModeRename
			m.input.SetValue(t.Name)
			m.input.Placeholder = "rename..."
		}
		return m, nil

	case "ctrl+x":
		if t := m.currentTab(); t != nil {
			if t.Kind == teWindow {
				m.Result = TabPickerResult{Action: "close", Session: m.drilled, Index: t.Index}
			} else if t.Tab != nil {
				// Riders and hidden tabs die by pane — killing the host
				// window (or the dock's) would take innocents with it.
				m.Result = TabPickerResult{Action: "close-pane", Session: m.drilled, Pane: t.Tab.PaneID}
			}
			m.Quitting = true
			return m, tea.Quit
		}
		return m, nil

	case "ctrl+left", "<":
		if m.input.Value() == "" {
			if t := m.currentTab(); t != nil && t.Kind == teWindow {
				m.Result = TabPickerResult{Action: "swap", Session: m.drilled, Index: t.Index, Delta: -1}
				m.Quitting = true
				return m, tea.Quit
			}
		}
		return m, nil

	case "ctrl+right", ">":
		if m.input.Value() == "" {
			if t := m.currentTab(); t != nil && t.Kind == teWindow {
				m.Result = TabPickerResult{Action: "swap", Session: m.drilled, Index: t.Index, Delta: 1}
				m.Quitting = true
				return m, tea.Quit
			}
		}
		return m, nil
	}

	// Anything else feeds the tab filter.
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.applyFilter()
	return m, cmd
}

// selectResult maps the cursor row to its selection action: windows select
// by index, riders focus their pane inside the host window, and hidden panes
// rejoin their recorded parent via show.
func (m TabPickerModel) selectResult(t *tabEntry) TabPickerResult {
	res := TabPickerResult{Session: m.drilled}
	if t.Tab != nil {
		res.TabID = t.Tab.ID
	}
	switch t.Kind {
	case teRider:
		res.Action = "select-pane"
		res.Index = t.Index
		res.Pane = t.Tab.PaneID
	case teHidden:
		res.Action = "show"
	default:
		res.Action = "select"
		res.Index = t.Index
	}
	return res
}

// backToSessions returns the cursor to session level, re-pinning it on the
// session that was drilled.
func (m *TabPickerModel) backToSessions() {
	m.nav = navSession
	m.focused = m.drilled
	m.drilled = ""
	m.input.SetValue("")
	m.input.Placeholder = "search sessions..."
	m.applyFilter()
}

// moveFocus shifts the session-level cursor by delta within the filtered
// sessions and re-pins the tree on the new focus (which re-expands it).
func (m *TabPickerModel) moveFocus(delta int) {
	if len(m.filtered) == 0 {
		return
	}
	idx := m.focusedIndex()
	idx += delta
	if idx < 0 {
		idx = 0
	}
	if idx >= len(m.filtered) {
		idx = len(m.filtered) - 1
	}
	m.focused = m.filtered[idx].Info.Name
	m.rebuild()
	m.tree.JumpToID(outline.SessionID(m.focused))
}

// focusedIndex returns the index of the focused session in filtered, or 0.
func (m TabPickerModel) focusedIndex() int {
	for i := range m.filtered {
		if m.filtered[i].Info.Name == m.focused {
			return i
		}
	}
	return 0
}

// entryByName returns the loaded entry for a session name, or nil.
func (m *TabPickerModel) entryByName(name string) *sessionEntry {
	for i := range m.entries {
		if m.entries[i].Info.Name == name {
			return &m.entries[i]
		}
	}
	return nil
}

// currentTab returns the tab under the cursor at tab level, or nil.
func (m TabPickerModel) currentTab() *tabEntry {
	row := m.tree.CurrentSelectable()
	if row == nil {
		return nil
	}
	te, _ := outline.RowData[tabEntry](row)
	return te
}

// applyFilter narrows by the current query — sessions at session level,
// the drilled session's tabs at tab level — then rebuilds the tree.
func (m *TabPickerModel) applyFilter() {
	if m.nav == navSession {
		m.filtered = filter.Fuzzy(m.entries, m.input.Value(), func(e sessionEntry) string {
			return session.RootName(e.Info.Name)
		})
		if !m.entryInFiltered(m.focused) {
			if len(m.filtered) > 0 {
				m.focused = m.filtered[0].Info.Name
			} else {
				m.focused = ""
			}
		}
	} else {
		// At tab level the session set is fixed to the drilled one; the
		// query filters its windows inside rebuild().
		m.filtered = m.entries
	}
	m.rebuild()
	if m.nav == navSession && m.focused != "" {
		m.tree.JumpToID(outline.SessionID(m.focused))
	}
}

// entryInFiltered reports whether name is present in the filtered set.
func (m TabPickerModel) entryInFiltered(name string) bool {
	for i := range m.filtered {
		if m.filtered[i].Info.Name == name {
			return true
		}
	}
	return false
}

// rebuild constructs the outline rows for the current nav state and pushes
// them into the tree (which preserves the cursor by stable ID).
func (m *TabPickerModel) rebuild() {
	m.tree.SetRows(m.buildRows())
}

// buildRows lays out sessions (depth 0) with the expanded session's tabs
// (depth 1) inline. At session level only sessions are selectable and the
// focused session is expanded for preview; at tab level only the drilled
// session's (filtered) tabs are selectable.
func (m *TabPickerModel) buildRows() []outline.Row {
	rows := make([]outline.Row, 0, len(m.filtered)*2)

	for i := range m.filtered {
		s := &m.filtered[i]
		name := s.Info.Name
		sid := outline.SessionID(name)

		expanded := (m.nav == navSession && name == m.focused) ||
			(m.nav == navTab && name == m.drilled)

		rows = append(rows, outline.Row{
			ID:         sid,
			Kind:       outline.RowSession,
			Depth:      0,
			Label:      session.RootName(name),
			Selectable: m.nav == navSession,
			Current:    session.RootName(name) == m.current,
			Attached:   s.Info.Attached,
			Expanded:   expanded,
			Data:       &s.Info,
		})

		if !expanded {
			continue
		}

		wins := s.Windows
		if m.nav == navTab && name == m.drilled {
			wins = filter.Fuzzy(s.Windows, m.input.Value(), func(e tabEntry) string { return e.displayName() })
		}
		for j := range wins {
			w := &wins[j]
			id := outline.WindowID(name, w.Index)
			if w.Kind != teWindow && w.Tab != nil {
				// Riders/hidden share (or lack) a window — the pane id is
				// the stable cursor anchor.
				id = outline.PaneID(name, w.Tab.PaneID)
			}
			rows = append(rows, outline.Row{
				ID:         id,
				Kind:       outline.RowWindow,
				Depth:      1,
				ParentID:   sid,
				Label:      w.displayName(),
				Selectable: m.nav == navTab && name == m.drilled,
				Attached:   w.rowActive(),
				Data:       w,
			})
		}
	}
	return rows
}
