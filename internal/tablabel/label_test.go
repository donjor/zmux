package tablabel

import (
	"strings"
	"testing"
)

func TestFormatRendersLabelOverlay(t *testing.T) {
	got := Format("#505050")
	for _, want := range []string{Option, DuplicateNameOption, "#{==:#{@zmux_label},#W}", "#{@zmux_label} #[fg=#505050][#W]", "#W#{?@zmux_duplicate_name,#[fg=#505050][#{b:pane_current_path}],}"} {
		if !strings.Contains(got, want) {
			t.Fatalf("Format missing %q: %s", want, got)
		}
	}
}

func TestPlainFormatRendersLabelOverlayWithoutStyles(t *testing.T) {
	got := PlainFormat()
	for _, want := range []string{Option, DuplicateNameOption, "#{==:#{@zmux_label},#W}", "#{@zmux_label} [#W]", "#W#{?@zmux_duplicate_name,[#{b:pane_current_path}],}"} {
		if !strings.Contains(got, want) {
			t.Fatalf("PlainFormat missing %q: %s", want, got)
		}
	}
	if strings.Contains(got, "#[") {
		t.Fatalf("PlainFormat should not include style escapes: %s", got)
	}
}
