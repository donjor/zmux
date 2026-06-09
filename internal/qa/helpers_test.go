package qa

import (
	"os"
	"path/filepath"
	"time"
)

// memFS is an in-memory config.FS with a working Glob (Discover needs it).
type memFS struct {
	files map[string][]byte
}

func newMemFS() *memFS {
	return &memFS{files: make(map[string][]byte)}
}

func (m *memFS) ReadFile(path string) ([]byte, error) {
	data, ok := m.files[path]
	if !ok {
		return nil, &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}
	}
	return data, nil
}

func (m *memFS) WriteFile(path string, data []byte, _ os.FileMode) error {
	m.files[path] = data
	return nil
}

func (m *memFS) MkdirAll(_ string, _ os.FileMode) error { return nil }

func (m *memFS) Stat(path string) (os.FileInfo, error) {
	if _, ok := m.files[path]; ok {
		return fakeFileInfo{name: path}, nil
	}
	return nil, &os.PathError{Op: "stat", Path: path, Err: os.ErrNotExist}
}

func (m *memFS) UserHomeDir() (string, error) { return "/home/test", nil }

func (m *memFS) Glob(pattern string) ([]string, error) {
	var matches []string
	for path := range m.files {
		ok, err := filepath.Match(pattern, path)
		if err != nil {
			return nil, err
		}
		if ok {
			matches = append(matches, path)
		}
	}
	return matches, nil
}

type fakeFileInfo struct{ name string }

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() os.FileMode  { return 0o644 }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return false }
func (f fakeFileInfo) Sys() any           { return nil }

// memStateFS is an in-memory StateFS.
type memStateFS struct {
	files  map[string][]byte
	writes int
}

func newMemStateFS() *memStateFS {
	return &memStateFS{files: make(map[string][]byte)}
}

func (m *memStateFS) ReadFile(path string) ([]byte, error) {
	data, ok := m.files[path]
	if !ok {
		return nil, &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}
	}
	return data, nil
}

func (m *memStateFS) WriteFileAtomic(path string, data []byte, _ os.FileMode) error {
	m.files[path] = data
	m.writes++
	return nil
}

func (m *memStateFS) MkdirAll(_ string, _ os.FileMode) error { return nil }
