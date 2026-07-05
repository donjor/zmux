package tmux

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Client implements Runner by shelling out to the tmux binary.
type Client struct {
	bin      string
	endpoint Endpoint
}

// NewClient creates a Client that uses the tmux binary in PATH
// with the default server endpoint.
func NewClient() *Client {
	return &Client{bin: "tmux"}
}

// NewClientFor creates a Client bound to a specific endpoint.
func NewClientFor(ep Endpoint) *Client {
	return &Client{bin: "tmux", endpoint: ep}
}

// Endpoint returns the endpoint this client is bound to.
func (c *Client) Endpoint() Endpoint {
	return c.endpoint
}

// buildArgs prepends the endpoint flags to any tmux command arguments.
func (c *Client) buildArgs(args ...string) []string {
	epArgs := c.endpoint.Args()
	if len(epArgs) == 0 {
		return args
	}
	return append(epArgs, args...)
}

// ErrCrossProfile marks a refused cross-profile tmux invocation, so degrade
// paths that tolerate "no server running" (e.g. ls printing "No sessions.")
// can re-surface this error instead of swallowing it into a misleading state.
var ErrCrossProfile = errors.New("cross-profile tmux access refused")

// ambientSocketMismatch reports a foreign $TMUX socket for a default-endpoint
// client. The default endpoint passes no -L flag, so tmux routes every command
// to the socket in $TMUX — running the live binary inside another profile's
// session (e.g. `zmux apply` in a zzmux pane) would silently read from and
// write themed bar options onto the wrong server. Refuse loudly instead.
// Reads are refused too: an answer from the wrong server is misinformation.
// Explicit -L/-S endpoints always win over $TMUX, so they need no check.
func (c *Client) ambientSocketMismatch() error {
	if c.endpoint.Mode != SocketDefault {
		return nil
	}
	tmuxEnv := os.Getenv("TMUX")
	if tmuxEnv == "" {
		return nil
	}
	if name := tmuxSocketName(tmuxEnv); name != "default" {
		return fmt.Errorf(
			"%w: this binary targets the default tmux server, but $TMUX points at socket %q — use that profile's binary (%s), or run from outside its session",
			ErrCrossProfile, name, name,
		)
	}
	return nil
}

