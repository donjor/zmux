package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/capturelog"
	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/tmux"
	"github.com/spf13/cobra"
)

// optLog stores the absolute log path on the recorded pane (provenance + exact
// path), set on `log start` and cleared on `log stop`. The live on/off bit is
// tmux's own #{pane_pipe}, so this option is records-not-truth.
const optLog = "@zmux_log"

// newLogCmd is the `zmux log` verb group: persistent, background recording of a
// tab's output stream via tmux pipe-pane into a byte-bounded file. Unlike
// `snapshot` (one-shot screen state) and `watch` (interactive polling), it keeps
// recording with no client attached and self-truncates so disk never runs away.
func newLogCmd(app *apppkg.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "log",
		Short: "Record a tab's output stream to a bounded file (tail-style logging)",
		Long: `Record a tab's output to a self-truncating log file.

Backed by tmux pipe-pane: recording continues server-side with no client
attached, and the file is capped (oldest output dropped past --max-bytes).
Best for line-oriented output (servers, builds, tests); a fullscreen TUI logs
as escape soup even with stripping. For live following use ` + "`zmux watch -f`" + `.

  zmux log start server              # begin recording the "server" tab
  zmux log start build --ansi        # keep colour instead of plain text
  zmux log status                    # what is being recorded
  zmux log tail server               # print the recorded log
  zmux log stop server               # end recording`,
	}
	cmd.AddCommand(
		newLogStartCmd(app),
		newLogStopCmd(app),
		newLogStatusCmd(app),
		newLogTailCmd(app),
	)
	return cmd
}

func newLogStartCmd(app *apppkg.App) *cobra.Command {
	var sessionFlag string
	var maxBytes int
	var ansi bool
	cmd := &cobra.Command{
		Use:   "start <tab>",
		Short: "Begin recording a tab's output to a log file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			session, err := resolveSessionTarget(app, sessionFlag)
			if err != nil {
				return err
			}
			rt, err := resolveTabTarget(app, session, args[0])
			if err != nil {
				return err
			}
			pane := rt.Target

			if piped, _ := paneIsPiped(app, pane); piped {
				return fmt.Errorf("tab %q is already being logged (zmux log stop %s to end it)", args[0], args[0])
			}
			if err := app.FS.MkdirAll(app.Profile.LogsDir, 0o755); err != nil {
				return fmt.Errorf("create logs dir: %w", err)
			}
			path := logFilePath(app.Profile, session, pane)

			// The piped command runs via /bin/sh -c; shell-quote every arg
			// through the same machinery tmux uses to reconstruct commands. Use
			// the resolved self-binary so the zzmux edge profile pipes to the
			// right binary rather than a bare `zmux` on PATH.
			argv := []string{config.SelfBin(app.Profile), "log-sink", "--file", path, "--max-bytes", strconv.Itoa(maxBytes)}
			if ansi {
				argv = append(argv, "--ansi")
			}
			sink := tmux.ShellCommand(argv)
			if err := app.Runner.PipePane(pane, sink); err != nil {
				return fmt.Errorf("start pipe on %s: %w", pane, err)
			}
			_ = app.Runner.SetPaneOption(pane, optLog, path)
			fmt.Fprintf(cmd.OutOrStdout(), "● logging %s → %s\n", args[0], path)
			return nil
		},
	}
	cmd.Flags().StringVarP(&sessionFlag, "session", "s", "", "target session (default: current)")
	cmd.Flags().IntVar(&maxBytes, "max-bytes", capturelog.DefaultMaxBytes, "byte cap before oldest output is dropped")
	cmd.Flags().BoolVar(&ansi, "ansi", false, "keep ANSI colour/escapes instead of stripping to plain text")
	return cmd
}

