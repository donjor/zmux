import { trimOutput } from "../shell.js";
import { tmux, zmux } from "./shared.js";

type PaneRow = {
	ID?: string;
	Index?: number;
	Title?: string;
};

type PaneResizeAxis = "width" | "height";

type PaneDimensions = {
	paneWidth: number;
	paneHeight: number;
	windowWidth: number;
	windowHeight: number;
};

function isRawPaneTarget(pane: string): boolean {
	return pane.startsWith("%") || pane.includes(":") || pane.includes(".");
}

async function resolvePaneTarget(pane: string, cwd: string): Promise<string> {
	if (isRawPaneTarget(pane)) return pane;
	let rows: PaneRow[] = [];
	try {
		const result = await zmux(["pane", "list", "--json"], { cwd, timeoutMs: 5_000 });
		const parsed = JSON.parse(result.stdout || "[]") as unknown;
		if (Array.isArray(parsed)) rows = parsed as PaneRow[];
	} catch {
		return pane;
	}
	const matches = rows.filter((row) => row.Title === pane || String(row.Index) === pane);
	if (matches.length === 0) return pane;
	if (matches.length > 1) throw new Error(`pane ${pane} is ambiguous (${matches.length} matches); use a pane id`);
	return matches[0]?.ID || pane;
}

export async function sendPaneKeys(pane: string, keys: string[], cwd: string): Promise<{ text: string; details: Record<string, unknown> }> {
	const target = await resolvePaneTarget(pane, cwd);
	await tmux(["send-keys", "-t", target, ...keys], { cwd, timeoutMs: 5_000 });
	return { text: `sent keys to pane ${pane}: ${keys.join(" ")}`, details: { pane, target, keys } };
}

export async function typePaneText(pane: string, text: string, cwd: string): Promise<{ text: string; details: Record<string, unknown> }> {
	const target = await resolvePaneTarget(pane, cwd);
	await tmux(["send-keys", "-t", target, "-l", text], { cwd, timeoutMs: 5_000 });
	await tmux(["send-keys", "-t", target, "Enter"], { cwd, timeoutMs: 5_000 });
	return { text: `typed text into pane ${pane}`, details: { pane, target, text } };
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

export function buildPaneOpenArgs(params: { name: string; command: string; cwd: string; direction?: "right" | "left" | "down" | "up"; size?: string; target?: string; labelTab?: boolean; focus?: boolean }): string[] {
	const args = ["pane", "open", params.name, "--cwd", params.cwd];
	if (params.target) args.push("--target", params.target);
	const directionFlag = params.direction ? ({ right: "-r", left: "-l", down: "-d", up: "-u" } as const)[params.direction] : "-r";
	args.push(directionFlag);
	if (params.size) args.push(params.size);
	if (params.labelTab) args.push("--label-tab");
	if (!params.focus) args.push("--no-focus");
	args.push("--", "bash", "-lc", params.command);
	return args;
}

export async function openPane(params: { name: string; command: string; cwd: string; direction?: "right" | "left" | "down" | "up"; size?: string; target?: string; labelTab?: boolean; focus?: boolean }): Promise<{ text: string; details: Record<string, unknown> }> {
	await zmux(buildPaneOpenArgs(params), { cwd: params.cwd, timeoutMs: 10_000 });
	return { text: `opened pane ${params.name}${params.focus ? " and focused it" : " without changing focus"}`, details: { ...params, focus: params.focus ?? false } };
}

export async function focusPane(pane: string, cwd: string): Promise<{ text: string; details: Record<string, unknown> }> {
	await zmux(["pane", "focus", pane], { cwd, timeoutMs: 5_000 });
	return { text: `focused pane ${pane}`, details: { pane } };
}

export async function closePane(pane: string, cwd: string): Promise<{ text: string; details: Record<string, unknown> }> {
	await zmux(["pane", "close", pane], { cwd, timeoutMs: 5_000 });
	return { text: `closed pane ${pane}`, details: { pane } };
}

async function paneDimensions(target: string, cwd: string): Promise<PaneDimensions | undefined> {
	try {
		const result = await tmux(["display-message", "-p", "-t", target, "#{pane_width} #{pane_height} #{window_width} #{window_height}"], { cwd, timeoutMs: 5_000 });
		const [paneWidth, paneHeight, windowWidth, windowHeight] = result.stdout.trim().split(/\s+/u).map((value) => Number.parseInt(value, 10));
		if ([paneWidth, paneHeight, windowWidth, windowHeight].some((value) => !Number.isFinite(value))) return undefined;
		return { paneWidth, paneHeight, windowWidth, windowHeight };
	} catch {
		return undefined;
	}
}

export function autoPaneResizeAxis(dimensions: PaneDimensions | undefined): PaneResizeAxis {
	if (dimensions && dimensions.paneWidth >= dimensions.windowWidth && dimensions.paneHeight < dimensions.windowHeight) return "height";
	return "width";
}

export async function resizePane(pane: string, cwd: string, size: string, axis?: PaneResizeAxis | "auto"): Promise<{ text: string; details: Record<string, unknown> }> {
	const target = await resolvePaneTarget(pane, cwd);
	const dimensions = await paneDimensions(target, cwd);
	const selectedAxis = !axis || axis === "auto" ? autoPaneResizeAxis(dimensions) : axis;
	const flag = selectedAxis === "height" ? "--height" : "--width";
	await zmux(["pane", "resize", target, flag, size], { cwd, timeoutMs: 5_000 });
	return { text: `resized pane ${pane} ${selectedAxis} to ${size}`, details: { pane, target, size, axis: selectedAxis, dimensions } };
}
