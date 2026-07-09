import { runFileStatus, trimOutput, type CommandStatusResult } from "../shell.js";
import { withSession, zmux, zmuxBin } from "./shared.js";

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

function numberedRemoteTabBase(tab?: string): string | undefined {
	if (!tab) return undefined;
	const match = /^(remote-[A-Za-z0-9_-]*?)([2-9]\d*)$/u.exec(tab);
	return match?.[1];
}

function sshHost(command: string): string | undefined {
	const match = /(?:^|[;&|]\s*)ssh\s+(?:-[A-Za-z0-9]+(?:\s+\S+)?\s+)*([A-Za-z0-9._-]+)/u.exec(command);
	return match?.[1];
}

function encodedRemoteCommandPreview(command: string): string | undefined {
	const match = /(?:^|\s)-EncodedCommand\s+([A-Za-z0-9+/=]+)/iu.exec(command);
	if (!match) return undefined;
	try {
		return Buffer.from(match[1], "base64").toString("utf16le").replace(/\u0000/g, "").trim().slice(0, 500);
	} catch {
		return "<could not decode encoded payload>";
	}
}

function looksLikeRemoteMutation(decoded: string | undefined, command: string): boolean {
	const haystack = `${decoded ?? ""}\n${command}`;
	return /\b(Add-Content|Set-Content|Out-File|New-Item|Remove-Item|Set-Item|Restart-Service|Stop-Service|Start-Service|setx)\b|\.env\b|\bDEPOT_|\b[A-Z0-9_]*(PASS|TOKEN|SECRET|USER)\b/iu.test(haystack);
}

export function zmuxRunSafetyWarnings(params: ZmuxRunParams): { text: string; details: Record<string, unknown> } {
	const warnings: string[] = [];
	const details: Record<string, unknown> = { warnings };
	const recommendedTab = numberedRemoteTabBase(params.tab);
	const host = sshHost(params.command);
	const decodedRemoteCommandPreview = encodedRemoteCommandPreview(params.command);

	if (recommendedTab) {
		warnings.push(`tab ${params.tab} looks like numbered remote tab sprawl; inspect/reuse one stable tab (${recommendedTab}) instead of creating suffix variants`);
		details.recommendedTab = recommendedTab;
	}
	if (decodedRemoteCommandPreview !== undefined) {
		warnings.push("opaque encoded remote/admin payload is hard to audit in tab history; decode/explain it before execution or prefer a visible script/here-doc when practical");
		details.decodedRemoteCommandPreview = decodedRemoteCommandPreview;
	}
	if (host && decodedRemoteCommandPreview !== undefined && looksLikeRemoteMutation(decodedRemoteCommandPreview, params.command)) {
		warnings.push(`about to change remote host ${host}; state the intended mutation before running encoded/admin commands`);
		details.remoteHost = host;
		details.remoteMutationLikely = true;
	}

	return { text: warnings.length ? `zmux_run safety warning:\n${warnings.map((warning) => `- ${warning}`).join("\n")}` : "", details };
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
	const timeoutSeconds = params.timeoutSeconds ?? 30;
	const timeoutMs = params.detach ? 15_000 : (timeoutSeconds + 5) * 1000;
	const result = await runFileStatus(zmuxBin(), buildZmuxRunArgs({ ...params, timeoutSeconds }), { cwd: params.cwd, timeoutMs });
	const output = trimOutput([result.stdout, result.stderr].filter(Boolean).join("\n"));
	const safety = zmuxRunSafetyWarnings(params);
	const details: Record<string, unknown> = {
		command: params.command,
		tab: params.tab,
		session: params.session,
		cwd: params.cwd,
		timeoutSeconds,
		detach: params.detach ?? false,
		follow: params.follow ?? false,
		...zmuxRunResultDetails(result, output),
		...safety.details,
	};
	const body = output || (result.failed ? `zmux run failed: ${details.warning}` : "zmux run completed");
	return { text: [safety.text, body].filter(Boolean).join("\n"), details };
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
