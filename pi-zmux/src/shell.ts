import { execFile, spawn } from "node:child_process";
import { promisify } from "node:util";

const execFileAsync = promisify(execFile);

export interface CommandResult {
	stdout: string;
	stderr: string;
}

export interface CommandStatusResult extends CommandResult {
	exitCode: number | null;
	signal?: NodeJS.Signals | string;
	failed: boolean;
	timedOut?: boolean;
	message?: string;
}

function execOptions(options: { cwd?: string; timeoutMs?: number } = {}) {
	return {
		cwd: options.cwd,
		timeout: options.timeoutMs ?? 15_000,
		maxBuffer: 1024 * 1024,
	};
}

export async function runFileStatus(command: string, args: string[], options: { cwd?: string; timeoutMs?: number } = {}): Promise<CommandStatusResult> {
	try {
		const { stdout, stderr } = await execFileAsync(command, args, execOptions(options));
		return { stdout: String(stdout), stderr: String(stderr), exitCode: 0, failed: false };
	} catch (error) {
		const err = error as NodeJS.ErrnoException & { stdout?: string | Buffer; stderr?: string | Buffer; code?: number | string | null; signal?: NodeJS.Signals | string; killed?: boolean };
		return {
			stdout: err.stdout ? String(err.stdout) : "",
			stderr: err.stderr ? String(err.stderr) : "",
			exitCode: typeof err.code === "number" ? err.code : null,
			signal: err.signal,
			failed: true,
			timedOut: err.killed === true && err.signal === "SIGTERM",
			message: err.message,
		};
	}
}

export async function runFile(command: string, args: string[], options: { cwd?: string; timeoutMs?: number } = {}): Promise<CommandResult> {
	const result = await runFileStatus(command, args, options);
	if (!result.failed) return { stdout: result.stdout, stderr: result.stderr };
	throw new Error(`${command} ${args.join(" ")} failed: ${result.stderr || result.stdout || result.message || "unknown error"}`);
}

export function spawnDetached(command: string, args: string[], options: { cwd?: string } = {}): void {
	const child = spawn(command, args, {
		cwd: options.cwd,
		detached: true,
		stdio: "ignore",
	});
	child.unref();
}

export function trimOutput(value: string): string {
	return value.replace(/\s+$/u, "");
}
