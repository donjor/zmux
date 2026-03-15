package tmux

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// parseSessions parses tab-delimited list-sessions output into []Session.
// Expected format per line: name\twindows\tattached\tactivity\tdir
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

		fields := strings.SplitN(line, "\t", 5)
		if len(fields) < 5 {
			return nil, fmt.Errorf("expected 5 tab-delimited fields, got %d: %q", len(fields), line)
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

		sessions = append(sessions, Session{
			Name:     fields[0],
			Windows:  windows,
			Attached: attached,
			Activity: time.Unix(activitySec, 0),
			Dir:      fields[4],
		})
	}

	return sessions, nil
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
