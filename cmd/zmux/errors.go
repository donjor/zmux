package main

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/donjor/zmux/internal/theme"
)

// Exit codes for zmux. Using a central place ensures consistency.
const (
	ExitOK          = 0
	ExitGeneral     = 1
	ExitUsage       = 2
	ExitConfig      = 3
	ExitDependency  = 4
	ExitThemeNotFound = 5
)

// formatError inspects err and returns a user-friendly message with
// actionable suggestions where possible.
func formatError(err error) string {
	if err == nil {
		return ""
	}

	msg := err.Error()

	// tmux not found — suggest installation.
	var pathErr *exec.Error
	if errors.As(err, &pathErr) {
		if pathErr.Name == "tmux" {
			return "tmux is not installed.\n\n" +
				"Install it with your package manager:\n" +
				"  macOS:   brew install tmux\n" +
				"  Ubuntu:  sudo apt install tmux\n" +
				"  Fedora:  sudo dnf install tmux\n" +
				"  Arch:    sudo pacman -S tmux"
		}
	}

	// tmux not found via exec.ErrNotFound wrapped in other errors.
	if errors.Is(err, exec.ErrNotFound) && strings.Contains(msg, "tmux") {
		return "tmux is not installed.\n\n" +
			"Install it with your package manager:\n" +
			"  macOS:   brew install tmux\n" +
			"  Ubuntu:  sudo apt install tmux\n" +
			"  Fedora:  sudo dnf install tmux\n" +
			"  Arch:    sudo pacman -S tmux"
	}

	// Config errors — suggest zmux init.
	if strings.Contains(msg, "read config") ||
		strings.Contains(msg, "parse config") ||
		(strings.Contains(msg, "config") && strings.Contains(msg, "no such file")) {
		return fmt.Sprintf("%s\n\nRun 'zmux init' to create a new config file.", msg)
	}

	// Theme not found — list available themes.
	if strings.Contains(msg, "not found") && strings.Contains(msg, "theme") {
		available := availableThemeNames()
		if len(available) > 0 {
			return fmt.Sprintf("%s\n\nAvailable themes:\n  %s",
				msg, strings.Join(available, "\n  "))
		}
	}

	return msg
}

// exitCodeForError maps an error to an appropriate exit code.
func exitCodeForError(err error) int {
	if err == nil {
		return ExitOK
	}

	msg := err.Error()

	var pathErr *exec.Error
	if errors.As(err, &pathErr) || errors.Is(err, exec.ErrNotFound) {
		return ExitDependency
	}

	if strings.Contains(msg, "not found") && strings.Contains(msg, "theme") {
		return ExitThemeNotFound
	}

	if strings.Contains(msg, "config") {
		return ExitConfig
	}

	if strings.Contains(msg, "usage") || strings.Contains(msg, "unknown command") {
		return ExitUsage
	}

	return ExitGeneral
}

// availableThemeNames returns bundled theme names for error messages.
// It silently returns nil on any failure since this is only used
// to enrich error output.
func availableThemeNames() []string {
	resolver := theme.NewResolver(nil, "", "")
	themes := resolver.List()
	names := make([]string, 0, len(themes))
	for _, ti := range themes {
		names = append(names, ti.Name)
	}
	return names
}
