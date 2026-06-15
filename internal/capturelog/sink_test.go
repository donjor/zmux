package capturelog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donjor/zmux/internal/config"
)

func newTestSink(t *testing.T, maxBytes int, strip bool) (*Sink, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pane.log")
	return New(&config.RealFS{}, path, maxBytes, strip), path
}

func readLog(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	return string(b)
}

func TestSinkBoundsToMaxBytes(t *testing.T) {
	s, path := newTestSink(t, 64, false)
	for i := 0; i < 200; i++ {
		if _, err := s.Write([]byte("0123456789\n")); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	got := readLog(t, path)
	if len(got) > 64 {
		t.Fatalf("log %d bytes exceeds cap 64", len(got))
	}
	// The most recent output must survive; the oldest must be gone.
	if !strings.HasSuffix(got, "0123456789\n") {
		t.Fatalf("recent output not retained: %q", got)
	}
}

func TestSinkTrimsAtLineBoundary(t *testing.T) {
	s, path := newTestSink(t, 20, false)
	if _, err := s.Write([]byte("aaaa\nbbbb\ncccc\ndddd\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Exceed the cap so a trim happens; head should begin at a line start.
	if _, err := s.Write([]byte("eeee\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	s.Close()
	got := readLog(t, path)
	if len(got) > 20 {
		t.Fatalf("log %d bytes exceeds cap 20", len(got))
	}
	for _, line := range strings.Split(strings.TrimRight(got, "\n"), "\n") {
		if line != "" && len(line) != 4 {
			t.Fatalf("head not trimmed to a clean line boundary: %q", got)
		}
	}
}

func TestSinkKeepsWholeLineWhenCutOnBoundary(t *testing.T) {
	// 4 five-byte lines = 20 bytes; cap 10 cuts at index 10, which is exactly a
	// line boundary (the byte before is '\n'). The retained window already
	// starts on a whole line, so "cccc" must survive — advancing to the next
	// newline would wrongly drop a complete line.
	s, path := newTestSink(t, 10, false)
	if _, err := s.Write([]byte("aaaa\nbbbb\ncccc\ndddd\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	s.Close()
	if got := readLog(t, path); got != "cccc\ndddd\n" {
		t.Fatalf("boundary-aligned cut dropped a whole line: got %q, want %q", got, "cccc\ndddd\n")
	}
}

func TestSinkRawPreservesANSI(t *testing.T) {
	s, path := newTestSink(t, 0, false)
	if _, err := s.Write([]byte("\x1b[31mred\x1b[0m\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	s.Close()
	if got := readLog(t, path); got != "\x1b[31mred\x1b[0m\n" {
		t.Fatalf("raw sink altered bytes: %q", got)
	}
}

func TestSinkStripsANSI(t *testing.T) {
	s, path := newTestSink(t, 0, true)
	if _, err := s.Write([]byte("\x1b[31mred\x1b[0m\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	s.Close()
	if got := readLog(t, path); got != "red\n" {
		t.Fatalf("stripped sink = %q, want %q", got, "red\n")
	}
}

func TestSinkWriteReportsFullLength(t *testing.T) {
	s, _ := newTestSink(t, 0, true)
	in := []byte("\x1b[31mred\x1b[0m") // stripping drops bytes
	n, err := s.Write(in)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if n != len(in) {
		t.Fatalf("Write reported %d, want full input %d (io.Copy needs no short write)", n, len(in))
	}
}
