package tmux

import (
	"strconv"
	"strings"
	"time"
)

// LogicalPaneRow is one row of the logical-tab scan: the raw per-pane facts
// the tabs layer needs to compute logical tabs and their placements from a
// single list-panes -a round-trip. Deliberately a dedicated type — Pane
// serves window-local listings and would bloat for every other consumer.
type LogicalPaneRow struct {
	PaneID          string // opaque tmux pane id, e.g. %57
	Session         string // session_name
	SessionGroup    string // session group name, empty if ungrouped
	SessionAttached int    // attached client count for the owning session
	WindowID        string // opaque tmux window id, e.g. @12
	WindowIndex     int
	WindowName      string
	WindowActive    bool // window is the session's current window
	WindowPanes     int  // pane count in the owning window
	WindowActivity  time.Time
	PaneActive      bool   // pane is its window's active pane
	Command         string // pane_current_command
	Dir             string // pane_current_path
	Title           string // pane_title

	// User options. TabID is authoritative for "zmux-managed": it is only
	// ever set at pane scope, so a non-empty value here always means this
	// exact pane. The rest are merged-scope reads — tmux format expansion
	// falls back pane → window → session, so e.g. Label can reflect a
	// window-level value (legacy label or stale mirror) when the pane has
	// none. Scope-exact reads go through ShowPaneOption/ShowWindowOption.
	TabID       string // @zmux_tab_id
	Label       string // @zmux_label
	LabelSource string // @zmux_label_source
	State       string // @zmux_state
	Anchor      string // @zmux_tab_anchor (advisory pane-of host)
	Hidden      string // @zmux_hidden (origin session name while docked)

	// Lifecycle (plan 038). Raw option values; parse via tabs.ParseUnix/ParseTTL.
	// Merged-scope reads like the rest, but these are only ever written at pane
	// scope, so the merged value == the pane value.
	Origin      string // @zmux_origin (agent|human|preexisting)
	Born        string // @zmux_born (unix seconds)
	Scope       string // @zmux_scope
	TTL         string // @zmux_ttl (seconds)
	Keep        string // @zmux_keep ("1" = never reap)
	StaleAt     string // @zmux_stale_at (unix seconds, set by an earlier sweep)
	LastInputAt string // @zmux_last_input_at (unix seconds)

	// Peer/agent-turn lifecycle metadata. Policy-level state, distinct from
	// @zmux_state's human-facing glyph mirror.
	TurnState    string // @zmux_turn_state
	TurnAt       string // @zmux_turn_at (unix seconds)
	PeerRole     string // @zmux_peer_role
	PeerHostTab  string // @zmux_peer_host_tab
	PeerHostPane string // @zmux_peer_host_pane
	PeerTopic    string // @zmux_peer_topic
	PeerTurns    string // @zmux_peer_turns
	PeerLastTurn string // @zmux_peer_last_turn
	KeepUntil    string // @zmux_keep_until (unix seconds)
	ParkUntil    string // @zmux_park_until (unix seconds)

	PanePID int // pane_pid — root of the pane's foreground process tree
}

const logicalRowFields = 38

// logicalRowFormat must stay in field-lockstep with parseLogicalRows and
// LogicalPaneRow. TAB-separated: tmux passes TAB through format output
// verbatim (control chars like \x1f get octal-escaped instead and are
// unusable as separators).
const logicalRowFormat = "#{pane_id}\t#{session_name}\t#{session_group}\t#{session_attached}\t" +
	"#{window_id}\t#{window_index}\t#{window_name}\t#{window_active}\t#{window_panes}\t#{window_activity}\t" +
	"#{pane_active}\t#{pane_current_command}\t#{pane_current_path}\t#{pane_title}\t" +
	"#{@zmux_tab_id}\t#{@zmux_label}\t#{@zmux_label_source}\t#{@zmux_state}\t#{@zmux_tab_anchor}\t#{@zmux_hidden}\t" +
	"#{@zmux_origin}\t#{@zmux_born}\t#{@zmux_scope}\t#{@zmux_ttl}\t#{@zmux_keep}\t#{@zmux_stale_at}\t#{@zmux_last_input_at}\t" +
	"#{@zmux_turn_state}\t#{@zmux_turn_at}\t#{@zmux_peer_role}\t#{@zmux_peer_host_tab}\t#{@zmux_peer_host_pane}\t#{@zmux_peer_topic}\t" +
	"#{@zmux_peer_turns}\t#{@zmux_peer_last_turn}\t#{@zmux_keep_until}\t#{@zmux_park_until}\t#{pane_pid}"

// ListLogicalPaneRows scans every pane on the server (list-panes -a) and
// returns one LogicalPaneRow per pane, unset options as empty strings.
func (c *Client) ListLogicalPaneRows() ([]LogicalPaneRow, error) {
	out, err := c.run("list-panes", "-a", "-F", logicalRowFormat)
	if err != nil {
		return nil, err
	}
	return parseLogicalRows(out), nil
}

// parseLogicalRows parses TAB-delimited scan output into rows. Lines are
// padded to the full field count before decoding: rows whose trailing user
// options are all unset end in literal TABs, which run()'s TrimSpace eats on
// the final line of output.
func parseLogicalRows(output string) []LogicalPaneRow {
	if output == "" {
		return nil
	}
	lines := strings.Split(output, "\n")
	rows := make([]LogicalPaneRow, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		f := strings.SplitN(line, "\t", logicalRowFields)
		for len(f) < logicalRowFields {
			f = append(f, "")
		}
		attached, _ := strconv.Atoi(f[3])
		windowIndex, _ := strconv.Atoi(f[5])
		windowPanes, _ := strconv.Atoi(f[8])
		panePID, _ := strconv.Atoi(f[37])
		var activity time.Time
		if sec, err := strconv.ParseInt(f[9], 10, 64); err == nil {
			activity = time.Unix(sec, 0)
		}
		rows = append(rows, LogicalPaneRow{
			PaneID:          f[0],
			Session:         f[1],
			SessionGroup:    f[2],
			SessionAttached: attached,
			WindowID:        f[4],
			WindowIndex:     windowIndex,
			WindowName:      f[6],
			WindowActive:    f[7] == "1",
			WindowPanes:     windowPanes,
			WindowActivity:  activity,
			PaneActive:      f[10] == "1",
			Command:         f[11],
			Dir:             f[12],
			Title:           f[13],
			TabID:           f[14],
			Label:           f[15],
			LabelSource:     f[16],
			State:           f[17],
			Anchor:          f[18],
			Hidden:          f[19],
			Origin:          f[20],
			Born:            f[21],
			Scope:           f[22],
			TTL:             f[23],
			Keep:            f[24],
			StaleAt:         f[25],
			LastInputAt:     f[26],
			TurnState:       f[27],
			TurnAt:          f[28],
			PeerRole:        f[29],
			PeerHostTab:     f[30],
			PeerHostPane:    f[31],
			PeerTopic:       f[32],
			PeerTurns:       f[33],
			PeerLastTurn:    f[34],
			KeepUntil:       f[35],
			ParkUntil:       f[36],
			PanePID:         panePID,
		})
	}
	return rows
}
