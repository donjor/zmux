package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	apppkg "github.com/donjor/zmux/internal/app"
	"github.com/donjor/zmux/internal/capturelog"
	"github.com/spf13/cobra"
)

// newLogSinkCmd is the hidden worker behind `zmux log`. tmux pipe-pane runs it
// with the pane's output on stdin; it streams that into a byte-bounded log file
// via capturelog.Sink. Not meant to be invoked by hand — `zmux log start` wires
// it up with the resolved self-binary so the zzmux edge profile pipes correctly.
func newLogSinkCmd(app *apppkg.App) *cobra.Command {
	var (
		file     string
		maxBytes int
		ansi     bool
	)
	cmd := &cobra.Command{
		Use:    "log-sink",
		Short:  "Internal: bounded stdin→file sink fed by tmux pipe-pane",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(file) == "" {
				return fmt.Errorf("--file is required")
			}
			// Defensive: the log dir is created by `log start`, but the sink is
			// a separate pipe-pane process, so ensure the parent exists.
			if err := app.FS.MkdirAll(filepath.Dir(file), 0o755); err != nil {
				return fmt.Errorf("create log dir: %w", err)
			}
			sink := capturelog.New(app.FS, file, maxBytes, !ansi)
			if _, err := io.Copy(sink, cmd.InOrStdin()); err != nil {
				sink.Close()
				return err
			}
			return sink.Close()
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "absolute path of the log file to write")
	cmd.Flags().IntVar(&maxBytes, "max-bytes", capturelog.DefaultMaxBytes, "byte cap; oldest output is dropped past this")
	cmd.Flags().BoolVar(&ansi, "ansi", false, "preserve ANSI/control sequences instead of stripping to plain text")
	return cmd
}
