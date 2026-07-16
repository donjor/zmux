package terminal

import (
	"encoding/json"
	"testing"
)

// TestTmuxContextJSONSchema pins the `terminal current --json` tmux block on the
// canonical lower-camelCase + Id-suffix contract (S-007): sessionId/windowId/
// paneId, not the old ...ID casing.
func TestTmuxContextJSONSchema(t *testing.T) {
	ctx := TmuxContext{
		ClientTty:     "/dev/pts/2",
		ClientSession: "dev",
		SessionID:     "$1",
		WindowID:      "@3",
		WindowIndex:   2,
		WindowName:    "work",
		PaneID:        "%5",
	}
	data, err := json.Marshal(ctx)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := map[string]any{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := map[string]any{
		"clientTty":     "/dev/pts/2",
		"clientSession": "dev",
		"sessionId":     "$1",
		"windowId":      "@3",
		"windowIndex":   float64(2),
		"windowName":    "work",
		"paneId":        "%5",
	}
	for k, wv := range want {
		gv, ok := got[k]
		if !ok {
			t.Errorf("missing JSON key %q", k)
			continue
		}
		if gv != wv {
			t.Errorf("JSON key %q = %#v, want %#v", k, gv, wv)
		}
	}
	for k := range got {
		if _, ok := want[k]; !ok {
			t.Errorf("unexpected JSON key %q (value %#v)", k, got[k])
		}
	}
	for _, gone := range []string{"sessionID", "windowID", "paneID"} {
		if _, ok := got[gone]; ok {
			t.Errorf("still emits removed key %q", gone)
		}
	}
}
