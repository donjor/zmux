import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";
import { spawn, type ChildProcess } from "node:child_process";
import { trimOutput } from "../shell.js";
import { summarizeWaitOutput } from "../wait-summary.js";
import { buildWaitArgs } from "./agent.js";
import { setTabPeer, typeText } from "./tabs.js";
import { zmuxBin } from "./shared.js";

export type CallbackDeliverAs = "steer" | "followUp" | "nextTurn";
export type CallbackAction = "watch" | "list" | "cancel";

export interface CallbackActivitySink {
	set(text: string | undefined): void;
}

export interface WatchCallbackParams {
	id?: string;
	tab: string;
	cwd: string;
	session?: string;
	lines?: number;
	waitFor?: string;
	idleSeconds?: number;
	turnState?: string;
	timeoutSeconds?: number;
	message?: string;
	deliverAs?: CallbackDeliverAs;
	triggerTurn?: boolean;
	activitySink?: CallbackActivitySink;
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
	activitySink?: CallbackActivitySink;
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
	condition: string;
	deadlineAt: number;
	activitySink?: CallbackActivitySink;
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
	basis?: string;
	failureKind?: string;
	alreadyInTail?: boolean;
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
let activityInterval: ReturnType<typeof setInterval> | undefined;
const renderedActivitySinks = new Set<CallbackActivitySink>();
const activityFrames = ["◐", "◓", "◑", "◒"];

function callbackCondition(params: Pick<WatchCallbackParams, "waitFor" | "idleSeconds" | "turnState">): string {
	if (params.turnState) return `waiting for ${params.turnState}`;
	if (params.waitFor) return `waiting for /${params.waitFor}/`;
	return `waiting for ${params.idleSeconds}s idle`;
}

function formatRemaining(deadlineAt: number): string {
	const seconds = Math.max(0, Math.ceil((deadlineAt - Date.now()) / 1000));
	if (seconds < 60) return `${seconds}s`;
	return `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, "0")}`;
}

function setCallbackActivity(sink: CallbackActivitySink, text: string | undefined): boolean {
	try {
		sink.set(text);
		return true;
	} catch {
		return false;
	}
}

function refreshCallbackActivity(): void {
	const grouped = new Map<CallbackActivitySink, CallbackHandle[]>();
	for (const handle of callbacks.values()) {
		if (!handle.activitySink) continue;
		const group = grouped.get(handle.activitySink) ?? [];
		group.push(handle);
		grouped.set(handle.activitySink, group);
	}
	for (const sink of renderedActivitySinks) {
		if (!grouped.has(sink)) {
			setCallbackActivity(sink, undefined);
			renderedActivitySinks.delete(sink);
		}
	}
	const frame = activityFrames[Math.floor(Date.now() / 1_000) % activityFrames.length];
	for (const [sink, handles] of grouped) {
		const nearest = handles.reduce((best, handle) => handle.deadlineAt < best.deadlineAt ? handle : best);
		const text = handles.length === 1
			? `${frame} pi-zmux · ${nearest.tab} · ${nearest.condition} · ${formatRemaining(nearest.deadlineAt)}`
			: `${frame} pi-zmux · ${handles.length} waits · nearest ${formatRemaining(nearest.deadlineAt)}`;
		if (setCallbackActivity(sink, text)) renderedActivitySinks.add(sink);
	}
	if (grouped.size > 0 && !activityInterval) activityInterval = setInterval(refreshCallbackActivity, 1_000);
	if (grouped.size === 0 && activityInterval) {
		clearInterval(activityInterval);
		activityInterval = undefined;
	}
}

function callbackId(prefix: string, explicit?: string): string {
	const clean = explicit?.trim();
	if (clean) return clean;
	return `${prefix}-${Date.now()}-${nextCallbackSeq++}`;
}

function capBuffer(value: string, max = 24_000): string {
	if (value.length <= max) return value;
	return value.slice(value.length - max);
}

function callbackOutcomeFields(stdout: string): { basis?: string; failureKind?: string; alreadyInTail?: boolean } {
	try {
		const parsed = JSON.parse(stdout) as { outcome?: { basis?: unknown; failureKind?: unknown; alreadyInTail?: unknown } };
		return {
			basis: typeof parsed.outcome?.basis === "string" ? parsed.outcome.basis : undefined,
			failureKind: typeof parsed.outcome?.failureKind === "string" ? parsed.outcome.failureKind : undefined,
			alreadyInTail: typeof parsed.outcome?.alreadyInTail === "boolean" ? parsed.outcome.alreadyInTail : undefined,
		};
	} catch {
		return {};
	}
}

export function buildCallbackWatchArgs(params: { tab: string; session?: string; lines?: number; waitFor?: string; idleSeconds?: number; turnState?: string; timeoutSeconds?: number }): string[] {
	return buildWaitArgs({
		tab: params.tab,
		session: params.session,
		lines: params.lines ?? 160,
		waitFor: params.waitFor,
		idleSeconds: params.idleSeconds,
		turnState: params.turnState,
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
		done.basis ? `basis: ${done.basis}` : "basis: unavailable",
		done.failureKind ? `failure: ${done.failureKind}` : "",
		done.alreadyInTail ? "alreadyInTail: true" : "",
		`exit: ${done.exitCode ?? "signal"}${done.signal ? ` (${done.signal})` : ""}`,
		output ? `output:\n${output}` : "output: <empty>",
	].filter(Boolean).join("\n"));
}

function sendCallbackMessage(pi: ExtensionAPI, done: CallbackCompletion, deliverAs: CallbackDeliverAs, triggerTurn: boolean): void {
	rememberCallbackCompletion(done, deliverAs, triggerTurn);
	const summary = summarizeWaitOutput(done.stdout);
	const compactSuccess = done.exitCode === 0 && summary.details.waitMet !== undefined;
	pi.sendMessage({
		customType: "pi-zmux-callback",
		content: compactSuccess ? summary.text : formatCallbackMessage(done),
		display: true,
		details: {
			kind: "callback_watch",
			id: done.id,
			callbackKind: done.kind,
			tab: done.tab,
			session: done.session,
			startedAt: done.startedAt,
			finishedAt: done.finishedAt,
			exitCode: done.exitCode,
			signal: done.signal,
			message: done.message,
			basis: done.basis,
			failureKind: done.failureKind,
			alreadyInTail: done.alreadyInTail,
			rawOutput: done.stdout,
			stderr: done.stderr,
			...summary.details,
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
		condition: handle.condition,
		deadlineAt: new Date(handle.deadlineAt).toISOString(),
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
		basis: completion.basis,
		failureKind: completion.failureKind,
		alreadyInTail: completion.alreadyInTail,
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
	refreshCallbackActivity();
	return true;
}

export function clearCallbacks(): void {
	for (const id of [...callbacks.keys()]) cancelCallback(id);
	completedCallbacks.length = 0;
	refreshCallbackActivity();
}

export function startWatchCallback(pi: ExtensionAPI, params: WatchCallbackParams, kind: "watch" | "peer_handoff" = "watch"): { text: string; details: Record<string, unknown> } {
	const conditions = [params.waitFor !== undefined, params.idleSeconds !== undefined, params.turnState !== undefined].filter(Boolean).length;
	if (conditions !== 1) {
		throw new Error("callback watch requires exactly one of waitFor, idleSeconds, or turnState");
	}
	const id = callbackId(kind === "peer_handoff" ? "peer-handoff" : "callback", params.id);
	if (callbacks.has(id)) throw new Error(`callback id already exists: ${id}`);
	const args = buildCallbackWatchArgs(params);
	const deliverAs = params.deliverAs ?? (kind === "peer_handoff" ? "followUp" : "steer");
	const triggerTurn = params.triggerTurn ?? true;
	if (deliverAs === "nextTurn" && triggerTurn) {
		throw new Error('deliverAs "nextTurn" never triggers a turn; use "steer" or "followUp", or set triggerTurn false');
	}
	const startedAt = new Date().toISOString();
	const timeoutSeconds = params.timeoutSeconds ?? 300;
	const condition = callbackCondition(params);
	const deadlineAt = Date.now() + timeoutSeconds * 1000;
	const proc = spawn(zmuxBin(), args, {
		cwd: params.cwd,
		env: process.env,
		stdio: ["ignore", "pipe", "pipe"],
	});
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
		condition,
		deadlineAt,
		activitySink: params.activitySink,
	};
	callbacks.set(id, handle);
	refreshCallbackActivity();
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
		refreshCallbackActivity();
		const outcome = callbackOutcomeFields(trimOutput(stdout));
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
			basis: outcome.basis,
			failureKind: outcome.failureKind,
			alreadyInTail: outcome.alreadyInTail,
		}, deliverAs, triggerTurn);
	});
	proc.on("error", (error) => {
		if (!callbacks.delete(id)) return;
		refreshCallbackActivity();
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
		details: { id, kind, tab: params.tab, session: params.session, args, startedAt, deadlineAt: new Date(deadlineAt).toISOString(), condition, deliverAs, triggerTurn, message: params.message },
	};
}

