package tabs

import (
	"strings"
	"testing"

	"github.com/donjor/zmux/internal/tmux"
)

func mruMock(current string) *tmux.MockRunner {
	mock := tmux.NewMockRunner()
	mock.DisplayMessageResult = current
	return mock
}

func lastMRUWrite(t *testing.T, mock *tmux.MockRunner) string {
	t.Helper()
	for i := len(mock.Calls) - 1; i >= 0; i-- {
		c := mock.Calls[i]
		if c.Method == "SetSessionOption" && c.Args[1] == OptMRU {
			return c.Args[2]
		}
	}
	t.Fatal("no MRU write recorded")
	return ""
}

func TestTouchMRUMovesToFront(t *testing.T) {
	mock := mruMock("ztab_b ztab_a ztab_c")
	if err := TouchMRU(mock, "work", "ztab_c"); err != nil {
		t.Fatal(err)
	}
	if got := lastMRUWrite(t, mock); got != "ztab_c ztab_b ztab_a" {
		t.Errorf("got %q", got)
	}
}

func TestTouchMRUCaps(t *testing.T) {
	ids := make([]string, mruCap)
	for i := range ids {
		ids[i] = "ztab_" + strings.Repeat("a", 3) + string(rune('a'+i%26)) + string(rune('a'+i/26))
	}
	mock := mruMock(strings.Join(ids, " "))
	if err := TouchMRU(mock, "work", "ztab_new"); err != nil {
		t.Fatal(err)
	}
	got := strings.Fields(lastMRUWrite(t, mock))
	if len(got) != mruCap || got[0] != "ztab_new" {
		t.Errorf("want capped list fronted by ztab_new, got %d entries", len(got))
	}
}

func TestPruneMRUDropsDeadIDs(t *testing.T) {
	mock := mruMock("ztab_live ztab_dead ztab_live2")
	live := map[string]bool{"ztab_live": true, "ztab_live2": true}
	if err := PruneMRU(mock, "work", func(id string) bool { return live[id] }); err != nil {
		t.Fatal(err)
	}
	if got := lastMRUWrite(t, mock); got != "ztab_live ztab_live2" {
		t.Errorf("got %q", got)
	}
}

func TestPruneMRUNoopWithoutDeaths(t *testing.T) {
	mock := mruMock("ztab_a ztab_b")
	if err := PruneMRU(mock, "work", func(string) bool { return true }); err != nil {
		t.Fatal(err)
	}
	for _, c := range mock.Calls {
		if c.Method == "SetSessionOption" {
			t.Fatalf("prune must not rewrite an unchanged list: %v", c)
		}
	}
}
