package source

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/donjor/zmux/internal/tmux"
)

// probeTimeout is the maximum time to wait for a socket to respond.
const probeTimeout = 2 * time.Second

// processEntry represents a single row from the process table.
type processEntry struct {
	PID  int
	PPID int
	Args string
}

// socketInfo represents a discovered tmux socket file.
type socketInfo struct {
	Name string // filename in the tmux socket directory
	Path string // full path to the socket file
}

// Discover scans for tmux sockets and returns a Catalog of all sessions across
// the active profile's server (local) and external servers, using real host I/O.
// local is the endpoint of the invoking binary's own server (default for zmux,
// -L zzmux for the edge profile) so that server is treated as "local" and every
// other server — including the live zmux server when running as zzmux — shows up
// as external rather than being silently attached.
func Discover(local tmux.Endpoint) (*Catalog, error) {
	return discoverWith(systemProber{}, local)
}

// localSocketName returns the socket basename for an endpoint: "default" for the
// default server, the -L name, or the -S path basename.
func localSocketName(ep tmux.Endpoint) string {
	switch ep.Mode {
	case tmux.SocketNamed:
		return ep.Value
	case tmux.SocketPath:
		return filepath.Base(ep.Value)
	default:
		return "default"
	}
}

// discoverWith is the orchestration core, parameterized over a prober for
// testability. It performs:
//  1. Local default-server sessions
//  2. Socket directory scan
//  3. Single ps call for process correlation + overmind correlation
//  4. Live probe of each candidate socket
//
// Errors in external discovery are handled gracefully; the local catalog
// is always populated when possible.
func discoverWith(p prober, local tmux.Endpoint) (*Catalog, error) {
	cat := &Catalog{}

	// Always populate the active profile's own server first.
	if localEntries, ok := p.localSessions(local); ok {
		cat.Local = localEntries
	}

	// Discover external sockets.
	sockets, err := p.listSockets()
	if err != nil {
		// Socket scan failed — return local-only catalog.
		return cat, nil
	}

	// Exclude the local server's own socket so it isn't double-listed as
	// external (for zmux that's "default"; for zzmux that's "zzmux").
	localSock := localSocketName(local)
	filtered := sockets[:0]
	for _, s := range sockets {
		if s.Name != localSock {
			filtered = append(filtered, s)
		}
	}
	sockets = filtered

	// Build process table for correlation.
	procs, psErr := p.processTable()

	// Correlate sockets to known owners.
	var sources []Source
	if psErr == nil {
		sources = correlateSources(sockets, procs)
	} else {
		// ps failed — treat all non-default sockets as generic external.
		for _, sock := range sockets {
			sources = append(sources, Source{
				ID:       sock.Name,
				Kind:     SourceExternal,
				Label:    sock.Name,
				Health:   HealthOK,
				Endpoint: tmux.NamedEndpoint(sock.Name),
			})
		}
	}

	// Probe each source and collect sessions.
	for _, src := range sources {
		entries, health, probeErr := p.probeSocket(src.Endpoint)
		src.Health = health
		if probeErr != nil {
			src.Error = probeErr.Error()
		}
		if health == HealthStale {
			continue // skip dead sockets
		}

		// Attach source ref to each entry.
		srcCopy := src
		for i := range entries {
			entries[i].Source = &srcCopy
		}

		cat.External = append(cat.External, SourceGroup{
			Source:  srcCopy,
			Entries: entries,
		})
	}

	return cat, nil
}

// findTmuxSockets scans the tmux socket directory for non-default sockets.
// It checks $TMUX_TMPDIR first, then falls back to /tmp/tmux-<uid>/.
func findTmuxSockets() ([]socketInfo, error) {
	dir := os.Getenv("TMUX_TMPDIR")
	if dir == "" {
		dir = fmt.Sprintf("/tmp/tmux-%d", os.Getuid())
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read socket dir %s: %w", dir, err)
	}

	var sockets []socketInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		// The active profile's own socket is excluded by discoverWith (which
		// knows the local endpoint); everything else — including the default
		// "zmux" server when running as zzmux — is a candidate external source.
		sockets = append(sockets, socketInfo{
			Name: name,
			Path: filepath.Join(dir, name),
		})
	}
	return sockets, nil
}

// buildProcessTable runs a single ps call and returns all process entries.
func buildProcessTable() ([]processEntry, error) {
	out, err := exec.Command("ps", "-eo", "pid,ppid,args", "--no-headers").Output()
	if err != nil {
		return nil, fmt.Errorf("ps: %w", err)
	}
	return parseProcessTable(string(out)), nil
}

