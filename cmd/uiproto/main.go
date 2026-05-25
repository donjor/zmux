// Command uiproto is a UI prototyping harness. It hosts a Bubbletea
// TUI that lets you iterate on zmux UI visuals (status bar, dashboard
// rows, pickers, etc.) without touching production rendering paths.
//
// Usage:
//
//	go run ./cmd/uiproto           # launch TUI
//
// Controls:
//
//	tab / shift+tab   switch between UI pages
//	↑↓ / j k          focus the next/previous control
//	← → / h l         adjust the focused control
//	space / enter     toggle booleans / cycle choices
//	q / esc / ctrl+c  quit
//
// Each "page" is one previewable UI surface. Add pages by implementing
// preview.Page and registering them here.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/donjor/zmux/internal/preview"
	barpage "github.com/donjor/zmux/internal/preview/bar"
	panepage "github.com/donjor/zmux/internal/preview/pane"
)

var dumpMode = flag.Bool("dump", false, "render one frame of every preview page to stdout and exit (no TUI)")

func main() {
	flag.Parse()

	if *dumpMode {
		barpage.Dump(os.Stdout, 100)
		panepage.Dump(os.Stdout, 100)
		return
	}

	app := preview.NewApp(
		barpage.New(),
		panepage.New(),
		// Future: dashboardpage.New(), pickerpage.New(), ...
	)

	p := tea.NewProgram(app)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "uiproto:", err)
		os.Exit(1)
	}
}
