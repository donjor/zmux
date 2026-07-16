package runexec

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tmux"
)

// These characterize the run-mechanics units T-402 relocated here from
// internal/cli (script writing, run-result lifecycle polling, nonce/exit
// parsing, start-grace clamping) plus the two loops that only became
// deterministically testable once the ctx/writers seam existed: WaitResult
// interrupt and Follow growth/reset.

// --- WriteCommandScript content contract ---

func TestWriteCommandScriptContract(t *testing.T) {
	path, cleanup, err := WriteCommandScript("echo hi", 30*time.Second)
	if err != nil {
		t.Fatalf("WriteCommandScript: %v", err)
	}
	if cleanup != nil {
		defer cleanup()
	}
	defer os.Remove(path)

	if !strings.HasPrefix(filepath.Base(path), "zmux-cmd-") || !strings.HasSuffix(path, ".sh") {
		t.Fatalf("script path not a zmux-cmd temp file: %q", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read script: %v", err)
	}
	script := string(data)

	wantParts := []string{
		"#!/usr/bin/env bash\n",                    // shebang
		`printf '\033[2m$ %s\033[0m\n'`,            // dim echo of the command
		"__zmux_cmd_cleanup() { __zmux_status=$?;", // preserve exit status
		"rm -f",                        // self-delete
		"trap __zmux_cmd_cleanup EXIT", // cleanup on exit
		"echo hi\n",                    // the command body, verbatim
	}
	for _, part := range wantParts {
		if !strings.Contains(script, part) {
			t.Fatalf("script missing %q:\n%s", part, script)
		}
	}
}

func TestWriteCommandScriptEscapesEchoedCommand(t *testing.T) {
	// The echoed (printf %q) copy escapes backslashes and quotes so the dim
	// preview line cannot break the script; the executed body stays verbatim.
	path, cleanup, err := WriteCommandScript(`grep "a\b" .`, 30*time.Second)
	if err != nil {
		t.Fatalf("WriteCommandScript: %v", err)
	}
	if cleanup != nil {
		defer cleanup()
	}
	defer os.Remove(path)
	data, _ := os.ReadFile(path)
	script := string(data)
	if !strings.Contains(script, `\\b`) || !strings.Contains(script, `\"a`) {
		t.Fatalf("echoed command not escaped (backslash/quote):\n%s", script)
	}
	// The real command runs verbatim on its own line.
	if !strings.Contains(script, "\n"+`grep "a\b" .`+"\n") {
		t.Fatalf("command body not preserved verbatim:\n%s", script)
	}
}

// --- start-grace clamp (pure) ---

func TestClampStartDeadline(t *testing.T) {
	grace := DefaultStartGrace
	cases := []struct {
		name    string
		timeout time.Duration
		want    time.Duration
	}{
		{"zero timeout uses full grace", 0, grace},
		{"timeout above grace clamps to grace", grace + time.Minute, grace},
		{"timeout below grace uses the shorter timeout", time.Second, time.Second},
		{"timeout equal to grace uses grace", grace, grace},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := clampStartDeadline(grace, c.timeout); got != c.want {
				t.Fatalf("clampStartDeadline(%v,%v) = %v, want %v", grace, c.timeout, got, c.want)
			}
		})
	}
}

// --- nonce-scoped run-result parsing ---

func TestRunResultExitNonceMatching(t *testing.T) {
	mock := &tmux.MockRunner{}
	pane := "%9"
	set := func(v string) {
		mock.PaneOptions = map[string]string{pane + "\x00" + tabs.OptRunResult: v}
	}

	set("abc123:0")
	if code, ok := RunResultExit(mock, pane, "abc123"); !ok || code != 0 {
		t.Fatalf("matching nonce exit 0: code=%d ok=%v", code, ok)
	}
	set("abc123:7")
	if code, ok := RunResultExit(mock, pane, "abc123"); !ok || code != 7 {
		t.Fatalf("matching nonce exit 7: code=%d ok=%v", code, ok)
	}
	set("deadbeef:0")
	if _, ok := RunResultExit(mock, pane, "abc123"); ok {
		t.Fatal("wrong nonce must not match")
	}
	set("abc123:notint")
	if _, ok := RunResultExit(mock, pane, "abc123"); ok {
		t.Fatal("malformed exit code must not match")
	}
	set("")
	if _, ok := RunResultExit(mock, pane, "abc123"); ok {
		t.Fatal("empty run result must not match")
	}
}

func TestLifecycleStartedDetectsConsumedNonce(t *testing.T) {
	mock := &tmux.MockRunner{}
	pane := "%9"
	mock.PaneOptions = map[string]string{pane + "\x00" + tabs.OptNextRunID: "abc123"}
	if LifecycleStarted(mock, pane, "abc123") {
		t.Fatal("staged nonce still present -> lifecycle not started yet")
	}
	mock.PaneOptions[pane+"\x00"+tabs.OptNextRunID] = ""
	if !LifecycleStarted(mock, pane, "abc123") {
		t.Fatal("nonce cleared/consumed -> lifecycle started")
	}
}

// --- WaitResult interrupt (ctx cancellation) ---

func TestWaitResultInterrupt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already-cancelled ctx: the select must pick ctx.Done immediately
	e := Executor{Runner: &tmux.MockRunner{}, Stdout: io.Discard, Stderr: io.Discard, tick: time.Millisecond}
	err := e.WaitResult(ctx, "sess:tab", "%1", "nonce", 5*time.Second, 50)
	if err == nil || err.Error() != "interrupted" {
		t.Fatalf("cancelled ctx should return interrupted, got %v", err)
	}
}

// --- Follow growth + reset ---

type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func TestFollowGrowthAndReset(t *testing.T) {
	// Frames: grow to a,b then c; shrink (pane cleared) to x; grow to x,y. The
	// shrink resets the high-water mark so "x" is never reprinted, and only the
	// newly-appended "y" prints afterward.
	frames := []string{"a\nb\n", "a\nb\nc\n", "x\n", "x\ny\n"}
	served := make(chan struct{})
	var mu sync.Mutex
	i := 0
	mock := &tmux.MockRunner{}
	mock.CapturePaneFunc = func(string, int) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		idx := i
		if idx >= len(frames) {
			idx = len(frames) - 1
		}
		if i < len(frames) {
			i++
			if i == len(frames) {
				close(served)
			}
		}
		return frames[idx], nil
	}

	var buf syncBuffer
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	e := Executor{Runner: mock, Stdout: &buf, Stderr: io.Discard, tick: time.Millisecond}
	go func() { done <- e.Follow(ctx, "target", 50) }()

	select {
	case <-served:
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("frames were not all served")
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Follow returned error: %v", err)
	}

	got := buf.String()
	want := "a\nb\nc\ny\n"
	if got != want {
		t.Fatalf("follow output = %q, want %q", got, want)
	}
}
