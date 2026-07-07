import { Type } from "typebox";
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";
import {
	capabilities,
	currentPane,
	listSessions,
	listTabs,
	reloadZmux,
	runCommand,
	schedulePiReload,
	sessionKill,
	sessionRun,
} from "../zmux.js";
import { configFor, content, rejectHeadlessAgentPrintMode, resolveCwd } from "./shared.js";

export function registerCoreTools(pi: ExtensionAPI): void {
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
			delayMs: Type.Optional(Type.Number({ description: "Delay before typing /reload; default 12000ms so the current assistant response can finish" })),
			retryAttempts: Type.Optional(Type.Number({ description: "Total /reload attempts when Pi says the current response is still active; default 3" })),
			retryDelayMs: Type.Optional(Type.Number({ description: "Delay between retry attempts after Pi prints the active-response warning; default 10000ms" })),
			cwd: Type.Optional(Type.String({ description: "Working directory; defaults to Pi cwd" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const result = await schedulePiReload({
				cwd: resolveCwd(ctx.cwd, params.cwd),
				paneId: params.paneId,
				continuationPrompt: params.continuationPrompt,
				delayMs: params.delayMs,
				retryAttempts: params.retryAttempts,
				retryDelayMs: params.retryDelayMs,
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
			"Do not add your own sentinels or wrapper scripts; zmux-managed shell lifecycle owns command state, and watch/log tools own output inspection.",
		],
		parameters: Type.Object({
			command: Type.String({ description: "Command to run" }),
			tab: Type.Optional(Type.String({ description: "Stable zmux tab name (`-n`). Defaults to zmux's command-derived name." })),
			session: Type.Optional(Type.String({ description: "Optional zmux session target (`-s`)" })),
			cwd: Type.Optional(Type.String({ description: "Working directory for the zmux CLI process; defaults to Pi cwd" })),
			timeoutSeconds: Type.Optional(Type.Number({ description: "Wait timeout seconds for non-detached runs; default 30" })),
			lines: Type.Optional(Type.Number({ description: "Lines to capture while waiting/following; default is zmux's default" })),
			detach: Type.Optional(Type.Boolean({ description: "Run detached (`-d`). For persistent servers prefer zmux_runtime_ensure." })),
			follow: Type.Optional(Type.Boolean({ description: "Follow output (`-f`) until timeout/interruption. Usually prefer zmux_runtime_logs for later reads." })),
			keep: Type.Optional(Type.Boolean({ description: "Exempt this tab from auto-reaping (`--keep`)" })),
			scope: Type.Optional(Type.String({ description: "Lifecycle scope, e.g. task or daemon" })),
		}),
		async execute(_id, params, _signal, _onUpdate, ctx) {
			const headlessAgentError = rejectHeadlessAgentPrintMode(params.command);
			if (headlessAgentError) return content(headlessAgentError, { command: params.command, failed: true, failureKind: "headless_agent_print_mode" });
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
			const headlessAgentError = rejectHeadlessAgentPrintMode(params.command);
			if (headlessAgentError) return content(headlessAgentError, { command: params.command, failed: true, failureKind: "headless_agent_print_mode" });
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
}
