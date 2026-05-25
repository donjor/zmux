package snapshot

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/donjor/zmux/internal/tmux"
)

// metaFormat mirrors the pi-parley pane metadata capture so downstream tooling
// sees a stable display-message line.
const metaFormat = "#{pane_id} #{pane_width}x#{pane_height} active=#{pane_active} title=#{pane_title} command=#{pane_current_command} path=#{pane_current_path}"

var (
	paneSizeRE  = regexp.MustCompile(`(?:^|\s)(\d+)x(\d+)(?:\s|$)`)
	stemUnsafe  = regexp.MustCompile(`[^a-z0-9._-]+`)
	stemTrimDsh = regexp.MustCompile(`^-+|-+$`)
)

const maxStemLen = 48

// paneFileStem turns a pane name into a safe, bounded filename stem.
func paneFileStem(name string) string {
	s := stemUnsafe.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "-")
	s = stemTrimDsh.ReplaceAllString(s, "")
	if len(s) > maxStemLen {
		s = stemTrimDsh.ReplaceAllString(s[:maxStemLen], "")
	}
	if s == "" {
		return "pane"
	}
	return s
}

func parsePaneSize(meta string) (width, height int) {
	m := paneSizeRE.FindStringSubmatch(meta)
	if m == nil {
		return 0, 0
	}
	w, _ := strconv.Atoi(m[1])
	h, _ := strconv.Atoi(m[2])
	return w, h
}

// uniqueStem returns stem the first time and stem-N on later collisions, so two
// panes whose names sanitize to the same stem never overwrite each other.
func uniqueStem(stem string, seen map[string]int) string {
	n := seen[stem]
	seen[stem]++
	if n == 0 {
		return stem
	}
	return fmt.Sprintf("%s-%d", stem, n+1)
}

// capturePane writes <stem>.ansi, <stem>.txt, and <stem>.meta.json for one
// pane. It returns nil (and warnings) if neither text nor ANSI capture
// succeeded, so an unreachable pane never produces an empty artifact.
func (s Snapshotter) capturePane(dir, stem string, target PaneTarget, lines int) (*PaneArtifact, []string) {
	var warnings []string
	ansiPath := join(dir, stem+".ansi")
	textPath := join(dir, stem+".txt")
	metaPath := join(dir, stem+".meta.json")

	ansiOK := s.writeCapture(ansiPath, target, tmux.CapturePaneOptions{Lines: lines, ANSI: true, Join: true}, &warnings)
	textOK := s.writeCapture(textPath, target, tmux.CapturePaneOptions{Lines: lines, Join: true}, &warnings)
	if !ansiOK && !textOK {
		return nil, warnings
	}

	meta, err := s.Runner.DisplayMessage(target.PaneID, metaFormat)
	meta = strings.TrimSpace(meta)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("pane metadata failed for %s (%s): %v", target.Name, target.PaneID, err))
		meta = ""
	}
	width, height := parsePaneSize(meta)

	metaDoc := map[string]any{
		"name":       target.Name,
		"paneId":     target.PaneID,
		"capturedAt": s.now().UTC().Format("2006-01-02T15:04:05.000Z"),
		"lines":      lines,
		"tmux":       meta,
	}
	if width > 0 {
		metaDoc["width"] = width
	}
	if height > 0 {
		metaDoc["height"] = height
	}
	if err := s.writeJSON(metaPath, metaDoc); err != nil {
		warnings = append(warnings, fmt.Sprintf("write pane meta failed for %s: %v", target.Name, err))
	}

	artifact := &PaneArtifact{Name: target.Name, PaneID: target.PaneID, MetaPath: metaPath, Width: width, Height: height}
	if ansiOK {
		artifact.AnsiPath = ansiPath
	}
	if textOK {
		artifact.TextPath = textPath
	}
	return artifact, warnings
}

// writeCapture captures pane content with the given options and writes it,
// recording a warning (and returning false) on failure.
func (s Snapshotter) writeCapture(path string, target PaneTarget, opts tmux.CapturePaneOptions, warnings *[]string) bool {
	content, err := s.Runner.CapturePaneOpts(target.PaneID, opts)
	if err != nil {
		mode := "text"
		if opts.ANSI {
			mode = "ansi"
		}
		*warnings = append(*warnings, fmt.Sprintf("%s capture failed for %s (%s): %v", mode, target.Name, target.PaneID, err))
		return false
	}
	if err := s.FS.WriteFile(path, []byte(content), 0o644); err != nil {
		*warnings = append(*warnings, fmt.Sprintf("write capture failed for %s: %v", path, err))
		return false
	}
	return true
}

func (s Snapshotter) writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return s.FS.WriteFile(path, append(data, '\n'), 0o644)
}
