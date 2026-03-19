#!/usr/bin/env bash
# zmux theme sync — pull-only theme sync from other tools

# Requires ZMUX_ vars from lib/theme.sh

# ── Pull from Ghostty ──
# Reads `theme = X` from ghostty config, returns theme name
pull_ghostty() {
    local config=""

    if [[ "$ZMUX_GHOSTTY_CONFIG" == "auto" ]]; then
        # Auto-detect ghostty config
        if [[ -f "${HOME}/.config/ghostty/config" ]]; then
            config="${HOME}/.config/ghostty/config"
        elif [[ -f "${XDG_CONFIG_HOME:-$HOME/.config}/ghostty/config" ]]; then
            config="${XDG_CONFIG_HOME:-$HOME/.config}/ghostty/config"
        fi
    else
        config="$ZMUX_GHOSTTY_CONFIG"
    fi

    [[ -z "$config" || ! -f "$config" ]] && return 1

    local theme_line
    theme_line=$(grep '^theme' "$config" 2>/dev/null | tail -1)
    [[ -z "$theme_line" ]] && return 1

    local name
    name=$(echo "$theme_line" | cut -d'=' -f2- | sed 's/^[[:space:]]*//' | sed 's/[[:space:]]*$//' | sed 's/^"//' | sed 's/"$//')
    [[ -z "$name" ]] && return 1

    echo "$name"
}

# ── Pull from neovim ──
# Queries nvim colorscheme, best-effort match to theme name
pull_nvim() {
    command -v nvim &>/dev/null || return 1

    local colorscheme
    colorscheme=$(nvim --headless +'lua io.write(vim.g.colors_name or "")' +qa 2>/dev/null)
    [[ -z "$colorscheme" ]] && return 1

    # Normalize common nvim colorscheme names to iterm2/ghostty theme names
    local name="$colorscheme"
    case "$colorscheme" in
        tokyonight*)     name="tokyonight" ;;
        catppuccin*)     name="catppuccin-mocha" ;;
        gruvbox*)        name="gruvbox-dark" ;;
        kanagawa*)       name="kanagawa-dragon" ;;
        rose-pine*)      name="rose-pine" ;;
        material*)       name="material-darker" ;;
    esac

    echo "$name"
}

# ── Sync from default target ──
action_theme_sync() {
    local target="${ZMUX_SYNC_TARGET:-none}"

    if [[ "$target" == "none" ]]; then
        echo "no sync target configured"
        echo "set sync.target in ~/.zmux.conf (ghostty or nvim)"
        return 1
    fi

    action_theme_pull "$target"
}

# ── Pull from explicit target ──
action_theme_pull() {
    local target="$1"
    [[ -z "$target" ]] && { echo "usage: zmux theme pull <ghostty|nvim>"; return 1; }

    local name=""
    case "$target" in
        ghostty) name=$(pull_ghostty) ;;
        nvim)    name=$(pull_nvim) ;;
        *)       echo "unknown sync target: $target"; return 1 ;;
    esac

    if [[ -z "$name" ]]; then
        echo "could not read theme from $target"
        return 1
    fi

    # Check if theme exists in zmux
    if zmux_resolve_theme "$name"; then
        _zmux_set_theme "$name"
        echo "synced theme from $target → $name"
    else
        echo "theme '$name' from $target not found in zmux theme library"
        return 1
    fi
}

# ── Helper: update config and apply ──
_zmux_set_theme() {
    local name="$1"
    local conf="${HOME}/.zmux.conf"

    if [[ -f "$conf" ]]; then
        if grep -q '^theme' "$conf" 2>/dev/null; then
            sed -i "s|^theme = .*|theme = $name|" "$conf"
        else
            echo "theme = $name" >> "$conf"
        fi
    else
        echo "theme = $name" > "${HOME}/.zmux.conf"
    fi

    # Apply if inside tmux
    if [[ -n "$TMUX" ]]; then
        "$ZMUX_ROOT/bin/zmux0-apply-theme"
    fi
}
