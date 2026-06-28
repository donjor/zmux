// Package scroll renders a bubbles viewport with a right-edge scrollbar thumb.
// It is the single scrollbar renderer shared by every scrollable TUI surface
// (dashboard tabs, the help viewer) so none of them reimplement the bar.
package scroll

import (
	"strings"

	"charm.land/bubbles/v2/viewport"

	"github.com/donjor/zmux/internal/tui/styles"
)

// Scrollable renders the viewport content with a scrollbar thumb on the right
// edge when the content is taller than the viewport. It returns vp.View()
// unchanged when no scrolling is needed. Callers that render at full pane width
// should reserve 2 trailing columns (" ▐") so the thumb cannot overflow.
func Scrollable(vp viewport.Model, st styles.Styles) string {
	view := vp.View()
	totalLines := vp.TotalLineCount()

	if totalLines <= vp.Height() || vp.Height() <= 0 {
		return view
	}

	lines := strings.Split(view, "\n")

	// Thumb size proportional to the visible fraction.
	thumbHeight := vp.Height() * vp.Height() / totalLines
	if thumbHeight < 1 {
		thumbHeight = 1
	}

	// Thumb position from scroll percentage.
	track := vp.Height() - thumbHeight
	thumbStart := 0
	if track > 0 {
		thumbStart = int(vp.ScrollPercent()*float64(track) + 0.5)
	}
	thumbEnd := thumbStart + thumbHeight

	thumb := st.Dim.Render("▐")

	for i := 0; i < len(lines) && i < vp.Height(); i++ {
		if i >= thumbStart && i < thumbEnd {
			lines[i] += " " + thumb
		}
	}

	return strings.Join(lines, "\n")
}
