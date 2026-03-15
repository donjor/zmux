#!/usr/bin/env bash
# zmux shared theme library — config reader, theme resolver, iterm2 parser

# ── Config reader ──
# Parses ~/.zmux.conf (flat key=value), sets global vars
zmux_load_config() {
    # Defaults
    ZMUX_THEME_NAME="${ZMUX_THEME_NAME:-ayu-dark}"
    ZMUX_STATUS_PRESET="${ZMUX_STATUS_PRESET:-default}"
    ZMUX_SYNC_TARGET="${ZMUX_SYNC_TARGET:-none}"
    ZMUX_PREFIX="${ZMUX_PREFIX:-C-Space}"
    ZMUX_GHOSTTY_CONFIG="${ZMUX_GHOSTTY_CONFIG:-auto}"
    ZMUX_CLIPBOARD_TOOL="${ZMUX_CLIPBOARD_TOOL:-auto}"
    ZMUX_ROOT="${ZMUX_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
    GUM="${GUM:-$(command -v gum 2>/dev/null || echo "$HOME/go/bin/gum")}"

    local conf="${HOME}/.zmux.conf"
    if [[ ! -f "$conf" ]]; then
        conf=""
    fi

    [[ -z "$conf" ]] && return 0

    while IFS='=' read -r key val; do
        key="${key#"${key%%[![:space:]]*}"}" ; key="${key%"${key##*[![:space:]]}"}"
        val="${val#"${val%%[![:space:]]*}"}" ; val="${val%"${val##*[![:space:]]}"}"
        val="${val#\"}" ; val="${val%\"}"
        [[ "$key" == \#* || -z "$key" ]] && continue
        case "$key" in
            theme)                  ZMUX_THEME_NAME="$val" ;;
            status)                 ZMUX_STATUS_PRESET="$val" ;;
            prefix)                 ZMUX_PREFIX="$val" ;;
            zmux_root)              ZMUX_ROOT="$val" ;;
            templates_dir)          ZMUX_TEMPLATES="$val" ;;
            gum_path)               [[ "$val" != "auto" ]] && GUM="$val" ;;
            sync.target)            ZMUX_SYNC_TARGET="$val" ;;
            sync.ghostty_config)    ZMUX_GHOSTTY_CONFIG="$val" ;;
            clipboard)              ZMUX_CLIPBOARD_TOOL="$val" ;;
            # Legacy keys from .zmux.conf
            theme_source)           : ;; # ignored, no longer used
        esac
    done < "$conf"

    TEMPLATES_DIR="${ZMUX_TEMPLATES:-$ZMUX_ROOT/templates}"

    # Auto-detect clipboard tool
    if [[ "$ZMUX_CLIPBOARD_TOOL" == "auto" ]]; then
        if command -v wl-copy &>/dev/null; then
            ZMUX_CLIPBOARD_TOOL="wl-copy"
        elif command -v xclip &>/dev/null; then
            ZMUX_CLIPBOARD_TOOL="xclip -selection clipboard"
        elif command -v pbcopy &>/dev/null; then
            ZMUX_CLIPBOARD_TOOL="pbcopy"
        else
            ZMUX_CLIPBOARD_TOOL=""
        fi
    fi
}

# ── Theme resolver ──
# Finds theme file: user → bundled → iterm2
# Sets ZMUX_THEME_FILE or returns 1
zmux_resolve_theme() {
    local name="${1:-$ZMUX_THEME_NAME}"
    ZMUX_THEME_FILE=""

    # 1. User custom themes
    if [[ -f "${HOME}/.zmux/themes/${name}" ]]; then
        ZMUX_THEME_FILE="${HOME}/.zmux/themes/${name}"
        return 0
    fi

    # 2. Bundled themes
    if [[ -f "$ZMUX_ROOT/themes/bundled/${name}" ]]; then
        ZMUX_THEME_FILE="$ZMUX_ROOT/themes/bundled/${name}"
        return 0
    fi

    # 3. Downloaded iterm2 set
    if [[ -f "$ZMUX_ROOT/themes/iterm2/${name}" ]]; then
        ZMUX_THEME_FILE="$ZMUX_ROOT/themes/iterm2/${name}"
        return 0
    fi

    return 1
}

