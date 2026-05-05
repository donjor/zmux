package wm

import "testing"

func TestParseHyprlandWindowsVisibilityAndGeometry(t *testing.T) {
	clients := []byte(`[
		{"address":"0x1","class":"com.mitchellh.ghostty","title":"zmux:v1;tty=/dev/pts/1;sid=$1;wid=@1;pane=%1","pid":1234,"workspace":{"name":"2"},"at":[12,57],"size":[2536,1371],"mapped":true,"hidden":false},
		{"address":"0x2","class":"com.mitchellh.ghostty","title":"Ghostty","workspace":{"name":"8"},"at":[0,0],"size":[80,40],"mapped":true,"hidden":false},
		{"address":"0x3","class":"com.mitchellh.ghostty","title":"hidden","workspace":{"name":"2"},"at":[1,2],"size":[3,4],"mapped":true,"hidden":true}
	]`)
	monitors := []byte(`[{"activeWorkspace":{"name":"2"}}]`)
	windows, err := parseHyprlandWindows(clients, monitors)
	if err != nil {
		t.Fatalf("parseHyprlandWindows failed: %v", err)
	}
	if len(windows) != 3 {
		t.Fatalf("expected 3 windows, got %d", len(windows))
	}
	if !windows[0].Visible || windows[0].Geometry != "12,57 2536x1371" || windows[0].PID != 1234 || !windows[0].OnActiveMonitor {
		t.Fatalf("unexpected visible window: %#v", windows[0])
	}
	if windows[1].Visible || windows[1].OnActiveMonitor {
		t.Fatalf("off-workspace window should not be visible: %#v", windows[1])
	}
	if windows[2].Visible || !windows[2].Hidden {
		t.Fatalf("hidden window should not be visible: %#v", windows[2])
	}
}

func TestParseHyprlandWindowsMalformedJSON(t *testing.T) {
	_, err := parseHyprlandWindows([]byte(`not-json`), []byte(`[]`))
	if err == nil {
		t.Fatal("expected malformed clients JSON error")
	}
	_, err = parseHyprlandWindows([]byte(`[]`), []byte(`not-json`))
	if err == nil {
		t.Fatal("expected malformed monitors JSON error")
	}
}
