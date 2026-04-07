package outline

// scrollMargin is how many rows of padding we try to keep above/below
// the cursor before it hits the edge of the visible window. It's a
// comfort feature — the cursor doesn't feel glued to the top/bottom.
const scrollMargin = 2

// ComputeWindow returns the [start, end) row indices to render for a
// list of `total` rows with `cursor` currently selected, given a visible
// `height` (row capacity, not pixel height).
//
// Defensive against zero and negative inputs: height <= 0 returns (0, 0),
// total <= 0 returns (0, 0). Cursor is clamped to [0, total).
//
// Honours a scroll-margin so the cursor stays comfortable when possible.
// If height is smaller than 2*scrollMargin+1, the cursor will sit flush
// with the edges.
func ComputeWindow(cursor, total, height int) (start, end int) {
	if total <= 0 || height <= 0 {
		return 0, 0
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= total {
		cursor = total - 1
	}

	// Small lists: render everything.
	if total <= height {
		return 0, total
	}

	// Compute a window that keeps the cursor inside it, biased so the
	// cursor isn't glued to the top/bottom edge when possible.
	margin := scrollMargin
	if 2*margin+1 > height {
		margin = (height - 1) / 2
	}

	start = cursor - margin
	if start < 0 {
		start = 0
	}
	end = start + height
	if end > total {
		end = total
		start = end - height
		if start < 0 {
			start = 0
		}
	}
	return start, end
}
