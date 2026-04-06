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

	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/tui"
	"github.com/donjor/zmux/internal/tui/dashboard"
	"github.com/donjor/zmux/internal/tui/views"
)

// currentReqCounter is a monotonic counter for async correctness.
var currentReqCounter atomic.Int64

// currentMode tracks the current interaction mode.
type currentMode int

const (
	currentModeList         currentMode = iota // browsing window list
	currentModeRename                          // inline rename input
	currentModeConfirmClose                    // y/N confirmation
	currentModeMoveWindow                      // session picker for moving
)

// windowDetail combines a window with its pane details and process stats.
type windowDetail struct {
	tmux.Window
	Panes  []tmux.Pane
	Stats  tmux.ProcessStats
	Uptime string
}

// ── Messages ──

// currentDataMsg carries fetched session data for the current session tab.
type currentDataMsg struct {
	reqID       int64
	sessionName string
	sessionDir  string
	attached    int
	windows     []windowDetail
	err         error
}

func (m currentDataMsg) TargetTab() dashboard.TabID { return dashboard.TabCurrent }

// currentMutationDoneMsg signals a mutation completed.
type currentMutationDoneMsg struct {
	reqID int64
	err   error
}

func (m currentMutationDoneMsg) TargetTab() dashboard.TabID { return dashboard.TabCurrent }

// currentMoveDestMsg carries the list of move destinations.
type currentMoveDestMsg struct {
	reqID    int64
	sessions []moveTarget
}

func (m currentMoveDestMsg) TargetTab() dashboard.TabID { return dashboard.TabCurrent }

// moveTarget is a simplified session entry for the move picker.
type moveTarget struct {
	Name    string
	Windows int
}

// CurrentTab implements the Tab interface for the "This Session" view.
type CurrentTab struct {
	runner tmux.Runner
	styles tui.Styles

	// Data.
	sessionName string
	sessionDir  string
	attached    int
	windows     []windowDetail
	cursor      int
	reqID       int64

	// Viewport.
	width  int
	height int

	// Interaction mode.
	mode currentMode

	// Rename state.
	renameInput textinput.Model

	// Close confirmation.
	closeTarget string
	closeIdx    int

	// Move window state.
	moveTargets []moveTarget
	moveCursor  int
	moveWindow  int // index of window being moved

	// Selection persistence.
	selectedIdx int
}

// NewCurrentTab creates a new "This Session" tab.
func NewCurrentTab(runner tmux.Runner, styles tui.Styles) *CurrentTab {
	ti := textinput.New()
	ti.Placeholder = "new name..."
	ti.CharLimit = 64

	return &CurrentTab{
		runner:      runner,
		styles:      styles,
		renameInput: ti,
		selectedIdx: -1,
	}
}

func (t *CurrentTab) ID() dashboard.TabID { return dashboard.TabCurrent }
func (t *CurrentTab) Title() string        { return "This Session" }
func (t *CurrentTab) Init() tea.Cmd        { return nil }

func (t *CurrentTab) Activate(reason dashboard.ActivateReason) tea.Cmd {
	t.reqID = currentReqCounter.Add(1)
	return t.fetchData(t.reqID)
}

func (t *CurrentTab) Deactivate() {
	if t.mode == currentModeConfirmClose || t.mode == currentModeMoveWindow {
		t.mode = currentModeList
	}
	if t.mode == currentModeRename {
		t.renameInput.Blur()
		t.mode = currentModeList
	}
}

func (t *CurrentTab) Resize(width, height int) {
	t.width = width
	t.height = height
}

