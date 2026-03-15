package tmux

import (
	"strings"
	"testing"
)

func TestClipboardBindingWlCopy(t *testing.T) {
	binding := ClipboardBinding("wl-copy")
	if !strings.Contains(binding, "wl-copy") {
		t.Errorf("expected wl-copy in binding, got %q", binding)
	}
	if !strings.Contains(binding, "copy-pipe-and-cancel") {
		t.Errorf("expected copy-pipe-and-cancel in binding, got %q", binding)
	}
	if !strings.Contains(binding, "copy-mode-vi y") {
		t.Errorf("expected copy-mode-vi y in binding, got %q", binding)
	}
}

func TestClipboardBindingXclip(t *testing.T) {
	binding := ClipboardBinding("xclip")
	if !strings.Contains(binding, "xclip -selection clipboard") {
		t.Errorf("expected 'xclip -selection clipboard' in binding, got %q", binding)
	}
	if !strings.Contains(binding, "copy-pipe-and-cancel") {
		t.Errorf("expected copy-pipe-and-cancel in binding, got %q", binding)
	}
}

func TestClipboardBindingPbcopy(t *testing.T) {
	binding := ClipboardBinding("pbcopy")
	if !strings.Contains(binding, "pbcopy") {
		t.Errorf("expected pbcopy in binding, got %q", binding)
	}
	if !strings.Contains(binding, "copy-pipe-and-cancel") {
		t.Errorf("expected copy-pipe-and-cancel in binding, got %q", binding)
	}
}

func TestClipboardBindingFallback(t *testing.T) {
	binding := ClipboardBinding("")
	if !strings.Contains(binding, "copy-selection-and-cancel") {
		t.Errorf("expected copy-selection-and-cancel in fallback binding, got %q", binding)
	}
}

func TestClipboardBindingUnknownTool(t *testing.T) {
	binding := ClipboardBinding("some-unknown-tool")
	if !strings.Contains(binding, "copy-selection-and-cancel") {
		t.Errorf("expected copy-selection-and-cancel for unknown tool, got %q", binding)
	}
}
