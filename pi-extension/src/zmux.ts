import { chmod, mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { basename, join } from "node:path";
import { writeReloadContinuation } from "./reload-continuation.js";
import { writeRespawnContinuation } from "./respawn-continuation.js";
import { runFile, runFileStatus, spawnDetached, trimOutput, type CommandStatusResult } from "./shell.js";

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
	session?: string;
}

function zmuxBin(): string {
	return process.env.PI_ZMUX_BIN?.trim() || "zmux";
}

function tmuxPrefix(): string[] {
	const explicitSocket = process.env.PI_ZMUX_TMUX_SOCKET?.trim();
	if (explicitSocket) return ["-L", explicitSocket];
	if (basename(zmuxBin()) === "zzmux") return ["-L", "zzmux"];
	return [];
}

function withSession(args: string[], session?: string): string[] {
	return session ? [...args, "-s", session] : args;
}

export async function zmux(args: string[], options: { cwd?: string; timeoutMs?: number } = {}) {
	return runFile(zmuxBin(), args, options);
}

async function tmux(args: string[], options: { cwd?: string; timeoutMs?: number } = {}) {
	return runFile("tmux", [...tmuxPrefix(), ...args], options);
}

export async function currentPane(cwd: string): Promise<CurrentPane | undefined> {
	try {
		const result = await zmux(["pane", "current", "--json"], { cwd, timeoutMs: 5_000 });
		return JSON.parse(result.stdout) as CurrentPane;
	} catch {
		return undefined;
	}
}

export async function listTabs(cwd: string, session?: string): Promise<string> {
	try {
		const result = await zmux(withSession(["tabs"], session), { cwd, timeoutMs: 5_000 });
		return trimOutput(result.stdout);
	} catch (error) {
		return `unavailable: ${error instanceof Error ? error.message : String(error)}`;
	}
}

export async function killTab(tab: string, cwd: string): Promise<{ text: string; details: Record<string, unknown> }> {
	await zmux(["tab", "kill", tab], { cwd, timeoutMs: 10_000 });
	return { text: `killed tab ${tab}`, details: { tab } };
}

export async function sendKeys(tab: string, keys: string[], cwd: string, session?: string): Promise<{ text: string; details: Record<string, unknown> }> {
	await zmux(withSession(["send", tab, ...keys], session), { cwd, timeoutMs: 5_000 });
	return { text: `sent keys to ${tab}: ${keys.join(" ")}`, details: { tab, keys, session } };
}

export async function sendPaneKeys(pane: string, keys: string[], cwd: string): Promise<{ text: string; details: Record<string, unknown> }> {
	await tmux(["send-keys", "-t", pane, ...keys], { cwd, timeoutMs: 5_000 });
	return { text: `sent keys to pane ${pane}: ${keys.join(" ")}`, details: { pane, keys } };
}

export async function typeText(tab: string, text: string, cwd: string, session?: string): Promise<{ text: string; details: Record<string, unknown> }> {
	await zmux(withSession(["type", tab, text], session), { cwd, timeoutMs: 5_000 });
	return { text: `typed text into ${tab}`, details: { tab, text, session } };
}

export async function typePaneText(pane: string, text: string, cwd: string): Promise<{ text: string; details: Record<string, unknown> }> {
	await tmux(["send-keys", "-t", pane, text, "Enter"], { cwd, timeoutMs: 5_000 });
	return { text: `typed text into pane ${pane}`, details: { pane, text } };
}

export async function listPanes(cwd: string, session?: string): Promise<string> {
	try {
		const args = session ? ["pane", "list", "--session", "--target", session] : ["pane", "list"];
		const result = await zmux(args, { cwd, timeoutMs: 5_000 });
		return trimOutput(result.stdout);
	} catch (error) {
		return `unavailable: ${error instanceof Error ? error.message : String(error)}`;
	}
}

export function buildPaneOpenArgs(params: { name: string; command: string; cwd: string; direction?: "right" | "left" | "down" | "up"; size?: string; target?: string; labelTab?: boolean }): string[] {
	const args = ["pane", "open", params.name, "--cwd", params.cwd];
	if (params.target) args.push("--target", params.target);
	const directionFlag = params.direction ? ({ right: "-r", left: "-l", down: "-d", up: "-u" } as const)[params.direction] : "-r";
	args.push(directionFlag);
	if (params.size) args.push(params.size);
	if (params.labelTab) args.push("--label-tab");
	args.push("--", "bash", "-lc", params.command);
	return args;
}

