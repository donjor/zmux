package tabs

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/donjor/zmux/internal/bar"
	"github.com/donjor/zmux/internal/config"
	"github.com/donjor/zmux/internal/theme"
	"github.com/donjor/zmux/internal/tui"
	"github.com/donjor/zmux/internal/tui/dashboard"
)

// Data + mutation commands for the ThemesTab. Split out of themes.go
// during plan 011 phase 6 to mirror the current_data.go split and
// bring themes.go under 400 lines.
//
// Every mutation closure snapshots its dependencies before returning
// the tea.Cmd so the command observes a stable view of the tab's
// state at the moment it was scheduled (the tab struct may have
// mutated by the time the scheduler runs the closure).

// themesGuard is the shared preamble used by every message handler in
// ThemesTab.Update: drop the message if the reqID is stale (a newer
// mutation / fetch has superseded it), or flash a status if the
// message carries an error. Returns (errCmd, proceed). When proceed is
// false the caller should early-return with the cmd (which may be nil
// for the stale case or a SetStatusIntent flash for the error case).
func themesGuard(tabReqID, msgReqID int64, err error, errFmt string) (tea.Cmd, bool) {
	if msgReqID != tabReqID {
		return nil, false
	}
	if err != nil {
		text := fmt.Sprintf(errFmt, err)
		return func() tea.Msg {
			return dashboard.SetStatusIntent{Text: text, IsError: true}
		}, false
	}
	return nil, true
}

// loadActiveConfig returns the path, the parsed config, and any load
// error. Every mutation command needs the same two-call dance
// (ConfigPath → Load with DefaultConfig fallback on parse failure), so
// it lives here once.
func loadActiveConfig(fs config.FS) (string, config.Config, error) {
	cfgPath, err := config.ConfigPath(fs)
	if err != nil {
		return "", config.Config{}, err
	}
	cfg, err := config.Load(fs, cfgPath)
	if err != nil {
		cfg = config.DefaultConfig()
	}
	return cfgPath, cfg, nil
}

// fetchData loads the theme list + current theme name into a themesDataMsg.
func (t *ThemesTab) fetchData(reqID int64) tea.Cmd {
	resolver := t.resolver
	fs := t.fs
	return func() tea.Msg {
		var themes []theme.ThemeInfo
		if resolver != nil {
			themes = resolver.List()
		}

		_, cfg, err := loadActiveConfig(fs)
		if err != nil {
			return themesDataMsg{reqID: reqID, err: err}
		}

		return themesDataMsg{
			reqID:        reqID,
			themes:       themes,
			currentTheme: cfg.Theme,
		}
	}
}

// applyTheme writes the theme to the config, then hot-reloads tmux
// env vars and the bar colors for the new palette.
func (t *ThemesTab) applyTheme(name string) tea.Cmd {
	fs := t.fs
	runner := t.runner
	resolver := t.resolver
	reqID := t.reqID
	return func() tea.Msg {
		cfgPath, cfg, err := loadActiveConfig(fs)
		if err != nil {
			return themesApplyMsg{reqID: reqID, err: err}
		}

		cfg.Theme = name
		if err := config.Save(fs, cfgPath, cfg); err != nil {
			return themesApplyMsg{reqID: reqID, err: err}
		}

		// Hot-reload: apply theme env vars + bar colors.
		var pal *theme.Palette
		var sty *tui.Styles
		if runner != nil && resolver != nil {
			resolved, resolveErr := resolver.Resolve(name)
			if resolveErr == nil {
				_ = theme.Apply(runner, fs, &cfg, resolved, cfgPath)
				p := resolved.SemanticPalette()
				preset, _ := bar.PresetFromString(cfg.Bar.Preset)
				lc := bar.BarLayoutConfig{
					Layout:    cfg.Bar.Layout,
					Indicator: cfg.Bar.Indicator,
					TopBar:    cfg.Bar.TopBar,
				}
				_ = bar.Apply(runner, preset, &p, lc)
				pal = &p
				s := tui.NewStyles(&p)
				sty = &s
			}
		}

		return themesApplyMsg{
			reqID:     reqID,
			themeName: name,
			palette:   pal,
			styles:    sty,
		}
	}
}

// revertPreview clears the ephemeral "we're currently previewing a
// non-persisted theme" flag. Pairs with emitRevert.
func (t *ThemesTab) revertPreview() {
	t.previewing = false
}

// emitRevert produces a ThemeChangeIntent that broadcasts the saved
// palette + styles back to the dashboard, reverting a preview. No-op
// if we never stashed a preview snapshot.
func (t *ThemesTab) emitRevert() tea.Cmd {
	if t.savedPalette == nil || t.savedStyles == nil {
		return nil
	}
	pal := *t.savedPalette
	sty := *t.savedStyles
	return func() tea.Msg {
		return dashboard.ThemeChangeIntent{Palette: pal, Styles: sty}
	}
}

// saveThemeFile writes the in-progress edited theme to
// ~/.zmux/themes/<name>.
func (t *ThemesTab) saveThemeFile() tea.Cmd {
	fs := t.fs
	editTheme := t.editTheme
	editName := t.editName
	reqID := t.reqID
	return func() tea.Msg {
		home, err := fs.UserHomeDir()
		if err != nil {
			return themesSaveThemeMsg{reqID: reqID, err: err}
		}

		dir := home + "/.zmux/themes"
		_ = fs.MkdirAll(dir, 0755)

		path := dir + "/" + editName
		if err := theme.WriteFile(fs, path, editTheme); err != nil {
			return themesSaveThemeMsg{reqID: reqID, err: fmt.Errorf("save theme: %w", err)}
		}

		return themesSaveThemeMsg{reqID: reqID, themeName: editName}
	}
}
