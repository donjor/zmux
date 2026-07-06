import { runFileStatus, trimOutput } from "../shell.js";
import { runCommand } from "./sessions.js";
import { delay, safeJson, withSession, zmux, zmuxBin } from "./shared.js";
import { setTabPeer, tabStatus, type TabPeerAction, typeText } from "./tabs.js";

export type TurnWaitOutcome = "accepted" | "running" | "ready" | "attention" | "failed" | "unproven";

export interface WatchTabParams {
	tab: string;
	cwd: string;
	session?: string;
	lines?: number;
	waitFor?: string;
	idleSeconds?: number;
	timeoutSeconds?: number;
}

export interface InspectTabParams extends WatchTabParams {}

export interface PeerEnsureParams {
	tab: string;
	cwd: string;
	command?: string;
	session?: string;
	role?: string;
	hostTab?: string;
	hostPane?: string;
	topic?: string;
	source?: string;
	message?: string;
	readiness?: string;
	waitForTurnState?: string;
	timeoutSeconds?: number;
	lines?: number;
	restart?: boolean;
}

export interface TypeTextWithWaitParams {
	tab: string;
	text: string;
	cwd: string;
	session?: string;
	markPeerRunning?: boolean;
	waitForTurnState?: string;
	timeoutSeconds?: number;
	lines?: number;
	source?: string;
	message?: string;
}

export function buildWatchArgs(params: { tab: string; session?: string; lines?: number; waitFor?: string; idleSeconds?: number; timeoutSeconds?: number }): string[] {
	if (params.waitFor && params.idleSeconds !== undefined) {
		throw new Error("waitFor and idleSeconds cannot be combined");
	}
	const args = ["watch", params.tab, "-l", String(params.lines ?? 120)];
	if (params.waitFor) args.push("--until", params.waitFor);
	if (params.idleSeconds !== undefined) args.push("--idle", String(params.idleSeconds));
	if (params.waitFor || params.idleSeconds !== undefined) args.push("-T", String(params.timeoutSeconds ?? 10));
	return withSession(args, params.session);
}

export function watchPatternPresentInText(pattern: string | undefined, text: string): boolean {
	if (!pattern || !text) return false;
	try {
		return new RegExp(pattern, "u").test(text);
	} catch {
		return false;
	}
}

export async function watchTabOutput(params: WatchTabParams): Promise<{ text: string; details: Record<string, unknown> }> {
	const args = buildWatchArgs(params);
	const timeoutSeconds = params.waitFor || params.idleSeconds !== undefined ? (params.timeoutSeconds ?? 10) : undefined;
	const result = await runFileStatus(zmuxBin(), args, {
		cwd: params.cwd,
		timeoutMs: timeoutSeconds !== undefined ? (timeoutSeconds + 5) * 1000 : 10_000,
	});
	const output = trimOutput([result.stdout, result.stderr].filter(Boolean).join("\n"));
	const details: Record<string, unknown> = {
		tab: params.tab,
		session: params.session,
		lines: params.lines ?? 120,
		waitFor: params.waitFor,
		idleSeconds: params.idleSeconds,
		timeoutSeconds,
		failed: result.failed,
		zmuxExitCode: result.exitCode,
	};
	if (result.signal) details.signal = result.signal;
	const patternPresentInTail = watchPatternPresentInText(params.waitFor, output);
	if (params.waitFor) details.patternPresentInTail = patternPresentInTail;
	if (result.timedOut) details.failureKind = "tool_timeout";
	else if (result.failed) details.failureKind = params.waitFor || params.idleSeconds !== undefined ? "watch_unproven" : "watch_failed";
	const notes: string[] = [];
	if (result.failed && params.waitFor && patternPresentInTail) {
		details.failureKind = "watch_already_in_tail";
		details.nextAction = "Pattern is present in the captured tail but was not observed as new output after the watch baseline; use a future marker, delayed output, or tab_inspect evidence.";
		notes.push(`NOTE: ${details.nextAction}`);
	}
	return { text: trimOutput([output, ...notes].filter(Boolean).join("\n")), details };
}

function recordFrom(value: unknown): Record<string, unknown> | undefined {
	if (value && typeof value === "object" && !Array.isArray(value)) return value as Record<string, unknown>;
	return undefined;
}

