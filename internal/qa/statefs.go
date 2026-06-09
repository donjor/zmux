package qa

import (
	"os"
	"path/filepath"
)

// StateFS is the run-state side-effect seam. It exists apart from
// config.FS for one reason: scorecards are written by a human picker and
// an agent CLI concurrently, so writes must be atomic (temp + rename) —
// an operation config.FS deliberately doesn't carry.
type StateFS interface {
	ReadFile(path string) ([]byte, error)
	WriteFileAtomic(path string, data []byte, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
}

// RealStateFS implements StateFS on the real filesystem.
type RealStateFS struct{}

func (RealStateFS) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }

func (RealStateFS) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// WriteFileAtomic writes via a temp file in the target directory and
// renames it into place — readers never observe a torn scorecard.
func (RealStateFS) WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".qa-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after a successful rename

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
	return os.Rename(tmpName, path)
}
