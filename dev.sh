#!/usr/bin/env bash
# dev.sh — build + install a zmux binary for local testing.
#
# Usage: ./dev.sh [zmux|zzmux]   (default: zmux)
#   zmux   build + install the live binary, refresh shared agent integrations
#   zzmux  build + install an identical edge binary (binary only), so you can
#          test changes without overwriting the zmux you're currently running

set -euo pipefail

ZMUX_ROOT="$(cd "$(dirname "$0")" && pwd)"
TARGET="${1:-zmux}"

case "$TARGET" in
	zmux | zzmux) ;;
	*)
		printf "usage: %s [zmux|zzmux]\n" "$0" >&2
		exit 1
		;;
esac

bold='\033[1m'
dim='\033[38;2;90;99;120m'
green='\033[38;2;127;217;98m'
reset='\033[0m'

VERSION="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"

# Build
printf "${dim}building ${bold}%s${reset}${dim}...${reset} " "$TARGET"
go build -ldflags "-X main.version=${VERSION}" -o "$TARGET" ./cmd/zmux/ 2>&1
printf "${green}ok${reset}\n"

# Install binary (handle "text file busy" by removing first)
printf "${dim}installing...${reset} "
rm -f "$HOME/.local/bin/$TARGET" 2>/dev/null || true
cp "$TARGET" "$HOME/.local/bin/$TARGET"
printf "${green}ok${reset}  ${dim}~/.local/bin/%s${reset}\n" "$TARGET"

# Agent integration links only for the live binary. The skill is brand-agnostic;
# edge testing does not relink shared Claude/Codex/Pi state.
if [ "$TARGET" = "zmux" ]; then
	SKILLS_ROOT="${DONJOR_SKILLS_ROOT:-$HOME/donjor/skills}"

	if [ -d "$SKILLS_ROOT/skills" ]; then
		printf "${dim}linking shared skill...${reset} "
		ln -sfn "$ZMUX_ROOT/skills/zmux" "$SKILLS_ROOT/skills/zmux"
		printf "${green}ok${reset}  ${dim}%s/skills/zmux${reset}\n" "$SKILLS_ROOT"
	else
		printf "${dim}skipping shared skill link; missing %s/skills${reset}\n" "$SKILLS_ROOT"
	fi

	if [ -f "$SKILLS_ROOT/codex/sync-skills.mjs" ] && command -v node >/dev/null 2>&1; then
		printf "${dim}refreshing codex skill mirror...${reset} "
		node "$SKILLS_ROOT/codex/sync-skills.mjs" --no-global >/dev/null
		printf "${green}ok${reset}  ${dim}${CODEX_HOME:-$HOME/.codex}/skills${reset}\n"
	else
		printf "${dim}skipping codex mirror; missing node or %s/codex/sync-skills.mjs${reset}\n" "$SKILLS_ROOT"
	fi

	if [ -f "$SKILLS_ROOT/pi/sync-pi.mjs" ] && command -v node >/dev/null 2>&1; then
		printf "${dim}refreshing pi skill mirror...${reset} "
		node "$SKILLS_ROOT/pi/sync-pi.mjs" --no-codex-sync --no-global --no-settings >/dev/null
		printf "${green}ok${reset}  ${dim}~/.pi/agent/skills/donjor${reset}\n"
	else
		printf "${dim}skipping pi skill mirror; missing node or %s/pi/sync-pi.mjs${reset}\n" "$SKILLS_ROOT"
	fi

	printf "${dim}linking pi extension...${reset} "
	mkdir -p "$HOME/.pi/agent/extensions"
	rm -rf "$HOME/.pi/agent/extensions/pi-zmux"
	ln -s "$ZMUX_ROOT/pi-extension" "$HOME/.pi/agent/extensions/pi-zmux"
	printf "${green}ok${reset}  ${dim}~/.pi/agent/extensions/pi-zmux${reset}\n"
fi
