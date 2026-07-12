#!/usr/bin/env bash
# dev.sh — build + install a zmux binary for local testing.
#
# Usage: ./dev.sh [--dirty] [--branch] [zmux|zzmux]   (default: zmux)
#   zmux     build + install the live binary, refresh shell + agent integrations
#            refuses dirty or non-main/master checkouts unless explicitly
#            escaped with --dirty / --branch
#   zzmux    build + install an identical edge binary only; dirty worktrees and
#            topic branches are allowed because edge installs skip live shell
#            and shared agent integrations so you can test changes without
#            overwriting or mutating the zmux profile you're currently running
#
# Guard escapes for live zmux installs only:
#   --dirty   allow installing live zmux from a dirty worktree
#   --branch  allow installing live zmux from a non-main/master branch
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
#   ZMUX_SKIP_SHELL_SETUP        set to 1 to skip `setup shell --yes`

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
			ZMUX_SKIP_SHELL_SETUP) ZMUX_SKIP_SHELL_SETUP="$ORIGINAL_ZMUX_SKIP_SHELL_SETUP" ;;
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
			ZMUX_SKIP_SHELL_SETUP \
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
ORIGINAL_ZMUX_SKIP_SHELL_SETUP="${ZMUX_SKIP_SHELL_SETUP-}"
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

usage() {
	cat <<EOF
usage: $0 [--dirty] [--branch] [zmux|zzmux]

Targets:
  zmux     install the live binary and refresh shell/agent integrations (default)
  zzmux    install the edge binary only; allows dirty and topic-branch testing

Live zmux guard escapes:
  --dirty   allow installing live zmux from a dirty worktree
  --branch  allow installing live zmux from a non-main/master branch
EOF
}

TARGET="zmux"
TARGET_SET=0
ALLOW_DIRTY=0
ALLOW_BRANCH=0

while [ "$#" -gt 0 ]; do
	case "$1" in
		--dirty)
			ALLOW_DIRTY=1
			;;
		--branch)
			ALLOW_BRANCH=1
			;;
		-h | --help)
			usage
			exit 0
			;;
		zmux | zzmux)
			if [ "$TARGET_SET" = "1" ]; then
				printf "error: target already set to %s\n\n" "$TARGET" >&2
				usage >&2
				exit 2
			fi
			TARGET="$1"
			TARGET_SET=1
			;;
		*)
			printf "error: unknown argument: %s\n\n" "$1" >&2
			usage >&2
			exit 2
			;;
	esac
	shift
done

BIN_DIR="${ZMUX_BIN_DIR:-${ZMUX_INSTALL_BIN_DIR:-$HOME/.local/bin}}"
SKILLS_ROOT="${ZMUX_SKILLS_ROOT:-}"

bold='\033[1m'
dim='\033[38;2;90;99;120m'
green='\033[38;2;127;217;98m'
yellow='\033[38;2;245;158;11m'
red='\033[38;2;239;68;68m'
reset='\033[0m'

stable_branch_hint() {
	if git -C "$ZMUX_ROOT" show-ref --verify --quiet refs/heads/master; then
		printf "master"
	elif git -C "$ZMUX_ROOT" show-ref --verify --quiet refs/heads/main; then
		printf "main"
	else
		printf "main-or-master"
	fi
}

