import type { PiZmuxConfig } from "./config.js";

export function hasExplicitBypass(command: string): boolean {
	return /(^|\s)PI_ZMUX_ALLOW=1(\s|$)/u.test(command) || /#\s*pi-zmux:\s*allow\b/iu.test(command);
}

export type BashClassification =
	| { kind: "safe"; reason: string }
	| { kind: "background"; reason: string; suggestion: string }
	| { kind: "runtime"; reason: string; suggestion: string }
	| { kind: "interactive"; reason: string; suggestion: string }
	| { kind: "direct_zmux"; reason: string; suggestion: string }
	| { kind: "direct_tmux"; reason: string; suggestion: string }
	| { kind: "headless_agent"; reason: string; suggestion: string };

const directZmuxPatterns: Array<{ re: RegExp; reason: string; tool: string }> = [
	{ re: /(^|[;&|\n]\s*)zmux\s+tab\s+status\b[\s\S]*zmux\s+watch\b|(^|[;&|\n]\s*)zmux\s+watch\b[\s\S]*zmux\s+tab\s+status\b/u, reason: "combined tab status/output inspection has a typed tool", tool: "zmux_tab_inspect" },
	{ re: /(^|[;&|\n]\s*)zmux\s+run\b[^\n;&|]*\s-n\s+\S*peer\b/u, reason: "peer tab startup has a typed tool", tool: "zmux_peer_ensure" },
	{ re: /(^|[;&|\n]\s*)zmux\s+tabs\b/u, reason: "tab listing has a typed tool", tool: "zmux_tabs" },
	{ re: /(^|[;&|\n]\s*)zmux\s+ls\b/u, reason: "session listing has a typed tool", tool: "zmux_sessions" },
	{ re: /(^|[;&|\n]\s*)zmux\s+where\b/u, reason: "context inspection has a typed tool", tool: "zmux_current" },
	{ re: /(^|[;&|\n]\s*)zmux\s+tab\s+state\b/u, reason: "tab lifecycle state has a typed tool", tool: "zmux_tab_state" },
	{ re: /(^|[;&|\n]\s*)zmux\s+tab\s+status\b/u, reason: "tab lifecycle/command status has a typed tool", tool: "zmux_tab_status" },
	{ re: /(^|[;&|\n]\s*)zmux\s+tab\s+peer\b/u, reason: "peer lifecycle metadata has a typed tool", tool: "zmux_tab_peer" },
	{ re: /(^|[;&|\n]\s*)zmux\s+tab\s+label\b/u, reason: "tab labelling has a typed tool", tool: "zmux_tab_label" },
	{ re: /(^|[;&|\n]\s*)zmux\s+tab\s+move\b/u, reason: "tab moving has a typed tool", tool: "zmux_tab_move" },
	{ re: /(^|[;&|\n]\s*)zmux\s+tab\s+(pane|full|hide|show)\b/u, reason: "tab placement has a typed tool", tool: "zmux_tab_place" },
	{ re: /(^|[;&|\n]\s*)zmux\s+tab\s+kill\b/u, reason: "tab cleanup has a typed tool", tool: "zmux_tab_kill" },
	{ re: /(^|[;&|\n]\s*)zmux\s+session\s+run\b/u, reason: "focus-safe session spawn has a typed tool", tool: "zmux_session_run" },
	{ re: /(^|[;&|\n]\s*)zmux\s+session\s+kill\b/u, reason: "session cleanup has a typed tool", tool: "zmux_session_kill" },
	{ re: /(^|[;&|\n]\s*)zmux\s+send\b/u, reason: "key sending has a typed tool", tool: "zmux_send_keys" },
	{ re: /(^|[;&|\n]\s*)zmux\s+type\b/u, reason: "typing has typed tools", tool: "zmux_type (optionally waitForTurnState for peers) or zmux_interactive_type" },
	{ re: /(^|[;&|\n]\s*)zmux\s+pane\s+list\b/u, reason: "pane listing has a typed tool", tool: "zmux_pane_list" },
	{ re: /(^|[;&|\n]\s*)zmux\s+pane\s+open\b/u, reason: "pane opening has a typed tool", tool: "zmux_pane_open" },
	{ re: /(^|[;&|\n]\s*)zmux\s+pane\s+focus\b/u, reason: "pane focus has a typed tool", tool: "zmux_pane_focus" },
	{ re: /(^|[;&|\n]\s*)zmux\s+pane\s+close\b/u, reason: "pane cleanup has a typed tool", tool: "zmux_pane_close" },
	{ re: /(^|[;&|\n]\s*)zmux\s+pane\s+resize\b/u, reason: "pane resize has a typed tool", tool: "zmux_pane_resize" },
	{ re: /(^|[;&|\n]\s*)zmux\s+run\b/u, reason: "command-in-tab execution has a typed tool", tool: "zmux_run (or zmux_runtime_ensure for persistent runtimes)" },
	{ re: /(^|[;&|\n]\s*)zmux\s+watch\b/u, reason: "runtime logs and tab inspection have typed tools", tool: "zmux_runtime_logs (supports waitFor/idleSeconds) or zmux_tab_inspect" },
	{ re: /(^|[;&|\n]\s*)zmux\s+log\b/u, reason: "persistent tab logging has a typed tool", tool: "zmux_log" },
	{ re: /(^|[;&|\n]\s*)zmux\s+snapshot\b/u, reason: "terminal evidence capture has a typed tool", tool: "zmux_snapshot" },
	{ re: /(^|[;&|\n]\s*)zmux\s+terminal\s+current\b/u, reason: "terminal target inspection has a typed tool", tool: "zmux_terminal_current" },
];

