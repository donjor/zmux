// GENERATED FILE — edit agent-doctrine/ and run `make gen-doctrine`.

export const SHARED_ZMUX_PROMPT_GUIDELINES = [
  "Use zmux instead of bash/raw tmux for runtimes, visible tabs, panes, sessions, waits, peers, and Pi lifecycle; never background long-running commands.",
  "Start with operation=sessions or tabs when target/session is ambiguous; never operate on a generic tab name like scratch unless the prompt names the exact tab/session.",
  "Never set focus:true unless the user explicitly wants terminal focus moved; if visibility is requested without focus, keep focus false.",
  "For servers/watchers, use runtime_ensure/logs/stop. If asked for another copy before checking logs, use runtime_logs on the existing target first and do not start duplicate processes.",
  "For sudo, ssh, passwords, REPLs, database shells, and other manual input, use interactive_type and never generic run; target one stable admin/remote tab, keep focus false, and use bounded waitForExit when appropriate.",
  "For wait/callback_watch, choose exactly one of waitFor or idleSeconds with a bounded timeout; waitFor is the regex only, outgoing text must not satisfy future evidence, and deliverAs=nextTurn cannot trigger a continuation.",
  "For remote/admin runs, reuse one stable admin/remote-host tab, decode opaque payloads, and state the intended host mutation before changing remote config."
] as const;

export const PI_DOCTRINE_RULE_IDS = [
  "ZD-001",
  "ZD-002",
  "ZD-003",
  "ZD-004",
  "ZD-005",
  "ZD-006",
  "ZD-007",
  "ZD-008",
  "ZD-009",
  "ZD-010",
  "ZD-011",
  "ZD-012"
] as const;
