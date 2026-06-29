#!/usr/bin/env bash
# dev.sh — build + install a zmux binary for local testing.
#
# Usage: ./dev.sh [zmux|zzmux]   (default: zmux)
#   zmux   build + install the live binary, refresh shared agent integrations
#   zzmux  build + install an identical edge binary (binary only), so you can
#          test changes without overwriting the zmux you're currently running
#
# Local config:
#   .env            shell-style env assignments, ignored by git
#   .dev.local.sh   optional local extension script, ignored by git
#                   may define zmux_dev_after_build,
#                   zmux_dev_after_install, zmux_dev_after_integrations
#
# Env:
#   ZMUX_BIN_DIR                 binary install dir (default: ~/.local/bin)
#   ZMUX_SKILLS_ROOT             shared skills repo root
#   ZMUX_SKIP_AGENT_INTEGRATIONS set to 1 to skip skill/mirror/extension updates

set -euo pipefail

ZMUX_ROOT="$(cd "$(dirname "$0")" && pwd)"

preexisting_var_names() {
	local name
	for name in "$@"; do
		if [ "${!name+x}" = "x" ]; then
			printf "%s\n" "$name"
		fi
	done
}

restore_preexisting_vars() {
	local name
	for name in "$@"; do
		case "$name" in
			ZMUX_BIN_DIR) ZMUX_BIN_DIR="$ORIGINAL_ZMUX_BIN_DIR" ;;
			ZMUX_DEV_LOCAL) ZMUX_DEV_LOCAL="$ORIGINAL_ZMUX_DEV_LOCAL" ;;
			ZMUX_INSTALL_BIN_DIR) ZMUX_INSTALL_BIN_DIR="$ORIGINAL_ZMUX_INSTALL_BIN_DIR" ;;
			ZMUX_SKIP_AGENT_INTEGRATIONS) ZMUX_SKIP_AGENT_INTEGRATIONS="$ORIGINAL_ZMUX_SKIP_AGENT_INTEGRATIONS" ;;
			ZMUX_SKILLS_ROOT) ZMUX_SKILLS_ROOT="$ORIGINAL_ZMUX_SKILLS_ROOT" ;;
		esac
		export "$name"
	done
}

load_env_file() {
	local file="$1"
	local preexisting
	preexisting="$(
		preexisting_var_names \
			ZMUX_BIN_DIR \
			ZMUX_DEV_LOCAL \
			ZMUX_INSTALL_BIN_DIR \
			ZMUX_SKIP_AGENT_INTEGRATIONS \
			ZMUX_SKILLS_ROOT
	)"

	set -a
	# shellcheck disable=SC1090
	. "$file"
	set +a

	if [ -n "$preexisting" ]; then
		restore_preexisting_vars $preexisting
	fi
}

ORIGINAL_ZMUX_BIN_DIR="${ZMUX_BIN_DIR-}"
ORIGINAL_ZMUX_DEV_LOCAL="${ZMUX_DEV_LOCAL-}"
ORIGINAL_ZMUX_INSTALL_BIN_DIR="${ZMUX_INSTALL_BIN_DIR-}"
ORIGINAL_ZMUX_SKIP_AGENT_INTEGRATIONS="${ZMUX_SKIP_AGENT_INTEGRATIONS-}"
ORIGINAL_ZMUX_SKILLS_ROOT="${ZMUX_SKILLS_ROOT-}"

if [ -f "$ZMUX_ROOT/.env" ]; then
	load_env_file "$ZMUX_ROOT/.env"
fi

LOCAL_EXTENSION="${ZMUX_DEV_LOCAL:-$ZMUX_ROOT/.dev.local.sh}"
if [ -f "$LOCAL_EXTENSION" ]; then
	# shellcheck disable=SC1090
	. "$LOCAL_EXTENSION"
fi

run_local_hook() {
	local hook="$1"
	shift
	if declare -F "$hook" >/dev/null; then
		"$hook" "$@"
	fi
}

TARGET="${1:-zmux}"
BIN_DIR="${ZMUX_BIN_DIR:-${ZMUX_INSTALL_BIN_DIR:-$HOME/.local/bin}}"
SKILLS_ROOT="${ZMUX_SKILLS_ROOT:-}"

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
yellow='\033[38;2;245;158;11m'
reset='\033[0m'

VERSION="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"

# Build
printf "${dim}building ${bold}%s${reset}${dim}...${reset} " "$TARGET"
go build -ldflags "-X main.version=${VERSION}" -o "$TARGET" ./cmd/zmux/ 2>&1
printf "${green}ok${reset}\n"
run_local_hook zmux_dev_after_build "$TARGET" "$ZMUX_ROOT"

# Install binary (handle "text file busy" by removing first)
printf "${dim}installing...${reset} "
mkdir -p "$BIN_DIR"
rm -f "$BIN_DIR/$TARGET" 2>/dev/null || true
cp "$TARGET" "$BIN_DIR/$TARGET"
printf "${green}ok${reset}  ${dim}%s/%s${reset}\n" "$BIN_DIR" "$TARGET"
run_local_hook zmux_dev_after_install "$TARGET" "$ZMUX_ROOT" "$BIN_DIR"