// parseProcessTable parses ps output into process entries.
func parseProcessTable(output string) []processEntry {
	var entries []processEntry
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		ppid, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		entries = append(entries, processEntry{
			PID:  pid,
			PPID: ppid,
			Args: strings.Join(fields[2:], " "),
		})
	}
	return entries
}

// correlateSources matches sockets to known owners using the process table.
func correlateSources(sockets []socketInfo, procs []processEntry) []Source {
	// Index overmind processes by their socket name.
	// Overmind creates sockets named "overmind-<hash>" and passes -s <control>.
	overmindProcs := findOvermindProcesses(procs)

	var sources []Source
	for _, sock := range sockets {
		if om, ok := overmindProcs[sock.Name]; ok {
			sources = append(sources, Source{
				ID:       sock.Name,
				Kind:     SourceOvermind,
				Label:    overmindLabel(om),
				Health:   HealthOK,
				Endpoint: tmux.NamedEndpoint(sock.Name),
				Overmind: om,
			})
		} else {
			sources = append(sources, Source{
				ID:       sock.Name,
				Kind:     SourceExternal,
				Label:    sock.Name,
				Health:   HealthOK,
				Endpoint: tmux.NamedEndpoint(sock.Name),
			})
		}
	}
	return sources
}

// findOvermindProcesses scans the process table for overmind start commands
// and extracts their socket names and flags.
func findOvermindProcesses(procs []processEntry) map[string]*OvermindMeta {
	result := make(map[string]*OvermindMeta)
	for _, p := range procs {
		// Look for "overmind start" or "overmind s" commands.
		if !isOvermindStart(p.Args) {
			continue
		}

		meta := &OvermindMeta{}
		args := strings.Fields(p.Args)
		for i := 0; i < len(args)-1; i++ {
			switch args[i] {
			case "-s":
				meta.ControlSocket = args[i+1]
			case "-f":
				meta.Procfile = args[i+1]
			}
		}

		// Derive the tmux socket name from the control socket path.
		// Overmind uses the tmux socket name based on its own socket.
		// The tmux socket name typically matches the overmind socket basename
		// or follows the pattern overmind-<hash>.
		sockName := deriveSocketName(p, meta)
		if sockName != "" {
			result[sockName] = meta
		}
	}
	return result
}

// isOvermindStart returns true if the command line looks like an overmind start.
func isOvermindStart(args string) bool {
	fields := strings.Fields(args)
	for i, f := range fields {
		base := filepath.Base(f)
		if base == "overmind" && i+1 < len(fields) {
			subcmd := fields[i+1]
			return subcmd == "start" || subcmd == "s"
		}
	}
	return false
}

// deriveSocketName attempts to find the tmux socket name that overmind created.
// Overmind names its tmux sockets using a pattern tied to the control socket.
func deriveSocketName(proc processEntry, meta *OvermindMeta) string {
	if meta.ControlSocket == "" {
		return ""
	}
	// Overmind's tmux socket name is typically the basename of the control
	// socket directory or a hash-based name. We look for sockets that start
	// with "overmind" since that's the naming convention.
	base := filepath.Base(meta.ControlSocket)
	// Strip the .sock extension if present.
	base = strings.TrimSuffix(base, ".sock")
	return base
}

// overmindLabel generates a human-readable label for an overmind source.
func overmindLabel(meta *OvermindMeta) string {
	if meta.Procfile != "" {
		dir := filepath.Dir(meta.Procfile)
		return filepath.Base(dir)
	}
	if meta.ControlSocket != "" {
		return filepath.Base(meta.ControlSocket)
	}
	return "overmind"
}

// probeSocket verifies a socket is live and returns its sessions.
// Uses a context with timeout to avoid hanging on dead sockets.
func probeSocket(ep tmux.Endpoint) ([]CatalogEntry, SourceHealth, error) {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()

	args := append(ep.Args(), "list-sessions", "-F",
		"#{session_name}\t#{session_windows}\t#{session_attached}")
	cmd := exec.CommandContext(ctx, "tmux", args...)
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, HealthStale, fmt.Errorf("probe timed out after %s", probeTimeout)
		}
		return nil, HealthStale, fmt.Errorf("probe failed: %w", err)
	}

	entries := parseProbeOutput(string(out))
	return entries, HealthOK, nil
}

// parseProbeOutput parses the output of tmux list-sessions from a probe.
func parseProbeOutput(output string) []CatalogEntry {
	var entries []CatalogEntry
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) < 3 {
			continue
		}
		windows, _ := strconv.Atoi(fields[1])
		attached := fields[2] == "1"
		entries = append(entries, CatalogEntry{
			Session:  fields[0],
			Windows:  windows,
			Attached: attached,
		})
	}
	return entries
}
