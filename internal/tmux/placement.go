package tmux

import "fmt"

// JoinPaneOptions describes a tmux join-pane call: move Source (a pane id)
// into Target's window as a sibling pane. tmux auto-unzooms a zoomed source
// or target window; callers wanting the zoom back re-toggle after the move.
type JoinPaneOptions struct {
	Source    string         // pane id (%N) to relocate — required
	Target    string         // destination pane (%N) or window target — required
	Direction SplitDirection // placement relative to Target; empty = down
	Size      string         // tmux -l value such as "40%" or "80"
	Detached  bool           // -d: don't move focus to the joined pane
}

// BreakPaneOptions describes a tmux break-pane call: promote Source (a pane
// id) into a window of its own.
type BreakPaneOptions struct {
	Source string // pane id (%N) to promote — required
	// Target window destination. Empty appends in the pane's own session.
	// "session:" (bare colon) appends in another session — this is how a
	// pane-of tab is hidden straight into the dock. Occupied explicit
	// indexes error ("index in use"); never -k through that.
	Target string
	// After inserts the new window directly after Target instead of
	// appending (break-pane -a), shifting later windows up by one.
	After    bool
	Name     string // -n: name the new window; empty leaves automatic-rename
	Detached bool   // -d: don't switch focus to the new window
}

// JoinPane relocates a pane into another window (tmux join-pane).
func (c *Client) JoinPane(opts JoinPaneOptions) error {
	args, err := buildJoinPaneArgs(opts)
	if err != nil {
		return err
	}
	return c.runSilent(args...)
}

func buildJoinPaneArgs(opts JoinPaneOptions) ([]string, error) {
	if opts.Source == "" || opts.Target == "" {
		return nil, fmt.Errorf("join-pane: source and target required")
	}
	args := []string{"join-pane"}
	switch opts.Direction {
	case "", SplitDown:
		args = append(args, "-v")
	case SplitUp:
		args = append(args, "-v", "-b")
	case SplitRight:
		args = append(args, "-h")
	case SplitLeft:
		args = append(args, "-h", "-b")
	default:
		return nil, fmt.Errorf("unknown split direction %q", opts.Direction)
	}
	if opts.Detached {
		args = append(args, "-d")
	}
	if opts.Size != "" {
		args = append(args, "-l", opts.Size)
	}
	args = append(args, "-s", opts.Source, "-t", opts.Target)
	return args, nil
}

// BreakPane promotes a pane into its own window (tmux break-pane) and
// returns the new window's id (@N).
func (c *Client) BreakPane(opts BreakPaneOptions) (string, error) {
	args, err := buildBreakPaneArgs(opts)
	if err != nil {
		return "", err
	}
	return c.run(args...)
}

func buildBreakPaneArgs(opts BreakPaneOptions) ([]string, error) {
	if opts.Source == "" {
		return nil, fmt.Errorf("break-pane: source required")
	}
	args := []string{"break-pane", "-P", "-F", "#{window_id}"}
	if opts.Detached {
		args = append(args, "-d")
	}
	if opts.After {
		args = append(args, "-a")
	}
	if opts.Name != "" {
		args = append(args, "-n", opts.Name)
	}
	args = append(args, "-s", opts.Source)
	if opts.Target != "" {
		args = append(args, "-t", opts.Target)
	}
	return args, nil
}

// SelectLayout applies a layout string (as read from #{window_layout}) to a
// window. A pane-count mismatch is SILENT — tmux exits 0 and best-efforts —
// so callers must compare pane counts themselves before trusting a restore.
func (c *Client) SelectLayout(target, layout string) error {
	return c.runSilent("select-layout", "-t", target, layout)
}

// ToggleZoom toggles pane zoom (resize-pane -Z). On a single-pane window
// tmux treats it as a no-op rather than an error.
func (c *Client) ToggleZoom(target string) error {
	return c.runSilent("resize-pane", "-Z", "-t", target)
}

// KillWindowByID kills a window addressed by its opaque id (@N), which stays
// valid across moves between sessions — unlike session:index targets.
func (c *Client) KillWindowByID(windowID string) error {
	return c.runSilent("kill-window", "-t", windowID)
}
