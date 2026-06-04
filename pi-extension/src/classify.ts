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
	| { kind: "direct_tmux"; reason: string; suggestion: string };

const directZmuxPatterns: Array<{ re: RegExp; reason: string; tool: string }> = [
	{ re: /(^|[;&|]\s*)zmux\s+tabs\b/u, reason: "tab listing has a typed tool", tool: "zmux_tabs" },
	{ re: /(^|[;&|]\s*)zmux\s+tab\s+kill\b/u, reason: "tab cleanup has a typed tool", tool: "zmux_tab_kill" },
	{ re: /(^|[;&|]\s*)zmux\s+send\b/u, reason: "key sending has a typed tool", tool: "zmux_send_keys" },
	{ re: /(^|[;&|]\s*)zmux\s+type\b/u, reason: "typing has typed tools", tool: "zmux_type or zmux_interactive_type" },
	{ re: /(^|[;&|]\s*)zmux\s+pane\s+list\b/u, reason: "pane listing has a typed tool", tool: "zmux_pane_list" },
	{ re: /(^|[;&|]\s*)zmux\s+pane\s+focus\b/u, reason: "pane focus has a typed tool", tool: "zmux_pane_focus" },
	{ re: /(^|[;&|]\s*)zmux\s+pane\s+close\b/u, reason: "pane cleanup has a typed tool", tool: "zmux_pane_close" },
	{ re: /(^|[;&|]\s*)zmux\s+run\b/u, reason: "runtime management has typed tools", tool: "zmux_runtime_ensure" },
	{ re: /(^|[;&|]\s*)zmux\s+watch\b/u, reason: "runtime logs have a typed tool", tool: "zmux_runtime_logs" },
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
	"list-sessions": { reason: "session listing belongs in zmux", tool: "the zmux CLI `zmux ls` (no typed session tool yet)" },
	ls: { reason: "session listing belongs in zmux", tool: "the zmux CLI `zmux ls` (no typed session tool yet)" },
	"list-panes": { reason: "pane listing has a typed tool", tool: "the zmux_pane_list typed tool" },
	lsp: { reason: "pane listing has a typed tool", tool: "the zmux_pane_list typed tool" },
	"split-window": { reason: "pane splitting belongs in zmux", tool: "the zmux CLI `zmux pane open` (no typed split tool yet)" },
	splitw: { reason: "pane splitting belongs in zmux", tool: "the zmux CLI `zmux pane open` (no typed split tool yet)" },
	"select-pane": { reason: "pane focus has a typed tool", tool: "the zmux_pane_focus typed tool" },
	selectp: { reason: "pane focus has a typed tool", tool: "the zmux_pane_focus typed tool" },
	"kill-pane": { reason: "pane cleanup has a typed tool", tool: "the zmux_pane_close typed tool" },
	killp: { reason: "pane cleanup has a typed tool", tool: "the zmux_pane_close typed tool" },
	"resize-pane": { reason: "pane resize belongs in zmux", tool: "the zmux CLI `zmux pane resize` (no typed resize tool yet)" },
	resizep: { reason: "pane resize belongs in zmux", tool: "the zmux CLI `zmux pane resize` (no typed resize tool yet)" },
	"new-window": { reason: "starting work in a tab has a typed tool", tool: "the zmux_runtime_ensure typed tool" },
	neww: { reason: "starting work in a tab has a typed tool", tool: "the zmux_runtime_ensure typed tool" },
	"kill-window": { reason: "tab cleanup has a typed tool", tool: "the zmux_tab_kill typed tool" },
	killw: { reason: "tab cleanup has a typed tool", tool: "the zmux_tab_kill typed tool" },
	"rename-window": { reason: "tab labelling belongs in zmux", tool: "the zmux CLI `zmux tab label` (no typed rename tool yet)" },
	renamew: { reason: "tab labelling belongs in zmux", tool: "the zmux CLI `zmux tab label` (no typed rename tool yet)" },
	"move-window": { reason: "tab moving belongs in zmux", tool: "the zmux CLI `zmux tab move` (no typed move tool yet)" },
	movew: { reason: "tab moving belongs in zmux", tool: "the zmux CLI `zmux tab move` (no typed move tool yet)" },
	"select-window": { reason: "tab focus has a typed tool", tool: "the zmux_tab_focus typed tool" },
	selectw: { reason: "tab focus has a typed tool", tool: "the zmux_tab_focus typed tool" },
	"new-session": { reason: "session creation belongs in zmux", tool: "the zmux CLI `zmux new` (no typed session tool yet)" },
	new: { reason: "session creation belongs in zmux", tool: "the zmux CLI `zmux new` (no typed session tool yet)" },
	"kill-session": { reason: "session cleanup belongs in zmux", tool: "the zmux CLI `zmux session kill` (no typed session tool yet)" },
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
		if (toks.length === 0 || toks[0] !== "tmux") continue;
		const args = toks.slice(1);
		if (hasSocketFlag(args)) continue; // socket-scoped (zzmux/profile) → exempt
		const redirect = tmuxSubcommandRedirects[tmuxSubcommand(args.join(" "))];
		if (redirect) {
			return { kind: "direct_tmux", reason: redirect.reason, suggestion: suggestionForTmux(redirect.tool) };
		}
		// unmapped subcommand (info, has-session, ...) — no zmux verb; keep scanning
	}
	return null;
}

