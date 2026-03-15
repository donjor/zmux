#!/usr/bin/env bash
# tmux session overview — shown on new terminal windows

[ -z "$TMUX" ] && exit 0

source "$(dirname "$0")/theme.sh"
zmux_load_config
zmux_load_theme

accent=$(zmux_ansi "$ZMUX_ACCENT")
info=$(zmux_ansi "$ZMUX_INFO")
dim=$(zmux_ansi "$ZMUX_DIM")
success=$(zmux_ansi "$ZMUX_SUCCESS")
reset='\033[0m'
bold='\033[1m'

current=$(tmux display-message -p '#S')

printf "\n"

# List sessions (skip other tmp sessions, only show current + named)
while IFS= read -r line; do
    name=$(echo "$line" | cut -d: -f1)
    windows=$(echo "$line" | cut -d: -f2 | xargs)

    # Skip other tmp sessions
    [[ "$name" == tmp-* && "$name" != "$current" ]] && continue

    if [[ "$name" == "$current" ]]; then
        printf "  ${accent}${bold}▸ ${name}${reset}  ${dim}(${windows} windows)${reset}\n"
    else
        printf "  ${dim}  ${name}  (${windows} windows)${reset}\n"
    fi
done < <(tmux list-sessions -F '#{session_name}:#{session_windows}' 2>/dev/null | sort)

printf "\n"
printf "  ${dim}prefix: ctrl+space${reset}\n"
printf "  ${accent},${reset} rename session  ${info}s${reset} switch  ${info}c${reset} new tab  ${success}v${reset} copy mode  ${info}?${reset} help\n"
printf "\n"
