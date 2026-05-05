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
func (RealFS) WriteFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}
func (RealFS) MkdirAll(path string, perm os.FileMode) error { return os.MkdirAll(path, perm) }
func (RealFS) Stat(path string) (os.FileInfo, error)        { return os.Stat(path) }
func (RealFS) UserHomeDir() (string, error)                 { return os.UserHomeDir() }
func (RealFS) Glob(pattern string) ([]string, error)        { return filepath.Glob(pattern) }
