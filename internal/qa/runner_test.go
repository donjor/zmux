package qa

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// mockRunner returns canned results keyed by expanded command.
type mockRunner struct {
	results map[string]ExecResult
	errs    map[string]error
	calls   []mockCall
}

type mockCall struct {
	command, dir string
	timeout      time.Duration
}

func (m *mockRunner) Run(command, dir string, timeout time.Duration) (ExecResult, error) {
	m.calls = append(m.calls, mockCall{command, dir, timeout})
	if err, ok := m.errs[command]; ok {
		return ExecResult{}, err
	}
	return m.results[command], nil
}

func TestExecuteStepAutomatic(t *testing.T) {
	cl := &Checklist{Vars: map[string]string{"bin": "zzmux"}}
	step := &Step{ID: "tabs", Cmd: "{bin} tabs", Expect: "lists tabs", Check: "smoke"}

	cases := []struct {
		name string
		res  ExecResult
		want Result
	}{
		{"match+exit0", ExecResult{Exit: 0, Output: "1: smoke"}, ResultPass},
		{"no match", ExecResult{Exit: 0, Output: "1: other"}, ResultFail},
		{"match but exit1", ExecResult{Exit: 1, Output: "smoke"}, ResultFail},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &mockRunner{results: map[string]ExecResult{"zzmux tabs": tc.res}}
			rec := ExecuteStep(r, cl, step, "/repo")
			if rec.Result != tc.want {
				t.Errorf("Result = %q, want %q", rec.Result, tc.want)
			}
			if rec.Evidence == nil || rec.Evidence.Command != "zzmux tabs" {
				t.Errorf("Evidence = %+v (substitution must reach the runner)", rec.Evidence)
			}
		})
	}
}

func TestExecuteStepHumanJudged(t *testing.T) {
	cl := &Checklist{}
	step := &Step{ID: "bar", Cmd: "show-bar", Expect: "pill visible"}

	r := &mockRunner{results: map[string]ExecResult{"show-bar": {Exit: 0, Output: "ok"}}}
	if rec := ExecuteStep(r, cl, step, "/repo"); rec.Result != ResultPending {
		t.Errorf("clean run = %q, want pending", rec.Result)
	}

	r = &mockRunner{results: map[string]ExecResult{"show-bar": {Exit: 3}}}
	if rec := ExecuteStep(r, cl, step, "/repo"); rec.Result != ResultError {
		t.Errorf("failed cmd = %q, want error — never forced into pass/fail", rec.Result)
	}
}

func TestExecuteStepErrors(t *testing.T) {
	cl := &Checklist{}

	t.Run("unstartable", func(t *testing.T) {
		r := &mockRunner{errs: map[string]error{"boom": errors.New("no shell")}}
		rec := ExecuteStep(r, cl, &Step{ID: "x", Cmd: "boom", Expect: "e"}, "/repo")
		if rec.Result != ResultError || !strings.Contains(rec.Note, "no shell") {
			t.Errorf("rec = %+v", rec)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		r := &mockRunner{results: map[string]ExecResult{"slow": {Exit: -1, TimedOut: true}}}
		rec := ExecuteStep(r, cl, &Step{ID: "x", Cmd: "slow", Expect: "e", Check: "ok"}, "/repo")
		if rec.Result != ResultError || !strings.Contains(rec.Note, "timed out") {
			t.Errorf("rec = %+v", rec)
		}
	})

	t.Run("unknown var", func(t *testing.T) {
		r := &mockRunner{}
		rec := ExecuteStep(r, cl, &Step{ID: "x", Cmd: "echo {nope}", Expect: "e"}, "/repo")
		if rec.Result != ResultError || len(r.calls) != 0 {
			t.Errorf("rec = %+v, calls = %v", rec, r.calls)
		}
	})
}

func TestExecuteStepContract(t *testing.T) {
	cl := &Checklist{}
	r := &mockRunner{results: map[string]ExecResult{"x": {}}}

	ExecuteStep(r, cl, &Step{ID: "a", Cmd: "x", Expect: "e"}, "/repo")
	ExecuteStep(r, cl, &Step{ID: "b", Cmd: "x", Expect: "e", Timeout: 5}, "/repo")

	if r.calls[0].dir != "/repo" {
		t.Errorf("cwd = %q, want repo root", r.calls[0].dir)
	}
	if r.calls[0].timeout != DefaultTimeout {
		t.Errorf("default timeout = %v", r.calls[0].timeout)
	}
	if r.calls[1].timeout != 5*time.Second {
		t.Errorf("step timeout = %v", r.calls[1].timeout)
	}
}

func TestShellRunner(t *testing.T) {
	var r ShellRunner

	res, err := r.Run("echo out; echo err >&2; exit 3", t.TempDir(), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if res.Exit != 3 {
		t.Errorf("Exit = %d", res.Exit)
	}
	if !strings.Contains(res.Output, "out") || !strings.Contains(res.Output, "err") {
		t.Errorf("combined output missing a stream: %q", res.Output)
	}

	res, err = r.Run("sleep 5", t.TempDir(), 100*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if !res.TimedOut {
		t.Error("want TimedOut")
	}
}

func TestTail(t *testing.T) {
	if got := tail("short", 100); got != "short" {
		t.Errorf("no-op tail = %q", got)
	}
	long := strings.Repeat("a", 100) + "end"
	got := tail(long, 10)
	if !strings.HasSuffix(got, "end") || !strings.HasPrefix(got, "…") || len(got) > 20 {
		t.Errorf("tail = %q", got)
	}
	// Never cuts mid-rune.
	multi := strings.Repeat("é", 50)
	if cut := strings.TrimPrefix(tail(multi, 7), "…"); !strings.HasPrefix(cut, "é") {
		t.Errorf("tail split a rune: %q", cut)
	}
}
