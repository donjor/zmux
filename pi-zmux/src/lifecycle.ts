import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";
import { currentPane } from "./zmux/context.js";
import { setTabPeer, setTabState } from "./zmux/tabs.js";

type LifecycleState = "running" | "ready" | "clear";
type LifecycleSurface = "glyph" | "peer-turn";

type LifecycleOptions = {
	cwd: string;
	state: LifecycleState;
	surface: LifecycleSurface;
	message?: string;
	notify?: (message: string) => void;
};

async function currentPaneId(cwd: string): Promise<string | undefined> {
	const pane = await currentPane(cwd);
	return pane?.ID;
}

export async function setCurrentPaneLifecycle(options: LifecycleOptions): Promise<boolean> {
	try {
		const paneId = await currentPaneId(options.cwd);
		if (!paneId) return false;
		if (options.surface === "peer-turn") {
			if (options.state === "clear") {
				await setTabState({ action: "clear", target: paneId, cwd: options.cwd, source: "pi-agent", ifState: "running" });
				return true;
			}
			await setTabPeer({
				action: options.state,
				target: paneId,
				cwd: options.cwd,
				source: "pi-agent",
				msg: options.message,
			});
			return true;
		}
		await setTabState({
			action: options.state,
			target: paneId,
			cwd: options.cwd,
			source: "pi-agent",
			msg: options.message,
			ifState: options.state === "clear" ? "running" : undefined,
		});
		return true;
	} catch (error) {
		options.notify?.(`pi-zmux lifecycle ${options.state} failed: ${error instanceof Error ? error.message : String(error)}`);
		return false;
	}
}

export function registerGlyphLifecycle(pi: ExtensionAPI): void {
	pi.on("agent_start", async (_event, ctx) => {
		await setCurrentPaneLifecycle({ cwd: ctx.cwd, state: "running", surface: "glyph" });
	});

	pi.on("agent_end", async (_event, ctx) => {
		await setCurrentPaneLifecycle({ cwd: ctx.cwd, state: "ready", surface: "glyph", message: "Pi ready" });
	});

	pi.on("session_shutdown", async (_event, ctx) => {
		await setCurrentPaneLifecycle({ cwd: ctx.cwd, state: "clear", surface: "glyph" });
	});
}

export function registerPeerLifecycle(pi: ExtensionAPI): void {
	pi.on("agent_start", async (_event, ctx) => {
		await setCurrentPaneLifecycle({
			cwd: ctx.cwd,
			state: "running",
			surface: "peer-turn",
			message: "Pi peer running",
			notify: (message) => ctx.ui.notify(message, "warning"),
		});
	});

	pi.on("agent_end", async (_event, ctx) => {
		await setCurrentPaneLifecycle({
			cwd: ctx.cwd,
			state: "ready",
			surface: "peer-turn",
			message: "Pi peer ready",
			notify: (message) => ctx.ui.notify(message, "warning"),
		});
	});

	pi.on("session_shutdown", async (_event, ctx) => {
		await setCurrentPaneLifecycle({ cwd: ctx.cwd, state: "clear", surface: "peer-turn" });
	});
}
