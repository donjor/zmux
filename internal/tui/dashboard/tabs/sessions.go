// Package tabs implements the individual dashboard tab models.
package tabs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/source"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui"
	"github.com/donjor/zmux/internal/tui/dashboard"
	"github.com/donjor/zmux/internal/tui/views"
)

// sessionsMode tracks the current interaction mode.
type sessionsMode int

const (
	sessionsModeList        sessionsMode = iota // browsing the session list
	sessionsModeRename                          // inline rename input
	sessionsModeConfirmKill                     // y/N confirmation
	sessionsModeMove                            // destination picker
	sessionsModePreview                         // captured pane preview
)

// sessionsReqID is a monotonic counter for async correctness.
var sessionsReqCounter atomic.Int64

// ── Messages ──

// sessionsDataMsg carries fetched session data.
type sessionsDataMsg struct {
	reqID    int64
	sessions []session.SessionInfo
	windows  map[string][]tmux.Window
	current  string
	err      error
}

func (m sessionsDataMsg) TargetTab() dashboard.TabID { return dashboard.TabSessions }

// sessionsMutationDoneMsg signals a mutation completed.
type sessionsMutationDoneMsg struct {
	reqID int64
	err   error
}

func (m sessionsMutationDoneMsg) TargetTab() dashboard.TabID { return dashboard.TabSessions }

// sessionsPreviewMsg carries captured pane content.
type sessionsPreviewMsg struct {
	reqID   int64
	session string
	content string
}

func (m sessionsPreviewMsg) TargetTab() dashboard.TabID { return dashboard.TabSessions }

// sessionsMoveDestMsg carries the list of move-tab destinations.
type sessionsMoveDestMsg struct {
	reqID    int64
	sessions []session.SessionInfo
}

func (m sessionsMoveDestMsg) TargetTab() dashboard.TabID { return dashboard.TabSessions }

// sessionsCatalogMsg carries the result of external source discovery.
type sessionsCatalogMsg struct {
	reqID   int64
	catalog *source.Catalog
}

func (m sessionsCatalogMsg) TargetTab() dashboard.TabID { return dashboard.TabSessions }

// SessionsTab implements the Tab interface for session management.
type SessionsTab struct {
	runner tmux.Runner
	styles tui.Styles

	// Data.
	sessions []session.SessionInfo
	windows  map[string][]tmux.Window
	current  string // name of the current session
	cursor   int
	reqID    int64

	// Viewport.
	width  int
	height int

	// Interaction mode.
	mode sessionsMode

	// Rename state.
	renameInput textinput.Model

	// Confirm kill state.
	killTarget string

	// Move tab state.
	moveTargets []session.SessionInfo
	moveCursor  int

	// Preview state.
	previewSession string
	previewContent string

	// Selection persistence.
	selectedName string

	// External sources.
	catalog       *source.Catalog
	groupExpanded map[int]bool
	// When cursor moves into external area, extCursor tracks position.
	// Total cursor count = len(sessions) + externalItemCount.
}

// NewSessionsTab creates a new sessions tab.
func NewSessionsTab(runner tmux.Runner, styles tui.Styles) *SessionsTab {
	ti := textinput.New()
	ti.Placeholder = "new name..."
	ti.CharLimit = 64

	return &SessionsTab{
		runner:        runner,
		styles:        styles,
		windows:       make(map[string][]tmux.Window),
		renameInput:   ti,
		groupExpanded: make(map[int]bool),
	}
}

func (t *SessionsTab) ID() dashboard.TabID  { return dashboard.TabSessions }
func (t *SessionsTab) Title() string         { return "Sessions" }

func (t *SessionsTab) Init() tea.Cmd {
	return nil // Activate handles initial fetch.
}

func (t *SessionsTab) Activate(reason dashboard.ActivateReason) tea.Cmd {
	t.reqID = sessionsReqCounter.Add(1)
	reqID := t.reqID
	return tea.Batch(t.fetchSessions(t.reqID), t.fetchCatalog(reqID))
}

