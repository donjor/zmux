package tabs

import (
	"charm.land/bubbles/v2/viewport"
)

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
