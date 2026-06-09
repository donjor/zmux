package tmux

import (
	"strings"
	"testing"
)

func TestIsInsideTmuxEndpointAware(t *testing.T) {
	cases := []struct {
		name    string
		tmuxEnv string
		client  *Client
		want    bool
	}{
		{"unset env, default", "", NewClient(), false},
		{"unset env, named", "", NewClientFor(NamedEndpoint("zzmux")), false},
		{"set env, default endpoint always true", "/tmp/tmux-1000/default,1,0", NewClient(), true},
		{"named matches socket", "/tmp/tmux-1000/zzmux,1,0", NewClientFor(NamedEndpoint("zzmux")), true},
		{"named mismatches default socket", "/tmp/tmux-1000/default,1,0", NewClientFor(NamedEndpoint("zzmux")), false},
		{"default client inside zzmux socket is foreign", "/tmp/tmux-1000/zzmux,1,0", NewClient(), false},
		{"path endpoint matches basename", "/tmp/tmux-1000/zzmux,1,0", NewClientFor(PathEndpoint("/tmp/tmux-1000/zzmux")), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TMUX", tc.tmuxEnv)
			if got := tc.client.IsInsideTmux(); got != tc.want {
				t.Errorf("IsInsideTmux() = %v, want %v (TMUX=%q)", got, tc.want, tc.tmuxEnv)
			}
		})
	}
}

// TestAmbientSocketMismatchGuard pins the cross-profile refusal: a
// default-endpoint client (live zmux) invoked inside another profile's
// session (foreign $TMUX socket) must refuse every server command instead
// of silently following $TMUX onto that profile's server — the 2026-06-06
// incident wrote the live binary's themed bar (no tab glyphs) onto the
// zzmux qa server exactly this way.
func TestAmbientSocketMismatchGuard(t *testing.T) {
	cases := []struct {
		name    string
		tmuxEnv string
		client  *Client
		refuse  bool
	}{
		{"outside tmux, default", "", NewClient(), false},
		{"inside own default socket", "/tmp/tmux-1000/default,1,0", NewClient(), false},
		{"default client inside zzmux socket", "/tmp/tmux-1000/zzmux,1,0", NewClient(), true},
		{"named endpoint is exempt (explicit -L wins)", "/tmp/tmux-1000/default,1,0", NewClientFor(NamedEndpoint("zzmux")), false},
		{"path endpoint is exempt", "/tmp/tmux-1000/default,1,0", NewClientFor(PathEndpoint("/tmp/tmux-1000/zzmux")), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TMUX", tc.tmuxEnv)
			err := tc.client.ambientSocketMismatch()
			if tc.refuse && err == nil {
				t.Error("want refusal, got nil")
			}
			if !tc.refuse && err != nil {
				t.Errorf("want pass, got %v", err)
			}
			if tc.refuse {
				if _, runErr := tc.client.run("list-sessions"); runErr == nil ||
					!strings.Contains(runErr.Error(), "cross-profile") {
					t.Errorf("run() must surface the refusal, got %v", runErr)
				}
				if tc.client.ServerRunning() {
					t.Error("ServerRunning() must report false under mismatch")
				}
				if tc.client.HasSession("any") {
					t.Error("HasSession() must report false under mismatch")
				}
			}
		})
	}
}

func TestDefaultEndpointArgs(t *testing.T) {
	ep := DefaultEndpoint()
	args := ep.Args()
	if args != nil {
		t.Errorf("DefaultEndpoint().Args() = %v, want nil", args)
	}
}

func TestNamedEndpointArgs(t *testing.T) {
	ep := NamedEndpoint("mysocket")
	args := ep.Args()
	if len(args) != 2 || args[0] != "-L" || args[1] != "mysocket" {
		t.Errorf("NamedEndpoint(\"mysocket\").Args() = %v, want [-L mysocket]", args)
	}
}

func TestPathEndpointArgs(t *testing.T) {
	ep := PathEndpoint("/tmp/tmux-1000/overmind-abc")
	args := ep.Args()
	if len(args) != 2 || args[0] != "-S" || args[1] != "/tmp/tmux-1000/overmind-abc" {
		t.Errorf("PathEndpoint(...).Args() = %v, want [-S /tmp/tmux-1000/overmind-abc]", args)
	}
}

func TestEndpointString(t *testing.T) {
	tests := []struct {
		name string
		ep   Endpoint
		want string
	}{
		{"default", DefaultEndpoint(), "default"},
		{"named", NamedEndpoint("foo"), "socket:foo"},
		{"path", PathEndpoint("/tmp/sock"), "path:/tmp/sock"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ep.String()
			if got != tt.want {
				t.Errorf("Endpoint.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEndpointMode(t *testing.T) {
	if DefaultEndpoint().Mode != SocketDefault {
		t.Error("DefaultEndpoint().Mode != SocketDefault")
	}
	if NamedEndpoint("x").Mode != SocketNamed {
		t.Error("NamedEndpoint().Mode != SocketNamed")
	}
	if PathEndpoint("/x").Mode != SocketPath {
		t.Error("PathEndpoint().Mode != SocketPath")
	}
}

func TestBuildArgsDefault(t *testing.T) {
	c := NewClient()
	args := c.buildArgs("list-sessions", "-F", "#{session_name}")
	expected := []string{"list-sessions", "-F", "#{session_name}"}
	if len(args) != len(expected) {
		t.Fatalf("buildArgs len = %d, want %d", len(args), len(expected))
	}
	for i, a := range args {
		if a != expected[i] {
			t.Errorf("buildArgs[%d] = %q, want %q", i, a, expected[i])
		}
	}
}

func TestBuildArgsNamed(t *testing.T) {
	c := NewClientFor(NamedEndpoint("overmind-abc"))
	args := c.buildArgs("list-sessions")
	expected := []string{"-L", "overmind-abc", "list-sessions"}
	if len(args) != len(expected) {
		t.Fatalf("buildArgs len = %d, want %d", len(args), len(expected))
	}
	for i, a := range args {
		if a != expected[i] {
			t.Errorf("buildArgs[%d] = %q, want %q", i, a, expected[i])
		}
	}
}

func TestBuildArgsPath(t *testing.T) {
	c := NewClientFor(PathEndpoint("/tmp/tmux-1000/overmind-abc"))
	args := c.buildArgs("has-session", "-t", "web")
	expected := []string{"-S", "/tmp/tmux-1000/overmind-abc", "has-session", "-t", "web"}
	if len(args) != len(expected) {
		t.Fatalf("buildArgs len = %d, want %d", len(args), len(expected))
	}
	for i, a := range args {
		if a != expected[i] {
			t.Errorf("buildArgs[%d] = %q, want %q", i, a, expected[i])
		}
	}
}

func TestNewClientBackwardsCompatible(t *testing.T) {
	c := NewClient()
	if c.endpoint.Mode != SocketDefault {
		t.Errorf("NewClient().endpoint.Mode = %d, want SocketDefault", c.endpoint.Mode)
	}
	if c.endpoint.Value != "" {
		t.Errorf("NewClient().endpoint.Value = %q, want empty", c.endpoint.Value)
	}
}

func TestClientEndpointAccessor(t *testing.T) {
	ep := NamedEndpoint("test")
	c := NewClientFor(ep)
	got := c.Endpoint()
	if got.Mode != ep.Mode || got.Value != ep.Value {
		t.Errorf("Endpoint() = %v, want %v", got, ep)
	}
}
