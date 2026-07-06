package cli

import (
	"testing"
	"time"

	"github.com/donjor/zmux/internal/tmux"
)

func TestSendKeysToWindow(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}

	rootCmd.SetArgs([]string{"send", "server", "C-c", "-s", "test-session"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("send command failed: %v", err)
	}

	found := false
	for _, c := range mock.Calls {
		if c.Method == "SendKeys" {
			found = true
			// Should target "test-session:server"
			if len(c.Args) > 0 && c.Args[0] != "test-session:server" {
				t.Errorf("expected target 'test-session:server', got %q", c.Args[0])
			}
		}
	}
	if !found {
		t.Error("expected SendKeys call")
	}
}

// When a long-running process has auto-renamed the window away from its zmux
// name, send must still find it via @zmux_label and target it by index —
// session:name no longer resolves.
func TestSendResolvesAutoRenamedWindowByLabel(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	mock.Windows["test-session"] = []tmux.Window{
		{Index: 4, Name: "node", Label: "server", Active: true},
	}

	rootCmd.SetArgs([]string{"send", "server", "C-c", "-s", "test-session"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("send command failed: %v", err)
	}

	for _, c := range mock.Calls {
		if c.Method == "SendKeys" {
			if len(c.Args) == 0 || c.Args[0] != "test-session:4" {
				t.Errorf("expected target 'test-session:4' (by index), got %v", c.Args)
			}
		}
	}
}

// type shares the resolver — confirm it also reaches an auto-renamed window
// via @zmux_label, targeted by index.
func TestTypeResolvesAutoRenamedWindowByLabel(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	mock.Windows["test-session"] = []tmux.Window{
		{Index: 4, Name: "node", Label: "server", Active: true},
	}

	rootCmd.SetArgs([]string{"type", "server", "echo hi", "-s", "test-session"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("type command failed: %v", err)
	}

	for _, c := range mock.Calls {
		if c.Method == "SendKeys" {
			if len(c.Args) == 0 || c.Args[0] != "test-session:4" {
				t.Errorf("expected target 'test-session:4' (by index), got %v", c.Args)
			}
		}
	}
}

// send claims an unlabeled live-name match as a logical tab — pane-scoped id
// + pane-canonical label — and must do so BEFORE the keys, so a `send X C-c`
// that drifts the name still leaves X reachable for the follow-up command.
func TestSendClaimsLabelBeforeKeysOnNameMatch(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	mock.Windows["test-session"] = []tmux.Window{
		{Index: 2, Name: "devserver", Active: true}, // matched by name, no label yet
	}
	mock.DisplayMessageFunc = displayByFormat(map[string]string{
		"#{pane_id}": "%4\ttest-session:2\n",
	})

	rootCmd.SetArgs([]string{"send", "devserver", "C-c", "-s", "test-session"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("send command failed: %v", err)
	}

	idIdx, labelIdx, sendIdx := -1, -1, -1
	for i, c := range mock.Calls {
		if c.Method == "ApplyOptions" && c.Args[0] == "-p" && c.Args[1] == "%4" {
			switch {
			case c.Args[2] == "@zmux_tab_id":
				idIdx = i
			case c.Args[2] == "@zmux_label" && c.Args[3] == "devserver":
				labelIdx = i
			}
		}
		if c.Method == "SendKeys" {
			sendIdx = i
		}
	}
	if idIdx == -1 || labelIdx == -1 {
		t.Fatalf("expected send to claim the unlabeled name-match (id=%d label=%d)", idIdx, labelIdx)
	}
	if sendIdx == -1 || labelIdx > sendIdx {
		t.Errorf("expected label claim (idx %d) BEFORE SendKeys (idx %d)", labelIdx, sendIdx)
	}
}

// watch is read-only: it resolves an auto-renamed window by label but must
// never claim/mutate window options.
func TestWatchDoesNotClaimLabel(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}
	mock.Windows["test-session"] = []tmux.Window{
		{Index: 2, Name: "logs", Active: true}, // unlabeled name-match
	}
	mock.CapturedPaneContent = "x\n"

	rootCmd.SetArgs([]string{"watch", "logs", "-s", "test-session"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("watch command failed: %v", err)
	}

	for _, c := range mock.Calls {
		if c.Method == "SetWindowOption" {
			t.Errorf("watch must not mutate window options, got %s %v", c.Method, c.Args)
		}
	}
}

// type must deliver text and Enter as SEPARATE SendKeys calls — gluing them
// into one call makes TUI paste-burst detection absorb the Enter as a
// newline and the message silently never submits.
func TestTypeSendsTextThenEnterSeparately(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session"}}

	rootCmd.SetArgs([]string{"type", "git", "git status", "-s", "test-session"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("type command failed: %v", err)
	}

	var sends [][]string
	for _, c := range mock.Calls {
		if c.Method == "SendKeys" {
			sends = append(sends, c.Args)
		}
	}
	if len(sends) != 2 {
		t.Fatalf("expected 2 SendKeys calls (text, then Enter), got %d: %v", len(sends), sends)
	}
	if len(sends[0]) != 3 || sends[0][1] != "-l" || sends[0][2] != "git status" {
		t.Errorf("first call should send the text literally only, got %v", sends[0])
	}
	if len(sends[1]) != 2 || sends[1][1] != "Enter" {
		t.Errorf("second call should send Enter only, got %v", sends[1])
	}
}

func TestTypeGapScalesWithPasteSize(t *testing.T) {
	if g := typeGap(10); g != 770*time.Millisecond {
		t.Errorf("short text: got %v", g)
	}
	if g := typeGap(1020); g != 2500*time.Millisecond {
		t.Errorf("1KB paste must use the bounded high-safety TUI gap: got %v", g)
	}
	if g := typeGap(100_000); g != 2500*time.Millisecond {
		t.Errorf("cap: got %v", g)
	}
}
