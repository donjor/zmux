package waitfor

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/donjor/zmux/internal/tabs"
	"github.com/donjor/zmux/internal/tmux"
)

type Basis string

const (
	BasisTurnState   Basis = "turnState"
	BasisCmdState    Basis = "cmdState"
	BasisOutput      Basis = "outputRegex"
	BasisIdle        Basis = "idleFallback"
	BasisTimeout     Basis = "timeout"
	BasisUnavailable Basis = "unavailable"
)

type ConditionKind string

const (
	ConditionTurn   ConditionKind = "turn"
	ConditionCmd    ConditionKind = "cmd"
	ConditionOutput ConditionKind = "output"
	ConditionIdle   ConditionKind = "idle"
)

type Condition struct {
	Kind  ConditionKind `json:"kind"`
	Value string        `json:"value,omitempty"`
	Idle  time.Duration `json:"-"`
}

type Status struct {
	PaneID       string `json:"paneId,omitempty"`
	CmdState     string `json:"cmdState,omitempty"`
	CmdSeq       string `json:"cmdSeq,omitempty"`
	LastExit     string `json:"lastExit,omitempty"`
	RunID        string `json:"runId,omitempty"`
	TurnState    string `json:"turnState,omitempty"`
	TurnAt       string `json:"turnAt,omitempty"`
	TurnSeq      string `json:"turnSeq,omitempty"`
	PeerTurns    string `json:"peerTurns,omitempty"`
	Scope        string `json:"scope,omitempty"`
	Origin       string `json:"origin,omitempty"`
	Command      string `json:"command,omitempty"`
	ResolvedTurn string `json:"resolvedTurn,omitempty"`
}

type Outcome struct {
	Met           bool     `json:"met"`
	Basis         Basis    `json:"basis"`
	State         string   `json:"state,omitempty"`
	Fresh         bool     `json:"fresh"`
	Status        Status   `json:"status"`
	OutputTail    string   `json:"outputTail,omitempty"`
	Warnings      []string `json:"warnings,omitempty"`
	FailureKind   string   `json:"failureKind,omitempty"`
	AlreadyInTail bool     `json:"alreadyInTail,omitempty"`
}

type Baseline struct {
	CmdSeq    int
	CmdState  string
	TurnSeq   int
	TurnState string
}

type Request struct {
	Runner       tmux.Runner
	Target       string
	PaneID       string
	Lines        int
	Timeout      time.Duration
	PollInterval time.Duration
	Condition    Condition
	AllowStale   bool
	Baseline     *Baseline
}

func ParseCondition(spec string) (Condition, error) {
	spec = strings.TrimSpace(spec)
	kind, value, ok := strings.Cut(spec, ":")
	if !ok || strings.TrimSpace(kind) == "" || strings.TrimSpace(value) == "" {
		return Condition{}, fmt.Errorf("condition must be kind:value (turn:ready, cmd:done, output:<regex>, idle:3s)")
	}
	kind = strings.TrimSpace(kind)
	value = strings.TrimSpace(value)
	switch ConditionKind(kind) {
	case ConditionTurn:
		state := tabs.NormalizeTurnState(value)
		switch state {
		case tabs.TurnRunning, tabs.TurnReady, tabs.TurnAttention, tabs.TurnFailed, tabs.TurnConsumed, tabs.TurnParked:
			return Condition{Kind: ConditionTurn, Value: state}, nil
		default:
			return Condition{}, fmt.Errorf("unknown turn state %q", value)
		}
	case ConditionCmd:
		if strings.HasPrefix(value, "exit=") {
			if _, err := strconv.Atoi(strings.TrimPrefix(value, "exit=")); err != nil {
				return Condition{}, fmt.Errorf("invalid cmd exit condition %q", value)
			}
			return Condition{Kind: ConditionCmd, Value: value}, nil
		}
		switch value {
		case tabs.CmdRunning, tabs.CmdDone, tabs.CmdFailed:
			return Condition{Kind: ConditionCmd, Value: value}, nil
		default:
			return Condition{}, fmt.Errorf("unknown command state %q", value)
		}
	case ConditionOutput:
		if _, err := regexp.Compile("(?i)" + value); err != nil {
			return Condition{}, fmt.Errorf("invalid output regex %q: %w", value, err)
		}
		return Condition{Kind: ConditionOutput, Value: value}, nil
	case ConditionIdle:
		d, err := parseIdleDuration(value)
		if err != nil {
			return Condition{}, err
		}
		return Condition{Kind: ConditionIdle, Value: d.String(), Idle: d}, nil
	default:
		return Condition{}, fmt.Errorf("unknown condition kind %q", kind)
	}
}

