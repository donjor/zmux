# pi-zmux config

Optional project config lives at `.pi/zmux.json` or `.config/pi-zmux.json` in the
current project or an ancestor directory. Because config can contain commands,
the global extension reads it only when Pi trusts the project. If project trust is
false, the extension reports the config path as ignored and falls back to default
policy/no configured runtimes.

```json
{
  "policy": {
    "mode": "enforce",
    "blockBackgroundJobs": true,
    "redirectInteractive": true
  },
  "runtimes": {
    "server": {
      "command": "npm run dev",
      "tab": "server",
      "readiness": "ready|listening|localhost",
      "kind": "server",
      "timeoutSeconds": 90
    },
    "worker": {
      "command": "python -m app.worker",
      "tab": "worker",
      "readiness": "ready|started",
      "kind": "worker"
    }
  }
}
```

For objective grounding against the isolated profile, set the zmux binary:

```sh
PI_ZMUX_BIN=zzmux pi -e ./pi-extension
```

If a low-level pane operation needs raw tmux, `PI_ZMUX_BIN=zzmux` implies
`tmux -L zzmux`. Override with `PI_ZMUX_TMUX_SOCKET=<socket>` for custom profiles.

Interactive one-shot commands can be waited on generically with a tool-owned
temporary wrapper script and status file, without printing sentinel markers into
the terminal. Agents should not create their own temp scripts or done markers for
this. With `focus: false`, common password/manual-input prompts return early with
`needsUserInput` so the agent can ask before switching focus:

```text
zmux_interactive_type({
  "tab": "admin",
  "command": "sudo ufw status",
  "waitForExit": true,
  "timeoutSeconds": 90,
  "focus": false
})
```

After Pi extension/skill changes, prefer the soft Pi reload path:

```text
zmux_pi_reload({
  "continuationPrompt": "Reload complete; verify the updated tools and continue."
})
```

It uses zmux/tmux to type Pi's built-in `/reload` into the current Pi pane after
a short delay, then nudges the agent after reload. `zmux_reload` is reserved for
zmux's own config reload. Use `zmux_pi_respawn` only when soft Pi reload is
unavailable or Pi is wedged.
