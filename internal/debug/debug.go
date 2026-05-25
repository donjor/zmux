// Package debug provides opt-in debug logging for zmux.
// No output by default. Set ZMUX_DEBUG=1 to write structured logs
// to ~/.zmux/debug.log.
package debug

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/charmbracelet/log"
)

var (
	logger  *slog.Logger
	once    sync.Once
	enabled bool
)

func init() {
	enabled = os.Getenv("ZMUX_DEBUG") == "1"
}

// Enabled returns whether debug logging is active.
func Enabled() bool {
	return enabled
}

// logPath overrides the default ~/.zmux/debug.log location. Set once from main
// (SetLogPath) with the active profile's path so zzmux logs to ~/.zzmux/debug.log
// instead of sharing the live zmux log.
var logPath string

// SetLogPath sets the debug log file path. Call before any debug.Log/Error
// (i.e. once at startup). Empty keeps the default ~/.zmux/debug.log.
func SetLogPath(p string) { logPath = p }

func getLogger() *slog.Logger {
	once.Do(func() {
		if !enabled {
			logger = slog.New(newHandler(nopWriter{}))
			return
		}
		path := logPath
		if path == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				logger = slog.New(newHandler(os.Stderr))
				return
			}
			path = filepath.Join(home, ".zmux", "debug.log")
		}
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			logger = slog.New(newHandler(os.Stderr))
			return
		}
		logger = slog.New(newHandler(f))
	})
	return logger
}

// newHandler builds a charmbracelet/log handler (which implements slog.Handler)
// at debug level with timestamps, writing to w. Keeps the *slog.Logger seam so
// the debug.Log/Error call sites are unchanged.
func newHandler(w io.Writer) slog.Handler {
	return log.NewWithOptions(w, log.Options{
		Level:           log.DebugLevel,
		ReportTimestamp: true,
	})
}

// Log writes a debug-level message.
func Log(msg string, args ...any) {
	if !enabled {
		return
	}
	getLogger().Debug(msg, args...)
}

// Error writes an error-level message.
func Error(msg string, args ...any) {
	if !enabled {
		return
	}
	getLogger().Error(msg, args...)
}

type nopWriter struct{}

func (nopWriter) Write(p []byte) (int, error) { return len(p), nil }
