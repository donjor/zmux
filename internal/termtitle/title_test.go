package termtitle

import (
	"errors"
	"strings"
	"testing"
)

func TestParseValidMarkerWithSuffix(t *testing.T) {
	m, err := Parse("zmux:v1;tty=/dev/pts/13;sid=$28;wid=@50;pane=%139 pi:1:parley")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if m.TTY != "/dev/pts/13" || m.SessionID != "$28" || m.WindowID != "@50" || m.PaneID != "%139" {
		t.Fatalf("unexpected metadata: %#v", m)
	}
	if !m.Matches("/dev/pts/13", "$28", "@50", "%139") {
		t.Fatal("expected metadata to match exact tmux ids")
	}
}

func TestParseMarkerEmbeddedInTitle(t *testing.T) {
	m, err := Parse("prefix zmux:v1;tty=/dev/pts/1;sid=$6;wid=@11;pane=%11 suffix")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if m.SessionID != "$6" || m.PaneID != "%11" {
		t.Fatalf("unexpected metadata: %#v", m)
	}
}

func TestParseNoMarker(t *testing.T) {
	_, err := Parse("Ghostty")
	if !errors.Is(err, ErrNoMarker) {
		t.Fatalf("expected ErrNoMarker, got %v", err)
	}
}

func TestParseUnsupportedVersion(t *testing.T) {
	_, err := Parse("zmux:v2;tty=/dev/pts/13;sid=$28;wid=@50;pane=%139")
	if !errors.Is(err, ErrUnsupportedVersion) {
		t.Fatalf("expected ErrUnsupportedVersion, got %v", err)
	}
}

func TestParseMissingField(t *testing.T) {
	_, err := Parse("zmux:v1;tty=/dev/pts/13;sid=$28;wid=@50")
	if !errors.Is(err, ErrMissingField) {
		t.Fatalf("expected ErrMissingField, got %v", err)
	}
}

func TestTmuxTitleFormatIsV1(t *testing.T) {
	for _, want := range []string{"zmux:v1", "tty=#{client_tty}", "sid=#{session_id}", "wid=#{window_id}", "pane=#{pane_id}", "@zmux_workspace", "@zmux_session_label"} {
		if !strings.Contains(TmuxTitleFormat, want) {
			t.Fatalf("TmuxTitleFormat missing %q: %s", want, TmuxTitleFormat)
		}
	}
	if strings.Contains(TmuxTitleFormat, "pane_pid") {
		t.Fatal("title format must not include volatile pane_pid")
	}
}
