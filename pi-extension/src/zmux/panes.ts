import { trimOutput } from "../shell.js";
import { tmux, zmux } from "./shared.js";

export async function sendPaneKeys(pane: string, keys: string[], cwd: string): Promise<{ text: string; details: Record<string, unknown> }> {
	await tmux(["send-keys", "-t", pane, ...keys], { cwd, timeoutMs: 5_000 });
	return { text: `sent keys to pane ${pane}: ${keys.join(" ")}`, details: { pane, keys } };
}

export async function typePaneText(pane: string, text: string, cwd: string): Promise<{ text: string; details: Record<string, unknown> }> {
	await tmux(["send-keys", "-t", pane, text, "Enter"], { cwd, timeoutMs: 5_000 });
	return { text: `typed text into pane ${pane}`, details: { pane, text } };
}

export async function listPanes(cwd: string, session?: string): Promise<string> {
	try {
		const args = session ? ["pane", "list", "--session", "--target", session] : ["pane", "list"];
		const result = await zmux(args, { cwd, timeoutMs: 5_000 });
		return trimOutput(result.stdout);
	} catch (error) {
		return `unavailable: ${error instanceof Error ? error.message : String(error)}`;
	}
}

export function buildPaneOpenArgs(params: { name: string; command: string; cwd: string; direction?: "right" | "left" | "down" | "up"; size?: string; target?: string; labelTab?: boolean }): string[] {
	const args = ["pane", "open", params.name, "--cwd", params.cwd];
	if (params.target) args.push("--target", params.target);
	const directionFlag = params.direction ? ({ right: "-r", left: "-l", down: "-d", up: "-u" } as const)[params.direction] : "-r";
	args.push(directionFlag);
	if (params.size) args.push(params.size);
	if (params.labelTab) args.push("--label-tab");
	args.push("--", "bash", "-lc", params.command);
	return args;
}

export async function openPane(params: { name: string; command: string; cwd: string; direction?: "right" | "left" | "down" | "up"; size?: string; target?: string; labelTab?: boolean }): Promise<{ text: string; details: Record<string, unknown> }> {
	await zmux(buildPaneOpenArgs(params), { cwd: params.cwd, timeoutMs: 10_000 });
	return { text: `opened pane ${params.name}`, details: { ...params } };
}

export async function focusPane(pane: string, cwd: string): Promise<{ text: string; details: Record<string, unknown> }> {
	await zmux(["pane", "focus", pane], { cwd, timeoutMs: 5_000 });
	return { text: `focused pane ${pane}`, details: { pane } };
}

export async function closePane(pane: string, cwd: string): Promise<{ text: string; details: Record<string, unknown> }> {
	await zmux(["pane", "close", pane], { cwd, timeoutMs: 5_000 });
	return { text: `closed pane ${pane}`, details: { pane } };
}

export async function resizePane(pane: string, cwd: string, size: string): Promise<{ text: string; details: Record<string, unknown> }> {
	await zmux(["pane", "resize", pane, "--size", size], { cwd, timeoutMs: 5_000 });
	return { text: `resized pane ${pane} to ${size}`, details: { pane, size } };
}
