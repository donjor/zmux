package main

import (
	"testing"

	"github.com/donjor/zmux/internal/tmux"
)

func TestAttachMirrorCallsPlainAttach(t *testing.T) {
	mock := withMockApp(t)
	mock.InsideTmux = false
	mock.Sessions = []tmux.Session{{Name: "dev", Attached: true}}

	mirrorFlag = true
	hijackFlag = false
	defer func() { mirrorFlag = false }()

	rootCmd.SetArgs([]string{"attach", "dev", "--mirror"})
	// This will try to actually attach (which fails in test), so we
	// just verify the mock got the right call.
	_ = rootCmd.Execute()

	found := false
	for _, c := range mock.Calls {
		if c.Method == "AttachSession" {
			found = true
		}
	}
	if !found {
		t.Error("expected plain AttachSession for mirror mode")
	}

	// Should NOT have created a grouped session.
	for _, c := range mock.Calls {
		if c.Method == "NewGroupedSession" {
			t.Error("mirror mode should not create a grouped session")
		}
	}
}

func TestAttachHijackCallsDetach(t *testing.T) {
	mock := withMockApp(t)
	mock.InsideTmux = false
	mock.Sessions = []tmux.Session{{Name: "dev", Attached: true}}

	mirrorFlag = false
	hijackFlag = true
	defer func() { hijackFlag = false }()

	rootCmd.SetArgs([]string{"attach", "dev", "--hijack"})
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

func TestAttachNonexistentSessionFails(t *testing.T) {
	_ = withMockApp(t)

	mirrorFlag = false
	hijackFlag = false

	rootCmd.SetArgs([]string{"attach", "nonexistent"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}
