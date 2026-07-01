import { focusTab } from "./context.js";
import { runtimeLogs } from "./runtimes.js";
import { tabStatus } from "./tabs.js";
import { delay, withSession, zmux } from "./shared.js";

export interface InteractiveTypeOptions {
	waitFor?: string;
	waitForExit?: boolean;
	timeoutSeconds?: number;
	lines?: number;
	focus?: boolean;
	session?: string;
}

export interface CommandStatus {
	cmdState?: unknown;
	cmdSeq?: unknown;
	lastExit?: unknown;
	state?: unknown;
	command?: unknown;
}

async function readTabCommandStatus(tab: string, cwd: string, session?: string): Promise<CommandStatus | undefined> {
	const result = await tabStatus({ tab, cwd, session });
	const status = result.details.status;
	if (!status || typeof status !== "object") return undefined;
	return status as CommandStatus;
}

async function readBaselineCommandStatus(tab: string, cwd: string, session?: string): Promise<CommandStatus | undefined> {
	let lastError: unknown;
	for (let attempt = 0; attempt < 2; attempt++) {
		try {
			return await readTabCommandStatus(tab, cwd, session);
		} catch (error) {
			lastError = error;
			if (attempt === 0) await delay(250);
		}
	}
	throw new Error(`could not read baseline tab status for ${tab}: ${lastError instanceof Error ? lastError.message : String(lastError)}`);
}

async function tabExists(tab: string, cwd: string, session?: string): Promise<boolean> {
	try {
		await readTabCommandStatus(tab, cwd, session);
		return true;
	} catch {
		return false;
	}
}

async function ensureInteractiveShellTab(tab: string, cwd: string, focus: boolean, session?: string): Promise<void> {
	await zmux(withSession(["run", "exec bash -l", "-n", tab, "-d"], session), { cwd, timeoutMs: 10_000 });
	if (focus) await focusTab(tab, cwd);
	await delay(300);
}

function outputAfterBaseline(latest: string, baseline: string): string {
	if (!baseline) return latest;
	if (latest === baseline) return "";
	if (latest.startsWith(baseline)) return latest.slice(baseline.length).replace(/^\n/u, "");
	const beforeLines = baseline.split("\n");
	const latestLines = latest.split("\n");
	const maxOverlap = Math.min(beforeLines.length, latestLines.length);
	for (let count = maxOverlap; count > 0; count--) {
		const beforeSuffix = beforeLines.slice(beforeLines.length - count).join("\n");
		const latestPrefix = latestLines.slice(0, count).join("\n");
		if (beforeSuffix === latestPrefix) return latestLines.slice(count).join("\n");
	}
	return latest;
}

export interface UserInputPrompt {
	kind: "sudo_password" | "password" | "ssh_confirm";
	line: string;
}

export class UserInputRequiredError extends Error {
	readonly output: string;
	readonly prompt: UserInputPrompt;

	constructor(output: string, prompt: UserInputPrompt) {
		super(`user input required: ${prompt.kind}`);
		this.output = output;
		this.prompt = prompt;
	}
}

export function detectUserInputPrompt(output: string): UserInputPrompt | undefined {
	const lines = output.split("\n").map((line) => line.trim()).filter(Boolean);
	for (let i = lines.length - 1; i >= 0; i--) {
		const line = lines[i];
		if (/\[sudo\]\s+password\s+for\s+.+:\s*$/iu.test(line)) return { kind: "sudo_password", line };
		if (/(password|passphrase).*:\s*$/iu.test(line)) return { kind: "password", line };
		if (/are you sure you want to continue connecting.*\?\s*$/iu.test(line)) return { kind: "ssh_confirm", line };
	}
	return undefined;
}

function parseNumber(value: unknown): number | undefined {
	if (typeof value === "number" && Number.isFinite(value)) return value;
	if (typeof value !== "string") return undefined;
	const trimmed = value.trim();
	if (!/^\d+$/u.test(trimmed)) return undefined;
	return Number(trimmed);
}

function commandSeq(status?: CommandStatus): number | undefined {
	return parseNumber(status?.cmdSeq);
}

function commandState(status?: CommandStatus): string {
	return typeof status?.cmdState === "string" ? status.cmdState : "";
}

function commandExitCode(status?: CommandStatus): number {
	const parsed = parseNumber(status?.lastExit);
	if (parsed !== undefined) return parsed;
	return commandState(status) === "failed" ? 1 : 0;
}

export function settledFreshCommandStatus(status: CommandStatus | undefined, baselineSeq: number | undefined): { fresh: boolean; settled: boolean; state: string; exitCode?: number; cmdSeq?: number } {
	const seq = commandSeq(status);
	const fresh = seq !== undefined && (baselineSeq === undefined ? seq >= 1 : seq > baselineSeq);
	const state = commandState(status);
	const settled = fresh && (state === "done" || state === "failed");
	return {
		fresh,
		settled,
		state,
		...(settled ? { exitCode: commandExitCode(status) } : {}),
		...(seq !== undefined ? { cmdSeq: seq } : {}),
	};
}

async function pollTab(
	tab: string,
	cwd: string,
	lines: number,
	timeoutSeconds: number,
	predicate: (output: string) => boolean,
	options: { detectUserInput?: boolean; baseline?: string; session?: string } = {},
): Promise<string> {
	const deadline = Date.now() + timeoutSeconds * 1000;
	let latest = "";
	while (Date.now() <= deadline) {
		latest = (await runtimeLogs(tab, cwd, lines, options.session)).text;
		if (predicate(latest)) return latest;
		if (options.detectUserInput) {
			const scoped = outputAfterBaseline(latest, options.baseline ?? "");
			const prompt = detectUserInputPrompt(scoped);
			if (prompt) throw new UserInputRequiredError(latest, prompt);
		}
		await delay(500);
	}
	throw new Error(`timeout after ${timeoutSeconds}s${latest ? `\n${latest}` : ""}`);
}

