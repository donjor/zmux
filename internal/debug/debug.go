// Package debug provides opt-in debug logging for zmux.
// No output by default. Set ZMUX_DEBUG=1 to write structured logs
// to ~/.zmux/debug.log.
package debug

import (
	"log/slog"
	"os"
	"path/filepath"
	"sync"
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

func getLogger() *slog.Logger {
	once.Do(func() {
		if !enabled {
			logger = slog.New(slog.NewTextHandler(nopWriter{}, nil))
			return
		}
		home, err := os.UserHomeDir()
		if err != nil {
			logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
			return
		}
		dir := filepath.Join(home, ".zmux")
		_ = os.MkdirAll(dir, 0o755)
		f, err := os.OpenFile(filepath.Join(dir, "debug.log"),
			os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
			return
		}
		logger = slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{Level: slog.LevelDebug}))
	})
	return logger
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
