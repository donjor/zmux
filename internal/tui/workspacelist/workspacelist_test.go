package workspacelist

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/donjor/zmux/internal/session"
	"github.com/donjor/zmux/internal/tui/styles"
	"github.com/donjor/zmux/internal/tui/tkey"
	"github.com/donjor/zmux/internal/tui/workspaceview"
	"github.com/donjor/zmux/internal/workspace"
)

func vm(name string, sessions ...session.SessionInfo) workspaceview.WorkspaceViewModel {
	return workspaceview.WorkspaceViewModel{
		Workspace:    workspace.Workspace{Name: name},
		LiveSessions: sessions,
	}
}

func sampleConfig(t *testing.T) Config {
	t.Helper()
	return Config{
		Workspaces: []workspaceview.WorkspaceViewModel{
			vm("alpha", session.SessionInfo{Name: "main"}),
			vm("beta", session.SessionInfo{Name: "main"}),
			vm("gamma", session.SessionInfo{Name: "main"}),
		},
		ShowEmpty: true,
		Styles:    styles.DefaultStyles(),
	}
}

func sendKey(m Model, k tea.KeyPressMsg) Model {
	updated, _ := m.Update(k)
	return updated
}

func typeChars(m Model, s string) Model {
	for _, r := range s {
		if r == ' ' {
			m = sendKey(m, tkey.Space())
			continue
		}
		m = sendKey(m, tkey.Rune(r))
	}
	return m
}

func TestWorkspacelist_LoadsAllWorkspaces(t *testing.T) {
	cfg := sampleConfig(t)
	m := New(cfg)
	if len(m.filtered) != 3 {
		t.Fatalf("filtered = %d, want 3", len(m.filtered))
	}
}

func TestWorkspacelist_HidesEmptyWhenShowEmptyFalse(t *testing.T) {
	cfg := sampleConfig(t)
	cfg.Workspaces = append(cfg.Workspaces, vm("empty"))
	cfg.ShowEmpty = false
	m := New(cfg)

	for _, ws := range m.filtered {
		if ws.Name == "empty" {
			t.Errorf("ShowEmpty=false should hide empty workspaces, got %q", ws.Name)
		}
	}
}

func TestWorkspacelist_FuzzyFilters(t *testing.T) {
	cfg := sampleConfig(t)
	m := New(cfg)
	m = typeChars(m, "be")
	if len(m.filtered) != 1 || m.filtered[0].Name != "beta" {
		t.Errorf("filtered = %v, want [beta]", workspaceNames(m.filtered))
	}
}

func TestWorkspacelist_OnSelectFiredByEnter(t *testing.T) {
	cfg := sampleConfig(t)
	var picked string
	cfg.OnSelect = func(ws workspaceview.WorkspaceViewModel) tea.Cmd {
		picked = ws.Name
		return nil
	}
	m := New(cfg)
	sendKey(m, tkey.Enter())
	if picked != "alpha" {
		t.Errorf("OnSelect not fired with alpha, got %q", picked)
	}
}

func TestWorkspacelist_OnCreateFiredByCreateGrammar(t *testing.T) {
	cfg := sampleConfig(t)
	cfg.Workspaces = nil // no workspaces — exercise the empty path
	var gotWs, gotSess string
	cfg.OnCreate = func(workspaceName, sessionName string) tea.Cmd {
		gotWs = workspaceName
		gotSess = sessionName
		return nil
	}
	cfg.OnSelect = func(ws workspaceview.WorkspaceViewModel) tea.Cmd {
		t.Errorf("OnSelect must not fire when create grammar is active")
		return nil
	}
	m := New(cfg)
	m = typeChars(m, "newproj dev")
	sendKey(m, tkey.Enter())
	if gotWs != "newproj" || gotSess != "dev" {
		t.Errorf("OnCreate args = (%q,%q), want (newproj,dev)", gotWs, gotSess)
	}
}

func TestWorkspacelist_OnRenameFiredAfterCtrlR(t *testing.T) {
	cfg := sampleConfig(t)
	var oldName, newName string
	cfg.OnRename = func(o, n string) tea.Cmd {
		oldName, newName = o, n
		return nil
	}
	m := New(cfg)

	m = sendKey(m, tkey.Ctrl('r'))
	if m.mode != modeRename {
		t.Fatal("ctrl+r should enter rename mode")
	}
	// Replace the prefilled name; we just send Backspaces and new text.
	backspace := tea.KeyPressMsg{Code: tea.KeyBackspace}
	for range "alpha" {
		m = sendKey(m, backspace)
	}
	m = typeChars(m, "ALPHA2")
	m = sendKey(m, tkey.Enter())

	if oldName != "alpha" || newName != "ALPHA2" {
		t.Errorf("OnRename args = (%q,%q), want (alpha,ALPHA2)", oldName, newName)
	}
}

func TestWorkspacelist_OnDeleteFiredByCtrlX(t *testing.T) {
	cfg := sampleConfig(t)
	var deleted string
	cfg.OnDelete = func(name string) tea.Cmd {
		deleted = name
		return nil
	}
	m := New(cfg)
	sendKey(m, tkey.Ctrl('x'))
	if deleted != "alpha" {
		t.Errorf("OnDelete not fired with alpha, got %q", deleted)
	}
}

func TestWorkspacelist_FooterOnlyShowsActiveHooks(t *testing.T) {
	cfg := sampleConfig(t)
	// Default cfg has no hooks set.
	m := New(cfg)
	footer := m.helpFooter()
	if strings.Contains(footer, "rename") || strings.Contains(footer, "delete") ||
		strings.Contains(footer, "new") || strings.Contains(footer, "select") {
		t.Errorf("footer without hooks should only show esc, got %q", footer)
	}

	cfg.OnSelect = func(workspaceview.WorkspaceViewModel) tea.Cmd { return nil }
	cfg.OnDelete = func(string) tea.Cmd { return nil }
	m = New(cfg)
	footer = m.helpFooter()
	if !strings.Contains(footer, "enter:select") {
		t.Errorf("OnSelect set: footer should contain enter:select, got %q", footer)
	}
	if !strings.Contains(footer, "ctrl+x:delete") {
		t.Errorf("OnDelete set: footer should contain ctrl+x:delete, got %q", footer)
	}
	if strings.Contains(footer, "rename") || strings.Contains(footer, "new") {
		t.Errorf("OnRename/OnCreate unset: footer should not show them, got %q", footer)
	}
}

func TestWorkspacelist_ReloadReplacesData(t *testing.T) {
	cfg := sampleConfig(t)
	m := New(cfg)
	updated, _ := m.Update(ReloadMsg{Workspaces: []workspaceview.WorkspaceViewModel{vm("delta")}})
	m = updated
	if len(m.filtered) != 1 || m.filtered[0].Name != "delta" {
		t.Errorf("Reload didn't replace data, got %v", workspaceNames(m.filtered))
	}
}

func workspaceNames(wss []workspaceview.WorkspaceViewModel) []string {
	out := make([]string, len(wss))
	for i, ws := range wss {
		out[i] = ws.Name
	}
	return out
}
