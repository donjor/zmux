package cli

import (
	"bytes"
	"strings"
	"testing"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/donjor/zmux/internal/workspace"
)

// testVersion is the placeholder build version threaded through NewRootCmd in
// tests (real value is injected via -ldflags in production builds).
const testVersion = "test"

// newDefaultTestApp creates a minimal App for command-level tests, backed by an
// in-memory FS so tests stay hermetic (no real home/disk access).
func newDefaultTestApp() *apppkg.App {
	mock := tmux.NewMockRunner()
	fs := newMemFS("/home/user")
	return &apppkg.App{
		FS:             fs,
		Runner:         mock,
		WorkspaceStore: workspace.NewStore(fs),
		Overmind:       noopOvermind{},
	}
}

func TestCompletionBashProducesOutput(t *testing.T) {
	rootCmd := NewRootCmd(newDefaultTestApp(), testVersion)
	var buf bytes.Buffer
	if err := rootCmd.GenBashCompletion(&buf); err != nil {
		t.Fatalf("GenBashCompletion failed: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("expected non-empty bash completion output")
	}
}

func TestCompletionZshProducesOutput(t *testing.T) {
	rootCmd := NewRootCmd(newDefaultTestApp(), testVersion)
	var buf bytes.Buffer
	if err := rootCmd.GenZshCompletion(&buf); err != nil {
		t.Fatalf("GenZshCompletion failed: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("expected non-empty zsh completion output")
	}
}

func TestCompletionFishProducesOutput(t *testing.T) {
	rootCmd := NewRootCmd(newDefaultTestApp(), testVersion)
	var buf bytes.Buffer
	if err := rootCmd.GenFishCompletion(&buf, true); err != nil {
		t.Fatalf("GenFishCompletion failed: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("expected non-empty fish completion output")
	}
}

func TestStatusExitsZero(t *testing.T) {
	rootCmd := NewRootCmd(newDefaultTestApp(), testVersion)
	rootCmd.SetArgs([]string{"status"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("status command failed: %v", err)
	}
}

func TestHelpExitsZero(t *testing.T) {
	rootCmd := NewRootCmd(newDefaultTestApp(), testVersion)
	rootCmd.SetArgs([]string{"help"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("help command failed: %v", err)
	}
}

func TestVersionExitsZero(t *testing.T) {
	rootCmd := NewRootCmd(newDefaultTestApp(), testVersion)
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"version"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("version command failed: %v", err)
	}
	// Guards the version-threading wired through NewRootCmd → newVersionCmd.
	if !strings.Contains(buf.String(), testVersion) {
		t.Errorf("version output %q does not contain injected version %q", buf.String(), testVersion)
	}
}
