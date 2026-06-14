package cli

import (
	"testing"

	"github.com/donjor/zmux/internal/theme"
	"github.com/donjor/zmux/internal/tmux"
)

// dimTestPalette returns a palette whose BGDim is a recognizable color so the
// dim sequence's bg= argument is easy to assert.
func dimTestPalette() *theme.Palette {
	return &theme.Palette{BGDim: theme.Color{R: 25, G: 29, B: 35}}
}

// mockCalledWith reports whether the mock recorded a call matching method +
// exact args.
func mockCalledWith(m *tmux.MockRunner, method string, args ...string) bool {
	for _, c := range m.Calls {
		if c.Method != method || len(c.Args) != len(args) {
			continue
		}
		match := true
		for i := range args {
			if c.Args[i] != args[i] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// touchedWindowStyles reports whether any window-style write happened.
func touchedWindowStyles(m *tmux.MockRunner) bool {
	for _, c := range m.Calls {
		if c.Method == "SetWindowOption" || c.Method == "UnsetWindowOption" {
			return true
		}
	}
	return false
}

func TestDimHostBehindPopupSetsThenUnsetsByDefault(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.InsideTmux = true
	pal := dimTestPalette()
	dimBG := "bg=" + pal.BGDim.Hex()

	restore := dimHostBehindPopup(mock, pal)

	// While open: both window styles are tinted to the dim background.
	if !mockCalledWith(mock, "SetWindowOption", "", "window-active-style", dimBG) {
		t.Error("expected window-active-style set to dim bg while popup open")
	}
	if !mockCalledWith(mock, "SetWindowOption", "", "window-style", dimBG) {
		t.Error("expected window-style set to dim bg while popup open")
	}

	restore()

	// No prior value was set, so restore unsets both back to the default.
	if !mockCalledWith(mock, "UnsetWindowOption", "", "window-active-style") {
		t.Error("expected window-active-style unset on restore")
	}
	if !mockCalledWith(mock, "UnsetWindowOption", "", "window-style") {
		t.Error("expected window-style unset on restore")
	}
}

func TestDimHostBehindPopupRestoresPriorStyle(t *testing.T) {
	mock := tmux.NewMockRunner()
	mock.InsideTmux = true
	// A pre-existing window-style must be put back verbatim, not unset. The
	// mock keys window options as "target\x00key" (empty target = current).
	mock.WindowOptions = map[string]string{"\x00window-style": "bg=#003344"}

	restore := dimHostBehindPopup(mock, dimTestPalette())
	restore()

	if !mockCalledWith(mock, "SetWindowOption", "", "window-style", "bg=#003344") {
		t.Error("expected the prior window-style to be restored verbatim")
	}
	// window-active-style had no prior value → unset.
	if !mockCalledWith(mock, "UnsetWindowOption", "", "window-active-style") {
		t.Error("expected window-active-style unset (no prior value)")
	}
}

func TestDimHostBehindPopupNoopWithoutPaletteOrTmux(t *testing.T) {
	// No palette (theme unresolved) → no tmux writes; restore is a safe no-op.
	inTmux := tmux.NewMockRunner()
	inTmux.InsideTmux = true
	dimHostBehindPopup(inTmux, nil)()
	if touchedWindowStyles(inTmux) {
		t.Error("nil palette must not touch window styles")
	}

	// Outside tmux (sessionless dashboard) → no writes even with a palette.
	noTmux := tmux.NewMockRunner()
	noTmux.InsideTmux = false
	dimHostBehindPopup(noTmux, dimTestPalette())()
	if touchedWindowStyles(noTmux) {
		t.Error("outside tmux must not touch window styles")
	}
}