export async function openPane(params: { name: string; command: string; cwd: string; direction?: "right" | "left" | "down" | "up"; size?: string; target?: string; labelTab?: boolean }): Promise<{ text: string; details: Record<string, unknown> }> {
	await zmux(buildPaneOpenArgs(params), { cwd: params.cwd, timeoutMs: 10_000 });
	return { text: `opened pane ${params.name}`, details: { ...params } };
}

export async function focusPane(pane: string, cwd: string): Promise<{ text: string; details: Record<string, unknown> }> {
	await zmux(["pane", "focus", pane], { cwd, timeoutMs: 5_000 });
	return { text: `focused pane ${pane}`, details: { pane } };
}

export async function closePane(pane: string, cwd: string): Promise<{ text: string; details: Record<string, unknown> }> {
	await zmux(["pane", "close", pane], { cwd, timeoutMs: 5_000 });
	return { text: `closed pane ${pane}`, details: { pane } };
}

export async function resizePane(pane: string, cwd: string, size: string): Promise<{ text: string; details: Record<string, unknown> }> {
	await zmux(["pane", "resize", pane, "--size", size], { cwd, timeoutMs: 5_000 });
	return { text: `resized pane ${pane} to ${size}`, details: { pane, size } };
}

export async function capabilities(cwd: string): Promise<string> {
	try {
		const result = await zmux(["terminal", "capabilities"], { cwd, timeoutMs: 5_000 });
		return trimOutput(result.stdout);
	} catch (error) {
		return `unavailable: ${error instanceof Error ? error.message : String(error)}`;
	}
}

export async function reloadZmux(cwd: string): Promise<{ text: string; details: Record<string, unknown> }> {
	const result = await zmux(["reload"], { cwd, timeoutMs: 15_000 });
	const output = trimOutput([result.stdout, result.stderr].filter(Boolean).join("\n"));
	return { text: output || "reloaded zmux", details: { command: "zmux reload" } };
}

export type TabStateAction = "attention" | "running" | "done" | "failed" | "clear";
export type TabPlacementAction = "pane" | "full" | "hide" | "show";
export type TabPlacementDirection = "right" | "left" | "up" | "down";
export type LogAction = "start" | "tail" | "status" | "stop";

export interface ZmuxRunParams {
	command: string;
	cwd: string;
	tab?: string;
	session?: string;
	timeoutSeconds?: number;
	lines?: number;
	detach?: boolean;
	follow?: boolean;
	keep?: boolean;
	scope?: string;
}

export function buildZmuxRunArgs(params: ZmuxRunParams): string[] {
	const args = ["run", "--command", params.command];
	if (params.tab) args.push("-n", params.tab);
	if (params.detach) args.push("-d");
	if (params.follow) args.push("-f");
	if (params.timeoutSeconds !== undefined) args.push("-T", String(params.timeoutSeconds));
	if (params.lines !== undefined) args.push("--lines", String(params.lines));
	if (params.keep) args.push("--keep");
	if (params.scope) args.push("--scope", params.scope);
	return withSession(args, params.session);
}

export function zmuxRunResultDetails(result: CommandStatusResult, output: string): Record<string, unknown> {
	const details: Record<string, unknown> = {
		zmuxExitCode: result.exitCode,
		failed: result.failed,
	};
	if (result.signal) details.signal = result.signal;
	if (result.timedOut) {
		details.failureKind = "tool_timeout";
		details.warning = result.message ?? "zmux run timed out at the Pi tool boundary";
		return details;
	}
	const commandExit = /command exited with code (\d+)/u.exec(output);
	if (commandExit) {
		details.failureKind = "command_exit";
		details.exitCode = Number(commandExit[1]);
		details.warning = `command exited with ${commandExit[1]}`;
		return details;
	}
	const zmuxTimeout = /timeout after (\d+)s/u.exec(output);
	if (zmuxTimeout) {
		details.failureKind = "zmux_timeout";
		details.timeoutSeconds = Number(zmuxTimeout[1]);
		details.warning = `zmux run timed out after ${zmuxTimeout[1]}s`;
		return details;
	}
	if (result.failed) {
		details.failureKind = "zmux_failure";
		details.exitCode = result.exitCode;
		details.warning = result.exitCode !== null ? `zmux exited with ${result.exitCode}` : (result.message ?? "zmux run failed");
	} else {
		details.exitCode = 0;
	}
	return details;
}