# ── iterm2/ghostty format parser ──
# Parses theme file, exports semantic ZMUX_ color vars
zmux_load_theme() {
    if ! zmux_resolve_theme; then
        # Fallback to ayu-dark defaults
        ZMUX_BG="#0b0e14"
        ZMUX_FG="#bfbdb6"
        ZMUX_SURFACE="#11151c"
        ZMUX_ERROR="#ea6c73"
        ZMUX_SUCCESS="#7fd962"
        ZMUX_ACCENT="#f9af4f"
        ZMUX_INFO="#53bdfa"
        ZMUX_SPECIAL="#cda1fa"
        ZMUX_META="#90e1c6"
        ZMUX_MUTED="#c7c7c7"
        ZMUX_DIM="#686868"
        ZMUX_HIGHLIGHT="#e6b450"
        _zmux_derive_colors
        return 0
    fi

    local file="$ZMUX_THEME_FILE"
    declare -A palette

    while IFS='=' read -r key val; do
        key="${key#"${key%%[![:space:]]*}"}" ; key="${key%"${key##*[![:space:]]}"}"
        val="${val#"${val%%[![:space:]]*}"}" ; val="${val%"${val##*[![:space:]]}"}"
        [[ -z "$key" || "$key" == \#* ]] && continue
        case "$key" in
            background)           ZMUX_BG="$val" ;;
            foreground)           ZMUX_FG="$val" ;;
            cursor-color)         ZMUX_HIGHLIGHT="$val" ;;
            selection-background) ZMUX_SEL_BG="$val" ;;
            palette)
                local idx=$(echo "$val" | cut -d'#' -f1 | tr -d '= ')
                local color="#$(echo "$val" | cut -d'#' -f2)"
                palette[$idx]="$color"
                ;;
        esac
    done < "$file"

    # Map ANSI palette to semantic roles
    ZMUX_SURFACE="${palette[0]:-#11151c}"
    ZMUX_ERROR="${palette[1]:-#ea6c73}"
    ZMUX_SUCCESS="${palette[2]:-#7fd962}"
    ZMUX_ACCENT="${palette[3]:-#f9af4f}"
    ZMUX_INFO="${palette[4]:-#53bdfa}"
    ZMUX_SPECIAL="${palette[5]:-#cda1fa}"
    ZMUX_META="${palette[6]:-#90e1c6}"
    ZMUX_MUTED="${palette[7]:-#c7c7c7}"
    ZMUX_DIM="${palette[8]:-#686868}"
    ZMUX_HIGHLIGHT="${ZMUX_HIGHLIGHT:-${palette[3]:-#e6b450}}"

    # If SURFACE is same as BG, lighten slightly
    if [[ "$ZMUX_SURFACE" == "$ZMUX_BG" ]]; then
        local r=$(($(printf '%d' "0x${ZMUX_BG:1:2}") + 8))
        local g=$(($(printf '%d' "0x${ZMUX_BG:3:2}") + 8))
        local b=$(($(printf '%d' "0x${ZMUX_BG:5:2}") + 8))
        ZMUX_SURFACE=$(printf '#%02x%02x%02x' $r $g $b)
    fi

    _zmux_derive_colors
}

# ── Derived colors ──
_zmux_derive_colors() {
    local r=$(($(printf '%d' "0x${ZMUX_BG:1:2}")))
    local g=$(($(printf '%d' "0x${ZMUX_BG:3:2}")))
    local b=$(($(printf '%d' "0x${ZMUX_BG:5:2}")))
    ZMUX_BG_DIM=$(printf '#%02x%02x%02x' $((r > 2 ? r - 2 : 0)) $((g > 2 ? g - 2 : 0)) $((b > 2 ? b - 2 : 0)))
    ZMUX_BG_PREFIX=$(printf '#%02x%02x%02x' $((r > 1 ? r - 1 : 0)) $((g > 1 ? g - 1 : 0)) $((b > 2 ? b - 2 : 0)))
}

# ── ANSI escape helper ──
# Usage: zmux_ansi "#rrggbb" → prints \033[38;2;R;G;Bm
zmux_ansi() {
    local hex="$1"
    local r=$((16#${hex:1:2}))
    local g=$((16#${hex:3:2}))
    local b=$((16#${hex:5:2}))
    printf '\033[38;2;%d;%d;%dm' "$r" "$g" "$b"
}
