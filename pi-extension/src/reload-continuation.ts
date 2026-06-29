import { existsSync, mkdirSync, readFileSync, renameSync, unlinkSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";

export interface ReloadContinuation {
	createdAt: string;
	prompt: string;
}

export function reloadContinuationPath(cwd: string): string {
	return join(cwd, ".dump", "pi-zmux", "reload-continuation.json");
}

export function writeReloadContinuation(cwd: string, continuation: ReloadContinuation): string {
	const target = reloadContinuationPath(cwd);
	mkdirSync(dirname(target), { recursive: true });
	const tmp = `${target}.${process.pid}.tmp`;
	writeFileSync(tmp, `${JSON.stringify(continuation, null, 2)}\n`, "utf8");
	renameSync(tmp, target);
	return target;
}

export function takeReloadContinuation(cwd: string): ReloadContinuation | undefined {
	const target = reloadContinuationPath(cwd);
	if (!existsSync(target)) return undefined;
	try {
		const parsed = JSON.parse(readFileSync(target, "utf8")) as ReloadContinuation;
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
