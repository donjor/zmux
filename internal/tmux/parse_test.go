package tmux

import (
	"testing"
	"time"
)

func TestParseSessions(t *testing.T) {
	input := "dev\t3\t1\t1700000000\t/home/user/dev\n" +
		"work\t2\t0\t1700001000\t/home/user/work\n"

	sessions, err := parseSessions(input)
	if err != nil {
		t.Fatalf("parseSessions: unexpected error: %v", err)
	}

	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	// First session
	s := sessions[0]
	if s.Name != "dev" {
		t.Errorf("session[0].Name = %q, want %q", s.Name, "dev")
	}
	if s.Windows != 3 {
		t.Errorf("session[0].Windows = %d, want %d", s.Windows, 3)
	}
	if !s.Attached {
		t.Error("session[0].Attached = false, want true")
	}
	if s.Activity != time.Unix(1700000000, 0) {
		t.Errorf("session[0].Activity = %v, want %v", s.Activity, time.Unix(1700000000, 0))
	}
	if s.Dir != "/home/user/dev" {
		t.Errorf("session[0].Dir = %q, want %q", s.Dir, "/home/user/dev")
	}

	// Second session
	s = sessions[1]
	if s.Name != "work" {
		t.Errorf("session[1].Name = %q, want %q", s.Name, "work")
	}
	if s.Windows != 2 {
		t.Errorf("session[1].Windows = %d, want %d", s.Windows, 2)
	}
	if s.Attached {
		t.Error("session[1].Attached = true, want false")
	}
	if s.Dir != "/home/user/work" {
		t.Errorf("session[1].Dir = %q, want %q", s.Dir, "/home/user/work")
	}
}

func TestParseSessionsEmpty(t *testing.T) {
	sessions, err := parseSessions("")
	if err != nil {
		t.Fatalf("parseSessions: unexpected error: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestParseSessionsSingleLine(t *testing.T) {
	input := "main\t1\t1\t1700000000\t/home/user"

	sessions, err := parseSessions(input)
	if err != nil {
		t.Fatalf("parseSessions: unexpected error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Name != "main" {
		t.Errorf("session.Name = %q, want %q", sessions[0].Name, "main")
	}
}

func TestParseSessionsInvalidFields(t *testing.T) {
	// Too few fields
	_, err := parseSessions("bad\tdata")
	if err == nil {
		t.Error("expected error for too few fields, got nil")
	}
}

func TestParseSessionsInvalidWindowCount(t *testing.T) {
	_, err := parseSessions("dev\tnotanumber\t1\t1700000000\t/home")
	if err == nil {
		t.Error("expected error for invalid window count, got nil")
	}
}

func TestParseWindows(t *testing.T) {
	input := "1\teditor\t1\t/home/user/dev\n" +
		"2\tshell\t0\t/home/user\n" +
		"3\tlogs\t0\t/var/log\n"

	windows, err := parseWindows(input)
	if err != nil {
		t.Fatalf("parseWindows: unexpected error: %v", err)
	}

	if len(windows) != 3 {
		t.Fatalf("expected 3 windows, got %d", len(windows))
	}

	// First window
	w := windows[0]
	if w.Index != 1 {
		t.Errorf("window[0].Index = %d, want %d", w.Index, 1)
	}
	if w.Name != "editor" {
		t.Errorf("window[0].Name = %q, want %q", w.Name, "editor")
	}
	if !w.Active {
		t.Error("window[0].Active = false, want true")
	}
	if w.Dir != "/home/user/dev" {
		t.Errorf("window[0].Dir = %q, want %q", w.Dir, "/home/user/dev")
	}

	// Second window
	w = windows[1]
	if w.Index != 2 {
		t.Errorf("window[1].Index = %d, want %d", w.Index, 2)
	}
	if w.Name != "shell" {
		t.Errorf("window[1].Name = %q, want %q", w.Name, "shell")
	}
	if w.Active {
		t.Error("window[1].Active = true, want false")
	}
}

func TestParseWindowsEmpty(t *testing.T) {
	windows, err := parseWindows("")
	if err != nil {
		t.Fatalf("parseWindows: unexpected error: %v", err)
	}
	if len(windows) != 0 {
		t.Fatalf("expected 0 windows, got %d", len(windows))
	}
}

func TestParseWindowsInvalidFields(t *testing.T) {
	_, err := parseWindows("bad\tdata")
	if err == nil {
		t.Error("expected error for too few fields, got nil")
	}
}

func TestParseWindowsInvalidIndex(t *testing.T) {
	_, err := parseWindows("abc\teditor\t1\t/home")
	if err == nil {
		t.Error("expected error for invalid index, got nil")
	}
}

func TestParseSessionsTrailingNewlines(t *testing.T) {
	input := "dev\t3\t1\t1700000000\t/home/user/dev\n\n\n"

	sessions, err := parseSessions(input)
	if err != nil {
		t.Fatalf("parseSessions: unexpected error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
}