// Update processes messages for the current session tab.
func (t *CurrentTab) Update(msg tea.Msg) (dashboard.Tab, tea.Cmd) {
	switch msg := msg.(type) {
	case dashboard.ThemeChangedMsg:
		t.styles = msg.Styles
		return t, nil
	case currentDataMsg:
		if msg.reqID != t.reqID {
			return t, nil
		}
		if msg.err != nil {
			return t, func() tea.Msg {
				return dashboard.SetStatusIntent{Text: "Failed to load session", IsError: true}
			}
		}
		t.sessionName = msg.sessionName
		t.sessionDir = msg.sessionDir
		t.attached = msg.attached
		t.windows = msg.windows
		t.restoreCursor()
		return t, nil

	case currentMutationDoneMsg:
		if msg.reqID != t.reqID {
			return t, nil
		}
		// Refetch after mutation.
		t.reqID = currentReqCounter.Add(1)
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

	// Forward to text input if renaming.
	if t.mode == currentModeRename {
		var cmd tea.Cmd
		t.renameInput, cmd = t.renameInput.Update(msg)
		return t, cmd
	}

	return t, nil
}

func (t *CurrentTab) handleKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	switch t.mode {
	case currentModeRename:
		return t.handleRenameKey(msg)
	case currentModeConfirmClose:
		return t.handleConfirmCloseKey(msg)
	case currentModeMoveWindow:
		return t.handleMoveKey(msg)
	default:
		return t.handleListKey(msg)
	}
}

func (t *CurrentTab) handleListKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
		if t.cursor > 0 {
			t.cursor--
			t.selectedIdx = t.currentWindowIndex()
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
		if t.cursor < len(t.windows)-1 {
			t.cursor++
			t.selectedIdx = t.currentWindowIndex()
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
		if t.cursor < len(t.windows) {
			w := t.windows[t.cursor]
			session := t.sessionName
			runner := t.runner
			return t, func() tea.Msg {
				_ = runner.SelectWindow(session, w.Index)
				return dashboard.QuitIntent{Action: "focus", Chosen: session}
			}
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("r"))):
		if t.cursor < len(t.windows) {
			t.mode = currentModeRename
			t.renameInput.SetValue(t.windows[t.cursor].Name)
			t.renameInput.Focus()
			return t, textinput.Blink
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("x"))):
		if t.cursor < len(t.windows) {
			t.closeTarget = t.windows[t.cursor].Name
			t.closeIdx = t.windows[t.cursor].Index
			t.mode = currentModeConfirmClose
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("m"))):
		if t.cursor < len(t.windows) {
			t.moveWindow = t.windows[t.cursor].Index
			return t, t.fetchMoveDestinations()
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("<"))):
		return t.swapWindow(-1)

	case key.Matches(msg, key.NewBinding(key.WithKeys(">"))):
		return t.swapWindow(1)

	case key.Matches(msg, key.NewBinding(key.WithKeys("n"))):
		session := t.sessionName
		dir := t.sessionDir
		runner := t.runner
		reqID := t.reqID
		return t, func() tea.Msg {
			_ = runner.NewWindow(session, "", dir)
			return currentMutationDoneMsg{reqID: reqID}
		}

	case key.Matches(msg, key.NewBinding(key.WithKeys("G"))):
		if len(t.windows) > 0 {
			t.cursor = len(t.windows) - 1
			t.selectedIdx = t.currentWindowIndex()
		}
		return t, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("g"))):
		t.cursor = 0
		if len(t.windows) > 0 {
			t.selectedIdx = t.currentWindowIndex()
		}
		return t, nil
	}

	return t, nil
}

func (t *CurrentTab) handleRenameKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		newName := strings.TrimSpace(t.renameInput.Value())
		if newName != "" && t.cursor < len(t.windows) {
			oldName := t.windows[t.cursor].Name
			session := t.sessionName
			runner := t.runner
			reqID := t.reqID
			t.mode = currentModeList
			t.renameInput.Blur()
			return t, func() tea.Msg {
				_ = runner.RenameWindow(session, oldName, newName)
				return currentMutationDoneMsg{reqID: reqID}
			}
		}
		t.mode = currentModeList
		t.renameInput.Blur()
		return t, nil

	case tea.KeyEscape:
		t.mode = currentModeList
		t.renameInput.Blur()
		return t, nil
	}

	var cmd tea.Cmd
	t.renameInput, cmd = t.renameInput.Update(msg)
	return t, cmd
}

