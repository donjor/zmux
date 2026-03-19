#!/usr/bin/env bash
# zmux init — setup wizard

# Requires ZMUX_ vars from lib/theme.sh

zmux_init() {
    local ZMUX_ROOT="${ZMUX_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
    local GUM="${GUM:-$(command -v gum 2>/dev/null || echo "$HOME/go/bin/gum")}"

    printf "\n"
    $GUM style --foreground "#e6b450" --bold --margin "0 2" "zmux init"
    $GUM style --foreground "#686868" --margin "0 2" "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    printf "\n"

    # ── Step 1: Check deps ──
    _init_check_deps || return 1

    # ── Step 2: Detect clipboard ──
    local clipboard
    clipboard=$(_init_detect_clipboard)

    # ── Step 3: Detect sync targets ──
    local ghostty_config=""
    local ghostty_theme=""
    local nvim_theme=""
    local has_nvim=false
    ghostty_config=$(_init_detect_ghostty)
    if command -v nvim &>/dev/null; then
        has_nvim=true
        $GUM style --foreground "#7fd962" --margin "0 2" "✓ nvim found" >&2
    fi

    # Read current themes from detected tools
    if [[ -n "$ghostty_config" ]]; then
        ghostty_theme=$(grep '^theme' "$ghostty_config" 2>/dev/null | tail -1 | cut -d'=' -f2- | sed 's/^[[:space:]]*//' | sed 's/[[:space:]]*$//' | sed 's/^"//' | sed 's/"$//')
        # Ensure the ghostty theme is available to zmux
        if [[ -n "$ghostty_theme" ]]; then
            if ! zmux_resolve_theme "$ghostty_theme"; then
                local ghostty_sys="/usr/share/ghostty/themes"
                if [[ -f "$ghostty_sys/$ghostty_theme" ]]; then
                    mkdir -p "${HOME}/.zmux/themes"
                    cp "$ghostty_sys/$ghostty_theme" "${HOME}/.zmux/themes/$ghostty_theme"
                fi
            fi
        fi
    fi
    if [[ "$has_nvim" == true ]]; then
        local _tmpf
        _tmpf=$(mktemp) 2>/dev/null
        if nvim --headless +"lua local f=io.open('$_tmpf','w'); f:write(vim.g.colors_name or ''); f:close()" +qa < /dev/null > /dev/null 2>&1; then
            nvim_theme=$(cat "$_tmpf" 2>/dev/null)
        fi
        rm -f "$_tmpf" 2>/dev/null
    fi

    # ── Step 4: Pick theme (with sync matches at top) ──
    local theme
    theme=$(_init_pick_theme "$ghostty_theme" "$nvim_theme") || { _init_cancelled; return 1; }

    # ── Step 5: Pick sync target ──
    local sync_target="none"
    sync_target=$(_init_pick_sync "$ghostty_config" "$has_nvim") || { _init_cancelled; return 1; }

    # ── Step 6: Pick status preset ──
    local preset
    preset=$(_init_pick_preset) || { _init_cancelled; return 1; }

    # ── Step 7: Offer iterm2 theme download ──
    _init_offer_download

    # ── Step 8: Generate ~/.zmux.conf ──
    _init_write_config "$theme" "$preset" "$sync_target" "$ghostty_config" "$clipboard"

    # ── Step 9: Set up ~/.tmux.conf ──
    _init_setup_tmux_conf

    # ── Step 10: Offer shell integration ──
    _init_offer_shell_integration

    # ── Step 11: Create user dirs ──
    mkdir -p "${HOME}/.zmux/themes"
    mkdir -p "${HOME}/.zmux/templates"

    printf "\n"
    $GUM style --foreground "#7fd962" --bold --margin "0 2" "✓ zmux initialized!"
    $GUM style --foreground "#686868" --margin "0 2" "restart tmux or run: tmux source ~/.tmux.conf"
    printf "\n"
}

_init_cancelled() {
    printf "\n"
    $GUM style --foreground "#686868" --margin "0 2" "  init cancelled"
    printf "\n"
}