export async function startPeerHandoff(pi: ExtensionAPI, params: PeerHandoffParams): Promise<{ text: string; details: Record<string, unknown> }> {
	const useLifecycle = params.waitFor === undefined && params.idleSeconds === undefined;
	const callbackParams: WatchCallbackParams = {
		id: params.id,
		tab: params.tab,
		cwd: params.cwd,
		session: params.session,
		lines: params.lines ?? 200,
		waitFor: params.waitFor,
		idleSeconds: params.idleSeconds,
		turnState: useLifecycle ? "ready" : undefined,
		timeoutSeconds: params.timeoutSeconds ?? 300,
		message: params.message ?? "peer handoff ready",
		deliverAs: params.deliverAs,
		triggerTurn: params.triggerTurn,
		activitySink: params.activitySink,
	};
	const markPeerRunning = params.markPeerRunning ?? true;
	const callback = startWatchCallback(pi, callbackParams, "peer_handoff");
	try {
		if (markPeerRunning) {
			await setTabPeer({ action: "running", tab: params.tab, session: params.session, source: params.source ?? "pi-zmux-handoff", msg: params.message, cwd: params.cwd });
		}
		const typed = await typeText(params.tab, params.text, params.cwd, params.session);
		return {
			text: trimOutput([typed.text, callback.text].join("\n")),
			details: {
				...typed.details,
				...callback.details,
				callback: callback.details,
				markPeerRunning,
				waitFor: params.waitFor,
				idleSeconds: params.idleSeconds,
				turnState: useLifecycle ? "ready" : undefined,
			},
		};
	} catch (error) {
		cancelCallback(String(callback.details.id));
		throw error;
	}
}
