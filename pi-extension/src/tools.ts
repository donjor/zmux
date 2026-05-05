import { Type } from "@mariozechner/pi-ai";
import { defineTool, type ExtensionAPI } from "@mariozechner/pi-coding-agent";
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
	runtimeEnsure,
	runtimeLogs,
	runtimeStop,
	schedulePiRespawn,
	sendKeys,
	sendPaneKeys,
	typePaneText,
	typeText,
} from "./zmux.js";

function content(text: string, details: Record<string, unknown> = {}) {
	return { content: [{ type: "text" as const, text }], details };
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

export function registerZmuxTools(pi: ExtensionAPI): void {
	pi.registerTool(defineTool({
		name: "zmux_current",
		label: "zmux current",
		description: "Inspect current zmux/tmux context: pane, tabs, terminal RGB capabilities, and loaded pi-zmux config. Use before managing persistent runtimes, panes, sidecars, or ambiguous sessions.",
		promptSnippet: "Inspect current zmux/tmux context",
		parameters: Type.Object({}),
		async execute(_id, _params, _signal, _onUpdate, ctx) {
			const [pane, tabs, caps] = await Promise.all([currentPane(ctx.cwd), listTabs(ctx.cwd), capabilities(ctx.cwd)]);
			const config = loadConfig(ctx.cwd);
			return content([
				`cwd: ${ctx.cwd}`,
				`config: ${config.path ?? "(none)"}`,
				`policy: ${config.policy.mode}`,
				`pane: ${pane ? JSON.stringify(pane) : "unavailable"}`,
				`tabs:\n${tabs}`,
				`terminal capabilities:\n${caps}`,
			].join("\n"), { pane, tabs, capabilities: caps, config });
		},
	}));

	pi.registerTool(defineTool({
		name: "zmux_tabs",
		label: "zmux tabs",
		description: "List tabs/windows in the current zmux session. Prefer this over running `zmux tabs` through bash.",
		promptSnippet: "List zmux tabs in the current session",
		parameters: Type.Object({}),
		async execute(_id, _params, _signal, _onUpdate, ctx) {
			const tabs = await listTabs(ctx.cwd);
			return content(tabs, { tabs });
		},
	}));

	pi.registerTool(defineTool({
		name: "zmux_tab_kill",
		label: "zmux tab kill",
		description: "Kill a zmux tab/window by name. Use for intentional tab cleanup instead of shelling out to `zmux tab kill` in bash.",
		promptSnippet: "Kill a zmux tab/window",
		parameters: Type.Object({
			tab: Type.String({ description: "Tab/window name to kill" }),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await killTab(params.tab, resolveCwd(ctx.cwd, params.cwd));
			return content(result.text, result.details);
		},
	}));

	pi.registerTool(defineTool({
		name: "zmux_tab_focus",
		label: "zmux tab focus",
		description: "Focus a zmux tab/window by name. Ask the user before using this in agent sessions because it moves terminal focus.",
		promptSnippet: "Focus a zmux tab/window",
		parameters: Type.Object({
			tab: Type.String({ description: "Tab/window name to focus" }),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await focusTab(params.tab, resolveCwd(ctx.cwd, params.cwd));
			return content(result.text, result.details);
		},
	}));

	pi.registerTool(defineTool({
		name: "zmux_send_keys",
		label: "zmux send keys",
		description: "Send raw keys such as C-c, Enter, Escape, or arrows to a zmux tab. Prefer this over `zmux send` in bash.",
		promptSnippet: "Send raw keys to a zmux tab",
		parameters: Type.Object({
			tab: Type.String({ description: "Target tab/window" }),
			keys: Type.Array(Type.String(), { description: "Raw keys to send, e.g. C-c, Enter, Escape" }),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await sendKeys(params.tab, params.keys, resolveCwd(ctx.cwd, params.cwd));
			return content(result.text, result.details);
		},
	}));

	pi.registerTool(defineTool({
		name: "zmux_type",
		label: "zmux type",
		description: "Type text plus Enter into an existing zmux tab. For sudo/password/manual-input commands, prefer zmux_interactive_type.",
		promptSnippet: "Type text into an existing zmux tab",
		parameters: Type.Object({
			tab: Type.String({ description: "Target tab/window" }),
			text: Type.String({ description: "Text to type and submit" }),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await typeText(params.tab, params.text, resolveCwd(ctx.cwd, params.cwd));
			return content(result.text, result.details);
		},
	}));

	pi.registerTool(defineTool({
		name: "zmux_pane_send_keys",
		label: "zmux pane send keys",
		description: "Send raw keys to a specific tmux pane id/title/index. Use for sidecar panes returned by clean_split_control instead of `tmux send-keys` in bash.",
		promptSnippet: "Send raw keys to a zmux pane",
		parameters: Type.Object({
			pane: Type.String({ description: "Target pane id/title/index, e.g. %347" }),
			keys: Type.Array(Type.String(), { description: "Raw keys to send, e.g. C-c, Enter, Escape, l" }),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await sendPaneKeys(params.pane, params.keys, resolveCwd(ctx.cwd, params.cwd));
			return content(result.text, result.details);
		},
	}));

	pi.registerTool(defineTool({
		name: "zmux_pane_type",
		label: "zmux pane type",
		description: "Type text plus Enter into a specific tmux pane id/title/index. Use for sidecar panes returned by clean_split_control instead of `tmux send-keys` in bash.",
		promptSnippet: "Type text into a zmux pane",
		parameters: Type.Object({
			pane: Type.String({ description: "Target pane id/title/index, e.g. %347" }),
			text: Type.String({ description: "Text to type and submit" }),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await typePaneText(params.pane, params.text, resolveCwd(ctx.cwd, params.cwd));
			return content(result.text, result.details);
		},
	}));

	pi.registerTool(defineTool({
		name: "zmux_pane_list",
		label: "zmux pane list",
		description: "List panes in the current zmux window. Prefer this over `zmux pane list` in bash.",
		promptSnippet: "List panes in the current zmux window",
		parameters: Type.Object({}),
		async execute(_id, _params, _signal, _onUpdate, ctx) {
			const panes = await listPanes(ctx.cwd);
			return content(panes, { panes });
		},
	}));

	pi.registerTool(defineTool({
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
	}));

	pi.registerTool(defineTool({
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
	}));

	pi.registerTool(defineTool({
		name: "zmux_pi_respawn",
		label: "zmux Pi respawn",
		description: "Hard-restart the current Pi agent pane by respawning it with `pi -c`. Use after verified Pi extension/tooling changes when soft `/reload` is unavailable, or when Pi is wedged; this kills the current pane process and discards unsent input. If autonomous follow-up is expected, pass continuationPrompt.",
		promptSnippet: "Hard-restart/respawn the current Pi pane",
		promptGuidelines: [
			"After changing/installing Pi extensions or tools, prefer zmux_pi_respawn over asking the user to manually reload, unless a non-destructive soft reload tool is available.",
			"If work should continue after respawn, pass continuationPrompt with the exact next smoke/validation steps; hard respawn does not preserve in-flight agent continuation by itself.",
			"Do not use if the user may have unsent input or if manual validation is in progress; explain that it hard-restarts the current Pi pane.",
		],
		parameters: Type.Object({
			paneId: Type.Optional(Type.String({ description: "Target tmux pane id; defaults to current Pi pane" })),
			command: Type.Optional(Type.String({ description: "Restart command; defaults to `pi -c`. Cannot be combined with continuationPrompt." })),
			continuationPrompt: Type.Optional(Type.String({ description: "Optional handoff prompt for the restarted Pi process. Use when autonomous follow-up should continue after respawn." })),
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
	}));

	pi.registerTool(defineTool({
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
			command: Type.Optional(Type.String({ description: "Command to run; optional when configured in .pi/zmux.json" })),
			tab: Type.Optional(Type.String({ description: "zmux tab name; defaults to configured tab or name" })),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
			readiness: Type.Optional(Type.String({ description: "Regex to wait for in new output, e.g. ready|listening|localhost" })),
			timeoutSeconds: Type.Optional(Type.Number({ description: "Readiness timeout seconds" })),
			restart: Type.Optional(Type.Boolean({ description: "Send C-c to the tab before ensuring" })),
			kind: Type.Optional(Type.String({ description: "Runtime kind for metadata: server, worker, watcher, tui, etc." })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const config = loadConfig(ctx.cwd);
			const runtime = mergeRuntimeConfig(params.name, params, config);
			if (!runtime.command) {
				return content(`ERROR: runtime ${params.name} has no command. Pass command or add it to .pi/zmux.json.`, { name: params.name, configPath: config.path });
			}
			const result = await runtimeEnsure({
				tab: runtime.tab,
				command: runtime.command,
				cwd: resolveCwd(ctx.cwd, runtime.cwd),
				readiness: runtime.readiness,
				timeoutSeconds: runtime.timeoutSeconds,
				restart: params.restart,
			});
			return content(result.text, { ...result.details, name: params.name, kind: runtime.kind, configPath: config.path });
		},
	}));

	pi.registerTool(defineTool({
		name: "zmux_runtime_logs",
		label: "zmux runtime logs",
		description: "Read logs/output from a named zmux runtime tab. Use when debugging software that should already be running instead of starting a duplicate process.",
		promptSnippet: "Read output from a named zmux runtime tab",
		parameters: Type.Object({
			name: Type.String({ description: "Runtime name or tab" }),
			tab: Type.Optional(Type.String({ description: "Explicit zmux tab name" })),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
			lines: Type.Optional(Type.Number({ description: "Lines to capture; default 120" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const config = loadConfig(ctx.cwd);
			const runtime = mergeRuntimeConfig(params.name, { tab: params.tab, cwd: params.cwd }, config);
			const result = await runtimeLogs(runtime.tab, resolveCwd(ctx.cwd, runtime.cwd), params.lines ?? 120);
			return content(result.text, { ...result.details, name: params.name, configPath: config.path });
		},
	}));

	pi.registerTool(defineTool({
		name: "zmux_runtime_stop",
		label: "zmux runtime stop",
		description: "Stop a named zmux runtime tab by sending C-c. Use for visible, controlled shutdown of dev servers, watchers, workers, or demos.",
		promptSnippet: "Stop a named zmux runtime tab",
		parameters: Type.Object({
			name: Type.String({ description: "Runtime name or tab" }),
			tab: Type.Optional(Type.String({ description: "Explicit zmux tab name" })),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const config = loadConfig(ctx.cwd);
			const runtime = mergeRuntimeConfig(params.name, { tab: params.tab, cwd: params.cwd }, config);
			const result = await runtimeStop(runtime.tab, resolveCwd(ctx.cwd, runtime.cwd));
			return content(result.text, { ...result.details, name: params.name, configPath: config.path });
		},
	}));

	pi.registerTool(defineTool({
		name: "zmux_interactive_type",
		label: "zmux interactive type",
		description: "Type an interactive or privileged command into a shared zmux tab. Use for sudo, ssh, REPLs, database shells, or commands needing user input instead of running them in bash.",
		promptSnippet: "Type an interactive command into a shared zmux tab",
		promptGuidelines: ["Use zmux_interactive_type for sudo/password/manual-input commands. For one-shot commands that should return output (for example sudo status checks), waitForExit defaults on; tell the user which tab needs attention while the tool waits."],
		parameters: Type.Object({
			command: Type.String({ description: "Command text to type and submit" }),
			tab: Type.Optional(Type.String({ description: "Tab name; defaults to admin" })),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
			waitFor: Type.Optional(Type.String({ description: "Advanced: regex to wait for instead of waiting for command exit" })),
			waitForExit: Type.Optional(Type.Boolean({ description: "Wrap command with a completion sentinel and wait for exit; defaults to true for sudo/manual one-shot commands, false for long interactive shells" })),
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
					details: { tab, waiting: true },
				});
			}
			const result = await interactiveType(tab, params.command, resolveCwd(ctx.cwd, params.cwd), {
				waitFor: params.waitFor,
				waitForExit,
				timeoutSeconds: params.timeoutSeconds,
				lines: params.lines,
				focus: params.focus,
			});
			const note = waitForExit || params.waitFor ? "" : `\nTell the user to complete any prompts in tab ${tab}.`;
			return content(`${result.text}${note}`, result.details);
		},
	}));
}
