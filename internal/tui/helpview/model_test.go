package helpview

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/donjor/zmux/internal/help"
	"github.com/donjor/zmux/internal/tui/styles"
)

func fixture() []help.Section {
	return []help.Section{
		{Title: "Sessions", Entries: []help.Entry{
			{Label: "zmux new <ws>", Desc: "Create workspace + main session"},
			{Label: "zmux kill <name>", Desc: "Smart kill — workspace-first"},
		}},
		{Title: "Panes", Entries: []help.Entry{
			{Label: "prefix + =", Desc: "Equalize splits evenly"},
		}},
	}
}

func countEntries(sections []help.Section) int {
	n := 0
	for _, s := range sections {
		n += len(s.Entries)
	}
	return n
}

func TestFilterEmptyQueryReturnsAll(t *testing.T) {
	in := fixture()
	got := FilterSections(in, "")
	if countEntries(got) != countEntries(in) || len(got) != len(in) {
		t.Fatalf("empty query: got %d sections / %d entries, want %d / %d",
			len(got), countEntries(got), len(in), countEntries(in))
	}
	// Whitespace-only is treated as empty.
	if countEntries(FilterSections(in, "   ")) != countEntries(in) {
		t.Errorf("whitespace query should return all entries")
	}
}

// TestFilterNarrowsAndDropsEmptySections: a query that matches one section's
// entries returns only those, and the non-matching section is dropped entirely.
func TestFilterNarrows(t *testing.T) {
	got := FilterSections(fixture(), "equalize")
	if len(got) != 1 || got[0].Title != "Panes" {
		t.Fatalf("query 'equalize': got sections %v, want only [Panes]", titles(got))
	}
	if len(got[0].Entries) != 1 || got[0].Entries[0].Label != "prefix + =" {
		t.Errorf("query 'equalize': got entries %v, want [prefix + =]", got[0].Entries)
	}
}

func TestFilterNoMatchReturnsEmpty(t *testing.T) {
	if got := FilterSections(fixture(), "zzzznotathing"); len(got) != 0 {
		t.Errorf("gibberish query returned %d sections, want 0", len(got))
	}
}

// TestFilterPreservesOrder: matches across sections keep original section order,
// not fuzzy rank order (a help reference reads best in a stable layout).
func TestFilterPreservesOrder(t *testing.T) {
	got := FilterSections(fixture(), "zmux")
	if len(got) != 1 || got[0].Title != "Sessions" {
		t.Fatalf("query 'zmux': got %v, want [Sessions]", titles(got))
	}
	if len(got[0].Entries) != 2 {
		t.Errorf("query 'zmux': got %d entries, want both session commands", len(got[0].Entries))
	}
}

// TestViewFillsHeightWithScrollbar pins two invariants at once: view() renders
// exactly the window height (no overflow that would scroll chrome off), and the
// full registry content draws the shared scrollbar thumb (the bar we now reuse
// from internal/tui/scroll rather than rendering a raw viewport).
func TestViewFillsHeightWithScrollbar(t *testing.T) {
	m := New(help.Sections(), styles.Styles{})
	tm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	out := tm.(*Model).view()

	if got := len(strings.Split(out, "\n")); got != 30 {
		t.Fatalf("view rendered %d lines, want exactly 30 (height invariant)", got)
	}
	if !strings.Contains(out, "▐") {
		t.Errorf("full help content should show a scrollbar thumb, found none")
	}
}

func titles(sections []help.Section) []string {
	var out []string
	for _, s := range sections {
		out = append(out, s.Title)
	}
	return out
}
