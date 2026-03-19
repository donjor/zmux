#!/usr/bin/env bash
# zmux installer — links zmux into PATH, then run zmux init

set -e

ZMUX_ROOT="$(cd "$(dirname "$0")" && pwd)"
BIN_DIR="${HOME}/.local/bin"

# ── Colors ──
gold='\033[38;2;230;180;80m'
green='\033[38;2;127;217;98m'
dim='\033[38;2;90;99;120m'
reset='\033[0m'
bold='\033[1m'

printf "\n"
printf "  ${gold}${bold}zmux installer${reset}\n"
printf "  ${dim}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${reset}\n\n"

# ── Link binaries ──
mkdir -p "$BIN_DIR"
ln -sf "$ZMUX_ROOT/bin/zmux0" "$BIN_DIR/zmux0"
ln -sf "$ZMUX_ROOT/bin/zmux0-apply-theme" "$BIN_DIR/zmux0-apply-theme"
printf "  ${green}✓${reset} linked zmux0 (v0) → %s/zmux0\n" "$BIN_DIR"

# Check PATH
if ! echo "$PATH" | tr ':' '\n' | grep -q "$BIN_DIR"; then
    printf "  ${dim}add to your shell rc: export PATH=\"\$HOME/.local/bin:\$PATH\"${reset}\n"
fi

printf "\n"
printf "  ${dim}Note: zmux0 is the legacy v0 bash version.${reset}\n"
printf "  ${dim}For v1 (Go): make install${reset}\n\n"