_init_check_deps() {
    local ok=true

    if command -v tmux &>/dev/null; then
        $GUM style --foreground "#7fd962" --margin "0 2" "✓ tmux $(tmux -V 2>&1 | awk '{print $2}')"
    else
        $GUM style --foreground "#ea6c73" --margin "0 2" "✗ tmux not found — install it first"
        ok=false
    fi

    if command -v gum &>/dev/null || [[ -x "$GUM" ]]; then
        $GUM style --foreground "#7fd962" --margin "0 2" "✓ gum found"
    else
        $GUM style --foreground "#ea6c73" --margin "0 2" "✗ gum not found — install: go install github.com/charmbracelet/gum@latest"
        ok=false
    fi

    if command -v bash &>/dev/null; then
        $GUM style --foreground "#7fd962" --margin "0 2" "✓ bash $(bash --version | head -1 | grep -oP '\d+\.\d+\.\d+')"
    fi

    if command -v curl &>/dev/null; then
        $GUM style --foreground "#7fd962" --margin "0 2" "✓ curl (for theme download)"
    else
        $GUM style --foreground "#686868" --margin "0 2" "  curl not found (optional, for theme download)"
    fi

    printf "\n"
    $ok || return 1
}

_init_detect_clipboard() {
    if command -v wl-copy &>/dev/null; then
        $GUM style --foreground "#7fd962" --margin "0 2" "✓ clipboard: wl-copy (Wayland)" >&2
        echo "wl-copy"
    elif command -v xclip &>/dev/null; then
        $GUM style --foreground "#7fd962" --margin "0 2" "✓ clipboard: xclip (X11)" >&2
        echo "xclip -selection clipboard"
    elif command -v pbcopy &>/dev/null; then
        $GUM style --foreground "#7fd962" --margin "0 2" "✓ clipboard: pbcopy (macOS)" >&2
        echo "pbcopy"
    else
        $GUM style --foreground "#686868" --margin "0 2" "  no clipboard tool found" >&2
        echo ""
    fi
}

_init_detect_ghostty() {
    local config=""
    if [[ -f "${HOME}/.config/ghostty/config" ]]; then
        config="${HOME}/.config/ghostty/config"
        $GUM style --foreground "#7fd962" --margin "0 2" "✓ ghostty config found" >&2
    fi
    echo "$config"
}

