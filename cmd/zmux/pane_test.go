package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donjor/zmux/internal/tmux"
)

func TestRunPaneOpenDefaultsToCurrentTmuxPaneAndPrintsID(t *testing.T) {
	mock := withMockApp(t)
	t.Setenv("TMUX_PANE", "%12")
	cmd := newPaneOpenCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)

	cwd := t.TempDir()
	flags := &paneOpenFlags{right: "40", cwd: cwd}
	cmd.Flags().Set("right", "40")
	if err := runPaneOpen(cmd, flags, []string{"clean-ui"}); err != nil {
		t.Fatalf("runPaneOpen failed: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "%57" {
		t.Fatalf("expected pane id %%57, got %q", got)
	}
	if len(mock.Calls) != 2 {
		t.Fatalf("expected IsInsideTmux + SplitPane calls, got %#v", mock.Calls)
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

func TestRunPaneOpenAutoLabelsWindowBeforeSplit(t *testing.T) {
	mock := withMockApp(t)
	mock.DisplayMessageResult = "zmux\t\tpi"
	t.Setenv("TMUX_PANE", "%12")
	cmd := newPaneOpenCmd()
	cmd.SetArgs([]string{"clean-ui", "-r", "35", "--label-tab"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("pane open command failed: %v", err)
	}
	methods := make([]string, 0, len(mock.Calls))
	for _, call := range mock.Calls {
		methods = append(methods, call.Method)
	}
	want := []string{"IsInsideTmux", "DisplayMessage", "SetWindowOption", "SetWindowOption", "SplitPane"}
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
	mock := withMockApp(t)
	t.Setenv("TMUX_PANE", "%12")
	cmd := newPaneOpenCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"clean-ui", "-r", "35", "--", "bash", "-lc", "echo hi && sleep 1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("pane open command failed: %v", err)
	}
	call := mock.Calls[len(mock.Calls)-1]
	if call.Method != "SplitPane" {
		t.Fatalf("expected SplitPane call, got %#v", call)
	}
	if call.Args[2] != "35%" || call.Args[4] != "clean-ui" || call.Args[5] != "[\"bash\" \"-lc\" \"echo hi && sleep 1\"]" {
		t.Fatalf("unexpected SplitPane args: %#v", call.Args)
	}
}

func TestPaneOpenNameFlagLeavesArgsAsCommand(t *testing.T) {
	mock := withMockApp(t)
	t.Setenv("TMUX_PANE", "%12")
	cmd := newPaneOpenCmd()
	cmd.SetArgs([]string{"-n", "clean-ui", "-r", "35", "--", "bash", "-lc", "echo hi"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("pane open command failed: %v", err)
	}
	call := mock.Calls[len(mock.Calls)-1]
	if call.Args[4] != "clean-ui" || call.Args[5] != "[\"bash\" \"-lc\" \"echo hi\"]" {
		t.Fatalf("unexpected SplitPane args: %#v", call.Args)
	}
}

func TestRunPaneOpenAllowsExplicitTargetOutsideTmux(t *testing.T) {
	mock := withMockApp(t)
	mock.InsideTmux = false
	cmd := newPaneOpenCmd()
	flags := &paneOpenFlags{target: "%99", down: paneAutoSize, size: "12"}
	cmd.Flags().Set("down", paneAutoSize)
	if err := runPaneOpen(cmd, flags, nil); err != nil {
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
	mock := withMockApp(t)
	mock.InsideTmux = false
	cmd := newPaneOpenCmd()
	flags := &paneOpenFlags{}
	if err := runPaneOpen(cmd, flags, nil); err == nil {
		t.Fatal("expected error outside tmux without target")
	}
}

func TestRunPaneOpenRejectsFileCWD(t *testing.T) {
	withMockApp(t)
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := newPaneOpenCmd()
	flags := &paneOpenFlags{target: "%1", cwd: file}
	if err := runPaneOpen(cmd, flags, nil); err == nil {
		t.Fatal("expected cwd file error")
	}
}

func TestRunPaneCurrentPlain(t *testing.T) {
	withMockApp(t)
	t.Setenv("TMUX_PANE", "%12")
	cmd := newPaneCurrentCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := runPaneCurrent(cmd, &paneCurrentFlags{}); err != nil {
		t.Fatalf("runPaneCurrent failed: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "%12" {
		t.Fatalf("expected %%12, got %q", got)
	}
}

func TestRunPaneCurrentJSON(t *testing.T) {
	mock := withMockApp(t)
	t.Setenv("TMUX_PANE", "%12")
	mock.Panes[""] = []tmux.Pane{{ID: "%12", Title: "main", Command: "pi"}}
	cmd := newPaneCurrentCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := runPaneCurrent(cmd, &paneCurrentFlags{json: true}); err != nil {
		t.Fatalf("runPaneCurrent failed: %v", err)
	}
	if !strings.Contains(out.String(), `"ID": "%12"`) || !strings.Contains(out.String(), `"Title": "main"`) {
		t.Fatalf("expected current pane JSON, got %s", out.String())
	}
}

func TestRunPaneToggleClosesExistingByDefault(t *testing.T) {
	mock := withMockApp(t)
	mock.Panes[""] = []tmux.Pane{{ID: "%57", Title: "clean-ui"}}
	cmd := newPaneToggleCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := runPaneToggle(cmd, &paneToggleFlags{}, []string{"clean-ui"}); err != nil {
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
	mock := withMockApp(t)
	mock.Panes[""] = []tmux.Pane{{ID: "%57", Title: "clean-ui"}}
	cmd := newPaneToggleCmd()
	if err := runPaneToggle(cmd, &paneToggleFlags{focus: true}, []string{"clean-ui"}); err != nil {
		t.Fatalf("runPaneToggle failed: %v", err)
	}
	if len(mock.Calls) != 2 || mock.Calls[1].Method != "SelectPane" || mock.Calls[1].Args[0] != "%57" {
		t.Fatalf("expected SelectPane %%57, got %#v", mock.Calls)
	}
}

func TestRunPaneToggleOpensMissing(t *testing.T) {
	mock := withMockApp(t)
	t.Setenv("TMUX_PANE", "%12")
	cmd := newPaneToggleCmd()
	flags := &paneToggleFlags{paneOpenFlags: paneOpenFlags{right: "40"}}
	cmd.Flags().Set("right", "40")
	if err := runPaneToggle(cmd, flags, []string{"clean-ui", "bash"}); err != nil {
		t.Fatalf("runPaneToggle failed: %v", err)
	}
	if len(mock.Calls) != 3 || mock.Calls[0].Method != "ListWindowPanes" || mock.Calls[1].Method != "IsInsideTmux" || mock.Calls[2].Method != "SplitPane" {
		t.Fatalf("expected ListWindowPanes + IsInsideTmux + SplitPane, got %#v", mock.Calls)
	}
	if mock.Calls[2].Args[2] != "40%" || mock.Calls[2].Args[4] != "clean-ui" || mock.Calls[2].Args[5] != "[\"bash\"]" {
		t.Fatalf("unexpected SplitPane args: %#v", mock.Calls[2].Args)
	}
}

func TestRunPaneListQuiet(t *testing.T) {
	mock := withMockApp(t)
	mock.Panes[""] = []tmux.Pane{{ID: "%1"}, {ID: "%2"}}
	cmd := newPaneListCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := runPaneList(cmd, &paneListFlags{quiet: true}); err != nil {
		t.Fatalf("runPaneList failed: %v", err)
	}
	if got := out.String(); got != "%1\n%2\n" {
		t.Fatalf("unexpected quiet list output: %q", got)
	}
}

func TestRunPaneListDefaultsToCurrentWindow(t *testing.T) {
	mock := withMockApp(t)
	mock.Panes[""] = []tmux.Pane{{ID: "%1"}}
	cmd := newPaneListCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := runPaneList(cmd, &paneListFlags{quiet: true}); err != nil {
		t.Fatalf("runPaneList failed: %v", err)
	}
	if len(mock.Calls) != 1 || mock.Calls[0].Method != "ListWindowPanes" || mock.Calls[0].Args[0] != "" {
		t.Fatalf("expected ListWindowPanes default call, got %#v", mock.Calls)
	}
}

func TestRunPaneListSessionScope(t *testing.T) {
	mock := withMockApp(t)
	mock.Panes["dev"] = []tmux.Pane{{ID: "%1"}}
	cmd := newPaneListCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := runPaneList(cmd, &paneListFlags{target: "dev", session: true, quiet: true}); err != nil {
		t.Fatalf("runPaneList failed: %v", err)
	}
	if len(mock.Calls) != 1 || mock.Calls[0].Method != "ListPanes" || mock.Calls[0].Args[0] != "dev" {
		t.Fatalf("expected ListPanes session call, got %#v", mock.Calls)
	}
}

func TestRunPaneListAllScope(t *testing.T) {
	mock := withMockApp(t)
	mock.Panes["dev"] = []tmux.Pane{{ID: "%1"}}
	cmd := newPaneListCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := runPaneList(cmd, &paneListFlags{all: true, quiet: true}); err != nil {
		t.Fatalf("runPaneList failed: %v", err)
	}
	if len(mock.Calls) != 1 || mock.Calls[0].Method != "ListAllPanes" {
		t.Fatalf("expected ListAllPanes call, got %#v", mock.Calls)
	}
}

func TestRunPaneListMarksCallerPane(t *testing.T) {
	mock := withMockApp(t)
	mock.Panes[""] = []tmux.Pane{
		{ID: "%1", Title: "main", Command: "pi", Width: 100, Height: 40},
		{ID: "%2", Title: "side", Command: "bash", Width: 80, Height: 40},
	}
	t.Setenv("TMUX_PANE", "%1")
	cmd := newPaneListCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := runPaneList(cmd, &paneListFlags{}); err != nil {
		t.Fatalf("runPaneList failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "CALLER") || !strings.Contains(got, "%1  you") {
		t.Fatalf("expected caller marker in list output, got:\n%s", got)
	}
}

func TestPaneCloseRawID(t *testing.T) {
	mock := withMockApp(t)
	cmd := newPaneCloseCmd()
	cmd.SetArgs([]string{"%57"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("pane close failed: %v", err)
	}
	if len(mock.Calls) != 1 || mock.Calls[0].Method != "KillPane" || mock.Calls[0].Args[0] != "%57" {
		t.Fatalf("expected KillPane %%57, got %#v", mock.Calls)
	}
}

func TestPaneCloseResolvesTitle(t *testing.T) {
	mock := withMockApp(t)
	mock.Panes[""] = []tmux.Pane{{ID: "%57", Title: "clean-ui"}}
	cmd := newPaneCloseCmd()
	cmd.SetArgs([]string{"clean-ui"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("pane close failed: %v", err)
	}
	if len(mock.Calls) != 2 || mock.Calls[0].Method != "ListWindowPanes" || mock.Calls[1].Method != "KillPane" || mock.Calls[1].Args[0] != "%57" {
		t.Fatalf("expected ListWindowPanes + KillPane %%57, got %#v", mock.Calls)
	}
}

func TestPaneCloseAmbiguousTitle(t *testing.T) {
	mock := withMockApp(t)
	mock.Panes[""] = []tmux.Pane{{ID: "%57", Title: "clean-ui"}, {ID: "%58", Title: "clean-ui"}}
	cmd := newPaneCloseCmd()
	cmd.SetArgs([]string{"clean-ui"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected ambiguous pane error")
	}
}

func TestPaneFocusRawID(t *testing.T) {
	mock := withMockApp(t)
	cmd := newPaneFocusCmd()
	cmd.SetArgs([]string{"%57"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("pane focus failed: %v", err)
	}
	if len(mock.Calls) != 1 || mock.Calls[0].Method != "SelectPane" || mock.Calls[0].Args[0] != "%57" {
		t.Fatalf("expected SelectPane %%57, got %#v", mock.Calls)
	}
}

func TestPaneResizeRawID(t *testing.T) {
	mock := withMockApp(t)
	cmd := newPaneResizeCmd()
	cmd.SetArgs([]string{"%57", "--size", "40%"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("pane resize failed: %v", err)
	}
	if len(mock.Calls) != 1 || mock.Calls[0].Method != "ResizePane" || mock.Calls[0].Args[0] != "%57" || mock.Calls[0].Args[1] != "width" || mock.Calls[0].Args[2] != "40%" {
		t.Fatalf("expected ResizePane %%57 width 40%%, got %#v", mock.Calls)
	}
}
