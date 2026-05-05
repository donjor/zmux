// Package tablabel defines zmux's optional stable tab label overlay.
package tablabel

import "fmt"

const (
	Option              = "@zmux_label"
	SourceOption        = "@zmux_label_source"
	DuplicateNameOption = "@zmux_duplicate_name"
	SourceManual        = "manual"
	SourcePane          = "pane"
)

// Format returns a tmux format for a window display name.
//
// Without a zmux label, it renders tmux's normal window name (#W), plus a dimmed
// cwd suffix like "[zmux]" when zmux has marked the window name as duplicated in
// the current session. With a zmux label, it renders "label [auto]" where the
// bracketed auto-name is dimmed; manual labels are already the disambiguator.
func Format(dimColor string) string {
	auto := fmt.Sprintf("#W#{?%s,#[fg=%s][#{b:pane_current_path}],}", DuplicateNameOption, dimColor)
	return fmt.Sprintf("#{?%s,#{?#{==:#{%s},#W},#W,#{%s} #[fg=%s][#W]},%s}", Option, Option, Option, dimColor, auto)
}

// PlainFormat is a label-aware window name without style escapes.
func PlainFormat() string {
	auto := fmt.Sprintf("#W#{?%s,[#{b:pane_current_path}],}", DuplicateNameOption)
	return fmt.Sprintf("#{?%s,#{?#{==:#{%s},#W},#W,#{%s} [#W]},%s}", Option, Option, Option, auto)
}
