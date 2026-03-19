package tmux

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// parseSessions parses tab-delimited list-sessions output into []Session.
// Expected format per line: name\twindows\tattached\tactivity\tdir\tcreated\tlast_attached
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

		fields := strings.SplitN(line, "\t", 7)
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

		sessions = append(sessions, s)
	}

	return sessions, nil
}

// parsePanes parses tab-delimited list-panes output into []Pane.
// Expected format per line: pane_index\tpane_active\tcommand\tpid\tdir\twidth\theight\ttitle\twindow_index
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

		fields := strings.SplitN(line, "\t", 9)
		if len(fields) < 8 {
			continue // skip malformed lines gracefully
		}

		index, _ := strconv.Atoi(fields[0])
		active := fields[1] == "1"
		pid, _ := strconv.Atoi(fields[3])
		width, _ := strconv.Atoi(fields[5])
		height, _ := strconv.Atoi(fields[6])

		windowIndex := 0
		if len(fields) >= 9 {
			windowIndex, _ = strconv.Atoi(fields[8])
		}

		panes = append(panes, Pane{
			Index:       index,
			WindowIndex: windowIndex,
			Active:      active,
			Command:     fields[2],
			PID:         pid,
			Dir:         fields[4],
			Width:       width,
			Height:      height,
			Title:       fields[7],
		})
	}

	return panes, nil
}

// parseWindows parses tab-delimited list-windows output into []Window.
// Expected format per line: index\tname\tactive\tdir
func parseWindows(output string) ([]Window, error) {
	if output == "" {
		return nil, nil
	}

	lines := strings.Split(output, "\n")
	windows := make([]Window, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.SplitN(line, "\t", 4)
		if len(fields) < 4 {
			return nil, fmt.Errorf("expected 4 tab-delimited fields, got %d: %q", len(fields), line)
		}

		index, err := strconv.Atoi(fields[0])
		if err != nil {
			return nil, fmt.Errorf("invalid window index %q: %w", fields[0], err)
		}

		active := fields[2] == "1"

		windows = append(windows, Window{
			Index:  index,
			Name:   fields[1],
			Active: active,
			Dir:    fields[3],
		})
	}

	return windows, nil
}
