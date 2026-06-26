package tmux

import (
	"fmt"
	"strings"
	"testing"

	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/theme"
)

func TestGenerateConfContainsGeneral(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	checks := []string{
		`set -g default-terminal "tmux-256color"`,
		`set -g mouse on`,
		`set -g history-limit 50000`,
		`set -g escape-time 50`,
		`set -g repeat-time 500`,
		`set -g base-index 1`,
		`setw -g pane-base-index 1`,
		`set -g renumber-windows on`,
		`setw -g automatic-rename on`,
		`setw -g automatic-rename-format "#{?pane_in_mode,[tmux],#{pane_current_command}}"`,
		`setw -g remain-on-exit failed`,
		`setw -g remain-on-exit-format "Pane stopped unexpectedly#{?#{!=:#{pane_dead_status},}, (status #{pane_dead_status}),}#{?#{!=:#{pane_dead_signal},}, (signal #{pane_dead_signal}),} — press prefix+x to close the tab"`,
		`set -g set-clipboard on`,
		`set -g status-position top`,
		`set -g set-titles on`,
		`set -g set-titles-string "zmux:v1;tty=#{client_tty};sid=#{session_id};wid=#{window_id};pane=#{pane_id} #{?#{@zmux_session_label},#{@zmux_workspace}/#{@zmux_session_label},#{session_name}}:#{window_index}:#{window_name}"`,
		`set -g focus-events on`,
		`set -g extended-keys on`,
		`set -g terminal-features[90] "xterm-256color:RGB:extkeys"`,
		`set -g terminal-features[91] "xterm-ghostty:RGB:extkeys"`,
		`set -g terminal-features[92] "tmux-256color:RGB:extkeys"`,
	}

	for _, want := range checks {
		if !strings.Contains(conf, want) {
			t.Errorf("conf missing general setting: %q", want)
		}
	}
}

func TestGenerateConfContainsReapHooks(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	// Baked-in reaper sweeps on attach + session birth (plan 038).
	checks := []string{
		`set-hook -g client-attached[1] "run-shell -b '/usr/local/bin/zmux reap --lazy --quiet >/dev/null 2>&1 || true'"`,
		`set-hook -g session-created[3] "run-shell -b '/usr/local/bin/zmux reap --lazy --quiet >/dev/null 2>&1 || true'"`,
	}
	for _, want := range checks {
		if !strings.Contains(conf, want) {
			t.Errorf("conf missing reap hook: %q", want)
		}
	}

	// Regression: an UNINDEXED `set-hook -g session-created`/`session-closed`
	// REPLACES the whole hook array at tmux runtime, silently wiping the indexed
	// refresh[2] and reaper[3] siblings (a string-contains test alone misses
	// this — the conf still "contains" [3], it just doesn't survive apply). The
	// bar-adjust hooks must therefore be pinned to [0], never bare.
	for _, bad := range []string{
		`set-hook -g session-created "`,
		`set-hook -g session-closed "`,
	} {
		if strings.Contains(conf, bad) {
			t.Errorf("conf has UNINDEXED hook that clobbers its indexed siblings (incl. the reaper): %q", bad)
		}
	}
	for _, want := range []string{
		`set-hook -g session-created[0] "run-shell '/usr/local/bin/zmux bar-adjust'"`,
		`set-hook -g session-closed[0] "run-shell '/usr/local/bin/zmux bar-adjust'"`,
	} {
		if !strings.Contains(conf, want) {
			t.Errorf("conf missing indexed bar-adjust hook: %q", want)
		}
	}
}

func TestGenerateConfContainsPrefix(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	if !strings.Contains(conf, "unbind C-b") {
		t.Error("conf missing 'unbind C-b'")
	}
	if !strings.Contains(conf, "set -g prefix C-Space") {
		t.Error("conf missing prefix set")
	}
	if !strings.Contains(conf, "bind C-Space send-prefix") {
		t.Error("conf missing prefix bind")
	}
}

func TestGenerateConfCustomPrefix(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Prefix = "C-a"
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	if !strings.Contains(conf, "set -g prefix C-a") {
		t.Error("conf should use custom prefix C-a")
	}
	if !strings.Contains(conf, "bind C-a send-prefix") {
		t.Error("conf should bind custom prefix C-a")
	}
}

