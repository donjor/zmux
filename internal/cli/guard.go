package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/guard"
	"github.com/spf13/cobra"
)

// targetSuggestion maps a guard suggestion key to the concrete zmux CLI form
// (the shell surface for Claude / Codex; pi renders the same keys to typed tools).
var targetSuggestion = map[string]string{
	"watch":        "zmux watch <tab>",
	"send":         "zmux send <tab> <keys>   (or zmux type <tab> 'text')",
	"tabs":         "zmux tabs",
	"ls":           "zmux ls",
	"pane-list":    "zmux pane list --json",
	"pane-open":    "zmux pane open <name> -r 35 -- <cmd>",
	"pane-focus":   "zmux pane focus <pane>",
	"pane-close":   "zmux pane close <pane>",
	"pane-resize":  "zmux pane resize <pane> --size 40%",
	"run":          "zmux run '<cmd>' -n <name>",
	"tab-kill":     "zmux tab kill <tab>",
	"tab-label":    "zmux tab label '<label>'",
	"tab-move":     "zmux tab move <tab> <dest-session>",
	"new":          "zmux new <ws> [session]",
	"session-kill": "zmux session kill <session>",
	"open":         "zmux open <ws> [session]",
	"runtime":      "zmux run '<cmd>' -n <name> -d   (keeps it in a visible, named tab)",
	"interactive":  "run it in a shared tab — zmux run '<cmd>' -n admin -d, then drive it — so it stays visible",
}

func newGuardCmd(app *apppkg.App) *cobra.Command {
	var jsonOut bool
	var cwd string

	cmd := &cobra.Command{
		Use:   "guard [command]",
		Short: "Classify a shell command for terminal-hygiene enforcement",
		Long: `Classify a shell command: does it reach past zmux (raw tmux), start a
dev server or background job that should live in a named tab, or need shared
interaction (sudo/ssh/REPL)?

The command is read from args or stdin. Exit code is 2 when the command should
be blocked, 0 otherwise (allow/warn). --json prints the full verdict. The ruleset
is shared with the Claude hook and pi-extension via testdata/zmux-guard-corpus.jsonl.

Bypass any verdict with a ZMUX_ALLOW=1 prefix or a "# zmux: allow" comment.`,
		Hidden: true, // agent-facing tool, not part of the everyday user surface
		Args:   cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			command := strings.TrimSpace(strings.Join(args, " "))
			if command == "" {
				data, err := io.ReadAll(cmd.InOrStdin())
				if err != nil {
					return err
				}
				command = strings.TrimSpace(string(data))
			}
			if command == "" {
				return fmt.Errorf("no command given — pass it as args or on stdin")
			}

			res := guard.Classify(command, guard.Options{RepoCwd: isZmuxRepo(app, cwd)})

			if jsonOut {
				enc, err := json.MarshalIndent(res, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(enc))
			} else if res.Decision != guard.Allow {
				fmt.Fprintln(cmd.ErrOrStderr(), guardMessage(res))
			}

			if res.Decision == guard.Block {
				// 2 = blocked (matches the PreToolUse hook convention); message
				// already printed above, so carry an empty codedError.
				return &codedError{code: 2}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print the full verdict as JSON")
	cmd.Flags().StringVar(&cwd, "cwd", "", "cwd to evaluate the repo exemption against (default: current dir)")
	return cmd
}

// guardMessage renders a human/agent-readable line for a non-allow verdict.
func guardMessage(res guard.Result) string {
	label := "zmux guard"
	switch res.Decision {
	case guard.Block:
		label += " BLOCK"
	case guard.Warn:
		label += " WARN"
	}
	out := fmt.Sprintf("%s [%s]: %s", label, res.Kind, res.Reason)
	if s := targetSuggestion[res.Target]; s != "" {
		out += "\n  → " + s
	}
	return out
}

// isZmuxRepo reports whether dir (default: the process cwd) sits inside the zmux
// source tree — where raw tmux is a legitimate dev tool and is therefore exempt.
// Detected by walking up to a go.mod declaring this module, not a hardcoded path.
func isZmuxRepo(app *apppkg.App, dir string) bool {
	if dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return false
		}
		dir = wd
	}
	for i := 0; i < 40; i++ {
		if data, err := app.FS.ReadFile(filepath.Join(dir, "go.mod")); err == nil {
			if strings.Contains(string(data), "module github.com/donjor/zmux") {
				return true
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return false
}