func parseIdleDuration(raw string) (time.Duration, error) {
	if n, err := strconv.Atoi(raw); err == nil {
		if n <= 0 {
			return 0, fmt.Errorf("idle duration must be positive")
		}
		return time.Duration(n) * time.Second, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("idle duration must be a positive duration or seconds count")
	}
	return d, nil
}

func SnapshotBaseline(r tmux.Runner, paneID string) Baseline {
	st := ReadStatus(r, paneID)
	return Baseline{
		CmdSeq:    atoi(st.CmdSeq),
		CmdState:  strings.TrimSpace(st.CmdState),
		TurnSeq:   statusTurnSeq(st),
		TurnState: tabs.NormalizeTurnState(strings.TrimSpace(st.TurnState)),
	}
}

func ReadStatus(r tmux.Runner, paneID string) Status {
	if paneID == "" {
		return Status{}
	}
	get := func(key string) string {
		v, _ := r.ShowPaneOption(paneID, key)
		return strings.TrimSpace(v)
	}
	st := Status{PaneID: paneID}
	st.CmdState = get(tabs.OptCmdState)
	st.CmdSeq = get(tabs.OptCmdSeq)
	st.LastExit = get(tabs.OptCmdLastExit)
	st.RunID = get(tabs.OptCmdRunID)
	st.TurnState = tabs.NormalizeTurnState(get(tabs.OptTurnState))
	st.TurnAt = get(tabs.OptTurnAt)
	st.TurnSeq = get(tabs.OptTurnSeq)
	st.PeerTurns = get(tabs.OptPeerTurns)
	st.Scope = get(tabs.OptScope)
	st.Origin = get(tabs.OptOrigin)
	st.Command = get(tabs.OptCmdText)
	if st.TurnState != "" {
		st.ResolvedTurn = st.TurnState
	}
	return st
}

func Wait(ctx context.Context, req Request) (Outcome, error) {
	if req.Runner == nil {
		return Outcome{}, fmt.Errorf("wait request requires runner")
	}
	if req.Target == "" {
		req.Target = req.PaneID
	}
	if req.PaneID == "" {
		req.PaneID = req.Target
	}
	if req.Lines <= 0 {
		req.Lines = 120
	}
	if req.Timeout <= 0 {
		req.Timeout = 10 * time.Second
	}
	if req.PollInterval <= 0 {
		req.PollInterval = 500 * time.Millisecond
	}
	baseline := SnapshotBaseline(req.Runner, req.PaneID)
	if req.Baseline != nil {
		baseline = *req.Baseline
	}
	switch req.Condition.Kind {
	case ConditionOutput:
		return waitOutput(ctx, req, baseline)
	case ConditionIdle:
		return waitIdle(ctx, req, baseline)
	case ConditionCmd, ConditionTurn:
		return waitLifecycle(ctx, req, baseline)
	default:
		return Outcome{}, fmt.Errorf("unsupported wait condition %q", req.Condition.Kind)
	}
}

func waitOutput(ctx context.Context, req Request, baseline Baseline) (Outcome, error) {
	re, err := regexp.Compile("(?i)" + req.Condition.Value)
	if err != nil {
		return Outcome{}, err
	}
	base := map[string]int{}
	alreadyInTail := false
	if initial, err := req.Runner.CapturePane(req.Target, req.Lines); err == nil {
		for _, line := range strings.Split(initial, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			base[trimmed]++
			if re.MatchString(line) {
				alreadyInTail = true
			}
		}
	}
	deadline := time.Now().Add(req.Timeout)
	ticker := time.NewTicker(req.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return timeoutOutcome(req, baseline, "interrupted"), ctx.Err()
		case <-ticker.C:
			out, err := req.Runner.CapturePane(req.Target, req.Lines)
			if err != nil {
				if time.Now().After(deadline) {
					oc := timeoutOutcome(req, baseline, "capture_failed")
					oc.Warnings = append(oc.Warnings, err.Error())
					return oc, nil
				}
				continue
			}
			seen := map[string]int{}
			for _, line := range strings.Split(out, "\n") {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" {
					continue
				}
				seen[trimmed]++
				if seen[trimmed] <= base[trimmed] {
					continue
				}
				if re.MatchString(line) {
					return Outcome{Met: true, Basis: BasisOutput, Fresh: true, State: req.Condition.Value, Status: ReadStatus(req.Runner, req.PaneID), OutputTail: out}, nil
				}
			}
			if time.Now().After(deadline) {
				failure := "output_regex_unproven"
				fresh := true
				var warnings []string
				if alreadyInTail {
					failure = "output_regex_already_present"
					fresh = false
					warnings = []string{"output regex was already present before wait baseline"}
				}
				oc := timeoutOutcome(req, baseline, failure)
				oc.Fresh = fresh
				oc.AlreadyInTail = alreadyInTail
				oc.Warnings = append(oc.Warnings, warnings...)
				oc.OutputTail = out
				return oc, nil
			}
		}
	}
}

