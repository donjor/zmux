package dashboard

// ContentRect describes the usable area for tab content,
// excluding chrome (tab bar, status line, help bar).
type ContentRect struct {
	Width  int
	Height int
}

const (
	// Chrome heights. Status line is inline with the tab bar, so it
	// contributes nothing to the reserved chrome height.
	tabBarHeight  = 3 // tab bar + separator + blank line
	helpBarHeight = 2 // blank line + help bar

	// Minimum dimensions.
	minWidth  = 60
	minHeight = 16

	// Compact thresholds.
	compactWidth  = 80
	compactHeight = 24
)

// ComputeContentRect calculates the available content area given terminal size.
func ComputeContentRect(termWidth, termHeight int) ContentRect {
	w := termWidth - 4 // 2-char padding each side
	if w < 0 {
		w = 0
	}

	h := termHeight - tabBarHeight - helpBarHeight
	if h < 0 {
		h = 0
	}

	return ContentRect{Width: w, Height: h}
}

// IsCompact returns true if the terminal is below the compact threshold.
func IsCompact(termWidth, termHeight int) bool {
	return termWidth < compactWidth || termHeight < compactHeight
}

// IsTooSmall returns true if the terminal is below the hard minimum.
func IsTooSmall(termWidth, termHeight int) bool {
	return termWidth < minWidth || termHeight < minHeight
}
