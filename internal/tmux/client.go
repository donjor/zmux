package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Client implements Runner by shelling out to the tmux binary.
type Client struct {
	bin string
}

// NewClient creates a Client that uses the tmux binary in PATH.
func NewClient() *Client {
	return &Client{bin: "tmux"}
}

// run executes a tmux command and returns its stdout.
func (c *Client) run(args ...string) (string, error) {
	cmd := exec.Command(c.bin, args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("tmux %s: %w (stderr: %s)",
				strings.Join(args, " "), err, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("tmux %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// runSilent executes a tmux command ignoring stdout.
func (c *Client) runSilent(args ...string) error {
	_, err := c.run(args...)
	return err
}

// IsInsideTmux returns true if we're running inside a tmux session.
func (c *Client) IsInsideTmux() bool {
	return os.Getenv("TMUX") != ""
}

// ServerRunning returns true if a tmux server is active.
func (c *Client) ServerRunning() bool {
	err := exec.Command(c.bin, "list-sessions").Run()
	return err == nil
}

// Version returns the tmux version string.
func (c *Client) Version() (string, error) {
	out, err := c.run("-V")
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(out, "tmux "), nil
}

// ListSessions lists all tmux sessions.
func (c *Client) ListSessions() ([]Session, error) {
	out, err := c.run("list-sessions", "-F",
		"#{session_name}\t#{session_windows}\t#{session_attached}\t#{session_activity}\t#{session_path}")
	if err != nil {
		return nil, err
	}
	return parseSessions(out)
}

// HasSession returns true if a session with the given name exists.
func (c *Client) HasSession(name string) bool {
	err := exec.Command(c.bin, "has-session", "-t", name).Run()
	return err == nil
}

// NewSession creates a new detached session.
func (c *Client) NewSession(name, dir string) error {
	return c.runSilent("new-session", "-d", "-s", name, "-c", dir)
}

// KillSession kills a session by name.
func (c *Client) KillSession(name string) error {
	return c.runSilent("kill-session", "-t", name)
}

// AttachSession attaches to a session, taking over the terminal.
func (c *Client) AttachSession(name string) error {
	cmd := exec.Command(c.bin, "attach-session", "-t", name)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// SwitchClient switches the current client to a different session.
func (c *Client) SwitchClient(target string) error {
	return c.runSilent("switch-client", "-t", target)
}

// RenameSession renames a session.
func (c *Client) RenameSession(old, new string) error {
	return c.runSilent("rename-session", "-t", old, new)
}

// ListWindows lists all windows in a session.
func (c *Client) ListWindows(session string) ([]Window, error) {
	out, err := c.run("list-windows", "-t", session, "-F",
		"#{window_index}\t#{window_name}\t#{window_active}\t#{pane_current_path}")
	if err != nil {
		return nil, err
	}
	return parseWindows(out)
}

// NewWindow creates a new window in a session.
func (c *Client) NewWindow(session, name, dir string) error {
	return c.runSilent("new-window", "-t", session, "-n", name, "-c", dir)
}

// KillWindow kills a window by session and index.
func (c *Client) KillWindow(session string, index int) error {
	target := fmt.Sprintf("%s:%d", session, index)
	return c.runSilent("kill-window", "-t", target)
}

// RenameWindow renames a window.
func (c *Client) RenameWindow(session, old, new string) error {
	target := fmt.Sprintf("%s:%s", session, old)
	return c.runSilent("rename-window", "-t", target, new)
}

// SelectWindow selects a window by session and index.
func (c *Client) SelectWindow(session string, index int) error {
	target := fmt.Sprintf("%s:%d", session, index)
	return c.runSilent("select-window", "-t", target)
}

// MoveWindow moves a window from source to destination.
func (c *Client) MoveWindow(srcSession, dstSession string) error {
	return c.runSilent("move-window", "-s", srcSession, "-t", dstSession)
}

// SendKeys sends keys to a target pane/window.
func (c *Client) SendKeys(target string, keys ...string) error {
	args := append([]string{"send-keys", "-t", target}, keys...)
	return c.runSilent(args...)
}

// DisplayMessage runs display-message and returns the formatted output.
func (c *Client) DisplayMessage(target, format string) (string, error) {
	return c.run("display-message", "-t", target, "-p", format)
}

// CapturePane captures pane content.
func (c *Client) CapturePane(target string, lines int) (string, error) {
	return c.run("capture-pane", "-t", target, "-p", "-S", strconv.Itoa(-lines))
}

// SetOption sets a tmux option. scope is e.g. "-g", "-s", "-w", etc.
func (c *Client) SetOption(scope, key, value string) error {
	if scope != "" {
		return c.runSilent("set-option", scope, key, value)
	}
	return c.runSilent("set-option", key, value)
}

// SetEnvironment sets a global environment variable in tmux.
func (c *Client) SetEnvironment(key, value string) error {
	return c.runSilent("set-environment", "-g", key, value)
}

// SourceFile sources a tmux config file.
func (c *Client) SourceFile(path string) error {
	return c.runSilent("source-file", path)
}

// DisplayPopup displays a tmux popup.
func (c *Client) DisplayPopup(args ...string) error {
	fullArgs := append([]string{"display-popup"}, args...)
	return c.runSilent(fullArgs...)
}
