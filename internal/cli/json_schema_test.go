package cli

import (
	"encoding/json"
	"testing"

	"github.com/donjor/zmux/internal/tmux"
)

// assertExactJSONKeys fails if got's key set is not exactly want's, or if any
// shared key's value differs. It pins the public --json contract: canonical
// keys present AND legacy variants absent (S-007, no aliases).
func assertExactJSONKeys(t *testing.T, got, want map[string]any) {
	t.Helper()
	for k, wv := range want {
		gv, ok := got[k]
		if !ok {
			t.Errorf("missing JSON key %q", k)
			continue
		}
		// Only scalar values are value-checked; nested slices/maps are
		// asserted separately (and comparing them with != would panic).
		switch wv.(type) {
		case string, float64, bool:
			if gv != wv {
				t.Errorf("JSON key %q = %#v, want %#v", k, gv, wv)
			}
		}
	}
	for k := range got {
		if _, ok := want[k]; !ok {
			t.Errorf("unexpected JSON key %q (value %#v)", k, got[k])
		}
	}
}

func marshalToMap(t *testing.T, v any) map[string]any {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := map[string]any{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return got
}

// TestPaneJSONSchema pins the explicit pane DTO (pane list / pane current):
// lower-camelCase paneId, no exported-Go-name leakage.
func TestPaneJSONSchema(t *testing.T) {
	dto := newPaneJSON(tmux.Pane{
		ID: "%7", Session: "dev", Index: 1, WindowIndex: 2, WindowName: "work",
		Active: true, Command: "bash", PID: 99, Dir: "/repo", Width: 80, Height: 24, Title: "host",
	})
	assertExactJSONKeys(t, marshalToMap(t, dto), map[string]any{
		"paneId":      "%7",
		"session":     "dev",
		"index":       float64(1),
		"windowIndex": float64(2),
		"windowName":  "work",
		"active":      true,
		"command":     "bash",
		"pid":         float64(99),
		"dir":         "/repo",
		"width":       float64(80),
		"height":      float64(24),
		"title":       "host",
	})
}

// TestJoinedPaneRowJSONSchema pins the joined-tab row tags: paneId/tabId/
// hostPaneId/anchorId (not the old ...ID casing).
func TestJoinedPaneRowJSONSchema(t *testing.T) {
	row := joinedPaneListRow{
		TabID: "ztab_peer", TabName: "codex-peer", PaneID: "%2", Session: "dev",
		HostName: "work", HostPaneID: "%1", AnchorID: "ztab_host",
		CWD: "/repo", Command: "codex", Title: "peer", Active: true, Caller: true,
	}
	assertExactJSONKeys(t, marshalToMap(t, row), map[string]any{
		"tabId":      "ztab_peer",
		"tabName":    "codex-peer",
		"paneId":     "%2",
		"session":    "dev",
		"hostName":   "work",
		"hostPaneId": "%1",
		"anchorId":   "ztab_host",
		"cwd":        "/repo",
		"command":    "codex",
		"title":      "peer",
		"active":     true,
		"caller":     true,
	})
}

// TestTerminalCapabilitiesJSONSchema pins the capabilities DTO camelCase:
// currentTty, and per-client windowId/paneId. insideEnv keeps literal env
// names (the documented sole exception).
func TestTerminalCapabilitiesJSONSchema(t *testing.T) {
	result := terminalCapabilitiesResult{
		SchemaVersion: "zmux-terminal-capabilities/v1",
		OK:            true,
		Status:        "ok",
		TmuxVersion:   "3.4",
		InsideTmux:    true,
		InsideEnv:     terminalInsideEnv{TERM: "xterm-256color", COLORTERM: "truecolor", TERMProgram: "ghostty"},
		CurrentTTY:    "/dev/pts/1",
		Clients: []terminalCapabilityClient{{
			TTY: "/dev/pts/1", Current: true, Focused: true, TermName: "tmux-256color",
			Features: []string{"RGB"}, RGB: true, SessionName: "dev",
			WindowID: "@1", WindowName: "work", PaneID: "%1",
		}},
	}
	got := marshalToMap(t, result)
	// recommendation is omitempty (only present when not OK), so it is absent here.
	assertExactJSONKeys(t, got, map[string]any{
		"schemaVersion": "zmux-terminal-capabilities/v1",
		"ok":            true,
		"status":        "ok",
		"tmuxVersion":   "3.4",
		"insideTmux":    true,
		"insideEnv":     got["insideEnv"], // literal env names asserted below (sole exception)
		"currentTty":    "/dev/pts/1",
		"clients":       got["clients"], // asserted per-client below
	})
	// insideEnv keeps SCREAMING_CASE literal env-var names — the sole exception.
	insideEnv, _ := got["insideEnv"].(map[string]any)
	assertExactJSONKeys(t, insideEnv, map[string]any{
		"TERM":         "xterm-256color",
		"COLORTERM":    "truecolor",
		"TERM_PROGRAM": "ghostty",
	})
	clients, _ := got["clients"].([]any)
	if len(clients) != 1 {
		t.Fatalf("clients = %#v, want one", got["clients"])
	}
	client, _ := clients[0].(map[string]any)
	// flags is omitempty and unset here, so it is absent.
	assertExactJSONKeys(t, client, map[string]any{
		"tty":         "/dev/pts/1",
		"current":     true,
		"focused":     true,
		"termName":    "tmux-256color",
		"features":    client["features"],
		"rgb":         true,
		"sessionName": "dev",
		"windowId":    "@1",
		"windowName":  "work",
		"paneId":      "%1",
	})
	for _, gone := range []string{"currentTTY", "windowID", "paneID"} {
		if _, ok := client[gone]; ok {
			t.Errorf("client still emits removed key %q", gone)
		}
	}
}
