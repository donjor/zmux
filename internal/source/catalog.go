// Package source provides discovery and management of tmux sessions across
// multiple server sockets, including overmind-managed processes.
package source

import "github.com/donjor/zmux/internal/tmux"

// SourceKind identifies the type of tmux server owner.
type SourceKind string

const (
	SourceLocal    SourceKind = "local"    // default tmux server
	SourceOvermind SourceKind = "overmind" // managed by overmind
	SourceExternal SourceKind = "external" // unknown owner
)

// SourceHealth indicates the reachability of a source.
type SourceHealth string

const (
	HealthOK       SourceHealth = "ok"       // socket responds, owner alive
	HealthDegraded SourceHealth = "degraded" // owner gone, tmux still alive
	HealthStale    SourceHealth = "stale"    // socket unreachable
)

// Source represents a tmux server instance identified by its socket.
type Source struct {
	ID       string
	Kind     SourceKind
	Label    string
	Health   SourceHealth
	Endpoint tmux.Endpoint
	Error    string // non-empty when Health != HealthOK

	// Overmind is non-nil only for SourceOvermind sources.
	Overmind *OvermindMeta
}

// OvermindMeta holds overmind-specific metadata for a source.
type OvermindMeta struct {
	ControlSocket string // -s flag value (overmind control socket path)
	Procfile      string // -f flag value (Procfile path, may be empty)
}

// CatalogEntry is a single tmux session within a source.
type CatalogEntry struct {
	Source   *Source
	Session  string // tmux session name on that socket
	Windows  int
	Attached bool
}

// Catalog holds all discovered tmux sessions grouped by source.
type Catalog struct {
	Local    []CatalogEntry // sessions on the default tmux server
	External []SourceGroup  // sessions grouped by external source
}

// SourceGroup groups catalog entries under a single external source.
type SourceGroup struct {
	Source  Source
	Entries []CatalogEntry
}

// GroupKey returns a stable key for an external source group. It avoids
// index-based IDs (which would break cursor + expansion state across a
// re-discover) by using a source-specific identifier: the overmind control
// socket for overmind sources, the source ID (socket path) otherwise.
func GroupKey(g *SourceGroup) string {
	if g == nil {
		return "unknown"
	}
	if g.Source.Overmind != nil && g.Source.Overmind.ControlSocket != "" {
		return g.Source.Overmind.ControlSocket
	}
	if g.Source.ID != "" {
		return g.Source.ID
	}
	return string(g.Source.Kind)
}
