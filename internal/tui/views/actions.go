package views

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ActionHint represents a single action with a hotkey for display in the action bar.
type ActionHint struct {
	Key  string
	Name string
}

// ActionBarStyles holds the lipgloss styles needed by the action bar renderer.
type ActionBarStyles struct {
	Help lipgloss.Style
}

// RenderActions renders a bottom bar showing available actions with hotkey hints.
func RenderActions(actions []ActionHint, styles ActionBarStyles, width int) string {
	parts := make([]string, 0, len(actions))
	for _, a := range actions {
		parts = append(parts, a.Key+":"+a.Name)
	}
	return styles.Help.Render("  " + strings.Join(parts, "  "))
}
