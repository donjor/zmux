package views

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func testRowStyles() SessionRowStyles {
	return SessionRowStyles{
		Normal:  lipgloss.NewStyle().Foreground(lipgloss.Color("15")),
		Accent:  lipgloss.NewStyle().Foreground(lipgloss.Color("3")),
		Dim:     lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		Info:    lipgloss.NewStyle().Foreground(lipgloss.Color("4")),
		Success: lipgloss.NewStyle().Foreground(lipgloss.Color("2")),
	}
}

func TestRenderSessionRowBasic(t *testing.T) {
	row := SessionRow{
		Name:          "dev",
		Age:           "2h",
		WindowsText:   "[editor, server]",
		DirectoryText: "~/work",
		IsAttached:    true,
	}

	result := RenderSessionRow(row, testRowStyles(), 80)

	if !strings.Contains(result, "dev") {
		t.Error("expected output to contain session name 'dev'")
	}
	if !strings.Contains(result, "2h") {
		t.Error("expected output to contain age '2h'")
	}
	if !strings.Contains(result, "attached") {
		t.Error("expected output to contain 'attached' status")
	}
	if !strings.Contains(result, "editor") {
		t.Error("expected output to contain window name 'editor'")
	}
	if !strings.Contains(result, "~/work") {
		t.Error("expected output to contain directory")
	}
}

func TestRenderSessionRowSelected(t *testing.T) {
	row := SessionRow{
		Name:       "dev",
		IsSelected: true,
	}

	result := RenderSessionRow(row, testRowStyles(), 80)

	// Should contain cursor indicator.
	if !strings.Contains(result, "▸") {
		t.Error("expected cursor indicator for selected row")
	}
}

func TestRenderSessionRowTmp(t *testing.T) {
	row := SessionRow{
		Name:  "tmp-1",
		IsTmp: true,
	}

	result := RenderSessionRow(row, testRowStyles(), 80)

	if !strings.Contains(result, "tmp-1") {
		t.Error("expected output to contain 'tmp-1'")
	}
}

func TestRenderSessionRowCurrent(t *testing.T) {
	row := SessionRow{
		Name:      "dev",
		IsCurrent: true,
	}

	result := RenderSessionRow(row, testRowStyles(), 80)

	if !strings.Contains(result, "*") {
		t.Error("expected current session marker")
	}
}

func TestRenderSessionRowWithIndex(t *testing.T) {
	row := SessionRow{
		Name:  "dev",
		Index: 1,
	}

	result := RenderSessionRow(row, testRowStyles(), 80)

	if !strings.Contains(result, "1") {
		t.Error("expected quick-select index '1'")
	}
}

func TestRenderSessionRowIdle(t *testing.T) {
	row := SessionRow{
		Name:       "idle-session",
		IsAttached: false,
	}

	result := RenderSessionRow(row, testRowStyles(), 80)

	if !strings.Contains(result, "idle") {
		t.Error("expected 'idle' status for non-attached session")
	}
}

func TestRenderSessionRowTwoLines(t *testing.T) {
	row := SessionRow{
		Name:          "dev",
		Age:           "1h",
		WindowsText:   "[editor]",
		DirectoryText: "~/code",
		IsAttached:    true,
	}

	result := RenderSessionRow(row, testRowStyles(), 80)

	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}
}

func TestRenderSessionDivider(t *testing.T) {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	result := RenderSessionDivider(dim, 80)

	if !strings.Contains(result, "─") {
		t.Error("expected divider line")
	}
}

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