validate_live_install_state() {
	if [ "$TARGET" != "zmux" ]; then
		return 0
	fi

	local dirty_status=""
	local branch=""
	local ref=""
	local problems=0
	local override_cmd="./dev.sh"

	if ! git -C "$ZMUX_ROOT" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
		printf "${red}error${reset} refusing live zmux install: could not inspect git checkout state\n" >&2
		printf "${dim}helpers:${reset}\n" >&2
		printf "  ./dev.sh zzmux        # edge install is the safe test path\n" >&2
		printf "  git status --short    # run from a real zmux checkout before live install\n" >&2
		exit 1
	fi

	dirty_status="$(git -C "$ZMUX_ROOT" status --porcelain)"
	branch="$(git -C "$ZMUX_ROOT" branch --show-current 2>/dev/null || true)"
	if [ -z "$branch" ]; then
		ref="$(git -C "$ZMUX_ROOT" rev-parse --short HEAD 2>/dev/null || printf unknown)"
		branch="detached@$ref"
	fi

	if [ -n "$dirty_status" ] && [ "$ALLOW_DIRTY" != "1" ]; then
		printf "${red}error${reset} refusing live zmux install from a dirty worktree\n" >&2
		problems=1
		override_cmd="$override_cmd --dirty"
	fi

	case "$branch" in
		main | master) ;;
		*)
			if [ "$ALLOW_BRANCH" != "1" ]; then
				printf "${red}error${reset} refusing live zmux install from non-main/master branch: %s\n" "$branch" >&2
				problems=1
				override_cmd="$override_cmd --branch"
			fi
			;;
	esac

	if [ "$problems" = "1" ]; then
		printf "\n${dim}live zmux mutates %s/zmux plus shell/agent integrations; use zzmux for edge testing.${reset}\n" "$BIN_DIR" >&2
		printf "${dim}helpers:${reset}\n" >&2
		printf "  %-36s # build/install edge safely from this state\n" "./dev.sh zzmux" >&2
		printf "  %-36s # inspect uncommitted changes\n" "git status --short" >&2
		printf "  %-36s # return to the live-install branch\n" "git switch $(stable_branch_hint)" >&2
		printf "  %-36s # explicit live-install escape\n" "$override_cmd zmux" >&2
		exit 1
	fi

	if [ -n "$dirty_status" ]; then
		printf "${yellow}warning${reset} installing live zmux from a dirty worktree because --dirty was passed\n" >&2
	fi
	case "$branch" in
		main | master) ;;
		*)
			printf "${yellow}warning${reset} installing live zmux from %s because --branch was passed\n" "$branch" >&2
			;;
	esac
}

validate_live_install_state

if [ "$TARGET" = "zmux" ]; then
	printf "${dim}checking generated agent doctrine...${reset} "
	if node "$ZMUX_ROOT/agent-doctrine/generate.mjs" --check >/dev/null; then
		printf "${green}ok${reset}\n"
	else
		printf "${red}stale${reset}\n" >&2
		printf "error: generated agent doctrine is stale; run make gen-doctrine and commit the outputs before live sync\n" >&2
		exit 1
	fi
fi

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

if [ "$TARGET" = "zmux" ] && [ "${ZMUX_SKIP_SHELL_SETUP:-0}" != "1" ]; then
	printf "${dim}updating shell integration...${reset} "
	if setup_output=$("$BIN_DIR/$TARGET" setup shell --yes --bin zmux 2>&1); then
		printf "${green}ok${reset}\n"
		if [ -n "$setup_output" ]; then
			printf "%s\n" "$setup_output"
		fi
	else
		printf "${yellow}skip${reset}\n"
		printf "%s\n" "$setup_output"
	fi
else
	if [ "$TARGET" = "zmux" ]; then
		printf "${dim}skipping shell integration; ZMUX_SKIP_SHELL_SETUP=1${reset}\n"
	else
		printf "${dim}skipping shell integration for edge binary %s; run setup explicitly when needed${reset}\n" "$TARGET"
	fi
fi

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

	if [ -n "$SKILLS_ROOT" ] && [ -d "$SKILLS_ROOT/pi/extensions" ]; then
		printf "${dim}linking Pi package source...${reset} "
		ln -sfn "$ZMUX_ROOT/pi-zmux" "$SKILLS_ROOT/pi/extensions/pi-zmux"
		printf "${green}ok${reset}  ${dim}%s/pi/extensions/pi-zmux${reset}\n" "$SKILLS_ROOT"
	elif [ -n "$SKILLS_ROOT" ]; then
		printf "${dim}skipping Pi package source link; missing %s/pi/extensions${reset}\n" "$SKILLS_ROOT"
	fi

	if [ -n "$SKILLS_ROOT" ] && [ -x "$SKILLS_ROOT/sync" ] && command -v bun >/dev/null 2>&1; then
		for harness in codex pi gemini; do
			printf "${dim}refreshing %s skill mirror...${reset} " "$harness"
			"$SKILLS_ROOT/sync" skills apply --harness "$harness" >/dev/null
			printf "${green}ok${reset}\n"
		done
	elif [ -z "$SKILLS_ROOT" ]; then
		:
	else
		printf "${dim}skipping skill mirrors; missing bun or %s/sync${reset}\n" "$SKILLS_ROOT"
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
    console.log('warning pi-zmux extension is disabled in ~/.pi/agent/settings.json (remove the -extensions/pi-zmux entry, then run Pi /reload or restart Pi)');
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
