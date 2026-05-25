package source

import (
	"fmt"

	"github.com/donjor/zmux/internal/tmux"
)

// ConnectFallback does a direct tmux attach when overmind is unavailable.
// It creates a client bound to the endpoint and attaches to the given session,
// optionally selecting a window first. This is a generic tmux attach, not
// overmind control (which lives in internal/overmind).
func ConnectFallback(endpoint tmux.Endpoint, session, window string) error {
	client := tmux.NewClientFor(endpoint)
	if window != "" {
		target := fmt.Sprintf("%s:%s", session, window)
		return client.AttachSession(target)
	}
	return client.AttachSession(session)
}
