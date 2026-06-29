import { isToolCallEventType, type ExtensionAPI } from "@earendil-works/pi-coding-agent";
import { classifyBash, hasExplicitBypass, shouldBlock } from "./classify.js";
import { loadConfig } from "./config.js";
import { takeRespawnContinuation } from "./respawn-continuation.js";
import { registerZmuxTools } from "./tools.js";
import { currentPane, listTabs } from "./zmux.js";

function compact(value: string, max = 1200): string {
	if (value.length <= max) return value;
	return `${value.slice(0, max)}…`;
}

async function buildContext(cwd: string): Promise<string> {
	const config = loadConfig(cwd);
	const pane = await currentPane(cwd);
	const tabs = await listTabs(cwd);
	const configured = Object.entries(config.runtimes)
		.map(([name, runtime]) => `${name}: tab=${runtime.tab ?? name}${runtime.command ? ` cmd=${runtime.command}` : ""}`)
		.join("\n");
	return [
		"pi-zmux context:",
		`- policy: ${config.policy.mode}${config.path ? ` (${config.path})` : ""}`,
		pane ? `- current zmux: session=${pane.Session ?? "?"} pane=${pane.ID ?? "?"} tab=${pane.WindowIndex ?? "?"} cwd=${pane.Dir ?? "?"}` : "- current zmux: unavailable/outside tmux",
		configured ? `- configured runtimes:\n${configured}` : "- configured runtimes: none",
		`- visible tabs:\n${compact(tabs, 900)}`,
		"Rules: use zmux_runtime_ensure/logs/stop for software under development that keeps running (servers, workers, watchers, TUI demos). Use zmux_interactive_type for sudo/password/manual-input commands. Do not start runtimes with bash backgrounding, nohup, or one-off foreground server commands.",
	].join("\n");
}

export default function (pi: ExtensionAPI): void {
	registerZmuxTools(pi);

	pi.on("before_agent_start", async (event, ctx) => {
		const zmuxContext = await buildContext(ctx.cwd);
		const current = event.systemPrompt ?? "";
		return { systemPrompt: current ? `${current}\n\n${zmuxContext}` : zmuxContext };
	});

	pi.on("session_start", async (_event, ctx) => {
		const continuation = takeRespawnContinuation(ctx.cwd);
		if (!continuation) return;
		ctx.ui.notify("pi-zmux · respawn continuation ready", "info");
		pi.sendMessage({
			customType: "pi-zmux-respawn-continuation",
			content: continuation.prompt,
			display: true,
			details: {
				kind: "respawn_continuation",
				createdAt: continuation.createdAt,
				handoffPath: continuation.handoffPath,
			},
		}, { deliverAs: "followUp", triggerTurn: true });
	});

	pi.on("tool_call", async (event, ctx) => {
		if (!isToolCallEventType("bash", event)) return;
		const command = event.input.command;
		if (typeof command !== "string") return;
		if (hasExplicitBypass(command)) {
			ctx.ui.notify("pi-zmux: explicit bash guardrail bypass used", "warning");
			return;
		}
		const config = loadConfig(ctx.cwd);
		const classification = classifyBash(command, config);
		if (classification.kind === "safe" || config.policy.mode === "observe") return;

		const message = [
			`pi-zmux ${config.policy.mode}: ${classification.kind} command detected (${classification.reason}).`,
			classification.suggestion,
		].join("\n");

		if (config.policy.mode === "warn") {
			ctx.ui.notify(message, "warning");
			return;
		}

		if (shouldBlock(classification, config)) {
			return { block: true, reason: message };
		}
	});

	pi.registerCommand("zmux", {
		description: "Show pi-zmux context and policy",
		handler: async (_args, ctx) => {
			ctx.ui.notify(await buildContext(ctx.cwd), "info");
		},
	});
}
