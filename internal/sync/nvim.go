package sync

import (
	"fmt"
	"os/exec"
	"strings"
)

// CmdRunner abstracts command execution for testability.
type CmdRunner interface {
	Run(name string, args ...string) (string, error)
}

// RealCmdRunner executes commands via os/exec.
type RealCmdRunner struct{}

// Run executes a command and returns its combined output.
func (RealCmdRunner) Run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// NvimTarget reads the current colorscheme from Neovim.
type NvimTarget struct {
	runner CmdRunner
}

// NewNvimTarget creates an NvimTarget with the given command runner.
func NewNvimTarget(runner CmdRunner) *NvimTarget {
	return &NvimTarget{runner: runner}
}

// Name returns "nvim".
func (n *NvimTarget) Name() string {
	return "nvim"
}

// Pull queries Neovim for the current colorscheme name and performs a
// best-effort mapping to iterm2/ghostty theme names.
func (n *NvimTarget) Pull() (string, error) {
	output, err := n.runner.Run(
		"nvim",
		"--headless",
		"+lua io.write(vim.g.colors_name or '')",
		"+qa",
	)
	if err != nil {
		return "", fmt.Errorf("run nvim: %w", err)
	}

	colorscheme := strings.TrimSpace(output)
	if colorscheme == "" {
		return "", fmt.Errorf("nvim returned empty colorscheme")
	}

	return normalizeNvimTheme(colorscheme), nil
}

// normalizeNvimTheme maps common Neovim colorscheme names to
// iterm2/ghostty theme names.
func normalizeNvimTheme(name string) string {
	// Try known mappings first.
	switch {
	case strings.HasPrefix(name, "tokyonight"):
		return "tokyonight"
	case strings.HasPrefix(name, "catppuccin"):
		return "catppuccin-mocha"
	case strings.HasPrefix(name, "gruvbox"):
		return "gruvbox-dark"
	case strings.HasPrefix(name, "kanagawa"):
		return "kanagawa-dragon"
	case strings.HasPrefix(name, "rose-pine"):
		return "rose-pine"
	case strings.HasPrefix(name, "material"):
		return "material-darker"
	}

	// Return the name as-is for direct matches.
	return name
}
