import { Type } from "typebox";
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";
import { mergeRuntimeConfig } from "../config.js";
import {
	cancelCallback,
	findRecentCallbackCompletion,
	interactiveType,
	listCallbacks,
	listRecentCallbackCompletions,
	runtimeEnsure,
	runtimeLogs,
	runtimeStop,
	schedulePiRespawn,
	startPeerHandoff,
	startWatchCallback,
} from "../zmux.js";
import { configFor, content, rejectHeadlessAgentPrintMode, resolveCwd, shouldWaitForExit } from "./shared.js";

export function registerRuntimeTools(pi: ExtensionAPI): void {
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
			if (params.command) {
				const headlessAgentError = rejectHeadlessAgentPrintMode(params.command);
				if (headlessAgentError) return content(headlessAgentError, { command: params.command, failed: true, failureKind: "headless_agent_print_mode" });
			}
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
			if (runtime.command) {
				const headlessAgentError = rejectHeadlessAgentPrintMode(runtime.command);
				if (headlessAgentError) return content(headlessAgentError, { command: runtime.command, failed: true, failureKind: "headless_agent_print_mode" });
			}
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
		description: "Read logs/output from a named zmux runtime tab. Use when debugging software that should already be running instead of starting a duplicate process. Can wait briefly for a regex or idle output instead of raw sleeps.",
		promptSnippet: "Read output from a named zmux runtime tab",
		parameters: Type.Object({
			name: Type.String({ description: "Runtime name or tab" }),
			tab: Type.Optional(Type.String({ description: "Explicit zmux tab name" })),
			session: Type.Optional(Type.String({ description: "Optional zmux session target (`-s`)" })),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
			lines: Type.Optional(Type.Number({ description: "Lines to capture; default 120" })),
			waitFor: Type.Optional(Type.String({ description: "Regex to wait for in new output" })),
			idleSeconds: Type.Optional(Type.Number({ description: "Wait until output is stable for N seconds" })),
			timeoutSeconds: Type.Optional(Type.Number({ description: "Wait timeout seconds for waitFor/idleSeconds; default 10" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const config = configFor(ctx);
			const runtime = mergeRuntimeConfig(params.name, { tab: params.tab, cwd: params.cwd, session: params.session }, config);
			const result = await runtimeLogs(runtime.tab, resolveCwd(ctx.cwd, runtime.cwd), params.lines ?? 120, runtime.session, {
				waitFor: params.waitFor,
				idleSeconds: params.idleSeconds,
				timeoutSeconds: params.timeoutSeconds,
			});
			return content(result.text, { ...result.details, name: params.name, configPath: config.path, ignoredReason: config.ignoredReason });
		},
	});

	pi.registerTool({
		name: "zmux_callback",
		label: "zmux callback",
		description: "Start/list/cancel a live-session-scoped zmux wait callback. Use when a visible tab should notify Pi later after new output matches a regex or the screen goes idle, instead of sleeping or polling in the agent shell.",
		promptSnippet: "Schedule a zmux output callback",
		promptGuidelines: [
			"Use action=watch for explicit long-running completion/readiness handoff from a visible tab.",
			"This starts an extension-owned `zmux wait --json` process and sends a Pi callback message when it completes; callbacks are live-session scoped, not durable across reload/crash.",
			"Default delivery is steer so active turns can see the completion before the next model call; pass deliverAs=followUp when you want an end-turn-only handoff.",
			"Prefer waitFor for known future output and idleSeconds for generic quiet-after-output handoff. Do not use this as a hidden dev-server substitute; persistent runtimes still use zmux_runtime_ensure.",
		],
		parameters: Type.Object({
			action: Type.String({ description: "watch, list, or cancel" }),
			id: Type.Optional(Type.String({ description: "Callback id. Optional for watch, required for cancel." })),
			tab: Type.Optional(Type.String({ description: "Tab to watch (required for action=watch)" })),
			session: Type.Optional(Type.String({ description: "Optional zmux session target (`-s`)" })),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
			lines: Type.Optional(Type.Number({ description: "Lines to capture in the completion report; default 160" })),
			waitFor: Type.Optional(Type.String({ description: "Regex to wait for in new output" })),
			idleSeconds: Type.Optional(Type.Number({ description: "Wait until output is quiet for N seconds" })),
			timeoutSeconds: Type.Optional(Type.Number({ description: "Callback wait timeout seconds; default 300" })),
			message: Type.Optional(Type.String({ description: "Human-readable purpose included in the follow-up" })),
			deliverAs: Type.Optional(Type.String({ description: "Pi delivery mode: steer (default), followUp, or nextTurn" })),
			triggerTurn: Type.Optional(Type.Boolean({ description: "Trigger an agent turn when the callback completes; default true" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const action = params.action.trim();
			if (action === "list") {
				const active = listCallbacks();
				const completed = listRecentCallbackCompletions();
				const activeText = active.length ? `active zmux callbacks:\n${active.map((callback) => `- ${callback.id} (${callback.kind}) ${callback.tab}`).join("\n")}` : "no active zmux callbacks";
				const completedText = completed.length ? `recent completed zmux callbacks:\n${completed.map((callback) => `- ${callback.id} (${callback.kind}) exit=${callback.exitCode ?? "signal"} ${callback.tab}`).join("\n")}` : "no recent completed zmux callbacks";
				return content(`${activeText}\n${completedText}`, { callbacks: active, completedCallbacks: completed });
			}
			if (action === "cancel") {
				if (!params.id) return content("ERROR: callback cancel requires id", { action });
				const cancelled = cancelCallback(params.id);
				const completed = cancelled ? undefined : findRecentCallbackCompletion(params.id);
				if (completed) return content(`zmux callback ${params.id} already completed`, { id: params.id, cancelled, completed: true, callback: completed });
				return content(cancelled ? `cancelled zmux callback ${params.id}` : `no active zmux callback ${params.id}`, { id: params.id, cancelled });
			}
			if (action !== "watch") return content(`ERROR: unknown callback action ${action}; use watch, list, or cancel`, { action });
			if (!params.tab) return content("ERROR: callback watch requires tab", { action });
			if (!params.waitFor && params.idleSeconds === undefined) return content("ERROR: callback watch requires waitFor or idleSeconds", { action, tab: params.tab });
			const result = startWatchCallback(pi, {
				id: params.id,
				tab: params.tab,
				cwd: resolveCwd(ctx.cwd, params.cwd),
				session: params.session,
				lines: params.lines,
				waitFor: params.waitFor,
				idleSeconds: params.idleSeconds,
				timeoutSeconds: params.timeoutSeconds,
				message: params.message,
				deliverAs: params.deliverAs as "steer" | "followUp" | "nextTurn" | undefined,
				triggerTurn: params.triggerTurn,
			});
			return content(result.text, result.details);
		},
	});

	pi.registerTool({
		name: "zmux_peer_handoff",
		label: "zmux peer handoff",
		description: "Type a prompt into an existing peer tab and schedule a Pi callback message when first-class zmux wait evidence completes. Use for explicit agent end-turn handoff without raw sleeps or private CLI parsing.",
		promptSnippet: "Type to a peer and schedule output handoff",
		promptGuidelines: [
			"Use after zmux_peer_ensure when you want Pi to continue later after a peer response arrives.",
			"For exact marker workflows, pass waitFor that is expected only in the assistant response, not in the typed prompt. For generic peer turns, use idleSeconds and inspect the returned output.",
			"Unsupported CLIs degrade to output/idle evidence; do not claim lifecycle readiness unless tab status/turnAt proves it.",
		],
		parameters: Type.Object({
			tab: Type.String({ description: "Peer tab name" }),
			text: Type.String({ description: "Prompt text to type and submit" }),
			session: Type.Optional(Type.String({ description: "Optional zmux session target (`-s`)" })),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
			id: Type.Optional(Type.String({ description: "Optional callback id" })),
			lines: Type.Optional(Type.Number({ description: "Lines to capture in the callback report; default 200" })),
			waitFor: Type.Optional(Type.String({ description: "Regex to wait for in new peer output" })),
			idleSeconds: Type.Optional(Type.Number({ description: "Wait until peer output is quiet for N seconds; default 3 when waitFor is omitted" })),
			timeoutSeconds: Type.Optional(Type.Number({ description: "Callback wait timeout seconds; default 300" })),
			markPeerRunning: Type.Optional(Type.Boolean({ description: "Stamp peer lifecycle as running before typing" })),
			source: Type.Optional(Type.String({ description: "Lifecycle source label when marking peer running" })),
			message: Type.Optional(Type.String({ description: "Purpose included in lifecycle/callback message" })),
			deliverAs: Type.Optional(Type.String({ description: "Pi delivery mode: steer (default), followUp, or nextTurn" })),
			triggerTurn: Type.Optional(Type.Boolean({ description: "Trigger an agent turn when handoff completes; default true" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await startPeerHandoff(pi, {
				id: params.id,
				tab: params.tab,
				text: params.text,
				cwd: resolveCwd(ctx.cwd, params.cwd),
				session: params.session,
				lines: params.lines,
				waitFor: params.waitFor,
				idleSeconds: params.idleSeconds,
				timeoutSeconds: params.timeoutSeconds,
				markPeerRunning: params.markPeerRunning,
				source: params.source,
				message: params.message,
				deliverAs: params.deliverAs as "steer" | "followUp" | "nextTurn" | undefined,
				triggerTurn: params.triggerTurn,
			});
			return content(result.text, result.details);
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
		promptGuidelines: ["Use zmux_interactive_type for sudo/password/manual-input commands. For one-shot commands that should return output, waitForExit defaults on and reads shell lifecycle status; tell the user which tab needs attention while the tool waits."],
		parameters: Type.Object({
			command: Type.String({ description: "Command text to type and submit" }),
			tab: Type.Optional(Type.String({ description: "Tab name; defaults to admin" })),
			session: Type.Optional(Type.String({ description: "Optional zmux session target (`-s`)" })),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
			waitFor: Type.Optional(Type.String({ description: "Advanced: regex to wait for instead of waiting for command exit" })),
			waitForExit: Type.Optional(Type.Boolean({ description: "Wait for the next shell lifecycle command to finish; defaults true for sudo/manual one-shots, false for long interactive shells" })),
			timeoutSeconds: Type.Optional(Type.Number({ description: "Wait timeout seconds; default 90" })),
			lines: Type.Optional(Type.Number({ description: "Lines to return from the tab while waiting; default 160" })),
			focus: Type.Optional(Type.Boolean({ description: "Focus the tab after creating/typing; default false. Ask the user before using this for agent sessions." })),
		}),
		async execute(_id, params, _signal, onUpdate, ctx) {
			const headlessAgentError = rejectHeadlessAgentPrintMode(params.command);
			if (headlessAgentError) return content(headlessAgentError, { command: params.command, failed: true, failureKind: "headless_agent_print_mode" });
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
