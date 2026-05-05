# pi-zmux config

Optional project config lives at `.pi/zmux.json` or `.config/pi-zmux.json` in the
current project or an ancestor directory.

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

Interactive one-shot commands can be waited on generically with a temporary
wrapper script and status file, without printing sentinel markers into the
terminal. With `focus: false`, common password/manual-input prompts return early
with `needsUserInput` so the agent can ask before switching focus:

```text
zmux_interactive_type({
  "tab": "admin",
  "command": "sudo ufw status",
  "waitForExit": true,
  "timeoutSeconds": 90,
  "focus": false
})
```
