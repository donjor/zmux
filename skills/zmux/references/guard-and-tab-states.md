# Guard & tab states (the zmux hooks layer)

The two `skills/zmux/hooks/` hooks that enforce this skill's hygiene: a `PreToolUse`
guard that keeps you on zmux instead of raw tmux, and a tab-state layer that renders
lifecycle glyphs in the bar. `SKILL.md` carries the invariants; this is the detail.

## Raw tmux → zmux verb mapping

zmux **is** the tmux wrapper. Raw `tmux` drops the `@zmux_label` pin + session/
workspace bookkeeping that keep tabs stably addressable — the window can auto-rename
out from under you and your next command lands on the wrong slot. Use the zmux verb;
it's almost always shorter:

| reaching for…                       | use instead                                        |
| ----------------------------------- | -------------------------------------------------- |
| `tmux capture-pane -t X`            | `zmux watch <tab>` (read-only; `--until` baselines)|
| `tmux send-keys -t X …`             | `zmux send <tab> <keys>` / `zmux type <tab> '…'`   |
| `tmux list-windows`                 | `zmux tabs`                                         |
| `tmux list-sessions` / `tmux ls`    | `zmux ls` (`-s` for a flat list)                   |
| `tmux list-panes`                   | `zmux pane list --json`                            |
| `tmux split-window …`               | `zmux pane open <name> -r 35 -- …`                 |
| `tmux select/kill/resize-pane`      | `zmux pane focus / close / resize`                 |
| `tmux new/kill/rename/move-window`  | `zmux run -n` / `zmux tab kill / label / move`     |
| `tmux new-session` / `attach`       | `zmux new` / `zmux open`                           |

## The guard hook

`hooks/zmux-guard.mjs` (symlinked into `~/.claude/hooks/`) **blocks** raw tmux calls
and prints the mapping back to you — so a slip self-corrects instead of silently
targeting the wrong window. The same guard enforces the rest of this skill's hygiene:
a dev server / background job (`npm run dev`, `&`, `nohup`) is **blocked** toward
`zmux run -n <name> -d`, and an interactive/remote command (`sudo`, `ssh`, a REPL)
draws a non-blocking **warn** nudging it into a shared tab.

**Exemptions** — genuinely need the raw command (zmux development, socket inspection,
a one-off)? Any of: prefix `ZMUX_ALLOW=1`, append `# zmux: allow`, use an explicit
`-L <socket>`, or run from the zmux repo.

## Tab lifecycle states

`zmux tab state <attention|running|done|failed|clear> [tab]` marks a tab's lifecycle;
the bar renders a colored glyph (● needs-human / ◐ running / ✓ done / ✗ failed)
visible from any tab.

Mostly automatic:

- `zmux run` sets running → done/failed on exit.
- `zmux send`/`type` clear a stale done/failed.
- Focusing a tab clears attention.
- A `Stop` hook (`hooks/zmux-tab-state-stop.mjs`, symlinked like the guard) marks the
  agent's own tab done/attention when a turn ends — no transcript parsing, just "the
  turn ended".

Set `attention` **manually** when handing the human a prompt they must act on (sudo,
permission prompt):

```bash
zmux tab state attention admin --msg 'sudo password'
```
