import { trimOutput } from "../shell.js";
import { safeJson, withSession, zmux } from "./shared.js";

export async function killTab(tab: string, cwd: string): Promise<{ text: string; details: Record<string, unknown> }> {
	await zmux(["tab", "kill", tab], { cwd, timeoutMs: 10_000 });
	return { text: `killed tab ${tab}`, details: { tab } };
}

export async function sendKeys(tab: string, keys: string[], cwd: string, session?: string): Promise<{ text: string; details: Record<string, unknown> }> {
	await zmux(withSession(["send", tab, ...keys], session), { cwd, timeoutMs: 5_000 });
	return { text: `sent keys to ${tab}: ${keys.join(" ")}`, details: { tab, keys, session } };
}

export async function typeText(tab: string, text: string, cwd: string, session?: string): Promise<{ text: string; details: Record<string, unknown> }> {
	await zmux(withSession(["type", tab, text], session), { cwd, timeoutMs: 5_000 });
	return { text: `typed text into ${tab}`, details: { tab, text, session } };
}

export type TabStateAction = "attention" | "failed" | "running" | "ready" | "done" | "clear";
export type TabPeerAction = "start" | "running" | "ready" | "waiting" | "attention" | "failed" | "consumed" | "park" | "keep" | "clear-keep";
export type TabPlacementAction = "pane" | "full" | "hide" | "show";
export type TabPlacementDirection = "right" | "left" | "up" | "down";
export type LogAction = "start" | "tail" | "status" | "stop";

export function buildTabStateArgs(params: { action: TabStateAction; tab?: string; target?: string; session?: string; source?: string; msg?: string; ifState?: string; byVisibility?: boolean }): string[] {
	const args = ["tab", "state", params.action];
	if (params.tab) args.push(params.tab);
	if (params.target) args.push("--target", params.target);
	if (params.source) args.push("--source", params.source);
	if (params.msg) args.push("--msg", params.msg);
	if (params.ifState) args.push("--if", params.ifState);
	if (params.byVisibility) args.push("--by-visibility");
	return withSession(args, params.session);
}

export async function setTabState(params: { action: TabStateAction; cwd: string; tab?: string; target?: string; session?: string; source?: string; msg?: string; ifState?: string; byVisibility?: boolean }): Promise<{ text: string; details: Record<string, unknown> }> {
	await zmux(buildTabStateArgs(params), { cwd: params.cwd, timeoutMs: 5_000 });
	return { text: `tab state ${params.action}${params.tab ? ` ${params.tab}` : ""}`, details: { ...params } };
}

export function buildTabPeerArgs(params: { action: TabPeerAction; tab?: string; target?: string; session?: string; role?: string; hostTab?: string; hostPane?: string; topic?: string; ttl?: string; source?: string; msg?: string }): string[] {
	const args = ["tab", "peer", params.action];
	if (params.tab) args.push(params.tab);
	if (params.target) args.push("--target", params.target);
	if (params.role) args.push("--role", params.role);
	if (params.hostTab) args.push("--host-tab", params.hostTab);
	if (params.hostPane) args.push("--host-pane", params.hostPane);
	if (params.topic) args.push("--topic", params.topic);
	if (params.ttl) args.push("--ttl", params.ttl);
	if (params.source) args.push("--source", params.source);
	if (params.msg) args.push("--msg", params.msg);
	return withSession(args, params.session);
}

export async function setTabPeer(params: { action: TabPeerAction; cwd: string; tab?: string; target?: string; session?: string; role?: string; hostTab?: string; hostPane?: string; topic?: string; ttl?: string; source?: string; msg?: string }): Promise<{ text: string; details: Record<string, unknown> }> {
	await zmux(buildTabPeerArgs(params), { cwd: params.cwd, timeoutMs: 5_000 });
	return { text: `tab peer ${params.action}${params.tab ? ` ${params.tab}` : ""}`, details: { ...params } };
}

export function buildTabStatusArgs(params: { tab: string; session?: string }): string[] {
	return withSession(["tab", "status", params.tab, "--json"], params.session);
}

export async function tabStatus(params: { cwd: string; tab: string; session?: string }): Promise<{ text: string; details: Record<string, unknown> }> {
	const result = await zmux(buildTabStatusArgs(params), { cwd: params.cwd, timeoutMs: 5_000 });
	const output = trimOutput(result.stdout || result.stderr);
	let details: Record<string, unknown> = { ...params };
	try {
		details = { ...params, status: safeJson(output) };
	} catch {
		// Keep text output when an older binary lacks JSON or returns plain errors.
	}
	return { text: output || `tab status ${params.tab}`, details };
}

export function buildTabLabelArgs(params: { label?: string; target?: string; clear?: boolean }): string[] {
	const args = ["tab", "label"];
	if (params.target) args.push("--target", params.target);
	if (params.clear) args.push("--clear");
	if (params.label !== undefined) args.push(params.label);
	return args;
}

