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

// RenderPreviewWithSegmentsOverride renders a preview with segment
// visibility AND a callback to modify the BarContext before rendering.
// Used by the two-line layout to suppress position suffixes etc.
func RenderPreviewWithSegmentsOverride(preset Preset, palette *theme.Palette, segments config.BarSegments, override func(*BarContext)) string {
	return renderPreviewCtxOverride(preset, palette, segments, override)
}

// RenderTopPreview renders the top row (workspace + session tabs) as
// ANSI for preview outside tmux. Uses the real RenderTop with bg-aware
// tmux→ANSI conversion so the bar bg persists across all segments.
func RenderTopPreview(preset Preset, palette *theme.Palette, sessions []string, currentSession string, width int) string {
	return RenderTopPreviewVariant(preset, palette, sessions, currentSession, width, "tabs")
}

// RenderTopPreviewVariant renders the top row for any variant (tabs,
// dots, minimal) as ANSI. Renders a single session too (always-2-line,
// plan 024); empty only when there are no sessions.
func RenderTopPreviewVariant(preset Preset, palette *theme.Palette, sessions []string, currentSession string, width int, variant string) string {
	ctx := makePreviewCtx(config.BarSegments{Workspace: true})
	ctx.WorkspaceSessions = sessions
	ctx.Session = currentSession
	ctx.WorkspaceCount = len(sessions)
	for i, s := range sessions {
		if s == currentSession {
			ctx.WorkspacePos = i + 1
			break
		}
	}

	bg := BarBGColor(palette, preset)
	top := RenderTopRow(palette, ctx, preset, variant)
	if top == "" {
		return ""
	}
	content := tmuxToANSIWithBarBG(top, bg)
	return padWithBarBG(content, bg, width)
}

// RenderBarPreview renders the main bar row as ANSI with proper bar bg.
func RenderBarPreview(preset Preset, palette *theme.Palette, segments config.BarSegments, width int) string {
	return RenderBarPreviewOverride(preset, palette, segments, width, nil)
}

// RenderBarPreviewOverride is RenderBarPreview with a BarContext override.
func RenderBarPreviewOverride(preset Preset, palette *theme.Palette, segments config.BarSegments, width int, override func(*BarContext)) string {
	ctx := makePreviewCtx(segments)
	if override != nil {
		override(&ctx)
	}

	bg := BarBGColor(palette, preset)
	left := RenderLeft(palette, ctx, preset)
	right := RenderRight(palette, ctx, preset)

	// Build the raw tmux content (left + windows + right), then
	// resolve bg=default and convert to ANSI in one pass so the
	// bar bg persists across ALL segments including the window tabs.
	rawWindows := previewWindowsRaw(palette, preset)
	combined := left + "  " + rawWindows + "  " + right
	content := tmuxToANSIWithBarBG(combined, bg)
	return padWithBarBG(content, bg, width)
}

// BarBGColor returns the bar background color for a preset. rpowerline
// and powerline use BG (darker); others use Surface.
func BarBGColor(palette *theme.Palette, preset Preset) theme.Color {
	if preset == Rpowerline || preset == Powerline {
		return palette.BG
	}
	return palette.Surface
}

// resolveBarBG replaces "bg=default" in tmux format strings with the
// actual bar bg hex. In real tmux, "bg=default" means "inherit from
// status-style"; in ANSI, [49m means "terminal bg" which breaks the
// bar surface. This substitution makes the ANSI preview match tmux.
func resolveBarBG(tmuxFmt string, bg theme.Color) string {
	return strings.ReplaceAll(tmuxFmt, "bg=default", "bg="+bg.Hex())
}

// tmuxToANSIWithBarBG converts tmux format strings to ANSI with
// bg=default resolved to the actual bar bg. Also replaces [0m full
// resets with fg+bg restore so the bar bg persists across segments.
func tmuxToANSIWithBarBG(s string, bg theme.Color) string {
	s = resolveBarBG(s, bg)
	result := tmuxToANSI(s)
	// Replace full resets with a bg-preserving reset: clear fg/bold
	// but keep the bar bg.
	bgRestore := fmt.Sprintf("\033[0m\033[48;2;%d;%d;%dm", bg.R, bg.G, bg.B)
	result = strings.ReplaceAll(result, "\033[0m", bgRestore)
	return result
}

// padWithBarBG pads ANSI content to width with the bar bg.
func padWithBarBG(content string, bg theme.Color, width int) string {
	bgAnsi := fmt.Sprintf("\033[48;2;%d;%d;%dm", bg.R, bg.G, bg.B)
	reset := "\033[0m"
	visible := stripANSI(content)
	pad := width - len(visible)
	if pad < 0 {
		pad = 0
	}
	return bgAnsi + content + strings.Repeat(" ", pad) + reset
}

func RenderPreview(preset Preset, palette *theme.Palette) string {
	return renderPreviewCtx(preset, palette, config.BarSegments{
		Workspace: true, Git: true, Lang: true, Clock: true,
		Directory: true, Process: true, Group: true,
	})
}

func makePreviewCtx(segments config.BarSegments) BarContext {
	return BarContext{
		Session:        "main",
		Workspace:      "myapp",
		WorkspacePos:   1,
		WorkspaceCount: 3,
		GitBranch:      "main",
		GitDirty:       false,
		LangIcon:       " ",
		LangVersion:    "1.24",
		Time:           "14:30",
		Date:           "Apr 07",
		PaneDir:        "~/src/myapp",
		PaneCmd:        "nvim",
		ShowWorkspace:  segments.Workspace,
		ShowGit:        segments.Git,
		ShowLang:       segments.Lang,
		ShowClock:      segments.Clock,
		ShowDirectory:  segments.Directory,
		ShowProcess:    segments.Process,
		ShowGroup:      segments.Group,
	}
}

func renderPreviewCtx(preset Preset, palette *theme.Palette, segments config.BarSegments) string {
	return renderPreviewCtxOverride(preset, palette, segments, nil)
}

func renderPreviewCtxOverride(preset Preset, palette *theme.Palette, segments config.BarSegments, override func(*BarContext)) string {
	ctx := makePreviewCtx(segments)
	if override != nil {
		override(&ctx)
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

// previewWindows generates fake window tabs for the preview (ANSI).
func previewWindows(p *theme.Palette, preset Preset) string {
	return tmuxToANSI(previewWindowsTmux(p, preset))
}

// previewWindowsRaw returns fake window tabs as TMUX FORMAT STRINGS
// (not converted to ANSI). Callers can combine with other tmux format
// content and do a single bg-aware ANSI conversion.
func previewWindowsRaw(p *theme.Palette, preset Preset) string {
	return previewWindowsTmux(p, preset)
}

// previewWindowsTmux builds fake window tabs as tmux format strings.
// Uses a sentinel path for dynamicOptions — it only appears in the
// #() commands (status-left/right) which we don't use from the preview.
func previewWindowsTmux(p *theme.Palette, preset Preset) string {
	opts := dynamicOptions(p, "zmux", preset, BarLayoutConfig{})

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

	return inactive + sep + active + sep + inactive2
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
	_, _ = fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
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
