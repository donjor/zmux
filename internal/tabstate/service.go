package tabstate

import (
	"strconv"
	"time"

	"github.com/donjor/zmux/internal/tmux"
)

// Service writes and clears tab states through an injected tmux.Runner.
// Every mutation ends with a best-effort RefreshStatus so the bar redraws
// immediately instead of waiting on status-interval.
type Service struct {
	runner tmux.Runner
	getenv func(string) string
	now    func() time.Time // injectable for tests
}

// New constructs a Service. getenv is injected (os.Getenv in production) so
// $TMUX_PANE fallback is testable.
func New(runner tmux.Runner, getenv func(string) string) *Service {
	return &Service{runner: runner, getenv: getenv, now: time.Now}
}

// Resolve resolves a target spec (see ResolveTarget).
func (s *Service) Resolve(spec string) (Target, error) {
	return ResolveTarget(s.runner, spec, s.getenv)
}

// Set writes st canonically to the pane, mirrors it to the window, and
// refreshes the status line. Empty source/msg unset their options so stale
// values never outlive the state that carried them. All eight option writes
// go out as ONE batched tmux invocation.
func (s *Service) Set(t Target, st State, source, msg string) error {
	at := strconv.FormatInt(s.now().Unix(), 10)
	values := map[string]string{
		OptState:  string(st),
		OptSource: source,
		OptAt:     at,
		OptMsg:    msg,
	}
	writes := make([]tmux.OptionWrite, 0, len(MirrorKeys)*2)
	for _, key := range MirrorKeys {
		writes = append(writes, stateWrites(t, key, values[key])...)
	}
	if err := s.runner.ApplyOptions(writes); err != nil {
		return err
	}
	s.syncStatusInterval(st == StateRunning)
	_ = s.runner.RefreshStatus() // best-effort: "no current client" when detached
	return nil
}

// Clear removes all state options from the pane and its window mirror in one
// batched invocation.
func (s *Service) Clear(t Target) error {
	writes := make([]tmux.OptionWrite, 0, len(MirrorKeys)*2)
	for _, key := range MirrorKeys {
		writes = append(writes, stateWrites(t, key, "")...)
	}
	if err := s.runner.ApplyOptions(writes); err != nil {
		return err
	}
	s.syncStatusInterval(false)
	_ = s.runner.RefreshStatus()
	return nil
}

// stateWrites is the canonical-pane + window-mirror write pair for one
// option; an empty value becomes an unset so stale values never linger.
func stateWrites(t Target, key, value string) []tmux.OptionWrite {
	unset := value == ""
	return []tmux.OptionWrite{
		{Scope: tmux.ScopePane, Target: t.PaneID, Key: key, Value: value, Unset: unset},
		{Scope: tmux.ScopeWindow, Target: t.Window, Key: key, Value: value, Unset: unset},
	}
}

// Status-interval cadence. The running glyph's spinner is a #() status job
// that only steps as fast as status-interval ticks — but interval 1 also
// re-runs every bar-render job each second, a constant background spawn
// load that's pure waste while nothing runs. So the conf baseline stays at
// the idle cadence and the service flips to the active one while any pane
// holds a running state. Both edges live here because every state write
// flows through Set/Clear; the flips are best-effort like RefreshStatus.
const (
	intervalActive = "1"
	intervalIdle   = tmux.StatusIntervalIdle // shared with the conf.go baseline
)

func (s *Service) syncStatusInterval(runningJustSet bool) {
	if runningJustSet {
		_ = s.runner.SetOption("-g", "status-interval", intervalActive)
		return
	}
	vals, err := s.runner.ListPaneOptionValues(OptState)
	if err != nil {
		return // never fail a state write over spinner cadence
	}
	for _, v := range vals {
		if trim(v) == string(StateRunning) {
			return // another tab still runs — keep the fast tick
		}
	}
	_ = s.runner.SetOption("-g", "status-interval", intervalIdle)
}

// Current reads the pane's canonical state; empty string when unset.
func (s *Service) Current(t Target) (string, error) {
	out, err := s.runner.DisplayMessage(t.PaneID, "#{"+OptState+"}")
	if err != nil {
		return "", err
	}
	return trim(out), nil
}

// ClearIf clears only when the current state matches one of want — the
// conditional behind "focus clears attention, not done". Reports whether a
// clear happened.
func (s *Service) ClearIf(t Target, want ...State) (bool, error) {
	cur, err := s.Current(t)
	if err != nil {
		return false, err
	}
	for _, st := range want {
		if cur == string(st) {
			return true, s.Clear(t)
		}
	}
	return false, nil
}

// Visible reports whether the pane's window is the current window of an
// attached session — window membership, not active-pane equality, so a
// visible sidecar pane counts (plan 026 review catch).
func (s *Service) Visible(t Target) (bool, error) {
	out, err := s.runner.DisplayMessage(t.PaneID, "#{window_active}\t#{session_attached}")
	if err != nil {
		return false, err
	}
	parts := splitPair(out)
	return parts[0] == "1" && parts[1] != "" && parts[1] != "0", nil
}

// SetDoneByVisibility applies the Stop-hook rule from the ratified clear
// table: done when the pane is visible, attention when it is not, done when
// visibility is indeterminate (prefer quiet success over a wrong alarm).
// Returns the state actually written.
func (s *Service) SetDoneByVisibility(t Target, source, msg string) (State, error) {
	st := StateDone
	if vis, err := s.Visible(t); err == nil && !vis {
		st = StateAttention
	}
	return st, s.Set(t, st, source, msg)
}

func trim(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}

func splitPair(s string) [2]string {
	s = trim(s)
	for i := 0; i < len(s); i++ {
		if s[i] == '\t' {
			return [2]string{s[:i], s[i+1:]}
		}
	}
	return [2]string{s, ""}
}