function stringField(status: Record<string, unknown> | undefined, field: string): string | undefined {
	const value = status?.[field];
	return typeof value === "string" && value.length > 0 ? value : undefined;
}

function turnAtSeconds(status: Record<string, unknown> | undefined): number | undefined {
	const raw = stringField(status, "turnAt");
	if (!raw) return undefined;
	const parsed = Number(raw);
	return Number.isFinite(parsed) ? parsed : undefined;
}

function normalizeTurnState(state: string): string {
	return state === "waiting" ? "ready" : state;
}

export function statusWarnings(status: Record<string, unknown> | undefined, context: { expectFreshAfter?: number; targetState?: string } = {}): { warnings: string[]; failureKind?: string } {
	const warnings: string[] = [];
	let failureKind: string | undefined;
	if (!status) {
		warnings.push("tab status JSON unavailable; using text/output evidence only");
		return { warnings, failureKind: "status_unavailable" };
	}
	const cmdState = stringField(status, "cmdState");
	const lastExit = stringField(status, "lastExit");
	const turnState = stringField(status, "turnState");
	const turnAt = turnAtSeconds(status);
	if (lastExit && lastExit !== "0") {
		warnings.push(`last command exited with ${lastExit}`);
		failureKind = "command_exit";
	}
	if (cmdState === "failed") {
		warnings.push("command lifecycle state is failed");
		failureKind = failureKind ?? "command_failed";
	}
	if (turnState === "failed") {
		warnings.push("peer turn state is failed");
		failureKind = failureKind ?? "peer_failed";
	}
	if (turnState === "attention") {
		warnings.push("peer turn state requires attention");
		failureKind = failureKind ?? "peer_attention";
	}
	if (context.targetState && !turnState) {
		warnings.push("turn state is unavailable; readiness is unproven");
		failureKind = failureKind ?? "turn_state_unavailable";
	}
	if (context.targetState && normalizeTurnState(context.targetState) === normalizeTurnState(turnState ?? "") && context.expectFreshAfter !== undefined && (turnAt === undefined || turnAt <= context.expectFreshAfter)) {
		warnings.push("matching turn state is stale; readiness is unproven");
		failureKind = failureKind ?? "stale_turn_state";
	}
	return { warnings, failureKind };
}

export function turnWaitOutcomeForStatus(status: Record<string, unknown> | undefined, targetState?: string, expectFreshAfter?: number): TurnWaitOutcome {
	const turnState = stringField(status, "turnState");
	if (turnState === "failed") return "failed";
	if (turnState === "attention") return "attention";
	if (turnState === "running") return "running";
	const target = targetState ? normalizeTurnState(targetState) : undefined;
	if (target && !["running", "ready", "attention", "failed"].includes(target)) return "unproven";
	if (target && normalizeTurnState(turnState ?? "") === target) {
		const turnAt = turnAtSeconds(status);
		if (expectFreshAfter === undefined || (turnAt !== undefined && turnAt > expectFreshAfter)) {
			return target as TurnWaitOutcome;
		}
	}
	return "unproven";
}

async function readStatusRecord(params: { tab: string; cwd: string; session?: string }): Promise<{ text: string; status?: Record<string, unknown>; warning?: string }> {
	try {
		const statusResult = await tabStatus(params);
		const status = recordFrom(statusResult.details.status) ?? recordFrom(safeJson(statusResult.text));
		return { text: statusResult.text, status };
	} catch (error) {
		return { text: "", warning: error instanceof Error ? error.message : String(error) };
	}
}

