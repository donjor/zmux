import type { ExtensionContext } from "@earendil-works/pi-coding-agent";
import { resolve } from "node:path";
import { stripQuotedSegments } from "../classify.js";
import { loadConfig } from "../config.js";
import type { LogAction, TabPlacementAction, TabPlacementDirection, TabStateAction } from "../zmux.js";

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

export function content(text: string, details: Record<string, unknown> = {}) {
	const truncated = truncateText(text);
	return {
		content: [{ type: "text" as const, text: truncated.text }],
		details: truncated.truncated ? { ...details, truncated: true, originalBytes: truncated.originalBytes, originalLines: truncated.originalLines } : details,
	};
}

export function configFor(ctx: ExtensionContext) {
	return loadConfig(ctx.cwd, { projectTrusted: ctx.isProjectTrusted() });
}

export function resolveCwd(cwd: string, maybeCwd?: string): string {
	return maybeCwd ? resolve(cwd, maybeCwd) : cwd;
}

export function shouldWaitForExit(command: string): boolean {
	const trimmed = command.trim();
	if (/^sudo\s+(-i|-s|su\b)/u.test(trimmed)) return false;
	if (/^(ssh|psql|mysql|sqlite3|redis-cli|python|node|irb|pry|bash|zsh|fish)(\s+.*)?$/u.test(trimmed) && !/^sudo\b/u.test(trimmed)) {
		return false;
	}
	return /(^|[;&|]\s*)(sudo|su)\b/u.test(trimmed);
}

const headlessAgentPrintPattern = /(^|[;&|\n]\s*)(claude|codex|pi|agy)\b[^\n;&|]*(\s-p\b|\s--print\b)/u;

export function hasHeadlessAgentPrintMode(command: string): boolean {
	return headlessAgentPrintPattern.test(stripQuotedSegments(command.trim()));
}

export function rejectHeadlessAgentPrintMode(command: string): string | undefined {
	if (!hasHeadlessAgentPrintMode(command)) return undefined;
	return "ERROR: do not launch agent peers with -p/--print. Use a visible interactive CLI in a zmux tab, then deliver prompts with zmux_type / zmux_peer_handoff.";
}

export type PaneDirection = "right" | "left" | "down" | "up";

export function paneDirection(value?: string): PaneDirection | undefined {
	if (value === undefined || value === "right" || value === "left" || value === "down" || value === "up") return value;
	throw new Error(`direction must be one of: right, left, down, up (got ${value})`);
}

export function tabStateAction(value: string): TabStateAction {
	if (value === "attention" || value === "failed" || value === "running" || value === "ready" || value === "done" || value === "clear") return value;
	throw new Error(`state must be one of: attention, failed, running, ready, done, clear (got ${value})`);
}

export function tabPlacementAction(value: string): TabPlacementAction {
	if (value === "pane" || value === "full" || value === "hide" || value === "show") return value;
	throw new Error(`action must be one of: pane, full, hide, show (got ${value})`);
}

export function tabPlacementDirection(value?: string): TabPlacementDirection | undefined {
	if (value === undefined || value === "right" || value === "left" || value === "up" || value === "down") return value;
	throw new Error(`direction must be one of: right, left, up, down (got ${value})`);
}

export function logAction(value: string): LogAction {
	if (value === "start" || value === "tail" || value === "status" || value === "stop") return value;
	throw new Error(`action must be one of: start, tail, status, stop (got ${value})`);
}

function invalidOption(action: string, option: string): Error {
	return new Error(`${option} is not valid for ${action}`);
}

export function validateLogParams(action: LogAction, params: { tab?: string; session?: string; ansi?: boolean; maxBytes?: number; lines?: number }): void {
	if (action !== "status" && !params.tab) throw new Error("tab is required for zmux_log start/tail/stop");
	if (action === "status" && params.tab) throw invalidOption("zmux_log status", "tab");
	if (action === "status" && params.session) throw invalidOption("zmux_log status", "session");
	if (action !== "start" && params.ansi === true) throw invalidOption(`zmux_log ${action}`, "ansi");
	if (action !== "start" && params.maxBytes !== undefined) throw invalidOption(`zmux_log ${action}`, "maxBytes");
	if (action !== "tail" && params.lines !== undefined) throw invalidOption(`zmux_log ${action}`, "lines");
}

export function validateTabPlacementParams(action: TabPlacementAction, params: { tab?: string; into?: string; direction?: string; size?: string; pane?: string; after?: boolean; focus?: boolean }): void {
	if (params.tab && params.pane) throw new Error("tab and pane cannot be combined");
	if (action === "pane" && !params.tab) throw new Error("tab is required for tab pane");
	if (action === "show" && !params.tab && !params.pane) throw new Error("tab or pane is required for tab show");
	if (action === "pane" && params.pane) throw invalidOption("tab pane", "pane");
	if (action !== "pane" && params.into) throw invalidOption(`tab ${action}`, "into");
	if (action !== "pane" && params.direction) throw invalidOption(`tab ${action}`, "direction");
	if (action !== "pane" && params.size) throw invalidOption(`tab ${action}`, "size");
	if (action !== "full" && params.after === true) throw invalidOption(`tab ${action}`, "after");
	if (action !== "pane" && action !== "show" && params.focus === true) throw invalidOption(`tab ${action}`, "focus");
}