export async function runCommand(params: ZmuxRunParams): Promise<{ text: string; details: Record<string, unknown> }> {
	const timeoutSeconds = params.timeoutSeconds ?? 120;
	const timeoutMs = params.detach ? 15_000 : (timeoutSeconds + 5) * 1000;
	const result = await runFileStatus(zmuxBin(), buildZmuxRunArgs({ ...params, timeoutSeconds }), { cwd: params.cwd, timeoutMs });
	const output = trimOutput([result.stdout, result.stderr].filter(Boolean).join("\n"));
	const details: Record<string, unknown> = {
		command: params.command,
		tab: params.tab,
		session: params.session,
		cwd: params.cwd,
		timeoutSeconds,
		detach: params.detach ?? false,
		follow: params.follow ?? false,
		...zmuxRunResultDetails(result, output),
	};
	return { text: output || (result.failed ? `zmux run failed: ${details.warning}` : "zmux run completed"), details };
}

export function buildSessionListArgs(params: { workspace?: string; flat?: boolean } = {}): string[] {
	const args = ["ls"];
	if (params.flat) args.push("-s");
	if (params.workspace) args.push(params.workspace);
	return args;
}

export async function listSessions(cwd: string, params: { workspace?: string; flat?: boolean } = {}): Promise<{ text: string; details: Record<string, unknown> }> {
	const result = await zmux(buildSessionListArgs(params), { cwd, timeoutMs: 5_000 });
	return { text: trimOutput(result.stdout), details: { ...params } };
}

export function buildSessionRunArgs(params: { sessionName: string; tab: string; command: string; workspace?: string; cwd?: string }): string[] {
	const args = ["session", "run", params.sessionName, "-n", params.tab];
	if (params.workspace) args.push("--workspace", params.workspace);
	if (params.cwd) args.push("--cwd", params.cwd);
	args.push("--", "bash", "-lc", params.command);
	return args;
}

export async function sessionRun(params: { sessionName: string; tab: string; command: string; cwd: string; workspace?: string; commandCwd?: string }): Promise<{ text: string; details: Record<string, unknown> }> {
	const args = buildSessionRunArgs({ sessionName: params.sessionName, tab: params.tab, command: params.command, workspace: params.workspace, cwd: params.commandCwd });
	const result = await zmux(args, { cwd: params.cwd, timeoutMs: 10_000 });
	const output = trimOutput([result.stdout, result.stderr].filter(Boolean).join("\n"));
	return { text: output || `created session ${params.sessionName} with tab ${params.tab}`, details: { ...params, args } };
}

export async function sessionKill(sessionName: string, cwd: string): Promise<{ text: string; details: Record<string, unknown> }> {
	await zmux(["session", "kill", sessionName], { cwd, timeoutMs: 10_000 });
	return { text: `killed session ${sessionName}`, details: { sessionName } };
}

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

export function buildTabPlacementArgs(params: { action: TabPlacementAction; tab?: string; session?: string; into?: string; direction?: TabPlacementDirection; size?: string; pane?: string; after?: boolean }): string[] {
	const args = ["tab", params.action];
	if (params.tab) args.push(params.tab);
	if (params.session) args.push("--session", params.session);
	if (params.action === "pane") {
		if (params.into) args.push("--into", params.into);
		if (params.direction) args.push(`--${params.direction}`);
		if (params.size) args.push("--size", params.size);
		return args;
	}
	if (params.pane) args.push("--pane", params.pane);
	if (params.action === "full" && params.after) args.push("--after");
	return args;
}

export async function placeTab(params: { action: TabPlacementAction; cwd: string; tab?: string; session?: string; into?: string; direction?: TabPlacementDirection; size?: string; pane?: string; after?: boolean }): Promise<{ text: string; details: Record<string, unknown> }> {
	const result = await zmux(buildTabPlacementArgs(params), { cwd: params.cwd, timeoutMs: 10_000 });
	return { text: trimOutput([result.stdout, result.stderr].filter(Boolean).join("\n")) || `tab ${params.action}`, details: { ...params } };
}

export async function terminalCurrent(cwd: string): Promise<{ text: string; details: Record<string, unknown> }> {
	const result = await zmux(["terminal", "current", "--json"], { cwd, timeoutMs: 5_000 });
	return { text: trimOutput(result.stdout), details: { terminal: safeJson(result.stdout) } };
}