export async function inspectTab(params: InspectTabParams): Promise<{ text: string; details: Record<string, unknown> }> {
	const statusResult = await readStatusRecord(params);
	let logTail = "";
	const warnings: string[] = [];
	if (statusResult.warning) warnings.push(`status unavailable: ${statusResult.warning}`);
	const statusClass = statusWarnings(statusResult.status);
	warnings.push(...statusClass.warnings);
	let watchDetails: Record<string, unknown> | undefined;
	try {
		const watch = await watchTabOutput(params);
		logTail = watch.text;
		watchDetails = watch.details;
		if (watch.details.failed) warnings.push("output capture failed or timed out");
	} catch (error) {
		warnings.push(`output capture unavailable: ${error instanceof Error ? error.message : String(error)}`);
	}
	const details: Record<string, unknown> = {
		tab: params.tab,
		session: params.session,
		lines: params.lines ?? 120,
		status: statusResult.status,
		statusText: statusResult.status ? undefined : trimOutput(statusResult.text),
		logTail,
		warnings,
		failureKind: statusClass.failureKind,
		watch: watchDetails,
	};
	const text = trimOutput([
		statusResult.status ? `status: ${JSON.stringify(statusResult.status)}` : statusResult.text,
		logTail ? `log tail:\n${logTail}` : "",
		warnings.length > 0 ? `warnings: ${warnings.join("; ")}` : "",
	].filter(Boolean).join("\n"));
	return { text: text || `inspected ${params.tab}`, details };
}

async function waitForTurnState(params: { tab: string; cwd: string; session?: string; targetState: string; timeoutSeconds?: number; expectFreshAfter?: number }): Promise<{ outcome: TurnWaitOutcome; status?: Record<string, unknown>; warnings: string[]; failureKind?: string }> {
	const timeoutSeconds = params.timeoutSeconds ?? 8;
	const deadline = Date.now() + timeoutSeconds * 1000;
	let latest: Record<string, unknown> | undefined;
	let latestWarnings: string[] = [];
	let latestFailure: string | undefined;
	while (Date.now() <= deadline) {
		const statusResult = await readStatusRecord(params);
		latest = statusResult.status;
		const classification = statusWarnings(latest, { targetState: params.targetState, expectFreshAfter: params.expectFreshAfter });
		latestWarnings = statusResult.warning ? [`status unavailable: ${statusResult.warning}`, ...classification.warnings] : classification.warnings;
		latestFailure = classification.failureKind;
		const outcome = turnWaitOutcomeForStatus(latest, params.targetState, params.expectFreshAfter);
		if (outcome === "failed" || outcome === "attention" || outcome === "ready" || outcome === normalizeTurnState(params.targetState)) {
			return { outcome, status: latest, warnings: latestWarnings, failureKind: latestFailure };
		}
		await delay(500);
	}
	return { outcome: "unproven", status: latest, warnings: latestWarnings, failureKind: latestFailure ?? "turn_state_unproven" };
}

export async function typeTextWithWait(params: TypeTextWithWaitParams): Promise<{ text: string; details: Record<string, unknown> }> {
	let freshAfter: number | undefined;
	if (params.waitForTurnState) {
		const before = await readStatusRecord(params);
		freshAfter = turnAtSeconds(before.status) ?? Math.floor(Date.now() / 1000);
	}
	const typed = await typeText(params.tab, params.text, params.cwd, params.session);
	let markStatus: Record<string, unknown> | undefined;
	if (params.markPeerRunning) {
		await setTabPeer({ action: "running", tab: params.tab, session: params.session, source: params.source, msg: params.message, cwd: params.cwd });
		const marked = await readStatusRecord(params);
		markStatus = marked.status;
		freshAfter = turnAtSeconds(marked.status) ?? freshAfter;
	}
	let waitOutcome: TurnWaitOutcome = params.waitForTurnState ? "unproven" : "accepted";
	let waitStatus: Record<string, unknown> | undefined = markStatus;
	let warnings: string[] = [];
	let failureKind: string | undefined;
	if (params.waitForTurnState) {
		const waited = await waitForTurnState({ tab: params.tab, cwd: params.cwd, session: params.session, targetState: params.waitForTurnState, timeoutSeconds: params.timeoutSeconds, expectFreshAfter: freshAfter });
		waitOutcome = waited.outcome;
		waitStatus = waited.status;
		warnings = waited.warnings;
		failureKind = waited.failureKind;
	}
	let logTail = "";
	if (params.waitForTurnState || params.lines !== undefined) {
		const watch = await watchTabOutput({ tab: params.tab, cwd: params.cwd, session: params.session, lines: params.lines ?? 80 });
		logTail = watch.text;
	}
	const details: Record<string, unknown> = {
		...typed.details,
		markPeerRunning: params.markPeerRunning ?? false,
		waitForTurnState: params.waitForTurnState,
		timeoutSeconds: params.timeoutSeconds ?? (params.waitForTurnState ? 8 : undefined),
		outcome: waitOutcome,
		turnState: stringField(waitStatus, "turnState"),
		turnAt: stringField(waitStatus, "turnAt"),
		freshAfter,
		status: waitStatus,
		logTail,
		warnings,
		failureKind,
	};
	const summary = params.waitForTurnState ? `typed text into ${params.tab}; wait outcome: ${waitOutcome}` : typed.text;
	return { text: trimOutput([summary, warnings.length > 0 ? `warnings: ${warnings.join("; ")}` : "", logTail ? `log tail:\n${logTail}` : ""].filter(Boolean).join("\n")), details };
}

