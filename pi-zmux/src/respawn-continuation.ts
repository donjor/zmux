import { existsSync, mkdirSync, readFileSync, renameSync, unlinkSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";

export interface RespawnContinuation {
	createdAt: string;
	prompt: string;
	handoffPath?: string;
}

export function respawnContinuationPath(cwd: string): string {
	return join(cwd, ".dump", "pi-zmux", "respawn-continuation.json");
}

export function writeRespawnContinuation(cwd: string, continuation: RespawnContinuation): string {
	const target = respawnContinuationPath(cwd);
	mkdirSync(dirname(target), { recursive: true });
	const tmp = `${target}.${process.pid}.tmp`;
	writeFileSync(tmp, `${JSON.stringify(continuation, null, 2)}\n`, "utf8");
	renameSync(tmp, target);
	return target;
}

export function takeRespawnContinuation(cwd: string): RespawnContinuation | undefined {
	const target = respawnContinuationPath(cwd);
	if (!existsSync(target)) return undefined;
	try {
		const parsed = JSON.parse(readFileSync(target, "utf8")) as RespawnContinuation;
		unlinkSync(target);
		if (!parsed.prompt?.trim()) return undefined;
		return parsed;
	} catch {
		try {
			renameSync(target, `${target}.invalid-${Date.now()}`);
		} catch {
			// Ignore quarantine failures; session startup should not fail because of a stale handoff.
		}
		return undefined;
	}
}
