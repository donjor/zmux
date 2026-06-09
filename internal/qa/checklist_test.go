package qa

import (
	"strings"
	"testing"
)

const validChecklist = `
[checklist]
name = "Tab placements"
doc = "Walk the 027 surface"

[checklist.vars]
bin = "zzmux"

[[step]]
id = "build"
name = "Edge build"
cmd = "make install-zzmux"
expect = "build succeeds"
check = "(?i)installed"

[[step]]
id = "bar"
name = "Bar shows tabs row"
cmd = "{bin} tabs"
expect = "tabs row lists the new tab"
needs = ["build"]

[[step]]
id = "look"
expect = "the dock pill is visible in the bar"
timeout = 5
`

func TestLoad(t *testing.T) {
	fs := newMemFS()
	fs.files["/repo/qa/tab-placements.toml"] = []byte(validChecklist)

	cl, issues, err := Load(fs, "/repo/qa/tab-placements.toml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(issues) > 0 {
		t.Fatalf("unexpected lint issues: %v", issues)
	}
	if cl.Name != "Tab placements" {
		t.Errorf("Name = %q", cl.Name)
	}
	if cl.Stem != "tab-placements" {
		t.Errorf("Stem = %q", cl.Stem)
	}
	if cl.Path != "/repo/qa/tab-placements.toml" {
		t.Errorf("Path = %q", cl.Path)
	}
	if len(cl.Checksum) != 64 {
		t.Errorf("Checksum = %q, want sha256 hex", cl.Checksum)
	}
	if cl.Vars["bin"] != "zzmux" {
		t.Errorf("Vars = %v", cl.Vars)
	}
	if len(cl.Steps) != 3 {
		t.Fatalf("Steps = %d, want 3", len(cl.Steps))
	}
	if got := cl.Steps[2].Timeout; got != 5 {
		t.Errorf("step look timeout = %d", got)
	}
}

func TestLoadErrors(t *testing.T) {
	fs := newMemFS()
	if _, _, err := Load(fs, "/repo/qa/missing.toml"); err == nil {
		t.Error("missing file: want error")
	}
	fs.files["/repo/qa/bad.toml"] = []byte("not = [valid")
	if _, _, err := Load(fs, "/repo/qa/bad.toml"); err == nil {
		t.Error("bad TOML: want error")
	}
}

func TestStepSemantics(t *testing.T) {
	auto := Step{Cmd: "true", Check: "ok"}
	human := Step{Cmd: "true"}
	instruction := Step{}
	if !auto.Automatic() {
		t.Error("cmd+check should be automatic")
	}
	if human.Automatic() || instruction.Automatic() {
		t.Error("cmd-only and instruction-only steps are not automatic")
	}
}

func TestLint(t *testing.T) {
	cases := []struct {
		name string
		cl   Checklist
		want string // substring of one issue; "" = clean
	}{
		{
			name: "clean",
			cl: Checklist{Name: "x", Steps: []Step{
				{ID: "a", Expect: "ok"},
			}},
		},
		{
			name: "missing name",
			cl:   Checklist{Steps: []Step{{ID: "a", Expect: "ok"}}},
			want: "name is required",
		},
		{
			name: "no steps",
			cl:   Checklist{Name: "x"},
			want: "no steps",
		},
		{
			name: "missing id",
			cl:   Checklist{Name: "x", Steps: []Step{{Expect: "ok"}}},
			want: "id is required",
		},
		{
			name: "bad id",
			cl:   Checklist{Name: "x", Steps: []Step{{ID: "Bad ID", Expect: "ok"}}},
			want: "id must match",
		},
		{
			name: "dup id",
			cl: Checklist{Name: "x", Steps: []Step{
				{ID: "a", Expect: "ok"}, {ID: "a", Expect: "ok"},
			}},
			want: "duplicate id",
		},
		{
			name: "missing expect",
			cl:   Checklist{Name: "x", Steps: []Step{{ID: "a"}}},
			want: "expect is required",
		},
		{
			name: "check without cmd",
			cl:   Checklist{Name: "x", Steps: []Step{{ID: "a", Expect: "ok", Check: "x"}}},
			want: "check without cmd",
		},
		{
			name: "bad regexp",
			cl: Checklist{Name: "x", Steps: []Step{
				{ID: "a", Expect: "ok", Cmd: "true", Check: "(unclosed"},
			}},
			want: "not a valid Go regexp",
		},
		{
			name: "negative timeout",
			cl:   Checklist{Name: "x", Steps: []Step{{ID: "a", Expect: "ok", Timeout: -1}}},
			want: "timeout must be >= 0",
		},
		{
			name: "unknown var",
			cl: Checklist{Name: "x", Steps: []Step{
				{ID: "a", Expect: "ok", Cmd: "echo {nope}"},
			}},
			want: "unknown var",
		},
		{
			name: "needs self",
			cl: Checklist{Name: "x", Steps: []Step{
				{ID: "a", Expect: "ok", Needs: []string{"a"}},
			}},
			want: "needs itself",
		},
		{
			name: "needs dangling",
			cl: Checklist{Name: "x", Steps: []Step{
				{ID: "a", Expect: "ok", Needs: []string{"ghost"}},
			}},
			want: "needs unknown step",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			issues := Lint(&tc.cl)
			if tc.want == "" {
				if len(issues) > 0 {
					t.Fatalf("want clean, got %v", issues)
				}
				return
			}
			for _, is := range issues {
				if strings.Contains(is, tc.want) {
					return
				}
			}
			t.Fatalf("no issue containing %q in %v", tc.want, issues)
		})
	}
}

func TestExpand(t *testing.T) {
	vars := map[string]string{"bin": "zzmux", "tab": "smoke"}
	cases := []struct {
		in, want string
		wantErr  bool
	}{
		{in: "echo plain", want: "echo plain"},
		{in: "{bin} tabs", want: "zzmux tabs"},
		{in: "{bin} watch {tab}", want: "zzmux watch smoke"},
		{in: "awk '{{print $1}}'", want: "awk '{print $1}'"},
		{in: "{{bin}} is literal, {bin} is not", want: "{bin} is literal, zzmux is not"},
		{in: "echo {missing}", wantErr: true},
	}
	for _, tc := range cases {
		got, err := expand(tc.in, vars)
		if tc.wantErr {
			if err == nil {
				t.Errorf("expand(%q): want error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("expand(%q): %v", tc.in, err)
		} else if got != tc.want {
			t.Errorf("expand(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestCommand(t *testing.T) {
	cl := Checklist{Vars: map[string]string{"bin": "zzmux"}}
	s := Step{ID: "a", Cmd: "{bin} ls"}
	got, err := cl.Command(&s)
	if err != nil {
		t.Fatal(err)
	}
	if got != "zzmux ls" {
		t.Errorf("Command = %q", got)
	}
}

func TestStepByID(t *testing.T) {
	cl := Checklist{Steps: []Step{{ID: "a"}, {ID: "b"}}}
	if s := cl.StepByID("b"); s == nil || s.ID != "b" {
		t.Errorf("StepByID(b) = %v", s)
	}
	if s := cl.StepByID("nope"); s != nil {
		t.Errorf("StepByID(nope) = %v, want nil", s)
	}
}
