package tmux

import (
	"strings"
	"testing"
	"time"
)

func TestParseLogicalRowsFullRow(t *testing.T) {
	line := strings.Join([]string{
		"%57", "work", "work", "1",
		"@12", "3", "buddy", "1", "2", "1780707143",
		"1", "claude", "/home/u/proj", "host",
		"ztab_ab12", "buddy", "user", "running", "ztab_ff00", "work",
	}, "\t")
	rows := parseLogicalRows(line)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	r := rows[0]
	want := LogicalPaneRow{
		PaneID: "%57", Session: "work", SessionGroup: "work", SessionAttached: 1,
		WindowID: "@12", WindowIndex: 3, WindowName: "buddy", WindowActive: true,
		WindowPanes: 2, WindowActivity: time.Unix(1780707143, 0),
		PaneActive: true, Command: "claude", Dir: "/home/u/proj", Title: "host",
		TabID: "ztab_ab12", Label: "buddy", LabelSource: "user",
		State: "running", Anchor: "ztab_ff00", Hidden: "work",
	}
	if r != want {
		t.Errorf("row mismatch:\n got %+v\nwant %+v", r, want)
	}
}

// Rows whose trailing user options are all unset end in literal TABs, which
// run()'s TrimSpace eats on the last line — the parser must pad them back.
func TestParseLogicalRowsTrimmedTrailingFields(t *testing.T) {
	line := "%0\twork\t\t0\t@0\t0\tbash\t0\t1\t1780707143\t1\tbash\t/home/u\tugrindtime"
	rows := parseLogicalRows(line)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	r := rows[0]
	if r.TabID != "" || r.Label != "" || r.State != "" || r.Anchor != "" || r.Hidden != "" {
		t.Errorf("expected empty user options, got %+v", r)
	}
	if r.PaneID != "%0" || r.Title != "ugrindtime" {
		t.Errorf("padded parse corrupted leading fields: %+v", r)
	}
}

func TestParseLogicalRowsSkipsBlankAndEmpty(t *testing.T) {
	if rows := parseLogicalRows(""); rows != nil {
		t.Errorf("empty output: expected nil, got %v", rows)
	}
	out := "%1\twork\t\t0\t@1\t1\tbuddy\t1\t1\t100\t1\tbash\t/h\tt\tztab_1\tbuddy\t\t\t\t\n\n"
	rows := parseLogicalRows(out)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].TabID != "ztab_1" {
		t.Errorf("TabID = %q, want ztab_1", rows[0].TabID)
	}
}

func TestLogicalRowFormatFieldCount(t *testing.T) {
	got := strings.Count(logicalRowFormat, "\t") + 1
	if got != logicalRowFields {
		t.Errorf("logicalRowFormat has %d fields, parser expects %d", got, logicalRowFields)
	}
}

func TestParseLogicalRowsLifecycleFields(t *testing.T) {
	line := strings.Join([]string{
		"%9", "work", "", "0",
		"@3", "1", "scratch", "0", "1", "1780707143",
		"1", "node", "/h", "t",
		"ztab_x", "scratch", "pane", "", "", "",
		"agent", "1780700000", "task", "3600", "1", "1780701000", "1780702000", "4242",
	}, "\t")
	rows := parseLogicalRows(line)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	r := rows[0]
	if r.Origin != "agent" || r.Born != "1780700000" || r.Scope != "task" ||
		r.TTL != "3600" || r.Keep != "1" || r.StaleAt != "1780701000" ||
		r.LastInputAt != "1780702000" || r.PanePID != 4242 {
		t.Errorf("lifecycle fields mismatch: %+v", r)
	}
}
