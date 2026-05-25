#!/usr/bin/env bash
# tmux which-key style help popup

source "$(dirname "$0")/theme.sh"
zmux_load_config
zmux_load_theme

accent=$(zmux_ansi "$ZMUX_ACCENT")
info=$(zmux_ansi "$ZMUX_INFO")
dim=$(zmux_ansi "$ZMUX_DIM")
success=$(zmux_ansi "$ZMUX_SUCCESS")
special=$(zmux_ansi "$ZMUX_SPECIAL")
error=$(zmux_ansi "$ZMUX_ERROR")
meta=$(zmux_ansi "$ZMUX_META")
fg=$(zmux_ansi "$ZMUX_FG")
reset='\033[0m'
bold='\033[1m'

clear
printf "\n"
printf "  ${accent}${bold}  tmux keybinds${reset}  ${dim}prefix: ctrl+space${reset}\n"
printf "  ${dim}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${reset}\n"
printf "\n"
printf "  ${accent}${bold} sessions${reset}\n"
printf "  ${info},${reset}  rename session        ${info}s${reset}  switch session\n"
printf "  ${info}x${reset}  kill session           ${info}d${reset}  detach\n"
printf "\n"
printf "  ${success}${bold} tabs${reset}\n"
printf "  ${info}c${reset}  new tab                ${info}.${reset}  rename tab\n"
printf "  ${info}n${reset}  next tab               ${info}p${reset}  prev tab\n"
printf "  ${info}Alt+1-5${reset}  jump to tab ${dim}(no prefix)${reset}\n"
printf "\n"
printf "  ${special}${bold} copy mode${reset}  ${dim}(vim keys)${reset}\n"
printf "  ${info}v${reset}  enter copy mode\n"
printf "  ${dim}then:${reset} ${success}v${reset} select  ${success}y${reset} yank  ${success}/${reset} search  ${success}Esc${reset} quit\n"
printf "\n"
printf "  ${meta}${bold} clipboard${reset}\n"
printf "  ${fg}ctrl+shift+c${reset}  copy     ${fg}ctrl+shift+v${reset}  paste\n"
printf "  ${fg}super+v${reset}       clipboard history ${dim}(rofi)${reset}\n"
printf "\n"
printf "  ${error}${bold} other${reset}\n"
printf "  ${info}r${reset}  reload config          ${info}?${reset}  this help\n"
printf "  ${dim}zmux${reset}  dashboard             ${dim}zmux theme${reset}  browse themes\n"
printf "\n"
printf "  ${dim}press any key to close${reset}\n"

read -rsn1
