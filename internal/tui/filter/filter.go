// Package filter holds the fuzzy list filtering shared by the picker-shaped
// TUI surfaces (pickers, theme lists, workspace lists, dashboard tabs).
package filter

import (
	"strings"

	"github.com/sahilm/fuzzy"
)

// Fuzzy returns the items whose name fuzzy-matches query, best match first.
// An empty (or whitespace) query returns items unchanged.
func Fuzzy[T any](items []T, query string, name func(T) string) []T {
	if strings.TrimSpace(query) == "" {
		return items
	}
	names := make([]string, len(items))
	for i := range items {
		names[i] = name(items[i])
	}
	matches := fuzzy.Find(query, names)
	out := make([]T, len(matches))
	for i, m := range matches {
		out[i] = items[m.Index]
	}
	return out
}
