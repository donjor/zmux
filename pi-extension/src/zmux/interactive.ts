import { chmod, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { focusTab } from "./context.js";
import { runtimeLogs } from "./runtimes.js";
import { delay, shellQuote, withSession, zmux } from "./shared.js";

export interface InteractiveTypeOptions {
	waitFor?: string;
	waitForExit?: boolean;
	timeoutSeconds?: number;
	lines?: number;
	focus?: boolean;
	session?: string;
}

async function tabExists(tab: string, cwd: string, session?: string): Promise<boolean> {
	try {
		await runtimeLogs(tab, cwd, 1, session);
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

interface WaitScript {
	dir: string;
	runPath: string;
	statusPath: string;
}

async function writeWaitScript(command: string): Promise<WaitScript> {
	const dir = await mkdtemp(join(tmpdir(), "pi-zmux-"));
	const cmdPath = join(dir, "cmd.sh");
	const runPath = join(dir, "run.sh");
	const statusPath = join(dir, "status");
	const statusTmpPath = join(dir, "status.tmp");
	await writeFile(cmdPath, `${command}\n`, { mode: 0o700 });
	const script = `#!/usr/bin/env bash
status=${shellQuote(statusPath)}
status_tmp=${shellQuote(statusTmpPath)}
cmd=${shellQuote(cmdPath)}
ec=0
write_status() {
  local code="$1"
  printf '%s\n' "$code" > "$status_tmp"
  mv "$status_tmp" "$status"
}
cleanup() {
  local code=$?
  write_status "$code"
  rm -f "$cmd" "$0" "$status_tmp"
}
trap cleanup EXIT
bash "$cmd"
ec=$?
exit "$ec"
`;
	await writeFile(runPath, script, { mode: 0o700 });
	await chmod(cmdPath, 0o700);
	await chmod(runPath, 0o700);
	return { dir, runPath, statusPath };
}

async function cleanupWaitScript(script: WaitScript): Promise<void> {
	await rm(script.dir, { recursive: true, force: true });
}

async function readExitCode(statusPath: string): Promise<number | undefined> {
	try {
		const value = (await readFile(statusPath, "utf8")).trim();
		if (!/^\d+$/u.test(value)) return undefined;
		return Number(value);
	} catch {
		return undefined;
	}
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

function stripRunnerCommand(output: string, runPath?: string): string {
	if (!runPath) return output.trimEnd();
	return output
		.split("\n")
		.filter((line) => !line.includes(runPath))
		.join("\n")
		.trimEnd();
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

async function pollWaitScript(
	tab: string,
	cwd: string,
	lines: number,
	timeoutSeconds: number,
	script: WaitScript,
	baseline: string,
	detectUserInput: boolean,
	session?: string,
): Promise<{ output: string; exitCode: number }> {
	const deadline = Date.now() + timeoutSeconds * 1000;
	let latest = "";
	while (Date.now() <= deadline) {
		latest = (await runtimeLogs(tab, cwd, lines, session)).text;
		const exitCode = await readExitCode(script.statusPath);
		if (exitCode !== undefined) return { output: latest, exitCode };
		if (detectUserInput) {
			const scoped = outputAfterBaseline(latest, baseline);
			const prompt = detectUserInputPrompt(scoped);
			if (prompt) throw new UserInputRequiredError(latest, prompt);
		}
		await delay(500);
	}
	throw new Error(`timeout after ${timeoutSeconds}s${latest ? `\n${latest}` : ""}`);
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
	const output = [`typed command into ${tab}${focus ? " and focused it" : " without changing focus"}; user may need to respond there`];
	const details: Record<string, unknown> = { tab, command, waitForExit: options.waitForExit ?? false, focus, session };
	if (options.waitForExit) {
		const baseline = await runtimeLogs(tab, cwd, lines, session).then((logs) => logs.text).catch(() => "");
		const script = await writeWaitScript(command);
		try {
			await zmux(withSession(["type", tab, `bash ${shellQuote(script.runPath)}`], session), { cwd, timeoutMs: 5_000 });
			const result = await pollWaitScript(tab, cwd, lines, timeoutSeconds, script, baseline, !focus, session);
			const scoped = stripRunnerCommand(outputAfterBaseline(result.output, baseline), script.runPath);
			if (scoped) output.push("", scoped);
			details.completed = true;
			details.exitCode = result.exitCode;
			if (result.exitCode !== 0) details.warning = `command exited with ${result.exitCode}`;
			await cleanupWaitScript(script);
		} catch (error) {
			if (error instanceof UserInputRequiredError) {
				const scoped = stripRunnerCommand(outputAfterBaseline(error.output, baseline), script.runPath);
				output.push("", `user input required in ${tab}: ${error.prompt.line}`);
				output.push("Ask the user before focusing this tab, or ask them to switch there manually.");
				details.completed = false;
				details.needsUserInput = true;
				details.promptKind = error.prompt.kind;
				details.prompt = error.prompt.line;
				details.output = scoped;
			} else {
				output.push("", `wait timed out or failed: ${error instanceof Error ? error.message : String(error)}`);
				details.completed = false;
				await cleanupWaitScript(script);
			}
		}
	} else if (options.waitFor) {
		const baseline = await runtimeLogs(tab, cwd, lines, session).then((logs) => logs.text).catch(() => "");
		await zmux(withSession(["type", tab, command], session), { cwd, timeoutMs: 5_000 });
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
	}
	return { text: output.join("\n"), details };
}
