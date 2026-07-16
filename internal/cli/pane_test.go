package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donjor/zmux/internal/tmux"
)

func TestRunPaneOpenDefaultsToCurrentTmuxPaneAndPrintsID(t *testing.T) {
	a, mock := newTestApp(t)
	t.Setenv("TMUX_PANE", "%12")
	cmd := newPaneOpenCmd(a)
	var out bytes.Buffer
	cmd.SetOut(&out)

	cwd := t.TempDir()
	flags := &paneOpenFlags{right: "40", cwd: cwd}
	_ = cmd.Flags().Set("right", "40")
	if err := runPaneOpen(a, cmd, flags, []string{"clean-ui"}); err != nil {
		t.Fatalf("runPaneOpen failed: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "%57" {
		t.Fatalf("expected pane id %%57, got %q", got)
	}
	if len(mock.Calls) != 3 {
		t.Fatalf("expected IsInsideTmux + SplitPane + SetPaneOption calls, got %#v", mock.Calls)
	}
	call := mock.Calls[1]
	if call.Method != "SplitPane" {
		t.Fatalf("expected SplitPane call, got %#v", call)
	}
	wantCWD, _ := filepath.Abs(cwd)
	want := []string{"%12", "right", "40%", wantCWD, "clean-ui", "[]"}
	for i, arg := range want {
		if call.Args[i] != arg {
			t.Fatalf("SplitPane arg %d mismatch: got %q want %q (all args %#v)", i, call.Args[i], arg, call.Args)
		}
	}
}

func TestRunPaneOpenNoFocusUsesDetachedSplit(t *testing.T) {
	a, mock := newTestApp(t)
	t.Setenv("TMUX_PANE", "%12")
	cmd := newPaneOpenCmd(a)
	cmd.SetArgs([]string{"logs", "--no-focus"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("pane open command failed: %v", err)
	}
	call, ok := srFindCall(mock.Calls, "SplitPane")
	if !ok || call.Args[6] != "detached=true" {
		t.Fatalf("expected detached SplitPane, got %#v", mock.Calls)
	}
}

func TestRunPaneOpenAutoLabelsWindowBeforeSplit(t *testing.T) {
	a, mock := newTestApp(t)
	mock.DisplayMessageResult = "zmux\t\tpi"
	t.Setenv("TMUX_PANE", "%12")
	cmd := newPaneOpenCmd(a)
	cmd.SetArgs([]string{"clean-ui", "-r", "35", "--label-tab"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("pane open command failed: %v", err)
	}
	methods := make([]string, 0, len(mock.Calls))
	for _, call := range mock.Calls {
		methods = append(methods, call.Method)
	}
	want := []string{"IsInsideTmux", "DisplayMessage", "SetWindowOption", "SetWindowOption", "SplitPane", "SetPaneOption"}
	if len(methods) != len(want) {
		t.Fatalf("methods = %#v, want %#v", methods, want)
	}
	for i := range want {
		if methods[i] != want[i] {
			t.Fatalf("methods = %#v, want %#v", methods, want)
		}
	}
	if mock.Calls[2].Args[1] != "@zmux_label" || mock.Calls[2].Args[2] != "pi" {
		t.Fatalf("expected pane auto label pi, got %#v", mock.Calls[2])
	}
	if mock.Calls[3].Args[1] != "@zmux_label_source" || mock.Calls[3].Args[2] != "pane" {
		t.Fatalf("expected pane label source, got %#v", mock.Calls[3])
	}
}

func TestPaneOpenCommandPreservesArgsAfterDash(t *testing.T) {
	a, mock := newTestApp(t)
	t.Setenv("TMUX_PANE", "%12")
	cmd := newPaneOpenCmd(a)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"clean-ui", "-r", "35", "--", "bash", "-lc", "echo hi && sleep 1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("pane open command failed: %v", err)
	}
	call, ok := srFindCall(mock.Calls, "SplitPane")
	if !ok {
		t.Fatalf("expected SplitPane call, got %#v", mock.Calls)
	}
	if call.Args[2] != "35%" || call.Args[4] != "clean-ui" || call.Args[5] != "[\"bash\" \"-lc\" \"echo hi && sleep 1\"]" {
		t.Fatalf("unexpected SplitPane args: %#v", call.Args)
	}
}

func TestPaneOpenNameFlagLeavesArgsAsCommand(t *testing.T) {
	a, mock := newTestApp(t)
	t.Setenv("TMUX_PANE", "%12")
	cmd := newPaneOpenCmd(a)
	cmd.SetArgs([]string{"-n", "clean-ui", "-r", "35", "--", "bash", "-lc", "echo hi"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("pane open command failed: %v", err)
	}
	call, ok := srFindCall(mock.Calls, "SplitPane")
	if !ok || call.Args[4] != "clean-ui" || call.Args[5] != "[\"bash\" \"-lc\" \"echo hi\"]" {
		t.Fatalf("unexpected SplitPane args/calls: %#v", mock.Calls)
	}
}

func TestRunPaneOpenAllowsExplicitTargetOutsideTmux(t *testing.T) {
	a, mock := newTestApp(t)
	mock.InsideTmux = false
	cmd := newPaneOpenCmd(a)
	flags := &paneOpenFlags{target: "%99", down: paneAutoSize, size: "12"}
	_ = cmd.Flags().Set("down", paneAutoSize)
	if err := runPaneOpen(a, cmd, flags, nil); err != nil {
		t.Fatalf("runPaneOpen failed: %v", err)
	}
	if len(mock.Calls) != 1 || mock.Calls[0].Method != "SplitPane" {
		t.Fatalf("expected only SplitPane call, got %#v", mock.Calls)
	}
	if mock.Calls[0].Args[0] != "%99" || mock.Calls[0].Args[1] != "down" || mock.Calls[0].Args[2] != "12" {
		t.Fatalf("unexpected SplitPane args: %#v", mock.Calls[0].Args)
	}
}

func TestRunPaneOpenErrorsOutsideTmuxWithoutTarget(t *testing.T) {
	a, mock := newTestApp(t)
	mock.InsideTmux = false
	cmd := newPaneOpenCmd(a)
	flags := &paneOpenFlags{}
	if err := runPaneOpen(a, cmd, flags, nil); err == nil {
		t.Fatal("expected error outside tmux without target")
	}
}

func TestRunPaneOpenRejectsFileCWD(t *testing.T) {
	a, _ := newTestApp(t)
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := newPaneOpenCmd(a)
	flags := &paneOpenFlags{target: "%1", cwd: file}
	if err := runPaneOpen(a, cmd, flags, nil); err == nil {
		t.Fatal("expected cwd file error")
	}
}

func TestRunPaneCurrentPlain(t *testing.T) {
	a, _ := newTestApp(t)
	t.Setenv("TMUX_PANE", "%12")
	cmd := newPaneCurrentCmd(a)
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := runPaneCurrent(a, cmd, &paneCurrentFlags{}); err != nil {
		t.Fatalf("runPaneCurrent failed: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "%12" {
		t.Fatalf("expected %%12, got %q", got)
	}
}

func TestRunPaneCurrentJSON(t *testing.T) {
	a, mock := newTestApp(t)
	t.Setenv("TMUX_PANE", "%12")
	mock.Panes["%12"] = []tmux.Pane{{ID: "%12", Session: "zws_repo__main", WindowIndex: 4, WindowName: "agent", Title: "main", Command: "pi"}}
	cmd := newPaneCurrentCmd(a)
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := runPaneCurrent(a, cmd, &paneCurrentFlags{json: true}); err != nil {
		t.Fatalf("runPaneCurrent failed: %v", err)
	}
	got := map[string]any{}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("pane current --json is not valid JSON: %v\n%s", err, out.String())
	}
	// Canonical lower-camelCase schema (S-007): every tmux.Pane field via the DTO.
	assertExactJSONKeys(t, got, map[string]any{
		"paneId":      "%12",
		"session":     "zws_repo__main",
		"index":       float64(0),
		"windowIndex": float64(4),
		"windowName":  "agent",
		"active":      false,
		"command":     "pi",
		"pid":         float64(0),
		"dir":         "",
		"width":       float64(0),
		"height":      float64(0),
		"title":       "main",
	})
	// Exported-Go-name leakage must be gone (explicit DTO, no untagged marshal).
	for _, gone := range []string{"ID", "Session", "WindowName", "WindowIndex", "Dir"} {
		if _, ok := got[gone]; ok {
			t.Errorf("pane current --json leaks exported field %q:\n%s", gone, out.String())
		}
	}
	if len(mock.Calls) != 2 || mock.Calls[1].Method != "ListWindowPanes" || mock.Calls[1].Args[0] != "%12" {
		t.Fatalf("expected pane-scoped lookup for %%12, got %#v", mock.Calls)
	}
}

func TestRunPaneToggleClosesExistingByDefault(t *testing.T) {
	a, mock := newTestApp(t)
	mock.Panes[""] = []tmux.Pane{{ID: "%57", Title: "clean-ui"}}
	cmd := newPaneToggleCmd(a)
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := runPaneToggle(a, cmd, &paneToggleFlags{}, []string{"clean-ui"}); err != nil {
		t.Fatalf("runPaneToggle failed: %v", err)
	}
	if len(mock.Calls) != 2 || mock.Calls[0].Method != "ListWindowPanes" || mock.Calls[1].Method != "KillPane" || mock.Calls[1].Args[0] != "%57" {
		t.Fatalf("expected ListWindowPanes + KillPane %%57, got %#v", mock.Calls)
	}
	if strings.TrimSpace(out.String()) != "%57" {
		t.Fatalf("expected toggled pane id output, got %q", out.String())
	}
}

func TestRunPaneToggleFocusExisting(t *testing.T) {
	a, mock := newTestApp(t)
	mock.Panes[""] = []tmux.Pane{{ID: "%57", Title: "clean-ui"}}
	cmd := newPaneToggleCmd(a)
	if err := runPaneToggle(a, cmd, &paneToggleFlags{focus: true}, []string{"clean-ui"}); err != nil {
		t.Fatalf("runPaneToggle failed: %v", err)
	}
	if len(mock.Calls) != 2 || mock.Calls[1].Method != "SelectPane" || mock.Calls[1].Args[0] != "%57" {
		t.Fatalf("expected SelectPane %%57, got %#v", mock.Calls)
	}
}

func TestRunPaneToggleOpensMissing(t *testing.T) {
	a, mock := newTestApp(t)
	t.Setenv("TMUX_PANE", "%12")
	cmd := newPaneToggleCmd(a)
	flags := &paneToggleFlags{paneOpenFlags: paneOpenFlags{right: "40"}}
	_ = cmd.Flags().Set("right", "40")
	if err := runPaneToggle(a, cmd, flags, []string{"clean-ui", "bash"}); err != nil {
		t.Fatalf("runPaneToggle failed: %v", err)
	}
	if len(mock.Calls) != 4 || mock.Calls[0].Method != "ListWindowPanes" || mock.Calls[1].Method != "IsInsideTmux" || mock.Calls[2].Method != "SplitPane" || mock.Calls[3].Method != "SetPaneOption" {
		t.Fatalf("expected ListWindowPanes + IsInsideTmux + SplitPane + SetPaneOption, got %#v", mock.Calls)
	}
	if mock.Calls[2].Args[2] != "40%" || mock.Calls[2].Args[4] != "clean-ui" || mock.Calls[2].Args[5] != "[\"bash\"]" {
		t.Fatalf("unexpected SplitPane args: %#v", mock.Calls[2].Args)
	}
}

func TestRunPaneListQuiet(t *testing.T) {
	a, mock := newTestApp(t)
	mock.Panes[""] = []tmux.Pane{{ID: "%1"}, {ID: "%2"}}
	cmd := newPaneListCmd(a, "list")
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := runPaneList(a, cmd, &paneListFlags{quiet: true}); err != nil {
		t.Fatalf("runPaneList failed: %v", err)
	}
	if got := out.String(); got != "%1\n%2\n" {
		t.Fatalf("unexpected quiet list output: %q", got)
	}
}

func TestRunPaneListDefaultsToCurrentWindow(t *testing.T) {
	a, mock := newTestApp(t)
	mock.Panes[""] = []tmux.Pane{{ID: "%1"}}
	cmd := newPaneListCmd(a, "list")
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := runPaneList(a, cmd, &paneListFlags{quiet: true}); err != nil {
		t.Fatalf("runPaneList failed: %v", err)
	}
	if len(mock.Calls) != 1 || mock.Calls[0].Method != "ListWindowPanes" || mock.Calls[0].Args[0] != "" {
		t.Fatalf("expected ListWindowPanes default call, got %#v", mock.Calls)
	}
}

func TestRunPaneListSessionScope(t *testing.T) {
	a, mock := newTestApp(t)
	mock.Sessions = []tmux.Session{{Name: "dev"}}
	mock.Panes["dev"] = []tmux.Pane{{ID: "%1"}}
	cmd := newPaneListCmd(a, "list")
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := runPaneList(a, cmd, &paneListFlags{target: "dev", session: true, quiet: true}); err != nil {
		t.Fatalf("runPaneList failed: %v", err)
	}
	call, ok := srFindCall(mock.Calls, "ListPanes")
	if !ok || call.Args[0] != "dev" {
		t.Fatalf("expected ListPanes(dev) call, got %#v", mock.Calls)
	}
}

func TestRunPaneListAllScope(t *testing.T) {
	a, mock := newTestApp(t)
	mock.Panes["dev"] = []tmux.Pane{{ID: "%1"}}
	cmd := newPaneListCmd(a, "list")
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := runPaneList(a, cmd, &paneListFlags{all: true, quiet: true}); err != nil {
		t.Fatalf("runPaneList failed: %v", err)
	}
	if len(mock.Calls) != 1 || mock.Calls[0].Method != "ListAllPanes" {
		t.Fatalf("expected ListAllPanes call, got %#v", mock.Calls)
	}
}

func TestRunPaneListJoinedImpliedSessionScope(t *testing.T) {
	a, mock := newTestApp(t)
	mock.Panes[""] = []tmux.Pane{
		{Session: "dev", ID: "%1", WindowIndex: 1, Command: "bash", Dir: "/repo", Title: "host"},
		{Session: "dev", ID: "%2", WindowIndex: 1, Active: true, Command: "codex", Dir: "/repo", Title: "peer"},
		{Session: "dev", ID: "%3", WindowIndex: 2, Active: true, Command: "vim", Dir: "/repo", Title: "scratch"},
	}
	mock.LogicalRows = []tmux.LogicalPaneRow{
		{PaneID: "%1", Session: "dev", WindowID: "@1", WindowIndex: 1, WindowName: "work", WindowPanes: 2, WindowActive: true, PaneActive: false, TabID: "ztab_host", Label: "work", Command: "bash", Dir: "/repo", Title: "host"},
		{PaneID: "%2", Session: "dev", WindowID: "@1", WindowIndex: 1, WindowName: "work", WindowPanes: 2, WindowActive: true, PaneActive: true, TabID: "ztab_peer", Label: "codex-peer", Anchor: "ztab_host", Command: "codex", Dir: "/repo", Title: "peer"},
		{PaneID: "%3", Session: "dev", WindowID: "@2", WindowIndex: 2, WindowName: "scratch", WindowPanes: 1, WindowActive: false, PaneActive: true, TabID: "ztab_scratch", Label: "scratch", Command: "vim", Dir: "/repo", Title: "scratch"},
		{PaneID: "%4", Session: "other", WindowID: "@3", WindowIndex: 1, WindowName: "other", WindowPanes: 2, WindowActive: true, PaneActive: false, TabID: "ztab_other_host", Label: "other"},
		{PaneID: "%5", Session: "other", WindowID: "@3", WindowIndex: 1, WindowName: "other", WindowPanes: 2, WindowActive: true, PaneActive: true, TabID: "ztab_other_peer", Label: "other-peer", Anchor: "ztab_other_host"},
	}
	t.Setenv("TMUX_PANE", "%2")

	cmd := newPaneListCmd(a, "list")
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := runPaneList(a, cmd, &paneListFlags{joined: true, json: true}); err != nil {
		t.Fatalf("runPaneList failed: %v", err)
	}
	if len(mock.Calls) != 2 || mock.Calls[0].Method != "ListPanes" || mock.Calls[1].Method != "ListLogicalPaneRows" {
		t.Fatalf("expected session pane list + logical scan, got %#v", mock.Calls)
	}
	var rows []joinedPaneListRow
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if len(rows) != 1 {
		t.Fatalf("joined rows = %#v, want one rider", rows)
	}
	row := rows[0]
	if row.TabID != "ztab_peer" || row.TabName != "codex-peer" || row.PaneID != "%2" {
		t.Fatalf("unexpected joined tab identity: %#v", row)
	}
	if row.Session != "dev" || row.AnchorID != "ztab_host" || row.HostName != "work" || row.HostPaneID != "%1" {
		t.Fatalf("unexpected joined host/session fields: %#v", row)
	}
	if row.CWD != "/repo" || row.Command != "codex" || row.Title != "peer" || !row.Active || !row.Caller {
		t.Fatalf("unexpected joined pane facts: %#v", row)
	}
}

func TestRunPaneListJoinedExplicitTarget(t *testing.T) {
	a, mock := newTestApp(t)
	mock.Sessions = []tmux.Session{{Name: "dev"}}
	mock.Panes["dev"] = []tmux.Pane{
		{Session: "dev", ID: "%10", WindowIndex: 1, Command: "bash", Dir: "/repo", Title: "host"},
		{Session: "dev", ID: "%11", WindowIndex: 1, Active: true, Command: "codex", Dir: "/repo", Title: "peer"},
	}
	mock.LogicalRows = []tmux.LogicalPaneRow{
		{PaneID: "%10", Session: "dev", WindowID: "@10", WindowIndex: 1, WindowName: "work", WindowPanes: 2, WindowActive: true, PaneActive: false, TabID: "ztab_t_host", Label: "work", Command: "bash", Dir: "/repo", Title: "host"},
		{PaneID: "%11", Session: "dev", WindowID: "@10", WindowIndex: 1, WindowName: "work", WindowPanes: 2, WindowActive: true, PaneActive: true, TabID: "ztab_t_peer", Label: "codex-peer", Anchor: "ztab_t_host", Command: "codex", Dir: "/repo", Title: "peer"},
		// "other" session — must be excluded via byPane filter.
		{PaneID: "%20", Session: "other", WindowID: "@20", WindowIndex: 1, WindowName: "other", WindowPanes: 2, WindowActive: true, PaneActive: false, TabID: "ztab_o_host", Label: "other"},
		{PaneID: "%21", Session: "other", WindowID: "@20", WindowIndex: 1, WindowName: "other", WindowPanes: 2, WindowActive: true, PaneActive: true, TabID: "ztab_o_peer", Label: "other-peer", Anchor: "ztab_o_host"},
	}

	cmd := newPaneListCmd(a, "list")
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := runPaneList(a, cmd, &paneListFlags{joined: true, target: "dev", json: true}); err != nil {
		t.Fatalf("runPaneList failed: %v", err)
	}
	// Must call ListPanes with the explicit session, not the empty-string key.
	call, ok := srFindCall(mock.Calls, "ListPanes")
	if !ok || call.Args[0] != "dev" {
		t.Fatalf("expected ListPanes(dev) + logical scan, got %#v", mock.Calls)
	}
	var rows []joinedPaneListRow
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if len(rows) != 1 || rows[0].TabID != "ztab_t_peer" || rows[0].Session != "dev" {
		t.Fatalf("expected one dev joined row, got %#v", rows)
	}
}

func TestRunPaneListJoinedRejectsAllScope(t *testing.T) {
	a, _ := newTestApp(t)
	cmd := newPaneListCmd(a, "list")
	err := runPaneList(a, cmd, &paneListFlags{joined: true, all: true})
	if err == nil || !strings.Contains(err.Error(), "--joined cannot be combined with --all") {
		t.Fatalf("expected --joined/--all error, got %v", err)
	}
}

func TestRunPaneListMarksCallerPane(t *testing.T) {
	a, mock := newTestApp(t)
	mock.Panes[""] = []tmux.Pane{
		{ID: "%1", Title: "main", Command: "pi", Width: 100, Height: 40},
		{ID: "%2", Title: "side", Command: "bash", Width: 80, Height: 40},
	}
	t.Setenv("TMUX_PANE", "%1")
	cmd := newPaneListCmd(a, "list")
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := runPaneList(a, cmd, &paneListFlags{}); err != nil {
		t.Fatalf("runPaneList failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "CALLER") || !strings.Contains(got, "%1  you") {
		t.Fatalf("expected caller marker in list output, got:\n%s", got)
	}
}

func TestPaneCloseRawID(t *testing.T) {
	a, mock := newTestApp(t)
	cmd := newPaneCloseCmd(a)
	cmd.SetArgs([]string{"%57"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("pane close failed: %v", err)
	}
	if len(mock.Calls) != 1 || mock.Calls[0].Method != "KillPane" || mock.Calls[0].Args[0] != "%57" {
		t.Fatalf("expected KillPane %%57, got %#v", mock.Calls)
	}
}

func TestPaneCloseResolvesTitle(t *testing.T) {
	a, mock := newTestApp(t)
	mock.Panes[""] = []tmux.Pane{{ID: "%57", Title: "clean-ui"}}
	cmd := newPaneCloseCmd(a)
	cmd.SetArgs([]string{"clean-ui"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("pane close failed: %v", err)
	}
	if len(mock.Calls) != 2 || mock.Calls[0].Method != "ListWindowPanes" || mock.Calls[1].Method != "KillPane" || mock.Calls[1].Args[0] != "%57" {
		t.Fatalf("expected ListWindowPanes + KillPane %%57, got %#v", mock.Calls)
	}
}

func TestPaneCloseResolvesStablePaneNameWhenTitleDrifts(t *testing.T) {
	a, mock := newTestApp(t)
	mock.Panes[""] = []tmux.Pane{{ID: "%57", Title: "bash"}}
	mock.PaneOptions = map[string]string{"%57\x00" + optPaneName: "clean-ui"}
	cmd := newPaneCloseCmd(a)
	cmd.SetArgs([]string{"clean-ui"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("pane close failed: %v", err)
	}
	if len(mock.Calls) != 3 || mock.Calls[0].Method != "ListWindowPanes" || mock.Calls[1].Method != "ShowPaneOption" || mock.Calls[2].Method != "KillPane" || mock.Calls[2].Args[0] != "%57" {
		t.Fatalf("expected stable-name lookup + KillPane %%57, got %#v", mock.Calls)
	}
}

func TestPaneCloseAmbiguousTitle(t *testing.T) {
	a, mock := newTestApp(t)
	mock.Panes[""] = []tmux.Pane{{ID: "%57", Title: "clean-ui"}, {ID: "%58", Title: "clean-ui"}}
	cmd := newPaneCloseCmd(a)
	cmd.SetArgs([]string{"clean-ui"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected ambiguous pane error")
	}
}

func TestPaneFocusRawID(t *testing.T) {
	a, mock := newTestApp(t)
	cmd := newPaneFocusCmd(a)
	cmd.SetArgs([]string{"%57"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("pane focus failed: %v", err)
	}
	if len(mock.Calls) != 1 || mock.Calls[0].Method != "SelectPane" || mock.Calls[0].Args[0] != "%57" {
		t.Fatalf("expected SelectPane %%57, got %#v", mock.Calls)
	}
}

func TestPaneResizeRawID(t *testing.T) {
	a, mock := newTestApp(t)
	cmd := newPaneResizeCmd(a)
	cmd.SetArgs([]string{"%57", "--size", "40%"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("pane resize failed: %v", err)
	}
	if len(mock.Calls) != 1 || mock.Calls[0].Method != "ResizePane" || mock.Calls[0].Args[0] != "%57" || mock.Calls[0].Args[1] != "width" || mock.Calls[0].Args[2] != "40%" {
		t.Fatalf("expected ResizePane %%57 width 40%%, got %#v", mock.Calls)
	}
}

func TestPaneResizeResolvesStablePaneNameWhenTitleDrifts(t *testing.T) {
	a, mock := newTestApp(t)
	mock.Panes[""] = []tmux.Pane{{ID: "%57", Title: "bash"}}
	mock.PaneOptions = map[string]string{"%57\x00" + optPaneName: "clean-ui"}
	cmd := newPaneResizeCmd(a)
	cmd.SetArgs([]string{"clean-ui", "--size", "35%"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("pane resize failed: %v", err)
	}
	if len(mock.Calls) != 3 || mock.Calls[0].Method != "ListWindowPanes" || mock.Calls[1].Method != "ShowPaneOption" || mock.Calls[2].Method != "ResizePane" || mock.Calls[2].Args[0] != "%57" {
		t.Fatalf("expected stable-name lookup + ResizePane %%57, got %#v", mock.Calls)
	}
}
