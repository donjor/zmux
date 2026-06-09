package source

import (
	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tmux"
)

// prober is the host-I/O seam for discovery. Discover orchestrates over it; the
// production impl (systemProber) performs real filesystem/ps/tmux calls, while
// tests substitute a deterministic fake. It mirrors the bar.Prober pattern.
//
// The interface is unexported by design: its signatures use package-private
// types (socketInfo, processEntry), so it can't be a public extension point —
// keeping it internal is honest. External callers use the exported Discover().
type prober interface {
	// listSockets scans the tmux socket directory for non-default sockets.
	listSockets() ([]socketInfo, error)
	// processTable returns the system process table for socket correlation.
	processTable() ([]processEntry, error)
	// probeSocket live-probes a candidate socket and returns its sessions.
	probeSocket(ep tmux.Endpoint) ([]CatalogEntry, SourceHealth, error)
	// localSessions returns the sessions on the active profile's server (the
	// given local endpoint) and whether that server is running.
	localSessions(local tmux.Endpoint) ([]CatalogEntry, bool)
}

// systemProber is the production prober — it performs real host I/O via the
// package helpers in discover.go.
type systemProber struct{}

var _ prober = systemProber{}

func (systemProber) listSockets() ([]socketInfo, error) { return findTmuxSockets() }

func (systemProber) processTable() ([]processEntry, error) { return buildProcessTable() }

func (systemProber) probeSocket(ep tmux.Endpoint) ([]CatalogEntry, SourceHealth, error) {
	return probeSocket(ep)
}

func (systemProber) localSessions(local tmux.Endpoint) ([]CatalogEntry, bool) {
	client := tmux.NewClientFor(local)
	if !client.ServerRunning() {
		return nil, false
	}
	sessions, err := client.ListSessions()
	if err != nil {
		return nil, true
	}
	localSource := &Source{
		ID:       "local",
		Kind:     SourceLocal,
		Label:    localSocketName(local),
		Health:   HealthOK,
		Endpoint: local,
	}
	entries := make([]CatalogEntry, 0, len(sessions))
	for _, s := range sessions {
		if tabs.IsReservedSession(s.Name) {
			continue // zmux-internal (hidden-tab dock) — never catalog
		}
		entries = append(entries, CatalogEntry{
			Source:   localSource,
			Session:  s.Name,
			Windows:  s.Windows,
			Attached: s.Attached,
		})
	}
	return entries, true
}