func (t *CurrentTab) handleConfirmCloseKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		idx := t.closeIdx
		session := t.sessionName
		runner := t.runner
		reqID := t.reqID
		t.mode = currentModeList
		t.closeTarget = ""
		return t, func() tea.Msg {
			_ = runner.KillWindow(session, idx)
			return currentMutationDoneMsg{reqID: reqID}
		}
	default:
		t.mode = currentModeList
		t.closeTarget = ""
		return t, nil
	}
}

func (t *CurrentTab) handleMoveKey(msg tea.KeyMsg) (dashboard.Tab, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
		t.mode = currentModeList
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
			src := fmt.Sprintf("%s:%d", t.sessionName, t.moveWindow)
			runner := t.runner
			reqID := t.reqID
			t.mode = currentModeList
			return t, func() tea.Msg {
				_ = runner.MoveWindow(src, dst)
				return currentMutationDoneMsg{reqID: reqID}
			}
		}
		return t, nil
	}

	return t, nil
}

func (t *CurrentTab) swapWindow(delta int) (dashboard.Tab, tea.Cmd) {
	if t.cursor < 0 || t.cursor >= len(t.windows) {
		return t, nil
	}

	neighborIdx := t.cursor + delta
	if neighborIdx < 0 || neighborIdx >= len(t.windows) {
		return t, nil
	}

	idx1 := t.windows[t.cursor].Index
	idx2 := t.windows[neighborIdx].Index
	session := t.sessionName
	runner := t.runner

	// Optimistic local swap — update UI instantly.
	t.windows[t.cursor], t.windows[neighborIdx] = t.windows[neighborIdx], t.windows[t.cursor]
	// Also swap the tmux indices to keep them consistent.
	t.windows[t.cursor].Index = idx1
	t.windows[neighborIdx].Index = idx2
	t.cursor = neighborIdx

	// Fire-and-forget: send the swap to tmux in the background.
	// No refetch needed — local state is already correct.
	return t, func() tea.Msg {
		_ = runner.SwapWindow(session, idx1, idx2)
		return nil // no mutation msg, no refetch
	}
}

// View renders the current session tab content.
func (t *CurrentTab) View() string {
	switch t.mode {
	case currentModeMoveWindow:
		return t.viewMove()
	default:
		return t.viewList()
	}
}

