// Package config handles TOML configuration loading, defaults, and validation.
package config

import (
	"os"
	"path/filepath"
)

// FS abstracts filesystem operations for testability.
type FS interface {
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
	Stat(path string) (os.FileInfo, error)
	UserHomeDir() (string, error)
	Glob(pattern string) ([]string, error)
}

// RealFS implements FS using the real filesystem.
type RealFS struct{}

func (RealFS) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }

// WriteFile writes atomically (temp file + rename in the target dir) so a
// concurrent reader never sees a torn file — workspaces.toml in particular is
// a multi-process read-modify-write hotspot (CLI + bar hooks + dashboard).
// A pre-existing symlink target is resolved first so rename replaces the
// destination file, not the link (e.g. a dotfiles-managed ~/.tmux.conf).
func (RealFS) WriteFile(path string, data []byte, perm os.FileMode) error {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".zmux-write-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
func (RealFS) MkdirAll(path string, perm os.FileMode) error { return os.MkdirAll(path, perm) }
func (RealFS) Stat(path string) (os.FileInfo, error)        { return os.Stat(path) }
func (RealFS) UserHomeDir() (string, error)                 { return os.UserHomeDir() }
func (RealFS) Glob(pattern string) ([]string, error)        { return filepath.Glob(pattern) }
