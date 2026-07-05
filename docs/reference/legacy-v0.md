# Legacy v0 command reference

The bash+gum prototype under `legacy/v0/` is archived and unsupported. New work
belongs in the Go implementation. Keep this file only so command-surface docs
remain explicit for the preserved scripts.

## Invocation

```bash
./legacy/v0/bin/zmux0
./legacy/v0/bin/zmux0-apply-theme
```

The helper scripts under `legacy/v0/lib/` and `legacy/v0/templates/` were sourced
or copied by the old prototype. They are not the current install path.

## Command inventory

| Path | Purpose |
| ---- | ------- |
| `legacy/v0/bin/zmux0` | Old prototype entry point. |
| `legacy/v0/bin/zmux0-apply-theme` | Old theme application helper. |
| `legacy/v0/lib/help-popup.sh` | Old popup help helper. |
| `legacy/v0/lib/init.sh` | Old initialization helper. |
| `legacy/v0/lib/startup-info.sh` | Old startup summary helper. |
| `legacy/v0/lib/status.sh` | Old status rendering helper. |
| `legacy/v0/lib/sync.sh` | Old shell/config sync helper. |
| `legacy/v0/lib/theme.sh` | Old theme helper library. |
| `legacy/v0/templates/claude.sh` | Old Claude-oriented template command. |
| `legacy/v0/templates/dev.sh` | Old development template command. |
| `legacy/v0/templates/monitor.sh` | Old monitoring template command. |
| `legacy/v0/templates/webdev.sh` | Old webdev template command. |

## Args, options, and modes

Do not add new flags or modes here. If you need behavior from v0, port it into
the Go command tree under `internal/cli` and document the new `zmux` command in
[cli.md](cli.md).

## Output and write behavior

These scripts may print gum/tmux UI output and may write old profile files,
templates, or tmux config used by the prototype. Treat their writes as legacy
state; they are not part of the current `~/.zmux.toml` / Go profile contract.

## Failure and help expectations

The archived scripts may not meet current `--help`, unknown-argument rejection,
or structured-exit standards. That is acceptable only because the surface is
archived. Current commands must satisfy the floor in [cli.md](cli.md).

## Update triggers

Update this file only if archived files are moved, removed, or explicitly
retired from the repository.
