package tmux

import (
	"path/filepath"
	"testing"
)

// T-106 (055 P-001) — scoped option-method argv contract (B-02/T-405). The six
// window/pane option methods each hand-build the same skeleton: a scope flag
// prefix (-w/-p, plus -u for unset, plus -q -v for show), an OPTIONAL "-t
// target" inserted only when target != "", then the trailing key[/value]. T-405
// consolidates this into one scopedArgs helper; these tables pin the exact argv
// for every method with and without a target so the extraction is provably
// argv-identical.
func TestClientOptionMethodArgv(t *testing.T) {
	t.Setenv("TMUX", "")
	logPath := filepath.Join(t.TempDir(), "tmux-args.log")
	client := &Client{bin: fakeTmux(t, logPath, "")}

	// Each step invokes one method; want is the space-joined argv the fake logs.
	steps := []struct {
		call func() error
		want string
	}{
		{func() error { return client.SetWindowOption("%1", "@k", "v") }, "set-option -w -t %1 @k v"},
		{func() error { return client.SetWindowOption("", "@k", "v") }, "set-option -w @k v"},
		{func() error { return client.UnsetWindowOption("%1", "@k") }, "set-option -w -u -t %1 @k"},
		{func() error { return client.UnsetWindowOption("", "@k") }, "set-option -w -u @k"},
		{func() error { return client.SetPaneOption("%1", "@k", "v") }, "set-option -p -t %1 @k v"},
		{func() error { return client.SetPaneOption("", "@k", "v") }, "set-option -p @k v"},
		{func() error { return client.UnsetPaneOption("%1", "@k") }, "set-option -p -u -t %1 @k"},
		{func() error { return client.UnsetPaneOption("", "@k") }, "set-option -p -u @k"},
		{func() error { _, err := client.ShowWindowOption("%1", "@k"); return err }, "show-options -w -q -v -t %1 @k"},
		{func() error { _, err := client.ShowWindowOption("", "@k"); return err }, "show-options -w -q -v @k"},
		{func() error { _, err := client.ShowPaneOption("%1", "@k"); return err }, "show-options -p -q -v -t %1 @k"},
		{func() error { _, err := client.ShowPaneOption("", "@k"); return err }, "show-options -p -q -v @k"},
	}

	for i, s := range steps {
		if err := s.call(); err != nil {
			t.Fatalf("step %d (%s): %v", i, s.want, err)
		}
	}

	calls := readFakeTmuxCalls(t, logPath)
	if len(calls) != len(steps) {
		t.Fatalf("expected %d calls, got %d: %v", len(steps), len(calls), calls)
	}
	for i, s := range steps {
		if calls[i] != s.want {
			t.Errorf("step %d argv = %q, want %q", i, calls[i], s.want)
		}
	}
}
