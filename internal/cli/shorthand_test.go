package cli

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/workspace"
)

// memFS is a minimal in-memory config.FS for CLI tests.
type memFS struct {
	files map[string][]byte
	home  string
}

func newMemFS(home string) *memFS {
	return &memFS{files: make(map[string][]byte), home: home}
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
		return fakeInfo{name: path}, nil
	}
	return nil, &os.PathError{Op: "stat", Path: path, Err: os.ErrNotExist}
}
func (m *memFS) UserHomeDir() (string, error) { return m.home, nil }
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

type fakeInfo struct{ name string }

func (f fakeInfo) Name() string       { return f.name }
func (f fakeInfo) Size() int64        { return 0 }
func (f fakeInfo) Mode() os.FileMode  { return 0o644 }
func (f fakeInfo) ModTime() time.Time { return time.Time{} }
func (f fakeInfo) IsDir() bool        { return false }
func (f fakeInfo) Sys() any           { return nil }

func withMockAppAndStore(t *testing.T) (*apppkg.App, *tmux.MockRunner, *workspace.Store) {
	t.Helper()

	fs := newMemFS("/home/user")
	mock := tmux.NewMockRunner()
	mock.InsideTmux = false
	mock.ServerUp = true

	store := workspace.NewStore(fs)
	_ = store.CreateWorkspace("myapp", "/home/user/myapp")
	_ = store.AddSession("myapp", "main")
	_ = store.CreateWorkspace("empty", "/home/user/empty")

	mock.Sessions = []tmux.Session{{Name: "main", Activity: time.Now()}}

	a := &apppkg.App{
		FS:             fs,
		Runner:         mock,
		WorkspaceStore: store,
	}
	return a, mock, store
}

func TestInvalidTopLevelCommandReturnsUsageError(t *testing.T) {
	a, _, _ := withMockAppAndStore(t)
	rootCmd := NewRootCmd(a, testVersion)
	rootCmd.SetArgs([]string{"reload"})

	err := rootCmd.Execute()
	if !errors.Is(err, errInvalidCommand) {
		t.Fatalf("expected invalid command error, got %v", err)
	}
	if got := exitCodeForError(err); got != ExitUsage {
		t.Fatalf("expected usage exit code, got %d", got)
	}
}

func TestRunInvalidTopLevelCommandPrintsHelp(t *testing.T) {
	a, _, _ := withMockAppAndStore(t)
	code, stdout, stderr := captureRunOutput(t, []string{"zmux", "reload"}, func() int {
		return Run(a, testVersion)
	})

	if code != ExitUsage {
		t.Fatalf("expected usage exit code, got %d", code)
	}
	if strings.TrimSpace(stderr) != "invalid command" {
		t.Fatalf("expected invalid command on stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Session Management") || !strings.Contains(stdout, "zmux open <ws> [session]") {
		t.Fatalf("expected help on stdout, got %q", stdout)
	}
	if strings.Contains(stdout+stderr, "shorthand was removed") {
		t.Fatalf("old shorthand guidance leaked into output:\nstdout=%q\nstderr=%q", stdout, stderr)
	}
}

func captureRunOutput(t *testing.T, argv []string, run func() int) (int, string, string) {
	t.Helper()

	oldArgs := os.Args
	oldStdout := os.Stdout
	oldStderr := os.Stderr

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}

	os.Args = argv
	os.Stdout = stdoutW
	os.Stderr = stderrW

	code := run()

	_ = stdoutW.Close()
	_ = stderrW.Close()
	os.Args = oldArgs
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	stdout, err := io.ReadAll(stdoutR)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	stderr, err := io.ReadAll(stderrR)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}

	return code, string(stdout), string(stderr)
}
