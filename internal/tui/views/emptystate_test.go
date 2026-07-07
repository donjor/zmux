package views

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestRenderEmptyState(t *testing.T) {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	result := RenderEmptyState("No items", "Press n to create", dim)

	if !strings.Contains(result, "No items") {
		t.Error("expected message")
	}
	if !strings.Contains(result, "Press n to create") {
		t.Error("expected hint")
	}
}

func TestRenderEmptyStateNoHint(t *testing.T) {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	result := RenderEmptyState("No items", "", dim)

	if !strings.Contains(result, "No items") {
		t.Error("expected message")
	}
}
