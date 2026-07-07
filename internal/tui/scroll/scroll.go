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

	if !needsScrollbar(vp.Height(), totalLines) {
		return view
	}

	lines := strings.Split(view, "\n")
	thumbStart, thumbEnd := thumbGeometry(vp.Height(), totalLines, vp.ScrollPercent())
	thumb := st.Dim.Render("▐")

	for i := 0; i < len(lines) && i < vp.Height(); i++ {
		if i >= thumbStart && i < thumbEnd {
			lines[i] += " " + thumb
		}
	}

	return strings.Join(lines, "\n")
}

// needsScrollbar reports whether a viewport of the given height needs a thumb to
// show totalLines of content — false for empty/zero-height viewports and for
// content that fits exactly (or under) the visible height.
func needsScrollbar(height, totalLines int) bool {
	return height > 0 && totalLines > height
}

// thumbGeometry computes the scrollbar thumb's [start, end) rows for a viewport
// of the given height showing totalLines of content at scrollPercent (0..1). It
// assumes needsScrollbar already held. The thumb height is proportional to the
// visible fraction (min 1 row) and its position tracks the scroll percentage, so
// scrollPercent 1 lands the thumb flush against the bottom row (end == height).
func thumbGeometry(height, totalLines int, scrollPercent float64) (start, end int) {
	thumbHeight := height * height / totalLines
	if thumbHeight < 1 {
		thumbHeight = 1
	}
	track := height - thumbHeight
	start = 0
	if track > 0 {
		start = int(scrollPercent*float64(track) + 0.5)
	}
	return start, start + thumbHeight
}
