# `zmux terminal capabilities`

Diagnoses whether tmux is passing truecolor/RGB through from the outer terminal
client to panes.

This matters for smooth grayscale animations and gradients. Apps inside tmux can
emit truecolor ANSI such as `ESC[48;2;R;G;Bm`, but tmux will quantize those
colors to the 256-color palette unless the outer client terminal has RGB/Tc
capability enabled.

## Usage

```bash
zmux terminal capabilities
zmux terminal capabilities --json
```

Text output marks the current tmux client with `*` and reports each attached
client's termname, resolved tmux features, and RGB status.

JSON output uses schema `zmux-terminal-capabilities/v1` and includes the inside
pane environment, tmux version, current client TTY, and per-client RGB status.

## Expected Ghostty path

Ghostty commonly exposes itself to tmux as:

```text
TERM=xterm-256color
COLORTERM=truecolor
TERM_PROGRAM=ghostty
```

That is valid, but tmux matches `terminal-features` against the **outer client**
termname (`xterm-256color` or `xterm-ghostty`), not the pane's inner
`TERM=tmux-256color`.

zmux-generated tmux config therefore enables RGB for common outer clients. zmux
uses deterministic `terminal-features[90..99]` entries so repeated `zmux apply`
keeps the live tmux server idempotent instead of appending duplicates:

```tmux
# zmux owns terminal-features[90..99] to keep repeated apply idempotent.
set -g terminal-features[90] "xterm-256color:RGB:extkeys"
set -g terminal-features[91] "xterm-ghostty:RGB:extkeys"
set -g terminal-features[92] "tmux-256color:RGB:extkeys"
```

After changing terminal features, existing attached tmux clients may need to
reconnect before tmux re-resolves their capabilities:

```bash
zmux refresh
zmux terminal capabilities
```

`zmux refresh` applies zmux's generated tmux config, then replaces the current
attached tmux client with a freshly attached one using RGB terminal features. It
is the non-manual equivalent of apply + detach/reattach; the screen will briefly
redraw while the client is replaced. The lower-level `zmux terminal refresh`
command is available when you only want to replace the client without applying
configuration first.

Healthy output should show `truecolor=RGB` for the current client.