func (t *SessionsTab) Deactivate() {
	// Drop transient overlays.
	if t.mode == sessionsModeConfirmKill || t.mode == sessionsModePreview {
		t.mode = sessionsModeList
	}
	// Blur inputs.
	t.renameInput.Blur()
	if t.mode == sessionsModeRename {
		t.mode = sessionsModeList
	}
}

func (t *SessionsTab) Resize(width, height int) {
	t.width = width
	t.height = height
}

// Update processes messages for the sessions tab.
func (t *SessionsTab) Update(msg tea.Msg) (dashboard.Tab, tea.Cmd) {
	switch msg := msg.(type) {
	case sessionsDataMsg:
		if msg.reqID != t.reqID {
			return t, nil // Stale.
		}
		if msg.err != nil {
			return t, func() tea.Msg {
				return dashboard.SetStatusIntent{Text: "Failed to load sessions", IsError: true}
			}
		}
		t.sessions = msg.sessions
		t.windows = msg.windows
		t.current = msg.current
		t.restoreCursor()
		return t, nil

	case sessionsMutationDoneMsg:
		if msg.reqID != t.reqID {
			return t, nil
		}
		// Refetch after mutation.
		t.reqID = sessionsReqCounter.Add(1)
		return t, t.fetchSessions(t.reqID)

	case sessionsPreviewMsg:
		if msg.reqID != t.reqID {
			return t, nil
		}
		t.previewSession = msg.session
		t.previewContent = msg.content
		t.mode = sessionsModePreview
		return t, nil

	case sessionsCatalogMsg:
		if msg.reqID != t.reqID {
			return t, nil
		}
		t.catalog = msg.catalog
		if t.catalog != nil {
			for i := range t.catalog.External {
				if _, ok := t.groupExpanded[i]; !ok {
					t.groupExpanded[i] = true
				}
			}
		}
		return t, nil

	case sessionsMoveDestMsg:
		if msg.reqID != t.reqID {
			return t, nil
		}
		t.moveTargets = msg.sessions
		t.moveCursor = 0
		t.mode = sessionsModeMove
		return t, nil

	case tea.KeyMsg:
		return t.handleKey(msg)
	}

	// Forward to text input if renaming.
	if t.mode == sessionsModeRename {
		var cmd tea.Cmd
		t.renameInput, cmd = t.renameInput.Update(msg)
		return t, cmd
	}

	return t, nil
}

func (t *SessionsTab) handleKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	switch t.mode {
	case sessionsModeRename:
		return t.handleRenameKey(msg)
	case sessionsModeConfirmKill:
		return t.handleConfirmKillKey(msg)
	case sessionsModeMove:
		return t.handleMoveKey(msg)
	case sessionsModePreview:
		return t.handlePreviewKey(msg)
	default:
		return t.handleListKey(msg)
	}
}