func TestGenerateConfContainsVimCopyMode(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	checks := []string{
		"setw -g mode-keys vi",
		"bind v copy-mode",
		"bind R respawn-pane -k",
		"bind -T copy-mode-vi v send -X begin-selection",
		"bind -T copy-mode-vi Escape send -X cancel",
		"bind -T copy-mode-vi C-v send -X rectangle-toggle",
	}

	for _, want := range checks {
		if !strings.Contains(conf, want) {
			t.Errorf("conf missing vim copy mode setting: %q", want)
		}
	}
}

func TestGenerateConfContainsAltTabSwitching(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	for i := 1; i <= 5; i++ {
		want := "bind -n M-" + string(rune('0'+i)) + " select-window -t " + string(rune('0'+i))
		if !strings.Contains(conf, want) {
			t.Errorf("conf missing alt tab binding: %q", want)
		}
	}
}

func TestGenerateConfContainsPrefixAltSessionSwitching(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	for i := 1; i <= 9; i++ {
		want := fmt.Sprintf(`bind M-%d run-shell "/usr/local/bin/zmux workspace switch-to %d"`, i, i)
		if !strings.Contains(conf, want) {
			t.Errorf("conf missing prefix alt session binding: %q", want)
		}
	}

	if strings.Contains(conf, "bind -n M-1 run-shell") {
		t.Error("session switching should stay behind prefix, not bind in the root table")
	}
}

func TestGenerateConfContainsWindowBindings(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	checks := []string{
		`bind c new-window -c "#{pane_current_path}"`,
		"bind n next-window",
		`bind . command-prompt -p "label tab (blank clears):"`,
		`set-option -w -t #{window_id} @zmux_label`,
		`bind J command-prompt -p "join tab here:" "run-shell '/usr/local/bin/zmux tab pane --notify \"%%\"'"`,
		`bind F run-shell "/usr/local/bin/zmux tab full --after --notify"`,
		`bind x confirm-before`,
		`#{?@zmux_label,#{?#{==:#{@zmux_label},#W},#W,#{@zmux_label} [#W]},#W#{?@zmux_duplicate_name,[#{b:pane_current_path}],}}`,
	}

	for _, want := range checks {
		if !strings.Contains(conf, want) {
			t.Errorf("conf missing window binding: %q", want)
		}
	}
}

func TestGenerateConfContainsNoPrefixPaneFocusBindings(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	checks := []string{
		"unbind -n M-Left",
		"unbind -n M-Right",
		"unbind -n M-Up",
		"unbind -n M-Down",
		"unbind -n M-S-Left",
		"unbind -n M-S-Right",
		"unbind -n M-S-Up",
		"unbind -n M-S-Down",
		"bind -n M-S-Left select-pane -L",
		"bind -n M-S-Right select-pane -R",
		"bind -n M-S-Up select-pane -U",
		"bind -n M-S-Down select-pane -D",
	}
	for _, want := range checks {
		if !strings.Contains(conf, want) {
			t.Errorf("conf missing pane focus binding: %q", want)
		}
	}

	for _, old := range []string{"M-Left", "M-Right", "M-Up", "M-Down"} {
		if strings.Contains(conf, "bind -n "+old+" select-pane") {
			t.Errorf("conf should not use plain Alt pane focus binding %q", old)
		}
	}
}

func TestGenerateConfContainsPaneLayoutBindings(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	checks := []string{
		// Directional swaps are repeatable (-r) so a pane can be walked.
		"bind -r S-Left swap-pane -t '{left-of}'",
		"bind -r S-Right swap-pane -t '{right-of}'",
		"bind -r S-Up swap-pane -t '{up-of}'",
		"bind -r S-Down swap-pane -t '{down-of}'",
		"bind = select-layout -E",
		// Orientation toggle: format-conditional on window_layout (no shell spawn).
		`bind s if -F "#{m:*{*,#{window_layout}}" "select-layout even-vertical" "select-layout even-horizontal"`,
	}
	for _, want := range checks {
		if !strings.Contains(conf, want) {
			t.Errorf("conf missing pane layout binding: %q", want)
		}
	}

	// prefix+s is the orient toggle now, no longer a session-picker alias.
	if strings.Contains(conf, "bind s display-popup") {
		t.Error("prefix+s should be the orient toggle, not the session picker")
	}
}

func TestGenerateConfContainsSessionBindings(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	checks := []string{
		`bind , command-prompt -p "rename session:"`,
		"bind w display-popup",
		"bind [ run-shell",
		"bind ] run-shell",
		`bind x confirm-before`,
	}

	for _, want := range checks {
		if !strings.Contains(conf, want) {
			t.Errorf("conf missing session binding: %q", want)
		}
	}
}