export async function peerEnsure(params: PeerEnsureParams): Promise<{ text: string; details: Record<string, unknown> }> {
	const output: string[] = [];
	const warnings: string[] = [];
	const startDetails: Record<string, unknown> = {};
	if (params.restart) {
		try {
			await zmux(withSession(["send", params.tab, "C-c"], params.session), { cwd: params.cwd, timeoutMs: 5_000 });
			output.push(`sent C-c to ${params.tab}`);
		} catch (error) {
			warnings.push(`restart stop skipped: ${error instanceof Error ? error.message : String(error)}`);
		}
	}
	if (params.command) {
		const run = await runCommand({ command: params.command, tab: params.tab, cwd: params.cwd, session: params.session, detach: true, timeoutSeconds: params.timeoutSeconds ?? 10, lines: params.lines ?? 80, scope: "peer" });
		output.push(run.text || `started ${params.tab}`);
		Object.assign(startDetails, run.details);
	}
	try {
		const action: TabPeerAction = "start";
		await setTabPeer({ action, tab: params.tab, session: params.session, role: params.role, hostTab: params.hostTab, hostPane: params.hostPane, topic: params.topic, source: params.source, msg: params.message, cwd: params.cwd });
		output.push(`peer lifecycle stamped for ${params.tab}`);
	} catch (error) {
		warnings.push(`peer lifecycle stamp failed: ${error instanceof Error ? error.message : String(error)}`);
	}
	if (params.readiness) {
		const watch = await watchTabOutput({ tab: params.tab, cwd: params.cwd, session: params.session, lines: params.lines ?? 120, waitFor: params.readiness, timeoutSeconds: params.timeoutSeconds ?? 10 });
		output.push(watch.text);
		if (watch.details.failed) warnings.push("readiness pattern not proven");
	}
	let waitOutcome: TurnWaitOutcome | undefined;
	let waitedStatus: Record<string, unknown> | undefined;
	let failureKind: string | undefined;
	if (params.waitForTurnState) {
		const baseline = await readStatusRecord(params);
		const waited = await waitForTurnState({ tab: params.tab, cwd: params.cwd, session: params.session, targetState: params.waitForTurnState, timeoutSeconds: params.timeoutSeconds, expectFreshAfter: turnAtSeconds(baseline.status) });
		waitOutcome = waited.outcome;
		waitedStatus = waited.status;
		warnings.push(...waited.warnings);
		failureKind = waited.failureKind;
	}
	const inspected = await inspectTab({ tab: params.tab, cwd: params.cwd, session: params.session, lines: params.lines ?? 120 });
	const status = waitedStatus ?? recordFrom(inspected.details.status);
	const classified = statusWarnings(status);
	warnings.push(...classified.warnings);
	failureKind = failureKind ?? classified.failureKind;
	const uniqueWarnings = Array.from(new Set(warnings.filter(Boolean)));
	const details: Record<string, unknown> = {
		tab: params.tab,
		command: params.command,
		session: params.session,
		role: params.role,
		topic: params.topic,
		readiness: params.readiness,
		waitForTurnState: params.waitForTurnState,
		timeoutSeconds: params.timeoutSeconds ?? 10,
		outcome: waitOutcome ?? (failureKind ? "failed" : "unproven"),
		status,
		logTail: inspected.details.logTail,
		warnings: uniqueWarnings,
		failureKind,
		start: startDetails,
	};
	return { text: trimOutput([...output, inspected.text, uniqueWarnings.length > 0 ? `warnings: ${uniqueWarnings.join("; ")}` : ""].filter(Boolean).join("\n")), details };
}
