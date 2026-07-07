import { runFileStatus, trimOutput } from "../shell.js";
import { safeJson, withSession, zmux, zmuxBin } from "./shared.js";

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

export function buildWaitArgs(params: { tab: string; session?: string; lines?: number; waitFor?: string; idleSeconds?: number; timeoutSeconds?: number }): string[] {
	if (params.waitFor && params.idleSeconds !== undefined) {
		throw new Error("waitFor and idleSeconds cannot be combined");
	}
	if (!params.waitFor && params.idleSeconds === undefined) {
		throw new Error("wait requires waitFor or idleSeconds");
	}
	const condition = params.waitFor ? `output:${params.waitFor}` : `idle:${params.idleSeconds}`;
	const args = ["wait", params.tab, "--for", condition, "-l", String(params.lines ?? 120), "-T", String(params.timeoutSeconds ?? 10), "--json"];
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

function recordFrom(value: unknown): Record<string, unknown> | undefined {
	if (value && typeof value === "object" && !Array.isArray(value)) return value as Record<string, unknown>;
	return undefined;
}

function stringField(status: Record<string, unknown> | undefined, field: string): string | undefined {
	const value = status?.[field];
	return typeof value === "string" && value.length > 0 ? value : undefined;
}

function numberField(status: Record<string, unknown> | undefined, field: string): number | undefined {
	const raw = stringField(status, field);
	if (!raw) return undefined;
	const parsed = Number(raw);
	return Number.isFinite(parsed) ? parsed : undefined;
}

function boolField(status: Record<string, unknown> | undefined, field: string): boolean | undefined {
	const value = status?.[field];
	return typeof value === "boolean" ? value : undefined;
}

function turnAtSeconds(status: Record<string, unknown> | undefined): number | undefined {
	return numberField(status, "turnAt");
}

function turnSeq(status: Record<string, unknown> | undefined): number | undefined {
	return numberField(status, "turnSeq") ?? numberField(status, "peerTurns");
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
	const seq = turnSeq(status);
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
	if (context.targetState && normalizeTurnState(context.targetState) === normalizeTurnState(turnState ?? "") && context.expectFreshAfter !== undefined) {
		const freshBySeq = seq !== undefined && seq > context.expectFreshAfter;
		const freshByTime = seq === undefined && turnAt !== undefined && turnAt > context.expectFreshAfter;
		if (!freshBySeq && !freshByTime) {
			warnings.push("matching turn state is stale; readiness is unproven");
			failureKind = failureKind ?? "stale_turn_state";
		}
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
		if (expectFreshAfter === undefined) return target as TurnWaitOutcome;
		const seq = turnSeq(status);
		const turnAt = turnAtSeconds(status);
		if ((seq !== undefined && seq > expectFreshAfter) || (seq === undefined && turnAt !== undefined && turnAt > expectFreshAfter)) {
			return target as TurnWaitOutcome;
		}
	}
	return "unproven";
}

async function readStatusRecord(params: { tab: string; cwd: string; session?: string }): Promise<{ text: string; status?: Record<string, unknown>; warning?: string }> {
	try {
		const result = await zmux(withSession(["tab", "status", params.tab, "--json"], params.session), { cwd: params.cwd, timeoutMs: 5_000 });
		const text = trimOutput(result.stdout || result.stderr);
		return { text, status: recordFrom(safeJson(text)) };
	} catch (error) {
		return { text: "", warning: error instanceof Error ? error.message : String(error) };
	}
}

function outcomeFromInspect(parsed: Record<string, unknown>): { status?: Record<string, unknown>; warnings: string[]; logTail: string } {
	const status = recordFrom(parsed.status);
	const warnings = Array.isArray(parsed.warnings) ? parsed.warnings.filter((w): w is string => typeof w === "string") : [];
	const logTail = typeof parsed.outputTail === "string" ? parsed.outputTail : "";
	return { status, warnings, logTail };
}

function waitOutcomeRecord(parsed: Record<string, unknown> | undefined): Record<string, unknown> | undefined {
	return recordFrom(parsed?.outcome);
}

export async function watchTabOutput(params: WatchTabParams): Promise<{ text: string; details: Record<string, unknown> }> {
	if (params.waitFor || params.idleSeconds !== undefined) {
		const args = buildWaitArgs(params);
		const timeoutSeconds = params.timeoutSeconds ?? 10;
		const result = await runFileStatus(zmuxBin(), args, {
			cwd: params.cwd,
			timeoutMs: (timeoutSeconds + 5) * 1000,
		});
		const stdout = trimOutput(result.stdout);
		const stderr = trimOutput(result.stderr);
		const parsed = recordFrom(safeJson(stdout));
		const outcome = waitOutcomeRecord(parsed) ?? recordFrom(safeJson(stdout));
		const outputTail = stringField(outcome, "outputTail") ?? stdout;
		const failureKind = stringField(outcome, "failureKind");
		const alreadyInTail = boolField(outcome, "alreadyInTail") === true || failureKind === "output_regex_already_present" || (params.waitFor ? (outcome?.met === false && watchPatternPresentInText(params.waitFor, outputTail)) : false);
		const details: Record<string, unknown> = {
			tab: params.tab,
			session: params.session,
			lines: params.lines ?? 120,
			waitFor: params.waitFor,
			idleSeconds: params.idleSeconds,
			timeoutSeconds,
			failed: (result.failed || outcome?.met === false) && !alreadyInTail,
			zmuxExitCode: result.exitCode,
			basis: stringField(outcome, "basis"),
			failureKind,
			alreadyInTail,
			fresh: boolField(outcome, "fresh"),
			outcome,
		};
		if (result.signal) details.signal = result.signal;
		if (result.timedOut) details.failureKind = "tool_timeout";
		if (params.waitFor) details.patternPresentInTail = watchPatternPresentInText(params.waitFor, outputTail);
		const note = alreadyInTail ? "output regex was already present before wait baseline" : "";
		return { text: trimOutput([note, outputTail, alreadyInTail ? "" : stderr].filter(Boolean).join("\n")), details };
	}
	const args = buildWatchArgs(params);
	const result = await runFileStatus(zmuxBin(), args, { cwd: params.cwd, timeoutMs: 10_000 });
	const output = trimOutput([result.stdout, result.stderr].filter(Boolean).join("\n"));
	const details: Record<string, unknown> = {
		tab: params.tab,
		session: params.session,
		lines: params.lines ?? 120,
		failed: result.failed,
		zmuxExitCode: result.exitCode,
	};
	if (result.signal) details.signal = result.signal;
	if (result.timedOut) details.failureKind = "tool_timeout";
	else if (result.failed) details.failureKind = "watch_failed";
	return { text: output, details };
}

export async function inspectTab(params: InspectTabParams): Promise<{ text: string; details: Record<string, unknown> }> {
	const args = withSession(["tab", "inspect", params.tab, "--json", "-l", String(params.lines ?? 120)], params.session);
	const result = await zmux(args, { cwd: params.cwd, timeoutMs: 10_000 });
	const output = trimOutput(result.stdout || result.stderr);
	const parsed = recordFrom(safeJson(output)) ?? {};
	const { status, warnings, logTail } = outcomeFromInspect(parsed);
	const details: Record<string, unknown> = {
		tab: params.tab,
		session: params.session,
		lines: params.lines ?? 120,
		status,
		logTail,
		warnings,
		inspect: parsed,
	};
	const text = trimOutput([
		status ? `status: ${JSON.stringify(status)}` : "",
		logTail ? `log tail:\n${logTail}` : "",
		warnings.length > 0 ? `warnings: ${warnings.join("; ")}` : "",
	].filter(Boolean).join("\n"));
	return { text: text || `inspected ${params.tab}`, details };
}

export async function typeTextWithWait(params: TypeTextWithWaitParams): Promise<{ text: string; details: Record<string, unknown> }> {
	const args = ["type", params.tab, params.text, "--json"];
	if (params.markPeerRunning) args.push("--mark-peer-running");
	if (params.waitForTurnState) args.push("--wait-turn", params.waitForTurnState);
	if (params.timeoutSeconds !== undefined) args.push("-T", String(params.timeoutSeconds));
	if (params.lines !== undefined) args.push("-l", String(params.lines));
	if (params.source) args.push("--source", params.source);
	if (params.message) args.push("--msg", params.message);
	const timeoutSeconds = params.timeoutSeconds ?? (params.waitForTurnState ? 8 : 5);
	const result = await runFileStatus(zmuxBin(), withSession(args, params.session), { cwd: params.cwd, timeoutMs: (timeoutSeconds + 5) * 1000 });
	const output = trimOutput([result.stdout, result.stderr].filter(Boolean).join("\n"));
	const parsed = recordFrom(safeJson(result.stdout)) ?? {};
	const outcome = waitOutcomeRecord(parsed);
	const status = recordFrom(parsed.status);
	const warnings = Array.isArray(parsed.warnings) ? parsed.warnings.filter((w): w is string => typeof w === "string") : [];
	const logTail = typeof parsed.outputTail === "string" ? parsed.outputTail : "";
	const details: Record<string, unknown> = {
		tab: params.tab,
		session: params.session,
		markPeerRunning: params.markPeerRunning ?? false,
		waitForTurnState: params.waitForTurnState,
		timeoutSeconds,
		outcome: stringField(outcome, "state") ?? (params.waitForTurnState ? "unproven" : "accepted"),
		basis: stringField(outcome, "basis"),
		failureKind: stringField(outcome, "failureKind"),
		status,
		logTail,
		warnings,
		failed: result.failed,
		zmuxExitCode: result.exitCode,
		raw: parsed,
	};
	const summary = params.waitForTurnState ? `typed text into ${params.tab}; wait outcome: ${details.outcome}` : `typed text into ${params.tab}`;
	return { text: trimOutput([summary, warnings.length > 0 ? `warnings: ${warnings.join("; ")}` : "", logTail ? `log tail:\n${logTail}` : "", result.failed ? output : ""].filter(Boolean).join("\n")), details };
}

export async function peerEnsure(params: PeerEnsureParams): Promise<{ text: string; details: Record<string, unknown> }> {
	const args = ["tab", "peer", "ensure", params.tab, "--json"];
	if (params.command) args.push("--command", params.command);
	if (params.role) args.push("--role", params.role);
	if (params.hostTab) args.push("--host-tab", params.hostTab);
	if (params.hostPane) args.push("--host-pane", params.hostPane);
	if (params.topic) args.push("--topic", params.topic);
	if (params.source) args.push("--source", params.source);
	if (params.message) args.push("--msg", params.message);
	if (params.readiness) args.push("--readiness", params.readiness);
	if (params.waitForTurnState) args.push("--wait-turn", params.waitForTurnState);
	if (params.timeoutSeconds !== undefined) args.push("-T", String(params.timeoutSeconds));
	if (params.lines !== undefined) args.push("-l", String(params.lines));
	if (params.restart) args.push("--restart");
	const timeoutSeconds = params.timeoutSeconds ?? 10;
	const result = await runFileStatus(zmuxBin(), withSession(args, params.session), { cwd: params.cwd, timeoutMs: (timeoutSeconds + 5) * 1000 });
	const output = trimOutput([result.stdout, result.stderr].filter(Boolean).join("\n"));
	const parsed = recordFrom(safeJson(result.stdout)) ?? {};
	const status = recordFrom(parsed.status);
	const outcome = waitOutcomeRecord(parsed);
	const readiness = recordFrom(parsed.readiness);
	const warnings = Array.isArray(parsed.warnings) ? parsed.warnings.filter((w): w is string => typeof w === "string") : [];
	const logTail = typeof parsed.outputTail === "string" ? parsed.outputTail : "";
	const details: Record<string, unknown> = {
		tab: params.tab,
		command: params.command,
		session: params.session,
		role: params.role,
		topic: params.topic,
		readiness: params.readiness,
		waitForTurnState: params.waitForTurnState,
		timeoutSeconds,
		outcome: stringField(outcome, "state") ?? (result.failed ? "failed" : "unproven"),
		basis: stringField(outcome, "basis") ?? stringField(readiness, "basis"),
		failureKind: stringField(outcome, "failureKind") ?? stringField(readiness, "failureKind"),
		status,
		logTail,
		warnings,
		failed: result.failed,
		zmuxExitCode: result.exitCode,
		raw: parsed,
	};
	const summary = `peer ${parsed.created ? "created" : parsed.restarted ? "restarted" : "reused"}: ${params.tab}`;
	return { text: trimOutput([summary, warnings.length > 0 ? `warnings: ${warnings.join("; ")}` : "", logTail ? `log tail:\n${logTail}` : "", result.failed ? output : ""].filter(Boolean).join("\n")), details };
}
