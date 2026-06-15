// Package tabs implements the individual dashboard tab models.
package tabs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui/dashboard"
	"github.com/donjor/zmux/internal/tui/outline"
	"github.com/donjor/zmux/internal/tui/styles"
	"github.com/donjor/zmux/internal/tui/workspaceview"
	"github.com/donjor/zmux/internal/workspace"
)

// currentMode tracks the current interaction mode.
type currentMode int

const (
	currentModeList                currentMode = iota // browsing the tree
	currentModeRename                                 // inline rename (ws / session / window)
	currentModeCreate                                 // inline "new session in workspace" input
	currentModeConfirmKill                            // y/N confirmation
	currentModeConfirmKillAttached                    // second-step confirm for killing attached ws
	currentModeMoveWindow                             // session picker for moving a window
	currentModeSearch                                 // inline `/` search input over the session list
)

// currentNavLevel tracks the two-level cursor model. In sessionLevel,
// the cursor hops session-to-session (window rows are rendered for
// context but aren't selectable). Press l/right to descend into
// windowLevel where the cursor navigates the expanded session's
// windows; press h/left to return.
type currentNavLevel int

const (
	navLevelSession currentNavLevel = iota
	navLevelWindow
)

// windowDetail combines a window with its pane details and process stats.
type windowDetail struct {
	tmux.Window
	Panes  []tmux.Pane
	Stats  tmux.ProcessStats
	Uptime string
}

// confirmState / renameState / moveState live in mode_state.go and are
// shared with the Workspaces tab.

// currentMoveTarget is a simplified session entry for the move picker.
type currentMoveTarget struct {
	Name    string
	Windows int
}

// ── Messages ──

// currentDataMsg carries fetched session + workspace data for the tab.
type currentDataMsg struct {
	reqID          int64
	wsName         string
	sessionName    string
	sessionDir     string
	attached       int
	windows        []windowDetail           // current session: full detail with CPU/mem
	siblings       []session.SessionInfo    // other sessions in the workspace
	siblingWindows map[string][]tmux.Window // sibling session name → basic windows
	wsModel        *workspaceview.WorkspaceViewModel
	err            error
}

func (m currentDataMsg) TargetTab() dashboard.TabID { return dashboard.TabSession }

// currentMutationDoneMsg signals that a mutation completed — the tab refetches.
type currentMutationDoneMsg struct {
	reqID int64
}

func (m currentMutationDoneMsg) TargetTab() dashboard.TabID { return dashboard.TabSession }

// currentMoveDestMsg carries the list of move destinations.
type currentMoveDestMsg struct {
	reqID    int64
	sessions []currentMoveTarget
}

func (m currentMoveDestMsg) TargetTab() dashboard.TabID { return dashboard.TabSession }

// CurrentTab implements the Tab interface for the unified
// "Session & Workspace" view.
type CurrentTab struct {
	runner   tmux.Runner
	styles   styles.Styles
	wsLoader workspaceview.WorkspaceDataLoader
	wsStore  *workspace.Store

	// Tree owns row data + cursor.
	tree *outline.Tree

	// Snapshot data.
	wsName         string
	sessionName    string
	sessionDir     string
	attached       int
	windows        []windowDetail
	siblings       []session.SessionInfo
	siblingWindows map[string][]tmux.Window
	wsModel        *workspaceview.WorkspaceViewModel

	// Viewport — handles scrolling automatically. Content is set on
	// each render; the viewport clips to height and manages YOffset.
	vp     viewport.Model
	width  int
	height int

	// Interaction mode.
	mode currentMode

	// Two-level cursor navigation.
	navLevel          currentNavLevel // session vs window cursor scope
	expandedSessionID string          // outline ID of the session whose windows are navigable

	// Inputs / overlay state.
	renameInput textinput.Model
	createInput textinput.Model
	searchInput textinput.Model
	rename      *renameState
	confirm     *confirmState
	moveSt      *moveState

	// searchQuery is the active filter over the session list, scoped to the
	// current workspace. Independent of currentModeSearch: editing the query
	// lives in that mode, but a committed query keeps filtering while the user
	// browses in list mode (mirrors the Workspaces tab).
	searchQuery string

	// Move-window overlay state.
	moveTargets []currentMoveTarget
	moveCursor  int

	// Async-correctness primitives.
	reqID         int64
	pending       *currentDataMsg // staged refetch arrived during a modal
	pendingJumpTo string          // one-shot row ID to land on after next apply
}