// Raw tmux app-subcommands that have a zmux equivalent. The KEY SET mirrors
// internal/guard/guard.go's tmuxTargets exactly (the shared corpus is the drift
// gate); the redirect text points at pi's typed tool where one exists, or the
// zmux CLI otherwise. Subcommands absent here (info, has-session, display-message)
// have no clean equivalent and pass through as safe.
const tmuxSubcommandRedirects: Record<string, { reason: string; tool: string }> = {
	"capture-pane": { reason: "reading pane output has a typed tool", tool: "the zmux_runtime_logs typed tool" },
	capturep: { reason: "reading pane output has a typed tool", tool: "the zmux_runtime_logs typed tool" },
	"send-keys": { reason: "pane key sending has a typed tool", tool: "the zmux_pane_send_keys / zmux_pane_type typed tools" },
	send: { reason: "key sending has a typed tool", tool: "the zmux_send_keys / zmux_type typed tools" },
	"list-windows": { reason: "tab listing has a typed tool", tool: "the zmux_tabs typed tool" },
	lsw: { reason: "tab listing has a typed tool", tool: "the zmux_tabs typed tool" },
	"list-sessions": { reason: "session listing has a typed tool", tool: "the zmux_sessions typed tool" },
	ls: { reason: "session listing has a typed tool", tool: "the zmux_sessions typed tool" },
	"list-panes": { reason: "pane listing has a typed tool", tool: "the zmux_pane_list typed tool" },
	lsp: { reason: "pane listing has a typed tool", tool: "the zmux_pane_list typed tool" },
	"split-window": { reason: "pane opening has a typed tool", tool: "the zmux_pane_open typed tool" },
	splitw: { reason: "pane opening has a typed tool", tool: "the zmux_pane_open typed tool" },
	"select-pane": { reason: "pane focus has a typed tool", tool: "the zmux_pane_focus typed tool" },
	selectp: { reason: "pane focus has a typed tool", tool: "the zmux_pane_focus typed tool" },
	"kill-pane": { reason: "pane cleanup has a typed tool", tool: "the zmux_pane_close typed tool" },
	killp: { reason: "pane cleanup has a typed tool", tool: "the zmux_pane_close typed tool" },
	"resize-pane": { reason: "pane resize has a typed tool", tool: "the zmux_pane_resize typed tool" },
	resizep: { reason: "pane resize has a typed tool", tool: "the zmux_pane_resize typed tool" },
	"new-window": { reason: "starting work in a tab has a typed tool", tool: "the zmux_run typed tool (or zmux_runtime_ensure for persistent runtimes)" },
	neww: { reason: "starting work in a tab has a typed tool", tool: "the zmux_run typed tool (or zmux_runtime_ensure for persistent runtimes)" },
	"kill-window": { reason: "tab cleanup has a typed tool", tool: "the zmux_tab_kill typed tool" },
	killw: { reason: "tab cleanup has a typed tool", tool: "the zmux_tab_kill typed tool" },
	"rename-window": { reason: "tab labelling has a typed tool", tool: "the zmux_tab_label typed tool" },
	renamew: { reason: "tab labelling has a typed tool", tool: "the zmux_tab_label typed tool" },
	"move-window": { reason: "tab moving has a typed tool", tool: "the zmux_tab_move typed tool" },
	movew: { reason: "tab moving has a typed tool", tool: "the zmux_tab_move typed tool" },
	"select-window": { reason: "tab focus has a typed tool", tool: "the zmux_tab_focus typed tool" },
	selectw: { reason: "tab focus has a typed tool", tool: "the zmux_tab_focus typed tool" },
	"new-session": { reason: "session creation belongs in zmux", tool: "the zmux_session_run typed tool for command-backed sessions, or the zmux CLI `zmux new` for attaching user sessions" },
	new: { reason: "session creation belongs in zmux", tool: "the zmux_session_run typed tool for command-backed sessions, or the zmux CLI `zmux new` for attaching user sessions" },
	"kill-session": { reason: "session cleanup has a typed tool", tool: "the zmux_session_kill typed tool" },
	"attach-session": { reason: "attaching belongs in zmux", tool: "the zmux CLI `zmux open` (no typed attach tool yet)" },
	attach: { reason: "attaching belongs in zmux", tool: "the zmux CLI `zmux open` (no typed attach tool yet)" },
	"switch-client": { reason: "client switching belongs in zmux", tool: "the zmux CLI `zmux open` (no typed switch tool yet)" },
	switchc: { reason: "client switching belongs in zmux", tool: "the zmux CLI `zmux open` (no typed switch tool yet)" },
};

