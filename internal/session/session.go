// Package session manages tmux session lifecycle, tmp session model, and cleanup.
package session

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tmux"
)

// SessionInfo provides enriched metadata about a tmux session.
type SessionInfo struct {
	Name            string
	Windows         int
	Attached        bool
	Activity        time.Time
	Created         time.Time
	LastAttached    time.Time
	Dir             string
	IsTmp           bool
	Group           string // session group name (empty if ungrouped)
	AttachedClients int    // number of attached clients (including grouped copies)
	Managed         bool
	Workspace       string
	Label           string
	SessionID       string
	Clone           bool
	PinnedView      bool
	ViewRoot        string
}

var tmpPattern = regexp.MustCompile(`^tmp-(\d+)$`)

// DefaultName is the primary session name created for a workspace when the
// user does not provide an explicit session name.
const DefaultName = "main"

// ValidateName checks if a session name is valid.
// Names cannot start with a digit (reserved for index-based selection).
func ValidateName(name string) error {
	if name == "" {
		return nil // empty is allowed (means auto tmp-N)
	}
	if len(name) > 0 && name[0] >= '0' && name[0] <= '9' {
		return fmt.Errorf("session name cannot start with a number (reserved for quick-select)")
	}
	return nil
}

// IsTemp returns true if the session name matches the "tmp-N" pattern.
func IsTemp(name string) bool {
	return tmpPattern.MatchString(name)
}

// NextTmpName finds the next available tmp-N name by scanning existing sessions.
func NextTmpName(runner tmux.Runner) string {
	sessions, err := runner.ListSessions()
	if err != nil {
		return "tmp-1"
	}

	maxN := 0
	for _, s := range sessions {
		m := tmpPattern.FindStringSubmatch(s.Name)
		if m != nil {
			n, _ := strconv.Atoi(m[1])
			if n > maxN {
				maxN = n
			}
		}
	}
	return fmt.Sprintf("tmp-%d", maxN+1)
}

// ListSessions returns enriched session info, sorted with named sessions
// first (alphabetically) and tmp sessions last (by number).
// Grouped sessions (e.g. zmux-b sharing windows with zmux) are collapsed
// into the root session with an AttachedClients count.
func ListSessions(runner tmux.Runner) ([]SessionInfo, error) {
	raw, err := runner.ListSessions()
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	// Reserved zmux-internal sessions (the hidden-tab dock) never surface:
	// this is the collapse point every user-facing list flows through
	// (ls, pickers, dashboard, bar, workspace views).
	filtered := raw[:0]
	for _, s := range raw {
		if !tabs.IsReservedSession(s.Name) {
			filtered = append(filtered, s)
		}
	}
	raw = filtered

	// Build a set of session names for root-existence checks.
	nameSet := make(map[string]bool, len(raw))
	for _, s := range raw {
		nameSet[s.Name] = true
	}

	// First pass: identify root sessions and count grouped copies.
	// A session is a grouped copy if it has a group, its RootName differs
	// from its own name, and the root session actually exists.
	roots := make(map[string]int) // group name → index into infos
	var infos []SessionInfo

	for _, s := range raw {
		root := RootName(s.Name)
		isGroupedCopy := s.Group != "" && root != s.Name && nameSet[root]

		if isGroupedCopy && !s.PinnedView {
			// Merge into root: count attached clients. Index, not pointer —
			// a pointer into infos goes stale when append reallocates.
			if idx, ok := roots[s.Group]; ok {
				if s.Attached {
					infos[idx].AttachedClients++
				}
			}
			continue // don't add ephemeral grouped copies to the list
		}

		info := SessionInfo{
			Name:            s.Name,
			Windows:         s.Windows,
			Attached:        s.Attached,
			Activity:        s.Activity,
			Created:         s.Created,
			LastAttached:    s.LastAttached,
			Dir:             s.Dir,
			IsTmp:           IsTemp(s.Name),
			Group:           s.Group,
			AttachedClients: 0,
			Managed:         s.Managed,
			Workspace:       s.Workspace,
			Label:           s.SessionLabel,
			SessionID:       s.SessionID,
			Clone:           s.Clone,
			PinnedView:      s.PinnedView,
			ViewRoot:        s.ViewRoot,
		}
		if info.PinnedView && info.ViewRoot == "" {
			info.ViewRoot = root
		}
		if s.Attached {
			info.AttachedClients = 1
		}
		infos = append(infos, info)
		if s.Group != "" {
			roots[s.Group] = len(infos) - 1
		}
	}

	sort.Slice(infos, func(i, j int) bool {
		// Named sessions come before tmp sessions.
		if infos[i].IsTmp != infos[j].IsTmp {
			return !infos[i].IsTmp
		}
		// Within the same category, sort alphabetically.
		return infos[i].Name < infos[j].Name
	})

	return infos, nil
}

// HumanAge formats a duration from the given time to now in a compact human-readable form.
// Examples: "5m", "2h", "1d", "3w".
func HumanAge(t time.Time) string {
	return HumanAgeSince(t, time.Now())
}

// HumanAgeSince formats a duration from t to now in a compact human-readable form.
// Extracted for testability.
func HumanAgeSince(t time.Time, now time.Time) string {
	d := now.Sub(t)
	if d < 0 {
		d = 0
	}

	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	default:
		return fmt.Sprintf("%dw", int(d.Hours()/(24*7)))
	}
}
