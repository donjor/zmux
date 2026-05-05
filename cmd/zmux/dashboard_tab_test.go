package main

import (
	"testing"

	"github.com/donjor/zmux/internal/tui/dashboard"
)

// TestResolveDashboardTabDeprecatedAliases locks in the backward-compat
// behavior for the --dashboard-tab flag. The picker refactor renamed the
// tab IDs from "current"/"sessions" to "session"/"workspaces"; the old
// names must still resolve so existing scripts and command palette
// providers keep working (Codex #10).
func TestResolveDashboardTabDeprecatedAliases(t *testing.T) {
	tests := []struct {
		flag string
		want dashboard.TabID
	}{
		{"", dashboard.TabSession},              // empty → default
		{"current", dashboard.TabSession},       // deprecated alias
		{"sessions", dashboard.TabWorkspaces},   // deprecated alias
		{"session", dashboard.TabSession},       // new canonical
		{"workspaces", dashboard.TabWorkspaces}, // new canonical
		{"themes", dashboard.TabThemes},         // pass-through
		{"bar", dashboard.TabBar},               // pass-through
		{"settings", dashboard.TabSettings},     // pass-through
		{"help", dashboard.TabHelp},             // pass-through
	}
	for _, tt := range tests {
		t.Run(tt.flag, func(t *testing.T) {
			got := resolveDashboardTab(tt.flag)
			if got != tt.want {
				t.Errorf("resolveDashboardTab(%q) = %q, want %q", tt.flag, got, tt.want)
			}
		})
	}
}
