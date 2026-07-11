export type WaitSummary = {
	text: string;
	details: Record<string, unknown>;
};

export function summarizeWaitOutput(raw: string): WaitSummary {
	try {
		const parsed = JSON.parse(raw) as Record<string, unknown>;
		const outcome = parsed.outcome;
		if (!outcome || typeof outcome !== "object" || Array.isArray(outcome)) return { text: raw, details: {} };
		const result = outcome as Record<string, unknown>;
		const met = result.met === true;
		const basis = typeof result.basis === "string" ? result.basis : undefined;
		const state = typeof result.state === "string" ? result.state : undefined;
		const fresh = result.fresh === true;
		const status = result.status && typeof result.status === "object" && !Array.isArray(result.status)
			? result.status as Record<string, unknown>
			: undefined;
		const tab = typeof parsed.tab === "string" ? parsed.tab : undefined;
		const session = typeof parsed.session === "string" ? parsed.session : undefined;
		const paneId = typeof status?.paneId === "string"
			? status.paneId
			: typeof parsed.target === "string" && parsed.target.startsWith("%")
				? parsed.target
				: undefined;
		const evidence = [basis, state, fresh ? "fresh" : undefined].filter(Boolean).join(" · ");
		return {
			text: [`wait ${met ? "matched" : "did not match"}${tab ? ` ${tab}` : ""}`, evidence].filter(Boolean).join("\n"),
			details: {
				waitMet: met,
				ready: met,
				evidenceBasis: basis,
				waitState: state,
				fresh,
				tab,
				session,
				paneId,
				cmdState: typeof status?.cmdState === "string" ? status.cmdState : undefined,
				cmdSeq: typeof status?.cmdSeq === "string" ? status.cmdSeq : undefined,
				runId: typeof status?.runId === "string" ? status.runId : undefined,
			},
		};
	} catch {
		return { text: raw, details: {} };
	}
}
