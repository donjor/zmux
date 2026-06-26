package tabs

import (
	"errors"
	"fmt"
	"strings"
)

// ErrNotFound means no logical tab matched; callers fall through to the
// legacy window-name path and finally the raw tmux target.
var ErrNotFound = errors.New("no logical tab matched")

// AmbiguousError reports a bare name that matches several tabs; the message
// lists ids so the user can address one exactly.
type AmbiguousError struct {
	Name    string
	Matches []LogicalTab
}

func (e *AmbiguousError) Error() string {
	parts := make([]string, 0, len(e.Matches))
	for _, t := range e.Matches {
		parts = append(parts, fmt.Sprintf("%s (%s, %s)", t.ID, t.OriginSession, t.Placement))
	}
	return fmt.Sprintf("tab %q is ambiguous — use an id: %s", e.Name, strings.Join(parts, ", "))
}

// Resolve finds a logical tab by exact id, then by exact label. Labels are
// scoped: tabs in the scope session (or docked with it as origin) win; a
// unique match elsewhere on the server still resolves, but duplicates
// outside the scope error rather than guess. Returns ErrNotFound when
// nothing matched — the caller owns the window-name and raw-target
// fallbacks (resolution order: id → label in scope → window label/name →
// raw tmux target).
func Resolve(tabs []LogicalTab, name, scope string) (*LogicalTab, error) {
	if t := byIDPreferScope(tabs, name, scope); t != nil {
		return t, nil
	}

	var labeled []LogicalTab
	for _, t := range tabs {
		if t.Label == name {
			labeled = append(labeled, t)
		}
	}
	if len(labeled) == 0 {
		return nil, ErrNotFound
	}
	if scope != "" {
		var scoped []LogicalTab
		for _, t := range labeled {
			if t.InScope(scope) {
				scoped = append(scoped, t)
			}
		}
		if len(scoped) == 1 {
			return &scoped[0], nil
		}
		if len(scoped) > 1 {
			return nil, &AmbiguousError{Name: name, Matches: scoped}
		}
	}
	if len(labeled) == 1 {
		return &labeled[0], nil
	}
	return nil, &AmbiguousError{Name: name, Matches: labeled}
}

// TabAtIndex returns the full tab at 1-based window index n in the session, or
// nil. It mirrors the numbered cells of the bar's logical tab row: only full
// tabs carry a visible number (pane-of tabs ride a host cell, docked tabs are
// unnumbered), so only a full tab sitting at that window index resolves.
//
// Pure and opt-in: placement verbs that want index addressing call this AFTER a
// normal id/label miss (so a tab literally labeled "2" still wins index 2). The
// shared name resolver never indexes — that would silently change send / watch
// / kill / run, which keep tmux window-index/raw semantics (codex R1#2).
func TabAtIndex(tabs []LogicalTab, session string, n int) *LogicalTab {
	if n < 1 || session == "" {
		return nil
	}
	for i := range tabs {
		t := &tabs[i]
		if t.Session == session && t.Placement == PlacementFull && t.WindowIndex == n {
			return t
		}
	}
	return nil
}

// byIDPreferScope finds the tab whose ID == id, preferring an in-scope row.
// Session-group clones repeat a shared pane once per clone session (FromRows
// does not collapse them), so the same ID can appear under several sessions.
// A bare ByID would return whichever clone tmux listed first — which may be a
// sibling session, making the tab read as out-of-scope to a session-only
// caller (report 039) even though the user sees it as local. Preferring the
// in-scope row keeps a clone-local tab resolving to the caller's session;
// with no in-scope row it falls back to the first match, so a genuinely unique
// out-of-session id still resolves (the existing id-ignores-scope convenience).
func byIDPreferScope(tabs []LogicalTab, id, scope string) *LogicalTab {
	var first *LogicalTab
	for i := range tabs {
		if tabs[i].ID != id {
			continue
		}
		if scope == "" || tabs[i].InScope(scope) {
			return &tabs[i]
		}
		if first == nil {
			first = &tabs[i]
		}
	}
	return first
}