func TestGenerateConfContainsPopupBinding(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	if !strings.Contains(conf, "bind Space display-popup") {
		t.Error("conf missing dashboard popup binding (prefix+Space)")
	}
}

func TestGenerateConfContainsHelpPopup(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	if !strings.Contains(conf, `bind ? display-popup`) {
		t.Error("conf missing help popup binding (prefix+?)")
	}
	if !strings.Contains(conf, "zmux help") {
		t.Error("conf missing zmux help in help popup")
	}
}

func TestGenerateConfContainsBootstrap(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	if !strings.Contains(conf, `run-shell "/usr/local/bin/zmux apply"`) {
		t.Error("conf missing run-shell bootstrap")
	}
}

func TestGenerateConfContainsClipboard(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	// Should contain some clipboard binding (the exact one depends on host)
	if !strings.Contains(conf, "copy-mode-vi y") {
		t.Error("conf missing clipboard y binding")
	}
}

func TestGenerateConfContainsPasteBuffer(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	if !strings.Contains(conf, "bind P paste-buffer") {
		t.Error("conf missing paste-buffer binding (prefix+P)")
	}
}

func TestGenerateConfContainsPaletteBinding(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	if !strings.Contains(conf, "bind p display-popup") {
		t.Error("conf missing command palette popup binding (prefix+p)")
	}
	if !strings.Contains(conf, "--palette") {
		t.Error("conf missing --palette flag in palette popup binding")
	}
}

func TestGenerateConfContainsScratchShell(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	if !strings.Contains(conf, "bind ! display-popup") {
		t.Error("conf missing scratch shell popup binding (prefix+!)")
	}
	if !strings.Contains(conf, `-d "#{pane_current_path}"`) {
		t.Error("conf missing pane cwd start dir for scratch shell")
	}
	if !strings.Contains(conf, `"$SHELL"`) {
		t.Error("conf missing $SHELL invocation in scratch shell popup")
	}
	// Scratch popup advertises the extract subcommand in its title so users
	// can discover the "promote scratch cwd to a real tab" affordance
	// without reading the docs.
	if !strings.Contains(conf, "zmux scratch extract") {
		t.Error("conf missing zmux scratch extract hint in scratch popup title")
	}
}

// When zmuxBin is empty (os.Executable failed AND a zero Profile — never the
// real app.New path), every self-invoking popup bind is OMITTED rather than
// falling back to a hardcoded "zmux". This guards the deleted palette/dashboard
// else-branches: a future edit that re-introduces a literal "zmux --…" fallback
// would break zzmux isolation, and this test catches it.
func TestGenerateConfEmptyBinOmitsSelfInvokingBinds(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "")

	for _, frag := range []string{
		"--tab-picker", "--workspace-picker", "--picker",
		"--palette", "--dashboard",
		"zmux --palette", "zmux --dashboard", // the removed hardcoded else-branches
	} {
		if strings.Contains(conf, frag) {
			t.Errorf("empty-bin conf should omit self-invoking bind, but contains %q", frag)
		}
	}
	// The scratch popup is always emitted (runs $SHELL, not the zmux binary) and
	// falls back to the literal "zmux" hint when no binary path is known.
	if !strings.Contains(conf, "zmux scratch extract") {
		t.Error("empty-bin conf should still emit the scratch popup with default 'zmux' hint")
	}
}

func TestGenerateConfPopupBorderRounded(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	if !strings.Contains(conf, "set -g popup-border-lines rounded") {
		t.Error("conf missing rounded popup-border-lines option")
	}
}

func TestGenerateConfPopupBorderStyleUsesDim(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	want := "set -g popup-border-style fg=" + palette.Dim.Hex()
	if !strings.Contains(conf, want) {
		t.Errorf("conf missing popup-border-style in dim color; want %q", want)
	}
}

// A nil palette (theme unresolved) must skip the colored border-style without
// panicking, while the palette-independent border-lines option still ships.
func TestGenerateConfNilPaletteSkipsBorderStyle(t *testing.T) {
	cfg := config.DefaultConfig()
	conf := GenerateConf(&cfg, nil, "/usr/local/bin/zmux")

	if strings.Contains(conf, "popup-border-style") {
		t.Error("nil palette should not emit a popup-border-style line")
	}
	if !strings.Contains(conf, "set -g popup-border-lines rounded") {
		t.Error("border-lines should ship regardless of palette")
	}
}

