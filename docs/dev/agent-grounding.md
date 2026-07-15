# Agent Grounding & Dev QA

How an agent **grounds and QAs zmux changes itself** before declaring them done.

> Scope: **developing zmux**, not using it. This is for agents working on this
> repo. Consumer-facing "how to use zmux" doctrine lives in the shipped
> `skills/zmux/SKILL.md` — keep dev/test/grounding instructions out of it.

## The split: who confirms what

Every check is either objective or subjective. Route by who can confirm it:

- **Objective → the agent drives it.** Anything observable without judgement:
  a CLI prints the right output, a flag toggles state, a tab is killed/spared,
  a conf hook is present, an option round-trips. Default surface is the
  isolated **zzmux** profile (below). This is the agent's job, not the human's.
- **Subjective → the human.** Only what needs taste or a real human-in-the-loop:
  does a nudge *read* as helpful vs naggy, does a layout *look* right, is the
  live attached-session experience pleasant. Plus the one destructive gate that
  is the human's call (activating reaping on the real `zmux` profile).

> The goal: the agent grounds everything objective; the human's QA plate shrinks
> to **purely subjective**. An unchecked agent box in `notes/QA.md` should mean
> "genuinely not drivable", not "agent didn't bother".

## zzmux — the grounding sandbox

`zzmux` is the isolated edge profile: its own socket, config, state dir, and
generated tmux conf — fully separate from the live `zmux` you're running inside.
Drive it freely; you cannot break the active session.

```sh
./dev.sh zzmux        # build + install the edge binary (binary only — safe in-session)
```

Then drive it **headless** (no attached client needed for most things):

```sh
# detached session + first tab
zzmux session run dbg -n shell --workspace boop -- bash
zzmux run 'echo hi' -n work -s dbg -d   # add a tab
zzmux ls -s                             # inspect
zzmux tabs dbg
zzmux reap --dry-run                    # classify
zzmux session kill zws_boop__dbg        # tear down when done
```

Notes that bite:

- `./dev.sh zzmux` installs only the edge binary. It intentionally does **not**
  rewrite `~/.bashrc`, `~/.profile`, shared skills, or the Pi extension package.
  If a shell-integration change needs live activation, prove it on `zzmux` first,
  then ask before running the live `./dev.sh` / `zmux setup shell` path. Use
  `zzmux doctor` to distinguish an outdated rc file from an already-open shell
  that simply needs a fresh tab to load the new hook version.
- Real session names are profile-mangled (`zws_<ws>__<session>`), e.g.
  `dbg` → `zws_boop__dbg`. Raw `tmux -L zzmux -t dbg` will NOT resolve a zmux
  label — address by the full tmux name, or via `zmux` verbs. For raw reads use
  `tmux -L zzmux list-panes -a -F '...'` and filter by `window_name`.
- `./dev.sh` with no arg reinstalls the **live** binary — disruptive in-session.
  Always `./dev.sh zzmux` for edge testing.
- Stale baked conf is a classic false bug: if profile isolation misbehaves,
  re-apply (`zzmux apply`) before code-diving.

## Time injection — testing the age/idle/flag timeline

Behavior gated on wall-clock age (the reaper's flag-then-kill ramp spans hours)
is driven through a deterministic clock instead of waiting:

```sh
zzmux reap --now <unix-seconds>            # override the policy clock (hidden flag)
zzmux reap --dry-run --now <unix-seconds>  # classify at that instant, change nothing
```

This is the same `Now`-injection the library tests use, exposed on the binary
for live grounding. Real `window_activity` can't be backdated, so inject a
**future** Now (e.g. `born + 100000`) to treat a real pane as aged. Worked
example — drive the full human flag→kill timeline end to end:

```sh
born=$(
  tmux -L zzmux list-panes -a -F '#{window_name} #{@zmux_born}' |
    awk '$1=="oldtab"{print $2}'
)
T1=$((born + 100000)); T2=$((T1 + 3600))
zzmux reap --now "$T1"   # +27.8h → flags (marks stale_at)
zzmux reap --now "$T2"   # +28.8h → kills the flagged tab, spares the last window
```

## Spawn protocol — the few attached-client cases

A handful of behaviors need a **real attached client** and cannot be driven
headless:

- the `visible-in-attached-client` keep-guard (`SessionAttached > 0`),
- tmux `client-attached` / `session-created` hooks firing naturally,
- truecolor / visual rendering, dashboard/popup UI.

For these, the human spawns an attached instance and the agent drives the same
socket:

1. **Agent asks** for an attached zzmux instance, naming the workspace/session
   it needs (e.g. "attach `zzmux open boop dbg` in a terminal").
2. **Human spawns** it in a real terminal:

   ```sh
   zzmux open <ws> [session]   # aliases: attach, a   — attaches a real client
   ```

   (or bare `zzmux` for the dashboard). That's the human's whole job here.
3. **Agent drives** the same profile headless (`zzmux run/send/type/watch/reap`,
   raw `tmux -L zzmux` reads) — the attached client now makes `SessionAttached`,
   hook firing, and visible-pane state real, while the agent does the work.
4. **Agent tears down** (`zzmux session kill ...`) and reports.

Keep the human's part to spawning the client. Everything before and after is the
agent's to drive.

## Prompt-driven agent-surface QA

For broad changes to the shipped zmux skill, Pi extension tools, guardrails, or
peer/worker flows, deterministic checks should be paired with a fresh-session
prompt run:

- `test-prompts/zmux-agent-skill-testing-prompt.md` activates the durable Claude/CLI framework under `agent-doctrine/harnesses/claude/`.
- `test-prompts/zmux-agent-pi-zmux-testing-prompt.md` targets the durable Pi framework under `agent-doctrine/harnesses/pi/` — deferred with the Pi extension reintegration and not present on this shared branch.

These files are thin launch wrappers, not behavior sources. Shared outcomes and prompts
come from authored Markdown under `agent-doctrine/scenarios/shared/`; generated worker prompts
and host-only answer keys render to stdout through `agent-doctrine/generate.mjs --render`.
Each framework owns harness-specific launch/inspection/teardown.
`make test-agent-surfaces` remains the repeatable deterministic gate.

## What stays on the human's plate

After the agent has grounded every objective check on zzmux, the residual human
track in a plan's `notes/QA.md` should be only:

- **subjective feel** — wording, polish, "is this the experience we wanted",
- **the live multi-hour wall-clock proof** where injected time isn't enough,
- **destructive activation on the real `zmux` profile** — the human's gate;
  the agent offers a `zmux reap --dry-run` preview, the human decides.

If a check sitting in the human track is actually objective, it belongs in the
agent track — drive it on zzmux instead of handing it over.
