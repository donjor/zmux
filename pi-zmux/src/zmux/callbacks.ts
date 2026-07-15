import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";
import { spawn, type ChildProcess } from "node:child_process";
import { trimOutput } from "../shell.js";
import { summarizeWaitOutput } from "../wait-summary.js";
import { buildWaitArgs, readTurnSeq } from "./agent.js";
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
	commandState?: string;
	allowStale?: boolean;
	freshAfter?: number;
	timeoutSeconds?: number;
	message?: string;
	deliverAs?: CallbackDeliverAs;
	triggerTurn?: boolean;
	activitySink?: CallbackActivitySink;
	continueOnRunningTimeout?: boolean;
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
	continueOnRunningTimeout: boolean;
	attempts: number;
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

function callbackCondition(params: Pick<WatchCallbackParams, "waitFor" | "idleSeconds" | "turnState" | "commandState">): string {
	if (params.turnState) return `waiting for turn ${params.turnState}`;
	if (params.commandState) return `waiting for command ${params.commandState}`;
	if (params.waitFor) return `waiting for /${params.waitFor}/`;
	return `waiting for ${params.idleSeconds}s idle`;
}

function formatRemaining(deadlineAt: number): string {
	const seconds = Math.max(0, Math.ceil((deadlineAt - Date.now()) / 1000));
	if (seconds < 60) return `${seconds}s`;
	const minutes = Math.floor(seconds / 60);
	if (minutes < 60) return `${minutes}:${String(seconds % 60).padStart(2, "0")}`;
	return `${Math.floor(minutes / 60)}:${String(minutes % 60).padStart(2, "0")}:${String(seconds % 60).padStart(2, "0")}`;
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

function callbackOutcomeFields(stdout: string): { basis?: string; state?: string; failureKind?: string; alreadyInTail?: boolean } {
	try {
		const parsed = JSON.parse(stdout) as { outcome?: { basis?: unknown; state?: unknown; failureKind?: unknown; alreadyInTail?: unknown } };
		return {
			basis: typeof parsed.outcome?.basis === "string" ? parsed.outcome.basis : undefined,
			state: typeof parsed.outcome?.state === "string" ? parsed.outcome.state : undefined,
			failureKind: typeof parsed.outcome?.failureKind === "string" ? parsed.outcome.failureKind : undefined,
			alreadyInTail: typeof parsed.outcome?.alreadyInTail === "boolean" ? parsed.outcome.alreadyInTail : undefined,
		};
	} catch {
		return {};
	}
}

export function buildCallbackWatchArgs(params: { tab: string; session?: string; lines?: number; waitFor?: string; idleSeconds?: number; turnState?: string; commandState?: string; timeoutSeconds?: number; allowStale?: boolean; freshAfter?: number }): string[] {
	return buildWaitArgs({
		tab: params.tab,
		session: params.session,
		lines: params.lines ?? 160,
		waitFor: params.waitFor,
		idleSeconds: params.idleSeconds,
		turnState: params.turnState,
		commandState: params.commandState,
		timeoutSeconds: params.timeoutSeconds ?? 300,
		allowStale: params.allowStale,
		freshAfter: params.freshAfter,
	});
}

export function formatCallbackMessage(done: CallbackCompletion): string {
	const ok = done.exitCode === 0;
	const concreteFailure = done.failureKind === "cmd_failed" || done.failureKind === "cmd_exit" || done.failureKind === "command_failed" || done.failureKind === "command_exit" || done.failureKind === "turn_failed";
	const heading = ok
		? `zmux callback ${done.id} completed for ${done.tab}`
		: concreteFailure
			? `zmux callback ${done.id} failed for ${done.tab}`
			: `zmux callback ${done.id} finished unproven for ${done.tab}`;
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
		continueOnRunningTimeout: handle.continueOnRunningTimeout,
		attempts: handle.attempts,
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

// resolveDelivery applies the kind's deliverAs default and rejects the
// contradictory nextTurn+triggerTurn pairing. Shared so a peer handoff can
// validate delivery semantics before any side effect, not only once the
// callback is being armed.
function resolveDelivery(deliverAs: CallbackDeliverAs | undefined, triggerTurn: boolean | undefined, kind: "watch" | "peer_handoff"): { deliverAs: CallbackDeliverAs; triggerTurn: boolean } {
	const resolvedDeliverAs = deliverAs ?? (kind === "peer_handoff" ? "followUp" : "steer");
	const resolvedTriggerTurn = triggerTurn ?? true;
	if (resolvedDeliverAs === "nextTurn" && resolvedTriggerTurn) {
		throw new Error('deliverAs "nextTurn" never triggers a turn; use "steer" or "followUp", or set triggerTurn false');
	}
	return { deliverAs: resolvedDeliverAs, triggerTurn: resolvedTriggerTurn };
}

export function startWatchCallback(pi: ExtensionAPI, params: WatchCallbackParams, kind: "watch" | "peer_handoff" = "watch"): { text: string; details: Record<string, unknown> } {
	const conditions = [params.waitFor !== undefined, params.idleSeconds !== undefined, params.turnState !== undefined, params.commandState !== undefined].filter(Boolean).length;
	if (conditions !== 1) {
		throw new Error("callback watch requires exactly one of waitFor, idleSeconds, turnState, or commandState");
	}
	const id = callbackId(kind === "peer_handoff" ? "peer-handoff" : "callback", params.id);
	if (callbacks.has(id)) throw new Error(`callback id already exists: ${id}`);
	const args = buildCallbackWatchArgs(params);
	const { deliverAs, triggerTurn } = resolveDelivery(params.deliverAs, params.triggerTurn, kind);
	const startedAt = new Date().toISOString();
	const timeoutSeconds = params.timeoutSeconds ?? 300;
	const condition = callbackCondition(params);
	const deadlineAt = Date.now() + timeoutSeconds * 1000;
	const spawnWait = () => spawn(zmuxBin(), args, {
		cwd: params.cwd,
		env: process.env,
		stdio: ["ignore", "pipe", "pipe"],
	});
	const proc = spawnWait();
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
		continueOnRunningTimeout: params.continueOnRunningTimeout ?? false,
		attempts: 1,
	};
	callbacks.set(id, handle);
	refreshCallbackActivity();
	const attachWait = (child: ChildProcess): void => {
		let stdout = "";
		let stderr = "";
		child.stdout?.on("data", (chunk) => {
			stdout = capBuffer(stdout + String(chunk));
		});
		child.stderr?.on("data", (chunk) => {
			stderr = capBuffer(stderr + String(chunk));
		});
		child.on("close", (exitCode, signal) => {
			if (callbacks.get(id) !== handle) return;
			const outcome = callbackOutcomeFields(trimOutput(stdout));
			const timedOutWhileRunning = outcome.state === "running" && outcome.failureKind?.endsWith("_unproven") === true;
			if (handle.continueOnRunningTimeout && timedOutWhileRunning) {
				try {
					const next = spawnWait();
					handle.proc = next;
					handle.attempts += 1;
					handle.deadlineAt = Date.now() + timeoutSeconds * 1000;
					refreshCallbackActivity();
					attachWait(next);
					return;
				} catch (error) {
					stderr = trimOutput([stderr, `failed to continue callback: ${error instanceof Error ? error.message : String(error)}`].filter(Boolean).join("\n"));
				}
			}
			callbacks.delete(id);
			refreshCallbackActivity();
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
		child.on("error", (error) => {
			if (callbacks.get(id) !== handle) return;
			callbacks.delete(id);
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
	};
	attachWait(proc);
	return {
		text: `scheduled zmux ${kind === "peer_handoff" ? "peer handoff" : "callback"} ${id} for ${params.tab}`,
		details: { id, kind, tab: params.tab, session: params.session, args, startedAt, deadlineAt: new Date(deadlineAt).toISOString(), condition, deliverAs, triggerTurn, message: params.message, continueOnRunningTimeout: handle.continueOnRunningTimeout },
	};
}

export async function startPeerHandoff(pi: ExtensionAPI, params: PeerHandoffParams): Promise<{ text: string; details: Record<string, unknown> }> {
	// Reject contradictory delivery semantics before issuing the turn-seq read
	// or arming the wait, so an invalid handoff has no observable side effect.
	resolveDelivery(params.deliverAs, params.triggerTurn, "peer_handoff");
	const useLifecycle = params.waitFor === undefined && params.idleSeconds === undefined;
	// Anchor the readiness wait to the peer's turn generation *before* we mark it
	// running and type the brief. The wait is spawned as a child process below,
	// and without a floor it can observe a pre-existing `ready` state and fire
	// the handoff before the brief is even delivered. Capturing the seq
	// synchronously here makes freshness independent of when that child
	// snapshots its own baseline. Only meaningful for the turn-lifecycle path.
	let freshAfter: number | undefined;
	if (useLifecycle) {
		const seq = await readTurnSeq({ tab: params.tab, cwd: params.cwd, session: params.session });
		if (seq !== undefined && seq > 0) freshAfter = seq;
	}
	const callbackParams: WatchCallbackParams = {
		id: params.id,
		tab: params.tab,
		cwd: params.cwd,
		session: params.session,
		lines: params.lines ?? 200,
		waitFor: params.waitFor,
		idleSeconds: params.idleSeconds,
		turnState: useLifecycle ? "ready" : undefined,
		freshAfter,
		timeoutSeconds: params.timeoutSeconds ?? 300,
		message: params.message ?? "peer handoff ready",
		deliverAs: params.deliverAs,
		triggerTurn: params.triggerTurn,
		activitySink: params.activitySink,
		continueOnRunningTimeout: useLifecycle,
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
				freshAfter,
			},
		};
	} catch (error) {
		cancelCallback(String(callback.details.id));
		throw error;
	}
}
