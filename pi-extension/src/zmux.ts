import { chmod, mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { writeRespawnContinuation } from "./respawn-continuation.js";
import { runFile, spawnDetached, trimOutput } from "./shell.js";

export interface CurrentPane {
	Session?: string;
	ID?: string;
	Index?: number;
	WindowIndex?: number;
	Command?: string;
	Dir?: string;
	Title?: string;
}

export interface InteractiveTypeOptions {
	waitFor?: string;
	waitForExit?: boolean;
	timeoutSeconds?: number;
	lines?: number;
	focus?: boolean;
}

export async function zmux(args: string[], options: { cwd?: string; timeoutMs?: number } = {}) {
	return runFile("zmux", args, options);
}

async function tmux(args: string[], options: { cwd?: string; timeoutMs?: number } = {}) {
	return runFile("tmux", args, options);
}

export async function currentPane(cwd: string): Promise<CurrentPane | undefined> {
	try {
		const result = await zmux(["pane", "current", "--json"], { cwd, timeoutMs: 5_000 });
		return JSON.parse(result.stdout) as CurrentPane;
	} catch {
		return undefined;
	}
}

export async function listTabs(cwd: string): Promise<string> {
	try {
		const result = await zmux(["tabs"], { cwd, timeoutMs: 5_000 });
		return trimOutput(result.stdout);
	} catch (error) {
		return `unavailable: ${error instanceof Error ? error.message : String(error)}`;
	}
}

export async function killTab(tab: string, cwd: string): Promise<{ text: string; details: Record<string, unknown> }> {
	await zmux(["tab", "kill", tab], { cwd, timeoutMs: 10_000 });
	return { text: `killed tab ${tab}`, details: { tab } };
}

export async function sendKeys(tab: string, keys: string[], cwd: string): Promise<{ text: string; details: Record<string, unknown> }> {
	await zmux(["send", tab, ...keys], { cwd, timeoutMs: 5_000 });
	return { text: `sent keys to ${tab}: ${keys.join(" ")}`, details: { tab, keys } };
}

export async function sendPaneKeys(pane: string, keys: string[], cwd: string): Promise<{ text: string; details: Record<string, unknown> }> {
	await tmux(["send-keys", "-t", pane, ...keys], { cwd, timeoutMs: 5_000 });
	return { text: `sent keys to pane ${pane}: ${keys.join(" ")}`, details: { pane, keys } };
}

export async function typeText(tab: string, text: string, cwd: string): Promise<{ text: string; details: Record<string, unknown> }> {
	await zmux(["type", tab, text], { cwd, timeoutMs: 5_000 });
	return { text: `typed text into ${tab}`, details: { tab, text } };
}

export async function typePaneText(pane: string, text: string, cwd: string): Promise<{ text: string; details: Record<string, unknown> }> {
	await tmux(["send-keys", "-t", pane, text, "Enter"], { cwd, timeoutMs: 5_000 });
	return { text: `typed text into pane ${pane}`, details: { pane, text } };
}

export async function listPanes(cwd: string): Promise<string> {
	try {
		const result = await zmux(["pane", "list"], { cwd, timeoutMs: 5_000 });
		return trimOutput(result.stdout);
	} catch (error) {
		return `unavailable: ${error instanceof Error ? error.message : String(error)}`;
	}
}

export async function focusPane(pane: string, cwd: string): Promise<{ text: string; details: Record<string, unknown> }> {
	await zmux(["pane", "focus", pane], { cwd, timeoutMs: 5_000 });
	return { text: `focused pane ${pane}`, details: { pane } };
}

export async function closePane(pane: string, cwd: string): Promise<{ text: string; details: Record<string, unknown> }> {
	await zmux(["pane", "close", pane], { cwd, timeoutMs: 5_000 });
	return { text: `closed pane ${pane}`, details: { pane } };
}

export async function capabilities(cwd: string): Promise<string> {
	try {
		const result = await zmux(["terminal", "capabilities"], { cwd, timeoutMs: 5_000 });
		return trimOutput(result.stdout);
	} catch (error) {
		return `unavailable: ${error instanceof Error ? error.message : String(error)}`;
	}
}

export async function runtimeEnsure(params: {
	tab: string;
	command: string;
	cwd: string;
	readiness?: string;
	timeoutSeconds?: number;
	restart?: boolean;
	labelTab?: boolean;
}): Promise<{ text: string; details: Record<string, unknown> }> {
	const details: Record<string, unknown> = { tab: params.tab, command: params.command, cwd: params.cwd };
	const output: string[] = [];

	if (params.restart) {
		try {
			await zmux(["send", params.tab, "C-c"], { cwd: params.cwd, timeoutMs: 5_000 });
			output.push(`sent C-c to ${params.tab}`);
		} catch (error) {
			output.push(`restart stop skipped: ${error instanceof Error ? error.message : String(error)}`);
		}
	}

	await zmux(["run", params.command, "-n", params.tab, "-d"], { cwd: params.cwd, timeoutMs: 10_000 });
	output.push(`runtime ${params.tab} ensured via zmux run -d`);

	if (params.labelTab) {
		try {
			await zmux(["tab", "label", params.tab], { cwd: params.cwd, timeoutMs: 5_000 });
			details.labelTab = true;
		} catch {
			// Labeling is helpful but not required for runtime ownership.
		}
	}

	if (params.readiness) {
		const timeout = String(params.timeoutSeconds ?? 90);
		try {
			const watch = await zmux(["watch", params.tab, "--until", params.readiness, "-T", timeout, "-l", "120"], {
				cwd: params.cwd,
				timeoutMs: (Number(timeout) + 5) * 1000,
			});
			output.push(trimOutput(watch.stdout));
			details.ready = true;
		} catch (error) {
			output.push(`readiness not confirmed: ${error instanceof Error ? error.message : String(error)}`);
			details.ready = false;
		}
	}

	try {
		const logs = await runtimeLogs(params.tab, params.cwd, 80);
		output.push("", "latest logs:", logs.text);
		details.logs = logs.details;
	} catch {
		// Ignore log capture failures; ensure already did the important work.
	}

	return { text: trimOutput(output.join("\n")), details };
}

export async function runtimeLogs(tab: string, cwd: string, lines = 120): Promise<{ text: string; details: Record<string, unknown> }> {
	const result = await zmux(["watch", tab, "-l", String(lines)], { cwd, timeoutMs: 10_000 });
	return { text: trimOutput(result.stdout), details: { tab, lines } };
}

export async function runtimeStop(tab: string, cwd: string): Promise<{ text: string; details: Record<string, unknown> }> {
	await zmux(["send", tab, "C-c"], { cwd, timeoutMs: 5_000 });
	return { text: `sent C-c to ${tab}`, details: { tab } };
}

function delay(ms: number): Promise<void> {
	return new Promise((resolve) => setTimeout(resolve, ms));
}

async function tabExists(tab: string, cwd: string): Promise<boolean> {
	try {
		await runtimeLogs(tab, cwd, 1);
		return true;
	} catch {
		return false;
	}
}

async function ensureInteractiveShellTab(tab: string, cwd: string, focus: boolean): Promise<void> {
	const pane = await currentPane(cwd);
	if (!pane?.Session) throw new Error("cannot resolve current zmux session for interactive tab creation");
	const args = ["new-window"];
	if (!focus) args.push("-d");
	args.push("-t", pane.Session, "-n", tab, "-c", cwd, "exec bash -l");
	await tmux(args, { cwd, timeoutMs: 5_000 });
	await delay(300);
}

export async function focusTab(tab: string, cwd: string): Promise<{ text: string; details: Record<string, unknown> }> {
	const pane = await currentPane(cwd);
	if (!pane?.Session) throw new Error("cannot resolve current zmux session for tab focus");
	await tmux(["select-window", "-t", `${pane.Session}:${tab}`], { cwd, timeoutMs: 5_000 });
	return { text: `focused tab ${tab}`, details: { tab, session: pane.Session } };
}

export async function schedulePiRespawn(params: {
	cwd: string;
	paneId?: string;
	command?: string;
	delayMs?: number;
	continuationPrompt?: string;
}): Promise<{ text: string; details: Record<string, unknown> }> {
	const pane = params.paneId ?? (await currentPane(params.cwd))?.ID;
	if (!pane) throw new Error("cannot resolve current pane for Pi respawn");
	if (params.command && params.continuationPrompt) throw new Error("continuationPrompt cannot be combined with a custom restart command");
	const details: Record<string, unknown> = { pane, delayMs: params.delayMs ?? 300, method: "tmux respawn-pane -k" };
	let command = params.command ?? "pi -c";
	if (params.continuationPrompt) {
		const handoffDir = join(params.cwd, ".dump", "pi-zmux", "respawn-handoffs");
		await mkdir(handoffDir, { recursive: true });
		const handoffPath = join(handoffDir, `${new Date().toISOString().replace(/[:.]/gu, "-")}.md`);
		await writeFile(handoffPath, `${params.continuationPrompt.trim()}\n`);
		const continuationPath = writeRespawnContinuation(params.cwd, {
			createdAt: new Date().toISOString(),
			prompt: params.continuationPrompt.trim(),
			handoffPath,
		});
		details.continuationHandoff = handoffPath;
		details.continuationPath = continuationPath;
	}
	const delay = Math.max(0, params.delayMs ?? 300) / 1000;
	const script = [
		`cd ${shellQuote(params.cwd)}`,
		`sleep ${delay}`,
		`tmux respawn-pane -k -t ${shellQuote(pane)} -c ${shellQuote(params.cwd)} ${shellQuote(command)}`,
	].join("; ");
	spawnDetached("bash", ["-lc", script], { cwd: params.cwd });
	details.command = command;
	return { text: `scheduled Pi pane respawn for ${pane} using ${command}`, details };
}

function shellQuote(value: string): string {
	return `'${value.replace(/'/gu, `'"'"'`)}'`;
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
	constructor(
		readonly output: string,
		readonly prompt: UserInputPrompt,
	) {
		super(`user input required: ${prompt.kind}`);
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
	options: { detectUserInput?: boolean; baseline?: string } = {},
): Promise<string> {
	const deadline = Date.now() + timeoutSeconds * 1000;
	let latest = "";
	while (Date.now() <= deadline) {
		latest = (await runtimeLogs(tab, cwd, lines)).text;
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
): Promise<{ output: string; exitCode: number }> {
	const deadline = Date.now() + timeoutSeconds * 1000;
	let latest = "";
	while (Date.now() <= deadline) {
		latest = (await runtimeLogs(tab, cwd, lines)).text;
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
	if (!(await tabExists(tab, cwd))) {
		await ensureInteractiveShellTab(tab, cwd, focus);
	} else if (focus) {
		await focusTab(tab, cwd);
	}

	const timeoutSeconds = options.timeoutSeconds ?? 90;
	const lines = options.lines ?? 160;
	const output = [`typed command into ${tab}${focus ? " and focused it" : " without changing focus"}; user may need to respond there`];
	const details: Record<string, unknown> = { tab, command, waitForExit: options.waitForExit ?? false, focus };
	if (options.waitForExit) {
		const baseline = await runtimeLogs(tab, cwd, lines).then((logs) => logs.text).catch(() => "");
		const script = await writeWaitScript(command);
		try {
			await zmux(["type", tab, `bash ${shellQuote(script.runPath)}`], { cwd, timeoutMs: 5_000 });
			const result = await pollWaitScript(tab, cwd, lines, timeoutSeconds, script, baseline, !focus);
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
		const baseline = await runtimeLogs(tab, cwd, lines).then((logs) => logs.text).catch(() => "");
		await zmux(["type", tab, command], { cwd, timeoutMs: 5_000 });
		const waitPattern = new RegExp(options.waitFor);
		try {
			const result = await pollTab(tab, cwd, lines, timeoutSeconds, (text) => waitPattern.test(outputAfterBaseline(text, baseline)), {
				detectUserInput: !focus,
				baseline,
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
		await zmux(["type", tab, command], { cwd, timeoutMs: 5_000 });
	}
	return { text: output.join("\n"), details };
}
