package cli

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestFormatErrorNil(t *testing.T) {
	if got := formatError(nil); got != "" {
		t.Errorf("expected empty string for nil error, got: %q", got)
	}
}

func TestFormatErrorTmuxNotFound(t *testing.T) {
	err := &exec.Error{Name: "tmux", Err: exec.ErrNotFound}
	msg := formatError(err)
	if !strings.Contains(msg, "not installed") {
		t.Errorf("expected 'not installed' message, got: %q", msg)
	}
	if !strings.Contains(msg, "brew install tmux") {
		t.Errorf("expected macOS install hint, got: %q", msg)
	}
	if !strings.Contains(msg, "apt install tmux") {
		t.Errorf("expected Ubuntu install hint, got: %q", msg)
	}
}

func TestFormatErrorThemeNotFound(t *testing.T) {
	err := errors.New("theme \"nonexistent\" not found")
	msg := formatError(err)
	if !strings.Contains(msg, "not found") {
		t.Errorf("expected 'not found' in message, got: %q", msg)
	}
	// Should list available themes.
	if !strings.Contains(msg, "Available themes") {
		t.Errorf("expected available themes listing, got: %q", msg)
	}
}

func TestFormatErrorGeneric(t *testing.T) {
	err := errors.New("something went wrong")
	msg := formatError(err)
	if msg != "something went wrong" {
		t.Errorf("expected passthrough message, got: %q", msg)
	}
}

func TestExitCodeForErrorNil(t *testing.T) {
	if got := exitCodeForError(nil); got != ExitOK {
		t.Errorf("expected ExitOK for nil, got: %d", got)
	}
}

func TestExitCodeForErrorDependency(t *testing.T) {
	err := &exec.Error{Name: "tmux", Err: exec.ErrNotFound}
	if got := exitCodeForError(err); got != ExitDependency {
		t.Errorf("expected ExitDependency, got: %d", got)
	}
}

func TestExitCodeForErrorTheme(t *testing.T) {
	err := errors.New("theme \"x\" not found")
	if got := exitCodeForError(err); got != ExitThemeNotFound {
		t.Errorf("expected ExitThemeNotFound, got: %d", got)
	}
}

func TestExitCodeForErrorGeneral(t *testing.T) {
	err := errors.New("generic error")
	if got := exitCodeForError(err); got != ExitGeneral {
		t.Errorf("expected ExitGeneral, got: %d", got)
	}
}

func TestExitCodes(t *testing.T) {
	// Verify the exit code constants are distinct.
	codes := map[int]string{
		ExitOK:            "ExitOK",
		ExitGeneral:       "ExitGeneral",
		ExitUsage:         "ExitUsage",
		ExitConfig:        "ExitConfig",
		ExitDependency:    "ExitDependency",
		ExitThemeNotFound: "ExitThemeNotFound",
	}
	if len(codes) != 6 {
		t.Error("exit code constants should all be distinct")
	}
}
