package tabstate

import (
	"errors"
	"testing"

	"github.com/donjor/zmux/internal/tmux"
)

func TestResolveTargetExplicitSpec(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.DisplayMessageResult = "%7\tdev:3\n"

	tgt, err := ResolveTarget(mock, "dev:3", func(string) string { return "%99" })
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if tgt.PaneID != "%7" || tgt.Window != "dev:3" {
		t.Fatalf("unexpected target: %+v", tgt)
	}
	// explicit spec must win over env
	if mock.Calls[0].Args[0] != "dev:3" {
		t.Fatalf("display should query the explicit spec, got %v", mock.Calls[0].Args)
	}
}

func TestResolveTargetFallsBackToTmuxPane(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.DisplayMessageResult = "%4\tdev:1\n"

	tgt, err := ResolveTarget(mock, "", func(key string) string {
		if key == "TMUX_PANE" {
			return "%4"
		}
		return ""
	})
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if tgt.PaneID != "%4" {
		t.Fatalf("unexpected target: %+v", tgt)
	}
	if mock.Calls[0].Args[0] != "%4" {
		t.Fatalf("display should query $TMUX_PANE, got %v", mock.Calls[0].Args)
	}
}

// Inside tmux ($TMUX set) but without $TMUX_PANE (scrubbed env), the ladder
// falls through to the client's current pane: an empty display-message
// target, the same idiom bar_render relies on.
func TestResolveTargetCurrentPaneInsideTmux(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.DisplayMessageResult = "%2\tdev:0\n"

	tgt, err := ResolveTarget(mock, "", func(key string) string {
		if key == "TMUX" {
			return "/tmp/tmux-1000/default,123,0"
		}
		return ""
	})
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if tgt.PaneID != "%2" || tgt.Window != "dev:0" {
		t.Fatalf("unexpected target: %+v", tgt)
	}
	if mock.Calls[0].Args[0] != "" {
		t.Fatalf("display should query the current pane (empty target), got %v", mock.Calls[0].Args)
	}
}

func TestResolveTargetNoTargetFailsOpen(t *testing.T) {
	mock := tmux.NewMockRunner()
	_, err := ResolveTarget(mock, "", func(string) string { return "" })
	if !errors.Is(err, ErrNoTarget) {
		t.Fatalf("want ErrNoTarget, got %v", err)
	}
	if len(mock.Calls) != 0 {
		t.Fatalf("no tmux calls expected without a target, got %v", mock.Calls)
	}
}
