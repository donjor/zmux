package tmux

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClientNewWindowTargetsNextFreeIndex(t *testing.T) {
	t.Setenv("TMUX", "")
	logPath := filepath.Join(t.TempDir(), "tmux-args.log")
	client := &Client{bin: fakeTmux(t, logPath, `
case "$1" in
  list-windows)
    printf '0	main	1	/tmp	\n2	tests	0	/tmp	\n'
    ;;
  new-window)
    printf '%%42\n'
    ;;
esac
`)}

	paneID, err := client.NewWindow("dev", "agent", "/repo", Detached())
	if err != nil {
		t.Fatalf("NewWindow failed: %v", err)
	}
	if paneID != "%42" {
		t.Fatalf("NewWindow paneID = %q, want %%42", paneID)
	}

	calls := readFakeTmuxCalls(t, logPath)
	if len(calls) != 2 {
		t.Fatalf("expected list-windows then new-window, got %v", calls)
	}
	if !strings.Contains(calls[1], "new-window") || !strings.Contains(calls[1], "-t =dev:3") {
		t.Fatalf("new-window should target next free index =dev:3, calls = %v", calls)
	}
	if !strings.Contains(calls[1], "-d") {
		t.Fatalf("detached option missing from new-window call: %v", calls)
	}
}

func TestClientNewWindowFallsBackToBareSessionWhenWindowsUnreadable(t *testing.T) {
	t.Setenv("TMUX", "")
	logPath := filepath.Join(t.TempDir(), "tmux-args.log")
	client := &Client{bin: fakeTmux(t, logPath, `
case "$1" in
  list-windows)
    printf 'tmux unavailable\n' >&2
    exit 1
    ;;
  new-window)
    printf '%%43\n'
    ;;
esac
`)}

	paneID, err := client.NewWindow("dev", "agent", "/repo")
	if err != nil {
		t.Fatalf("NewWindow failed: %v", err)
	}
	if paneID != "%43" {
		t.Fatalf("NewWindow paneID = %q, want %%43", paneID)
	}

	calls := readFakeTmuxCalls(t, logPath)
	if len(calls) != 2 {
		t.Fatalf("expected list-windows then new-window, got %v", calls)
	}
	if !strings.Contains(calls[1], "new-window") || !strings.Contains(calls[1], "-t =dev -c") {
		t.Fatalf("new-window should fall back to exact session target, calls = %v", calls)
	}
}

func TestClientSessionTargetsUseExactMatch(t *testing.T) {
	t.Setenv("TMUX", "")
	logPath := filepath.Join(t.TempDir(), "tmux-args.log")
	client := &Client{bin: fakeTmux(t, logPath, `
exit 0
`)}

	_ = client.HasSession("zws_skills__skills")
	_, _ = client.ListWindows("zws_skills__skills")
	_ = client.SwitchClient("zws_skills__skills")
	_ = client.KillSession("zws_skills__skills")
	_ = client.RenameSession("zws_skills__skills", "zws_skills__renamed")

	calls := readFakeTmuxCalls(t, logPath)
	wantTargets := []string{
		"-t =zws_skills__skills",
		"-t =zws_skills__skills",
		"-t =zws_skills__skills",
		"-t =zws_skills__skills",
		"-t =zws_skills__skills",
	}
	if len(calls) != len(wantTargets) {
		t.Fatalf("calls = %v", calls)
	}
	for i, want := range wantTargets {
		if !strings.Contains(calls[i], want) {
			t.Fatalf("call %d = %q, want target %q", i, calls[i], want)
		}
	}
}

func fakeTmux(t *testing.T, logPath, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "tmux")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> " + shellQuote(logPath) + "\n" +
		body + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	return path
}

func readFakeTmuxCalls(t *testing.T, logPath string) []string {
	t.Helper()
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read fake tmux log: %v", err)
	}
	return strings.Split(strings.TrimSpace(string(raw)), "\n")
}

func TestClientPipePaneOpensAndClosesPipe(t *testing.T) {
	t.Setenv("TMUX", "")
	logPath := filepath.Join(t.TempDir(), "tmux-args.log")
	client := &Client{bin: fakeTmux(t, logPath, "")}

	if err := client.PipePane("%7", "zmux log-sink --file /tmp/x.log"); err != nil {
		t.Fatalf("PipePane open failed: %v", err)
	}
	if err := client.PipePane("%7", ""); err != nil {
		t.Fatalf("PipePane close failed: %v", err)
	}

	calls := readFakeTmuxCalls(t, logPath)
	if len(calls) != 2 {
		t.Fatalf("expected open then close, got %v", calls)
	}
	if !strings.Contains(calls[0], "pipe-pane -t %7 zmux log-sink --file /tmp/x.log") {
		t.Fatalf("open call should carry the sink command: %v", calls[0])
	}
	// Close must NOT pass a command (a bare pipe-pane -t target closes the pipe).
	if strings.TrimSpace(calls[1]) != "pipe-pane -t %7" {
		t.Fatalf("close call should be a bare pipe-pane with no command: %q", calls[1])
	}
}

func TestTailLines(t *testing.T) {
	tests := []struct {
		name string
		in   string
		n    int
		want string
	}{
		{"trim to last 2 with trailing newline", "a\nb\nc\nd\n", 2, "c\nd\n"},
		{"trim to last 2 no trailing newline", "a\nb\nc\nd", 2, "c\nd"},
		{"fewer lines than n unchanged", "a\nb\n", 5, "a\nb\n"},
		{"exact count unchanged", "a\nb\nc\n", 3, "a\nb\nc\n"},
		{"n<=0 no trim", "a\nb\nc\n", 0, "a\nb\nc\n"},
		{"negative n no trim", "a\nb\nc\n", -10, "a\nb\nc\n"},
		{"empty input", "", 5, ""},
		{"single newline preserved", "\n", 1, "\n"},
		{"joined wrapped line counts once", "wrapped-long-single-logical-line\n", 1, "wrapped-long-single-logical-line\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tailLines(tt.in, tt.n); got != tt.want {
				t.Errorf("tailLines(%q, %d) = %q, want %q", tt.in, tt.n, got, tt.want)
			}
		})
	}
}
