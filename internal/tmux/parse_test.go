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

func TestParseSessionsPinnedViewMetadata(t *testing.T) {
	input := "zws_dev__main__clone_b\t1\t0\t1700000000\t/repo\t1700000000\t0\tzws_dev__main\t1\tdev\tmain\ts_1\t1\t1\tzws_dev__main"

	sessions, err := parseSessions(input)
	if err != nil {
		t.Fatalf("parseSessions: unexpected error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	s := sessions[0]
	if !s.Clone || !s.PinnedView || s.ViewRoot != "zws_dev__main" {
		t.Fatalf("clone/pin/root = %v/%v/%q", s.Clone, s.PinnedView, s.ViewRoot)
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

func TestParseWindowsLabelField(t *testing.T) {
	// 5th field is the @zmux_label overlay; absent/empty must yield "".
	input := "0\tnode\t1\t/home/user\tserver\n" + // labeled (auto-renamed away)
		"1\teditor\t0\t/home/user\t\n" + // present-but-empty label
		"2\tshell\t0\t/home/user\n" // legacy 4-field line

	windows, err := parseWindows(input)
	if err != nil {
		t.Fatalf("parseWindows: unexpected error: %v", err)
	}
	if len(windows) != 3 {
		t.Fatalf("expected 3 windows, got %d", len(windows))
	}
	if windows[0].Name != "node" || windows[0].Label != "server" {
		t.Errorf("window[0] = {Name:%q Label:%q}, want {node server}", windows[0].Name, windows[0].Label)
	}
	if windows[1].Label != "" {
		t.Errorf("window[1].Label = %q, want empty", windows[1].Label)
	}
	if windows[2].Label != "" {
		t.Errorf("window[2].Label = %q, want empty (legacy 4-field)", windows[2].Label)
	}
}

func TestParseClients(t *testing.T) {
	input := "/dev/pts/13\tpi\t$28\tpi\t@50\t1\tparley\t%139\t2028292\t0\txterm-256color\tbpaste,RGB,title\tattached,focused,UTF-8\n" +
		"/dev/pts/26\tbridge-b\t$21\tbridge\t@36\t3\tbash\t%36\t3055165\t1\txterm-ghostty\tbpaste,title\tattached,UTF-8\n"
	clients, err := parseClients(input)
	if err != nil {
		t.Fatalf("parseClients: unexpected error: %v", err)
	}
	if len(clients) != 2 {
		t.Fatalf("expected 2 clients, got %d", len(clients))
	}
	c := clients[0]
	if c.TTY != "/dev/pts/13" || c.SessionName != "pi" || c.SessionID != "$28" || c.SessionGroup != "pi" || c.WindowID != "@50" || c.WindowIndex != 1 || c.WindowName != "parley" || c.PaneID != "%139" || c.PID != 2028292 || c.ControlMode || c.TermName != "xterm-256color" || c.TermFeatures != "bpaste,RGB,title" || c.Flags != "attached,focused,UTF-8" {
		t.Fatalf("unexpected client[0]: %#v", c)
	}
	if !clients[1].ControlMode {
		t.Fatalf("expected client[1] control mode: %#v", clients[1])
	}
}

func TestParseClientsAcceptsLegacyTenFieldFormat(t *testing.T) {
	input := "/dev/pts/13\tpi\t$28\tpi\t@50\t1\tparley\t%139\t2028292\t0\n"
	clients, err := parseClients(input)
	if err != nil {
		t.Fatalf("parseClients legacy format: unexpected error: %v", err)
	}
	if len(clients) != 1 {
		t.Fatalf("expected 1 client, got %d", len(clients))
	}
	if clients[0].TTY != "/dev/pts/13" || clients[0].TermName != "" || clients[0].TermFeatures != "" || clients[0].Flags != "" {
		t.Fatalf("unexpected legacy client: %#v", clients[0])
	}
}

func TestParseClientsInvalidFields(t *testing.T) {
	_, err := parseClients("/dev/pts/13\tpi")
	if err == nil {
		t.Fatal("expected invalid client field error")
	}
}

func TestParsePanes(t *testing.T) {
	input := "dev\t%57\t1\t1\tnvim\t1234\t/home/user/dev\t120\t40\tclean-ui\t2\teditor\n" +
		"dev\t%58\t2\t0\tzsh\t1235\t/home/user/dev\t80\t40\tlogs\t2\teditor\n"

	panes, err := parsePanes(input)
	if err != nil {
		t.Fatalf("parsePanes: unexpected error: %v", err)
	}
	if len(panes) != 2 {
		t.Fatalf("expected 2 panes, got %d", len(panes))
	}
	p := panes[0]
	if p.Session != "dev" || p.ID != "%57" || p.Index != 1 || !p.Active || p.Command != "nvim" || p.PID != 1234 || p.Dir != "/home/user/dev" || p.Width != 120 || p.Height != 40 || p.Title != "clean-ui" || p.WindowIndex != 2 || p.WindowName != "editor" {
		t.Fatalf("unexpected pane[0]: %#v", p)
	}
	if panes[1].ID != "%58" || panes[1].Active {
		t.Fatalf("unexpected pane[1]: %#v", panes[1])
	}
}

func TestParseWindowsAllowsEmptyDir(t *testing.T) {
	windows, err := parseWindows("2\t[tmux]\t0\t")
	if err != nil {
		t.Fatalf("parseWindows empty dir: unexpected error: %v", err)
	}
	if len(windows) != 1 {
		t.Fatalf("expected 1 window, got %d", len(windows))
	}
	if windows[0].Index != 2 || windows[0].Name != "[tmux]" || windows[0].Active || windows[0].Dir != "" {
		t.Fatalf("unexpected empty-dir window: %#v", windows[0])
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
