#!/usr/bin/env bash
# zmux status bar presets — 4 built-in layouts

# Requires ZMUX_ color vars to be loaded (via lib/theme.sh)

zmux_apply_status_preset() {
    local preset="${1:-default}"

    # ── Shared across all presets ──
    tmux set -g pane-border-style "fg=${ZMUX_DIM}"
    tmux set -g pane-active-border-style "fg=${ZMUX_ACCENT}"
    tmux set -g message-style "bg=${ZMUX_SURFACE},fg=${ZMUX_FG}"
    tmux set -g message-command-style "bg=${ZMUX_SURFACE},fg=${ZMUX_FG}"
    tmux set -g mode-style "bg=${ZMUX_INFO},fg=${ZMUX_BG}"
    tmux set -g clock-mode-colour "${ZMUX_ACCENT}"
    tmux set -g window-active-style "#{?client_prefix,bg=${ZMUX_BG_PREFIX},bg=${ZMUX_BG}}"
    tmux set -g window-style "bg=${ZMUX_BG_DIM}"

    case "$preset" in
        minimal)  _zmux_status_minimal ;;
        powerline) _zmux_status_powerline ;;
        blocks)   _zmux_status_blocks ;;
        *)        _zmux_status_default ;;
    esac
}

# ── default ──
# Session pill (ACCENT bg, INFO on prefix), prefix hints, clock
_zmux_status_default() {
    tmux set -g status-style "bg=${ZMUX_SURFACE},fg=${ZMUX_MUTED}"

    tmux set -g status-left \
        "#{?client_prefix,#[bg=${ZMUX_INFO}],#[bg=${ZMUX_ACCENT}]}#[fg=${ZMUX_BG},bold] #S #{?client_prefix,#[fg=${ZMUX_INFO}],#[fg=${ZMUX_ACCENT}]}#[bg=${ZMUX_SURFACE}] "

    tmux set -g @prefix_hint \
        "#[fg=${ZMUX_INFO}],#[fg=${ZMUX_DIM}]rename #[fg=${ZMUX_INFO}]s#[fg=${ZMUX_DIM}]witch #[fg=${ZMUX_INFO}]c#[fg=${ZMUX_DIM}] tab #[fg=${ZMUX_INFO}]v#[fg=${ZMUX_DIM}]isual #[fg=${ZMUX_INFO}]x#[fg=${ZMUX_DIM}]kill #[fg=${ZMUX_INFO}]r#[fg=${ZMUX_DIM}]eload #[fg=${ZMUX_INFO}]?#[fg=${ZMUX_DIM}]help "

    tmux set -g status-right \
        "#{?client_prefix,#{@prefix_hint},#[fg=${ZMUX_DIM}]ctrl+space #[fg=${ZMUX_MUTED}]%I:%M %p }"

    tmux set -g window-status-format "#[fg=${ZMUX_DIM}] #I #W "
    tmux set -g window-status-current-format \
        "#{?client_prefix,#[fg=${ZMUX_INFO}],#[fg=${ZMUX_ACCENT}]}#[bold] #I #W #[fg=${ZMUX_MUTED},nobold]"
    tmux set -g window-status-separator "#[fg=${ZMUX_DIM}]│"
}

# ── minimal ──
# Session name + pipe, minimal tabs, optional time
_zmux_status_minimal() {
    tmux set -g status-style "bg=${ZMUX_SURFACE},fg=${ZMUX_MUTED}"

    tmux set -g status-left \
        "#{?client_prefix,#[fg=${ZMUX_INFO}],#[fg=${ZMUX_ACCENT}]}#[bold] #S #[fg=${ZMUX_DIM},nobold]│ "

    tmux set -g status-right \
        "#[fg=${ZMUX_DIM}]%H:%M "

    tmux set -g window-status-format "#[fg=${ZMUX_DIM}] #W "
    tmux set -g window-status-current-format \
        "#[fg=${ZMUX_FG},bold] #W "
    tmux set -g window-status-separator " "
}

# ── powerline ──
# Angled separators, filled segments
_zmux_status_powerline() {
    tmux set -g status-style "bg=${ZMUX_SURFACE},fg=${ZMUX_MUTED}"

    tmux set -g status-left \
        "#{?client_prefix,#[bg=${ZMUX_INFO}],#[bg=${ZMUX_ACCENT}]}#[fg=${ZMUX_BG},bold] #S #{?client_prefix,#[fg=${ZMUX_INFO}],#[fg=${ZMUX_ACCENT}]}#[bg=${ZMUX_SURFACE}] "

    tmux set -g status-right \
        "#[fg=${ZMUX_DIM}]#[bg=${ZMUX_DIM},fg=${ZMUX_MUTED}] %H:%M #[fg=${ZMUX_ACCENT}]#[bg=${ZMUX_ACCENT},fg=${ZMUX_BG},bold] %b %d "

    tmux set -g window-status-format "#[fg=${ZMUX_SURFACE},bg=${ZMUX_DIM}]#[fg=${ZMUX_MUTED}] #I #W #[fg=${ZMUX_DIM},bg=${ZMUX_SURFACE}]"
    tmux set -g window-status-current-format \
        "#{?client_prefix,#[fg=${ZMUX_SURFACE}]#[bg=${ZMUX_INFO}]#[fg=${ZMUX_BG}]#[bold] #I #W #[fg=${ZMUX_INFO}]#[bg=${ZMUX_SURFACE}],#[fg=${ZMUX_SURFACE}]#[bg=${ZMUX_ACCENT}]#[fg=${ZMUX_BG}]#[bold] #I #W #[fg=${ZMUX_ACCENT}]#[bg=${ZMUX_SURFACE}]}"
    tmux set -g window-status-separator ""
}

# ── blocks ──
# Square bracket segments, monospace aesthetic
_zmux_status_blocks() {
    tmux set -g status-style "bg=${ZMUX_SURFACE},fg=${ZMUX_MUTED}"

    tmux set -g status-left \
        "#{?client_prefix,#[fg=${ZMUX_INFO}],#[fg=${ZMUX_ACCENT}]}#[bold] [#S] #[fg=${ZMUX_DIM},nobold]"

    tmux set -g status-right \
        "#[fg=${ZMUX_DIM}][%H:%M] "

    tmux set -g window-status-format "#[fg=${ZMUX_DIM}] [#I:#W] "
    tmux set -g window-status-current-format \
        "#{?client_prefix,#[fg=${ZMUX_INFO}],#[fg=${ZMUX_ACCENT}]}#[bold] [#I:#W] #[nobold]"
    tmux set -g window-status-separator ""
}
