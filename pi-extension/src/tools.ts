import { Type } from "typebox";
import type { ExtensionAPI, ExtensionContext } from "@earendil-works/pi-coding-agent";
import { resolve } from "node:path";
import { loadConfig, mergeRuntimeConfig } from "./config.js";
import {
	capabilities,
	closePane,
	currentPane,
	focusPane,
	focusTab,
	interactiveType,
	killTab,
	listPanes,
	listTabs,
	openPane,
	resizePane,
	runtimeEnsure,
	runtimeLogs,
	runtimeStop,
	schedulePiRespawn,
	sendKeys,
	sendPaneKeys,
	typePaneText,
	typeText,
} from "./zmux.js";

const MAX_RESULT_BYTES = 50 * 1024;
const MAX_RESULT_LINES = 2_000;

function truncateText(text: string): { text: string; truncated: boolean; originalBytes: number; originalLines: number } {
	const originalBytes = Buffer.byteLength(text, "utf8");
	const originalLines = text.split("\n").length;
	if (originalBytes <= MAX_RESULT_BYTES && originalLines <= MAX_RESULT_LINES) {
		return { text, truncated: false, originalBytes, originalLines };
	}
	const lines = text.split("\n");
	let selected = lines.slice(Math.max(0, lines.length - MAX_RESULT_LINES)).join("\n");
	while (Buffer.byteLength(selected, "utf8") > MAX_RESULT_BYTES) {
		selected = selected.slice(Math.ceil(selected.length * 0.1));
	}
	return {
		text: `${selected}\n\n[pi-zmux: output truncated to last ${MAX_RESULT_LINES} lines / ${MAX_RESULT_BYTES} bytes; original ${originalLines} lines / ${originalBytes} bytes]`,
		truncated: true,
		originalBytes,
		originalLines,
	};
}

function content(text: string, details: Record<string, unknown> = {}) {
	const truncated = truncateText(text);
	return {
		content: [{ type: "text" as const, text: truncated.text }],
		details: truncated.truncated ? { ...details, truncated: true, originalBytes: truncated.originalBytes, originalLines: truncated.originalLines } : details,
	};
}

function configFor(ctx: ExtensionContext) {
	return loadConfig(ctx.cwd, { projectTrusted: ctx.isProjectTrusted() });
}

function resolveCwd(cwd: string, maybeCwd?: string): string {
	return maybeCwd ? resolve(cwd, maybeCwd) : cwd;
}

function shouldWaitForExit(command: string): boolean {
	const trimmed = command.trim();
	if (/^sudo\s+(-i|-s|su\b)/u.test(trimmed)) return false;
	if (/^(ssh|psql|mysql|sqlite3|redis-cli|python|node|irb|pry|bash|zsh|fish)(\s+.*)?$/u.test(trimmed) && !/^sudo\b/u.test(trimmed)) {
		return false;
	}
	return /(^|[;&|]\s*)(sudo|su)\b/u.test(trimmed);
}

type PaneDirection = "right" | "left" | "down" | "up";

function paneDirection(value?: string): PaneDirection | undefined {
	if (value === undefined || value === "right" || value === "left" || value === "down" || value === "up") return value;
	throw new Error(`direction must be one of: right, left, down, up (got ${value})`);
}