const tmuxFlagWithArg = new Set(["-L", "-f", "-S", "-c"]);
// segSplit breaks a (quote-stripped) command into simple-command segments on
// shell control operators, so each can be checked for a command-position tmux.
const segSplit = /[;&|\n]+/u;
// Matches a here-document redirection (`<<EOF`, `<<-'EOF'`, `<< "EOF"`),
// capturing the delimiter word so stripHeredocs can close the body by line scan.
const heredocStart = /<<-?\s*["']?([A-Za-z_][A-Za-z0-9_]*)["']?/u;

// Pulls the payload out of a command-position `sh -c '…'` / `bash -lc "…"` so the
// inner command can be recursively classified — a raw tmux or `&` in the quoted
// `-c` arg would otherwise be blanked by the quote-strip and escape. Anchored at
// segment position so a quoted mention or argument isn't matched; an optional
// `env ` wrapper and path prefix keep `env sh -c …` / `/bin/sh -c …` from slipping
// past. Mirrors internal/guard/guard.go's shellCExtract. Global for matchAll;
// capture 1 is the payload.
const shellCExtract = /(?:^|[;&|\n]\s*)(?:env\s+)?(?:\S*\/)?(?:sh|bash|zsh|dash|ksh)\s+-[a-zA-Z]*c[a-zA-Z]*\s+('[^']*'|"[^"]*"|`[^`]*`|[^\s;&|]+)/gu;
// Command words that EXECUTE a here-doc body fed on stdin (`bash <<EOF … EOF`).
// A file-writer receiver (cat/tee) makes the body inert data and is skipped.
const shellReceivers = new Set(["sh", "bash", "zsh", "dash", "ksh"]);
// xargs options that consume the next token as their value.
const xargsValueFlags = new Set(["-I", "-i", "-n", "-P", "-s", "-d", "-E", "-a", "-L"]);
// Bounds the recursive payload scan so a pathological nest can't loop.
const maxClassifyDepth = 4;

function tmuxSubcommand(rest: string): string {
	const toks = rest.trim().split(/\s+/u).filter(Boolean);
	for (let i = 0; i < toks.length; ) {
		if (toks[i].startsWith("-")) {
			i += tmuxFlagWithArg.has(toks[i]) ? 2 : 1;
			continue;
		}
		return toks[i];
	}
	return "";
}

// hasSocketFlag reports whether a tmux arg list is socket-scoped (`-L <socket>`),
// marking it as zzmux/profile testing — exempt (pi folds this into safe).
function hasSocketFlag(args: string[]): boolean {
	return args.some((a) => a === "-L" || a.startsWith("-L"));
}

// classifyTmux scans each simple-command segment for a command-position raw tmux
// call (first token == "tmux"). A mapped, non-socket subcommand returns a
// direct_tmux block; socket-scoped or unmapped invocations are skipped — so
// `tmux info; tmux capture-pane` still blocks on the second segment and
// `echo tmux capture-pane` (tmux as an argument) is never flagged. Mirrors
// internal/guard/guard.go's scanTmux (pi has no repo-cwd exemption).
function classifyTmux(scan: string): BashClassification | null {
	for (const seg of scan.split(segSplit)) {
		const toks = seg.trim().split(/\s+/u).filter(Boolean);
		if (toks.length === 0) continue;
		// Raw tmux at command position, or `xargs … tmux …` where tmux is the
		// command xargs execs. Either way `args` is everything after tmux.
		let args: string[];
		if (toks[0] === "tmux") {
			args = toks.slice(1);
		} else if (toks[0] === "xargs") {
			const cmd = xargsCommand(toks);
			if (cmd.length === 0 || cmd[0] !== "tmux") continue;
			args = cmd.slice(1);
		} else {
			continue;
		}
		if (hasSocketFlag(args)) continue; // socket-scoped (zzmux/profile) → exempt
		const redirect = tmuxSubcommandRedirects[tmuxSubcommand(args.join(" "))];
		if (redirect) {
			return { kind: "direct_tmux", reason: redirect.reason, suggestion: suggestionForTmux(redirect.tool) };
		}
		// unmapped subcommand (info, has-session, ...) — no zmux verb; keep scanning
	}
	return null;
}

// xargsCommand returns the command (word + args) an `xargs …` segment would
// execute, skipping xargs's own flags. toks[0] is "xargs". Combined flags
// (`-n1`, `-I{}`) skip as one token; value-taking flags spelled apart (`-n 1`)
// skip their value too. Returns [] if no command word follows. Mirrors guard.go.
function xargsCommand(toks: string[]): string[] {
	for (let i = 1; i < toks.length; ) {
		const t = toks[i];
		if (t.startsWith("-")) {
			i += xargsValueFlags.has(t) ? 2 : 1;
			continue;
		}
		return toks.slice(i);
	}
	return [];
}

// executablePayloads returns inner command strings a segment would itself
// execute — `sh -c '<payload>'` args and here-doc bodies fed to a shell — so
// classifyBash can recurse into them. Env prefixes stripped first so
// `FOO=bar sh -c …` still matches; here-doc bodies blanked before the
// shellCExtract scan so a `sh -c '…'` inside an INERT file-writer here-doc
// (`cat > run.sh <<'EOF' … EOF`) isn't falsely extracted — executable
// shell-receiver bodies are recovered separately by shellHeredocBodies on the
// raw command. Mirrors internal/guard/guard.go's executablePayloads.
function executablePayloads(command: string): string[] {
	const out: string[] = [];
	for (const m of stripHeredocs(stripEnvPrefix(command)).matchAll(shellCExtract)) {
		out.push(unquotePayload(m[1]));
	}
	return out.concat(shellHeredocBodies(command));
}

// heredocReceiver returns the command word of a here-doc's opening line,
// normalized so a here-doc fed to a path-qualified or env-wrapped shell still
// matches shellReceivers (env assignments + a bare `env` dropped, path
// basename'd). Mirrors internal/guard/guard.go's heredocReceiver.
function heredocReceiver(openLine: string): string {
	const toks = stripEnvPrefix(openLine).trim().split(/\s+/u).filter(Boolean);
	if (toks.length === 0) return "";
	let word = toks[0];
	if (word === "env" && toks.length > 1) word = toks[1];
	return word.slice(word.lastIndexOf("/") + 1);
}

// unquotePayload strips a single wrapping quote pair from a captured `-c` arg.
function unquotePayload(p: string): string {
	if (p.length >= 2) {
		const q = p[0];
		if ((q === "'" || q === '"' || q === "`") && p[p.length - 1] === q) {
			return p.slice(1, -1);
		}
	}
	return p;
}

// shellHeredocBodies returns the bodies of here-documents whose receiver is a
// shell (`bash <<EOF … EOF`), which executes the body. A file-writer receiver
// (`cat > f <<EOF`, `tee`) makes the body inert data and is skipped. Mirrors
// internal/guard/guard.go's shellHeredocBodies.
function shellHeredocBodies(command: string): string[] {
	if (!command.includes("<<")) return [];
	const bodies: string[] = [];
	let cur: string[] = [];
	let tag = "";
	let capturing = false;
	for (const line of command.split("\n")) {
		if (tag) {
			if (line.trim() === tag) {
				if (capturing) bodies.push(cur.join("\n"));
				tag = "";
				cur = [];
				capturing = false;
				continue;
			}
			if (capturing) cur.push(line);
			continue;
		}
		const m = line.match(heredocStart);
		if (m) {
			tag = m[1];
			capturing = shellReceivers.has(heredocReceiver(line));
			cur = [];
		}
	}
	return bodies;
}

const runtimePatterns: Array<{ re: RegExp; reason: string }> = [
	{ re: /(^|[;&|\n]\s*)(npm|pnpm|yarn|bun)\s+(run\s+)?(dev|serve|start:dev|watch)\b/u, reason: "package-manager dev/watch command" },
	{ re: /(^|[;&|\n]\s*)(vite|next\s+dev|nuxt\s+dev|astro\s+dev|svelte-kit\s+dev)\b/u, reason: "frontend dev server" },
	{ re: /(^|[;&|\n]\s*)(rails\s+s|rails\s+server|bin\/rails\s+s)\b/u, reason: "Rails server" },
	{ re: /(^|[;&|\n]\s*)python\s+manage\.py\s+runserver\b/u, reason: "Django dev server" },
	{ re: /(^|[;&|\n]\s*)(uvicorn|hypercorn|fastapi\s+dev|flask\s+run)\b/u, reason: "Python web server" },
	{ re: /(^|[;&|\n]\s*)air\b/u, reason: "Go live-reload server" },
	{ re: /(^|[;&|\n]\s*)go\s+run\s+\.\/(cmd\/)?(server|api|web)\b/u, reason: "Go server/API runtime command" },
	{ re: /(^|[;&|\n]\s*)cargo\s+(run|watch)\b/u, reason: "Rust runtime/watch command" },
	{ re: /(^|[;&|\n]\s*)make\s+(dev|serve|server|run|watch|start)\b/u, reason: "make runtime target" },
	{ re: /(^|[;&|\n]\s*)(watchexec|entr|nodemon|ts-node-dev)\b/u, reason: "watch/reload command" },
];

// docker compose up is runtime only in its foreground form. Detached
// (-d/--detach) compose hands the stack to dockerd and returns at once — a
// one-shot that stays safe. Checked per-segment (mirrors guard.go) so a detach
// flag in one segment can't excuse a foreground compose in another.
const DOCKER_COMPOSE_UP_SEG = /^\s*docker\s+compose\s+up\b/u;
const DETACH_FLAG = /(^|\s)(-d|--detach)(\s|$)/u;

// foregroundComposeUp reports whether any segment runs a foreground
// `docker compose up` (no -d/--detach). Mirrors guard.go's foregroundComposeUp.
function foregroundComposeUp(scan: string): boolean {
	for (const seg of scan.split(segSplit)) {
		if (DOCKER_COMPOSE_UP_SEG.test(seg) && !DETACH_FLAG.test(seg)) return true;
	}
	return false;
}

// Single source of truth for the headless-agent print-mode guard: the pattern
// and the remediation string are shared with tools/shared.ts so the two block
// sites can't drift.
export const HEADLESS_AGENT_PRINT_PATTERN = /(^|[;&|\n]\s*)(claude|codex|pi|agy)\b[^\n;&|]*(\s-p\b|\s--print\b)/u;
export const HEADLESS_AGENT_SUGGESTION =
	"Do not launch agent peers with -p/--print. Use zmux_peer_ensure for a visible interactive CLI, then deliver prompts with zmux_type or zmux_peer_handoff.";

const headlessAgentPatterns: Array<{ re: RegExp; reason: string }> = [
	{ re: HEADLESS_AGENT_PRINT_PATTERN, reason: "agent headless/print mode bypasses visible zmux peer flow" },
];

const interactivePatterns: Array<{ re: RegExp; reason: string }> = [
	{ re: /(^|[;&|\n]\s*)sudo\b/u, reason: "sudo requires shared user interaction" },
	{ re: /(^|[;&|\n]\s*)su\b/u, reason: "su requires shared user interaction" },
	{ re: /(^|[;&|\n]\s*)ssh\b/u, reason: "ssh may require interactive auth/control" },
	{ re: /(^|[;&|\n]\s*)(psql|mysql|sqlite3|redis-cli)\b/u, reason: "interactive database shell" },
	{ re: /(^|[;&|\n]\s*)(python|node|irb|pry|iex|ghci)\s*$/u, reason: "interactive REPL" },
];

// hasBackgrounding flags a lone `&` backgrounding operator (excluding `&&`, the
// `>&`/`&>` fd-redirects like `2>&1`, and `|&` bash pipe-both) plus nohup/disown.
// Runs on the quote-stripped scan so a literal `&` inside a string isn't mistaken
// for it.
function hasBackgrounding(command: string): boolean {
	return /(^|\s)(nohup|disown)\b/u.test(command) || /(^|[^&>|])&([^&>]|$)/u.test(command);
}

function suggestionForRuntime(command: string): string {
	return [
		"Use zmux_runtime_ensure instead of bash for software that keeps running.",
		"Example:",
		`  zmux_runtime_ensure({ name: "server", command: ${JSON.stringify(command)} })`,
	].join("\n");
}

function suggestionForInteractive(command: string): string {
	return [
		"Use zmux_interactive_type so the user can see/respond in a shared tab.",
		"Example:",
		`  zmux_interactive_type({ tab: "admin", command: ${JSON.stringify(command)} })`,
	].join("\n");
}

function suggestionForDirectTool(tool: string): string {
	return `Use the typed ${tool} tool instead of shelling out through bash.`;
}

function suggestionForTmux(tool: string): string {
	return `Use ${tool} instead of raw tmux — zmux owns the @zmux_label pin + session/workspace bookkeeping.`;
}

export function stripQuotedSegments(command: string): string {
	let output = "";
	let quote: "'" | '"' | "`" | undefined;
	let escaped = false;
	for (const char of command) {
		if (quote) {
			if (quote === '"' && !escaped && char === "\\") {
				escaped = true;
				output += " ";
				continue;
			}
			if (!escaped && char === quote) {
				quote = undefined;
			} else {
				output += char === "\n" ? "\n" : " ";
			}
			escaped = false;
			continue;
		}
		if (char === "'" || char === '"' || char === "`") {
			quote = char;
			output += " ";
			continue;
		}
		output += char;
	}
	return output;
}

// stripEnvPrefix removes a run of NAME=VALUE assignments (optionally led by
// `env`) at command position so `NODE_ENV=prod npm run dev` still classifies on
// `npm`. Runs on the raw command (before quotes are blanked) so the quote-aware
// value alternation handles values with spaces (`FOO="a b" npm …`). Mirrors
// internal/guard/guard.go's envAssignPrefix.
function stripEnvPrefix(command: string): string {
	return command.replace(/(^|[;&|\n]\s*)(env\s+)?([A-Za-z_][A-Za-z0-9_]*=("[^"]*"|'[^']*'|\S+)\s+)+/gu, "$1");
}

// stripHeredocs blanks the body of any here-document (`cmd <<EOF` … `EOF`) so
// shell metacharacters or a `tmux` inside a literal body aren't read as
// operators/commands. The body is stdin data, never executed, so removing it is
// loss-free. The opening line (the real command) is kept; body + closing
// delimiter lines are blanked. Mirrors internal/guard/guard.go's stripHeredocs.
function stripHeredocs(command: string): string {
	if (!command.includes("<<")) return command;
	const lines = command.split("\n");
	let tag = ""; // non-empty while inside a here-doc body
	for (let i = 0; i < lines.length; i++) {
		if (tag) {
			if (lines[i].trim() === tag) tag = ""; // closing delimiter reached
			lines[i] = ""; // blank body + closing-delimiter lines
			continue;
		}
		const m = lines[i].match(heredocStart);
		if (m) tag = m[1];
	}
	return lines.join("\n");
}

// Payload kinds that represent a blockable slip on the shell surface (matching
// the Go classifier's Block decisions) — used to decide when a recursively
// classified `sh -c …` / here-doc payload should win.
const blockablePayloadKinds = new Set(["direct_tmux", "runtime", "background", "headless_agent"]);

export function classifyBash(command: string, config: PiZmuxConfig, depth = 0): BashClassification {
	const normalized = command.trim();
	if (!normalized) return { kind: "safe", reason: "empty command" };

	// Recursive "executable payload" pass FIRST: a raw tmux or background job
	// hidden inside a `sh -c '…'` arg or a shell-fed here-doc body would be blanked
	// by the quote/heredoc stripping below and escape. Extract those inner commands
	// and classify them; a blockable verdict from any of them wins. (`xargs tmux …`
	// is handled in classifyTmux — its payload isn't quoted.)
	if (depth < maxClassifyDepth) {
		for (const payload of executablePayloads(normalized)) {
			const sub = classifyBash(payload, config, depth + 1);
			if (blockablePayloadKinds.has(sub.kind)) return sub;
		}
	}

	// Pipeline: env-strip (quote-aware) → blank here-doc bodies → blank quoted
	// spans, all before the dimension scans. Env-strip first so `FOO="bar baz" npm
	// run dev` classifies on `npm`; heredoc-strip before quote-strip so a `<<'EOF'`
	// delimiter survives to bound the body, and a literal `&` inside a body or
	// string isn't mistaken for backgrounding.
	const scan = stripQuotedSegments(stripHeredocs(stripEnvPrefix(normalized)));

	if (config.policy.blockBackgroundJobs && hasBackgrounding(scan)) {
		return { kind: "background", reason: "background jobs hide process state from zmux/Pi", suggestion: suggestionForRuntime(normalized) };
	}

	for (const pattern of directZmuxPatterns) {
		if (pattern.re.test(scan)) {
			return { kind: "direct_zmux", reason: pattern.reason, suggestion: suggestionForDirectTool(pattern.tool) };
		}
	}

	// Raw-tmux dimension: a mapped, non-socket invocation in command position
	// wins; socket-scoped/unmapped tmux falls through to the runtime/interactive
	// checks (so `tmux -L s x && npm run dev` still catches the dev server). pi
	// folds socket-scoped tmux into `safe` (kind-derived decision); see the
	// corpus README.
	const tmuxBlock = classifyTmux(scan);
	if (tmuxBlock) return tmuxBlock;

	for (const pattern of headlessAgentPatterns) {
		if (pattern.re.test(scan)) {
			return { kind: "headless_agent", reason: pattern.reason, suggestion: HEADLESS_AGENT_SUGGESTION };
		}
	}

	if (config.policy.redirectInteractive) {
		for (const pattern of interactivePatterns) {
			if (pattern.re.test(scan)) {
				return { kind: "interactive", reason: pattern.reason, suggestion: suggestionForInteractive(normalized) };
			}
		}
	}

	if (foregroundComposeUp(scan)) {
		return { kind: "runtime", reason: "Docker Compose runtime", suggestion: suggestionForRuntime(normalized) };
	}

	for (const pattern of runtimePatterns) {
		if (pattern.re.test(scan)) {
			return { kind: "runtime", reason: pattern.reason, suggestion: suggestionForRuntime(normalized) };
		}
	}

	return { kind: "safe", reason: "no zmux runtime/interactive pattern matched" };
}

export function shouldBlock(classification: BashClassification, config: PiZmuxConfig): boolean {
	return config.policy.mode === "enforce" && classification.kind !== "safe";
}
