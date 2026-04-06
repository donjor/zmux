package bar

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/theme"
)

// RenderPreview returns an ANSI-colored string that shows what the bar looks
// like for a given preset. Uses the SAME render functions as the live bar,
// converted from tmux format strings to ANSI escape codes.
// RenderPreviewWithSegments renders a preview with specific segment visibility.
func RenderPreviewWithSegments(preset Preset, palette *theme.Palette, segments config.BarSegments) string {
	return renderPreviewCtx(preset, palette, segments)
}

func RenderPreview(preset Preset, palette *theme.Palette) string {
	return renderPreviewCtx(preset, palette, config.BarSegments{
		Workspace: true, Git: true, Lang: true, Clock: true,
		Directory: true, Process: true, Group: true,
	})
}

func renderPreviewCtx(preset Preset, palette *theme.Palette, segments config.BarSegments) string {
	ctx := BarContext{
		Session:       "main",
		Workspace:     "project",
		GitBranch:     "main",
		GitDirty:      false,
		LangIcon:      " ",
		LangVersion:   "1.24",
		Time:          "14:30",
		PaneDir:       "~/project",
		PaneCmd:       "nvim",
		ShowWorkspace: segments.Workspace,
		ShowGit:       segments.Git,
		ShowLang:      segments.Lang,
		ShowClock:     segments.Clock,
		ShowDirectory: segments.Directory,
		ShowProcess:   segments.Process,
		ShowGroup:     segments.Group,
	}

	left := RenderLeft(palette, ctx, preset)
	right := RenderRight(palette, ctx, preset)

	// Convert tmux #[...] format strings to ANSI escape codes.
	left = tmuxToANSI(left)
	right = tmuxToANSI(right)

	// Build a fake window section in the middle.
	windows := previewWindows(palette, preset)

	// Assemble with padding.
	reset := "\033[0m"
	surfaceBg := fmt.Sprintf("\033[48;2;%d;%d;%dm", palette.Surface.R, palette.Surface.G, palette.Surface.B)

	content := left + "  " + windows + "  " + right + reset
	visible := stripANSI(content)
	pad := 72 - len(visible)
	if pad < 0 {
		pad = 0
	}

	return content + surfaceBg + strings.Repeat(" ", pad) + reset
}

// previewWindows generates fake window tabs for the preview.
func previewWindows(p *theme.Palette, preset Preset) string {
	// Get the window format strings from dynamicOptions.
	opts := dynamicOptions(p, "/usr/bin/zmux", preset)

	var windowFmt, windowCurrentFmt, windowSep string
	for _, opt := range opts {
		switch opt.Key {
		case "window-status-format":
			windowFmt = opt.Value
		case "window-status-current-format":
			windowCurrentFmt = opt.Value
		case "window-status-separator":
			windowSep = opt.Value
		}
	}

	// Replace tmux variables with sample data.
	inactive := strings.ReplaceAll(windowFmt, "#I", "1")
	inactive = strings.ReplaceAll(inactive, "#W", "zsh")
	active := strings.ReplaceAll(windowCurrentFmt, "#I", "2")
	active = strings.ReplaceAll(active, "#W", "nvim")
	inactive2 := strings.ReplaceAll(windowFmt, "#I", "3")
	inactive2 = strings.ReplaceAll(inactive2, "#W", "git")

	// Strip tmux conditionals (always use non-prefix state for preview).
	inactive = stripTmuxConditionals(inactive, false)
	active = stripTmuxConditionals(active, false)
	inactive2 = stripTmuxConditionals(inactive2, false)
	sep := stripTmuxConditionals(windowSep, false)

	// Convert to ANSI.
	return tmuxToANSI(inactive + sep + active + sep + inactive2)
}

// tmuxToANSI converts tmux #[fg=...,bg=...] format strings to ANSI escape codes.
var tmuxFormatRe = regexp.MustCompile(`#\[([^\]]*)\]`)

func tmuxToANSI(s string) string {
	return tmuxFormatRe.ReplaceAllStringFunc(s, func(match string) string {
		inner := match[2 : len(match)-1] // strip #[ and ]
		parts := strings.Split(inner, ",")

		var codes []string
		for _, part := range parts {
			part = strings.TrimSpace(part)
			switch {
			case part == "bold":
				codes = append(codes, "1")
			case part == "nobold":
				codes = append(codes, "22")
			case part == "blink":
				codes = append(codes, "5")
			case part == "noblink":
				codes = append(codes, "25")
			case strings.HasPrefix(part, "fg="):
				color := strings.TrimPrefix(part, "fg=")
				if color == "default" {
					codes = append(codes, "39")
				} else {
					codes = append(codes, hexToANSI("38", color))
				}
			case strings.HasPrefix(part, "bg="):
				color := strings.TrimPrefix(part, "bg=")
				if color == "default" {
					codes = append(codes, "49")
				} else {
					codes = append(codes, hexToANSI("48", color))
				}
			}
		}

		if len(codes) == 0 {
			return "\033[0m"
		}
		return "\033[" + strings.Join(codes, ";") + "m"
	})
}

// hexToANSI converts a hex color like "#f9af4f" to an ANSI 24-bit color code prefix.
func hexToANSI(prefix, hex string) string {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return prefix + ";2;128;128;128"
	}
	var r, g, b int
	fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
	return fmt.Sprintf("%s;2;%d;%d;%d", prefix, r, g, b)
}

// stripTmuxConditionals resolves #{?client_prefix,A,B} to A or B.
func stripTmuxConditionals(s string, prefixActive bool) string {
	re := regexp.MustCompile(`#\{[?]client_prefix,([^,}]*),([^}]*)\}`)
	return re.ReplaceAllStringFunc(s, func(match string) string {
		m := re.FindStringSubmatch(match)
		if len(m) < 3 {
			return match
		}
		if prefixActive {
			return m[1]
		}
		return m[2]
	})
}

// stripANSI removes ANSI escape sequences to measure visible string length.
func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
