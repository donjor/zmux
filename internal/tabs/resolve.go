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
	if t := ByID(tabs, name); t != nil {
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