async function pollCommandStatus(
	tab: string,
	cwd: string,
	lines: number,
	timeoutSeconds: number,
	baselineSeq: number | undefined,
	baselineOutput: string,
	detectUserInput: boolean,
	session?: string,
): Promise<{ output: string; exitCode: number; status: CommandStatus }> {
	const deadline = Date.now() + timeoutSeconds * 1000;
	let latest = "";
	let lastStatus: CommandStatus | undefined;
	let sawFreshCommand = false;
	while (Date.now() <= deadline) {
		latest = (await runtimeLogs(tab, cwd, lines, session)).text;
		lastStatus = await readTabCommandStatus(tab, cwd, session);
		const status = lastStatus;
		const outcome = settledFreshCommandStatus(status, baselineSeq);
		if (outcome.fresh) {
			sawFreshCommand = true;
			if (outcome.settled) {
				return { output: latest, exitCode: outcome.exitCode ?? 0, status: status ?? {} };
			}
		}
		if (detectUserInput) {
			const scoped = outputAfterBaseline(latest, baselineOutput);
			const prompt = detectUserInputPrompt(scoped);
			if (prompt) throw new UserInputRequiredError(latest, prompt);
		}
		await delay(500);
	}
	const state = commandState(lastStatus);
	const reason = sawFreshCommand ? `command still ${state || "unsettled"}` : "no fresh command lifecycle; run `zmux setup shell` and open a fresh tab";
	throw new Error(`timeout after ${timeoutSeconds}s (${reason})${latest ? `\n${latest}` : ""}`);
}

export async function interactiveType(
	tab: string,
	command: string,
	cwd: string,
	options: InteractiveTypeOptions = {},
): Promise<{ text: string; details: Record<string, unknown> }> {
	const focus = options.focus ?? false;
	const session = options.session;
	if (!(await tabExists(tab, cwd, session))) {
		await ensureInteractiveShellTab(tab, cwd, focus, session);
	} else if (focus) {
		await focusTab(tab, cwd);
	}

	const timeoutSeconds = options.timeoutSeconds ?? 90;
	const lines = options.lines ?? 160;
	const typedMessage = `typed command into ${tab}${focus ? " and focused it" : " without changing focus"}; user may need to respond there`;
	const output: string[] = [];
	const details: Record<string, unknown> = { tab, command, waitForExit: options.waitForExit ?? false, focus, session };
	if (options.waitForExit) {
		const baseline = await runtimeLogs(tab, cwd, lines, session).then((logs) => logs.text).catch(() => "");
		let typed = false;
		try {
			const baselineSeq = commandSeq(await readBaselineCommandStatus(tab, cwd, session));
			await zmux(withSession(["type", tab, command], session), { cwd, timeoutMs: 5_000 });
			typed = true;
			output.push(typedMessage);
			const result = await pollCommandStatus(tab, cwd, lines, timeoutSeconds, baselineSeq, baseline, !focus, session);
			const scoped = outputAfterBaseline(result.output, baseline).trimEnd();
			if (scoped) output.push("", scoped);
			details.completed = true;
			details.exitCode = result.exitCode;
			details.cmdSeq = commandSeq(result.status);
			details.cmdState = commandState(result.status);
			if (result.status.command) details.observedCommand = result.status.command;
			if (result.exitCode !== 0) details.warning = `command exited with ${result.exitCode}`;
		} catch (error) {
			if (error instanceof UserInputRequiredError) {
				const scoped = outputAfterBaseline(error.output, baseline).trimEnd();
				output.push("", `user input required in ${tab}: ${error.prompt.line}`);
				output.push("Ask the user before focusing this tab, or ask them to switch there manually.");
				details.completed = false;
				details.needsUserInput = true;
				details.promptKind = error.prompt.kind;
				details.prompt = error.prompt.line;
				details.output = scoped;
			} else {
				output.push(typed ? "" : `did not type command into ${tab}`);
				output.push(`wait timed out or failed: ${error instanceof Error ? error.message : String(error)}`);
				details.completed = false;
			}
		}
	} else if (options.waitFor) {
		const baseline = await runtimeLogs(tab, cwd, lines, session).then((logs) => logs.text).catch(() => "");
		await zmux(withSession(["type", tab, command], session), { cwd, timeoutMs: 5_000 });
		output.push(typedMessage);
		const waitPattern = new RegExp(options.waitFor);
		try {
			const result = await pollTab(tab, cwd, lines, timeoutSeconds, (text) => waitPattern.test(outputAfterBaseline(text, baseline)), {
				detectUserInput: !focus,
				baseline,
				session,
			});
			output.push("", outputAfterBaseline(result, baseline));
			details.waitFor = options.waitFor;
			details.matched = true;
		} catch (error) {
			output.push("", `wait timed out or failed: ${error instanceof Error ? error.message : String(error)}`);
			details.waitFor = options.waitFor;
			details.matched = false;
		}
	} else {
		await zmux(withSession(["type", tab, command], session), { cwd, timeoutMs: 5_000 });
		output.push(typedMessage);
	}
	return { text: output.join("\n"), details };
}
