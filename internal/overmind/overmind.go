// Package overmind is a control client for overmind-managed process groups.
// It drives the `overmind` CLI against a control socket — connect, restart,
// stop, and log capture. Discovery of overmind sources (scanning the process
// table, correlating sockets) is a separate concern and lives in
// internal/source.
package overmind

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Client controls overmind processes addressed by their control socket. The
// default implementation (CLI) shells out to the overmind binary; tests and
// (post-DI) the app composition root can substitute a fake.
type Client interface {
	Connect(controlSocket, process string) error
	Restart(controlSocket, process string) error
	Stop(controlSocket, process string) error
	StopAll(controlSocket string) error
	Logs(controlSocket, process string) (string, error)
}

// CLI is the default Client, backed by the `overmind` command-line binary.
type CLI struct{}

var _ Client = CLI{}

// Connect attaches to an overmind process, taking over the terminal.
func (CLI) Connect(controlSocket, process string) error {
	cmd := exec.Command("overmind", "connect", process, "-s", controlSocket)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Restart restarts an overmind process.
func (CLI) Restart(controlSocket, process string) error {
	cmd := exec.Command("overmind", "restart", process, "-s", controlSocket)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Stop stops a single overmind process.
func (CLI) Stop(controlSocket, process string) error {
	cmd := exec.Command("overmind", "stop", process, "-s", controlSocket)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// StopAll stops all overmind processes managed by the given control socket.
func (CLI) StopAll(controlSocket string) error {
	cmd := exec.Command("overmind", "stop", "-s", controlSocket)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Logs captures recent output from an overmind process using overmind echo.
func (CLI) Logs(controlSocket, process string) (string, error) {
	out, err := exec.Command("overmind", "echo", process, "-s", controlSocket).Output()
	if err != nil {
		return "", fmt.Errorf("overmind echo %s: %w", process, err)
	}
	return strings.TrimSpace(string(out)), nil
}
