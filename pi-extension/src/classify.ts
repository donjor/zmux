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

const directTmuxPatterns: Array<{ re: RegExp; reason: string; tool: string }> = [
	{ re: /(^|[;&|]\s*)tmux\s+send-keys\b/u, reason: "pane key sending has a typed tool", tool: "zmux_pane_send_keys or zmux_pane_type" },
	{ re: /(^|[;&|]\s*)tmux\s+kill-pane\b/u, reason: "pane cleanup has a typed tool", tool: "zmux_pane_close" },
	{ re: /(^|[;&|]\s*)tmux\s+select-pane\b/u, reason: "pane focus has a typed tool", tool: "zmux_pane_focus" },
];

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

function hasBackgrounding(command: string): boolean {
	return /(^|\s)(nohup|disown)\b/u.test(command) || /(^|[^&])&\s*($|[;#])/u.test(command);
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

function stripEnvPrefix(command: string): string {
	return command
		.replace(/(^|[;&|]\s*)env\s+(?:[A-Za-z_][A-Za-z0-9_]*=\S+\s+)+/gu, "$1")
		.replace(/(^|[;&|]\s*)(?:[A-Za-z_][A-Za-z0-9_]*=\S+\s+)+(?=[A-Za-z0-9_./-])/gu, "$1");
}

export function classifyBash(command: string, config: PiZmuxConfig): BashClassification {
	const normalized = command.trim();
	const scan = stripEnvPrefix(stripQuotedSegments(normalized));
	if (!normalized) return { kind: "safe", reason: "empty command" };

	if (config.policy.blockBackgroundJobs && hasBackgrounding(normalized)) {
		return { kind: "background", reason: "background jobs hide process state from zmux/Pi", suggestion: suggestionForRuntime(normalized) };
	}

	for (const pattern of directZmuxPatterns) {
		if (pattern.re.test(scan)) {
			return { kind: "direct_zmux", reason: pattern.reason, suggestion: suggestionForDirectTool(pattern.tool) };
		}
	}

	for (const pattern of directTmuxPatterns) {
		if (pattern.re.test(scan)) {
			return { kind: "direct_tmux", reason: pattern.reason, suggestion: suggestionForDirectTool(pattern.tool) };
		}
	}

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
