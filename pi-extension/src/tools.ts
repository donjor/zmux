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
	labelTab,
	listPanes,
	listSessions,
	listTabs,
	logCommand,
	moveTab,
	openPane,
	placeTab,
	reloadZmux,
	resizePane,
	runCommand,
	runtimeEnsure,
	runtimeLogs,
	runtimeStop,
	schedulePiReload,
	schedulePiRespawn,
	sendKeys,
	sendPaneKeys,
	sessionKill,
	sessionRun,
	setTabState,
	snapshot,
	terminalCurrent,
	typePaneText,
	typeText,
	type TabPlacementAction,
	type TabPlacementDirection,
	type TabStateAction,
	type LogAction,
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

function tabStateAction(value: string): TabStateAction {
	if (value === "attention" || value === "running" || value === "done" || value === "failed" || value === "clear") return value;
	throw new Error(`state must be one of: attention, running, done, failed, clear (got ${value})`);
}

function tabPlacementAction(value: string): TabPlacementAction {
	if (value === "pane" || value === "full" || value === "hide" || value === "show") return value;
	throw new Error(`action must be one of: pane, full, hide, show (got ${value})`);
}

function tabPlacementDirection(value?: string): TabPlacementDirection | undefined {
	if (value === undefined || value === "right" || value === "left" || value === "up" || value === "down") return value;
	throw new Error(`direction must be one of: right, left, up, down (got ${value})`);
}

function logAction(value: string): LogAction {
	if (value === "start" || value === "tail" || value === "status" || value === "stop") return value;
	throw new Error(`action must be one of: start, tail, status, stop (got ${value})`);
}

function invalidOption(action: string, option: string): Error {
	return new Error(`${option} is not valid for ${action}`);
}

function validateLogParams(action: LogAction, params: { tab?: string; session?: string; ansi?: boolean; maxBytes?: number; lines?: number }): void {
	if (action !== "status" && !params.tab) throw new Error("tab is required for zmux_log start/tail/stop");
	if (action === "status" && params.tab) throw invalidOption("zmux_log status", "tab");
	if (action === "status" && params.session) throw invalidOption("zmux_log status", "session");
	if (action !== "start" && params.ansi === true) throw invalidOption(`zmux_log ${action}`, "ansi");
	if (action !== "start" && params.maxBytes !== undefined) throw invalidOption(`zmux_log ${action}`, "maxBytes");
	if (action !== "tail" && params.lines !== undefined) throw invalidOption(`zmux_log ${action}`, "lines");
}