# Agent integration links only for the live binary. The skill is brand-agnostic;
# edge testing does not relink shared Claude/Codex/Pi state.
if [ "$TARGET" = "zmux" ] && [ "${ZMUX_SKIP_AGENT_INTEGRATIONS:-0}" != "1" ]; then
	if [ -z "$SKILLS_ROOT" ]; then
		printf "${dim}skipping skill mirrors; set ZMUX_SKILLS_ROOT in .env${reset}\n"
	elif [ -d "$SKILLS_ROOT/skills" ]; then
		printf "${dim}linking shared skill...${reset} "
		ln -sfn "$ZMUX_ROOT/skills/zmux" "$SKILLS_ROOT/skills/zmux"
		printf "${green}ok${reset}  ${dim}%s/skills/zmux${reset}\n" "$SKILLS_ROOT"
	else
		printf "${dim}skipping shared skill link; missing %s/skills${reset}\n" "$SKILLS_ROOT"
	fi

	if [ -n "$SKILLS_ROOT" ] && [ -f "$SKILLS_ROOT/codex/sync-skills.mjs" ] && command -v node >/dev/null 2>&1; then
		printf "${dim}refreshing codex skill mirror...${reset} "
		node "$SKILLS_ROOT/codex/sync-skills.mjs" --no-global >/dev/null
		printf "${green}ok${reset}  ${dim}${CODEX_HOME:-$HOME/.codex}/skills${reset}\n"
	elif [ -z "$SKILLS_ROOT" ]; then
		:
	else
		printf "${dim}skipping codex mirror; missing node or %s/codex/sync-skills.mjs${reset}\n" "$SKILLS_ROOT"
	fi

	if [ -n "$SKILLS_ROOT" ] && [ -f "$SKILLS_ROOT/pi/sync-pi.mjs" ] && command -v node >/dev/null 2>&1; then
		printf "${dim}refreshing pi skill mirror...${reset} "
		node "$SKILLS_ROOT/pi/sync-pi.mjs" --no-codex-sync --no-global --no-settings >/dev/null
		printf "${green}ok${reset}\n"
	elif [ -z "$SKILLS_ROOT" ]; then
		:
	else
		printf "${dim}skipping pi skill mirror; missing node or %s/pi/sync-pi.mjs${reset}\n" "$SKILLS_ROOT"
	fi

	if [ -n "$SKILLS_ROOT" ] && [ -f "$SKILLS_ROOT/gemini/sync-gemini.mjs" ] && command -v node >/dev/null 2>&1; then
		printf "${dim}refreshing gemini skill mirror...${reset} "
		node "$SKILLS_ROOT/gemini/sync-gemini.mjs" --no-codex-sync --no-global >/dev/null
		printf "${green}ok${reset}  ${dim}${GEMINI_HOME:-$HOME/.gemini}/skills${reset}\n"
	elif [ -z "$SKILLS_ROOT" ]; then
		:
	else
		printf "${dim}skipping gemini mirror; missing node or %s/gemini/sync-gemini.mjs${reset}\n" "$SKILLS_ROOT"
	fi

	printf "${dim}checking pi extension package...${reset} "
	if [ -L "$HOME/.pi/agent/extensions/pi-zmux" ]; then
		rm -f "$HOME/.pi/agent/extensions/pi-zmux"
		printf "${green}ok${reset}  ${dim}removed legacy ~/.pi/agent/extensions/pi-zmux; settings package is source of truth${reset}\n"
	elif [ -e "$HOME/.pi/agent/extensions/pi-zmux" ]; then
		printf "${yellow}skip${reset}  ${dim}~/.pi/agent/extensions/pi-zmux exists but is not a symlink${reset}\n"
	else
		printf "${green}ok${reset}  ${dim}settings package is source of truth${reset}\n"
	fi
	if command -v node >/dev/null 2>&1 && [ -f "$HOME/.pi/agent/settings.json" ]; then
		node - "$HOME/.pi/agent/settings.json" <<'NODE'
const fs = require('node:fs');
const settingsPath = process.argv[2];
try {
  const settings = JSON.parse(fs.readFileSync(settingsPath, 'utf8'));
  const extensions = Array.isArray(settings.extensions) ? settings.extensions : [];
  const disabled = extensions.some((entry) => entry === '-extensions/pi-zmux/index.ts' || entry === '-extensions/pi-zmux' || entry === '-extensions/pi-zmux/index.js');
  if (disabled) {
    console.log('warning pi-zmux extension is disabled in ~/.pi/agent/settings.json (remove the -extensions/pi-zmux entry, then /zmux reload or restart Pi)');
  } else {
    console.log('ok pi-zmux extension is not disabled by ~/.pi/agent/settings.json');
  }
} catch (error) {
  console.log(`warning could not inspect ~/.pi/agent/settings.json: ${error.message}`);
}
NODE
	fi
	run_local_hook zmux_dev_after_integrations "$TARGET" "$ZMUX_ROOT" "$SKILLS_ROOT"
elif [ "$TARGET" = "zmux" ]; then
	printf "${dim}skipping agent integrations; ZMUX_SKIP_AGENT_INTEGRATIONS=1${reset}\n"
fi
