import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";
import { spawn, type ChildProcess } from "node:child_process";
import { trimOutput } from "../shell.js";
import { buildWatchArgs } from "./agent.js";
import { setTabPeer, typeText } from "./tabs.js";
import { zmuxBin } from "./shared.js";

export type CallbackDeliverAs = "steer" | "followUp" | "nextTurn";
export type CallbackAction = "watch" | "list" | "cancel";

export interface WatchCallbackParams {
	id?: string;
	tab: string;
	cwd: string;
	session?: string;
	lines?: number;
	waitFor?: string;
	idleSeconds?: number;
	timeoutSeconds?: number;
	message?: string;
	deliverAs?: CallbackDeliverAs;
	triggerTurn?: boolean;
}

export interface PeerHandoffParams {
	id?: string;
	tab: string;
	text: string;
	cwd: string;
	session?: string;
	lines?: number;
	waitFor?: string;
	idleSeconds?: number;
	timeoutSeconds?: number;
	markPeerRunning?: boolean;
	source?: string;
	message?: string;
	deliverAs?: CallbackDeliverAs;
	triggerTurn?: boolean;
}

interface CallbackHandle {
	id: string;
	kind: "watch" | "peer_handoff";
	tab: string;
	session?: string;
	startedAt: string;
	args: string[];
	proc: ChildProcess;
	message?: string;
	deliverAs: CallbackDeliverAs;
	triggerTurn: boolean;
}

interface CallbackCompletion {
	id: string;
	kind: "watch" | "peer_handoff";
	tab: string;
	session?: string;
	startedAt: string;
	finishedAt: string;
	exitCode: number | null;
	signal: NodeJS.Signals | null;
	stdout: string;
	stderr: string;
	message?: string;
}

interface CompletedCallbackRecord extends CallbackCompletion {
	status: "completed";
	deliverAs: CallbackDeliverAs;
	triggerTurn: boolean;
}

const callbacks = new Map<string, CallbackHandle>();
const completedCallbacks: CompletedCallbackRecord[] = [];
const maxCompletedCallbacks = 20;
let nextCallbackSeq = 1;

function callbackId(prefix: string, explicit?: string): string {
	const clean = explicit?.trim();
	if (clean) return clean;
	return `${prefix}-${Date.now()}-${nextCallbackSeq++}`;
}

function capBuffer(value: string, max = 24_000): string {
	if (value.length <= max) return value;
	return value.slice(value.length - max);
}

export function buildCallbackWatchArgs(params: { tab: string; session?: string; lines?: number; waitFor?: string; idleSeconds?: number; timeoutSeconds?: number }): string[] {
	return buildWatchArgs({
		tab: params.tab,
		session: params.session,
		lines: params.lines ?? 160,
		waitFor: params.waitFor,
		idleSeconds: params.idleSeconds,
		timeoutSeconds: params.timeoutSeconds ?? 300,
	});
}

export function formatCallbackMessage(done: CallbackCompletion): string {
	const ok = done.exitCode === 0;
	const heading = ok ? `zmux callback ${done.id} completed for ${done.tab}` : `zmux callback ${done.id} finished unproven for ${done.tab}`;
	const output = trimOutput([done.stdout, done.stderr ? `stderr:\n${done.stderr}` : ""].filter(Boolean).join("\n"));
	return trimOutput([
		heading,
		done.message ? `message: ${done.message}` : "",
		`session: ${done.session ?? "current"}`,
		`exit: ${done.exitCode ?? "signal"}${done.signal ? ` (${done.signal})` : ""}`,
		output ? `output:\n${output}` : "output: <empty>",
	].filter(Boolean).join("\n"));
}

function sendCallbackMessage(pi: ExtensionAPI, done: CallbackCompletion, deliverAs: CallbackDeliverAs, triggerTurn: boolean): void {
	rememberCallbackCompletion(done, deliverAs, triggerTurn);
	pi.sendMessage({
		customType: "pi-zmux-callback",
		content: formatCallbackMessage(done),
		display: true,
		details: {
			kind: "zmux_callback",
			id: done.id,
			callbackKind: done.kind,
			tab: done.tab,
			session: done.session,
			startedAt: done.startedAt,
			finishedAt: done.finishedAt,
			exitCode: done.exitCode,
			signal: done.signal,
			message: done.message,
		},
	}, { deliverAs, triggerTurn });
}

export function listCallbacks(): Array<Record<string, unknown>> {
	return [...callbacks.values()].map((handle) => ({
		id: handle.id,
		status: "active",
		kind: handle.kind,
		tab: handle.tab,
		session: handle.session,
		startedAt: handle.startedAt,
		args: handle.args,
		message: handle.message,
		deliverAs: handle.deliverAs,
		triggerTurn: handle.triggerTurn,
	}));
}

export function listRecentCallbackCompletions(): Array<Record<string, unknown>> {
	return completedCallbacks.map((completion) => ({
		id: completion.id,
		status: completion.status,
		kind: completion.kind,
		tab: completion.tab,
		session: completion.session,
		startedAt: completion.startedAt,
		finishedAt: completion.finishedAt,
		exitCode: completion.exitCode,
		signal: completion.signal,
		message: completion.message,
		deliverAs: completion.deliverAs,
		triggerTurn: completion.triggerTurn,
	}));
}