export async function labelTab(params: { cwd: string; label?: string; target?: string; clear?: boolean }): Promise<{ text: string; details: Record<string, unknown> }> {
	const result = await zmux(buildTabLabelArgs(params), { cwd: params.cwd, timeoutMs: 5_000 });
	const output = trimOutput(result.stdout || result.stderr);
	return { text: output || (params.clear ? "cleared tab label" : `set tab label ${params.label ?? ""}`), details: { ...params } };
}

export function buildTabMoveArgs(params: { tab: string; destination: string; force?: boolean }): string[] {
	const args = ["tab", "move", params.tab, params.destination];
	if (params.force) args.push("--force");
	return args;
}

export async function moveTab(params: { cwd: string; tab: string; destination: string; force?: boolean }): Promise<{ text: string; details: Record<string, unknown> }> {
	const result = await zmux(buildTabMoveArgs(params), { cwd: params.cwd, timeoutMs: 10_000 });
	const output = trimOutput([result.stdout, result.stderr].filter(Boolean).join("\n"));
	return { text: output || `moved tab ${params.tab} to ${params.destination}`, details: { ...params } };
}

export function buildLogArgs(params: { action: LogAction; tab?: string; session?: string; ansi?: boolean; maxBytes?: number; lines?: number }): string[] {
	const args = ["log", params.action];
	if (params.action === "status") return args;
	if (params.tab) args.push(params.tab);
	if (params.action === "start") {
		if (params.ansi) args.push("--ansi");
		if (params.maxBytes !== undefined) args.push("--max-bytes", String(params.maxBytes));
	}
	if (params.action === "tail" && params.lines !== undefined) args.push("-n", String(params.lines));
	if (params.session) args.push("-s", params.session);
	return args;
}

export async function logCommand(params: { action: LogAction; cwd: string; tab?: string; session?: string; ansi?: boolean; maxBytes?: number; lines?: number }): Promise<{ text: string; details: Record<string, unknown> }> {
	const result = await zmux(buildLogArgs(params), { cwd: params.cwd, timeoutMs: 10_000 });
	return { text: trimOutput([result.stdout, result.stderr].filter(Boolean).join("\n")) || `zmux log ${params.action}`, details: { ...params } };
}

export function buildSnapshotArgs(params: { noPng?: boolean; panes?: string[]; lines?: number; out?: string; json?: boolean }): string[] {
	const args = ["snapshot"];
	if (params.noPng) args.push("--no-png");
	for (const pane of params.panes ?? []) args.push("--pane", pane);
	if (params.lines !== undefined) args.push("--lines", String(params.lines));
	if (params.out) args.push("--out", params.out);
	if (params.json) args.push("--json");
	return args;
}

export async function snapshot(params: { cwd: string; noPng?: boolean; panes?: string[]; lines?: number; out?: string; json?: boolean }): Promise<{ text: string; details: Record<string, unknown> }> {
	const result = await zmux(buildSnapshotArgs(params), { cwd: params.cwd, timeoutMs: 30_000 });
	return { text: trimOutput([result.stdout, result.stderr].filter(Boolean).join("\n")) || "snapshot captured", details: { ...params } };
}

export function buildTabPlacementArgs(params: { action: TabPlacementAction; tab?: string; session?: string; into?: string; direction?: TabPlacementDirection; size?: string; pane?: string; after?: boolean; focus?: boolean }): string[] {
	const args = ["tab", params.action];
	if (params.tab) args.push(params.tab);
	if (params.session) args.push("--session", params.session);
	if (params.action === "pane") {
		if (params.into) args.push("--into", params.into);
		if (params.direction) args.push(`--${params.direction}`);
		if (params.size) args.push("--size", params.size);
		if (params.focus) args.push("--focus");
		return args;
	}
	if (params.pane) args.push("--pane", params.pane);
	if (params.action === "show" && params.focus) args.push("--focus");
	if (params.action === "full" && params.after) args.push("--after");
	return args;
}

export async function placeTab(params: { action: TabPlacementAction; cwd: string; tab?: string; session?: string; into?: string; direction?: TabPlacementDirection; size?: string; pane?: string; after?: boolean; focus?: boolean }): Promise<{ text: string; details: Record<string, unknown> }> {
	const result = await zmux(buildTabPlacementArgs(params), { cwd: params.cwd, timeoutMs: 10_000 });
	return { text: trimOutput([result.stdout, result.stderr].filter(Boolean).join("\n")) || `tab ${params.action}`, details: { ...params, focus: params.focus ?? false } };
}

export async function terminalCurrent(cwd: string): Promise<{ text: string; details: Record<string, unknown> }> {
	const result = await zmux(["terminal", "current", "--json"], { cwd, timeoutMs: 5_000 });
	return { text: trimOutput(result.stdout), details: { terminal: safeJson(result.stdout) } };
}
