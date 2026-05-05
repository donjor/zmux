import { execFile, spawn } from "node:child_process";
import { promisify } from "node:util";

const execFileAsync = promisify(execFile);

export interface CommandResult {
	stdout: string;
	stderr: string;
}

export async function runFile(command: string, args: string[], options: { cwd?: string; timeoutMs?: number } = {}): Promise<CommandResult> {
	try {
		const { stdout, stderr } = await execFileAsync(command, args, {
			cwd: options.cwd,
			timeout: options.timeoutMs ?? 15_000,
			maxBuffer: 1024 * 1024,
		});
		return { stdout: String(stdout), stderr: String(stderr) };
	} catch (error) {
		const err = error as NodeJS.ErrnoException & { stdout?: string | Buffer; stderr?: string | Buffer };
		const stderr = err.stderr ? String(err.stderr) : err.message;
		const stdout = err.stdout ? String(err.stdout) : "";
		throw new Error(`${command} ${args.join(" ")} failed: ${stderr || stdout}`);
	}
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
