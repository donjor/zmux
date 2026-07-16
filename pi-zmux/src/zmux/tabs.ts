import { withSession, zmux } from "./shared.js";

// This module is arg-builder-canonical: every `build*Args` is the single source
// of truth for its CLI mapping, consumed by the dispatcher (src/dispatcher.ts)
// and unit-pinned in test/run.mjs. Only the three lifecycle wrappers still run
// commands directly (setTabState/setTabPeer/typeText, used by lifecycle.ts and
// callbacks.ts); the rest of the tab surface routes through the dispatcher.

export function buildTabKillArgs(params: { tab: string; session?: string }): string[] {
	return withSession(["tab", "kill", params.tab], params.session);
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

export function buildTabLabelArgs(params: { label?: string; target?: string; clear?: boolean }): string[] {
	const args = ["tab", "label"];
	if (params.target) args.push("--target", params.target);
	if (params.clear) args.push("--clear");
	if (params.label !== undefined) args.push(params.label);
	return args;
}

export function buildTabMoveArgs(params: { tab: string; destination: string; force?: boolean; session?: string }): string[] {
	const args = ["tab", "move", params.tab, params.destination];
	if (params.force) args.push("--force");
	return withSession(args, params.session);
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

export function buildSnapshotArgs(params: { noPng?: boolean; panes?: string[]; lines?: number; out?: string; json?: boolean }): string[] {
	const args = ["snapshot"];
	if (params.noPng) args.push("--no-png");
	for (const pane of params.panes ?? []) args.push("--pane", pane);
	if (params.lines !== undefined) args.push("--lines", String(params.lines));
	if (params.out) args.push("--out", params.out);
	if (params.json) args.push("--json");
	return args;
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
