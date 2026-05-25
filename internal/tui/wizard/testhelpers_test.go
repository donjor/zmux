package wizard

import (
	"os"

	"github.com/donjor/zmux/internal/theme"
)

func newTestThemeResolver() *theme.Resolver {
	// Use bundled themes only (no user/iterm2 dirs).
	fs := &noopFS{}
	return theme.NewResolver(fs, "", "")
}

// noopFS satisfies config.FS but returns errors/nil for all file ops.
// This is fine since we only use bundled themes in tests.
type noopFS struct{}

func (noopFS) ReadFile(path string) ([]byte, error)                       { return nil, os.ErrNotExist }
func (noopFS) WriteFile(path string, data []byte, perm os.FileMode) error { return nil }
func (noopFS) MkdirAll(path string, perm os.FileMode) error               { return nil }
func (noopFS) Stat(path string) (os.FileInfo, error)                      { return nil, os.ErrNotExist }
func (noopFS) UserHomeDir() (string, error)                               { return "/tmp", nil }
func (noopFS) Glob(pattern string) ([]string, error)                      { return nil, nil }
