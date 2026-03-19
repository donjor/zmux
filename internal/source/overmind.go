package source

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/donjor/zmux/internal/tmux"
)

// Connect attaches to an overmind process, taking over the terminal.
// Uses the overmind CLI with the -s flag for the control socket.
func Connect(controlSocket, process string) error {
	cmd := exec.Command("overmind", "connect", process, "-s", controlSocket)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Restart restarts an overmind process.
func Restart(controlSocket, process string) error {
	cmd := exec.Command("overmind", "restart", process, "-s", controlSocket)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Stop stops a single overmind process.
func Stop(controlSocket, process string) error {
	cmd := exec.Command("overmind", "stop", process, "-s", controlSocket)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// StopAll stops all overmind processes managed by the given control socket.
func StopAll(controlSocket string) error {
	cmd := exec.Command("overmind", "stop", "-s", controlSocket)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Logs captures recent output from an overmind process using overmind echo.
func Logs(controlSocket, process string) (string, error) {
	out, err := exec.Command("overmind", "echo", process, "-s", controlSocket).Output()
	if err != nil {
		return "", fmt.Errorf("overmind echo %s: %w", process, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ConnectFallback does a direct tmux attach when overmind is unavailable.
// It creates a client bound to the endpoint and attaches to the given session,
// optionally selecting a window first.
func ConnectFallback(endpoint tmux.Endpoint, session, window string) error {
	client := tmux.NewClientFor(endpoint)
	if window != "" {
		target := fmt.Sprintf("%s:%s", session, window)
		return client.AttachSession(target)
	}
	return client.AttachSession(session)
}