const runtimePatterns: Array<{ re: RegExp; reason: string }> = [
	{ re: /(^|[;&|]\s*)(npm|pnpm|yarn|bun)\s+(run\s+)?(dev|serve|start:dev|watch)\b/u, reason: "package-manager dev/watch command" },
	{ re: /(^|[;&|]\s*)(vite|next\s+dev|nuxt\s+dev|astro\s+dev|svelte-kit\s+dev)\b/u, reason: "frontend dev server" },
	{ re: /(^|[;&|]\s*)(rails\s+s|rails\s+server|bin\/rails\s+s)\b/u, reason: "Rails server" },
	{ re: /(^|[;&|]\s*)python\s+manage\.py\s+runserver\b/u, reason: "Django dev server" },
	{ re: /(^|[;&|]\s*)(uvicorn|hypercorn|fastapi\s+dev|flask\s+run)\b/u, reason: "Python web server" },
	{ re: /(^|[;&|]\s*)air\b/u, reason: "Go live-reload server" },
	{ re: /(^|[;&|]\s*)go\s+run\s+\.\/(cmd\/)?(server|api|web)\b/u, reason: "Go server/API runtime command" },
	{ re: /(^|[;&|]\s*)cargo\s+(run|watch)\b/u, reason: "Rust runtime/watch command" },
	{ re: /(^|[;&|]\s*)docker\s+compose\s+up\b/u, reason: "Docker Compose runtime" },
	{ re: /(^|[;&|]\s*)make\s+(dev|serve|server|run|watch|start)\b/u, reason: "make runtime target" },
	{ re: /(^|[;&|]\s*)(watchexec|entr|nodemon|ts-node-dev)\b/u, reason: "watch/reload command" },
];

const interactivePatterns: Array<{ re: RegExp; reason: string }> = [
	{ re: /(^|[;&|]\s*)sudo\b/u, reason: "sudo requires shared user interaction" },
	{ re: /(^|[;&|]\s*)su\b/u, reason: "su requires shared user interaction" },
	{ re: /(^|[;&|]\s*)ssh\b/u, reason: "ssh may require interactive auth/control" },
	{ re: /(^|[;&|]\s*)(psql|mysql|sqlite3|redis-cli)\b/u, reason: "interactive database shell" },
	{ re: /(^|[;&|]\s*)(python|node|irb|pry|iex|ghci)\s*$/u, reason: "interactive REPL" },
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
	return command.replace(/(^|[;&|]\s*)(env\s+)?([A-Za-z_][A-Za-z0-9_]*=("[^"]*"|'[^']*'|\S+)\s+)+/gu, "$1");
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

export function classifyBash(command: string, config: PiZmuxConfig): BashClassification {
	const normalized = command.trim();
	if (!normalized) return { kind: "safe", reason: "empty command" };
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

	if (config.policy.redirectInteractive) {
		for (const pattern of interactivePatterns) {
			if (pattern.re.test(scan)) {
				return { kind: "interactive", reason: pattern.reason, suggestion: suggestionForInteractive(normalized) };
			}
		}
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
