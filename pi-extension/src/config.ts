import { existsSync, readFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";

export type PolicyMode = "observe" | "warn" | "enforce";

export interface RuntimeConfig {
	command?: string;
	tab?: string;
	readiness?: string;
	kind?: string;
	cwd?: string;
	port?: number;
	timeoutSeconds?: number;
	session?: string;
}

export interface PiZmuxConfig {
	policy: {
		mode: PolicyMode;
		blockBackgroundJobs: boolean;
		redirectInteractive: boolean;
	};
	runtimes: Record<string, RuntimeConfig>;
	path?: string;
	ignoredReason?: "project-untrusted" | "invalid-json";
	projectTrusted: boolean;
}

export interface LoadConfigOptions {
	projectTrusted: boolean;
}

function defaultPolicy(): PiZmuxConfig["policy"] {
	return {
		mode: envMode() ?? "enforce",
		blockBackgroundJobs: true,
		redirectInteractive: true,
	};
}

function envMode(): PolicyMode | undefined {
	const mode = process.env.PI_ZMUX_POLICY;
	return mode === "observe" || mode === "warn" || mode === "enforce" ? mode : undefined;
}

function findConfig(startCwd: string): string | undefined {
	let dir = resolve(startCwd);
	for (;;) {
		for (const rel of [".pi/zmux.json", ".config/pi-zmux.json"]) {
			const candidate = join(dir, rel);
			if (existsSync(candidate)) return candidate;
		}
		const parent = dirname(dir);
		if (parent === dir) return undefined;
		dir = parent;
	}
}

function asBool(value: unknown, fallback: boolean): boolean {
	return typeof value === "boolean" ? value : fallback;
}

function asMode(value: unknown, fallback: PolicyMode): PolicyMode {
	return value === "observe" || value === "warn" || value === "enforce" ? value : fallback;
}

function asRuntimeConfig(value: unknown): RuntimeConfig | undefined {
	if (!value || typeof value !== "object" || Array.isArray(value)) return undefined;
	const input = value as Record<string, unknown>;
	const out: RuntimeConfig = {};
	for (const key of ["command", "tab", "readiness", "kind", "cwd", "session"] as const) {
		if (typeof input[key] === "string" && input[key].trim() !== "") out[key] = input[key].trim();
	}
	if (typeof input.port === "number") out.port = input.port;
	if (typeof input.timeoutSeconds === "number") out.timeoutSeconds = input.timeoutSeconds;
	return out;
}

export function loadConfig(cwd: string, options: LoadConfigOptions): PiZmuxConfig {
	const projectTrusted = options.projectTrusted;
	const path = findConfig(cwd);
	if (!path) return { policy: defaultPolicy(), runtimes: {}, projectTrusted };
	if (!projectTrusted) {
		return {
			path,
			policy: defaultPolicy(),
			runtimes: {},
			ignoredReason: "project-untrusted",
			projectTrusted,
		};
	}
	try {
		const raw = JSON.parse(readFileSync(path, "utf8")) as Record<string, unknown>;
		const policyInput = raw.policy && typeof raw.policy === "object" ? (raw.policy as Record<string, unknown>) : {};
		const runtimesInput = raw.runtimes && typeof raw.runtimes === "object" ? (raw.runtimes as Record<string, unknown>) : {};
		const runtimes: Record<string, RuntimeConfig> = {};
		for (const [name, value] of Object.entries(runtimesInput)) {
			const runtime = asRuntimeConfig(value);
			if (runtime) runtimes[name] = runtime;
		}
		return {
			path,
			projectTrusted,
			policy: {
				mode: envMode() ?? asMode(policyInput.mode, defaultPolicy().mode),
				blockBackgroundJobs: asBool(policyInput.blockBackgroundJobs, defaultPolicy().blockBackgroundJobs),
				redirectInteractive: asBool(policyInput.redirectInteractive, defaultPolicy().redirectInteractive),
			},
			runtimes,
		};
	} catch {
		return { policy: defaultPolicy(), runtimes: {}, path, ignoredReason: "invalid-json", projectTrusted };
	}
}

export function mergeRuntimeConfig(name: string, params: RuntimeConfig, config: PiZmuxConfig): Required<Pick<RuntimeConfig, "tab">> & RuntimeConfig {
	const configured = config.runtimes[name] ?? {};
	return {
		...configured,
		...params,
		tab: params.tab || configured.tab || name,
	};
}
