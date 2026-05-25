package pane

// Workload fixtures for the pane preview page. Each fixture set models a
// real situation (coding, services, review, agent) so the preview shows
// representative pane chrome — not lorem-ipsum filler.

type paneSpec struct {
	Slot    string
	ID      string
	Title   string
	Command string
	CWD     string
	Size    string
	Focused bool
	State   string
	Lines   []string
}

func fixturePanes(workload string, cleanUIExample bool, auxState string, focus string) []paneSpec {
	primary := paneSpec{
		Slot:    focusPrimary,
		ID:      "%11",
		Title:   "editor",
		Command: "nvim",
		CWD:     "~/donjor/zmux",
		Size:    "142×52",
		Focused: focus == focusPrimary,
		State:   stateRunning,
		Lines: []string{
			"internal/tmux/conf.go",
			"internal/preview/pane/page.go",
			"cmd/zmux/pane.go",
			"",
			"// Pane chrome should make focus and purpose obvious",
			"// before the user reads pane contents.",
		},
	}
	secondary := paneSpec{
		Slot:    focusSecondary,
		ID:      "%12",
		Title:   "server",
		Command: "go run ./cmd/api",
		CWD:     "~/donjor/myapp",
		Size:    "86×52",
		Focused: focus == focusSecondary,
		State:   auxState,
		Lines: []string{
			"listening on :8080",
			"GET /health 200 2ms",
			"GET /api/sessions 200 8ms",
			"",
			"hot reload: ready",
		},
	}
	tertiary := paneSpec{
		Slot:    focusTertiary,
		ID:      "%13",
		Title:   "tests",
		Command: "go test ./...",
		CWD:     "~/donjor/zmux",
		Size:    "86×18",
		Focused: focus == focusTertiary,
		State:   stateRunning,
		Lines: []string{
			"ok  github.com/donjor/zmux/cmd/zmux",
			"ok  github.com/donjor/zmux/internal/tmux",
			"ok  github.com/donjor/zmux/internal/tui/dashboard/tabs",
			"",
			"watching for changes…",
		},
	}

	switch workload {
	case workloadServices:
		primary.Title, primary.Command = "api", "air ./cmd/api"
		primary.Lines = []string{"POST /v1/chat 200 418ms", "GET /v1/tasks 200 12ms", "worker queue depth: 3", "cache hit rate: 94%"}
		secondary.Title, secondary.Command = "worker", "npm run worker"
		secondary.CWD = "~/donjor/myapp/services/worker"
		secondary.Lines = []string{"job sync_catalog done", "job refresh_cache running", "job emit_metrics pending", "", "press pfx+q to reveal pane ids"}
		tertiary.Title, tertiary.Command = "logs", "tail -f app.log"
		tertiary.Lines = []string{"INFO deploy sha=9f32", "WARN retry payment-webhook", "INFO queue drained"}
	case workloadReview:
		primary.Title, primary.Command = "diff", "git diff"
		primary.Lines = []string{"diff --git a/internal/tmux/conf.go b/internal/tmux/conf.go", "+ set -g pane-border-status top", "+ set -g pane-active-border-style fg=cyan,bold", "", "Reviewing whether this should graduate from proto."}
		secondary.Title, secondary.Command = "notes", "vim docs/review.md"
		secondary.Lines = []string{"Review checklist", "✓ current pane command", "✓ toggle command", "→ pane border design", "", "Question: compact or verbose header?"}
		tertiary.Title, tertiary.Command = "shell", "git status"
		tertiary.Lines = []string{"## master...origin/master [ahead 45]", " M internal/preview/pane/page.go"}
	case workloadAgent:
		primary.Title, primary.Command = "pi", "pi"
		primary.Lines = []string{"Mission: general pane visual system", "", "User: panes in a general sense; sidecar is just a feature", "", "Assistant: updating prototype boundaries."}
		secondary.Title, secondary.Command = "captain", "zmux watch captain"
		secondary.Lines = []string{"Tasks 2/4 done · 1 active", "→ implement generalized pane prototype", "", "No global status clutter."}
		tertiary.Title, tertiary.Command = "scratch", "rg pane"
		tertiary.Lines = []string{"cmd/zmux/pane.go", "internal/tui/dashboard/tabs/current_tree.go", "internal/preview/pane/page.go"}
	}

	if cleanUIExample {
		secondary = cleanUIPane(auxState, focus == focusSecondary)
	}
	return []paneSpec{primary, secondary, tertiary}
}

func cleanUIPane(state string, focused bool) paneSpec {
	mode := "live"
	if state == stateStale {
		mode = "degraded"
	}
	return paneSpec{
		Slot:    focusSecondary,
		ID:      "%73",
		Title:   "clean-ui",
		Command: "clean-ui-sidecar",
		CWD:     "~/pi-extensions/pi-clean-ui",
		Size:    "92×52",
		Focused: focused,
		State:   state,
		Lines: []string{
			"clean-ui sidecar  operations cockpit      seq 42 · gen 3",
			"~/donjor/zmux",
			"",
			"▸ watch   tasks   artifacts",
			"━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━",
			"WATCH  7   TASKS  3/4   ARTIFACTS  2",
			"",
			"✓ edit internal/preview/pane/page.go · +142/-88",
			"→ task: generalize pane prototype",
			"",
			"j/k select · h/l tabs · q quit          " + mode,
		},
	}
}

func narrowPanes(layout string, panes []paneSpec) []paneSpec {
	if layout == layoutSplit && len(panes) > 2 {
		return panes[:2]
	}
	if layout == layoutFocusRail {
		ordered := make([]paneSpec, 0, len(panes))
		for _, pane := range panes {
			if pane.Focused {
				ordered = append(ordered, pane)
			}
		}
		for _, pane := range panes {
			if !pane.Focused {
				ordered = append(ordered, pane)
			}
		}
		if len(ordered) > 0 {
			return ordered
		}
	}
	return panes
}
