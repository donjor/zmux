package tmux

import "fmt"

// SocketMode determines how tmux connects to a server socket.
type SocketMode int

const (
	SocketDefault SocketMode = iota // no flag, uses default server
	SocketNamed                     // -L <name>
	SocketPath                      // -S <path>
)

// Endpoint identifies a tmux server socket.
type Endpoint struct {
	Mode  SocketMode
	Value string // socket name or path (empty for default)
}

// DefaultEndpoint returns an endpoint for the default tmux server.
func DefaultEndpoint() Endpoint {
	return Endpoint{Mode: SocketDefault}
}

// NamedEndpoint returns an endpoint that uses tmux -L <name>.
func NamedEndpoint(name string) Endpoint {
	return Endpoint{Mode: SocketNamed, Value: name}
}

// PathEndpoint returns an endpoint that uses tmux -S <path>.
func PathEndpoint(path string) Endpoint {
	return Endpoint{Mode: SocketPath, Value: path}
}

// Args returns the tmux CLI flags for this endpoint.
// Returns nil for the default endpoint.
func (e Endpoint) Args() []string {
	switch e.Mode {
	case SocketNamed:
		return []string{"-L", e.Value}
	case SocketPath:
		return []string{"-S", e.Value}
	default:
		return nil
	}
}

// String returns a human-readable description of the endpoint.
func (e Endpoint) String() string {
	switch e.Mode {
	case SocketNamed:
		return fmt.Sprintf("socket:%s", e.Value)
	case SocketPath:
		return fmt.Sprintf("path:%s", e.Value)
	default:
		return "default"
	}
}
