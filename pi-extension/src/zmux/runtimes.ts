import { trimOutput } from "../shell.js";
import { watchTabOutput } from "./agent.js";
import { tabStatus } from "./tabs.js";
import { withSession, zmux } from "./shared.js";

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
		const status = await tabStatus({ tab: params.tab, cwd: params.cwd, session: params.session });
		details.status = status.details.status;
	} catch {
		// Older/stale binaries may not expose status; ensure already did the important work.
	}

	return { text: trimOutput(output.join("\n")), details };
}

export async function runtimeLogs(tab: string, cwd: string, lines = 120, session?: string, options: { waitFor?: string; idleSeconds?: number; timeoutSeconds?: number } = {}): Promise<{ text: string; details: Record<string, unknown> }> {
	return watchTabOutput({ tab, cwd, lines, session, waitFor: options.waitFor, idleSeconds: options.idleSeconds, timeoutSeconds: options.timeoutSeconds });
}

export async function runtimeStop(tab: string, cwd: string, session?: string): Promise<{ text: string; details: Record<string, unknown> }> {
	await zmux(withSession(["send", tab, "C-c"], session), { cwd, timeoutMs: 5_000 });
	return { text: `sent C-c to ${tab}`, details: { tab, session } };
}
