// Package capturelog implements the bounded sink behind `zmux log`. A tmux
// pipe-pane feeds a pane's raw output to `zmux log-sink`, which writes it here.
//
// The sink keeps only the trailing maxBytes of (optionally ANSI-stripped)
// output in memory and rewrites the whole bounded buffer to disk on every
// flush, so the log file never exceeds maxBytes and no rotation artifacts
// accumulate. File I/O goes through config.FS (whole-file WriteFile), matching
// the codebase's interface boundary rather than streaming os calls.
package capturelog

import (
	"bytes"

	"github.com/donjor/zmux/internal/config"
)

// DefaultMaxBytes caps a log file when the caller passes maxBytes <= 0.
const DefaultMaxBytes = 1 << 20 // 1 MiB

// Sink is an io.WriteCloser that appends pane output to a byte-bounded log file.
// Feed it chunks via Write (e.g. io.Copy from the pipe) and Close to flush.
// It is not safe for concurrent use; a single pipe feeds a single sink.
type Sink struct {
	fs       config.FS
	path     string
	maxBytes int
	stripper *ansiStripper // nil when raw (--ansi) output is requested
	buf      []byte
}

// New builds a Sink writing to path. When stripANSI is true, escape/control
// sequences are removed for a readable plain-text log; otherwise the raw byte
// stream (including ANSI colour) is preserved. maxBytes <= 0 uses DefaultMaxBytes.
func New(fs config.FS, path string, maxBytes int, stripANSI bool) *Sink {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBytes
	}
	s := &Sink{fs: fs, path: path, maxBytes: maxBytes}
	if stripANSI {
		s.stripper = &ansiStripper{}
	}
	return s
}

// Write appends p (optionally stripped) to the bounded buffer and flushes to
// disk. It always reports len(p) consumed so io.Copy never sees a short write,
// even when stripping drops bytes.
func (s *Sink) Write(p []byte) (int, error) {
	n := len(p)
	if s.stripper != nil {
		p = s.stripper.feed(p)
	}
	s.buf = append(s.buf, p...)
	if len(s.buf) > s.maxBytes {
		cut := len(s.buf) - s.maxBytes
		trimmed := s.buf[cut:]
		// Start the retained window at a line boundary so the head of the log is
		// never a half line — but only when the cut split a line. If the byte
		// before the cut is itself a newline, the window already begins on a
		// whole line and advancing to the next newline would drop a complete
		// line of output.
		if cut > 0 && s.buf[cut-1] != '\n' {
			if i := bytes.IndexByte(trimmed, '\n'); i >= 0 && i+1 < len(trimmed) {
				trimmed = trimmed[i+1:]
			}
		}
		s.buf = append([]byte(nil), trimmed...) // copy off the old backing array
	}
	if err := s.flush(); err != nil {
		return n, err
	}
	return n, nil
}

// Close flushes the final buffer state. Any incomplete escape sequence still
// pending in the stripper is intentionally dropped.
func (s *Sink) Close() error {
	return s.flush()
}

func (s *Sink) flush() error {
	return s.fs.WriteFile(s.path, s.buf, 0o644)
}