// NewCurrentTab creates a new "Session & Workspace" tab.
// wsLoader returns enriched workspace view models; wsStore is required for
// workspace CRUD mutations from inside the tab.
func NewCurrentTab(runner tmux.Runner, styles styles.Styles, wsLoader workspaceview.WorkspaceDataLoader, wsStore *workspace.Store) *CurrentTab {
	ri := textinput.New()
	ri.Placeholder = "new name..."
	ri.CharLimit = 64

	ci := textinput.New()
	ci.Placeholder = "session name..."
	ci.CharLimit = 64

	si := textinput.New()
	si.Placeholder = "search sessions & tabs..."
	si.CharLimit = 64

	return &CurrentTab{
		runner:      runner,
		styles:      styles,
		wsLoader:    wsLoader,
		wsStore:     wsStore,
		tree:        outline.NewTree(),
		renameInput: ri,
		createInput: ci,
		searchInput: si,
	}
}

// ── Tab interface ──

func (t *CurrentTab) ID() dashboard.TabID { return dashboard.TabSession }
func (t *CurrentTab) Title() string       { return "Session & Workspace" }
func (t *CurrentTab) Init() tea.Cmd       { return nil }

// CapturesEscape reports that Esc should be handled by the tab rather than
// close the dashboard: while in a capturing mode (rename/create/move/confirm/
// search), or while a committed search filter is active in list mode (Esc
// clears it).
func (t *CurrentTab) CapturesEscape() bool {
	return t.mode != currentModeList || t.searchQuery != ""
}

func (t *CurrentTab) Activate(reason dashboard.ActivateReason) tea.Cmd {
	// Fresh activation always starts at session level so session-hopping
	// is one keystroke away. Callers that re-enter window level do so
	// via the l/tab keybinding.
	t.navLevel = navLevelSession
	t.expandedSessionID = ""
	t.reqID = dashboard.NextReqID()
	return t.fetchData(t.reqID)
}

func (t *CurrentTab) Deactivate() {
	// Bump reqID so any in-flight responses become stale.
	t.reqID = dashboard.NextReqID()
	t.exitMode()
	t.pending = nil
	t.pendingJumpTo = ""
	t.renameInput.Blur()
	t.createInput.Blur()
	// Drop any active search filter so the tab is fresh on re-entry.
	t.searchQuery = ""
	t.searchInput.SetValue("")
	t.searchInput.Blur()
}

func (t *CurrentTab) Resize(width, height int) {
	t.width = width
	t.height = height
	t.vp.SetWidth(width)
	t.vp.SetHeight(height)
}

// Update processes messages for the session tab.
func (t *CurrentTab) Update(msg tea.Msg) (dashboard.Tab, tea.Cmd) {
	switch msg := msg.(type) {
	case dashboard.ThemeChangedMsg:
		t.styles = msg.Styles
		return t, nil

	case currentDataMsg:
		t.onDataMsg(msg)
		return t, nil

	case currentMutationDoneMsg:
		if msg.reqID != t.reqID {
			return t, nil
		}
		t.reqID = dashboard.NextReqID()
		return t, t.fetchData(t.reqID)

	case currentMoveDestMsg:
		if msg.reqID != t.reqID {
			return t, nil
		}
		t.moveTargets = msg.sessions
		t.moveCursor = 0
		t.mode = currentModeMoveWindow
		return t, nil

	case tea.KeyMsg:
		return t.handleKey(msg)
	}

	// Forward stray messages to the active text input.
	switch t.mode {
	case currentModeRename:
		var cmd tea.Cmd
		t.renameInput, cmd = t.renameInput.Update(msg)
		return t, cmd
	case currentModeCreate:
		var cmd tea.Cmd
		t.createInput, cmd = t.createInput.Update(msg)
		return t, cmd
	case currentModeSearch:
		var cmd tea.Cmd
		t.searchInput, cmd = t.searchInput.Update(msg)
		return t, cmd
	}
	return t, nil
}

// onDataMsg handles an incoming data message: dropped if stale, staged if a
// modal is open, or applied immediately otherwise.
func (t *CurrentTab) onDataMsg(msg currentDataMsg) {
	if msg.reqID != t.reqID {
		return
	}
	if msg.err != nil {
		// Surface via the next status flash; don't crash the tab.
		return
	}
	if t.mode != currentModeList {
		t.pending = &msg
		return
	}
	t.applyData(msg)
}

