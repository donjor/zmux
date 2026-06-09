package tabs

import (
	"regexp"
	"testing"

	"github.com/donjor/zmux/internal/tablabel"
	"github.com/donjor/zmux/internal/tmux"
)

func TestNewIDShape(t *testing.T) {
	id, err := NewID()
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^ztab_[0-9a-f]{12}$`).MatchString(id) {
		t.Errorf("id %q does not match ztab_<12 hex>", id)
	}
	id2, _ := NewID()
	if id == id2 {
		t.Errorf("two ids collided: %s", id)
	}
}

func optionWrites(mock *tmux.MockRunner) []tmux.MockCall {
	var out []tmux.MockCall
	for _, c := range mock.Calls {
		if c.Method == "ApplyOptions" {
			out = append(out, c)
		}
	}
	return out
}

func TestStampFreshPane(t *testing.T) {
	mock := tmux.NewMockRunner()
	id, err := Stamp(mock, "%5", "work:2", "buddy", tablabel.SourceManual)
	if err != nil || id == "" {
		t.Fatalf("stamp failed: id=%q err=%v", id, err)
	}
	writes := optionWrites(mock)
	if len(writes) != 5 { // id + pane label/source + window label/source mirror
		t.Fatalf("want 5 batched writes, got %d: %v", len(writes), writes)
	}
	if writes[0].Args[0] != string(tmux.ScopePane) || writes[0].Args[2] != OptTabID || writes[0].Args[3] != id {
		t.Errorf("first write must stamp the pane id: %v", writes[0].Args)
	}
}

func TestStampPreservesExistingID(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.PaneOptions = map[string]string{"%5\x00" + OptTabID: "ztab_keepme"}
	id, err := Stamp(mock, "%5", "work:2", "buddy", tablabel.SourcePane)
	if err != nil || id != "ztab_keepme" {
		t.Fatalf("stamp must preserve identity: id=%q err=%v", id, err)
	}
}

func TestStampWithoutLabelOnlyWritesID(t *testing.T) {
	mock := tmux.NewMockRunner()
	if _, err := Stamp(mock, "%5", "work:2", "", ""); err != nil {
		t.Fatal(err)
	}
	if writes := optionWrites(mock); len(writes) != 1 {
		t.Fatalf("label-less stamp should only write the id, got %v", writes)
	}
}

func TestMigrateWindowLabelClaims(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.WindowOptions = map[string]string{
		"work:2\x00" + tablabel.Option:       "legacy",
		"work:2\x00" + tablabel.SourceOption: tablabel.SourcePane,
	}
	id, err := MigrateWindowLabel(mock, "work:2", "%5")
	if err != nil || id == "" {
		t.Fatalf("migration should claim the pane: id=%q err=%v", id, err)
	}
	var sawPaneLabel bool
	for _, w := range optionWrites(mock) {
		if w.Args[0] == string(tmux.ScopePane) && w.Args[2] == tablabel.Option && w.Args[3] == "legacy" {
			sawPaneLabel = true
		}
	}
	if !sawPaneLabel {
		t.Error("migration must copy the window label to the pane")
	}
}

func TestMigrateWindowLabelNoops(t *testing.T) {
	// no window label
	mock := tmux.NewMockRunner()
	if id, err := MigrateWindowLabel(mock, "work:2", "%5"); err != nil || id != "" {
		t.Fatalf("unlabeled window must not be claimed: id=%q err=%v", id, err)
	}
	// already managed
	mock2 := tmux.NewMockRunner()
	mock2.WindowOptions = map[string]string{"work:2\x00" + tablabel.Option: "legacy"}
	mock2.PaneOptions = map[string]string{"%5\x00" + OptTabID: "ztab_done"}
	if id, err := MigrateWindowLabel(mock2, "work:2", "%5"); err != nil || id != "" {
		t.Fatalf("managed pane must not be re-claimed: id=%q err=%v", id, err)
	}
}