export function findRecentCallbackCompletion(id: string): Record<string, unknown> | undefined {
	return listRecentCallbackCompletions().find((completion) => completion.id === id);
}

function rememberCallbackCompletion(done: CallbackCompletion, deliverAs: CallbackDeliverAs, triggerTurn: boolean): void {
	completedCallbacks.unshift({ ...done, status: "completed", deliverAs, triggerTurn });
	completedCallbacks.splice(maxCompletedCallbacks);
}

export function cancelCallback(id: string): boolean {
	const handle = callbacks.get(id);
	if (!handle) return false;
	callbacks.delete(id);
	handle.proc.kill("SIGTERM");
	return true;
}

export function clearCallbacks(): void {
	for (const id of [...callbacks.keys()]) cancelCallback(id);
	completedCallbacks.length = 0;
}

export function startWatchCallback(pi: ExtensionAPI, params: WatchCallbackParams, kind: "watch" | "peer_handoff" = "watch"): { text: string; details: Record<string, unknown> } {
	if (params.waitFor && params.idleSeconds !== undefined) {
		throw new Error("waitFor and idleSeconds cannot be combined");
	}
	if (!params.waitFor && params.idleSeconds === undefined) {
		throw new Error("callback watch requires waitFor or idleSeconds");
	}
	const id = callbackId(kind === "peer_handoff" ? "peer-handoff" : "callback", params.id);
	if (callbacks.has(id)) throw new Error(`callback id already exists: ${id}`);
	const args = buildCallbackWatchArgs(params);
	const startedAt = new Date().toISOString();
	const proc = spawn(zmuxBin(), args, {
		cwd: params.cwd,
		env: process.env,
		stdio: ["ignore", "pipe", "pipe"],
	});
	const deliverAs = params.deliverAs ?? "steer";
	const triggerTurn = params.triggerTurn ?? true;
	const handle: CallbackHandle = {
		id,
		kind,
		tab: params.tab,
		session: params.session,
		startedAt,
		args,
		proc,
		message: params.message,
		deliverAs,
		triggerTurn,
	};
	callbacks.set(id, handle);
	let stdout = "";
	let stderr = "";
	proc.stdout.on("data", (chunk) => {
		stdout = capBuffer(stdout + String(chunk));
	});
	proc.stderr.on("data", (chunk) => {
		stderr = capBuffer(stderr + String(chunk));
	});
	proc.on("close", (exitCode, signal) => {
		if (!callbacks.delete(id)) return;
		sendCallbackMessage(pi, {
			id,
			kind,
			tab: params.tab,
			session: params.session,
			startedAt,
			finishedAt: new Date().toISOString(),
			exitCode,
			signal,
			stdout: trimOutput(stdout),
			stderr: trimOutput(stderr),
			message: params.message,
		}, deliverAs, triggerTurn);
	});
	proc.on("error", (error) => {
		if (!callbacks.delete(id)) return;
		sendCallbackMessage(pi, {
			id,
			kind,
			tab: params.tab,
			session: params.session,
			startedAt,
			finishedAt: new Date().toISOString(),
			exitCode: null,
			signal: null,
			stdout,
			stderr: error instanceof Error ? error.message : String(error),
			message: params.message,
		}, deliverAs, triggerTurn);
	});
	return {
		text: `scheduled zmux ${kind === "peer_handoff" ? "peer handoff" : "callback"} ${id} for ${params.tab}`,
		details: { id, kind, tab: params.tab, session: params.session, args, startedAt, deliverAs, triggerTurn, message: params.message },
	};
}

export async function startPeerHandoff(pi: ExtensionAPI, params: PeerHandoffParams): Promise<{ text: string; details: Record<string, unknown> }> {
	const idleSeconds = params.waitFor ? params.idleSeconds : (params.idleSeconds ?? 3);
	const callbackParams: WatchCallbackParams = {
		id: params.id,
		tab: params.tab,
		cwd: params.cwd,
		session: params.session,
		lines: params.lines ?? 200,
		waitFor: params.waitFor,
		idleSeconds,
		timeoutSeconds: params.timeoutSeconds ?? 300,
		message: params.message ?? "peer handoff ready",
		deliverAs: params.deliverAs,
		triggerTurn: params.triggerTurn,
	};
	if (params.markPeerRunning) {
		await setTabPeer({ action: "running", tab: params.tab, session: params.session, source: params.source ?? "pi-zmux-handoff", msg: params.message, cwd: params.cwd });
	}
	let callback: { text: string; details: Record<string, unknown> } | undefined;
	if (params.waitFor) {
		callback = startWatchCallback(pi, callbackParams, "peer_handoff");
	}
	const typed = await typeText(params.tab, params.text, params.cwd, params.session);
	if (!callback) {
		callback = startWatchCallback(pi, callbackParams, "peer_handoff");
	}
	return {
		text: trimOutput([typed.text, callback.text].join("\n")),
		details: { ...typed.details, callback: callback.details, markPeerRunning: params.markPeerRunning ?? false, waitFor: params.waitFor, idleSeconds },
	};
}
