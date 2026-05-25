package snapshot

import (
	"strconv"
	"strings"
)

// Manifest is the lightweight, stable index written to manifest.json. It is a
// subset of Result kept for display consumers (e.g. the pi-clean-ui sidecar
// reads manifest.json) that want a cheap summary without the full result.
type Manifest struct {
	SchemaVersion  string         `json:"schemaVersion"`
	Version        int            `json:"version"`
	Type           string         `json:"type"`
	OK             bool           `json:"ok"`
	Dir            string         `json:"dir"`
	CreatedAt      string         `json:"createdAt"`
	Modalities     []string       `json:"modalities"`
	Panes          []PaneArtifact `json:"panes"`
	ScreenshotPath string         `json:"screenshotPath,omitempty"`
	Warnings       []string       `json:"warnings"`
}

func manifestOf(r Result) Manifest {
	return Manifest{
		SchemaVersion:  r.SchemaVersion,
		Version:        1,
		Type:           r.Type,
		OK:             r.OK,
		Dir:            r.Dir,
		CreatedAt:      r.CreatedAt,
		Modalities:     r.Modalities,
		Panes:          r.Panes,
		ScreenshotPath: r.ScreenshotPath,
		Warnings:       r.Warnings,
	}
}

// writeBundle writes snapshot.json (full), manifest.json (summary), and a
// human-readable README.md into the snapshot directory.
func (s Snapshotter) writeBundle(r Result) error {
	if err := s.writeJSON(join(r.Dir, "snapshot.json"), r); err != nil {
		return err
	}
	if err := s.writeJSON(join(r.Dir, "manifest.json"), manifestOf(r)); err != nil {
		return err
	}
	return s.FS.WriteFile(join(r.Dir, "README.md"), []byte(readme(r)), 0o644)
}

func readme(r Result) string {
	var b strings.Builder
	b.WriteString("# zmux snapshot\n\n")
	b.WriteString("Created: " + r.CreatedAt + "\n")
	b.WriteString("OK: ")
	if r.OK {
		b.WriteString("true\n")
	} else {
		b.WriteString("false\n")
	}
	b.WriteString("Modalities: " + strings.Join(r.Modalities, ", ") + "\n\n")

	b.WriteString("## Artifacts\n\n")
	for _, p := range r.Panes {
		size := ""
		if p.Width > 0 && p.Height > 0 {
			size = ", " + strconv.Itoa(p.Width) + "x" + strconv.Itoa(p.Height)
		}
		b.WriteString("### " + p.Name + " (" + p.PaneID + size + ")\n")
		b.WriteString("- pane metadata: " + p.MetaPath + "\n")
		if p.AnsiPath != "" {
			b.WriteString("- ANSI pane: " + p.AnsiPath + "\n")
		}
		if p.TextPath != "" {
			b.WriteString("- text pane: " + p.TextPath + "\n")
		}
	}
	if r.ScreenshotPath != "" {
		b.WriteString("- PNG screenshot: " + r.ScreenshotPath + "\n")
	}

	if len(r.ViewCommands) > 0 {
		b.WriteString("\n## View commands\n\n")
		for _, c := range r.ViewCommands {
			b.WriteString("- `" + c + "`\n")
		}
	}
	if len(r.Warnings) > 0 {
		b.WriteString("\n## Warnings\n\n")
		for _, w := range r.Warnings {
			b.WriteString("- " + w + "\n")
		}
	}
	return b.String()
}