function safeJson(value: string): unknown {
	try {
		return JSON.parse(value);
	} catch {
		return undefined;
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
	session?: string;
}): Promise<{ text: string; details: Record<string, unknown> }> {
	const details: Record<string, unknown> = { tab: params.tab, command: params.command, cwd: params.cwd, session: params.session };
	const output: string[] = [];

	if (params.restart) {
		try {
			await zmux(withSession(["send", params.tab, "C-c"], params.session), { cwd: params.cwd, timeoutMs: 5_000 });
			output.push(`sent C-c to ${params.tab}`);
		} catch (error) {
			output.push(`restart stop skipped: ${error instanceof Error ? error.message : String(error)}`);
		}
	}

	await zmux(withSession(["run", params.command, "-n", params.tab, "-d"], params.session), { cwd: params.cwd, timeoutMs: 10_000 });
	output.push(`runtime ${params.tab} ensured via zmux run -d`);

	if (params.labelTab) {
		try {
			await zmux(withSession(["tab", "label", params.tab], params.session), { cwd: params.cwd, timeoutMs: 5_000 });
			details.labelTab = true;
		} catch {
			// Labeling is helpful but not required for runtime ownership.
		}
	}

	if (params.readiness) {
		const timeout = String(params.timeoutSeconds ?? 90);
		try {
			const watch = await zmux(withSession(["watch", params.tab, "--until", params.readiness, "-T", timeout, "-l", "120"], params.session), {
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
		const logs = await runtimeLogs(params.tab, params.cwd, 80, params.session);
		output.push("", "latest logs:", logs.text);
		details.logs = logs.details;
	} catch {
		// Ignore log capture failures; ensure already did the important work.
	}

	return { text: trimOutput(output.join("\n")), details };
}

export async function runtimeLogs(tab: string, cwd: string, lines = 120, session?: string): Promise<{ text: string; details: Record<string, unknown> }> {
	const result = await zmux(withSession(["watch", tab, "-l", String(lines)], session), { cwd, timeoutMs: 10_000 });
	return { text: trimOutput(result.stdout), details: { tab, lines, session } };
}

export async function runtimeStop(tab: string, cwd: string, session?: string): Promise<{ text: string; details: Record<string, unknown> }> {
	await zmux(withSession(["send", tab, "C-c"], session), { cwd, timeoutMs: 5_000 });
	return { text: `sent C-c to ${tab}`, details: { tab, session } };
}

function delay(ms: number): Promise<void> {
	return new Promise((resolve) => setTimeout(resolve, ms));
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

export async function focusTab(tab: string, cwd: string): Promise<{ text: string; details: Record<string, unknown> }> {
	const pane = await currentPane(cwd);
	if (!pane?.Session) throw new Error("cannot resolve current zmux session for tab focus");
	await tmux(["select-window", "-t", `${pane.Session}:${tab}`], { cwd, timeoutMs: 5_000 });
	return { text: `focused tab ${tab}`, details: { tab, session: pane.Session } };
}

export async function schedulePiReload(params: {
	cwd: string;
	paneId?: string;
	delayMs?: number;
	continuationPrompt?: string;
}): Promise<{ text: string; details: Record<string, unknown> }> {
	const pane = params.paneId ?? (await currentPane(params.cwd))?.ID;
	if (!pane) throw new Error("cannot resolve current pane for Pi reload");
	const prompt = params.continuationPrompt?.trim() || "Pi runtime reload complete. Continue the work from before reload; first verify the reloaded tool/extension surface if that was the reason for reload.";
	const continuationPath = writeReloadContinuation(params.cwd, {
		createdAt: new Date().toISOString(),
		prompt,
	});
	const delayMs = params.delayMs ?? 5_000;
	const script = buildPiReloadScript({ cwd: params.cwd, pane, delayMs });
	spawnDetached("bash", ["-lc", script], { cwd: params.cwd });
	return {
		text: `scheduled Pi /reload for ${pane}`,
		details: { pane, delayMs, continuationPath, method: "tmux send-keys /reload Enter" },
	};
}

export function buildPiReloadScript(params: { cwd: string; pane: string; delayMs: number }): string {
	const delay = Math.max(0, params.delayMs) / 1000;
	const tmuxArgs = ["tmux", ...tmuxPrefix(), "send-keys", "-t", params.pane, "/reload", "Enter"];
	return [
		`cd ${shellQuote(params.cwd)}`,
		`sleep ${delay}`,
		tmuxArgs.map(shellQuote).join(" "),
	].join("; ");
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
	const script = buildTmuxRespawnScript({
		cwd: params.cwd,
		pane,
		command,
		delayMs: params.delayMs ?? 300,
	});
	spawnDetached("bash", ["-lc", script], { cwd: params.cwd });
	details.command = command;
	return { text: `scheduled Pi pane respawn for ${pane} using ${command}`, details };
}

export function buildTmuxRespawnScript(params: { cwd: string; pane: string; command: string; delayMs: number }): string {
	const delay = Math.max(0, params.delayMs) / 1000;
	const tmuxArgs = ["tmux", ...tmuxPrefix(), "respawn-pane", "-k", "-t", params.pane, "-c", params.cwd, params.command];
	return [
		`cd ${shellQuote(params.cwd)}`,
		`sleep ${delay}`,
		tmuxArgs.map(shellQuote).join(" "),
	].join("; ");
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