func (t *CurrentTab) viewList() string {
	var b strings.Builder

	// ── Session header ──
	b.WriteString("\n")

	if t.sessionName == "" {
		b.WriteString(views.RenderEmptyState(
			"No active session.",
			"Open this dashboard from within a tmux session.",
			t.styles.Dim,
		))
		return b.String()
	}

	// Line 1: session name + created info.
	nameStr := t.styles.Accent.Bold(true).Render(t.sessionName)

	b.WriteString("  " + nameStr + "\n")

	// Line 2: dir + window count + attached count.
	var metaParts []string
	if t.sessionDir != "" {
		metaParts = append(metaParts, shortenDir(t.sessionDir))
	}
	metaParts = append(metaParts, fmt.Sprintf("%d tabs", len(t.windows)))
	if t.attached > 0 {
		metaParts = append(metaParts, fmt.Sprintf("%d client", t.attached))
	}
	b.WriteString("  " + t.styles.Dim.Render(strings.Join(metaParts, "  ")) + "\n")

	// Separator.
	lineWidth := t.width - 4
	if lineWidth < 20 {
		lineWidth = 20
	}
	if lineWidth > 60 {
		lineWidth = 60
	}
	b.WriteString("  " + t.styles.Dim.Render(strings.Repeat("━", lineWidth)) + "\n")

	// ── Rename overlay ──
	if t.mode == currentModeRename {
		prompt := t.styles.Accent.Render("  rename ▸ ")
		b.WriteString("\n" + prompt + t.renameInput.View() + "\n")
	}

	// ── Confirm close overlay ──
	if t.mode == currentModeConfirmClose {
		prompt := t.styles.Error.Render("  Close ") +
			t.styles.Error.Bold(true).Render(t.closeTarget) +
			t.styles.Error.Render("? ") +
			t.styles.Dim.Render("(y/N)")
		b.WriteString("\n" + prompt + "\n")
	}

	// ── Empty state ──
	if len(t.windows) == 0 {
		b.WriteString("\n")
		b.WriteString(views.RenderEmptyState(
			"No tabs in this session.",
			"Press n to create a new tab.",
			t.styles.Dim,
		))
		return b.String()
	}

	b.WriteString("\n")

	// ── Window list with scrolling ──
	listHeight := t.height - 10
	if listHeight < 6 {
		listHeight = 12
	}
	entriesVisible := listHeight / 3 // ~3 lines per entry (2 content + 1 blank)
	if entriesVisible < 3 {
		entriesVisible = 3
	}

	start := 0
	if t.cursor >= entriesVisible {
		start = t.cursor - entriesVisible + 1
	}
	end := start + entriesVisible
	if end > len(t.windows) {
		end = len(t.windows)
		start = max(0, end-entriesVisible)
	}

	if start > 0 {
		b.WriteString(t.styles.Dim.Render("      ↑ " + fmt.Sprintf("%d more", start)) + "\n")
	}

	rowStyles := views.SessionRowStyles{
		Normal:  t.styles.Normal,
		Accent:  t.styles.Accent,
		Dim:     t.styles.Dim,
		Info:    t.styles.Info,
		Success: t.styles.Success,
	}

	for i := start; i < end; i++ {
		w := t.windows[i]

		row := views.WindowRow{
			Index:      w.Index,
			Name:       w.Name,
			IsActive:   w.Active,
			IsSelected: i == t.cursor,
			Dir:        shortenDir(w.Dir),
			Uptime:     w.Uptime,
			IsIdle:     isIdleWindow(w),
		}

		// Primary pane command.
		if len(w.Panes) > 0 {
			for _, p := range w.Panes {
				if p.Active {
					row.Command = p.Command
					break
				}
			}
			if row.Command == "" {
				row.Command = w.Panes[0].Command
			}
		}

		// CPU/mem — only show when meaningful.
		if w.Stats.CPU > 0.1 {
			row.CPU = fmt.Sprintf("%.1f%%", w.Stats.CPU)
		}
		if w.Stats.MemMB > 1.0 {
			if w.Stats.MemMB >= 1024 {
				row.Mem = fmt.Sprintf("%.1fGB", w.Stats.MemMB/1024)
			} else {
				row.Mem = fmt.Sprintf("%.0fMB", w.Stats.MemMB)
			}
		}

		b.WriteString(views.RenderWindowRow(row, rowStyles, t.width))
		b.WriteString("\n")
	}

	if end < len(t.windows) {
		b.WriteString(t.styles.Dim.Render("      ↓ " + fmt.Sprintf("%d more", len(t.windows)-end)) + "\n")
	}

	return b.String()
}

func (t *CurrentTab) viewMove() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(t.styles.Accent.Bold(true).Render("Move Tab") + "\n")
	b.WriteString(t.styles.Dim.Render("Move tab to another session") + "\n\n")

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

func (t *CurrentTab) ShortHelp() string {
	switch t.mode {
	case currentModeRename:
		return "enter:confirm  esc:cancel"
	case currentModeConfirmClose:
		return "y:confirm  any:cancel"
	case currentModeMoveWindow:
		return "enter:move  esc:cancel"
	default:
		parts := []string{
			"enter:focus",
			"r:rename",
			"x:close",
			"m:move",
			"</>:reorder",
			"n:new",
		}
		return strings.Join(parts, "  ")
	}
}

// ── Data fetching ──

