package cli

import (
	"testing"

	"github.com/donjor/zmux/internal/tmux"
)

func TestOpenAttachAlias(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.InsideTmux = false
	mock.Sessions = []tmux.Session{{Name: "dev"}}

	// `zmux attach dev` should work as alias for `zmux open dev`.
	rootCmd.SetArgs([]string{"attach", "dev"})
	_ = rootCmd.Execute()

	// Should have attempted to attach.
	found := false
	for _, c := range mock.Calls {
		if c.Method == "AttachSession" || c.Method == "SwitchClient" || c.Method == "NewGroupedSession" {
			found = true
		}
	}
	if !found {
		t.Error("expected attach attempt via open alias")
	}
}

func TestOpenHijackFlag(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.InsideTmux = false
	mock.Sessions = []tmux.Session{{Name: "dev", Attached: true}}

	rootCmd.SetArgs([]string{"open", "dev", "--hijack"})
	_ = rootCmd.Execute()

	found := false
	for _, c := range mock.Calls {
		if c.Method == "AttachSessionDetach" {
			found = true
		}
	}
	if !found {
		t.Error("expected AttachSessionDetach for hijack mode")
	}
}

func TestOpenNonexistentFails(t *testing.T) {
	rootCmd, _ := withMockApp(t)

	rootCmd.SetArgs([]string{"open", "nonexistent"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent workspace/session")
	}
}
