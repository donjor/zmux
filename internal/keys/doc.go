package keys

import (
	_ "embed"
	"strings"
	"text/template"
)

//go:embed keybindings.tmpl.md
var docTemplate string

// docData is the view model passed to the docs template.
type docData struct {
	Prefix    []Binding
	NoPrefix  []Binding
	CopyMode  []Binding
	Inherited []Binding
	Dashboard []Binding
}

// mdCode wraps s as a markdown inline code span, using double-backtick
// delimiters when s itself contains a backtick (e.g. the "Alt+`" key) so the
// span renders correctly.
func mdCode(s string) string {
	if strings.Contains(s, "`") {
		return "`` " + s + " ``"
	}
	return "`" + s + "`"
}

// GenerateDoc renders docs/keybindings.md from the registry. The binding tables
// are generated from the registry; surrounding prose lives in the embedded
// template. Output is deterministic so `zmux keys gen --check` can golden-diff
// it in CI.
func GenerateDoc() (string, error) {
	tmpl, err := template.New("keybindings").Funcs(template.FuncMap{"code": mdCode}).Parse(docTemplate)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	data := docData{
		Prefix:    PrefixBindings,
		NoPrefix:  NoPrefixBindings,
		CopyMode:  CopyModeBindings,
		Inherited: InheritedBindings,
		Dashboard: DashboardBindings,
	}
	if err := tmpl.Execute(&b, data); err != nil {
		return "", err
	}
	out := b.String()
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return out, nil
}
