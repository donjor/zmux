package snapshot

import (
	"context"
	"fmt"
	"os/exec"
)

// createScreenshot resolves the current terminal's geometry and shoots a PNG.
// It is strict: a hidden/ambiguous/unsupported target produces a warning plus a
// terminal.png.meta.json refusal record rather than a misleading screenshot.
func (s Snapshotter) createScreenshot(ctx context.Context, dir string) (string, []string) {
	var warnings []string
	if s.Resolver == nil {
		return "", append(warnings, "screenshot requested but no terminal resolver configured")
	}
	res, err := s.Resolver.Resolve(ctx)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("terminal target resolution failed: %v", err))
		return "", warnings
	}
	if !res.OK || res.Target == nil || res.Target.Geometry == "" {
		reason := res.Reason
		if reason == "" {
			reason = string(res.Status)
		}
		warnings = append(warnings, fmt.Sprintf("screenshot target refused: %s", reason))
		s.writeScreenshotMeta(dir, map[string]any{"mode": "terminal-current-refused", "zmux": res}, &warnings)
		return "", warnings
	}

	if s.Shooter == nil {
		return "", append(warnings, "screenshot requested but no screenshot tool configured")
	}
	path := join(dir, "terminal.png")
	if err := s.Shooter.Shoot(res.Target.Geometry, path); err != nil {
		warnings = append(warnings, fmt.Sprintf("screenshot tool failed: %v", err))
		s.writeScreenshotMeta(dir, map[string]any{"mode": "terminal-current-shoot-failed", "geometry": res.Target.Geometry, "zmux": res}, &warnings)
		return "", warnings
	}
	if _, err := s.FS.Stat(path); err != nil {
		warnings = append(warnings, "screenshot tool exited cleanly but wrote no PNG file")
		s.writeScreenshotMeta(dir, map[string]any{"mode": "terminal-current-missing-file", "geometry": res.Target.Geometry, "zmux": res}, &warnings)
		return "", warnings
	}
	s.writeScreenshotMeta(dir, map[string]any{"mode": "terminal-current", "geometry": res.Target.Geometry, "zmux": res}, &warnings)
	return path, warnings
}

func (s Snapshotter) writeScreenshotMeta(dir string, meta any, warnings *[]string) {
	if err := s.writeJSON(join(dir, "terminal.png.meta.json"), meta); err != nil {
		*warnings = append(*warnings, fmt.Sprintf("write screenshot meta failed: %v", err))
	}
}

// GrimShooter captures via `grim -g <geometry> <out>` (Wayland/Hyprland). It is
// the default Shooter; the Shooter seam keeps macOS/X11 tools pluggable later.
type GrimShooter struct{}

func (GrimShooter) Shoot(geometry, outPath string) error {
	cmd := exec.Command("grim", "-g", geometry, outPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		msg := string(out)
		if msg == "" {
			return err
		}
		return fmt.Errorf("%v: %s", err, msg)
	}
	return nil
}
