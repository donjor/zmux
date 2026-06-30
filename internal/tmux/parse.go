package tmux

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// parseSessions parses tab-delimited list-sessions output into []Session.
// Expected format per line:
// name\twindows\tattached\tactivity\tdir\tcreated\tlast_attached\tgroup[\tmanaged\tworkspace\tlabel\tid\tclone\tpinned_view\tview_root]
func parseSessions(output string) ([]Session, error) {
	if output == "" {
		return nil, nil
	}

	lines := strings.Split(output, "\n")
	sessions := make([]Session, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.SplitN(line, "\t", 15)
		if len(fields) < 5 {
			return nil, fmt.Errorf("expected at least 5 tab-delimited fields, got %d: %q", len(fields), line)
		}

		windows, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, fmt.Errorf("invalid window count %q: %w", fields[1], err)
		}

		attached := fields[2] == "1"

		activitySec, err := strconv.ParseInt(fields[3], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid activity timestamp %q: %w", fields[3], err)
		}

		s := Session{
			Name:     fields[0],
			Windows:  windows,
			Attached: attached,
			Activity: time.Unix(activitySec, 0),
			Dir:      fields[4],
		}

		// Parse optional created timestamp (field 5).
		if len(fields) > 5 && fields[5] != "" {
			if sec, err := strconv.ParseInt(fields[5], 10, 64); err == nil {
				s.Created = time.Unix(sec, 0)
			}
		}

		// Parse optional last_attached timestamp (field 6).
		if len(fields) > 6 && fields[6] != "" {
			if sec, err := strconv.ParseInt(fields[6], 10, 64); err == nil {
				s.LastAttached = time.Unix(sec, 0)
			}
		}

		// Parse optional group name (field 7).
		if len(fields) > 7 && fields[7] != "" {
			s.Group = fields[7]
		}
		if len(fields) > 8 {
			s.Managed = fields[8] == "1" || fields[8] == "true"
		}
		if len(fields) > 9 {
			s.Workspace = fields[9]
		}
		if len(fields) > 10 {
			s.SessionLabel = fields[10]
		}
		if len(fields) > 11 {
			s.SessionID = fields[11]
		}
		if len(fields) > 12 {
			s.Clone = fields[12] == "1" || fields[12] == "true"
		}
		if len(fields) > 13 {
			s.PinnedView = fields[13] == "1" || fields[13] == "true"
		}
		if len(fields) > 14 {
			s.ViewRoot = fields[14]
		}

		sessions = append(sessions, s)
	}

	return sessions, nil
}

// parseClients parses tab-delimited list-clients output into []ClientInfo.
// Expected format per line: tty\tclient_session\tsession_id\tsession_group\twindow_id\twindow_index\twindow_name\tpane_id\tclient_pid\tclient_control_mode[\tclient_termname\tclient_termfeatures\tclient_flags]
func parseClients(output string) ([]ClientInfo, error) {
	if output == "" {
		return nil, nil
	}

	lines := strings.Split(output, "\n")
	clients := make([]ClientInfo, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, "\t", 13)
		if len(fields) < 10 {
			return nil, fmt.Errorf("expected 10 tab-delimited client fields, got %d: %q", len(fields), line)
		}
		windowIndex, err := strconv.Atoi(fields[5])
		if err != nil {
			return nil, fmt.Errorf("invalid client window index %q: %w", fields[5], err)
		}
		pid, err := strconv.Atoi(fields[8])
		if err != nil {
			return nil, fmt.Errorf("invalid client pid %q: %w", fields[8], err)
		}
		client := ClientInfo{
			TTY:          fields[0],
			SessionName:  fields[1],
			SessionID:    fields[2],
			SessionGroup: fields[3],
			WindowID:     fields[4],
			WindowIndex:  windowIndex,
			WindowName:   fields[6],
			PaneID:       fields[7],
			PID:          pid,
			ControlMode:  fields[9] == "1",
		}
		if len(fields) > 10 {
			client.TermName = fields[10]
		}
		if len(fields) > 11 {
			client.TermFeatures = fields[11]
		}
		if len(fields) > 12 {
			client.Flags = fields[12]
		}
		clients = append(clients, client)
	}
	return clients, nil
}

// parsePanes parses tab-delimited list-panes output into []Pane.
// Expected format per line: session_name\tpane_id\tpane_index\tpane_active\tcommand\tpid\tdir\twidth\theight\ttitle\twindow_index
func parsePanes(output string) ([]Pane, error) {
	if output == "" {
		return nil, nil
	}

	lines := strings.Split(output, "\n")
	panes := make([]Pane, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.SplitN(line, "\t", 12)
		if len(fields) < 10 {
			continue // skip malformed lines gracefully
		}

		index, _ := strconv.Atoi(fields[2])
		active := fields[3] == "1"
		pid, _ := strconv.Atoi(fields[5])
		width, _ := strconv.Atoi(fields[7])
		height, _ := strconv.Atoi(fields[8])

		windowIndex := 0
		if len(fields) >= 11 {
			windowIndex, _ = strconv.Atoi(fields[10])
		}
		windowName := ""
		if len(fields) >= 12 {
			windowName = fields[11]
		}

		panes = append(panes, Pane{
			Session:     fields[0],
			ID:          fields[1],
			Index:       index,
			WindowIndex: windowIndex,
			WindowName:  windowName,
			Active:      active,
			Command:     fields[4],
			PID:         pid,
			Dir:         fields[6],
			Width:       width,
			Height:      height,
			Title:       fields[9],
		})
	}

	return panes, nil
}

// parseWindows parses tab-delimited list-windows output into []Window.
// Expected format per line: index\tname\tactive\tdir[\tlabel]
// The label (@zmux_label) field is optional for backward compatibility.
func parseWindows(output string) ([]Window, error) {
	if output == "" {
		return nil, nil
	}

	lines := strings.Split(output, "\n")
	windows := make([]Window, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.SplitN(line, "\t", 5)
		if len(fields) < 4 {
			return nil, fmt.Errorf("expected at least 4 tab-delimited fields, got %d: %q", len(fields), line)
		}

		index, err := strconv.Atoi(fields[0])
		if err != nil {
			return nil, fmt.Errorf("invalid window index %q: %w", fields[0], err)
		}

		active := fields[2] == "1"

		label := ""
		if len(fields) > 4 {
			label = fields[4]
		}

		windows = append(windows, Window{
			Index:  index,
			Name:   fields[1],
			Active: active,
			Dir:    fields[3],
			Label:  label,
		})
	}

	return windows, nil
}