func newLogStopCmd(app *apppkg.App) *cobra.Command {
	var sessionFlag string
	cmd := &cobra.Command{
		Use:   "stop <tab>",
		Short: "Stop recording a tab's output",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			session, err := resolveSessionTarget(app, sessionFlag)
			if err != nil {
				return err
			}
			rt, err := resolveTabTarget(app, session, args[0])
			if err != nil {
				return err
			}
			pane := rt.Target
			if piped, _ := paneIsPiped(app, pane); !piped {
				return fmt.Errorf("tab %q is not being logged", args[0])
			}
			if err := app.Runner.PipePane(pane, ""); err != nil {
				return fmt.Errorf("stop pipe on %s: %w", pane, err)
			}
			_ = app.Runner.UnsetPaneOption(pane, optLog)
			fmt.Fprintf(cmd.OutOrStdout(), "○ stopped logging %s\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVarP(&sessionFlag, "session", "s", "", "target session (default: current)")
	return cmd
}

func newLogStatusCmd(app *apppkg.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "List tabs currently being recorded",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Scan every pane on the server, not just logical tabs: `log start`
			// can record a legacy/raw window through resolveTabTarget's fallback,
			// and raw panes (no @zmux_tab_id) never appear in ListLogicalTabs —
			// so a managed-tabs-only scan would report "nothing" while a raw
			// recording is live.
			rows, err := app.Runner.ListLogicalPaneRows()
			if err != nil {
				return fmt.Errorf("list panes: %w", err)
			}
			out := cmd.OutOrStdout()
			tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			n := 0
			for i := range rows {
				row := &rows[i]
				piped, _ := paneIsPiped(app, row.PaneID)
				path, _ := app.Runner.ShowPaneOption(row.PaneID, optLog)
				if !piped && path == "" {
					continue
				}
				if n == 0 {
					fmt.Fprintln(tw, "\tTAB\tPANE\tSIZE\tFILE")
				}
				n++
				mark := "○" // pipe gone but option lingers (stale)
				if piped {
					mark = "●"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", mark, logRowName(row), row.PaneID, logSize(app, path), pathOrDash(path))
			}
			if n == 0 {
				fmt.Fprintln(out, "no tabs are being logged")
				return nil
			}
			return tw.Flush()
		},
	}
	return cmd
}

func newLogTailCmd(app *apppkg.App) *cobra.Command {
	var sessionFlag string
	var lines int
	cmd := &cobra.Command{
		Use:   "tail <tab>",
		Short: "Print a tab's recorded log",
		Long: `Print the recorded log for a tab (already bounded to --max-bytes).
For live following of a running tab, use ` + "`zmux watch <tab> -f`" + `.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			session, err := resolveSessionTarget(app, sessionFlag)
			if err != nil {
				return err
			}
			rt, err := resolveTabTarget(app, session, args[0])
			if err != nil {
				return err
			}
			path := logPathFor(app, rt.Target, session)
			data, err := app.FS.ReadFile(path)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("no log for tab %q (zmux log start %s to begin)", args[0], args[0])
				}
				return fmt.Errorf("read log: %w", err)
			}
			fmt.Fprint(cmd.OutOrStdout(), lastLines(string(data), lines))
			return nil
		},
	}
	cmd.Flags().StringVarP(&sessionFlag, "session", "s", "", "target session (default: current)")
	cmd.Flags().IntVarP(&lines, "lines", "n", 0, "limit to the last N lines (0 = whole log)")
	return cmd
}

// logRowName is the display name for a recorded pane in `log status`. A managed
// tab uses its label (else its id), mirroring tabs.DisplayName; a raw/legacy
// window logged via the fallback target has no label, so its window name (else
// pane id) stands in.
func logRowName(row *tmux.LogicalPaneRow) string {
	if row.TabID != "" {
		if row.Label != "" {
			return row.Label
		}
		return row.TabID
	}
	if row.WindowName != "" {
		return row.WindowName
	}
	return row.PaneID
}

// paneIsPiped reports whether tmux is currently piping the pane's output.
func paneIsPiped(app *apppkg.App, pane string) (bool, error) {
	out, err := app.Runner.DisplayMessage(pane, "#{pane_pipe}")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "1", nil
}

// logPathFor prefers the path recorded on the pane (@zmux_log), falling back to
// the deterministic path so a tail still works if the option was lost.
func logPathFor(app *apppkg.App, pane, session string) string {
	if p, _ := app.Runner.ShowPaneOption(pane, optLog); strings.TrimSpace(p) != "" {
		return p
	}
	return logFilePath(app.Profile, session, pane)
}

// logFilePath is the deterministic recording path for a (session, pane) pair.
func logFilePath(p config.Profile, session, paneID string) string {
	return filepath.Join(p.LogsDir, sanitizeLogName(session)+"__"+sanitizeLogName(paneID)+".log")
}

// sanitizeLogName reduces an identifier to a filesystem-safe token.
func sanitizeLogName(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "pane"
	}
	return out
}

// lastLines returns the last n lines of s (n <= 0 returns all). A trailing
// newline is preserved and does not count as a line.
func lastLines(s string, n int) string {
	if n <= 0 || s == "" {
		return s
	}
	trailing, body := "", s
	if strings.HasSuffix(body, "\n") {
		trailing, body = "\n", body[:len(body)-1]
	}
	parts := strings.Split(body, "\n")
	if len(parts) <= n {
		return s
	}
	return strings.Join(parts[len(parts)-n:], "\n") + trailing
}

func logSize(app *apppkg.App, path string) string {
	if path == "" {
		return "-"
	}
	info, err := app.FS.Stat(path)
	if err != nil {
		return "-"
	}
	return humanBytes(info.Size())
}

func pathOrDash(path string) string {
	if path == "" {
		return "-"
	}
	return path
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%dB", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(n)/float64(div), "KMGT"[exp])
}
