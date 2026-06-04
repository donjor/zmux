package source

import (
	"testing"

	"github.com/donjor/zmux/internal/tmux"
)

func TestParseProcessTable(t *testing.T) {
	input := ` 1234     1 /usr/bin/overmind start -s /tmp/overmind.sock -f Procfile
 5678  1234 tmux -L overmind new-session
 9999     1 bash
`
	entries := parseProcessTable(input)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	if entries[0].PID != 1234 {
		t.Errorf("entry[0].PID = %d, want 1234", entries[0].PID)
	}
	if entries[0].PPID != 1 {
		t.Errorf("entry[0].PPID = %d, want 1", entries[0].PPID)
	}
	if entries[0].Args != "/usr/bin/overmind start -s /tmp/overmind.sock -f Procfile" {
		t.Errorf("entry[0].Args = %q", entries[0].Args)
	}

	if entries[1].PID != 5678 {
		t.Errorf("entry[1].PID = %d, want 5678", entries[1].PID)
	}
	if entries[1].PPID != 1234 {
		t.Errorf("entry[1].PPID = %d, want 1234", entries[1].PPID)
	}
}

func TestParseProcessTableEmpty(t *testing.T) {
	entries := parseProcessTable("")
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestParseProcessTableMalformed(t *testing.T) {
	input := "not a pid line\n  abc def ghi\n"
	entries := parseProcessTable(input)
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestIsOvermindStart(t *testing.T) {
	tests := []struct {
		args string
		want bool
	}{
		{"overmind start -f Procfile", true},
		{"overmind s -f Procfile", true},
		{"/usr/local/bin/overmind start -s /tmp/sock", true},
		{"overmind connect web", false},
		{"overmind restart api", false},
		{"bash -c overmind", false},
		{"vim", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.args, func(t *testing.T) {
			got := isOvermindStart(tt.args)
			if got != tt.want {
				t.Errorf("isOvermindStart(%q) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func TestFindOvermindProcesses(t *testing.T) {
	procs := []processEntry{
		{PID: 100, PPID: 1, Args: "overmind start -s /tmp/overmind.sock -f /home/user/project/Procfile"},
		{PID: 200, PPID: 100, Args: "tmux -L overmind new-session"},
		{PID: 300, PPID: 1, Args: "bash"},
	}

	result := findOvermindProcesses(procs)
	if len(result) != 1 {
		t.Fatalf("expected 1 overmind process, got %d", len(result))
	}

	meta, ok := result["overmind"]
	if !ok {
		t.Fatal("expected key 'overmind' in result")
	}
	if meta.ControlSocket != "/tmp/overmind.sock" {
		t.Errorf("ControlSocket = %q, want /tmp/overmind.sock", meta.ControlSocket)
	}
	if meta.Procfile != "/home/user/project/Procfile" {
		t.Errorf("Procfile = %q, want /home/user/project/Procfile", meta.Procfile)
	}
}

func TestFindOvermindProcessesNone(t *testing.T) {
	procs := []processEntry{
		{PID: 100, PPID: 1, Args: "bash"},
		{PID: 200, PPID: 1, Args: "vim"},
	}
	result := findOvermindProcesses(procs)
	if len(result) != 0 {
		t.Fatalf("expected 0 overmind processes, got %d", len(result))
	}
}

func TestOvermindLabel(t *testing.T) {
	tests := []struct {
		name string
		meta *OvermindMeta
		want string
	}{
		{
			"with procfile",
			&OvermindMeta{Procfile: "/home/user/myproject/Procfile"},
			"myproject",
		},
		{
			"control socket only",
			&OvermindMeta{ControlSocket: "/tmp/overmind-abc.sock"},
			"overmind-abc.sock",
		},
		{
			"empty",
			&OvermindMeta{},
			"overmind",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := overmindLabel(tt.meta)
			if got != tt.want {
				t.Errorf("overmindLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseProbeOutput(t *testing.T) {
	input := "web\t3\t1\napi\t2\t0\nworker\t1\t0\n"
	entries := parseProbeOutput(input)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	if entries[0].Session != "web" {
		t.Errorf("entries[0].Session = %q, want web", entries[0].Session)
	}
	if entries[0].Windows != 3 {
		t.Errorf("entries[0].Windows = %d, want 3", entries[0].Windows)
	}
	if !entries[0].Attached {
		t.Error("entries[0].Attached = false, want true")
	}
	if entries[1].Session != "api" {
		t.Errorf("entries[1].Session = %q, want api", entries[1].Session)
	}
	if entries[1].Attached {
		t.Error("entries[1].Attached = true, want false")
	}
}

func TestParseProbeOutputEmpty(t *testing.T) {
	entries := parseProbeOutput("")
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestParseProbeOutputMalformed(t *testing.T) {
	entries := parseProbeOutput("bad data\nonly\ttwo")
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestCorrelateSources(t *testing.T) {
	sockets := []socketInfo{
		{Name: "overmind", Path: "/tmp/tmux-1000/overmind"},
		{Name: "tmate", Path: "/tmp/tmux-1000/tmate"},
	}
	procs := []processEntry{
		{PID: 100, PPID: 1, Args: "overmind start -s /tmp/overmind.sock -f Procfile"},
	}

	sources := correlateSources(sockets, procs)
	if len(sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(sources))
	}

	// First should be overmind.
	if sources[0].Kind != SourceOvermind {
		t.Errorf("sources[0].Kind = %q, want overmind", sources[0].Kind)
	}
	if sources[0].Overmind == nil {
		t.Error("sources[0].Overmind is nil")
	}

	// Second should be external.
	if sources[1].Kind != SourceExternal {
		t.Errorf("sources[1].Kind = %q, want external", sources[1].Kind)
	}
	if sources[1].Overmind != nil {
		t.Error("sources[1].Overmind should be nil")
	}
}

func TestCorrelateSourcesNoProcs(t *testing.T) {
	sockets := []socketInfo{
		{Name: "mysock", Path: "/tmp/tmux-1000/mysock"},
	}
	sources := correlateSources(sockets, nil)
	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}
	if sources[0].Kind != SourceExternal {
		t.Errorf("sources[0].Kind = %q, want external", sources[0].Kind)
	}
}

func TestExternalLabel(t *testing.T) {
	cases := map[string]string{
		"default":  "zmux",         // the live zmux server, seen from a sibling
		"zzmux":    "zzmux (edge)", // the edge profile, seen from zmux
		"tmate":    "tmate",        // unknown socket → raw name
		"overmind": "overmind",     // overmind sockets relabel elsewhere; raw here
	}
	for sock, want := range cases {
		if got := externalLabel(sock); got != want {
			t.Errorf("externalLabel(%q) = %q, want %q", sock, got, want)
		}
	}
}

// A zmux-family sibling socket keeps its raw socket as the Source ID but gets a
// friendly Label: socket "default" (the live zmux server) reads as "zmux".
func TestCorrelateSourcesSiblingLabel(t *testing.T) {
	sockets := []socketInfo{{Name: "default", Path: "/tmp/tmux-1000/default"}}
	sources := correlateSources(sockets, nil)
	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}
	if sources[0].ID != "default" {
		t.Errorf("ID = %q, want raw socket %q (dedup/expansion key)", sources[0].ID, "default")
	}
	if sources[0].Label != "zmux" {
		t.Errorf("Label = %q, want friendly %q", sources[0].Label, "zmux")
	}
}

func TestCatalogTypes(t *testing.T) {
	// Verify the catalog can be constructed with proper types.
	src := &Source{
		ID:       "test",
		Kind:     SourceLocal,
		Label:    "default",
		Health:   HealthOK,
		Endpoint: tmux.DefaultEndpoint(),
	}
	entry := CatalogEntry{
		Source:   src,
		Session:  "dev",
		Windows:  3,
		Attached: true,
	}
	cat := &Catalog{
		Local: []CatalogEntry{entry},
		External: []SourceGroup{
			{
				Source: Source{
					ID:       "ext",
					Kind:     SourceExternal,
					Label:    "external",
					Health:   HealthOK,
					Endpoint: tmux.NamedEndpoint("ext"),
				},
				Entries: []CatalogEntry{
					{Session: "web", Windows: 1, Attached: false},
				},
			},
		},
	}

	if len(cat.Local) != 1 {
		t.Errorf("cat.Local len = %d, want 1", len(cat.Local))
	}
	if len(cat.External) != 1 {
		t.Errorf("cat.External len = %d, want 1", len(cat.External))
	}
	if cat.External[0].Entries[0].Session != "web" {
		t.Errorf("external entry session = %q, want web", cat.External[0].Entries[0].Session)
	}
}
