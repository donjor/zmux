package outline

import (
	"strings"
	"testing"
)

func TestRowIDConstructors(t *testing.T) {
	tests := []struct {
		name  string
		id    string
		want  string
	}{
		{"top action", TopActionID(), "top"},
		{"workspace", WorkspaceID("myapp"), "ws:myapp"},
		{"session", SessionID("main"), "session:main"},
		{"window", WindowID("main", 3), "window:main:3"},
		{"extgroup overmind", ExternalGroupID("overmind", "/home/x/proj"), "extgroup:overmind:/home/x/proj"},
		{"extentry overmind", ExternalEntryID("overmind", "web"), "extentry:overmind:web"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.id != tt.want {
				t.Errorf("got %q, want %q", tt.id, tt.want)
			}
		})
	}
}

func TestRowIDPrefixes(t *testing.T) {
	// Every non-top ID should be recognizable by prefix — callers may
	// need to classify rows before having the Row itself available.
	cases := []struct {
		id     string
		prefix string
	}{
		{WorkspaceID("a"), "ws:"},
		{SessionID("a"), "session:"},
		{WindowID("a", 1), "window:"},
		{ExternalGroupID("k", "x"), "extgroup:"},
		{ExternalEntryID("k", "x"), "extentry:"},
	}
	for _, c := range cases {
		if !strings.HasPrefix(c.id, c.prefix) {
			t.Errorf("id %q should have prefix %q", c.id, c.prefix)
		}
	}
}

func TestExternalIDsIncludeKind(t *testing.T) {
	// Codex #7: external IDs must include the source kind so catalog
	// reordering across refetches doesn't invalidate cursor / expansion
	// state.
	a := ExternalEntryID("overmind", "web")
	b := ExternalEntryID("tmux-socket", "web")
	if a == b {
		t.Errorf("same entry key across different source kinds must have different IDs; got %q and %q", a, b)
	}
}

func TestFormatSessionCount(t *testing.T) {
	if got := FormatSessionCount(0); got != "0 sessions" {
		t.Errorf("0 sessions: got %q", got)
	}
	if got := FormatSessionCount(1); got != "1 session" {
		t.Errorf("1 session: got %q", got)
	}
	if got := FormatSessionCount(5); got != "5 sessions" {
		t.Errorf("5 sessions: got %q", got)
	}
}

type sampleData struct {
	Name string
}

func TestRowDataUnpacksTypedPayload(t *testing.T) {
	payload := &sampleData{Name: "hello"}
	row := &Row{Data: payload}

	got, ok := RowData[sampleData](row)
	if !ok {
		t.Fatal("RowData returned ok=false on matching type")
	}
	if got != payload {
		t.Errorf("got %v, want %v", got, payload)
	}
	if got.Name != "hello" {
		t.Errorf("payload.Name = %q, want hello", got.Name)
	}
}

func TestRowDataNilRowReturnsFalse(t *testing.T) {
	got, ok := RowData[sampleData](nil)
	if ok {
		t.Error("RowData(nil) ok=true, want false")
	}
	if got != nil {
		t.Errorf("RowData(nil) val=%v, want nil", got)
	}
}

func TestRowDataWrongTypeReturnsFalse(t *testing.T) {
	row := &Row{Data: "a string, not a *sampleData"}
	got, ok := RowData[sampleData](row)
	if ok {
		t.Error("RowData wrong type ok=true, want false")
	}
	if got != nil {
		t.Errorf("RowData wrong type val=%v, want nil", got)
	}
}

func TestRowDataNilPayloadReturnsFalse(t *testing.T) {
	row := &Row{Data: nil}
	got, ok := RowData[sampleData](row)
	if ok {
		t.Error("RowData nil payload ok=true, want false")
	}
	if got != nil {
		t.Errorf("RowData nil payload val=%v, want nil", got)
	}
}
