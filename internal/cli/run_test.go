package cli

import (
	"testing"

	"github.com/donjor/zmux/internal/tmux"
)

func TestRunCreatesNewWindow(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}

	rootCmd.SetArgs([]string{"run", "npm run dev", "-n", "server", "-s", "test-session", "-d"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("run command failed: %v", err)
	}

	found := map[string]bool{}
	for _, c := range mock.Calls {
		found[c.Method] = true
	}
	if !found["NewWindow"] {
		t.Error("expected NewWindow call")
	}
	if !found["SendKeys"] {
		t.Error("expected SendKeys call")
	}
}

func TestRunReusesExistingWindow(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.Windows["test-session"] = []tmux.Window{
		{Index: 1, Name: "server", Active: true},
	}

	rootCmd.SetArgs([]string{"run", "npm run dev", "-n", "server", "-s", "test-session", "-d"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("run command failed: %v", err)
	}

	for _, c := range mock.Calls {
		if c.Method == "NewWindow" {
			t.Error("should not create new window when it already exists")
		}
	}
}

func TestRunDerivesNameFromCommand(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}

	rootCmd.SetArgs([]string{"run", "npm run dev", "-s", "test-session", "-d"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("run command failed: %v", err)
	}

	for _, c := range mock.Calls {
		if c.Method == "NewWindow" && len(c.Args) >= 2 {
			if c.Args[1] != "npm" {
				t.Errorf("expected window name 'npm', got %q", c.Args[1])
			}
		}
	}
}

func TestRunWaitDetectsSentinel(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	// Mock CapturePane to return output with the sentinel.
	mock.CapturedPaneContent = "building...\ndone\n:::AGENT_DONE 0:::\n"

	rootCmd.SetArgs([]string{"run", "make build", "-n", "build", "-s", "test-session", "-T", "5"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("run --wait should succeed when sentinel found: %v", err)
	}
}

func TestRunWaitDetectsNonZeroExit(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.Sessions = []tmux.Session{{Name: "test-session", Windows: 1}}
	mock.CapturedPaneContent = "error: build failed\n:::AGENT_DONE 1:::\n"

	rootCmd.SetArgs([]string{"run", "make build", "-n", "build", "-s", "test-session", "-T", "5"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for non-zero exit code")
	}
}

func TestRunFailsWithoutSession(t *testing.T) {
	rootCmd, mock := withMockApp(t)
	mock.InsideTmux = false

	rootCmd.SetArgs([]string{"run", "echo hello"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when not in tmux and no --session")
	}
}

func TestRunFailsSessionNotExist(t *testing.T) {
	rootCmd, _ := withMockApp(t)

	rootCmd.SetArgs([]string{"run", "echo hello", "-s", "nonexistent"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when session doesn't exist")
	}
}