func (t *SessionsTab) handleListKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	total := t.totalItems()

	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
		if t.cursor > 0 {
			t.cursor--
			if t.cursor < len(t.sessions) {
				t.selectedName = t.currentSessionName()
			}
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
		if t.cursor < total-1 {
			t.cursor++
			if t.cursor < len(t.sessions) {
				t.selectedName = t.currentSessionName()
			}
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		// External item?
		if ext := t.externalItemAt(t.cursor); ext != nil {
			if ext.isHeader {
				t.groupExpanded[ext.groupIdx] = !t.groupExpanded[ext.groupIdx]
				return t, nil
			}
			srcCopy := ext.group.Source
			if srcCopy.Kind == source.SourceOvermind && srcCopy.Overmind != nil {
				return t, func() tea.Msg {
					return dashboard.QuitIntent{Action: "overmind-connect", Chosen: ext.entry.Session + "\t" + srcCopy.Overmind.ControlSocket}
				}
			}
			epArgs := strings.Join(srcCopy.Endpoint.Args(), " ")
			return t, func() tea.Msg {
				return dashboard.QuitIntent{Action: "external-attach", Chosen: ext.entry.Session + "\t" + epArgs}
			}
		}
		// Local session.
		if t.cursor < len(t.sessions) {
			name := t.sessions[t.cursor].Name
			return t, func() tea.Msg {
				return dashboard.QuitIntent{Action: "switch", Chosen: name}
			}
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("n"))):
		return t, func() tea.Msg {
			return dashboard.QuitIntent{Action: "new", Chosen: ""}
		}

	case key.Matches(msg, key.NewBinding(key.WithKeys("r"))):
		// 'r' on overmind process = restart.
		if ext := t.externalItemAt(t.cursor); ext != nil && !ext.isHeader {
			if ext.group.Source.Kind == source.SourceOvermind && ext.group.Source.Overmind != nil {
				cs := ext.group.Source.Overmind.ControlSocket
				proc := ext.entry.Session
				return t, func() tea.Msg {
					_ = source.Restart(cs, proc)
					return dashboard.SetStatusIntent{Text: "Restarted " + proc}
				}
			}
			return t, nil
		}
		// Local rename.
		if t.cursor < len(t.sessions) {
			t.mode = sessionsModeRename
			t.renameInput.SetValue(t.sessions[t.cursor].Name)
			t.renameInput.Focus()
			return t, textinput.Blink
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("x"))):
		// 'x' on overmind process = stop.
		if ext := t.externalItemAt(t.cursor); ext != nil && !ext.isHeader {
			if ext.group.Source.Kind == source.SourceOvermind && ext.group.Source.Overmind != nil {
				cs := ext.group.Source.Overmind.ControlSocket
				proc := ext.entry.Session
				return t, func() tea.Msg {
					_ = source.Stop(cs, proc)
					return dashboard.SetStatusIntent{Text: "Stopped " + proc}
				}
			}
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("d"))):
		if t.cursor < len(t.sessions) {
			t.killTarget = t.sessions[t.cursor].Name
			t.mode = sessionsModeConfirmKill
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("m"))):
		return t, t.fetchMoveDestinations()

	case key.Matches(msg, key.NewBinding(key.WithKeys("p"))):
		if t.cursor < len(t.sessions) {
			return t, t.fetchPreview(t.sessions[t.cursor].Name)
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("c"))):
		// Cleanup tmp sessions.
		runner := t.runner
		reqID := t.reqID
		return t, func() tea.Msg {
			_, _ = session.CleanupTmp(runner)
			return sessionsMutationDoneMsg{reqID: reqID}
		}

	case key.Matches(msg, key.NewBinding(key.WithKeys("G"))):
		// Jump to bottom.
		if total > 0 {
			t.cursor = total - 1
			if t.cursor < len(t.sessions) {
				t.selectedName = t.currentSessionName()
			}
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("g"))):
		// Jump to top.
		t.cursor = 0
		if len(t.sessions) > 0 {
			t.selectedName = t.currentSessionName()
		}
		return t, nil
	}

	return t, nil
}

func (t *SessionsTab) handleRenameKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		newName := strings.TrimSpace(t.renameInput.Value())
		if newName != "" && t.cursor < len(t.sessions) {
			oldName := t.sessions[t.cursor].Name
			runner := t.runner
			reqID := t.reqID
			t.selectedName = newName // Persist cursor to new name.
			t.mode = sessionsModeList
			t.renameInput.Blur()
			return t, func() tea.Msg {
				_ = session.Rename(runner, oldName, newName)
				return sessionsMutationDoneMsg{reqID: reqID}
			}
		}
		t.mode = sessionsModeList
		t.renameInput.Blur()
		return t, nil

	case tea.KeyEscape:
		t.mode = sessionsModeList
		t.renameInput.Blur()
		return t, nil
	}

	var cmd tea.Cmd
	t.renameInput, cmd = t.renameInput.Update(msg)
	return t, cmd
}

