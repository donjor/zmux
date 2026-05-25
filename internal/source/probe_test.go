package source

import (
	"errors"
	"testing"

	"github.com/donjor/zmux/internal/tmux"
)

// fakeProber is a deterministic prober for exercising discoverWith orchestration
// without touching the host. Mirrors bar/probe_test.go's fakeProber.
type fakeProber struct {
	sockets    []socketInfo
	socketsErr error
	procs      []processEntry
	procsErr   error
	local      []CatalogEntry
	localOK    bool
	probeFn    func(ep tmux.Endpoint) ([]CatalogEntry, SourceHealth, error)
}

func (f fakeProber) listSockets() ([]socketInfo, error)    { return f.sockets, f.socketsErr }
func (f fakeProber) processTable() ([]processEntry, error) { return f.procs, f.procsErr }

func (f fakeProber) localSessions(tmux.Endpoint) ([]CatalogEntry, bool) {
	return f.local, f.localOK
}

func (f fakeProber) probeSocket(ep tmux.Endpoint) ([]CatalogEntry, SourceHealth, error) {
	if f.probeFn != nil {
		return f.probeFn(ep)
	}
	return nil, HealthOK, nil
}

func okProbe(entries ...CatalogEntry) func(tmux.Endpoint) ([]CatalogEntry, SourceHealth, error) {
	return func(tmux.Endpoint) ([]CatalogEntry, SourceHealth, error) {
		return entries, HealthOK, nil
	}
}

func TestDiscoverWith_LocalAndOvermind(t *testing.T) {
	p := fakeProber{
		local:   []CatalogEntry{{Session: "main", Windows: 2, Attached: true}},
		localOK: true,
		sockets: []socketInfo{{Name: "overmind-abc", Path: "/tmp/tmux-1000/overmind-abc"}},
		// Control socket basename (minus .sock) must match the socket name so
		// correlateSources tags it SourceOvermind.
		procs:   []processEntry{{PID: 100, PPID: 1, Args: "overmind start -s /tmp/overmind-abc.sock -f /proj/Procfile"}},
		probeFn: okProbe(CatalogEntry{Session: "web", Windows: 1}),
	}

	cat, err := discoverWith(p, tmux.DefaultEndpoint())
	if err != nil {
		t.Fatalf("discoverWith: %v", err)
	}
	if len(cat.Local) != 1 || cat.Local[0].Session != "main" {
		t.Fatalf("local = %#v, want one 'main'", cat.Local)
	}
	if len(cat.External) != 1 {
		t.Fatalf("external groups = %d, want 1", len(cat.External))
	}
	g := cat.External[0]
	if g.Source.Kind != SourceOvermind {
		t.Errorf("source kind = %v, want SourceOvermind", g.Source.Kind)
	}
	if len(g.Entries) != 1 || g.Entries[0].Session != "web" {
		t.Errorf("entries = %#v, want one 'web'", g.Entries)
	}
}

func TestDiscoverWith_PsFailureFallsBackToGenericExternal(t *testing.T) {
	p := fakeProber{
		sockets:  []socketInfo{{Name: "foo", Path: "/tmp/tmux-1000/foo"}},
		procsErr: errors.New("ps unavailable"),
		probeFn:  okProbe(CatalogEntry{Session: "bar"}),
	}

	cat, err := discoverWith(p, tmux.DefaultEndpoint())
	if err != nil {
		t.Fatalf("discoverWith: %v", err)
	}
	if len(cat.External) != 1 {
		t.Fatalf("external groups = %d, want 1", len(cat.External))
	}
	if cat.External[0].Source.Kind != SourceExternal {
		t.Errorf("source kind = %v, want SourceExternal (ps-failure fallback)", cat.External[0].Source.Kind)
	}
}

func TestDiscoverWith_StaleSocketSkipped(t *testing.T) {
	p := fakeProber{
		sockets: []socketInfo{{Name: "dead", Path: "/tmp/tmux-1000/dead"}},
		probeFn: func(tmux.Endpoint) ([]CatalogEntry, SourceHealth, error) {
			return nil, HealthStale, errors.New("probe timed out")
		},
	}

	cat, err := discoverWith(p, tmux.DefaultEndpoint())
	if err != nil {
		t.Fatalf("discoverWith: %v", err)
	}
	if len(cat.External) != 0 {
		t.Fatalf("external groups = %d, want 0 (stale skipped)", len(cat.External))
	}
}

func TestDiscoverWith_SocketScanErrorReturnsLocalOnly(t *testing.T) {
	p := fakeProber{
		local:      []CatalogEntry{{Session: "main"}},
		localOK:    true,
		socketsErr: errors.New("read socket dir: permission denied"),
	}

	cat, err := discoverWith(p, tmux.DefaultEndpoint())
	if err != nil {
		t.Fatalf("discoverWith: %v", err)
	}
	if len(cat.Local) != 1 {
		t.Errorf("local = %#v, want one entry", cat.Local)
	}
	if len(cat.External) != 0 {
		t.Errorf("external groups = %d, want 0 (socket scan failed)", len(cat.External))
	}
}

// Under the zzmux profile, the local server is -L zzmux: its own socket must be
// excluded from external discovery, while the default zmux server shows up as a
// regular external source (not silently treated as local / attached).
func TestDiscoverWith_ZzmuxExcludesOwnSocketKeepsDefaultExternal(t *testing.T) {
	p := fakeProber{
		local:   []CatalogEntry{{Session: "edge"}},
		localOK: true,
		sockets: []socketInfo{
			{Name: "zzmux", Path: "/tmp/tmux-1000/zzmux"},
			{Name: "default", Path: "/tmp/tmux-1000/default"},
		},
		procsErr: errors.New("ps unavailable"), // → generic external for remaining sockets
		probeFn:  okProbe(CatalogEntry{Session: "live"}),
	}

	cat, err := discoverWith(p, tmux.NamedEndpoint("zzmux"))
	if err != nil {
		t.Fatalf("discoverWith: %v", err)
	}
	if len(cat.External) != 1 {
		t.Fatalf("external groups = %d, want 1 (zzmux excluded, default kept)", len(cat.External))
	}
	if got := cat.External[0].Source.ID; got == "zzmux" {
		t.Errorf("external source = %q, want the default server, not the local zzmux socket", got)
	}
}

func TestLocalSocketName(t *testing.T) {
	cases := map[string]struct {
		ep   tmux.Endpoint
		want string
	}{
		"default": {tmux.DefaultEndpoint(), "default"},
		"named":   {tmux.NamedEndpoint("zzmux"), "zzmux"},
		"path":    {tmux.PathEndpoint("/tmp/tmux-1000/zzmux"), "zzmux"},
	}
	for name, tc := range cases {
		if got := localSocketName(tc.ep); got != tc.want {
			t.Errorf("%s: localSocketName = %q, want %q", name, got, tc.want)
		}
	}
}