export function registerZmuxTools(pi: ExtensionAPI): void {
	pi.registerTool({
		name: "zmux_current",
		label: "zmux current",
		description: "Inspect current zmux context: pane, tabs, terminal RGB capabilities, binary/profile, project trust, and loaded pi-zmux config. Use before managing persistent runtimes, panes, sidecars, or ambiguous sessions.",
		promptSnippet: "Inspect current zmux context and pi-zmux config",
		parameters: Type.Object({}),
		async execute(_id, _params, _signal, _onUpdate, ctx) {
			const [pane, tabs, caps] = await Promise.all([currentPane(ctx.cwd), listTabs(ctx.cwd), capabilities(ctx.cwd)]);
			const config = configFor(ctx);
			return content([
				`cwd: ${ctx.cwd}`,
				`projectTrusted: ${ctx.isProjectTrusted()}`,
				`zmuxBinary: ${process.env.PI_ZMUX_BIN?.trim() || "zmux"}`,
				`tmuxSocket: ${process.env.PI_ZMUX_TMUX_SOCKET?.trim() || "(default/inferred)"}`,
				`config: ${config.path ?? "(none)"}${config.ignoredReason ? ` ignored=${config.ignoredReason}` : ""}`,
				`policy: ${config.policy.mode}`,
				`pane: ${pane ? JSON.stringify(pane) : "unavailable"}`,
				`tabs:\n${tabs}`,
				`terminal capabilities:\n${caps}`,
			].join("\n"), { pane, tabs, capabilities: caps, config });
		},
	});

	pi.registerTool({
		name: "zmux_reload",
		label: "zmux reload",
		description: "Queue a soft Pi runtime reload via `/zmux reload`. Use after changing Pi extensions, skills, prompts, or themes when a non-destructive reload is enough. This queues a follow-up command because tools cannot call ctx.reload() directly.",
		promptSnippet: "Queue a soft Pi extension/runtime reload",
		promptGuidelines: ["Use zmux_reload after editing Pi extension code before trying hard respawn; it queues `/zmux reload` as a follow-up command."],
		parameters: Type.Object({}),
		async execute() {
			pi.sendUserMessage("/zmux reload", { deliverAs: "followUp" });
			return content("Queued /zmux reload as a follow-up command.", { queued: true });
		},
	});

	pi.registerTool({
		name: "zmux_tabs",
		label: "zmux tabs",
		description: "List tabs/windows in the current or targeted zmux session. Prefer this over running `zmux tabs` through bash.",
		promptSnippet: "List zmux tabs in a session",
		parameters: Type.Object({
			session: Type.Optional(Type.String({ description: "Optional zmux session target (`-s`), e.g. workspace/session or raw session" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const tabs = await listTabs(ctx.cwd, params.session);
			return content(tabs, { tabs, session: params.session });
		},
	});

	pi.registerTool({
		name: "zmux_tab_kill",
		label: "zmux tab kill",
		description: "Kill a zmux tab/window by name. Use for intentional tab cleanup instead of shelling out to `zmux tab kill` in bash.",
		promptSnippet: "Kill a zmux tab/window",
		parameters: Type.Object({
			tab: Type.String({ description: "Tab/window name to kill in the current session" }),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await killTab(params.tab, resolveCwd(ctx.cwd, params.cwd));
			return content(result.text, result.details);
		},
	});

	pi.registerTool({
		name: "zmux_tab_focus",
		label: "zmux tab focus",
		description: "Focus a zmux tab/window by name. Ask the user before using this in agent sessions because it moves terminal focus.",
		promptSnippet: "Focus a zmux tab/window",
		promptGuidelines: ["Ask the user before calling zmux_tab_focus because it moves terminal focus."],
		parameters: Type.Object({
			tab: Type.String({ description: "Tab/window name to focus" }),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await focusTab(params.tab, resolveCwd(ctx.cwd, params.cwd));
			return content(result.text, result.details);
		},
	});

	pi.registerTool({
		name: "zmux_send_keys",
		label: "zmux send keys",
		description: "Send raw keys such as C-c, Enter, Escape, or arrows to a zmux tab. Prefer this over `zmux send` in bash.",
		promptSnippet: "Send raw keys to a zmux tab",
		parameters: Type.Object({
			tab: Type.String({ description: "Target tab/window" }),
			keys: Type.Array(Type.String(), { description: "Raw keys to send, e.g. C-c, Enter, Escape" }),
			session: Type.Optional(Type.String({ description: "Optional zmux session target (`-s`)" })),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await sendKeys(params.tab, params.keys, resolveCwd(ctx.cwd, params.cwd), params.session);
			return content(result.text, result.details);
		},
	});

	pi.registerTool({
		name: "zmux_type",
		label: "zmux type",
		description: "Type text plus Enter into an existing zmux tab. For sudo/password/manual-input commands, prefer zmux_interactive_type.",
		promptSnippet: "Type text into an existing zmux tab",
		parameters: Type.Object({
			tab: Type.String({ description: "Target tab/window" }),
			text: Type.String({ description: "Text to type and submit" }),
			session: Type.Optional(Type.String({ description: "Optional zmux session target (`-s`)" })),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await typeText(params.tab, params.text, resolveCwd(ctx.cwd, params.cwd), params.session);
			return content(result.text, result.details);
		},
	});

	pi.registerTool({
		name: "zmux_pane_send_keys",
		label: "zmux pane send keys",
		description: "Send raw keys to a specific tmux pane id/title/index. Use sparingly for sidecar panes; prefer tab-level zmux_send_keys when a logical tab name exists.",
		promptSnippet: "Send raw keys to a specific pane",
		parameters: Type.Object({
			pane: Type.String({ description: "Target pane id/title/index, e.g. %347" }),
			keys: Type.Array(Type.String(), { description: "Raw keys to send, e.g. C-c, Enter, Escape, l" }),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await sendPaneKeys(params.pane, params.keys, resolveCwd(ctx.cwd, params.cwd));
			return content(result.text, result.details);
		},
	});

	pi.registerTool({
		name: "zmux_pane_type",
		label: "zmux pane type",
		description: "Type text plus Enter into a specific tmux pane id/title/index. Use sparingly for sidecar panes; prefer tab-level zmux_type when a logical tab name exists.",
		promptSnippet: "Type text into a specific pane",
		parameters: Type.Object({
			pane: Type.String({ description: "Target pane id/title/index, e.g. %347" }),
			text: Type.String({ description: "Text to type and submit" }),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await typePaneText(params.pane, params.text, resolveCwd(ctx.cwd, params.cwd));
			return content(result.text, result.details);
		},
	});

	pi.registerTool({
		name: "zmux_pane_list",
		label: "zmux pane list",
		description: "List panes in the current window or session. Prefer this over `zmux pane list` in bash.",
		promptSnippet: "List panes in zmux",
		parameters: Type.Object({
			session: Type.Optional(Type.String({ description: "Optional session target; lists joined/session panes when supported" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const panes = await listPanes(ctx.cwd, params.session);
			return content(panes, { panes, session: params.session });
		},
	});

	pi.registerTool({
		name: "zmux_pane_open",
		label: "zmux pane open",
		description: "Open a named sidecar pane using `zmux pane open`. Use for visible sidecars or terminal UI helpers instead of raw tmux split-window.",
		promptSnippet: "Open a named zmux sidecar pane",
		parameters: Type.Object({
			name: Type.String({ description: "Pane title/name" }),
			command: Type.String({ description: "Command to run in the pane" }),
			direction: Type.Optional(Type.String({ description: "Split direction: right, left, down, or up; default right" })),
			size: Type.Optional(Type.String({ description: "Pane size, e.g. 35% or 80 cells" })),
			target: Type.Optional(Type.String({ description: "Target pane/window; defaults to current" })),
			labelTab: Type.Optional(Type.Boolean({ description: "Preserve current tab name as a zmux label before opening pane" })),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const cwd = resolveCwd(ctx.cwd, params.cwd);
			const result = await openPane({
				name: params.name,
				command: params.command,
				cwd,
				direction: paneDirection(params.direction),
				size: params.size,
				target: params.target,
				labelTab: params.labelTab,
			});
			return content(result.text, result.details);
		},
	});

	pi.registerTool({
		name: "zmux_pane_focus",
		label: "zmux pane focus",
		description: "Focus a zmux pane by id/title/index. Prefer this over `zmux pane focus` in bash.",
		promptSnippet: "Focus a zmux pane",
		parameters: Type.Object({
			pane: Type.String({ description: "Pane id/title/index to focus" }),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await focusPane(params.pane, resolveCwd(ctx.cwd, params.cwd));
			return content(result.text, result.details);
		},
	});

	pi.registerTool({
		name: "zmux_pane_close",
		label: "zmux pane close",
		description: "Close a zmux pane by id/title/index. Use for intentional pane cleanup instead of shelling out to `zmux pane close` in bash.",
		promptSnippet: "Close a zmux pane",
		parameters: Type.Object({
			pane: Type.String({ description: "Pane id/title/index to close" }),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await closePane(params.pane, resolveCwd(ctx.cwd, params.cwd));
			return content(result.text, result.details);
		},
	});

	pi.registerTool({
		name: "zmux_pane_resize",
		label: "zmux pane resize",
		description: "Resize a zmux pane by id/title/index. Use for intentional pane layout control instead of raw tmux resize-pane.",
		promptSnippet: "Resize a zmux pane",
		parameters: Type.Object({
			pane: Type.String({ description: "Pane id/title/index to resize" }),
			size: Type.String({ description: "New pane size, e.g. 40% or 80" }),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await resizePane(params.pane, resolveCwd(ctx.cwd, params.cwd), params.size);
			return content(result.text, result.details);
		},
	});

	pi.registerTool({
		name: "zmux_pi_respawn",
		label: "zmux Pi respawn",
		description: "Hard-restart the current Pi agent pane by respawning it with `pi -c`. Use only when soft `/zmux reload` is unavailable or Pi is wedged; this kills the current pane process and discards unsent input. If autonomous follow-up is expected, pass continuationPrompt.",
		promptSnippet: "Hard-restart/respawn the current Pi pane",
		promptGuidelines: [
			"Prefer zmux_reload / `/zmux reload` after changing Pi extensions or tools; use zmux_pi_respawn only as a hard fallback.",
			"If work should continue after respawn, pass continuationPrompt with the exact next smoke/validation steps.",
			"Do not use if the user may have unsent input or manual validation is in progress; explain that it hard-restarts the current Pi pane.",
		],
		parameters: Type.Object({
			paneId: Type.Optional(Type.String({ description: "Target tmux pane id; defaults to current Pi pane" })),
			command: Type.Optional(Type.String({ description: "Restart command; defaults to `pi -c`. Cannot be combined with continuationPrompt." })),
			continuationPrompt: Type.Optional(Type.String({ description: "Optional handoff prompt for the restarted Pi process." })),
			delayMs: Type.Optional(Type.Number({ description: "Delay before respawning the pane; default 300ms" })),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await schedulePiRespawn({
				cwd: resolveCwd(ctx.cwd, params.cwd),
				paneId: params.paneId,
				command: params.command,
				continuationPrompt: params.continuationPrompt,
				delayMs: params.delayMs,
			});
			return content(result.text, result.details);
		},
	});

	pi.registerTool({
		name: "zmux_runtime_ensure",
		label: "zmux runtime ensure",
		description: "Ensure software under development is running in a stable named zmux tab. Use for dev servers, API services, workers, watch processes, TUI demos, and any persistent runtime instead of running them through bash.",
		promptSnippet: "Ensure a persistent runtime is running in a named zmux tab",
		promptGuidelines: [
			"Use zmux_runtime_ensure for any command expected to keep running, serve traffic, watch files, or provide logs.",
			"Check zmux_runtime_logs rather than starting duplicate runtimes when debugging.",
		],
		parameters: Type.Object({
			name: Type.String({ description: "Logical runtime name, e.g. server, api, worker, test-watch" }),
			command: Type.Optional(Type.String({ description: "Command to run; optional when configured in trusted .pi/zmux.json" })),
			tab: Type.Optional(Type.String({ description: "zmux tab name; defaults to configured tab or name" })),
			session: Type.Optional(Type.String({ description: "Optional zmux session target (`-s`)" })),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
			readiness: Type.Optional(Type.String({ description: "Regex to wait for in new output, e.g. ready|listening|localhost" })),
			timeoutSeconds: Type.Optional(Type.Number({ description: "Readiness timeout seconds" })),
			restart: Type.Optional(Type.Boolean({ description: "Send C-c to the tab before ensuring" })),
			kind: Type.Optional(Type.String({ description: "Runtime kind for metadata: server, worker, watcher, tui, etc." })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const config = configFor(ctx);
			const runtime = mergeRuntimeConfig(params.name, params, config);
			if (!runtime.command) {
				return content(`ERROR: runtime ${params.name} has no command. Pass command or add it to trusted .pi/zmux.json / .config/pi-zmux.json.`, { name: params.name, configPath: config.path, ignoredReason: config.ignoredReason });
			}
			const result = await runtimeEnsure({
				tab: runtime.tab,
				command: runtime.command,
				cwd: resolveCwd(ctx.cwd, runtime.cwd),
				readiness: runtime.readiness,
				timeoutSeconds: runtime.timeoutSeconds,
				restart: params.restart,
				session: runtime.session,
			});
			return content(result.text, { ...result.details, name: params.name, kind: runtime.kind, configPath: config.path, ignoredReason: config.ignoredReason });
		},
	});

	pi.registerTool({
		name: "zmux_runtime_logs",
		label: "zmux runtime logs",
		description: "Read logs/output from a named zmux runtime tab. Use when debugging software that should already be running instead of starting a duplicate process.",
		promptSnippet: "Read output from a named zmux runtime tab",
		parameters: Type.Object({
			name: Type.String({ description: "Runtime name or tab" }),
			tab: Type.Optional(Type.String({ description: "Explicit zmux tab name" })),
			session: Type.Optional(Type.String({ description: "Optional zmux session target (`-s`)" })),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
			lines: Type.Optional(Type.Number({ description: "Lines to capture; default 120" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const config = configFor(ctx);
			const runtime = mergeRuntimeConfig(params.name, { tab: params.tab, cwd: params.cwd, session: params.session }, config);
			const result = await runtimeLogs(runtime.tab, resolveCwd(ctx.cwd, runtime.cwd), params.lines ?? 120, runtime.session);
			return content(result.text, { ...result.details, name: params.name, configPath: config.path, ignoredReason: config.ignoredReason });
		},
	});

	pi.registerTool({
		name: "zmux_runtime_stop",
		label: "zmux runtime stop",
		description: "Stop a named zmux runtime tab by sending C-c. Use for visible, controlled shutdown of dev servers, watchers, workers, or demos.",
		promptSnippet: "Stop a named zmux runtime tab",
		parameters: Type.Object({
			name: Type.String({ description: "Runtime name or tab" }),
			tab: Type.Optional(Type.String({ description: "Explicit zmux tab name" })),
			session: Type.Optional(Type.String({ description: "Optional zmux session target (`-s`)" })),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const config = configFor(ctx);
			const runtime = mergeRuntimeConfig(params.name, { tab: params.tab, cwd: params.cwd, session: params.session }, config);
			const result = await runtimeStop(runtime.tab, resolveCwd(ctx.cwd, runtime.cwd), runtime.session);
			return content(result.text, { ...result.details, name: params.name, configPath: config.path, ignoredReason: config.ignoredReason });
		},
	});

	pi.registerTool({
		name: "zmux_interactive_type",
		label: "zmux interactive type",
		description: "Type an interactive or privileged command into a shared zmux tab. Use for sudo, ssh, REPLs, database shells, or commands needing user input instead of running them in bash.",
		promptSnippet: "Type an interactive command into a shared zmux tab",
		promptGuidelines: ["Use zmux_interactive_type for sudo/password/manual-input commands. For one-shot commands that should return output, waitForExit defaults on; tell the user which tab needs attention while the tool waits."],
		parameters: Type.Object({
			command: Type.String({ description: "Command text to type and submit" }),
			tab: Type.Optional(Type.String({ description: "Tab name; defaults to admin" })),
			session: Type.Optional(Type.String({ description: "Optional zmux session target (`-s`)" })),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
			waitFor: Type.Optional(Type.String({ description: "Advanced: regex to wait for instead of waiting for command exit" })),
			waitForExit: Type.Optional(Type.Boolean({ description: "Wrap command with a status-file completion script and wait for exit; defaults true for sudo/manual one-shots, false for long interactive shells" })),
			timeoutSeconds: Type.Optional(Type.Number({ description: "Wait timeout seconds; default 90" })),
			lines: Type.Optional(Type.Number({ description: "Lines to return from the tab while waiting; default 160" })),
			focus: Type.Optional(Type.Boolean({ description: "Focus the tab after creating/typing; default false. Ask the user before using this for agent sessions." })),
		}),
		async execute(_id, params, _signal, onUpdate, ctx) {
			const tab = params.tab || "admin";
			const waitForExit = params.waitForExit ?? shouldWaitForExit(params.command);
			if (waitForExit || params.waitFor) {
				onUpdate?.({
					content: [{ type: "text", text: `Typed command into ${tab}. Waiting up to ${params.timeoutSeconds ?? 90}s for user input and command completion...` }],
					details: { tab, waiting: true, session: params.session },
				});
			}
			const result = await interactiveType(tab, params.command, resolveCwd(ctx.cwd, params.cwd), {
				waitFor: params.waitFor,
				waitForExit,
				timeoutSeconds: params.timeoutSeconds,
				lines: params.lines,
				focus: params.focus,
				session: params.session,
			});
			const note = waitForExit || params.waitFor ? "" : `\nTell the user to complete any prompts in tab ${tab}.`;
			return content(`${result.text}${note}`, result.details);
		},
	});
}