func (t *SessionsTab) handleConfirmKillKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		target := t.killTarget
		runner := t.runner
		reqID := t.reqID
		t.mode = sessionsModeList
		t.killTarget = ""
		return t, func() tea.Msg {
			_ = session.Kill(runner, target)
			return sessionsMutationDoneMsg{reqID: reqID}
		}
	default:
		t.mode = sessionsModeList
		t.killTarget = ""
		return t, nil
	}
}

func (t *SessionsTab) handleMoveKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		t.mode = sessionsModeList
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
		if t.moveCursor > 0 {
			t.moveCursor--
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
		if t.moveCursor < len(t.moveTargets)-1 {
			t.moveCursor++
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		if t.moveCursor < len(t.moveTargets) {
			dst := t.moveTargets[t.moveCursor].Name
			src := t.current
			runner := t.runner
			t.mode = sessionsModeList
			return t, func() tea.Msg {
				_ = runner.MoveWindow(src, dst)
				_ = runner.SwitchClient(dst)
				return dashboard.QuitIntent{Action: "moved", Chosen: dst}
			}
		}
		return t, nil
	}

	return t, nil
}

func (t *SessionsTab) handlePreviewKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	// Any key dismisses the preview.
	t.mode = sessionsModeList
	t.previewContent = ""
	t.previewSession = ""
	return t, nil
}

// View renders the sessions tab content.
func (t *SessionsTab) View() string {
	switch t.mode {
	case sessionsModeMove:
		return t.viewMove()
	case sessionsModePreview:
		return t.viewPreview()
	default:
		return t.viewList()
	}
}

func (t *SessionsTab) viewList() string {
	var b strings.Builder

	// Section header.
	countStr := fmt.Sprintf("%d sessions", len(t.sessions))
	if t.current != "" {
		countStr += "  " + t.styles.Dim.Render("|") + "  " + t.styles.Success.Render(t.current)
	}
	b.WriteString("\n")
	b.WriteString(t.styles.Dim.Render(countStr) + "\n")
	b.WriteString("\n")

	// Rename overlay.
	if t.mode == sessionsModeRename {
		prompt := t.styles.Accent.Render("  rename ▸ ")
		b.WriteString(prompt + t.renameInput.View() + "\n\n")
	}

	// Confirm kill overlay.
	if t.mode == sessionsModeConfirmKill {
		prompt := t.styles.Error.Render("  Kill ") +
			t.styles.Error.Bold(true).Render(t.killTarget) +
			t.styles.Error.Render("? ") +
			t.styles.Dim.Render("(y/N)")
		b.WriteString(prompt + "\n\n")
	}

	// Empty state.
	if len(t.sessions) == 0 {
		b.WriteString(views.RenderEmptyState(
			"No sessions running.",
			"Press n to create a new session.",
			t.styles.Dim,
		))
		return b.String()
	}

	// Session list with scrolling.
	listHeight := t.height - 8 // Reserve for header, overlays, etc.
	if listHeight < 5 {
		listHeight = 12
	}

	start := 0
	if t.cursor >= listHeight/2 {
		start = t.cursor - listHeight/2
	}
	end := start + listHeight/2 // Each entry is 2 lines.
	if end > len(t.sessions) {
		end = len(t.sessions)
		start = max(0, end-listHeight/2)
	}

	// Scroll-up indicator.
	if start > 0 {
		b.WriteString(t.styles.Dim.Render("      ↑ " + fmt.Sprintf("%d more", start)) + "\n")
	}

	// Track named/tmp divider.
	shownDivider := false
	hasNamed := false
	for _, s := range t.sessions {
		if !s.IsTmp {
			hasNamed = true
			break
		}
	}

	// Pre-count named sessions before the visible window for correct indexing.
	namedCount := 0
	for j := 0; j < start; j++ {
		if !t.sessions[j].IsTmp {
			namedCount++
		}
	}

	for i := start; i < end; i++ {
		s := t.sessions[i]

		// Divider before first tmp session.
		if s.IsTmp && !shownDivider && hasNamed {
			b.WriteString(views.RenderSessionDivider(t.styles.Dim, t.width))
			shownDivider = true
		}

		// Quick-select index for named sessions.
		idx := 0
		if !s.IsTmp {
			namedCount++
			if namedCount <= 9 {
				idx = namedCount
			}
		}

		// Build windows text.
		windowsText := ""
		if wins, ok := t.windows[s.Name]; ok && len(wins) > 0 {
			names := make([]string, 0, len(wins))
			for _, w := range wins {
				names = append(names, w.Name)
			}
			windowsText = "[" + strings.Join(names, ", ") + "]"
		} else if s.Windows > 0 {
			windowsText = fmt.Sprintf("%dw", s.Windows)
		}

		row := views.SessionRow{
			Name:          s.Name,
			Age:           session.HumanAge(s.Created),
			StatusText:    statusText(s),
			WindowsText:   windowsText,
			DirectoryText: shortenPath(s.Dir),
			IsCurrent:     s.Name == t.current,
			IsAttached:    s.Attached,
			IsTmp:         s.IsTmp,
			IsSelected:    i == t.cursor,
			Index:         idx,
		}

		rowStyles := views.SessionRowStyles{
			Normal:  t.styles.Normal,
			Accent:  t.styles.Accent,
			Dim:     t.styles.Dim,
			Info:    t.styles.Info,
			Success: t.styles.Success,
		}
		b.WriteString(views.RenderSessionRow(row, rowStyles, t.width))
	}

	// Scroll-down indicator.
	if end < len(t.sessions) {
		b.WriteString(t.styles.Dim.Render("      \u2193 " + fmt.Sprintf("%d more", len(t.sessions)-end)) + "\n")
	}

	// External source groups.
	if t.catalog != nil && len(t.catalog.External) > 0 {
		b.WriteString(t.viewExternalSources())
	}

	return b.String()
}