func waitIdle(ctx context.Context, req Request, baseline Baseline) (Outcome, error) {
	idle := req.Condition.Idle
	if idle <= 0 {
		idle = time.Second
	}
	deadline := time.Now().Add(req.Timeout)
	ticker := time.NewTicker(req.PollInterval)
	defer ticker.Stop()
	last := ""
	stableSince := time.Time{}
	if out, err := req.Runner.CapturePane(req.Target, req.Lines); err == nil {
		last = out
		stableSince = time.Now()
	}
	for {
		select {
		case <-ctx.Done():
			return timeoutOutcome(req, baseline, "interrupted"), ctx.Err()
		case <-ticker.C:
			out, err := req.Runner.CapturePane(req.Target, req.Lines)
			if err == nil {
				if stableSince.IsZero() || out != last {
					last = out
					stableSince = time.Now()
				} else if time.Since(stableSince) >= idle {
					return Outcome{Met: true, Basis: BasisIdle, Fresh: true, State: idle.String(), Status: ReadStatus(req.Runner, req.PaneID), OutputTail: out}, nil
				}
			}
			if time.Now().After(deadline) {
				oc := timeoutOutcome(req, baseline, "idle_unproven")
				oc.OutputTail = last
				if err != nil {
					oc.Warnings = append(oc.Warnings, err.Error())
				}
				return oc, nil
			}
		}
	}
}

func waitLifecycle(ctx context.Context, req Request, baseline Baseline) (Outcome, error) {
	deadline := time.Now().Add(req.Timeout)
	ticker := time.NewTicker(req.PollInterval)
	defer ticker.Stop()
	for {
		st := ReadStatus(req.Runner, req.PaneID)
		if oc, done := classifyLifecycle(req, baseline, st); done {
			if oc.OutputTail == "" {
				oc.OutputTail, _ = req.Runner.CapturePane(req.Target, req.Lines)
			}
			return oc, nil
		}
		if time.Now().After(deadline) {
			oc := timeoutOutcome(req, baseline, string(req.Condition.Kind)+"_unproven")
			oc.Status = st
			oc.OutputTail, _ = req.Runner.CapturePane(req.Target, req.Lines)
			return oc, nil
		}
		select {
		case <-ctx.Done():
			return timeoutOutcome(req, baseline, "interrupted"), ctx.Err()
		case <-ticker.C:
		}
	}
}

func classifyLifecycle(req Request, baseline Baseline, st Status) (Outcome, bool) {
	switch req.Condition.Kind {
	case ConditionTurn:
		state := tabs.NormalizeTurnState(st.TurnState)
		fresh := req.AllowStale || freshTurn(baseline, st)
		oc := Outcome{Basis: BasisTurnState, State: state, Fresh: fresh, Status: st}
		if state == req.Condition.Value && fresh {
			oc.Met = true
			return oc, true
		}
		if fresh && state == tabs.TurnFailed && req.Condition.Value != tabs.TurnFailed {
			oc.FailureKind = "turn_failed"
			oc.Warnings = []string{"peer turn state is failed"}
			return oc, true
		}
		if fresh && state == tabs.TurnAttention && req.Condition.Value != tabs.TurnAttention {
			oc.FailureKind = "turn_attention"
			oc.Warnings = []string{"peer turn state requires attention"}
			return oc, true
		}
	case ConditionCmd:
		fresh := req.AllowStale || freshCmd(baseline, st, req.Condition.Value)
		oc := Outcome{Basis: BasisCmdState, State: st.CmdState, Fresh: fresh, Status: st}
		if cmdMatches(req.Condition.Value, st) && fresh {
			oc.Met = true
			if strings.HasPrefix(req.Condition.Value, "exit=") {
				oc.State = "exit=" + st.LastExit
			}
			return oc, true
		}
		if fresh && st.CmdState == tabs.CmdFailed && req.Condition.Value != tabs.CmdFailed {
			oc.FailureKind = "cmd_failed"
			if st.LastExit != "" {
				oc.FailureKind = "cmd_exit"
				oc.Warnings = []string{"command exited with " + st.LastExit}
			} else {
				oc.Warnings = []string{"command lifecycle state is failed"}
			}
			return oc, true
		}
		if fresh && strings.HasPrefix(req.Condition.Value, "exit=") && (st.CmdState == tabs.CmdDone || st.CmdState == tabs.CmdFailed) {
			oc.FailureKind = "cmd_exit"
			oc.State = "exit=" + st.LastExit
			oc.Warnings = []string{"command exited with " + emptyDefault(st.LastExit, "unknown")}
			return oc, true
		}
	}
	return Outcome{}, false
}

