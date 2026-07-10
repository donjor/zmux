import { isToolCallEventType, type ExtensionAPI } from "@earendil-works/pi-coding-agent";
import { classifyBash, hasExplicitBypass, shouldBlock } from "./classify.js";
import { loadConfig } from "./config.js";
import { shouldTriggerContinuation, takeReloadContinuation, type ReloadContinuation } from "./reload-continuation.js";
import { takeRespawnContinuation, type RespawnContinuation } from "./respawn-continuation.js";
import { registerZmuxDispatcher } from "./dispatcher.js";
import { registerGlyphLifecycle } from "./lifecycle.js";
import { clearCallbacks, currentPane, listTabs, reloadZmux, zmux } from "./zmux.js";


function compact(value: string, max = 1200): string {
	if (value.length <= max) return value;
	return `${value.slice(0, max)}… [truncated]`;
}

async function zmuxVersion(cwd: string): Promise<string> {
	try {
		const result = await zmux(["version"], { cwd, timeoutMs: 3_000 });
		return result.stdout.trim();
	} catch (error) {
		return `unavailable: ${error instanceof Error ? error.message : String(error)}`;
	}
}

async function buildContext(cwd: string, projectTrusted: boolean): Promise<string> {
	const config = loadConfig(cwd, { projectTrusted });
	const [pane, tabs, version] = await Promise.all([currentPane(cwd), listTabs(cwd), zmuxVersion(cwd)]);
	const configured = compact(
		Object.entries(config.runtimes)
			.slice(0, 8)
			.map(([name, runtime]) => `${name}: tab=${runtime.tab ?? name}${runtime.session ? ` session=${runtime.session}` : ""}${runtime.command ? ` cmd=${compact(runtime.command, 120)}` : ""}`)
			.join("\n"),
		600,
	);
	let configStatus = "none";
	if (config.path) {
		configStatus = config.ignoredReason ? `${config.path} ignored (${config.ignoredReason})` : config.path;
	}
	return [
		"pi-zmux context:",
		`- zmux binary: ${process.env.PI_ZMUX_BIN?.trim() || "zmux"} (${version})`,
		`- policy: ${config.policy.mode}; config: ${configStatus}; projectTrusted=${projectTrusted}`,
		pane ? `- current zmux: session=${pane.Session ?? "?"} pane=${pane.ID ?? "?"} tab=${pane.WindowIndex ?? "?"} cwd=${pane.Dir ?? "?"}` : "- current zmux: unavailable/outside tmux",
		configured ? `- configured runtimes:\n${configured}` : "- configured runtimes: none",
		`- visible tabs:\n${compact(tabs, 700)}`,
		"Rules: use zmux_lite operation=runtime_ensure/runtime_logs/runtime_stop for persistent software, and operation=interactive_type for sudo/password/manual input. Bounded checks can stay in bash; never hide runtimes with &, nohup, disown, or raw tmux.",
	].join("\n");
}

function sendContinuation(
	pi: ExtensionAPI,
	kind: "reload" | "respawn",
	continuation: ReloadContinuation | RespawnContinuation,
): boolean {
	if (!shouldTriggerContinuation(continuation.prompt)) return false;
	pi.sendMessage({
		customType: `pi-zmux-${kind}-continuation`,
		content: continuation.prompt,
		display: true,
		details: {
			kind: `${kind}_continuation`,
			createdAt: continuation.createdAt,
			...("handoffPath" in continuation ? { handoffPath: continuation.handoffPath } : {}),
		},
	}, { deliverAs: "followUp", triggerTurn: true });
	return true;
}

export default function (pi: ExtensionAPI): void {
	registerZmuxDispatcher(pi);

	pi.on("session_shutdown", async () => {
		clearCallbacks();
	});

	registerGlyphLifecycle(pi);

	pi.on("session_start", async (_event, ctx) => {
		const reloadContinuation = takeReloadContinuation(ctx.cwd);
		if (reloadContinuation) {
			const sent = sendContinuation(pi, "reload", reloadContinuation);
			ctx.ui.notify(sent ? "pi-zmux · reload continuation ready" : "pi-zmux · reload complete; waiting for user", "info");
			return;
		}

		const respawnContinuation = takeRespawnContinuation(ctx.cwd);
		if (!respawnContinuation) return;
		const sent = sendContinuation(pi, "respawn", respawnContinuation);
		ctx.ui.notify(sent ? "pi-zmux · respawn continuation ready" : "pi-zmux · respawn complete; waiting for user", "info");
	});

	pi.on("tool_call", async (event, ctx) => {
		if (!isToolCallEventType("bash", event)) return;
		const command = event.input.command;
		if (typeof command !== "string") return;
		if (hasExplicitBypass(command)) {
			ctx.ui.notify("pi-zmux: explicit bash guardrail bypass used", "warning");
			return;
		}
		const config = loadConfig(ctx.cwd, { projectTrusted: ctx.isProjectTrusted() });
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
		description: "Show pi-zmux context and policy, or run zmux config reload with `/zmux reload`",
		handler: async (args, ctx) => {
			const command = args.trim();
			if (command === "reload") {
				const result = await reloadZmux(ctx.cwd);
				ctx.ui.notify(result.text, "info");
				return;
			}
			if (command && command !== "status") {
				ctx.ui.notify("Usage: /zmux [status|reload]", "warning");
				return;
			}
			ctx.ui.notify(await buildContext(ctx.cwd, ctx.isProjectTrusted()), "info");
		},
	});
}