func (t *SessionsTab) viewExternalSources() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(t.styles.Dim.Render(strings.Repeat("\u2501", 40)) + "\n")

	pos := len(t.sessions)
	for i, g := range t.catalog.External {
		selected := t.cursor == pos

		arrow := "\u25bc"
		if !t.groupExpanded[i] {
			arrow = "\u25b6"
		}

		kindLabel := string(g.Source.Kind)
		label := g.Source.Label
		countStr := fmt.Sprintf("(%d procs)", len(g.Entries))

		headerStyle := t.styles.Dim
		if selected {
			headerStyle = t.styles.Accent
		}

		cursor := "  "
		if selected {
			cursor = t.styles.Accent.Render("\u25b8 ")
		}

		healthBadge := ""
		if g.Source.Health == source.HealthDegraded {
			healthBadge = " " + t.styles.Error.Render("[degraded]")
		}

		b.WriteString("  " + cursor + headerStyle.Render(arrow+" "+kindLabel+": "+label) +
			"  " + t.styles.Dim.Render(countStr) + healthBadge + "\n")
		pos++

		if t.groupExpanded[i] {
			for _, entry := range g.Entries {
				entrySelected := t.cursor == pos

				icon := "\u25cb"
				iconStyle := t.styles.Dim
				if entry.Attached {
					icon = "\u25cf"
					iconStyle = t.styles.Info
				}
				if entrySelected {
					iconStyle = t.styles.Accent
				}

				nameStyle := t.styles.Normal
				if entrySelected {
					nameStyle = t.styles.Accent.Bold(true)
				}

				entryCursor := "    "
				if entrySelected {
					entryCursor = "  " + t.styles.Accent.Render("\u25b8 ")
				}

				statusLabel := t.styles.Dim.Render("stopped")
				if entry.Attached {
					statusLabel = t.styles.Info.Render("running")
				}

				b.WriteString("  " + entryCursor + iconStyle.Render(icon) + " " +
					nameStyle.Render(fmt.Sprintf("%-16s", entry.Session)) + " " +
					statusLabel + "\n")
				pos++
			}
		}
	}

	return b.String()
}

