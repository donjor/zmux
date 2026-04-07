package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/tmux"
)

// Async message types carrying wizard command results.

// detectDepsMsg carries dependency detection results.
type detectDepsMsg struct {
	deps WizardDeps
}

// configWrittenMsg signals that config was written successfully.
type configWrittenMsg struct{}

// configWriteErrMsg signals that config writing failed.
type configWriteErrMsg struct {
	err error
}

// detectDeps detects system dependencies. Runs synchronously inside a
// tea.Cmd and returns a detectDepsMsg. Note that several of the checks
// use exec.Command / exec.LookPath directly rather than a mockable seam,
// so this function is intentionally best-effort and untestable.
func (m WizardModel) detectDeps() tea.Msg {
	var deps WizardDeps

	// Check tmux version.
	if out, err := exec.Command("tmux", "-V").Output(); err == nil {
		ver := strings.TrimSpace(string(out))
		ver = strings.TrimPrefix(ver, "tmux ")
		deps.TmuxVersion = ver
	}

	// Check clipboard.
	deps.ClipboardTool = tmux.DetectClipboard()

	// Check ghostty config.
	home, err := m.fs.UserHomeDir()
	if err == nil {
		xdgConfig := os.Getenv("XDG_CONFIG_HOME")
		if xdgConfig == "" {
			xdgConfig = filepath.Join(home, ".config")
		}
		ghosttyPath := filepath.Join(xdgConfig, "ghostty", "config")
		if _, err := m.fs.Stat(ghosttyPath); err == nil {
			deps.HasGhostty = true
			deps.GhosttyPath = ghosttyPath
		}
	}

	// Check nvim.
	if _, err := exec.LookPath("nvim"); err == nil {
		deps.HasNvim = true
	}

	return detectDepsMsg{deps: deps}
}

// writeConfig writes ~/.zmux.toml and ~/.tmux.conf based on the wizard
// selections, creating user directories along the way.
func (m WizardModel) writeConfig() tea.Msg {
	home, err := m.fs.UserHomeDir()
	if err != nil {
		return configWriteErrMsg{err: fmt.Errorf("get home dir: %w", err)}
	}

	// Create directories.
	dirs := []string{
		filepath.Join(home, ".zmux"),
		filepath.Join(home, ".zmux", "themes"),
		filepath.Join(home, ".zmux", "templates"),
	}
	for _, dir := range dirs {
		if err := m.fs.MkdirAll(dir, 0o755); err != nil {
			return configWriteErrMsg{err: fmt.Errorf("create dir %s: %w", dir, err)}
		}
	}

	// Build config.
	cfg := config.Config{
		Theme:  m.chosenTheme,
		Prefix: "C-Space",
		Bar: config.BarConfig{
			Preset: m.chosenPreset,
		},
		Sessions: config.SessionsConfig{
			AutoCleanupTmp: true,
		},
		Templates: config.TemplatesConfig{
			Paths: []string{"~/.zmux/templates"},
		},
		Sync: config.SyncConfig{
			Target:        m.chosenSync,
			GhosttyConfig: "auto",
		},
	}

	// Write config.
	cfgPath := filepath.Join(home, ".zmux.toml")
	if err := config.Save(m.fs, cfgPath, cfg); err != nil {
		return configWriteErrMsg{err: fmt.Errorf("save config: %w", err)}
	}

	// Generate and write tmux.conf.
	zmuxBin, _ := os.Executable()
	if zmuxBin == "" {
		zmuxBin = "zmux"
	}

	// Resolve theme palette for conf generation.
	t, err := m.resolver.Resolve(m.chosenTheme)
	if err == nil {
		palette := t.SemanticPalette()
		confContent := tmux.GenerateConf(&cfg, &palette, zmuxBin)
		confPath := filepath.Join(home, ".tmux.conf")
		if writeErr := tmux.WriteConf(m.fs, confPath, confContent); writeErr != nil {
			return configWriteErrMsg{err: fmt.Errorf("write tmux.conf: %w", writeErr)}
		}
	}

	return configWrittenMsg{}
}

// RestartCmd returns the command the user should run after init.
// Exported so the caller can echo it after the TUI exits.
func RestartCmd() string { return restartCmd() }

// restartCmd is the copy-to-clipboard target shown on the success
// screen. Sourced in one place so the copy matches the display.
func restartCmd() string {
	return "tmux source-file ~/.tmux.conf 2>/dev/null; exec $SHELL"
}

// copyToClipboard writes text to whichever clipboard tool is available
// on the current platform. Returns an error if no tool is found.
func copyToClipboard(text string) error {
	tools := []string{"wl-copy", "xclip", "pbcopy"}
	for _, tool := range tools {
		path, err := exec.LookPath(tool)
		if err != nil {
			continue
		}
		var cmd *exec.Cmd
		switch tool {
		case "xclip":
			cmd = exec.Command(path, "-selection", "clipboard")
		default:
			cmd = exec.Command(path)
		}
		cmd.Stdin = strings.NewReader(text)
		return cmd.Run()
	}
	return fmt.Errorf("no clipboard tool found")
}