func (t *CurrentTab) fetchData(reqID int64) tea.Cmd {
	runner := t.runner
	return func() tea.Msg {
		// Get current session name.
		sessionName, err := runner.DisplayMessage("", "#{session_name}")
		if err != nil {
			return currentDataMsg{reqID: reqID, err: err}
		}
		sessionName = strings.TrimSpace(sessionName)

		if sessionName == "" {
			return currentDataMsg{reqID: reqID, err: fmt.Errorf("no active session")}
		}

		// Get session dir and attached count.
		sessionDir, _ := runner.DisplayMessage("", "#{session_path}")
		sessionDir = strings.TrimSpace(sessionDir)

		attachedStr, _ := runner.DisplayMessage("", "#{session_attached}")
		attached := 0
		fmt.Sscanf(strings.TrimSpace(attachedStr), "%d", &attached)

		// Get windows.
		rawWindows, err := runner.ListWindows(sessionName)
		if err != nil {
			return currentDataMsg{reqID: reqID, sessionName: sessionName, err: err}
		}

		// Get all panes in one call (ListPanes with -s lists across all windows).
		rawPanes, _ := runner.ListPanes(sessionName)

		// Map panes to windows using the WindowIndex field from tmux.
		panesByWindow := make(map[int][]tmux.Pane)
		for _, p := range rawPanes {
			panesByWindow[p.WindowIndex] = append(panesByWindow[p.WindowIndex], p)
		}

		// Collect all active pane PIDs for a single batch stats call.
		var pids []int
		pidToWindow := make(map[int]int) // pid → window index
		for _, w := range rawWindows {
			panes := panesByWindow[w.Index]
			for _, p := range panes {
				if p.Active && p.PID > 0 {
					pids = append(pids, p.PID)
					pidToWindow[p.PID] = w.Index
				}
			}
		}

		// Batch process stats: one ps call for all PIDs.
		allStats := tmux.GetBatchProcessStats(pids)

		// Build window details.
		details := make([]windowDetail, 0, len(rawWindows))
		for _, w := range rawWindows {
			wd := windowDetail{
				Window: w,
				Panes:  panesByWindow[w.Index],
			}

			// Find stats for this window's active pane.
			for _, p := range wd.Panes {
				if p.Active && p.PID > 0 {
					if stats, ok := allStats[p.PID]; ok {
						wd.Stats = stats
						wd.Uptime = stats.Uptime
					}
					break
				}
			}

			details = append(details, wd)
		}

		return currentDataMsg{
			reqID:       reqID,
			sessionName: sessionName,
			sessionDir:  sessionDir,
			attached:    attached,
			windows:     details,
		}
	}
}

func (t *CurrentTab) fetchMoveDestinations() tea.Cmd {
	runner := t.runner
	current := t.sessionName
	reqID := t.reqID
	return func() tea.Msg {
		sessions, err := runner.ListSessions()
		if err != nil {
			return currentMoveDestMsg{reqID: reqID}
		}
		var targets []moveTarget
		for _, s := range sessions {
			if s.Name != current {
				targets = append(targets, moveTarget{
					Name:    s.Name,
					Windows: s.Windows,
				})
			}
		}
		return currentMoveDestMsg{
			reqID:    reqID,
			sessions: targets,
		}
	}
}

// ── Helpers ──

func (t *CurrentTab) currentWindowIndex() int {
	if t.cursor >= 0 && t.cursor < len(t.windows) {
		return t.windows[t.cursor].Index
	}
	return -1
}

func (t *CurrentTab) restoreCursor() {
	if t.selectedIdx < 0 {
		t.clampCursor()
		return
	}

	for i, w := range t.windows {
		if w.Index == t.selectedIdx {
			t.cursor = i
			return
		}
	}

	t.clampCursor()
}

func (t *CurrentTab) clampCursor() {
	if t.cursor >= len(t.windows) {
		t.cursor = max(0, len(t.windows)-1)
	}
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
