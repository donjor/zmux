package tabs

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	"github.com/donjor/zmux/internal/tui/styles"
)

// renderScrollable renders the viewport content with a scrollbar indicator
// on the right edge when content is taller than the viewport. Returns
// vp.View() unchanged when no scrolling is needed.
func renderScrollable(vp viewport.Model, styles styles.Styles) string {
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

	thumb := styles.Dim.Render("▐")

	for i := 0; i < len(lines) && i < vp.Height(); i++ {
		if i >= thumbStart && i < thumbEnd {
			lines[i] += " " + thumb
		}
	}

	return strings.Join(lines, "\n")
}

// ensureCursorVisible adjusts the viewport's YOffset so the given
// cursor line is visible with a small margin above and below.
func ensureCursorVisible(vp *viewport.Model, cursorLine int) {
	const margin = 2
	top := vp.YOffset()
	bottom := top + vp.Height() - 1

	if cursorLine < top+margin {
		offset := cursorLine - margin
		if offset < 0 {
			offset = 0
		}
		vp.SetYOffset(offset)
	} else if cursorLine > bottom-margin {
		vp.SetYOffset(cursorLine - vp.Height() + margin + 1)
	}
}
