// Package tui provides bubbletea-based terminal UI components
// for zmux: session picker, theme picker, dashboard, and init wizard.
package tui

// max returns the larger of a and b.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
