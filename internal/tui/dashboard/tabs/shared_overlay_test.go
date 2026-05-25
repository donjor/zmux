package tabs

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/textinput"
	"github.com/donjor/zmux/internal/tui/styles"
)

func TestRenderRenameOverlaySharedShowsKindLabel(t *testing.T) {
	in := textinput.New()
	in.SetValue("foo")
	out := renderRenameOverlayShared(styles.DefaultStyles(), "session", in)
	if !strings.Contains(out, "rename session") {
		t.Errorf("expected 'rename session' label, got %q", out)
	}
}

func TestRenderRenameOverlaySharedHandlesEmptyKind(t *testing.T) {
	in := textinput.New()
	out := renderRenameOverlaySharedNoCrash(t, "", in)
	if !strings.Contains(out, "rename") {
		t.Errorf("expected 'rename' label, got %q", out)
	}
}

func renderRenameOverlaySharedNoCrash(t *testing.T, kind string, in textinput.Model) string {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("renderRenameOverlayShared panicked: %v", r)
		}
	}()
	return renderRenameOverlayShared(styles.DefaultStyles(), kind, in)
}

func TestRenderConfirmOverlaySharedNilReturnsEmpty(t *testing.T) {
	out := renderConfirmOverlayShared(styles.DefaultStyles(), nil, 1)
	if out != "" {
		t.Errorf("expected empty output for nil confirm, got %q", out)
	}
}

func TestRenderConfirmOverlaySharedStep1ShowsKindAndName(t *testing.T) {
	c := &confirmState{kind: "workspace", name: "myapp"}
	out := renderConfirmOverlayShared(styles.DefaultStyles(), c, 1)
	if !strings.Contains(out, "Kill workspace") {
		t.Errorf("expected 'Kill workspace', got %q", out)
	}
	if !strings.Contains(out, "myapp") {
		t.Errorf("expected name in output, got %q", out)
	}
	if !strings.Contains(out, "(y/N)") {
		t.Errorf("expected (y/N) prompt, got %q", out)
	}
}

func TestRenderConfirmOverlaySharedStep2ShowsDetachWarning(t *testing.T) {
	c := &confirmState{kind: "workspace", name: "myapp", attached: true}
	out := renderConfirmOverlayShared(styles.DefaultStyles(), c, 2)
	if !strings.Contains(out, "detach") {
		t.Errorf("expected detach warning, got %q", out)
	}
	if !strings.Contains(out, "myapp") {
		t.Errorf("expected name in output, got %q", out)
	}
}