_init_pick_theme() {
    local ghostty_theme="$1"
    local nvim_theme="$2"
    printf "\n" >&2

    local items=()
    local -A seen

    # Match with: section — detected themes from sync targets
    if [[ -n "$ghostty_theme" ]] && zmux_resolve_theme "$ghostty_theme" 2>/dev/null; then
        items+=("ghostty › $ghostty_theme")
        seen["$ghostty_theme"]=1
    fi
    if [[ -n "$nvim_theme" ]] && zmux_resolve_theme "$nvim_theme" 2>/dev/null; then
        items+=("nvim › $nvim_theme")
        seen["$nvim_theme"]=1
    fi

    # Add separator if we had matches
    [[ ${#items[@]} -gt 0 ]] && items+=("────────────────────")

    # All available themes
    for f in "$ZMUX_ROOT/themes/bundled/"*; do
        [[ -f "$f" ]] || continue
        local name=$(basename "$f")
        [[ -n "${seen[$name]}" ]] && continue
        items+=("$name")
    done
    if [[ -d "${HOME}/.zmux/themes" ]]; then
        for f in "${HOME}/.zmux/themes/"*; do
            [[ -f "$f" ]] || continue
            local name=$(basename "$f")
            [[ -n "${seen[$name]}" ]] && continue
            items+=("$name")
        done
    fi
    if [[ -d "$ZMUX_ROOT/themes/iterm2" ]]; then
        for f in "$ZMUX_ROOT/themes/iterm2/"*; do
            [[ -f "$f" ]] || continue
            local name=$(basename "$f")
            [[ -n "${seen[$name]}" ]] && continue
            items+=("$name")
        done
    fi

    if [[ ${#items[@]} -eq 0 ]]; then
        echo "ayu-dark"
        return 0
    fi

    local picked
    picked=$($GUM filter \
        --placeholder "pick a theme..." \
        --prompt "  ▸ " \
        --prompt.foreground "#e6b450" \
        --indicator.foreground "#e6b450" \
        --match.foreground "#53bdfa" \
        --header "  theme:" \
        --header.foreground "#e6b450" \
        --height 15 \
        "${items[@]}")
    local rc=$?
    [[ $rc -ne 0 ]] && return 1
    [[ -z "$picked" ]] && { echo "ayu-dark"; return 0; }

    # Strip "ghostty › " or "nvim › " prefix, skip separator
    [[ "$picked" == "────"* ]] && { echo "ayu-dark"; return 0; }
    picked="${picked#ghostty › }"
    picked="${picked#nvim › }"

    echo "$picked"
}

_init_pick_preset() {
    local picked
    picked=$($GUM choose \
        --cursor "  ▸ " \
        --cursor.foreground "#e6b450" \
        --selected.foreground "#53bdfa" \
        --item.foreground "#c7c7c7" \
        --header "  status bar preset:" \
        --header.foreground "#e6b450" \
        "default   — session pill, prefix hints, clock" \
        "minimal   — just session name and tabs" \
        "powerline — angled separators, filled segments" \
        "blocks    — square bracket segments")
    local rc=$?
    [[ $rc -ne 0 ]] && return 1

    echo "${picked%% *}"
}

_init_pick_sync() {
    local ghostty_config="$1"
    local has_nvim="$2"

    local choices=("skip")
    [[ -n "$ghostty_config" ]] && choices+=("ghostty")
    [[ "$has_nvim" == "true" ]] && choices+=("nvim")

    if [[ ${#choices[@]} -le 1 ]]; then
        echo "none"
        return 0
    fi

    local picked
    picked=$($GUM choose \
        --cursor "  ▸ " \
        --cursor.foreground "#e6b450" \
        --selected.foreground "#53bdfa" \
        --item.foreground "#c7c7c7" \
        --header "  default sync target for 'zmux theme sync':" \
        --header.foreground "#e6b450" \
        "${choices[@]}")
    local rc=$?
    [[ $rc -ne 0 ]] && return 1

    [[ "$picked" == "skip" ]] && picked="none"
    echo "${picked:-none}"
}

_init_offer_download() {
    if [[ -d "$ZMUX_ROOT/themes/iterm2" ]] && [[ -n "$(ls -A "$ZMUX_ROOT/themes/iterm2" 2>/dev/null)" ]]; then
        return 0
    fi

    command -v curl &>/dev/null || return 0

    printf "\n"
    if $GUM confirm \
        --selected.background "#e6b450" --selected.foreground "#0b0e14" \
        --unselected.background "#11151c" --unselected.foreground "#c7c7c7" \
        "  download ~300 extra themes from iterm2-color-schemes?"; then

        $GUM style --foreground "#686868" --margin "0 2" "downloading themes..."
        mkdir -p "$ZMUX_ROOT/themes/iterm2"

        local tmpdir
        tmpdir=$(mktemp -d) || { $GUM style --foreground "#ea6c73" --margin "0 2" "✗ failed to create temp dir"; return 1; }
        if curl -sL "https://github.com/mbadolato/iTerm2-Color-Schemes/archive/refs/heads/master.tar.gz" \
            | tar xz -C "$tmpdir" --strip-components=2 "iTerm2-Color-Schemes-master/ghostty" 2>/dev/null; then
            mv "$tmpdir"/* "$ZMUX_ROOT/themes/iterm2/" 2>/dev/null
            local count=$(ls -1 "$ZMUX_ROOT/themes/iterm2/" 2>/dev/null | wc -l)
            $GUM style --foreground "#7fd962" --margin "0 2" "✓ downloaded $count themes"
        else
            $GUM style --foreground "#ea6c73" --margin "0 2" "✗ download failed (you can retry with zmux init)"
        fi
        rm -rf "$tmpdir"
    fi
}

_init_write_config() {
    local theme="$1" preset="$2" sync_target="$3" ghostty_config="$4" clipboard="$5"
    local conf="${HOME}/.zmux.conf"

    # Warn if existing config will be overwritten
    if [[ -f "$conf" ]]; then
        if ! $GUM confirm \
            --selected.background "#e6b450" --selected.foreground "#0b0e14" \
            --unselected.background "#11151c" --unselected.foreground "#c7c7c7" \
            "  ~/.zmux.conf exists — overwrite?"; then
            $GUM style --foreground "#686868" --margin "0 2" "  kept existing config"
            return 0
        fi
    fi

    {
        echo "# zmux configuration"
        echo "# https://github.com/donjor/zmux"
        echo ""
        echo "# Theme - any bundled or iterm2-color-schemes theme name"
        echo "theme = $theme"
        echo ""
        echo "# Status bar preset: default, minimal, powerline, blocks"
        echo "status = $preset"
        echo ""
        echo "# Prefix key"
        echo "prefix = C-Space"
        echo ""
        echo "# Clipboard tool (auto-detected, or set manually)"
        echo "clipboard = ${clipboard:-auto}"
        echo ""
        echo "# Theme sync - pull-only, reads from another tool to match zmux"
        echo "sync.target = $sync_target"
        if [[ -n "$ghostty_config" && "$sync_target" == "ghostty" ]]; then
            echo "sync.ghostty_config = auto"
        fi
    } > "$conf"

    $GUM style --foreground "#7fd962" --margin "0 2" "✓ wrote ~/.zmux.conf"
}

_init_setup_tmux_conf() {
    local tmux_conf="${HOME}/.tmux.conf"
    local zmux_conf_line="source-file ${ZMUX_ROOT}/tmux/zmux.tmux.conf"
    local zmux_run_line="run-shell \"${ZMUX_ROOT}/bin/zmux0-apply-theme\""

    printf "\n"

    # Check if already configured
    if [[ -f "$tmux_conf" ]] && grep -q "zmux.tmux.conf" "$tmux_conf"; then
        $GUM style --foreground "#686868" --margin "0 2" "  ~/.tmux.conf already sources zmux"
        return 0
    fi

    if $GUM confirm \
        --selected.background "#e6b450" --selected.foreground "#0b0e14" \
        --unselected.background "#11151c" --unselected.foreground "#c7c7c7" \
        "  add zmux to ~/.tmux.conf?"; then

        # Append zmux lines
        printf '\n# zmux\n%s\n%s\n' "$zmux_conf_line" "$zmux_run_line" >> "$tmux_conf"
        $GUM style --foreground "#7fd962" --margin "0 2" "✓ updated ~/.tmux.conf"
    else
        printf "\n"
        $GUM style --foreground "#686868" --margin "0 2" "add to your ~/.tmux.conf:"
        $GUM style --foreground "#e6b450" --margin "0 4" "# zmux"
        $GUM style --foreground "#e6b450" --margin "0 4" "$zmux_conf_line"
        $GUM style --foreground "#e6b450" --margin "0 4" "$zmux_run_line"
    fi
}

_init_offer_shell_integration() {
    printf "\n"

    # Check if already in shell rc
    local shell_rc=""
    if [[ -n "$ZSH_VERSION" ]] || [[ "$SHELL" == *zsh ]]; then
        shell_rc="${HOME}/.zshrc"
    else
        shell_rc="${HOME}/.bashrc"
    fi

    if [[ -f "$shell_rc" ]] && grep -q "zmux" "$shell_rc"; then
        $GUM style --foreground "#686868" --margin "0 2" "  shell integration already configured"
        return 0
    fi

    if $GUM confirm \
        --selected.background "#e6b450" --selected.foreground "#0b0e14" \
        --unselected.background "#11151c" --unselected.foreground "#c7c7c7" \
        "  add zmux auto-start to $(basename "$shell_rc")?"; then

        cat >> "$shell_rc" <<'SHELL'

# zmux — auto-start tmux session picker
if command -v tmux &>/dev/null && [ -z "$TMUX" ]; then
    zmux
fi
SHELL
        $GUM style --foreground "#7fd962" --margin "0 2" "✓ updated $(basename "$shell_rc")"
    fi
}
