package tmux

import (
	"reflect"
	"testing"
)

func TestBuildSplitPaneArgsRightSizeCWDCommand(t *testing.T) {
	args, err := buildSplitPaneArgs(SplitPaneOptions{
		Target:    "%1",
		Direction: SplitRight,
		Size:      "40%",
		CWD:       "/tmp/project",
		Command:   []string{"bash", "-lc", "echo hi && sleep 1"},
	})
	if err != nil {
		t.Fatalf("buildSplitPaneArgs failed: %v", err)
	}
	want := []string{"split-window", "-P", "-F", "#{pane_id}", "-h", "-l", "40%", "-c", "/tmp/project", "-t", "%1", "bash -lc 'echo hi && sleep 1'"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args mismatch\n got: %#v\nwant: %#v", args, want)
	}
}

func TestBuildSplitPaneArgsLeftUsesBeforeFlag(t *testing.T) {
	args, err := buildSplitPaneArgs(SplitPaneOptions{Direction: SplitLeft, Size: "80"})
	if err != nil {
		t.Fatalf("buildSplitPaneArgs failed: %v", err)
	}
	want := []string{"split-window", "-P", "-F", "#{pane_id}", "-h", "-b", "-l", "80"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args mismatch\n got: %#v\nwant: %#v", args, want)
	}
}

func TestBuildSplitPaneArgsDetachedUsesNoFocusFlag(t *testing.T) {
	args, err := buildSplitPaneArgs(SplitPaneOptions{Direction: SplitDown, Detached: true})
	if err != nil {
		t.Fatalf("buildSplitPaneArgs failed: %v", err)
	}
	want := []string{"split-window", "-d", "-P", "-F", "#{pane_id}", "-v"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args mismatch\n got: %#v\nwant: %#v", args, want)
	}
}

func TestShellCommandQuotesSingleQuotes(t *testing.T) {
	got := shellCommand([]string{"bash", "-lc", "printf '%s\\n' hi"})
	want := "bash -lc 'printf '\\''%s\\n'\\'' hi'"
	if got != want {
		t.Fatalf("shell command mismatch\n got: %q\nwant: %q", got, want)
	}
}