func (t *SessionsTab) viewMove() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(t.styles.Accent.Bold(true).Render("Move Window") + "\n")
	b.WriteString(t.styles.Dim.Render("Move current window to another session") + "\n\n")

	if len(t.moveTargets) == 0 {
		b.WriteString(t.styles.Dim.Render("  No other sessions available.") + "\n")
	} else {
		for i, s := range t.moveTargets {
			cursor := "  "
			if i == t.moveCursor {
				cursor = t.styles.Accent.Render("▸ ")
			}
			nameStyle := t.styles.Normal
			if i == t.moveCursor {
				nameStyle = t.styles.Accent.Bold(true)
			}
			meta := t.styles.Dim.Render(fmt.Sprintf("  %dw", s.Windows))
			b.WriteString("  " + cursor + nameStyle.Render(s.Name) + meta + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(t.styles.Dim.Render("  enter:move  esc:cancel") + "\n")

	return b.String()
}

func (t *SessionsTab) viewPreview() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(t.styles.Accent.Bold(true).Render("Preview") + "  ")
	b.WriteString(t.styles.Normal.Bold(true).Render(t.previewSession) + "\n")

	lineWidth := t.width - 4
	if lineWidth < 20 {
		lineWidth = 20
	}
	if lineWidth > 72 {
		lineWidth = 72
	}
	b.WriteString(t.styles.Dim.Render(strings.Repeat("─", lineWidth)) + "\n")

	if t.previewContent == "" {
		b.WriteString(t.styles.Dim.Render("  (empty)") + "\n")
	} else {
		lines := strings.Split(t.previewContent, "\n")
		maxLines := 20
		if len(lines) > maxLines {
			lines = lines[len(lines)-maxLines:]
		}
		for _, line := range lines {
			b.WriteString("  " + t.styles.Normal.Render(line) + "\n")
		}
	}

	b.WriteString(t.styles.Dim.Render(strings.Repeat("─", lineWidth)) + "\n")
	b.WriteString(t.styles.Dim.Render("  press any key to dismiss") + "\n")

	return b.String()
}

func (t *SessionsTab) ShortHelp() string {
	switch t.mode {
	case sessionsModeRename:
		return "enter:confirm  esc:cancel"
	case sessionsModeConfirmKill:
		return "y:confirm  any:cancel"
	case sessionsModeMove:
		return "enter:move  esc:cancel"
	case sessionsModePreview:
		return "any key:dismiss"
	default:
		// Context-sensitive help for external items.
		if ext := t.externalItemAt(t.cursor); ext != nil {
			if ext.isHeader {
				return "enter:toggle  n:new"
			}
			if ext.group.Source.Kind == source.SourceOvermind {
				return "enter:connect  r:restart  x:stop  n:new"
			}
			return "enter:attach  n:new"
		}
		parts := []string{
			"enter:switch",
			"n:new",
			"r:rename",
			"d:kill",
			"m:move",
			"p:preview",
			"c:cleanup",
		}
		return strings.Join(parts, "  ")
	}
}

// ── External source helpers ──

// sessionsExternalItem describes a single item in the external source area.
type sessionsExternalItem struct {
	isHeader bool
	groupIdx int
	group    *source.SourceGroup
	entry    source.CatalogEntry
}

func (t *SessionsTab) externalItemCount() int {
	if t.catalog == nil || len(t.catalog.External) == 0 {
		return 0
	}
	count := 0
	for i, g := range t.catalog.External {
		count++ // header
		if t.groupExpanded[i] {
			count += len(g.Entries)
		}
	}
	return count
}

func (t *SessionsTab) totalItems() int {
	return len(t.sessions) + t.externalItemCount()
}

func (t *SessionsTab) externalItemAt(cursor int) *sessionsExternalItem {
	base := len(t.sessions)
	if cursor < base || t.catalog == nil {
		return nil
	}
	pos := base
	for i := range t.catalog.External {
		g := &t.catalog.External[i]
		if cursor == pos {
			return &sessionsExternalItem{isHeader: true, groupIdx: i, group: g}
		}
		pos++
		if t.groupExpanded[i] {
			for j := range g.Entries {
				if cursor == pos {
					return &sessionsExternalItem{groupIdx: i, group: g, entry: g.Entries[j]}
				}
				pos++
			}
		}
	}
	return nil
}

func (t *SessionsTab) isOnExternalItem() bool {
	return t.cursor >= len(t.sessions) && t.externalItemAt(t.cursor) != nil
}

func (t *SessionsTab) fetchCatalog(reqID int64) tea.Cmd {
	return func() tea.Msg {
		cat, _ := source.Discover()
		return sessionsCatalogMsg{reqID: reqID, catalog: cat}
	}
}

// ── Data fetching ──

func (t *SessionsTab) fetchSessions(reqID int64) tea.Cmd {
	runner := t.runner
	return func() tea.Msg {
		sessions, err := session.ListSessions(runner)
		if err != nil {
			return sessionsDataMsg{reqID: reqID, err: err}
		}

		// Fetch windows for all sessions.
		windows := make(map[string][]tmux.Window)
		for _, s := range sessions {
			w, werr := runner.ListWindows(s.Name)
			if werr == nil {
				windows[s.Name] = w
			}
		}

		// Get current session name.
		current, _ := runner.DisplayMessage("", "#{session_name}")

		return sessionsDataMsg{
			reqID:    reqID,
			sessions: sessions,
			windows:  windows,
			current:  strings.TrimSpace(current),
		}
	}
}

func (t *SessionsTab) fetchPreview(name string) tea.Cmd {
	runner := t.runner
	reqID := t.reqID
	return func() tea.Msg {
		content, _ := runner.CapturePane(name, 30)
		return sessionsPreviewMsg{
			reqID:   reqID,
			session: name,
			content: content,
		}
	}
}

func (t *SessionsTab) fetchMoveDestinations() tea.Cmd {
	runner := t.runner
	current := t.current
	reqID := t.reqID
	return func() tea.Msg {
		sessions, _ := session.ListSessions(runner)
		var filtered []session.SessionInfo
		for _, s := range sessions {
			if s.Name != current {
				filtered = append(filtered, s)
			}
		}
		return sessionsMoveDestMsg{
			reqID:    reqID,
			sessions: filtered,
		}
	}
}

// ── Helpers ──

func (t *SessionsTab) currentSessionName() string {
	if t.cursor >= 0 && t.cursor < len(t.sessions) {
		return t.sessions[t.cursor].Name
	}
	return ""
}

func (t *SessionsTab) restoreCursor() {
	if t.selectedName == "" {
		t.clampCursor()
		return
	}

	// Find the session by name.
	for i, s := range t.sessions {
		if s.Name == t.selectedName {
			t.cursor = i
			return
		}
	}

	// Name not found -- clamp to nearest.
	t.clampCursor()
}

func (t *SessionsTab) clampCursor() {
	if t.cursor >= len(t.sessions) {
		t.cursor = max(0, len(t.sessions)-1)
	}
}

func statusText(s session.SessionInfo) string {
	if s.Attached {
		return "attached"
	}
	return ""
}

// shortenPath replaces the home directory with ~ and truncates long paths.
func shortenPath(path string) string {
	if path == "" {
		return ""
	}
	home, err := os.UserHomeDir()
	if err == nil && strings.HasPrefix(path, home) {
		path = "~" + path[len(home):]
	}
	parts := strings.Split(path, string(filepath.Separator))
	if len(parts) > 3 {
		path = filepath.Join("...", parts[len(parts)-2], parts[len(parts)-1])
	}
	return path
}