func timeoutOutcome(req Request, baseline Baseline, failure string) Outcome {
	st := ReadStatus(req.Runner, req.PaneID)
	basis := BasisTimeout
	switch req.Condition.Kind {
	case ConditionCmd:
		basis = BasisCmdState
	case ConditionTurn:
		basis = BasisTurnState
	case ConditionOutput:
		basis = BasisOutput
	case ConditionIdle:
		basis = BasisIdle
	}
	return Outcome{Met: false, Basis: basis, State: stateFor(req.Condition, st), Fresh: req.AllowStale || freshnessFor(req.Condition, baseline, st), Status: st, FailureKind: failure}
}

func stateFor(cond Condition, st Status) string {
	switch cond.Kind {
	case ConditionCmd:
		if strings.HasPrefix(cond.Value, "exit=") && st.LastExit != "" {
			return "exit=" + st.LastExit
		}
		return st.CmdState
	case ConditionTurn:
		return st.TurnState
	case ConditionOutput:
		return cond.Value
	case ConditionIdle:
		return cond.Value
	default:
		return ""
	}
}

func freshnessFor(cond Condition, baseline Baseline, st Status) bool {
	switch cond.Kind {
	case ConditionCmd:
		return freshCmd(baseline, st, cond.Value)
	case ConditionTurn:
		return freshTurn(baseline, st)
	case ConditionOutput, ConditionIdle:
		return true
	default:
		return false
	}
}

func freshCmd(b Baseline, st Status, want string) bool {
	seq := atoi(st.CmdSeq)
	if seq <= 0 {
		return false
	}
	if seq > b.CmdSeq {
		return true
	}
	if seq == b.CmdSeq && b.CmdState != "" && b.CmdState != st.CmdState && cmdMatches(want, st) {
		return true
	}
	return false
}

func freshTurn(b Baseline, st Status) bool {
	seq := statusTurnSeq(st)
	if seq <= 0 {
		return false
	}
	if seq > b.TurnSeq {
		return true
	}
	state := tabs.NormalizeTurnState(st.TurnState)
	if seq == b.TurnSeq && b.TurnState != "" && b.TurnState != state {
		return true
	}
	return false
}

func cmdMatches(want string, st Status) bool {
	if strings.HasPrefix(want, "exit=") {
		return (st.CmdState == tabs.CmdDone || st.CmdState == tabs.CmdFailed) && st.LastExit == strings.TrimPrefix(want, "exit=")
	}
	return st.CmdState == want
}

func statusTurnSeq(st Status) int {
	if n := atoi(st.TurnSeq); n > 0 {
		return n
	}
	return atoi(st.PeerTurns)
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func emptyDefault(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return strings.TrimSpace(s)
}

func WarningsForStatus(st Status) []string {
	var warnings []string
	if st.LastExit != "" && st.LastExit != "0" {
		warnings = append(warnings, "last command exited with "+st.LastExit)
	}
	if st.CmdState == tabs.CmdFailed {
		warnings = append(warnings, "command lifecycle state is failed")
	}
	switch tabs.NormalizeTurnState(st.TurnState) {
	case tabs.TurnFailed:
		warnings = append(warnings, "peer turn state is failed")
	case tabs.TurnAttention:
		warnings = append(warnings, "peer turn state requires attention")
	}
	return warnings
}