function validateTabPlacementParams(action: TabPlacementAction, params: { tab?: string; into?: string; direction?: string; size?: string; pane?: string; after?: boolean }): void {
	if (params.tab && params.pane) throw new Error("tab and pane cannot be combined");
	if (action === "pane" && !params.tab) throw new Error("tab is required for tab pane");
	if (action === "show" && !params.tab && !params.pane) throw new Error("tab or pane is required for tab show");
	if (action === "pane" && params.pane) throw invalidOption("tab pane", "pane");
	if (action !== "pane" && params.into) throw invalidOption(`tab ${action}`, "into");
	if (action !== "pane" && params.direction) throw invalidOption(`tab ${action}`, "direction");
	if (action !== "pane" && params.size) throw invalidOption(`tab ${action}`, "size");
	if (action !== "full" && params.after === true) throw invalidOption(`tab ${action}`, "after");
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
		description: "Reload zmux's own tmux configuration via `zmux reload`. This is for zmux config/key/theme changes, not Pi runtime reload.",
		promptSnippet: "Reload zmux configuration",
		promptGuidelines: ["Use zmux_reload only for zmux config/key/theme changes; use zmux_pi_reload to reload the current Pi runtime after Pi extension, skill, prompt, or theme changes."],
		parameters: Type.Object({}),
		async execute(_id, _params, _signal, _onUpdate, ctx) {
			const result = await reloadZmux(ctx.cwd);
			return content(result.text, result.details);
		},
	});

	pi.registerTool({
		name: "zmux_pi_reload",
		label: "zmux Pi reload",
		description: "Soft-reload the current Pi runtime by using zmux/tmux to type Pi's built-in `/reload` into the current Pi pane, then nudge the agent after reload. Use after changing Pi extensions, skills, prompts, or themes when a non-destructive Pi reload is enough.",
		promptSnippet: "Reload the current Pi runtime via zmux",
		promptGuidelines: [
			"Use zmux_pi_reload after editing Pi extension code, skills, prompts, or themes before trying hard respawn.",
			"Do not use zmux_pi_reload if the user may have unsent input in the Pi editor; it types `/reload` into the current Pi pane.",
		],
		parameters: Type.Object({
			paneId: Type.Optional(Type.String({ description: "Target tmux pane id; defaults to the current Pi pane" })),
			continuationPrompt: Type.Optional(Type.String({ description: "Prompt to inject after reload so the agent resumes. Defaults to a generic reload-continuation nudge." })),
			delayMs: Type.Optional(Type.Number({ description: "Delay before typing /reload; default 5000ms so the current assistant response can finish" })),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await schedulePiReload({
				cwd: resolveCwd(ctx.cwd, params.cwd),
				paneId: params.paneId,
				continuationPrompt: params.continuationPrompt,
				delayMs: params.delayMs,
			});
			return content(result.text, result.details);
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
		name: "zmux_run",
		label: "zmux run",
		description: "Run a reviewable command in a stable named zmux tab via native `zmux run`. Use for command-in-tab one-shots that a human may want to inspect or re-run; keep ordinary bounded checks in bash and persistent servers in zmux_runtime_ensure.",
		promptSnippet: "Run a command in a named zmux tab",
		promptGuidelines: [
			"Use zmux_run for reviewable command-in-tab one-shots; use normal bash for bounded checks whose captured stdout is enough.",
			"Use zmux_runtime_ensure for software that keeps running, and zmux_interactive_type for sudo/password/manual-input commands.",
			"Do not add your own sentinels or wrapper scripts; native zmux run owns completion tracking.",
		],
		parameters: Type.Object({
			command: Type.String({ description: "Command to run" }),
			tab: Type.Optional(Type.String({ description: "Stable zmux tab name (`-n`). Defaults to zmux's command-derived name." })),
			session: Type.Optional(Type.String({ description: "Optional zmux session target (`-s`)" })),
			cwd: Type.Optional(Type.String({ description: "Working directory for the zmux CLI process; defaults to Pi cwd" })),
			timeoutSeconds: Type.Optional(Type.Number({ description: "Wait timeout seconds for non-detached runs; default 120" })),
			lines: Type.Optional(Type.Number({ description: "Lines to capture while waiting/following; default is zmux's default" })),
			detach: Type.Optional(Type.Boolean({ description: "Run detached (`-d`). For persistent servers prefer zmux_runtime_ensure." })),
			follow: Type.Optional(Type.Boolean({ description: "Follow output (`-f`) until timeout/interruption. Usually prefer zmux_runtime_logs for later reads." })),
			keep: Type.Optional(Type.Boolean({ description: "Exempt this tab from auto-reaping (`--keep`)" })),
			scope: Type.Optional(Type.String({ description: "Lifecycle scope, e.g. task or daemon" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await runCommand({
				command: params.command,
				tab: params.tab,
				session: params.session,
				cwd: resolveCwd(ctx.cwd, params.cwd),
				timeoutSeconds: params.timeoutSeconds,
				lines: params.lines,
				detach: params.detach,
				follow: params.follow,
				keep: params.keep,
				scope: params.scope,
			});
			return content(result.text, result.details);
		},
	});

	pi.registerTool({
		name: "zmux_sessions",
		label: "zmux sessions",
		description: "List zmux workspaces/sessions. Prefer this over shelling out to `zmux ls` in Pi.",
		promptSnippet: "List zmux sessions/workspaces",
		parameters: Type.Object({
			workspace: Type.Optional(Type.String({ description: "Optional workspace to list sessions within" })),
			flat: Type.Optional(Type.Boolean({ description: "Use `zmux ls -s` flat session listing" })),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await listSessions(resolveCwd(ctx.cwd, params.cwd), { workspace: params.workspace, flat: params.flat });
			return content(result.text, result.details);
		},
	});

	pi.registerTool({
		name: "zmux_session_run",
		label: "zmux session run",
		description: "Create a detached zmux session and run a command as its first tab. Use for focus-safe peer/worker session birth instead of `zmux new`, which attaches and creates a blank shell tab.",
		promptSnippet: "Create a detached zmux session with a first command tab",
		parameters: Type.Object({
			sessionName: Type.String({ description: "Session label/name to create" }),
			tab: Type.String({ description: "First tab name" }),
			command: Type.String({ description: "Command to run in the first tab" }),
			workspace: Type.Optional(Type.String({ description: "Workspace to tag the session into" })),
			commandCwd: Type.Optional(Type.String({ description: "Working directory for the command inside the new session (`--cwd`)" })),
			cwd: Type.Optional(Type.String({ description: "Working directory for invoking zmux; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await sessionRun({
				sessionName: params.sessionName,
				tab: params.tab,
				command: params.command,
				workspace: params.workspace,
				commandCwd: params.commandCwd ? resolveCwd(ctx.cwd, params.commandCwd) : undefined,
				cwd: resolveCwd(ctx.cwd, params.cwd),
			});
			return content(result.text, result.details);
		},
	});

	pi.registerTool({
		name: "zmux_session_kill",
		label: "zmux session kill",
		description: "Kill a zmux session explicitly. Use for intentional worker/session cleanup after work is integrated.",
		promptSnippet: "Kill a zmux session",
		parameters: Type.Object({
			sessionName: Type.String({ description: "Session to kill" }),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await sessionKill(params.sessionName, resolveCwd(ctx.cwd, params.cwd));
			return content(result.text, result.details);
		},
	});

	pi.registerTool({
		name: "zmux_tab_state",
		label: "zmux tab state",
		description: "Set or clear a zmux tab lifecycle glyph (attention/running/done/failed/clear). Use for peer/worker handoffs and human-visible status instead of shelling out to `zmux tab state`.",
		promptSnippet: "Set a zmux tab lifecycle state",
		parameters: Type.Object({
			state: Type.String({ description: "attention, running, done, failed, or clear" }),
			tab: Type.Optional(Type.String({ description: "Tab name target; omitted means current pane" })),
			target: Type.Optional(Type.String({ description: "Raw pane/window/tab target; overrides tab" })),
			session: Type.Optional(Type.String({ description: "Session for tab-name targets (`-s`)" })),
			source: Type.Optional(Type.String({ description: "State source label" })),
			message: Type.Optional(Type.String({ description: "Display-only message for attention/failed states" })),
			ifState: Type.Optional(Type.String({ description: "For clear: only clear if current state matches" })),
			byVisibility: Type.Optional(Type.Boolean({ description: "For done: store attention instead when pane is not visible" })),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await setTabState({
				action: tabStateAction(params.state),
				tab: params.tab,
				target: params.target,
				session: params.session,
				source: params.source,
				msg: params.message,
				ifState: params.ifState,
				byVisibility: params.byVisibility,
				cwd: resolveCwd(ctx.cwd, params.cwd),
			});
			return content(result.text, result.details);
		},
	});

	pi.registerTool({
		name: "zmux_tab_label",
		label: "zmux tab label",
		description: "Set or clear a stable zmux label for the current/targeted tab. Use for intentional tab identity, not routine output state.",
		promptSnippet: "Set or clear a zmux tab label",
		parameters: Type.Object({
			label: Type.Optional(Type.String({ description: "Label to set; empty or clear=true clears" })),
			target: Type.Optional(Type.String({ description: "Target tmux window/pane; defaults to current" })),
			clear: Type.Optional(Type.Boolean({ description: "Clear the label" })),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await labelTab({ label: params.label, target: params.target, clear: params.clear, cwd: resolveCwd(ctx.cwd, params.cwd) });
			return content(result.text, result.details);
		},
	});

	pi.registerTool({
		name: "zmux_tab_move",
		label: "zmux tab move",
		description: "Move a full zmux tab to another session. Use mainly to recover tabs spawned in the wrong session or intentional session cleanup.",
		promptSnippet: "Move a zmux tab to another session",
		parameters: Type.Object({
			tab: Type.String({ description: "Tab to move" }),
			destination: Type.String({ description: "Destination session target" }),
			force: Type.Optional(Type.Boolean({ description: "Allow cross-workspace move" })),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await moveTab({ tab: params.tab, destination: params.destination, force: params.force, cwd: resolveCwd(ctx.cwd, params.cwd) });
			return content(result.text, result.details);
		},
	});

	pi.registerTool({
		name: "zmux_log",
		label: "zmux log",
		description: "Manage persistent bounded output recording for a zmux tab (`start`, `tail`, `status`, `stop`). Use for logs that should survive detach or pane-buffer truncation; use zmux_runtime_logs for quick live-buffer reads.",
		promptSnippet: "Start/tail/status/stop zmux tab output recording",
		parameters: Type.Object({
			action: Type.String({ description: "start, tail, status, or stop" }),
			tab: Type.Optional(Type.String({ description: "Tab target for start/tail/stop" })),
			session: Type.Optional(Type.String({ description: "Optional session target for tab actions (`-s`)" })),
			ansi: Type.Optional(Type.Boolean({ description: "Keep ANSI escapes when starting a log" })),
			maxBytes: Type.Optional(Type.Number({ description: "Maximum bytes for a bounded log" })),
			lines: Type.Optional(Type.Number({ description: "Lines for tail" })),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const action = logAction(params.action);
			validateLogParams(action, params);
			const result = await logCommand({ action, tab: params.tab, session: params.session, ansi: params.ansi, maxBytes: params.maxBytes, lines: params.lines, cwd: resolveCwd(ctx.cwd, params.cwd) });
			return content(result.text, result.details);
		},
	});

	pi.registerTool({
		name: "zmux_snapshot",
		label: "zmux snapshot",
		description: "Capture terminal/TUI evidence with `zmux snapshot` (text/ANSI plus optional PNG). Use when terminal visual state matters; output usually points to artifact files rather than inlining everything.",
		promptSnippet: "Capture a zmux terminal snapshot evidence bundle",
		parameters: Type.Object({
			noPng: Type.Optional(Type.Boolean({ description: "Capture text/ANSI only" })),
			panes: Type.Optional(Type.Array(Type.String(), { description: "Specific pane ids to capture" })),
			lines: Type.Optional(Type.Number({ description: "Scrollback lines to capture" })),
			out: Type.Optional(Type.String({ description: "Output directory" })),
			json: Type.Optional(Type.Boolean({ description: "Print JSON result" })),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await snapshot({ noPng: params.noPng, panes: params.panes, lines: params.lines, out: params.out, json: params.json, cwd: resolveCwd(ctx.cwd, params.cwd) });
			return content(result.text, result.details);
		},
	});

	pi.registerTool({
		name: "zmux_tab_place",
		label: "zmux tab place",
		description: "Move logical tabs between full, pane, hidden, and shown placements. Use instead of shelling out to `zmux tab pane/full/hide/show` when managing tab layout from Pi.",
		promptSnippet: "Place a zmux logical tab as pane/full/hidden/shown",
		parameters: Type.Object({
			action: Type.String({ description: "pane, full, hide, or show" }),
			tab: Type.Optional(Type.String({ description: "Tab name/index target; required for pane and show, optional for full/hide current-pane flows" })),
			session: Type.Optional(Type.String({ description: "Session for tab-name targets" })),
			into: Type.Optional(Type.String({ description: "Host tab for pane action" })),
			direction: Type.Optional(Type.String({ description: "Pane direction: right, left, up, or down" })),
			size: Type.Optional(Type.String({ description: "Pane size, e.g. 40% or 80" })),
			pane: Type.Optional(Type.String({ description: "Raw pane id for mouse/menu-style targets" })),
			after: Type.Optional(Type.Boolean({ description: "For full: insert after old host" })),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const action = tabPlacementAction(params.action);
			validateTabPlacementParams(action, params);
			const result = await placeTab({
				action,
				tab: params.tab,
				session: params.session,
				into: params.into,
				direction: tabPlacementDirection(params.direction),
				size: params.size,
				pane: params.pane,
				after: params.after,
				cwd: resolveCwd(ctx.cwd, params.cwd),
			});
			return content(result.text, result.details);
		},
	});

	pi.registerTool({
		name: "zmux_terminal_current",
		label: "zmux terminal current",
		description: "Resolve the visible desktop terminal target as JSON. Use for terminal/screenshot diagnostics; do not run disruptive terminal refreshes unless asked.",
		promptSnippet: "Resolve current zmux terminal target",
		parameters: Type.Object({
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await terminalCurrent(resolveCwd(ctx.cwd, params.cwd));
			return content(result.text, result.details);
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
		description: "Hard-restart the current Pi agent pane by respawning it with `pi -c`. Use only when soft Pi `/reload` via zmux_pi_reload is unavailable or Pi is wedged; this kills the current pane process and discards unsent input. If autonomous follow-up is expected, pass continuationPrompt.",
		promptSnippet: "Hard-restart/respawn the current Pi pane",
		promptGuidelines: [
			"Prefer zmux_pi_reload after changing Pi extensions or tools; use zmux_pi_respawn only as a hard fallback.",
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
