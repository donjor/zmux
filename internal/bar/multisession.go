package bar

import (
	"fmt"
	"strings"

	"github.com/donjor/zmux/internal/theme"
)

// ── Session indicator (inside the session pill) ─────────────────────

// AttachState is the attach status of a workspace's sibling session from the
// bar's point of view. The enum keeps the door open for SSH/remote work
// without a refactor — AttachRemote is reserved for the SSH chapter and is
// not populated by the local-tmux probe today.
type AttachState int

const (
	// AttachUnknown means we have no attach signal for the session, or the
	// state was not populated by the caller. Renderers treat this the same
	// as "exists, no client attached" (an empty dot).
	AttachUnknown AttachState = iota
	// AttachLocal means at least one tmux client on this socket has the
	// session attached. For sibling sessions this is the "attached
	// elsewhere" signal — some other client is on it.
	AttachLocal
	// AttachRemote is reserved for SSH-mediated remote attaches and is
	// unused today. Defined so the dot/pill renderers can introduce a
	// distinct glyph without another data-model migration.
	AttachRemote
)

// CompactDots returns a plain unicode dot string for embedding inside
// the session pill. No ANSI colors — the preset's pill fg/bg applies.
//
// states is index-aligned with sessions and may be nil (treated as all
// Unknown). When a sibling session is attached elsewhere (AttachLocal),
// it renders as ◉ instead of ○ so the user can spot "another client is on
// that session" at a glance.
//
//	main ○●◉    (3 sessions, current is #2, session #3 is attached elsewhere)
func CompactDots(sessions []string, currentSession string, states []AttachState) string {
	if len(sessions) <= 1 {
		return ""
	}
	var b strings.Builder
	for i, s := range sessions {
		switch {
		case s == currentSession:
			b.WriteRune('●')
		case i < len(states) && (states[i] == AttachLocal || states[i] == AttachRemote):
			b.WriteRune('◉')
		default:
			b.WriteRune('○')
		}
	}
	return b.String()
}

// ── Top-row variant renderers (tmux format strings) ─────────────────
//
// These produce tmux format strings for the top status row. They're
// called by RenderTopRow which dispatches on the variant config.

// RenderTopRow renders the top status row based on variant + preset.
// Returns tmux format strings. Renders a single session too (always-2-line,
// plan 024); empty only when there are no sessions at all.
func RenderTopRow(p *theme.Palette, ctx BarContext, preset Preset, variant string) string {
	if len(ctx.WorkspaceSessions) == 0 {
		return ""
	}
	switch variant {
	case "tabs":
		return RenderTop(p, ctx, preset)
	case "dots":
		return renderTopDots(p, ctx, preset)
	case "minimal":
		return renderTopMinimalVariant(p, ctx, preset)
	default:
		return RenderTop(p, ctx, preset)
	}
}

// renderTopDots: workspace pill + enriched dots (● name  ○ name).
func renderTopDots(p *theme.Palette, ctx BarContext, preset Preset) string {
	var b strings.Builder

	// Workspace pill (preset-matched).
	b.WriteString(renderTopWorkspacePill(p, ctx.Workspace, preset))
	b.WriteString(" ")

	// Enriched dots with session names.
	for i, sess := range ctx.WorkspaceSessions {
		isCurrent := sess == ctx.Session
		if i > 0 {
			fmt.Fprintf(&b, "#[fg=%s]   ", p.Dim.Hex())
		}
		if isCurrent {
			fmt.Fprintf(&b, "#[fg=%s]● #[fg=%s,bold]%s#[nobold]",
				p.Accent.Hex(), p.Accent.Hex(), topSessionLabel(ctx, sess))
		} else {
			fmt.Fprintf(&b, "#[fg=%s]○ #[fg=%s]%s",
				p.Dim.Hex(), p.Dim.Hex(), sess)
		}
	}
	return b.String()
}

// renderTopMinimalVariant: workspace name + plain session names.
func renderTopMinimalVariant(p *theme.Palette, ctx BarContext, preset Preset) string {
	var b strings.Builder

	b.WriteString(renderTopWorkspacePill(p, ctx.Workspace, preset))
	b.WriteString(" ")

	for i, sess := range ctx.WorkspaceSessions {
		if i > 0 {
			b.WriteString("  ")
		}
		if sess == ctx.Session {
			fmt.Fprintf(&b, "#[fg=%s,bold]%s#[nobold]", p.FG.Hex(), topSessionLabel(ctx, sess))
		} else {
			fmt.Fprintf(&b, "#[fg=%s]%s", p.Dim.Hex(), sess)
		}
	}
	return b.String()
}

// renderTopWorkspacePill renders the workspace pill for the top row
// matching each preset's visual language. Uses tmux format strings.
func renderTopWorkspacePill(p *theme.Palette, workspace string, preset Preset) string {
	if workspace == "" {
		return ""
	}
	label := "󱂬 " + workspace
	switch preset {
	case Powerline:
		return fmt.Sprintf(
			"#[fg=%s,bg=%s]\ue0b0#[bg=%s,fg=%s,bold] %s #[nobold]",
			p.BG.Hex(), p.Special.Hex(), p.Special.Hex(), p.BG.Hex(), label,
		)
	case Rpowerline:
		return fmt.Sprintf(
			"#[fg=%s]\ue0b6#[bg=%s,fg=%s,bold] %s #[nobold,fg=%s,bg=default]\ue0b4",
			p.Special.Hex(), p.Special.Hex(), p.BG.Hex(), label, p.Special.Hex(),
		)
	case Rounded:
		return fmt.Sprintf(
			"#[fg=%s]\ue0b6#[bg=%s,fg=%s,bold] %s #[fg=%s,bg=default]\ue0b4",
			p.Special.Hex(), p.Special.Hex(), p.BG.Hex(), label, p.Special.Hex(),
		)
	case Blocks:
		return fmt.Sprintf("#[fg=%s,bold] [%s] #[nobold]", p.Special.Hex(), label)
	case Hacker:
		return fmt.Sprintf("#[fg=%s]%s", p.Success.Hex(), label)
	case Zen:
		return fmt.Sprintf("#[fg=%s]%s", p.Dim.Hex(), label)
	case Minimal:
		return fmt.Sprintf("#[fg=%s,bold] %s #[nobold]", p.FG.Hex(), label)
	case Starship:
		return fmt.Sprintf("#[fg=%s,bold] %s #[nobold]", p.Special.Hex(), label)
	default:
		return fmt.Sprintf(
			"#[fg=%s]\ue0b6#[bg=%s,fg=%s,bold] %s #[fg=%s,bg=default]\ue0b4",
			p.Special.Hex(), p.Special.Hex(), p.BG.Hex(), label, p.Special.Hex(),
		)
	}
}
