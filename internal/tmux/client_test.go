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
	if !strings.Contains(calls[1], "new-window") || !strings.Contains(calls[1], "-t dev:3") {
		t.Fatalf("new-window should target next free index dev:3, calls = %v", calls)
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
	if !strings.Contains(calls[1], "new-window") || !strings.Contains(calls[1], "-t dev -c") {
		t.Fatalf("new-window should fall back to bare session target, calls = %v", calls)
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