func TestGenerateConfLeavesDetachOnDestroyDefault(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	if strings.Contains(conf, "detach-on-destroy") {
		t.Error("generated config must leave tmux detach-on-destroy at its default/on value")
	}
}

func TestGenerateConfContainsReload(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	if !strings.Contains(conf, "bind r run-shell") {
		t.Error("conf missing reload binding")
	}
}

func TestGenerateConfContainsDuplicateNameRefreshHooks(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	for _, want := range []string{
		`set-hook -gu window-unlinked[0]`,
		`set-hook -gu window-linked[1]`,
		`set-hook -gu window-unlinked[1]`,
		`set-hook -gu window-renamed[1]`,
		`set-hook -g window-linked[1] "run-shell -b '/usr/local/bin/zmux tab refresh-names #{session_name} >/dev/null 2>&1 || true'"`,
		`set-hook -g window-unlinked[1] "run-shell -b '/usr/local/bin/zmux tab refresh-names #{session_name} >/dev/null 2>&1 || true'"`,
		`set-hook -g window-renamed[1] "run-shell -b '/usr/local/bin/zmux tab refresh-names #{session_name} >/dev/null 2>&1 || true'"`,
	} {
		if !strings.Contains(conf, want) {
			t.Errorf("conf missing duplicate-name hook: %q", want)
		}
	}
}

// Bare `refresh-client -S` in a status hook errors "(null):0: no current
// client" when the hook fires with nothing attached (e.g. a zmux CLI command
// from an unattached shell triggers session-created/closed). The refresh must
// be guarded by the triggering client's tty so it skips silently in that case.
func TestGenerateConfGuardsRefreshHooksAgainstNoClient(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	for _, ev := range []string{
		"client-session-changed", "session-window-changed", "window-linked",
		"window-renamed", "session-renamed", "session-created[2]", "session-closed[2]",
	} {
		want := fmt.Sprintf(`set-hook -g %s "if-shell -F '#{client_tty}' 'refresh-client -S'"`, ev)
		if !strings.Contains(conf, want) {
			t.Errorf("conf missing guarded refresh hook for %s:\n  want substring: %q", ev, want)
		}
		// The unguarded form must not survive — it's the source of the error.
		if strings.Contains(conf, fmt.Sprintf(`set-hook -g %s "refresh-client -S"`, ev)) {
			t.Errorf("conf still has UNGUARDED refresh hook for %s (would error with no client)", ev)
		}
	}
}

// Focus clears `attention` via the *-changed hooks — pane-focus-in is
// deliberately absent (doesn't fire without focus-events; plan 026 spike C).
// Hook commands must be silent and fail-open like the other generated hooks.
func TestGenerateConfFocusClearHooks(t *testing.T) {
	cfg := config.DefaultConfig()
	palette := testPalette()
	conf := GenerateConf(&cfg, &palette, "/usr/local/bin/zmux")

	cmd := "/usr/local/bin/zmux tab state clear --target #{pane_id} --if attention --source focus --quiet >/dev/null 2>&1 || true"
	for _, ev := range []string{"session-window-changed[1]", "window-pane-changed"} {
		want := fmt.Sprintf(`set-hook -g %s "run-shell -b '%s'"`, ev, cmd)
		if !strings.Contains(conf, want) {
			t.Errorf("conf missing focus-clear hook for %s:\n  want substring: %q", ev, want)
		}
	}
	if strings.Contains(conf, "pane-focus-in") {
		t.Error("pane-focus-in must not be used — it does not fire without focus-events (spike C)")
	}

	// No zmux binary → no focus-clear hooks (nothing to run).
	confNoBin := GenerateConf(&cfg, &palette, "")
	if strings.Contains(confNoBin, "tab state clear") {
		t.Error("focus-clear hooks must be omitted without a zmux binary")
	}
}

func testPalette() theme.Palette {
	c := func(r, g, b uint8) theme.Color { return theme.Color{R: r, G: g, B: b} }
	return theme.Palette{
		BG:        c(10, 14, 20),
		FG:        c(184, 191, 198),
		Surface:   c(0, 0, 0),
		Error:     c(255, 51, 51),
		Success:   c(127, 204, 60),
		Accent:    c(255, 180, 84),
		Info:      c(54, 163, 217),
		Special:   c(209, 97, 203),
		Meta:      c(149, 230, 203),
		Muted:     c(200, 200, 200),
		Dim:       c(100, 100, 100),
		Highlight: c(255, 204, 102),
		BGDim:     c(25, 29, 35),
		BGPrefix:  c(35, 39, 45),
	}
}