// run executes a tmux command and returns its stdout.
func (c *Client) run(args ...string) (string, error) {
	if err := c.ambientSocketMismatch(); err != nil {
		return "", err
	}
	full := c.buildArgs(args...)
	cmd := exec.Command(c.bin, full...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("tmux %s: %w (stderr: %s)",
				strings.Join(full, " "), err, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("tmux %s: %w", strings.Join(full, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// runSilent executes a tmux command ignoring stdout.
func (c *Client) runSilent(args ...string) error {
	_, err := c.run(args...)
	return err
}

// IsInsideTmux reports whether we're running inside a tmux session that this
// client's endpoint actually owns. Every endpoint mode requires the $TMUX
// socket to match: a named/path endpoint (e.g. the zzmux profile's -L zzmux)
// compares against its socket, and the default endpoint requires the stock
// "default" socket — so the live binary inside a zzmux pane reports false
// instead of routing current-client work onto a server it doesn't own (the
// ambientSocketMismatch guard would refuse it anyway, but a true here would
// send commands down inside-tmux paths that mask that refusal with generic
// "no current session" errors).
func (c *Client) IsInsideTmux() bool {
	tmuxEnv := os.Getenv("TMUX")
	if tmuxEnv == "" {
		return false
	}
	switch c.endpoint.Mode {
	case SocketNamed:
		return tmuxSocketName(tmuxEnv) == c.endpoint.Value
	case SocketPath:
		return tmuxSocketName(tmuxEnv) == filepath.Base(c.endpoint.Value)
	default:
		return tmuxSocketName(tmuxEnv) == "default"
	}
}

// tmuxSocketName extracts the socket basename from $TMUX, which tmux formats as
// "<socket-path>,<pid>,<session-id>".
func tmuxSocketName(tmuxEnv string) string {
	sock := tmuxEnv
	if i := strings.IndexByte(sock, ','); i >= 0 {
		sock = sock[:i]
	}
	return filepath.Base(sock)
}

// ServerRunning returns true if a tmux server is active.
func (c *Client) ServerRunning() bool {
	if c.ambientSocketMismatch() != nil {
		return false
	}
	err := exec.Command(c.bin, c.buildArgs("list-sessions")...).Run()
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
		"#{session_name}\t#{session_windows}\t#{session_attached}\t#{session_activity}\t#{session_path}\t#{session_created}\t#{session_last_attached}\t#{session_group}\t#{@zmux_managed}\t#{@zmux_workspace}\t#{@zmux_session_label}\t#{@zmux_session_id}\t#{@zmux_clone}\t#{@zmux_pinned_view}\t#{@zmux_view_root}")
	if err != nil {
		return nil, err
	}
	return parseSessions(out)
}

// ListClients lists attached tmux clients and their current view.
func (c *Client) ListClients() ([]ClientInfo, error) {
	out, err := c.run("list-clients", "-F",
		"#{client_tty}\t#{client_session}\t#{session_id}\t#{session_group}\t#{window_id}\t#{window_index}\t#{window_name}\t#{pane_id}\t#{client_pid}\t#{client_control_mode}\t#{client_termname}\t#{client_termfeatures}\t#{client_flags}")
	if err != nil {
		return nil, err
	}
	return parseClients(out)
}

// HasSession returns true if a session with the given name exists.
func (c *Client) HasSession(name string) bool {
	if c.ambientSocketMismatch() != nil {
		return false
	}
	err := exec.Command(c.bin, c.buildArgs("has-session", "-t", exactSessionTarget(name))...).Run()
	return err == nil
}

// NewSession creates a new detached session.
func (c *Client) NewSession(name, dir string) error {
	return c.runSilent("new-session", "-d", "-s", name, "-c", dir)
}

// NewSessionWindow creates a detached session with its first window named
// `window` and returns that window's initial pane id (%N). Detached (-d) means
// no client attaches or switches, so worker/background session birth never
// steals the user's focus; naming the first window at creation avoids a
// leftover blank shell tab — the caller runs its command in this birth pane
// instead of a follow-up NewWindow.
func (c *Client) NewSessionWindow(session, window, dir string) (string, error) {
	return c.run("new-session", "-d", "-P", "-F", "#{pane_id}", "-s", session, "-n", window, "-c", dir)
}

// NewGroupedSession creates a grouped session linked to target.
// The new session shares windows with target but has an independent viewport.
func (c *Client) NewGroupedSession(target, name string) error {
	return c.runSilent("new-session", "-d", "-t", exactSessionTarget(target), "-s", name)
}

// KillSession kills a session by name.
func (c *Client) KillSession(name string) error {
	return c.runSilent("kill-session", "-t", exactSessionTarget(name))
}

// AttachSession attaches to a session, taking over the terminal.
func (c *Client) AttachSession(name string) error {
	if err := c.ambientSocketMismatch(); err != nil {
		return err
	}
	cmd := exec.Command(c.bin, c.buildArgs("attach-session", "-t", exactSessionTarget(name))...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// AttachSessionDetach attaches to a session, detaching any other clients first.
// This is "hijack mode" — steals the session from whoever has it.
func (c *Client) AttachSessionDetach(name string) error {
	if err := c.ambientSocketMismatch(); err != nil {
		return err
	}
	cmd := exec.Command(c.bin, c.buildArgs("attach-session", "-d", "-t", exactSessionTarget(name))...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// RefreshClient replaces an attached client with a freshly attached tmux client.
// This forces tmux to re-resolve terminal features without a manual detach/reattach.
func (c *Client) RefreshClient(targetClient, session string) error {
	if targetClient == "" {
		return fmt.Errorf("target client is required")
	}
	if session == "" {
		return fmt.Errorf("session is required")
	}
	attachArgs := c.buildArgs("-T", "RGB,extkeys", "attach-session", "-t", session)
	attachCmd := shellCommand(append([]string{c.bin}, attachArgs...))
	return c.runSilent("detach-client", "-t", targetClient, "-E", attachCmd)
}

// SwitchClient switches the current client to a different session.
func (c *Client) SwitchClient(target string) error {
	return c.runSilent("switch-client", "-t", exactSessionTarget(target))
}

// RenameSession renames a session.
func (c *Client) RenameSession(old, new string) error {
	return c.runSilent("rename-session", "-t", exactSessionTarget(old), new)
}

// ListWindows lists all windows in a session.
func (c *Client) ListWindows(session string) ([]Window, error) {
	out, err := c.run("list-windows", "-t", exactSessionTarget(session), "-F",
		"#{window_index}\t#{window_name}\t#{window_active}\t#{pane_current_path}\t#{@zmux_label}")
	if err != nil {
		return nil, err
	}
	return parseWindows(out)
}

// WindowOpt configures NewWindow.
type WindowOpt func(*windowOpts)

type windowOpts struct{ detached bool }

// Detached creates the window without switching the attached client to it
// (tmux `new-window -d`), so background work never steals the user's focus.
func Detached() WindowOpt { return func(o *windowOpts) { o.detached = true } }

// NewWindow creates a new window in a session and returns its initial pane
// id (%N) so callers can stamp pane-scoped identity without a follow-up
// lookup race. An empty name omits `-n` so tmux's automatic-rename can label
// the window from the running command.
func (c *Client) NewWindow(session, name, dir string, opts ...WindowOpt) (string, error) {
	var o windowOpts
	for _, fn := range opts {
		fn(&o)
	}
	// Target an explicit next-free index rather than the bare session. A bare
	// `-t <session>` target makes tmux resolve the new index relative to the
	// session's *current* window; on some tmux builds that is current-index+1,
	// which fails with "index N in use" when the active window isn't the last
	// one — i.e. whenever an agent lives in window 1 of a multi-window session.
	// max+1 always appends past every existing window, free of that collision
	// and identical across tmux versions. Falls back to the bare target if the
	// window list is unreadable.
	target := exactSessionTarget(session)
	if idx, err := c.nextWindowIndex(session); err == nil {
		target = fmt.Sprintf("%s:%d", exactSessionTarget(session), idx)
	}
	args := []string{"new-window", "-P", "-F", "#{pane_id}", "-t", target, "-c", dir}
	if o.detached {
		args = append(args, "-d")
	}
	if name != "" {
		args = append(args, "-n", name)
	}
	return c.run(args...)
}

func exactSessionTarget(name string) string {
	if name == "" || strings.HasPrefix(name, "=") {
		return name
	}
	return "=" + name
}

// nextWindowIndex returns one past the highest window index in session, the
// next slot a new window can claim without colliding with an existing one.
func (c *Client) nextWindowIndex(session string) (int, error) {
	wins, err := c.ListWindows(session)
	if err != nil {
		return 0, err
	}
	maxIdx := -1
	for _, w := range wins {
		if w.Index > maxIdx {
			maxIdx = w.Index
		}
	}
	if maxIdx < 0 {
		return 0, fmt.Errorf("session %q has no windows", session)
	}
	return maxIdx + 1, nil
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

// SwapWindow swaps two windows within a session.
func (c *Client) SwapWindow(session string, idx1, idx2 int) error {
	src := fmt.Sprintf("%s:%d", session, idx1)
	dst := fmt.Sprintf("%s:%d", session, idx2)
	return c.runSilent("swap-window", "-s", src, "-t", dst)
}

const paneListFormat = "#{session_name}\t#{pane_id}\t#{pane_index}\t#{pane_active}\t#{pane_current_command}\t#{pane_pid}\t#{pane_current_path}\t#{pane_width}\t#{pane_height}\t#{pane_title}\t#{window_index}\t#{window_name}"

// ListPanes lists all panes across all windows in a target session. Empty target uses tmux's current session.
func (c *Client) ListPanes(target string) ([]Pane, error) {
	args := []string{"list-panes"}
	if target != "" {
		args = append(args, "-t", exactSessionTarget(target))
	}
	args = append(args, "-s", "-F", paneListFormat)
	out, err := c.run(args...)
	if err != nil {
		return nil, err
	}
	return parsePanes(out)
}

// ListWindowPanes lists panes in a target window/pane. Empty target uses tmux's current window.
func (c *Client) ListWindowPanes(target string) ([]Pane, error) {
	args := []string{"list-panes"}
	if target != "" {
		args = append(args, "-t", target)
	}
	args = append(args, "-F", paneListFormat)
	out, err := c.run(args...)
	if err != nil {
		return nil, err
	}
	return parsePanes(out)
}

// ListAllPanes lists panes across all tmux sessions.
func (c *Client) ListAllPanes() ([]Pane, error) {
	out, err := c.run("list-panes", "-a", "-F", paneListFormat)
	if err != nil {
		return nil, err
	}
	return parsePanes(out)
}

// SplitWindow splits the target window/pane. direction is "-h" or "-v".
func (c *Client) SplitWindow(target, direction string) error {
	return c.runSilent("split-window", direction, "-t", target)
}

// SplitPane creates a pane and returns tmux's opaque pane id, e.g. %57.
func (c *Client) SplitPane(opts SplitPaneOptions) (string, error) {
	args, err := buildSplitPaneArgs(opts)
	if err != nil {
		return "", err
	}
	paneID, err := c.run(args...)
	if err != nil {
		return "", err
	}
	if opts.Title != "" {
		if err := c.runSilent("select-pane", "-t", paneID, "-T", opts.Title); err != nil {
			return "", err
		}
	}
	return paneID, nil
}

func buildSplitPaneArgs(opts SplitPaneOptions) ([]string, error) {
	args := []string{"split-window"}
	if opts.Detached {
		args = append(args, "-d")
	}
	args = append(args, "-P", "-F", "#{pane_id}")
	switch opts.Direction {
	case "", SplitRight:
		args = append(args, "-h")
	case SplitLeft:
		args = append(args, "-h", "-b")
	case SplitDown:
		args = append(args, "-v")
	case SplitUp:
		args = append(args, "-v", "-b")
	default:
		return nil, fmt.Errorf("unknown split direction %q", opts.Direction)
	}
	if opts.Size != "" {
		args = append(args, "-l", opts.Size)
	}
	if opts.CWD != "" {
		args = append(args, "-c", opts.CWD)
	}
	if opts.Target != "" {
		args = append(args, "-t", opts.Target)
	}
	if len(opts.Command) > 0 {
		args = append(args, shellCommand(opts.Command))
	}
	return args, nil
}

func shellCommand(argv []string) string {
	parts := make([]string, 0, len(argv))
	for _, arg := range argv {
		parts = append(parts, shellQuote(arg))
	}
	return strings.Join(parts, " ")
}

// ShellCommand joins argv into a single shell-safe command string, quoting each
// element only as needed. Exposed so CLI callers can faithfully reconstruct a
// command from post-`--` argv — a raw space-join would drop quoting and change
// the meaning of e.g. `bash -lc 'printf x; sleep 2'`.
func ShellCommand(argv []string) string { return shellCommand(argv) }

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if strings.IndexFunc(s, func(r rune) bool {
		//nolint:staticcheck // QF1001: negation-of-allowed-set ("not a safe char")
		// reads clearer than the De Morgan expansion into negated ranges.
		return !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || strings.ContainsRune("_+-=.,/:@%", r))
	}) == -1 {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// KillPane kills a pane by target.
func (c *Client) KillPane(target string) error {
	return c.runSilent("kill-pane", "-t", target)
}

// SelectPane focuses a pane by target.
func (c *Client) SelectPane(target string) error {
	return c.runSilent("select-pane", "-t", target)
}

// ResizePane sets a pane's width or height. Axis must be "width" or "height".
func (c *Client) ResizePane(target, axis, size string) error {
	flag := "-x"
	if axis == "height" {
		flag = "-y"
	}
	return c.runSilent("resize-pane", "-t", target, flag, size)
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

// ShowMessage flashes text on the current client's status line. -l prints the
// text literally: the messages carry user-controlled tab labels and window
// names, and without -l a label like "#(cmd)" would run a shell command and
// "#{...}" would expand as a tmux format.
func (c *Client) ShowMessage(text string) error {
	return c.runSilent("display-message", "-l", text)
}

// CapturePaneOptions configures a capture-pane invocation.
type CapturePaneOptions struct {
	Lines int  // history lines to include (captures from -Lines to the bottom)
	ANSI  bool // include escape sequences so colours/styling survive (-e)
	Join  bool // join wrapped lines into single logical lines (-J)
}

// CapturePane captures at most `lines` logical lines of pane content as plain
// text, counting from the bottom. Retained with stable plain semantics because
// run/watch depend on it; richer captures use CapturePaneOpts.
//
// tmux's `-S -N` start line is N lines of *history before* the visible pane, so
// `capture-pane -S -N` returns N history lines PLUS the full visible pane — more
// than N. Bounded callers (watch --lines, run follow) want N total, so the
// result is tail-trimmed to the last `lines` lines in Go. lines <= 0 is left
// untrimmed (capture the visible pane as tmux returns it).
func (c *Client) CapturePane(target string, lines int) (string, error) {
	out, err := c.run("capture-pane", "-t", target, "-p", "-S", strconv.Itoa(-lines))
	if err != nil {
		return out, err
	}
	return tailLines(out, lines), nil
}

// tailLines returns at most the last n lines of s. n <= 0 returns s unchanged.
// A single trailing newline is preserved (it does not count as a line), so the
// capture keeps its shape and successive captures stay byte-comparable for idle
// detection.
func tailLines(s string, n int) string {
	if n <= 0 || s == "" {
		return s
	}
	trailing := ""
	body := s
	if strings.HasSuffix(body, "\n") {
		trailing, body = "\n", body[:len(body)-1]
	}
	lines := strings.Split(body, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n") + trailing
}

// CapturePaneOpts captures pane content with options (ANSI escapes, line join).
func (c *Client) CapturePaneOpts(target string, opts CapturePaneOptions) (string, error) {
	args := []string{"capture-pane", "-t", target, "-p"}
	if opts.ANSI {
		args = append(args, "-e")
	}
	if opts.Join {
		args = append(args, "-J")
	}
	args = append(args, "-S", strconv.Itoa(-opts.Lines))
	return c.run(args...)
}

// PipePane opens or closes a tmux output pipe on target. A non-empty command is
// handed to tmux as a single shell string (run via /bin/sh -c) and streams the
// pane's raw output to that command's stdin until the pipe is closed or the
// pane dies; an empty command closes any pipe currently open on the target.
// Unlike capture-pane this is continuous and server-side, so it keeps recording
// with no client attached. State is queryable via #{pane_pipe}.
func (c *Client) PipePane(target, command string) error {
	if command == "" {
		return c.runSilent("pipe-pane", "-t", target)
	}
	return c.runSilent("pipe-pane", "-t", target, command)
}

// SetOption sets a tmux option. scope is e.g. "-g", "-s", "-w", etc.
func (c *Client) SetOption(scope, key, value string) error {
	if scope != "" {
		return c.runSilent("set-option", scope, key, value)
	}
	return c.runSilent("set-option", key, value)
}

// SetSessionOption sets a tmux option for a specific session target.
func (c *Client) SetSessionOption(target, key, value string) error {
	return c.runSilent("set-option", "-t", target, key, value)
}

// SetWindowOption sets a tmux window option for a specific window target.
func (c *Client) SetWindowOption(target, key, value string) error {
	args := []string{"set-option", "-w"}
	if target != "" {
		args = append(args, "-t", target)
	}
	args = append(args, key, value)
	return c.runSilent(args...)
}

// UnsetWindowOption unsets a tmux window option for a specific window target.
func (c *Client) UnsetWindowOption(target, key string) error {
	args := []string{"set-option", "-w", "-u"}
	if target != "" {
		args = append(args, "-t", target)
	}
	args = append(args, key)
	return c.runSilent(args...)
}

// SetPaneOption sets a tmux pane option for a specific pane target.
func (c *Client) SetPaneOption(target, key, value string) error {
	args := []string{"set-option", "-p"}
	if target != "" {
		args = append(args, "-t", target)
	}
	args = append(args, key, value)
	return c.runSilent(args...)
}

// UnsetPaneOption unsets a tmux pane option for a specific pane target.
func (c *Client) UnsetPaneOption(target, key string) error {
	args := []string{"set-option", "-p", "-u"}
	if target != "" {
		args = append(args, "-t", target)
	}
	args = append(args, key)
	return c.runSilent(args...)
}

// ShowWindowOption reads a window option scope-exactly (show-options -w),
// returning "" when unset (-q). Format reads (#{@opt}) merge scopes — a
// pane-target read can surface a window value and vice versa — so mirror
// validation and migration must use these instead.
func (c *Client) ShowWindowOption(target, key string) (string, error) {
	args := []string{"show-options", "-w", "-q", "-v"}
	if target != "" {
		args = append(args, "-t", target)
	}
	args = append(args, key)
	return c.run(args...)
}

// ShowPaneOption reads a pane option scope-exactly (show-options -p),
// returning "" when unset (-q). See ShowWindowOption for why format reads
// don't suffice.
func (c *Client) ShowPaneOption(target, key string) (string, error) {
	args := []string{"show-options", "-p", "-q", "-v"}
	if target != "" {
		args = append(args, "-t", target)
	}
	args = append(args, key)
	return c.run(args...)
}

// ShowGlobalOption reads a global option scope-exactly (show-options -gqv),
// returning "" when unset. Unlike DisplayMessage format expansion it needs no
// attached client, so it's safe from a run-shell hook (the reaper throttle).
func (c *Client) ShowGlobalOption(key string) (string, error) {
	return c.run("show-options", "-g", "-q", "-v", key)
}

// PaneHasLiveChildren reports whether panePID (a pane's foreground shell) has
// any child process — a backgrounded job an idle prompt hides from
// pane_current_command. Single `ps --ppid` round-trip; false on any failure so
// the reaper never blocks a kill on a lookup error it can't interpret.
func (c *Client) PaneHasLiveChildren(panePID int) bool {
	if panePID <= 0 {
		return false
	}
	out, err := exec.Command("ps", "--ppid", strconv.Itoa(panePID), "-o", "pid=", "--no-headers").Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

// ListPaneOptionValues returns key's value for every pane on the server,
// one entry per pane (empty string when unset).
func (c *Client) ListPaneOptionValues(key string) ([]string, error) {
	out, err := c.run("list-panes", "-a", "-F", "#{"+key+"}")
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimRight(out, "\n"), "\n"), nil
}

// RefreshStatus forces a status-line redraw via refresh-client -S. Fails
// with "no current client" when no client is attached — best-effort only;
// callers must not fail state writes on this error.
func (c *Client) RefreshStatus() error {
	return c.runSilent("refresh-client", "-S")
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