// applyData replaces the snapshot data and rebuilds the tree.
func (t *CurrentTab) applyData(msg currentDataMsg) {
	t.wsName = msg.wsName
	t.sessionName = msg.sessionName
	t.sessionDir = msg.sessionDir
	t.attached = msg.attached
	t.windows = msg.windows
	t.siblings = msg.siblings
	t.siblingWindows = msg.siblingWindows
	t.wsModel = msg.wsModel
	rows := t.buildRows()
	if t.pendingJumpTo != "" {
		// A committed filter may hide the row we just acted on (the new/renamed
		// session): drop the filter and rebuild so the jump lands on the real
		// target instead of SetRowsAndJumpTo's silent fallback row.
		if t.searchQuery != "" && !rowsContain(rows, t.pendingJumpTo) {
			t.searchQuery = ""
			t.searchInput.SetValue("")
			rows = t.buildRows()
		}
		t.tree.SetRowsAndJumpTo(rows, t.pendingJumpTo)
		t.pendingJumpTo = ""
	} else {
		t.tree.SetRows(rows)
	}
}

// exitMode resets to list mode and applies any staged data refetch.
func (t *CurrentTab) exitMode() {
	t.mode = currentModeList
	t.rename = nil
	t.confirm = nil
	t.moveSt = nil
	t.renameInput.Blur()
	t.createInput.Blur()
	t.searchInput.Blur()
	if t.pending != nil {
		t.applyData(*t.pending)
		t.pending = nil
	}
}

// ── ShortHelp ──

func (t *CurrentTab) ShortHelp() string {
	switch t.mode {
	case currentModeRename:
		return "enter:confirm  esc:cancel"
	case currentModeCreate:
		return "enter:create  esc:cancel"
	case currentModeConfirmKill, currentModeConfirmKillAttached:
		return "y:confirm  any:cancel"
	case currentModeMoveWindow:
		return "enter:move  esc:cancel"
	}

	if t.mode == currentModeSearch {
		return "type:filter  ↑↓:move  enter:apply  esc:cancel"
	}

	if t.sessionName == "" {
		return "c:new tmp  tab:workspaces  esc:exit"
	}

	// List mode: the trailing hint advertises search + digit quick-jump, plus
	// esc:clear when a committed filter is active. "switch" (not "jump") signals
	// that a digit activates the session — focuses the current one, switches to
	// a sibling — rather than only moving the cursor.
	tail := "  /:search  1-9:switch"
	if t.searchQuery != "" {
		tail += "  esc:clear"
	}

	row := t.tree.Current()
	if row == nil {
		return "r:rename  x:kill  c:new" + tail
	}

	switch row.Kind {
	case outline.RowWorkspaceHeader:
		return strings.Join([]string{"r:rename", "x:kill", "c:new session"}, "  ") + tail
	case outline.RowSession:
		// Session-level cursor: show session ops + the "l:tabs" hint to
		// descend into window navigation. "c" creates a new tab in the
		// session (mirrors the behavior on window rows).
		return strings.Join([]string{"enter:switch", "l:tabs", "r:rename", "x:kill", "c:new tab"}, "  ") + tail
	case outline.RowPane:
		return strings.Join([]string{"enter:focus", "h:back", "x:kill pane"}, "  ") + tail
	case outline.RowWindow:
		// Window-level cursor: differentiate current-session windows
		// (full action set) from sibling-session windows (move + reorder
		// aren't wired for those yet).
		if _, ok := outline.RowData[windowDetail](row); ok {
			return strings.Join([]string{"enter:focus", "h:back", "r:rename", "x:kill", "m:move", "</>:reorder", "c:new"}, "  ") + tail
		}
		return strings.Join([]string{"enter:switch", "h:back", "r:rename", "x:kill", "c:new"}, "  ") + tail
	}
	return "r:rename  x:kill  c:new" + tail
}

// ── View ──

// View renders the session tab content. All rows are rendered into a
// full content string; the viewport handles clipping and scrolling.
func (t *CurrentTab) View() string {
	if t.mode == currentModeMoveWindow {
		return t.viewMove()
	}

	if t.sessionName == "" {
		var b strings.Builder
		b.WriteString("  " + t.styles.Dim.Render("No active session.") + "\n")
		b.WriteString("  " + t.styles.Dim.Render("Press c to create a temporary session, or Tab for Workspaces.") + "\n")
		return b.String()
	}

	// Fixed chrome pinned above the scrollable rows: the workspace scope cue
	// (always visible even when the row list scrolls) plus any active overlay.
	chrome := t.renderChrome()

	// Reserve the chrome's height so the viewport's rows take the remainder —
	// the cue never scrolls off and the row area is sized to what's left.
	if t.height > 0 {
		vpHeight := t.height - strings.Count(chrome, "\n")
		if vpHeight < 1 {
			vpHeight = 1
		}
		t.vp.SetHeight(vpHeight)
	}

	var b strings.Builder
	rows := t.tree.Rows
	cursorLine := 0
	lineCount := 0
	for i := range rows {
		if i == t.tree.Cursor {
			cursorLine = lineCount
		}
		rendered := t.renderRow(&rows[i], i == t.tree.Cursor)
		b.WriteString(rendered)
		lineCount += strings.Count(rendered, "\n")
	}
	if len(rows) == 0 {
		b.WriteString(t.styles.Dim.Render("  (no rows)") + "\n")
	}

	// Set content on viewport and scroll to keep cursor visible.
	t.vp.SetContent(b.String())
	ensureCursorVisible(&t.vp, cursorLine)
	return chrome + renderScrollable(t.vp, t.styles)
}

