// Package termtitle parses zmux terminal-title correlation metadata.
package termtitle

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	SchemaVersion = "zmux-terminal-title/v1"
	MarkerPrefix  = "zmux:v1;"
)

// TmuxTitleFormat is the tmux set-titles-string fragment that zmux owns.
const TmuxTitleFormat = "zmux:v1;tty=#{client_tty};sid=#{session_id};wid=#{window_id};pane=#{pane_id} #{session_name}:#{window_index}:#{window_name}"

var markerRE = regexp.MustCompile(`zmux:v([0-9]+);\S+`)

// Metadata is the machine-readable zmux:v1 token embedded in terminal titles.
type Metadata struct {
	TTY       string
	SessionID string
	WindowID  string
	PaneID    string
}

// Parse extracts and validates the first zmux terminal metadata token from a title.
func Parse(title string) (Metadata, error) {
	match := markerRE.FindStringSubmatch(title)
	if match == nil {
		return Metadata{}, ErrNoMarker
	}
	if match[1] != "1" {
		return Metadata{}, fmt.Errorf("%w: v%s", ErrUnsupportedVersion, match[1])
	}

	token := match[0]
	if !strings.HasPrefix(token, MarkerPrefix) {
		return Metadata{}, ErrMalformedMarker
	}
	payload := strings.TrimPrefix(token, MarkerPrefix)
	values := make(map[string]string)
	for _, part := range strings.Split(payload, ";") {
		key, value, ok := strings.Cut(part, "=")
		if !ok || key == "" || value == "" {
			return Metadata{}, ErrMalformedMarker
		}
		values[key] = value
	}

	m := Metadata{
		TTY:       values["tty"],
		SessionID: values["sid"],
		WindowID:  values["wid"],
		PaneID:    values["pane"],
	}
	if err := m.Validate(); err != nil {
		return Metadata{}, err
	}
	return m, nil
}

// Validate checks that all strict v1 fields are present.
func (m Metadata) Validate() error {
	if m.TTY == "" || m.SessionID == "" || m.WindowID == "" || m.PaneID == "" {
		return ErrMissingField
	}
	return nil
}

// Matches reports whether the marker exactly identifies the given tmux view.
func (m Metadata) Matches(tty, sessionID, windowID, paneID string) bool {
	return m.TTY == tty && m.SessionID == sessionID && m.WindowID == windowID && m.PaneID == paneID
}

var (
	ErrNoMarker           = fmt.Errorf("no zmux terminal metadata marker")
	ErrUnsupportedVersion = fmt.Errorf("unsupported zmux terminal metadata version")
	ErrMalformedMarker    = fmt.Errorf("malformed zmux terminal metadata marker")
	ErrMissingField       = fmt.Errorf("missing zmux terminal metadata field")
)
