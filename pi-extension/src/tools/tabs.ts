import { Type } from "typebox";
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";
import {
	focusTab,
	inspectTab,
	killTab,
	labelTab,
	logCommand,
	moveTab,
	peerEnsure,
	placeTab,
	sendKeys,
	setTabPeer,
	setTabState,
	snapshot,
	tabStatus,
	terminalCurrent,
	typeTextWithWait,
} from "../zmux.js";
import {
	content,
	logAction,
	resolveCwd,
	tabPlacementAction,
	tabPlacementDirection,
	tabStateAction,
	validateLogParams,
	validateTabPlacementParams,
} from "./shared.js";

function tabPeerAction(value: string): "start" | "running" | "ready" | "waiting" | "attention" | "failed" | "consumed" | "park" | "keep" | "clear-keep" {
	switch (value) {
		case "start":
		case "running":
		case "ready":
		case "waiting":
		case "attention":
		case "failed":
		case "consumed":
		case "park":
		case "keep":
		case "clear-keep":
			return value;
		default:
			throw new Error(`peer action must be one of: start, running, ready, waiting, attention, failed, consumed, park, keep, clear-keep (got ${value})`);
	}
}

export function registerTabTools(pi: ExtensionAPI): void {
	pi.registerTool({
		name: "zmux_tab_state",
		label: "zmux tab state",
		description: "Set or clear a zmux tab lifecycle glyph (attention/failed/running/ready/done/clear). Use for peer/worker handoffs and human-visible status instead of shelling out to `zmux tab state`.",
		promptSnippet: "Set a zmux tab lifecycle state",
		parameters: Type.Object({
			state: Type.String({ description: "attention, failed, running, ready, done, or clear" }),
			tab: Type.Optional(Type.String({ description: "Tab name target; omitted means current pane" })),
			target: Type.Optional(Type.String({ description: "Raw pane/window/tab target; overrides tab" })),
			session: Type.Optional(Type.String({ description: "Session for tab-name targets (`-s`)" })),
			source: Type.Optional(Type.String({ description: "State source label" })),
			message: Type.Optional(Type.String({ description: "Display-only message for ready/attention/failed states" })),
			ifState: Type.Optional(Type.String({ description: "For clear: only clear if current state matches" })),
			byVisibility: Type.Optional(Type.Boolean({ description: "For done only: store attention instead when pane is not visible" })),
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
		name: "zmux_tab_peer",
		label: "zmux tab peer",
		description: "Record semantic peer/agent-turn lifecycle metadata (start/running, ready, attention, failed, consumed, park, timestamped keep). Prefer this over manual glyph-only state for prompt-scoped peer tabs.",
		promptSnippet: "Set peer lifecycle metadata",
		parameters: Type.Object({
			action: Type.String({ description: "start, running, ready, waiting, attention, failed, consumed, park, keep, or clear-keep" }),
			tab: Type.Optional(Type.String({ description: "Tab name target; omitted means current pane" })),
			target: Type.Optional(Type.String({ description: "Raw pane/window/tab target; overrides tab" })),
			session: Type.Optional(Type.String({ description: "Session for tab-name targets (`-s`)" })),
			role: Type.Optional(Type.String({ description: "Peer role/CLI label, e.g. claude or codex" })),
			hostTab: Type.Optional(Type.String({ description: "Stable host logical tab id" })),
			hostPane: Type.Optional(Type.String({ description: "Host pane id" })),
			topic: Type.Optional(Type.String({ description: "Sanitized display topic/title; do not include full prompts" })),
			ttl: Type.Optional(Type.String({ description: "Retention TTL for park/keep, e.g. 30m or 2h. keep requires this." })),
			source: Type.Optional(Type.String({ description: "Lifecycle source label" })),
			message: Type.Optional(Type.String({ description: "Optional glyph message for ready/attention/failed" })),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await setTabPeer({
				action: tabPeerAction(params.action),
				tab: params.tab,
				target: params.target,
				session: params.session,
				role: params.role,
				hostTab: params.hostTab,
				hostPane: params.hostPane,
				topic: params.topic,
				ttl: params.ttl,
				source: params.source,
				msg: params.message,
				cwd: resolveCwd(ctx.cwd, params.cwd),
			});
			return content(result.text, result.details);
		},
	});

	pi.registerTool({
		name: "zmux_tab_status",
		label: "zmux tab status",
		description: "Read lifecycle, command, and peer turn status for a zmux tab as JSON. Use this for status/freshness checks; do not set glyphs with this tool.",
		promptSnippet: "Read zmux tab lifecycle/command/peer status",
		parameters: Type.Object({
			tab: Type.String({ description: "Tab name target" }),
			session: Type.Optional(Type.String({ description: "Session for tab-name targets (`-s`)" })),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await tabStatus({ tab: params.tab, session: params.session, cwd: resolveCwd(ctx.cwd, params.cwd) });
			return content(result.text, result.details);
		},
	});

	pi.registerTool({
		name: "zmux_tab_inspect",
		label: "zmux tab inspect",
		description: "Inspect a zmux tab in one call: lifecycle/status JSON plus recent output tail and warnings. Prefer this over repeated tabs/status/logs calls when diagnosing agent or peer state.",
		promptSnippet: "Inspect tab status plus recent output",
		parameters: Type.Object({
			tab: Type.String({ description: "Tab name target" }),
			session: Type.Optional(Type.String({ description: "Session for tab-name targets (`-s`)" })),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
			lines: Type.Optional(Type.Number({ description: "Output lines to capture; default 120" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await inspectTab({ tab: params.tab, session: params.session, cwd: resolveCwd(ctx.cwd, params.cwd), lines: params.lines });
			return content(result.text, result.details);
		},
	});

	pi.registerTool({
		name: "zmux_peer_ensure",
		label: "zmux peer ensure",
		description: "Create/reuse a peer tab, stamp peer lifecycle metadata, wait briefly for readiness if requested, and return status plus output evidence. Use for prompt-scoped peer CLIs instead of hand-rolled run/status/watch loops.",
		promptSnippet: "Ensure a peer tab and inspect readiness",
		parameters: Type.Object({
			tab: Type.String({ description: "Peer tab name, e.g. claude-peer or codex-peer" }),
			command: Type.Optional(Type.String({ description: "Command to run when starting/restarting the peer; omit to stamp/inspect an existing tab" })),
			session: Type.Optional(Type.String({ description: "Session for tab-name targets (`-s`)" })),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
			role: Type.Optional(Type.String({ description: "Peer role/CLI label, e.g. claude or codex" })),
			hostTab: Type.Optional(Type.String({ description: "Stable host logical tab id" })),
			hostPane: Type.Optional(Type.String({ description: "Host pane id" })),
			topic: Type.Optional(Type.String({ description: "Sanitized display topic/title; do not include full prompts" })),
			source: Type.Optional(Type.String({ description: "Lifecycle source label" })),
			message: Type.Optional(Type.String({ description: "Optional glyph message" })),
			readiness: Type.Optional(Type.String({ description: "Regex to wait for in new peer output" })),
			waitForTurnState: Type.Optional(Type.String({ description: "Peer turn state to wait for briefly, e.g. ready, attention, failed, running" })),
			timeoutSeconds: Type.Optional(Type.Number({ description: "Short readiness/wait timeout; default 10" })),
			lines: Type.Optional(Type.Number({ description: "Output lines to capture; default 120" })),
			restart: Type.Optional(Type.Boolean({ description: "Send C-c before starting command" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await peerEnsure({
				tab: params.tab,
				command: params.command,
				session: params.session,
				cwd: resolveCwd(ctx.cwd, params.cwd),
				role: params.role,
				hostTab: params.hostTab,
				hostPane: params.hostPane,
				topic: params.topic,
				source: params.source,
				message: params.message,
				readiness: params.readiness,
				waitForTurnState: params.waitForTurnState,
				timeoutSeconds: params.timeoutSeconds,
				lines: params.lines,
				restart: params.restart,
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
			session: Type.Optional(Type.String({ description: "Source session for tab-name targets (`-s`)" })),
			force: Type.Optional(Type.Boolean({ description: "Allow cross-workspace move" })),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await moveTab({ tab: params.tab, destination: params.destination, session: params.session, force: params.force, cwd: resolveCwd(ctx.cwd, params.cwd) });
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
		description: "Move logical tabs between full, pane, hidden, and shown placements. Defaults to not moving terminal focus; set focus only when the user explicitly wants to be taken there.",
		promptSnippet: "Place a zmux logical tab as pane/full/hidden/shown",
		promptGuidelines: ["By default pane/show placement is focus-safe for agents. Set focus: true only when the user asked to move terminal focus."],
		parameters: Type.Object({
			action: Type.String({ description: "pane, full, hide, or show" }),
			tab: Type.Optional(Type.String({ description: "Tab name/index target; required for pane and show, optional for full/hide current-pane flows" })),
			session: Type.Optional(Type.String({ description: "Session for tab-name targets" })),
			into: Type.Optional(Type.String({ description: "Host tab for pane action" })),
			direction: Type.Optional(Type.String({ description: "Pane direction: right, left, up, or down" })),
			size: Type.Optional(Type.String({ description: "Pane size, e.g. 40% or 80" })),
			pane: Type.Optional(Type.String({ description: "Raw pane id for mouse/menu-style targets" })),
			after: Type.Optional(Type.Boolean({ description: "For full: insert after old host" })),
			focus: Type.Optional(Type.Boolean({ description: "For pane/show: select the placed pane after moving it; default false for agent safety" })),
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
				focus: params.focus,
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
			tab: Type.String({ description: "Tab/window name to kill" }),
			session: Type.Optional(Type.String({ description: "Source session for tab-name targets (`-s`)" })),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await killTab(params.tab, resolveCwd(ctx.cwd, params.cwd), params.session);
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
		description: "Type text plus Enter into an existing zmux tab. For sudo/password/manual-input commands, prefer zmux_interactive_type. For peer turns, optionally mark running and wait briefly for fresh lifecycle readiness.",
		promptSnippet: "Type text into an existing zmux tab",
		parameters: Type.Object({
			tab: Type.String({ description: "Target tab/window" }),
			text: Type.String({ description: "Text to type and submit" }),
			session: Type.Optional(Type.String({ description: "Optional zmux session target (`-s`)" })),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
			markPeerRunning: Type.Optional(Type.Boolean({ description: "After typing, stamp peer lifecycle as running" })),
			waitForTurnState: Type.Optional(Type.String({ description: "Wait briefly for a fresh turn state, e.g. ready, attention, failed, running" })),
			timeoutSeconds: Type.Optional(Type.Number({ description: "Short lifecycle wait timeout; default 8" })),
			lines: Type.Optional(Type.Number({ description: "Output lines to include when waiting" })),
			source: Type.Optional(Type.String({ description: "Lifecycle source label when marking peer running" })),
			message: Type.Optional(Type.String({ description: "Optional glyph message when marking peer running" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await typeTextWithWait({
				tab: params.tab,
				text: params.text,
				session: params.session,
				cwd: resolveCwd(ctx.cwd, params.cwd),
				markPeerRunning: params.markPeerRunning,
				waitForTurnState: params.waitForTurnState,
				timeoutSeconds: params.timeoutSeconds,
				lines: params.lines,
				source: params.source,
				message: params.message,
			});
			return content(result.text, result.details);
		},
	});
}