// renderChrome builds the fixed block pinned above the scrollable rows: a
// blank spacer, the workspace scope cue, and whichever overlay (rename /
// create / confirm / search) is active — or a blank spacer when none. Kept
// out of the viewport content so the scope cue stays visible while rows
// scroll under it.
func (t *CurrentTab) renderChrome() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + t.renderScopeCue() + "\n")

	switch t.mode {
	case currentModeRename:
		b.WriteString(t.renderRenameOverlay())
	case currentModeCreate:
		b.WriteString(t.renderCreateOverlay())
	case currentModeConfirmKill:
		b.WriteString(t.renderConfirmOverlay(1))
	case currentModeConfirmKillAttached:
		b.WriteString(t.renderConfirmOverlay(2))
	case currentModeSearch:
		b.WriteString(t.renderSearchOverlay())
	default:
		b.WriteString("\n")
	}
	return b.String()
}

// renderScopeCue renders the "N sessions in <ws>" scope line, the only signal
// that anchors these rows as scoped to one workspace. A committed filter adds
// a "filter: <q>" chip (suppressed while the search input is open, since the
// query is visible there).
func (t *CurrentTab) renderScopeCue() string {
	sessionCount := 1 + len(t.siblings)
	cue := t.styles.Dim.Render(fmt.Sprintf("%d %s in %s", sessionCount, pluralize("session", sessionCount), t.wsName))
	if t.attached > 0 {
		cue += t.styles.Dim.Render(" · attached")
	}
	if t.mode != currentModeSearch && t.searchQuery != "" {
		cue += "  " + t.styles.Dim.Render("|") + "  " + t.styles.Info.Render("filter: "+t.searchQuery)
	}
	return cue
}

// viewMove renders the window-move destination picker (full-screen overlay).
func (t *CurrentTab) viewMove() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + t.styles.Accent.Bold(true).Render("Move Window") + "\n")
	b.WriteString("  " + t.styles.Dim.Render("Move window to another session") + "\n\n")

	if len(t.moveTargets) == 0 {
		b.WriteString("  " + t.styles.Dim.Render("No other sessions available.") + "\n")
	} else {
		for i, s := range t.moveTargets {
			cursor := "  "
			nameStyle := t.styles.Normal
			if i == t.moveCursor {
				cursor = t.styles.Accent.Render("▸ ")
				nameStyle = t.styles.Accent.Bold(true)
			}
			meta := t.styles.Dim.Render(fmt.Sprintf("  %dw", s.Windows))
			b.WriteString("  " + cursor + nameStyle.Render(s.Name) + meta + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString("  " + t.styles.Dim.Render("enter:move  esc:cancel") + "\n")
	return b.String()
}

// ── Helpers ──

// shortenDir replaces the home directory with ~ and truncates long paths.
func shortenDir(path string) string {
	if path == "" {
		return ""
	}
	home, err := os.UserHomeDir()
	if err == nil && strings.HasPrefix(path, home) {
		path = "~" + path[len(home):]
	}
	parts := strings.Split(path, string(filepath.Separator))
	if len(parts) > 4 {
		path = filepath.Join("...", parts[len(parts)-2], parts[len(parts)-1])
	}
	return path
}

// isIdleWindow returns true if the window's command is a shell with no CPU usage.
func isIdleWindow(w windowDetail) bool {
	if w.Stats.CPU > 0.5 {
		return false
	}

	cmd := ""
	for _, p := range w.Panes {
		if p.Active {
			cmd = p.Command
			break
		}
	}
	if cmd == "" && len(w.Panes) > 0 {
		cmd = w.Panes[0].Command
	}

	switch cmd {
	case "bash", "zsh", "fish", "sh", "dash", "ksh":
		return true
	}
	return false
}
