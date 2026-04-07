package palette

import (
	"os"
	"time"
)

// fakeFS is a minimal in-memory config.FS implementation for palette tests.
// Most tests only need Read/Write/Stat/UserHomeDir; Glob and MkdirAll are
// stubbed no-ops. It's declared in one place and reused across
// providers_test.go and executor_test.go.
type fakeFS struct {
	files map[string][]byte
	home  string
}

func newFakeFS(home string) *fakeFS {
	return &fakeFS{files: make(map[string][]byte), home: home}
}

func (f *fakeFS) ReadFile(path string) ([]byte, error) {
	if f.files == nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}
	}
	data, ok := f.files[path]
	if !ok {
		return nil, &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}
	}
	return data, nil
}

func (f *fakeFS) WriteFile(path string, data []byte, _ os.FileMode) error {
	if f.files == nil {
		f.files = make(map[string][]byte)
	}
	f.files[path] = data
	return nil
}

func (f *fakeFS) MkdirAll(_ string, _ os.FileMode) error { return nil }

func (f *fakeFS) Stat(path string) (os.FileInfo, error) {
	if f.files == nil {
		return nil, &os.PathError{Op: "stat", Path: path, Err: os.ErrNotExist}
	}
	if _, ok := f.files[path]; ok {
		return fakeInfo{name: path}, nil
	}
	return nil, &os.PathError{Op: "stat", Path: path, Err: os.ErrNotExist}
}

func (f *fakeFS) UserHomeDir() (string, error) { return f.home, nil }

func (f *fakeFS) Glob(_ string) ([]string, error) { return nil, nil }

type fakeInfo struct{ name string }

func (i fakeInfo) Name() string       { return i.name }
func (i fakeInfo) Size() int64        { return 0 }
func (i fakeInfo) Mode() os.FileMode  { return 0o644 }
func (i fakeInfo) ModTime() time.Time { return time.Time{} }
func (i fakeInfo) IsDir() bool        { return false }
func (i fakeInfo) Sys() any           { return nil }
